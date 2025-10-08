package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"syscall"
	"time"
)

type CmdFlag string

const (
	CmdFlagSender   = "sender"
	CmdFlagReciever = "reciever"
)

// PHAT BOI
type CmdArgs struct {
	reciever   bool
	port       string
	textToSend string
}

func (c *CmdArgs) Register(flagSet *flag.FlagSet, cmd CmdFlag) {
	flagSet.StringVar(&c.port, "port", "8080", "What port should the tcp connection be on")
	if cmd == CmdFlagSender {
		flagSet.StringVar(&c.textToSend, "text", "", "What text to send over the TCP connection")
	}
}

// Step one, let's send data between a sender and reciever. Should just send shit via TCP.
func main() {
	cmdArgs := CmdArgs{reciever: true}
	recieverCmd := flag.NewFlagSet(CmdFlagReciever, flag.ExitOnError)
	cmdArgs.Register(recieverCmd, CmdFlagReciever)

	senderCmd := flag.NewFlagSet(CmdFlagSender, flag.ExitOnError)
	cmdArgs.Register(senderCmd, CmdFlagSender)

	if len(os.Args) < 2 {
		fmt.Println("expected 'listen' or 'send' subcommands")
		os.Exit(1)
	}

	switch os.Args[1] {
	case CmdFlagReciever:
		recieverCmd.Parse(os.Args[2:])
		fmt.Println("Creating tcp connection to recieve...")

	case CmdFlagSender:
		senderCmd.Parse(os.Args[2:])
		cmdArgs.reciever = false
		fmt.Println("Creating tcp connection to send...")

	default:
		fmt.Printf("expected '%s' or '%s' subcommands\n", CmdFlagSender, CmdFlagReciever)
		os.Exit(1)
	}

	conn := createTcpConnection(cmdArgs.port, cmdArgs.reciever)
	if cmdArgs.reciever {
		for msg := range handleConnection(conn) {
			fmt.Println(msg)
		}
	} else {
		sendData(conn, cmdArgs.textToSend)
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
