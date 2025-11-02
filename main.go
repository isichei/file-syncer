package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	// "log/slog"
	"net"
	// "strconv"
	"strings"
	"syscall"
	"time"
)

type MsgType byte

const (
	MsgTypeCheck     MsgType = 'C'
	MsgTypeMatch     MsgType = 'M'
	MsgTypeData      MsgType = 'D'
	MsgTypeUndefined MsgType = 'U'
)

// Phat struct
type Message struct {
	Type     MsgType
	FileName string
	Data     []byte
	MD5      string
	Match    bool
}

type CmdArgs struct {
	replica bool
	port    string
}

func (c *CmdArgs) Register() {
	flag.BoolVar(&c.replica, "replica", false, "If this is the main filesystem or replica")
	flag.StringVar(&c.port, "port", "8080", "What port should the tcp connection be on")
	flag.Parse()
}

type Syncer struct {
	replica   bool
	conn      net.Conn
	directory string
}

func (s *Syncer) RunAsMain() {
	// TODO use struct values
	defer s.conn.Close()
	fc := createFileCache(s.directory)

	for fileName, fcData := range fc.data {
		match := CheckFile(s.conn, fileName, fcData.md5)

		if !match {
			// Todo make go routine and add wg to see if speed up
			s.SendFile(fileName)
		}
	}
}

func (s *Syncer) RunAsReplica() {
	// TODO use struct values
	defer s.conn.Close()
	panic("Not yet implemented")
}

// TODO
func (s *Syncer) SendFile(filename string) error {
	return nil
}

// Sends check msg and awaits a reply for reciever
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

// parse the format `<MsgType>:<r-filepath>,{...}\n`
// {...} Is then the relevant data depending on the MsgType
func ParseMessage(msgStream []byte) (Message, error) {
	msg := Message{Type: MsgTypeUndefined}
	if len(msgStream) < 4 {
		return msg, errors.New("Message too short")
	}
	split := bytes.SplitAfterN(msgStream[2:], []byte(","), 2)
	msg.FileName = string(split[0][:len(split[0])-1])

	if !bytes.HasSuffix(split[1], []byte("\x00")) {
		return msg, errors.New("Msg does not end in expected null byte")
	}

	switch MsgType(msgStream[0]) {
	case MsgTypeCheck:
		msg.Type = MsgTypeCheck
		msg.MD5 = string(split[1][:len(split[1])-1])

	case MsgTypeMatch:
		msg.Type = MsgTypeMatch
		// Only exect one value after filename in format
		switch split[1][0] {
		case '0':
			msg.Match = false
		case '1':
			msg.Match = true
		default:
			return msg, errors.New("Expected 1 or 0 on MsgCheck response")
		}

	case MsgTypeData:
		msg.Type = MsgTypeData
		msg.Data = append(msg.Data, split[1][:len(split[1])-1]...)

	default:
		return msg, errors.New("Could not parse error bad starting value in msg")
	}
	return msg, nil
}

// TODO
func RunSyncAsReplica() {}

// Q: Maps copy or pass by ref
// func (s *Syncer) Run(conn net.Conn) {
// 	buf := make([]byte, 1024)
// 	defer conn.Close()
// 	fc := createFileCache()
//
// 	if s.replica {
// 		fmt.Println("I am the replica")
// 		for {
// 			// Read the incoming data (main always sends first)
// 			// Todo I think I need a reader / handler channel otherwise it just runs and exists before
// 			// anything is sent/recieved
// 			n, err := conn.Read(buf)
// 			if isConnError(err) {
// 				break
// 			}
// 			data := string(buf[:n])
// 			slog.Info("replica", "recieved", data)
//
// 			// Response
// 			response := fmt.Sprintf("recieved %s", data)
// 			n, err = conn.Write([]byte(response))
// 			if isConnError(err) {
// 				break
// 			}
// 			slog.Info("replica", "sent", response)
// 		}
// 	} else {
// 		fmt.Println("I am the main")
// 		for i := range 5 {
// 			// main initiates
// 			n, err := conn.Write([]byte(strconv.Itoa(i)))
// 			slog.Info("main", "sent", i)
// 			if isConnError(err) {
// 				break
// 			}
//
// 			// Check recieved
// 			n, err = conn.Read(buf)
// 			if isConnError(err) {
// 				break
// 			}
// 			response := string(buf[:n])
// 			slog.Info("main", "recieved", response)
// 			expectedResponse := fmt.Sprintf("recieved %d", i)
// 			if expectedResponse != response {
// 				errMsg := fmt.Sprintf("Did not get expected response. Wanted: `%s`, Got: `%s`", expectedResponse, response)
// 				panic(errMsg)
// 			}
// 		}
// 	}
// }

