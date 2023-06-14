package main

import (
	"context"
	"github.com/rs/zerolog/log"
	"io"
	"net"
	"os"
	"testing"
	"time"
)

const (
	ADDRA = "localhost:8080"
	ADDRB = "localhost:8081"
)

var (
	conn1    net.Conn
	conn2    net.Conn
	done     = make(chan int, 2)
	filename = "data.csv"
)

func init() {
	go initServer(ADDRA)
	go initServer(ADDRB)

	for i := 0; i < cap(done); i++ {
		<-done
	}

	conn1, _ = net.Dial("tcp", ADDRA)
	conn2, _ = net.Dial("tcp", ADDRB)
	conn1.SetDeadline(time.Now().Add(5 * time.Second))
	conn2.SetDeadline(time.Now().Add(5 * time.Second))
	stat, _ := os.Stat(filename)
	log.Info().Msgf("test data file size %d", stat.Size())
}

func initServer(addr string) {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		log.Err(err).Msgf("failed to create listen server %s", addr)
		done <- 1
		return
	}
	defer func(ln net.Listener) {
		err := ln.Close()
		if err != nil {
			log.Err(err).Msgf("failed to close tcp server %s", addr)
		}
	}(ln)
	done <- 1

	for {
		conn, err := ln.Accept()
		log.Info().Msgf("accept conn from %s", conn.RemoteAddr())
		if err != nil {
			log.Err(err).Msg("error when accept connection")
		}
		_, err = copyWithTimeout(conn, io.Discard, 3*time.Second)
		if err != nil {
			log.Err(err).Msg("error when copy data")
		}
	}
}

func copyWithTimeout(src io.Reader, dst io.Writer, timeout time.Duration) (int64, error) {
	// Create a context with a timeout
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// Create a copy function
	copyFunc := func() {
		// Copy data using the context
		_, _ = io.CopyBuffer(dst, src, make([]byte, 1024*1024))
	}

	// Start copying in a goroutine
	go copyFunc()

	// Wait for copying to finish or context to timeout
	<-ctx.Done()

	// Check if timeout happened
	if ctx.Err() == context.DeadlineExceeded {
		return 0, ctx.Err()
	}

	// Otherwise, copying finished successfully
	return -1, nil
}

func test(data *os.File) {
	buffer := make([]byte, 4096)
	numWrittenBytes := 0
	for {
		n, err := data.Read(buffer)

		if err != nil {
			if err == io.EOF {
				log.Info().Msgf("EOF")
			} else {
				log.Err(err).Msgf("failed to read")
			}
			break
		}
		_, err = conn1.Write(buffer[:n])

		if err != nil {
			log.Err(err).Msg("failed write")
			break
		}
		numWrittenBytes += n
	}
	log.Info().Msgf("transferred %d", numWrittenBytes)
}

func TestBounceDefault(t *testing.T) {
	go bounceDefault(conn1, conn2)
	data, _ := os.Open(filename)

	start := time.Now()
	test(data)
	elapsed := time.Since(start)
	log.Info().Msgf("BenchmarkBounceDefault took %s", elapsed)
}

func TestBounce(t *testing.T) {
	go bounce(conn1, conn2)
	data, _ := os.Open(filename)

	start := time.Now()
	test(data)
	elapsed := time.Since(start)
	log.Info().Msgf("BenchmarkBounceDefault took %s", elapsed)
}

//
//func BenchmarkBounceDefault(b *testing.B) {
//	go bounceDefault(conn1, conn2)
//	data, _ := os.Open(filename)
//
//	start := time.Now()
//	test(data)
//	elapsed := time.Since(start)
//	log.Info().Msgf("BenchmarkBounceDefault took %s", elapsed)
//}
//
//func BenchmarkBounce(b *testing.B) {
//	go bounce(conn1, conn2)
//	data, _ := os.Open(filename)
//
//	start := time.Now()
//	test(data)
//	elapsed := time.Since(start)
//	log.Info().Msgf("BenchmarkBounceDefault took %s", elapsed)
//}
