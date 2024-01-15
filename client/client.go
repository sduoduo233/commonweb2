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

	utls "github.com/refraction-networking/utls"
)

type client struct {
	up         string
	down       string
	listen     string
	listener   net.Listener
	httpClient http.Client
}

func NewClient(up, down, listen string, useUTLS bool) *client {
	httpClient := http.Client{}

	if useUTLS {
		slog.Info("utls is enabled")

		roller, err := utls.NewRoller()
		if err != nil {
			panic("new utls roller: " + err.Error())
		}

		httpClient.Transport = &http.Transport{
			DialTLSContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				host, _, err := net.SplitHostPort(addr)
				if err != nil {
					return nil, fmt.Errorf("dial tls: split host port: %w", err)
				}

				uConn, err := roller.Dial(network, addr, host)
				if err != nil {
					return nil, fmt.Errorf("dial tls: roller dial: %w", err)
				}

				return uConn, nil
			},
		}
	}
	return &client{
		up:         up,
		down:       down,
		listen:     listen,
		httpClient: httpClient,
	}
}

func (c *client) Start() error {
	slog.Info("listening on", "addr", c.listen)

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
	c.httpClient.CloseIdleConnections()
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

		resp, err := c.httpClient.Do(upRequest)
		if err != nil {
			unwrap := errors.Unwrap(err)
			// ignore the error if it is caused by context cancel
			if !(unwrap != nil && errors.Is(unwrap, context.Canceled)) {
				slog.Error("upload request", "error", err, "sessionId", hex.EncodeToString(sessionId))
			}
			return
		}

		defer resp.Body.Close()

		slog.Debug("upload reqeust", "status", resp.Status, "sessionId", hex.EncodeToString(sessionId))

		io.Copy(io.Discard, resp.Body)
	}()

	// down
	go func() {
		defer cancel()

		resp, err := c.httpClient.Do(downRequest)
		if err != nil {
			unwrap := errors.Unwrap(err)
			// ignore the error if it is caused by context cancel
			if !(unwrap != nil && errors.Is(unwrap, context.Canceled)) {
				slog.Error("upload request", "error", err, "sessionId", hex.EncodeToString(sessionId))
			}
			return
		}

		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			slog.Error("download request", "status", resp.Status, "sessionId", hex.EncodeToString(sessionId))
			return
		}

		slog.Debug("download reqeust", "status", resp.Status, "sessionId", hex.EncodeToString(sessionId))

		_, err = io.Copy(conn, resp.Body)
		if err != nil {
			slog.Debug("read doanload request", "err", err, "sessionId", hex.EncodeToString(sessionId))
		}
	}()

	<-ctx.Done()
	slog.Info("session ends", "sessionId", hex.EncodeToString(sessionId))
	return nil
}