func isConnError(e error) bool {
	if e == io.EOF {
		fmt.Println("Connection terminated")
		return true
	} else if e != nil {
		panic(fmt.Sprintf("Connection read error %s\n", e.Error()))
	}
	return false
}

// Step one, let's send data between a sender and reciever. Should just send shit via TCP.
func main() {
	cmdArgs := CmdArgs{}
	cmdArgs.Register()
	tcpConn := createTcpConnection(cmdArgs.port, cmdArgs.replica)
	syncer := Syncer{replica: cmdArgs.replica, conn: tcpConn, directory: "test_data/"}
	if cmdArgs.replica {
		fmt.Printf("Running as Replica sender on %s\n", cmdArgs.port)
		syncer.RunAsReplica()
	} else {
		fmt.Printf("Running as Main sender on %s\n", cmdArgs.port)
		syncer.RunAsMain()
	}
	// fileCache := map[string]fileCacheData{}
	//
	// ch := make(chan fileDetails)
	//
	// go getFileDetails("files_folder", ch)
	//
	// for fd := range ch {
	// 	fmt.Printf("%v\n", fd)
	// 	tcpConn.CheckFileMatches(fd.name, fcd.md5)
	// 	if err != nil {
	// 		panic(fmt.Sprintf("Could not check %s. Got error %s", fd.name, e))
	// 	}
	// 	if !isMatch {
	// 		go tcpConn.SendFile(fd.name, fcd.md5, wg)
	// 	}
	// }
	//
	// fmt.Printf("%v\n", fileCache)
	//
	// tcpConn := createTcpConnection(cmdArgs.port, cmdArgs.replica)
	// for fileName, fcd := range fileCache {
	// 	resp, err := tcpConn.CheckFile(fileName, fcd.md5)
	// 	if err != nil {
	// 		panic(fmt.Sprintf("Bad msg %s", e))
	// 	}
	// 	switch resp {
	// 		case true:
	// 			go tcpConn.SendFile(fileName)
	// 		case false:
	// 	}
	// }
	//
	// // syncer := Syncer{cmdArgs.replica}
	// for msg := range syncer.Start(conn) {
	// 	switch parseSyncerMsg(msg) {
	// 	case: # ERROR
	// 	default:
	// 		panic("Unknown Msg")
	// 	}
	// }
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
			// handle error
			panic(err) // Done!
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

func sendData(conn net.Conn, data string) {
	defer conn.Close()
	bytesData := []byte(data)
	length := len(bytesData)
	for {
		n, err := conn.Write(bytesData)
		if err != nil {
			panic(fmt.Sprintf("Could not write data to tcp connection: %s\n", err.Error()))
		}
		if n == length {
			break
		}
	}
}

func handleConnection(conn io.ReadCloser) <-chan string {
	buf := make([]byte, 1024)
	ch := make(chan string)

	go func() {
		defer conn.Close()
		defer close(ch)
		for {
			n, err := conn.Read(buf)
			ch <- string(buf[:n])
			if err == io.EOF {
				fmt.Println("Connection terminated")
				break
			} else if err != nil {
				panic(fmt.Sprintf("Connection read error %s\n", err.Error()))
			}
		}
	}()

	return ch
}
