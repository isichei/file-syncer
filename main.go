package main

import (
	"errors"
	"flag"
	"fmt"
	"golang.org/x/sync/errgroup"
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

func createTcpConnection(port string, listener bool) (net.Conn, error) {
	var conn net.Conn
	var err error

	if !strings.HasPrefix(port, ":") {
		port = ":" + port
	}
	if listener {
		ln, err := net.Listen("tcp", port)
		if err != nil {
			return nil, fmt.Errorf("Failed to listen on %s: %w", port, err)
		}
		conn, err = ln.Accept()
		if err != nil {
			return nil, fmt.Errorf("Failed to accept connection: %w", err)
		}
	} else {
		for retry := range 4 {
			conn, err = net.Dial("tcp", port)

			// Done
			if err == nil {
				break
			}

			// Every other error
			if !errors.Is(err, syscall.ECONNREFUSED) {
				return nil, fmt.Errorf("Failed to dial %s: %w", port, err)
			}

			// Failed to connect
			if retry == 3 {
				return nil, errors.Join(errors.New("Retried connection 3 times but failed"), err)
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
	return conn, nil
}

func main() {
	cmdArgs := CmdArgs{}
	cmdArgs.Register()
	var g errgroup.Group
	var conn net.Conn
	var fc *fileCache

	// Set off TCP Connection
	g.Go(func() error {
		var err error
		conn, err = createTcpConnection(cmdArgs.port, cmdArgs.replica)
		return err
	})

	// Set of file cache creation
	g.Go(func() error {
		var err error
		fc, err = createFileCache(cmdArgs.directory)
		return err
	})

	if err := g.Wait(); err != nil {
		slog.Error("Setup for TCP or File cache failed", "error", err)
		os.Exit(1)
	}

	syncer := Syncer{replica: cmdArgs.replica, conn: conn, fc: fc}
	if cmdArgs.replica {
		slog.Info("Running sender as Replica", "port", cmdArgs.port)
		if err := syncer.RunAsReplica(); err != nil {
			slog.Error("RunAsReplica failed", "error", err)
			os.Exit(1)
		}
	} else {
		slog.Info("Running sender as Main", "port", cmdArgs.port)
		if err := syncer.RunAsMain(); err != nil {
			slog.Error("RunAsMain failed", "error", err)
			os.Exit(1)
		}
	}
}
