package filesyncer

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path"
)

type Syncer struct {
	Replica   bool
	Conn      io.ReadWriteCloser
	FileCache *FileCache
}

func (s *Syncer) SendMessage(msg Message) error {
	msgBuf := msg.AsBytesBuf()
	totalWritten := 0
	for totalWritten < len(msgBuf) {
		n, err := s.Conn.Write(msgBuf[totalWritten:])
		slog.Debug("SendMessage", "sent", string(msgBuf[totalWritten:]))
		if err != nil {
			return errors.Join(err, fmt.Errorf("Could not send msg data to tcp connection"))
		}
		totalWritten += n
	}
	return nil
}

// Send finish msg from main
func (s *Syncer) SendFinish() error {
	msg := Message{Type: MsgTypeFinish}
	err := s.SendMessage(msg)
	if err != nil {
		return err
	}
	return nil
}

func (s *Syncer) Run() error {
	if s.Replica {
		return s.RunAsReplica()
	} else {
		return s.RunAsMain()
	}
}
func (s *Syncer) RunAsMain() error {
	defer s.Conn.Close()
	reader := bufio.NewReader(s.Conn)

	for fileName, fcData := range s.FileCache.data {
		// Send out msg to reciver to replica
		checkMsg := Message{Type: MsgTypeCheck, FileName: fileName, MD5: fcData.md5}
		_, err := s.Conn.Write(checkMsg.AsBytesBuf()) // I am going to assume the msg is so small I don't need to check n
		slog.Debug("Main check message sent", "type", string(checkMsg.Type), "filename", checkMsg.FileName, "md5", checkMsg.MD5)

		if err != nil {
			slog.Error("Could not send message for fileCheck", "filename", fileName, "error", err)
			return fmt.Errorf("failed to send check message for file %s: %w", fileName, err)
		}

		// Check response from replica then send if non matching
		msgStream, err := reader.ReadBytes('\x00')
		if err != nil {
			slog.Error("Could not read incomming data from replica on check request", "error", err)
			return fmt.Errorf("failed to read response from replica: %w", err)
		}
		msg, err := ParseMessage(msgStream)
		if err != nil {
			slog.Error("Could not parse message from msg stream from replica on check request", "error", err)
			return fmt.Errorf("failed to parse message from replica: %w", err)
		}
		slog.Debug("Main received match message", "type", string(msg.Type), "filename", msg.FileName, "match", msg.Match)
		if msg.Type != MsgTypeMatch {
			slog.Error("Unexpected msg type from replica on check request", "expected", string(MsgTypeMatch), "got", string(msg.Type))
			return fmt.Errorf("unexpected message type from replica: expected %c, got %c", MsgTypeMatch, msg.Type)
		}
		if !msg.Match {
			if err := s.SendFile(fileName); err != nil {
				slog.Error("Failed to send file", "filename", fileName, "error", err)
				return err
			}
		}
	}

	err := s.SendFinish()
	if err != nil {
		slog.Error("Failed to send finish msg", "error", err)
		return fmt.Errorf("failed to send finish message: %w", err)
	}
	slog.Debug("Main sent finish message", "type", string(MsgTypeFinish))
	return nil
}

