package main

import (
	"io"
	"fmt"
	"net"
	"flag"
)

// Step one, let's send data between a sender and reciever. Should just send shit via TCP.
func main() {
	fmt.Println("Hi")
	ln, err := net.Listen("tcp", ":8080")
	if err != nil {
		// handle error
		panic(err) // Done!
	}

	conn, err := ln.Accept()
	if err != nil {
		panic(err)
	}
	fmt.Println("Listening to TCP on 8080 babs!")

	for msg := range handleConnection(conn) {
		fmt.Println(msg)
	}
}

func handleConnection(conn io.ReadCloser) <- chan string {
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
