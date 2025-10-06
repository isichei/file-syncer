package main

import (
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
)

type CmdFlag string

const (
	CmdFlagSender   = "sender"
	CmdFlagReciever = "reciever"
)

// Step one, let's send data between a sender and reciever. Should just send shit via TCP.
func main() {
	recieverCmd := flag.NewFlagSet(CmdFlagReciever, flag.ExitOnError)
	recieverPort := recieverCmd.String("port", "8080", "What port should the tcp listener be listening on")

	// senderCmd := flag.NewFlagSet(CmdFlagSender, flag.ExitOnError)

	if len(os.Args) < 2 {
		fmt.Println("expected 'listen' or 'send' subcommands")
		os.Exit(1)
	}

	switch os.Args[1] {
	case CmdFlagReciever:
		recieverCmd.Parse(os.Args[2:])
		fmt.Println("Creating tcp connection...")
		conn := createTcpConnection(*recieverPort)
		for msg := range handleConnection(conn) {
			fmt.Println(msg)
		}
	case CmdFlagSender:
		fmt.Printf("Not yet implemeted 🖕\n")
		os.Exit(1)
	default:
		fmt.Printf("expected '%s' or '%s' subcommands\n", CmdFlagSender, CmdFlagReciever)
		os.Exit(1)
	}
}

func createTcpConnection(port string) net.Conn {
	if !strings.HasPrefix(port, ":") {
		port = ":" + port
	}
	ln, err := net.Listen("tcp", port)
	if err != nil {
		// handle error
		panic(err) // Done!
	}

	conn, err := ln.Accept()
	if err != nil {
		panic(err)
	}
	fmt.Printf("Listening to TCP on %s babs!\n", port)
	return conn
}

func handleConnection(conn io.ReadCloser) <-chan string {
	buf := make([]byte, 1024)
	ch := make(chan string)

	go func() {
		defer conn.Close()
		defer close(ch)
		for {
			n, err := conn.Read(buf)
			if err != nil {
				panic(fmt.Sprintf("Connection read error %s\n", err.Error()))
			}
			fmt.Printf("Read n: %d\n", n)
			ch <- string(buf[:n])
		}
	}()

	return ch
}
