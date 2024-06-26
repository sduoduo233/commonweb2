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

const SESSION_TIMEOUT = 10

type server struct {
	listen   string
	remote   string
	sessions sync.Map
	listener net.Listener
}

type session struct {
	sessionId  string
	up         io.Reader // upload connection
	down       io.Writer // download connection
	ch         chan struct{}
	timeActive int64     // timestamp when a up/down connection is connected
	closeOnce  sync.Once // prevent closing ch multiple times
	sync.Mutex
}

// close s.ch if it is not closed
func (s *session) close() {
	s.closeOnce.Do(func() {
		close(s.ch)
	})
}

// connect to remote and copy data
func (s *session) copy(remote string) {

	conn, err := net.Dial("tcp", remote)
	if err != nil {
		s.close()
		slog.Error("dial remote", "error", err)
		return
	}

	// up conn -> remote
	go func() {
		defer s.close()
		defer conn.Close()
		defer slog.Debug("session closed", "sessionId", s.sessionId, "cause", "up -> remote")

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

			if length == 0 {
				// zero length indicating end of stream
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
		defer s.close()
		defer conn.Close()
		defer slog.Debug("session closed", "sessionId", s.sessionId, "cause", "remote -> down")

		buf := make([]byte, 2048)

		for {
			n, err := conn.Read(buf)
			if err != nil {
				return
			}

			// http chunked transfer
			// https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Transfer-Encoding#directives

			if n == 2048 { // sprintf could be time consuming
				_, err = s.down.Write([]byte("800\r\n"))
			} else {
				_, err = s.down.Write([]byte(fmt.Sprintf("%x\r\n", n)))
			}
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

	<-s.ch
}

func (s *server) Start() error {
	l, err := net.Listen("tcp", s.listen)
	if err != nil {
		return err
	}

	s.listener = l

	// session timeout
	go func() {
		for {
			s.sessions.Range(func(key, value any) bool {
				sess := value.(*session)

				sess.Lock()

				ready := sess.up != nil && sess.down != nil

				if !ready && time.Now().Unix()-sess.timeActive > SESSION_TIMEOUT && sess.timeActive != 0 {
					slog.Warn("session timeout", "sessionId", sess.sessionId)
					sess.close()
					s.sessions.Delete(key)
				}

				sess.Unlock()

				return true
			})
			time.Sleep(time.Second * 5)
		}
	}()

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

			slog.Info("connection ends", "addr", conn.RemoteAddr())
			conn.Close()
		}()
	}
}

func (s *server) Close() error {
	return s.listener.Close()
}

func (*server) writeResponse(code int, conn io.Writer) error {
	status := http.StatusText(code)
	_, err := conn.Write([]byte(fmt.Sprintf("HTTP/1.1 %d %s\r\nContent-Length: 0\r\nConnection: close\r\n\r\n", code, status)))
	return err
}

// find or create a new session
func (s *server) findSession(sessionId string) *session {
	v, _ := s.sessions.LoadOrStore(sessionId, &session{
		sessionId: sessionId,
		ch:        make(chan struct{}),
	})

	return v.(*session)
}

func (s *server) handleConnection(conn net.Conn) error {

	conn.(*net.TCPConn).SetNoDelay(true)

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
		slog.Debug("bad request", "reason", "missing session id", "addr", conn.RemoteAddr())
		return s.writeResponse(http.StatusBadRequest, conn)
	}
	if len(sessionId) > 16 {
		slog.Debug("bad request", "reason", "session id too long", "addr", conn.RemoteAddr())
		return s.writeResponse(http.StatusBadRequest, conn)
	}

	// get session
	sess := s.findSession(sessionId)

	slog.Info("new request", "method", method, "sessionId", sessionId, "addr", conn.RemoteAddr())

	// handle request
	if method == http.MethodGet {
		sess.Lock()

		if sess.down != nil {
			sess.Unlock()
			// donwload connection already exists
			slog.Debug("bad request", "reason", "donwload connection already exists", "addr", conn.RemoteAddr())
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
			slog.Debug("bad request", "reason", "upload connection already exists", "addr", conn.RemoteAddr())
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
	sess.timeActive = time.Now().Unix()

	ready := sess.up != nil && sess.down != nil

	if ready {
		slog.Info("session ready", "sessionId", sess.sessionId)
		go sess.copy(s.remote)
	}

	sess.Unlock()

	<-sess.ch // waiting the session to end
	slog.Debug("upload connection ends", "sessionId", sess.sessionId)

	// remove the session from the map
	s.sessions.Delete(sess.sessionId)

	return s.writeResponse(http.StatusOK, writer)
}

// handle download connection
func (s *server) handleDownload(_ io.Reader, writer io.Writer, sess *session) error {
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
	sess.timeActive = time.Now().Unix()

	ready := sess.up != nil && sess.down != nil

	if ready {
		slog.Info("session ready", "sessionId", sess.sessionId)
		go sess.copy(s.remote)
	}

	sess.Unlock()

	<-sess.ch // waiting the session to end

	// https://www.rfc-editor.org/rfc/rfc9112#section-7.1
	// sending an empty chunk to close the stream
	_, err = writer.Write([]byte("0\r\n\r\n"))
	if err != nil {
		slog.Error("download connection write 0 chunk", "err", err)
	}

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