func (s *Syncer) RunAsReplica() error {
	defer s.Conn.Close()

	reader := bufio.NewReader(s.Conn)

	// Not sure how I feel about labels...
OUTER:
	for {
		msgStream, err := reader.ReadBytes('\x00')
		if err != nil {
			slog.Error("Replica could not read incomming data", "error", err)
			return fmt.Errorf("failed to read message from main: %w", err)
		}

		msg, err := ParseMessage(msgStream)
		if err != nil {
			slog.Error("Replica could not parse message from msg stream", "error", err)
			return fmt.Errorf("failed to parse message from main: %w", err)
		}

		switch msg.Type {
		case MsgTypeFinish:
			slog.Debug("Replica received finish message", "type", string(msg.Type))
			break OUTER

		case MsgTypeCheck:
			slog.Debug("Replica received check message", "type", string(msg.Type), "filename", msg.FileName, "md5", msg.MD5)
			responseMessage := Message{Type: MsgTypeMatch, FileName: msg.FileName}
			fileData, ok := s.FileCache.data[msg.FileName]
			responseMessage.Match = ok && fileData.md5 == msg.MD5
			respErr := s.SendMessage(responseMessage)
			slog.Debug("Replica sent match message", "type", string(responseMessage.Type), "filename", responseMessage.FileName, "match", responseMessage.Match)
			if respErr != nil {
				slog.Error("Replica failed to send the response message for the md5 file check", "filename", msg.FileName, "error", respErr)
				return fmt.Errorf("Failed to send match response for file %s: %w", msg.FileName, respErr)
			}

			// Update the file cache
			if responseMessage.Match {
				fileData.synced = true
				s.FileCache.data[msg.FileName] = fileData
			}

		case MsgTypeMatch:
			slog.Error("Replica received unexpected Match message type", "type", string(msg.Type))
			return fmt.Errorf("Replica should not receive Match messages")

		case MsgTypeData:
			slog.Debug("Replica received data message", "type", string(msg.Type), "filename", msg.FileName, "dataSize", len(msg.Data))
			if err := s.WriteFile(msg); err != nil {
				slog.Error("Failed to write file", "filename", msg.FileName, "error", err)
				return err
			}
			fileData, ok := s.FileCache.data[msg.FileName]
			if ok {
				fileData.synced = true
				s.FileCache.data[msg.FileName] = fileData
			} else {
				// Not the best idea to just set md5 to empty but only using it for final check on synced so ok for now
				s.FileCache.data[msg.FileName] = fileCacheData{md5: "", synced: true}
			}

		default:
			slog.Error("Replica received unknown message type", "type", string(msg.Type))
			return fmt.Errorf("Replica got unknown message type: %c", msg.Type)
		}
	}

	// remove all un-recieved files from the cache (aka not synced)
	for k, v := range s.FileCache.data {
		if !v.synced {
			fileToDelete := path.Join(s.FileCache.directory, k)
			err := os.Remove(fileToDelete)
			if err != nil {
				slog.Error("Replica could not delete file", "filename", k, "path", fileToDelete, "error", err)
				return fmt.Errorf("Replica failed to delete file %s: %w", fileToDelete, err)
			} else {
				slog.Debug("Replica deleting file", "filename", k)
			}
		}
	}
	return nil
}

// Reads the file and then sends it over tcp using the Message format
func (s *Syncer) SendFile(filename string) error {
	var err error
	msg := Message{Type: MsgTypeData, FileName: filename}
	msg.Data, err = os.ReadFile(path.Join(s.FileCache.directory, filename))
	if err != nil {
		return errors.Join(err, fmt.Errorf("Could not read file %s", filename))
	}

	msgDataStream := msg.AsBytesBuf()
	totalWritten := 0
	for totalWritten < len(msgDataStream) {
		n, err := s.Conn.Write(msgDataStream[totalWritten:])
		if err != nil {
			return errors.Join(err, fmt.Errorf("Could not write data to tcp connection"))
		}
		totalWritten += n
	}
	slog.Debug("Main sent data message", "type", string(msg.Type), "filename", msg.FileName, "dataSize", len(msg.Data))
	return nil
}

func (s *Syncer) WriteFile(msg Message) error {
	if msg.Type != MsgTypeData {
		slog.Error("Trying to write a message that is not a 'D' type msg", "type", string(msg.Type))
		return fmt.Errorf("invalid message type for WriteFile: expected %c, got %c", MsgTypeData, msg.Type)
	}
	if len(msg.Data) == 0 {
		slog.Error("Data message has no data", "filename", msg.FileName)
		return fmt.Errorf("data message has no data for file %s", msg.FileName)
	}

	err := os.WriteFile(path.Join(s.FileCache.directory, msg.FileName), msg.Data, 0644)
	if err != nil {
		return errors.Join(fmt.Errorf("failed to write %s from msg", msg.FileName), err)
	}
	return nil
}
