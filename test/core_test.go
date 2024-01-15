package test

import (
	"bytes"
	"commonweb2/client"
	"commonweb2/server"
	"crypto/rand"
	"fmt"
	"io"
	"net"
	"sync"
	"testing"
	"time"
)

// return a byte slice of length n, filled with random data
func randomBytes(n int) []byte {
	b := make([]byte, n)
	_, err := rand.Read(b)
	if err != nil {
		panic(err)
	}

	return b
}

// setup commonweb client and server
//
// closing the channel will stop commonweb client and server
func setupCommonweb(t *testing.T, ch chan any) {
	// client
	c := client.NewClient("http://127.0.0.1:20010", "http://127.0.0.1:20010", "127.0.0.1:30010", true)

	go func() {
		err := c.Start()
		if err != nil {
			fmt.Println("client start", err)
			return
		}
	}()

	go func() {
		<-ch
		c.Close()
	}()

	// server
	s := server.NewServer("127.0.0.1:20010", "127.0.0.1:30020")

	go func() {
		err := s.Start()
		if err != nil {
			fmt.Println("server start", err)
			return
		}
	}()

	go func() {
		<-ch
		s.Close()
	}()
}

func TestUpload(t *testing.T) {
	testDate := randomBytes(4096)

	ch := make(chan any)
	defer close(ch)

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

		conn, err := l.Accept()
		if err != nil {
			t.Fail()
			t.Log("remote accept", err)
			return
		}

		received := make([]byte, 4096)

		_, err = io.ReadFull(conn, received)
		if err != nil {
			t.Fail()
			t.Log("remote readfull", err)
			return
		}

		if !bytes.Equal(received, testDate) {
			t.Fail()
			t.Log("received wrong data")
			return
		}

		defer conn.Close()
	}()

	time.Sleep(time.Second * 5) // wait for client and server to start

	conn, err := net.Dial("tcp", "127.0.0.1:30010")
	if err != nil {
		t.Fail()
		t.Log("dial", err)
		return
	}

	defer conn.Close()

	_, err = conn.Write(testDate)
	if err != nil {
		t.Fail()
		t.Log("write", err)
		return
	}

	wg.Wait() // wait for remote to finish reading
}

func TestDownload(t *testing.T) {
	testDate := randomBytes(4096)

	ch := make(chan any)
	defer close(ch)

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

		conn, err := l.Accept()
		if err != nil {
			t.Fail()
			t.Log("remote accept", err)
			return
		}

		_, err = conn.Write(testDate)
		if err != nil {
			t.Fail()
			t.Log("remote write", err)
			return
		}

		defer conn.Close()
	}()

	time.Sleep(time.Second * 5) // wait for client and server to start

	conn, err := net.Dial("tcp", "127.0.0.1:30010")
	if err != nil {
		t.Fail()
		t.Log("dial", err)
		return
	}

	defer conn.Close()

	received := make([]byte, 4096)
	_, err = io.ReadFull(conn, received)
	if err != nil {
		t.Fail()
		t.Log("read", err)
		return
	}

	if !bytes.Equal(received, testDate) {
		t.Fail()
		t.Log("received wrong data")
		return
	}

	wg.Wait() // wait for remote to finish reading
}
