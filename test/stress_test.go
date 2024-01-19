package test

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"io"
	"net"
	"sync"
	"testing"
	"time"
)

// TestStress simulates concurrent connections
func TestStress(t *testing.T) {
	ch := make(chan any)
	defer close(ch)

	n := 1000 // number of connections

	// test data
	upload := randomBytes(1024 * 1024)
	download := randomBytes(1024 * 1024)
	uploadHash := sha256sum(upload)
	downloadHash := sha256sum(download)

	setupCommonweb(t, ch)

	var wg sync.WaitGroup
	wg.Add(1)

	// remote
	go func() {
		defer wg.Done()

		l, err := net.Listen("tcp", "127.0.0.1:30020")
		if err != nil {
			t.Log("remote listen", err)
			t.Fail()
			return
		}

		defer l.Close()

		for i := 0; i < n; i++ {
			t.Log("accept", i)
			conn, err := l.Accept()
			if err != nil {
				t.Log("remote accept", err)
				t.Fail()
				break
			}

			conn.(*net.TCPConn).SetNoDelay(true)

			go func() {
				defer conn.Close()

				// upload
				h := sha256.New()
				io.Copy(h, io.LimitReader(conn, 1024*1024))

				if !bytes.Equal(h.Sum(make([]byte, 0)), uploadHash) {
					t.Fail()
					t.Log("wrong upload", conn.RemoteAddr().String())
				}

				// download
				_, err := conn.Write(download)
				if err != nil {
					t.Fail()
					t.Log("write failed", err)
				}
			}()
		}

		fmt.Println("remote closed")
	}()

	time.Sleep(time.Second * 2)

	// clients

	// clientTest connects to the cw2 client and perform testing
	clientTest := func() {
		t.Log("client test started")
		conn, err := net.Dial("tcp", "127.0.0.1:30010")
		if err != nil {
			t.Log("dial cw2 client", err)
			t.Fail()
			return
		}
		defer conn.Close()

		// upload
		_, err = conn.Write(upload)
		if err != nil {
			t.Fail()
			t.Log("client write", err)
		}
		t.Log("upload received", conn.LocalAddr().String())

		// download
		buf := make([]byte, 1024*1024)
		num, err := io.ReadFull(conn, buf)
		if err != nil {
			t.Fail()
			t.Log("client read", num, err, conn.LocalAddr().String())
		}
		t.Log("download sent", conn.LocalAddr().String())

		if !bytes.Equal(sha256sum(buf), downloadHash) {
			t.Fail()
			t.Log("wrong download", num, conn.LocalAddr().String())
			t.Log(buf[len(buf)-1024:])
		}
	}

	for i := 0; i < 10; i++ { // 10 concurrent workers
		go func() {
			for j := 0; j < n/10; j++ {
				clientTest()
			}
		}()
	}

	wg.Wait() // wait for the remote to finish
}
