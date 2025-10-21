package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net"
	"strconv"
	"strings"
	"syscall"
	"time"
)

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
	replica bool
}

func (s *Syncer) Start(conn net.Conn) {
	buf := make([]byte, 1024)
	defer conn.Close()
	if s.replica {
		fmt.Println("I am the replica")
		for {
			// Read the incoming data (main always sends first)
			// Todo I think I need a reader / handler channel otherwise it just runs and exists before
			// anything is sent/recieved
			n, err := conn.Read(buf)
			if isConnError(err) {
				break
			}
			data := string(buf[:n])
			slog.Info("replica", "recieved", data)

			// Response
			response := fmt.Sprintf("recieved %s", data)
			n, err = conn.Write([]byte(response))
			if isConnError(err) {
				break
			}
			slog.Info("replica", "sent", response)
		}
	} else {
		fmt.Println("I am the main")
		for i := range 5 {
			// main initiates
			n, err := conn.Write([]byte(strconv.Itoa(i)))
			slog.Info("main", "sent", i)
			if isConnError(err) {
				break
			}

			// Check recieved
			n, err = conn.Read(buf)
			if isConnError(err) {
				break
			}
			response := string(buf[:n])
			slog.Info("main", "recieved", response)
			expectedResponse := fmt.Sprintf("recieved %d", i)
			if expectedResponse != response {
				errMsg := fmt.Sprintf("Did not get expected response. Wanted: `%s`, Got: `%s`", expectedResponse, response)
				panic(errMsg)
			}
		}
	}
}

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
	if cmdArgs.replica {
		fmt.Printf("Running as Replica sender on %s\n", cmdArgs.port)
	} else {
		fmt.Printf("Running as Main sender on %s\n", cmdArgs.port)
	}
	
	fileCache := map[string]fileCacheData{}

	ch := make(chan fileDetails)
	go getFileDetails("files_folder", ch)
	
	for fd := range ch {
		fmt.Printf("%v\n", fd)
		fileCache[fd.name] = fileCacheData{md5: fd.md5, synced: false}
	}

	fmt.Printf("%v\n", fileCache)
	
	// conn := createTcpConnection(cmdArgs.port, cmdArgs.replica)
	// syncer := Syncer{cmdArgs.replica}
	// syncer.Start(conn)
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
