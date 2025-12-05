package main

import (
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"os"
	"strings"
	"syscall"
	"time"
)

type CmdArgs struct {
	replica   bool
	port      string
	directory string
	debug     bool
}

func (c *CmdArgs) Register() {
	flag.BoolVar(&c.replica, "replica", false, "If this is the main filesystem or replica")
	flag.StringVar(&c.port, "port", "8080", "What port should the tcp connection be on")
	flag.StringVar(&c.directory, "directory", "test_data", "Path to the dir to sync the files to")
	flag.BoolVar(&c.debug, "debug", false, "Enable debug logging")
	flag.Parse()

	if c.debug {
		slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug})))
	}
	slog.Debug("CmdArgs.Register", "replica", c.replica, "port", c.port, "directory", c.directory)
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
