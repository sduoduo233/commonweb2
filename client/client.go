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
	"time"

	utls "github.com/refraction-networking/utls"
)

type client struct {
	up         string
	down       string
	listen     string
	listener   net.Listener
	httpClient http.Client
}

func NewClient(up, down, listen string, useUTLS, skipVerify bool) *client {
	httpClient := http.Client{
		Transport: http.DefaultTransport.(*http.Transport).Clone(),
	}

	if useUTLS {
		slog.Info("using utls")

		httpClient.Transport.(*http.Transport).DialTLSContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
			tcpConn, err := net.DialTimeout(network, addr, time.Second*30)
			if err != nil {
				return nil, fmt.Errorf("dial tls: dial tcp: %w", err)
			}

			host, _, err := net.SplitHostPort(addr)
			if err != nil {
				return nil, fmt.Errorf("dial tls: split host port: %s: %w", addr, err)
			}

			uConn := utls.UClient(tcpConn, &utls.Config{
				ServerName:         host,
				InsecureSkipVerify: skipVerify,
			}, utls.HelloChrome_Auto)

			err = uConn.HandshakeContext(ctx)
			if err != nil {
				return nil, fmt.Errorf("dial tls: handshake: %w", err)
			}

			return uConn, nil
		}
	} else {
		slog.Info("using crypto/tls")

		httpClient.Transport.(*http.Transport).TLSClientConfig.InsecureSkipVerify = skipVerify
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
	sessionIdHex := hex.EncodeToString(sessionId)

	slog.Info("new session", "sessionId", sessionIdHex, "conn", conn.RemoteAddr())

	ctx, cancel := context.WithCancel(context.Background())

	upRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, c.up, conn)
	if err != nil {
		cancel()
		return fmt.Errorf("new upload request: %w", err)
	}
	upRequest.Header.Add("X-Session-Id", sessionIdHex)

	downRequest, err := http.NewRequestWithContext(ctx, http.MethodGet, c.down, nil)
	if err != nil {
		cancel()
		return fmt.Errorf("new download request: %w", err)
	}
	downRequest.Header.Add("X-Session-Id", sessionIdHex)

	// up
	go func() {
		defer cancel()
		defer slog.Debug("context cancel by up", "sessionId", sessionIdHex)

		resp, err := c.httpClient.Do(upRequest)
		if err != nil {
			unwrap := errors.Unwrap(err)
			// ignore the error if it is caused by context cancel
			if !(unwrap != nil && errors.Is(unwrap, context.Canceled)) {
				slog.Error("upload request", "error", err, "sessionId", sessionIdHex)
			}
			return
		}

		defer resp.Body.Close()

		slog.Debug("upload reqeust", "status", resp.Status, "sessionId", sessionIdHex)

		io.Copy(io.Discard, resp.Body)
	}()

	// down
	go func() {
		defer cancel()
		defer slog.Debug("context cancel by down", "sessionId", sessionIdHex)

		resp, err := c.httpClient.Do(downRequest)
		if err != nil {
			unwrap := errors.Unwrap(err)
			// ignore the error if it is caused by context cancel
			if !(unwrap != nil && errors.Is(unwrap, context.Canceled)) {
				slog.Error("upload request", "error", err, "sessionId", sessionIdHex)
			}
			return
		}

		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			slog.Error("download request", "status", resp.Status, "sessionId", sessionIdHex)
			return
		}

		slog.Debug("download reqeust", "status", resp.Status, "sessionId", sessionIdHex)

		_, err = io.Copy(conn, resp.Body)
		if err != nil {
			slog.Debug("read doanload request", "err", err, "sessionId", sessionIdHex, "addr", conn.RemoteAddr())
		}
	}()

	<-ctx.Done()

	slog.Info("session ends", "sessionId", sessionIdHex)
	return nil
}
