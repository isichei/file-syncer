package main

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
	replica bool
	conn    io.ReadWriteCloser
	fc      *fileCache
}

func (s *Syncer) SendMessage(msg Message) error {
	msgBuf := msg.AsBytesBuf()
	totalWritten := 0
	for totalWritten < len(msgBuf) {
		n, err := s.conn.Write(msgBuf[totalWritten:])
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

func (s *Syncer) RunAsMain() {
	defer s.conn.Close()
	reader := bufio.NewReader(s.conn)

	for fileName, fcData := range s.fc.data {
		// Send out msg to reciver to replica
		checkMsg := Message{Type: MsgTypeCheck, FileName: fileName, MD5: fcData.md5}
		_, err := s.conn.Write(checkMsg.AsBytesBuf()) // I am going to assume the msg is so small I don't need to check n
		slog.Debug("Main check message sent", "type", string(checkMsg.Type), "filename", checkMsg.FileName, "md5", checkMsg.MD5)

		if err != nil {
			panic(fmt.Sprintf("Could not send message for fileCheck %s\n", fileName))
		}

		// Check response from replica then send if non matching
		msgStream, err := reader.ReadBytes('\x00')
		if err != nil {
			panic("Could not read incomming data from replica on check request")
		}
		msg, err := ParseMessage(msgStream)
		if err != nil {
			panic(fmt.Sprintf("Could parse message from msg stream from replica on check request - %s", err))
		}
		slog.Debug("Main received match message", "type", string(msg.Type), "filename", msg.FileName, "match", msg.Match)
		if msg.Type != MsgTypeMatch {
			panic("Unexpected msg type from replica on check request")
		}
		if !msg.Match {
			s.SendFile(fileName)
		}
	}

	err := s.SendFinish()
	if err != nil {
		panic("Failed to send finish msg")
	}
	slog.Debug("Main sent finish message", "type", string(MsgTypeFinish))
}

func (s *Syncer) RunAsReplica() {
	defer s.conn.Close()

	reader := bufio.NewReader(s.conn)

	// Not sure how I feel about labels...
OUTER:
	for {
		msgStream, err := reader.ReadBytes('\x00')
		if err != nil {
			panic("Could not read incomming data")
		}

		msg, err := ParseMessage(msgStream)
		if err != nil {
			panic("Could parse message from msg stream")
		}

		switch msg.Type {
		case MsgTypeFinish:
			slog.Debug("Replica received finish message", "type", string(msg.Type))
			break OUTER

		case MsgTypeCheck:
			slog.Debug("Replica received check message", "type", string(msg.Type), "filename", msg.FileName, "md5", msg.MD5)
			responseMessage := Message{Type: MsgTypeMatch, FileName: msg.FileName}
			fileData, ok := s.fc.data[msg.FileName]
			responseMessage.Match = ok && fileData.md5 == msg.MD5
			respErr := s.SendMessage(responseMessage)
			slog.Debug("Replica sent match message", "type", string(responseMessage.Type), "filename", responseMessage.FileName, "match", responseMessage.Match)
			if respErr != nil {
				panic("Failed to send the response message for the md5 file check")
			}

			// Update the file cache
			if responseMessage.Match {
				fileData.synced = true
				s.fc.data[msg.FileName] = fileData
			}

		case MsgTypeMatch:
			panic("I am the replica I shouldn't be being sent Match messages!")

		case MsgTypeData:
			slog.Debug("Replica received data message", "type", string(msg.Type), "filename", msg.FileName, "dataSize", len(msg.Data))
			s.WriteFile(msg)
			fileData, ok := s.fc.data[msg.FileName]
			if ok {
				fileData.synced = true
				s.fc.data[msg.FileName] = fileData
			} else {
				// Not the best idea to just set md5 to empty but only using it for final check on synced so ok for now
				s.fc.data[msg.FileName] = fileCacheData{md5: "", synced: true}
			}

		default:
			panic(fmt.Sprintf("For loop recieving msgs over tcp based on msg type got an unknown message type: %c", msg.Type))
		}
	}

	// remove all un-recieved files from the cache (aka not synced)
	for k, v := range s.fc.data {
		if !v.synced {
			fileToDelete := path.Join(s.fc.directory, k)
			err := os.Remove(fileToDelete)
			if err != nil {
				panic(fmt.Sprintf("Could not delete %s. Error: %s", fileToDelete, err))
			} else {
				slog.Debug("Replica deleting file", "filename", k)
			}
		}
	}
}

// Reads the file and then sends it over tcp using the Message format
func (s *Syncer) SendFile(filename string) error {
	var err error
	msg := Message{Type: MsgTypeData, FileName: filename}
	msg.Data, err = os.ReadFile(path.Join(s.fc.directory, filename))
	if err != nil {
		return errors.Join(err, fmt.Errorf("Could not read file %s", filename))
	}

	msgDataStream := msg.AsBytesBuf()
	totalWritten := 0
	for totalWritten < len(msgDataStream) {
		n, err := s.conn.Write(msgDataStream[totalWritten:])
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
		panic("Trying to write a message that is not a 'D' type msg")
	}
	if len(msg.Data) == 0 {
		panic("Data message has no data")
	}

	err := os.WriteFile(path.Join(s.fc.directory, msg.FileName), msg.Data, 0644)
	if err != nil {
		return errors.Join(fmt.Errorf("Failed to write %s from msg", msg.FileName), err)
	}
	return nil
}
