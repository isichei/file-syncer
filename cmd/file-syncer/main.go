package main

import (
	"flag"
	"github.com/isichei/file-syncer"
	"golang.org/x/sync/errgroup"
	"log/slog"
	"net"
	"os"
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

func main() {
	cmdArgs := CmdArgs{}
	cmdArgs.Register()
	var conn net.Conn
	var fc *filesyncer.FileCache

	g := new(errgroup.Group)

	// Set off TCP Connection
	g.Go(func() error {
		var err error
		conn, err = filesyncer.CreateTcpConnection(cmdArgs.port, cmdArgs.replica)
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
