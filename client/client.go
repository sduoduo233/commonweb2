package client

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
)

type client struct {
	up       string
	down     string
	listen   string
	listener net.Listener
}

func NewClient(up, down, listen string) *client {
	return &client{
		up:     up,
		down:   down,
		listen: listen,
	}
}

func (c *client) Start() error {
	l, err := net.Listen("tcp", c.listen)
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}

	c.listener = l

	for {
		conn, err := l.Accept()
		if err != nil {
			return fmt.Errorf("accept: %w", err)
		}
		slog.Debug("new connection", "addr", conn.RemoteAddr())

		go func() {
			err := c.handleConnection(conn)
			if err != nil {
				slog.Error("handle connection", "error", err)
			}
			conn.Close()
		}()
	}
}

func (c *client) Close() error {
	return c.listener.Close()
}

func (c *client) handleConnection(conn net.Conn) error {
	// generate session id
	sessionId := make([]byte, 8)
	_, err := rand.Read(sessionId)
	if err != nil {
		panic("rand: " + err.Error())
	}

	slog.Info("new session", "sessionId", hex.EncodeToString(sessionId), "conn", conn.RemoteAddr())

	ctx, cancel := context.WithCancel(context.Background())

	upRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, c.up, conn)
	if err != nil {
		cancel()
		return fmt.Errorf("new upload request: %w", err)
	}
	upRequest.Header.Add("X-Session-Id", hex.EncodeToString(sessionId))

	downRequest, err := http.NewRequestWithContext(ctx, http.MethodGet, c.down, nil)
	if err != nil {
		cancel()
		return fmt.Errorf("new download request: %w", err)
	}
	downRequest.Header.Add("X-Session-Id", hex.EncodeToString(sessionId))

	// up
	go func() {
		defer cancel()

		resp, err := http.DefaultClient.Do(upRequest)
		if err != nil {
			unwrap := errors.Unwrap(err)
			// ignore the error if it is caused by context cancel
			if !(unwrap != nil && errors.Is(unwrap, context.Canceled)) {
				slog.Error("upload request", "error", err)
			}
			return
		}

		slog.Debug("upload reqeust", "status", resp.Status)
	}()

	// down
	go func() {
		defer cancel()

		resp, err := http.DefaultClient.Do(downRequest)
		if err != nil {
			unwrap := errors.Unwrap(err)
			// ignore the error if it is caused by context cancel
			if !(unwrap != nil && errors.Is(unwrap, context.Canceled)) {
				slog.Error("upload request", "error", err)
			}
			return
		}

		if resp.StatusCode != http.StatusOK {
			slog.Error("download request", "status", resp.Status)
			return
		}

		slog.Debug("download reqeust", "status", resp.Status)

		io.Copy(conn, resp.Body)
	}()

	<-ctx.Done()
	slog.Info("session ends", "sessionId", hex.EncodeToString(sessionId))
	return nil
}
