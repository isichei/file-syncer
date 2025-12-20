package main

import (
	"flag"
	"fmt"
	"log/slog"
	"net"
	"os"

	"github.com/isichei/file-syncer"
	"golang.org/x/sync/errgroup"
)

type CmdArgs struct {
	replica   bool
	addr      string
	directory string
	debug     bool
}

func (c *CmdArgs) Register() {
	flag.BoolVar(&c.replica, "replica", false, "If this is the main filesystem or replica")
	flag.StringVar(&c.addr, "addr", ":8080", "What address should the tcp connection be on")
	flag.StringVar(&c.directory, "directory", "test_data", "Path to the dir to sync the files to")
	flag.BoolVar(&c.debug, "debug", false, "Enable debug logging")
	flag.Parse()
	
	if c.debug {
		slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug})))
	}
	slog.Debug("CmdArgs.Register", "replica", c.replica, "addr", c.addr, "directory", c.directory)
}

func main() {
	cmdArgs := CmdArgs{}
	cmdArgs.Register()

	// Get API key from environment
	apiKey := os.Getenv("FILE_SYNCER_API_KEY")
	if apiKey == "" {
		slog.Error("FILE_SYNCER_API_KEY environment variable is required")
		os.Exit(1)
	}

	var conn net.Conn
	var fc *filesyncer.FileCache

	g := new(errgroup.Group)

	// Set off TCP Connection
	g.Go(func() error {
		var err error
		conn, err = filesyncer.CreateTcpConnection(cmdArgs.addr, apiKey, cmdArgs.replica)
		return err
	})

	// Set of file cache creation
	g.Go(func() error {
		var err error
		fc, err = filesyncer.CreateFileCache(cmdArgs.directory)
		return err
	})

	if err := g.Wait(); err != nil {
		slog.Error("Setup for TCP or File cache failed", "error", err)
		os.Exit(1)
	}

	syncer := filesyncer.Syncer{Replica: cmdArgs.replica, Conn: conn, FileCache: fc}
	var syncerName string
	if syncer.Replica {
		syncerName = "Replica"
	} else {
		syncerName = "Main"
	}

	slog.Info(fmt.Sprintf("Running sender as %s", syncerName), "addr", cmdArgs.addr)
	if err := syncer.Run(); err != nil {
		slog.Error(fmt.Sprintf("%s failed", syncerName),  "error", err)
		os.Exit(1)
	}
}
