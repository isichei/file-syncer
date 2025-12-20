package filesyncer

import (
	"bufio"
	"crypto/subtle"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"strings"
	"syscall"
	"time"
)

var ErrAuthFailed = errors.New("Authentication failed")

func CreateTcpConnection(port string, apiKey string, replica bool) (net.Conn, error) {
	if replica {
		return CreateReplicaListenerConn(port, apiKey)
	}
	return CreateMainSenderConn(port, apiKey)
}

// CreateMainSenderConn dials the replica, sends auth, and waits for acceptance
func CreateMainSenderConn(port string, apiKey string) (net.Conn, error) {
	if !strings.HasPrefix(port, ":") {
		port = ":" + port
	}

	var conn net.Conn
	var err error

	// Retry connection logic
	for retry := range 4 {
		conn, err = net.Dial("tcp", port)

		if err == nil {
			break
		}

		if !errors.Is(err, syscall.ECONNREFUSED) {
			return nil, fmt.Errorf("failed to dial %s: %w", port, err)
		}

		if retry == 3 {
			return nil, errors.Join(errors.New("retried connection 3 times but failed"), err)
		}

		slog.Info("Retrying connection in 1 sec...")
		time.Sleep(time.Second)
	}

	// Send auth message
	authMsg := Message{Type: MsgTypeAuth, Data: []byte(apiKey)}
	_, err = conn.Write(authMsg.AsBytesBuf())
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to send auth message: %w", err)
	}

	// Read response
	reader := bufio.NewReader(conn)
	msgStream, err := reader.ReadBytes('\x00')
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to read auth response: %w", err)
	}

	msg, err := ParseMessage(msgStream)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to parse auth response: %w", err)
	}

	if msg.Type != MsgTypeAuthOK {
		conn.Close()
		return nil, ErrAuthFailed
	}

	slog.Info("TCP connection authenticated", "port", port)
	return conn, nil
}

// CreateReplicaListenerConn listens on port, accepts connections in a loop,
// and returns the first connection that authenticates successfully
func CreateReplicaListenerConn(port string, validAPIKey string) (net.Conn, error) {
	if !strings.HasPrefix(port, ":") {
		port = ":" + port
	}

	ln, err := net.Listen("tcp", port)
	if err != nil {
		return nil, fmt.Errorf("failed to listen on %s: %w", port, err)
	}
	defer ln.Close()

	slog.Info("TCP Listening for authenticated connection", "port", port)

	AcceptConnErrCounter := 0
	for {
		conn, err := ln.Accept()
		if err != nil {
			slog.Warn("Failed to accept connection", "error", err)
			if AcceptConnErrCounter >= 5 {
				conn.Close()
				return nil, err
			}
			AcceptConnErrCounter += 1
			continue
		}
		conn, err = AuthenticateListenerConnection(conn, validAPIKey)
		if err != nil {
			conn.Close()
			return nil, err
		}
		return conn, nil
	}
}

// To be used if you want to create a listener and manage the the connection creation
// yourself but then still want to authenticate it.
func AuthenticateListenerConnection(conn net.Conn, validAPIKey string) (net.Conn, error) {
	// Set 5 second deadline for auth
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))

	reader := bufio.NewReader(conn)
	msgStream, err := reader.ReadBytes('\x00')
	if err != nil {
		slog.Warn("Failed to read auth message", "remote", conn.RemoteAddr(), "error", err)
		return conn, errors.Join(ErrAuthFailed, errors.New("Failed to read auth message"))
	}

	msg, err := ParseMessage(msgStream)
	if err != nil {
		slog.Warn("Failed to parse auth message", "remote", conn.RemoteAddr(), "error", err)
		sendAuthFail(conn)
		return conn, errors.Join(ErrAuthFailed, errors.New("Failed to parse auth message"))
	}

	if msg.Type != MsgTypeAuth {
		slog.Warn("Expected auth message", "remote", conn.RemoteAddr(), "got", string(msg.Type))
		sendAuthFail(conn)
		return conn, errors.Join(ErrAuthFailed, errors.New("Recieved non auth message type"))
	}

	// Check auth
	if subtle.ConstantTimeCompare(msg.Data, []byte(validAPIKey)) != 1 {
		slog.Warn("Auth failed: invalid API key", "remote", conn.RemoteAddr())
		sendAuthFail(conn)
		return conn, errors.Join(ErrAuthFailed, errors.New("Recieved non auth message type"))
	}

	// Clear deadline for normal operation
	conn.SetReadDeadline(time.Time{})

	// Send success
	authOK := Message{Type: MsgTypeAuthOK}
	_, err = conn.Write(authOK.AsBytesBuf())
	if err != nil {
		slog.Warn("Failed to send auth OK", "remote", conn.RemoteAddr(), "error", err)
		return conn, errors.Join(ErrAuthFailed, errors.New("Failed to send auth OK message"))
	}

	slog.Info("Client authenticated", "remote", conn.RemoteAddr())
	return conn, nil
}

func sendAuthFail(conn net.Conn) {
	authFail := Message{Type: MsgTypeAuthFail}
	conn.Write(authFail.AsBytesBuf())
}
