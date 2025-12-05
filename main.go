package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"path"
	"strings"
	"syscall"
	"time"
)

type CmdArgs struct {
	replica   bool
	port      string
	directory string
}

func (c *CmdArgs) Register() {
	flag.BoolVar(&c.replica, "replica", false, "If this is the main filesystem or replica")
	flag.StringVar(&c.port, "port", "8080", "What port should the tcp connection be on")
	flag.StringVar(&c.directory, "directory", "test_data", "Path to the dir to sync the files to")
	flag.Parse()

	slog.Debug("CmdArgs.Register", "replica", c.replica, "port", c.port, "directory", c.directory)
}

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
		_, err := s.conn.Write(checkMsg.AsBytesBuf())  // I am going to assume the msg is so small I don't need to check n
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
		if msg.Type != MsgTypeMatch {
			panic("Unexpected msg type from replica on check request")
		}
		if !msg.Match {
			s.SendFile(fileName)
		}
	}

	s.SendFinish()
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
			break OUTER

		case MsgTypeCheck:
			responseMessage := Message{Type: MsgTypeMatch, FileName: msg.FileName}
			fileData, ok := s.fc.data[msg.FileName]
			responseMessage.Match = !ok || fileData.md5 != msg.MD5
			respErr := s.SendMessage(responseMessage)
			if respErr != nil {
				panic("Failed to send the response message for the md5 file check")
			}

		case MsgTypeMatch:
			panic("I am the replica I shouldn't be being sent Match messages!")

		case MsgTypeData:
			s.WriteFile(msg)

		default:
			panic(fmt.Sprintf("For loop recieving msgs over tcp based on msg type got an unknown message type: %c", msg.Type))
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
	return nil
}

// Sends check msg and awaits a reply for reciever
// TODO Need to change this to send a Message, requires a re-read. I got the Replica interfaces / methods
// good at this point. So just need to align the Main entrypoint method to match the same style
func CheckFile(conn net.Conn, fileName string, md5 string) bool {
	buf := make([]byte, 128)

	// Send Check message
	msg := fmt.Appendf([]byte{}, "check:%s,%s", fileName, md5)
	n, err := conn.Write(msg)
	if err != nil {
		panic(fmt.Sprintf("Could not send message for fileCheck %s\n", fileName))
	}

	// Check recieved
	n, err = conn.Read(buf)
	if err != nil {
		panic(fmt.Sprintf("Could not recieve message for fileCheck: %s\n", fileName))
	}

	message, err := ParseMessage(buf[:n])
	if err != nil {
		panic(fmt.Sprintf("Could not parse recieved message for fileCheck: %s\n", fileName))
	}
	return message.Match
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

// Step one, let's send data between a sender and reciever. Should just send shit via TCP.
func main() {
	cmdArgs := CmdArgs{}
	cmdArgs.Register()

	connChannel := make(chan net.Conn)
	fcChannel := make(chan *fileCache)

	go func() {
		connChannel <- createTcpConnection(cmdArgs.port, cmdArgs.replica)
	}()

	go func() {
		fcChannel <- createFileCache(cmdArgs.directory)
	}()

	syncer := Syncer{replica: cmdArgs.replica, conn: <-connChannel, fc: <-fcChannel}
	if cmdArgs.replica {
		fmt.Printf("Running as Replica sender on %s\n", cmdArgs.port)
		syncer.RunAsReplica()
	} else {
		fmt.Printf("Running as Main sender on %s\n", cmdArgs.port)
		syncer.RunAsMain()
	}
}

func createTcpConnection(port string, listener bool) net.Conn {
	var conn net.Conn
	var err error

	if !strings.HasPrefix(port, ":") {
		port = ":" + port
	}
	if listener {
		ln, err := net.Listen("tcp", port)
		if err != nil {
			panic(err)
		}
		conn, err = ln.Accept()

	} else {
		for retry := range 4 {
			conn, err = net.Dial("tcp", port)

			// Done
			if err == nil {
				break
			}

			// Every other error
			if !errors.Is(err, syscall.ECONNREFUSED) {
				panic(err)
			}

			// Failed to connect
			if retry == 3 {
				panic(errors.Join(errors.New("Retried connection 3 times but failed"), err))
			}

			// Retry connection
			fmt.Println("Retrying...")
			time.Sleep(time.Second)
		}
	}

	if listener {
		fmt.Printf("TCP Listening on %s babs!\n", port)
	} else {
		fmt.Printf("TCP Sending on %s babs!\n", port)
	}
	return conn
}

