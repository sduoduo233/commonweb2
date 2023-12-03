package test

import (
	"bytes"
	"crypto"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net"
	"sync"
	"testing"
	"time"
)

// return a random int between 0 and n
func randint(n int) int {
	i, err := rand.Int(rand.Reader, big.NewInt(int64(n)))
	if err != nil {
		panic(err)
	}

	return int(i.Int64())
}

// testing uploading random data with random length
func TestUpload2(t *testing.T) {
	write := crypto.SHA256.New()
	read := crypto.SHA256.New()

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

		// read from conn and calculate hash of received data
		for {
			buf := make([]byte, 2048)
			n, err := conn.Read(buf)
			if err != nil {
				if errors.Is(err, io.EOF) {
					break
				}

				t.Fail()
				t.Log("remote read", err)
				return
			}

			fmt.Println("read", time.Now().Format(time.DateTime), n)

			read.Write(buf[:n])
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

	tcpConn := conn.(*net.TCPConn)
	err = tcpConn.SetNoDelay(true)
	if err != nil {
		conn.Close()
		t.Fail()
		t.Log("set no delay", err)
		return
	}

	n := randint(20)
	for i := 0; i < n; i++ {
		buf := randomBytes(randint(2048))

		if len(buf) == 0 {
			continue
		}

		write.Write(buf)

		fmt.Println("write", time.Now().Format(time.DateTime), len(buf))

		_, err := conn.Write(buf)
		if err != nil {
			conn.Close()

			t.Fail()
			t.Log("write", err)
			return
		}
	}

	conn.Close()

	wg.Wait() // wait for remote to finish reading

	// compare hash
	readHash := read.Sum(make([]byte, 0))
	writeHash := write.Sum(make([]byte, 0))

	if !bytes.Equal(readHash, writeHash) {
		t.Fail()
		t.Log("wrong hash")
		return
	}

}

// testing upload buffering
func TestUploadBuffering(t *testing.T) {
	write := crypto.SHA256.New()
	read := crypto.SHA256.New()

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

		// read from conn and calculate hash of received data
		for {
			buf := make([]byte, 2048)
			n, err := conn.Read(buf)
			if err != nil {
				if errors.Is(err, io.EOF) {
					break
				}

				t.Fail()
				t.Log("remote read", err)
				return
			}

			fmt.Println("read", time.Now().Format(time.DateTime), n)

			read.Write(buf[:n])
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

	tcpConn := conn.(*net.TCPConn)
	err = tcpConn.SetNoDelay(true)
	if err != nil {
		conn.Close()
		t.Fail()
		t.Log("set no delay", err)
		return
	}

	n := randint(20)
	for i := 0; i < n+10; i++ {
		buf := randomBytes(randint(1024))

		if len(buf) == 0 {
			continue
		}

		write.Write(buf)

		fmt.Println("write", time.Now().Format(time.DateTime), len(buf))

		_, err := conn.Write(buf)
		if err != nil {
			conn.Close()

			t.Fail()
			t.Log("write", err)
			return
		}

		time.Sleep(time.Second * time.Duration(randint(10)))
	}

	conn.Close()

	wg.Wait() // wait for remote to finish reading

	// compare hash
	readHash := read.Sum(make([]byte, 0))
	writeHash := write.Sum(make([]byte, 0))

	if !bytes.Equal(readHash, writeHash) {
		t.Fail()
		t.Log("wrong hash")
		return
	}

}
