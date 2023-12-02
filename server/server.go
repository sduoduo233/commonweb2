package server

import (
	"bufio"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/textproto"
	"strconv"
	"strings"
	"sync"
	"time"
)

type server struct {
	listen   string
	remote   string
	sessions sync.Map
}

type session struct {
	sessionId string
	up        io.Reader // upload connection
	down      io.Writer // download connection
	ch        chan struct{}
	sync.Mutex
}

// connect to remote and copy data
func (s *session) copy(remote string) {
	defer close(s.ch)

	conn, err := net.Dial("tcp", remote)
	if err != nil {
		slog.Error("dial remote", "error", err)
		return
	}

	var wg sync.WaitGroup
	wg.Add(2)

	// up conn -> remote
	go func() {
		defer wg.Done()
		defer conn.Close()

		for {

			// http chunked transfer
			reader := s.up.(*bufio.Reader)
			line, _, err := reader.ReadLine()
			if err != nil {
				return
			}
			length, err := strconv.ParseUint(string(line), 16, 32)
			if err != nil {
				return
			}

			_, err = io.CopyN(conn, reader, int64(length))
			if err != nil {
				return
			}

			_, _, err = reader.ReadLine()
			if err != nil {
				return
			}
		}
	}()

	// remote -> down conn
	go func() {
		defer wg.Done()
		defer conn.Close()

		for {
			buf := make([]byte, 1024)
			n, err := conn.Read(buf)
			if err != nil {
				return
			}

			// http chunked transfer
			// https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Transfer-Encoding#directives

			_, err = s.down.Write([]byte(fmt.Sprintf("%x\r\n", n)))
			if err != nil {
				return
			}
			_, err = s.down.Write(buf[:n])
			if err != nil {
				return
			}
			_, err = s.down.Write([]byte("\r\n"))
			if err != nil {
				return
			}
		}

	}()

	wg.Wait()
}

func (s *server) Start() error {
	l, err := net.Listen("tcp", s.listen)
	if err != nil {
		return err
	}

	for {
		conn, err := l.Accept()
		if err != nil {
			return fmt.Errorf("accept: %w", err)
		}

		slog.Debug("new connection", "addr", conn.RemoteAddr())

		go func() {
			err := s.handleConnection(conn)
			if err != nil {
				slog.Error("handle connection", "error", err)
			}

			slog.Debug("connection ends", "addr", conn.RemoteAddr())
			conn.Close()
		}()
	}
}

func (*server) writeResponse(code int, conn io.Writer) error {
	status := http.StatusText(code)
	_, err := conn.Write([]byte(fmt.Sprintf("HTTP/1.1 %d %s\r\nContent-Length: 0\r\nConnection: close\r\n\r\n", code, status)))
	return err
}

// find or create a new session
func (s *server) findSession(sessionId string) *session {
	sess, ok := s.sessions.Load(sessionId)
	if ok {
		return sess.(*session)
	}

	sess = &session{
		sessionId: sessionId,
		ch:        make(chan struct{}),
	}
	s.sessions.Store(sessionId, sess)
	return sess.(*session)
}

func (s *server) handleConnection(conn net.Conn) error {
	bufReader := bufio.NewReader(conn)
	reader := textproto.NewReader(bufReader)
	line, err := reader.ReadLine()
	if err != nil {
		return fmt.Errorf("readline: %w", err)
	}

	// read http request
	splites := strings.SplitN(line, " ", 3) // example: GET /file.txt HTTP/1.1
	if len(splites) != 3 {
		return s.writeResponse(http.StatusBadRequest, conn)
	}
	method, _, version := splites[0], splites[1], splites[2]
	if version != "HTTP/1.1" && version != "HTTP/1.0" {
		return s.writeResponse(http.StatusHTTPVersionNotSupported, conn)
	}

	if method != http.MethodGet && method != http.MethodPost {
		return s.writeResponse(http.StatusMethodNotAllowed, conn)
	}

	headers, err := reader.ReadMIMEHeader()
	if err != nil {
		return s.writeResponse(http.StatusBadRequest, conn)
	}

	sessionId := headers.Get("X-Session-Id")
	if sessionId == "" {
		return s.writeResponse(http.StatusBadRequest, conn)
	}
	if len(sessionId) > 16 {
		return s.writeResponse(http.StatusBadRequest, conn)
	}

	// get session
	sess := s.findSession(sessionId)

	slog.Info("new request", "method", method, "sessionId", sessionId, "addr", conn.RemoteAddr())

	// session timeout
	go func() {
		time.Sleep(time.Second * 10)
		sess.Lock()
		ready := sess.up != nil && sess.down != nil
		sess.Unlock()
		if !ready {
			slog.Warn("session timeout", "sessionId", sess.sessionId)
			close(sess.ch)
		}
	}()

	// handle request
	if method == http.MethodGet {
		sess.Lock()

		if sess.down != nil {
			sess.Unlock()
			// donwload connection already exists
			return s.writeResponse(http.StatusBadRequest, conn)
		}
		sess.Unlock()

		return s.handleDownload(bufReader, conn, sess)
	}

	if method == http.MethodPost {
		sess.Lock()

		if sess.up != nil {
			sess.Unlock()
			// upload connection already exists
			return s.writeResponse(http.StatusBadRequest, conn)
		}
		sess.Unlock()

		return s.handleUpload(bufReader, conn, sess)
	}

	panic("impossible to reach here")
}

// handle upload connection
func (s *server) handleUpload(reader io.Reader, writer io.Writer, sess *session) error {
	sess.Lock()
	sess.up = reader
	sess.Unlock()

	sess.Lock()
	ready := sess.up != nil && sess.down != nil
	sess.Unlock()

	if ready {
		slog.Info("session ready", "sessionId", sess.sessionId)
		go sess.copy(s.remote)
	}

	<-sess.ch // waiting the session to end
	slog.Debug("upload connection ends", "sessionId", sess.sessionId)

	// remove the session from the map
	s.sessions.Delete(sess.sessionId)

	return s.writeResponse(http.StatusOK, writer)
}

// handle download connection
func (s *server) handleDownload(reader io.Reader, writer io.Writer, sess *session) error {
	resp := "HTTP/1.1 200 OK\r\n"
	resp += "Transfer-Encoding: chunked\r\n"
	resp += "Content-Type: application/octet-stream\r\n"
	resp += "Connection: close\r\n"
	resp += "\r\n"
	_, err := writer.Write([]byte(resp))
	if err != nil {
		return err
	}

	sess.Lock()
	sess.down = writer
	sess.Unlock()

	sess.Lock()
	ready := sess.up != nil && sess.down != nil
	sess.Unlock()

	if ready {
		slog.Info("session ready", "sessionId", sess.sessionId)
		go sess.copy(s.remote)
	}

	<-sess.ch // waiting the session to end
	slog.Debug("download connection ends", "sessionId", sess.sessionId)

	// remote the session from the map
	s.sessions.Delete(sess.sessionId)

	return nil
}

func NewServer(listen string, remote string) *server {
	return &server{
		sessions: sync.Map{},
		listen:   listen,
		remote:   remote,
	}
}
