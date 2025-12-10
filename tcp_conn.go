package filesyncer

import (
	"errors"
	"fmt"
	"net"
	"strings"
	"syscall"
	"time"
)

func CreateTcpConnection(port string, listener bool) (net.Conn, error) {
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
