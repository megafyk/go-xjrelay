package main

import (
	"errors"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"io"
	"net"
	"sync/atomic"
	"syscall"
)

const (
	addrA = "localhost:8080"
	addrB = "localhost:8081"
)

var (
	done = make(chan int, 2)
)

func main() {
	process2()
}

func process1() {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	go relay()
	done := make(chan bool)
	log.Info().Msgf("relay server started")
	<-done
	close(done)
}

func process2() {
	go initServer(addrA)
	go initServer(addrB)
	log.Info().Msg("start server...")
	<-done
}

func relay() {
	// init A listener
	addrA := "localhost:8080"
	addrB := "192.168.112.118:22"
	ln, err := net.Listen("tcp", addrA)
	if err != nil {
		log.Err(err).Msgf("failed to create listener to %s", addrA)
		return
	}
	defer closeListener(ln)

	for {
		// accept connection
		conn1, err := ln.Accept()
		if err != nil {
			log.Err(err).Msgf("failed to accept connection to %s", addrA)
			return
		}

		// create connection to node B
		conn2, err := net.Dial("tcp", addrB)
		if err != nil {
			log.Err(err).Msgf("failed to create connection to %s", addrB)
			return
		}

		go bounce(conn1, conn2)
		go bounce(conn2, conn1)
		defer closeConn(conn1)
		defer closeConn(conn2)
	}
}

// bounce between conn1 and conn 2 use disruptor
func bounceDisruptor(conn1 net.Conn, conn2 net.Conn) {

	buffer := NewRingBuffer(1024)
	t := false
	go func() {
		for {
			data := make([]byte, 32*1024)
			n, err := conn1.Read(data)
			if err != nil {
				if !isNetConnClosedErr(err) {
					log.Err(err).Msgf("failed to read conn1")
				}
				t = true
				return
			}
			if n > 0 {
				err = buffer.Write(data[:n])
				if err != nil {
					log.Err(err).Msgf("failed to write data from conn1 to buffer")
					return
				}
			}
		}
	}()

	for !t {
		data, _ := buffer.Read()
		if data != nil {
			_, err := conn2.Write(data)
			if err != nil {
				log.Err(err).Msgf("failed to write data from buffer to conn2")
			}
		}
	}
	log.Info().Msgf("end bounce %s -> %s", conn1.RemoteAddr(), conn2.RemoteAddr())

}

// bounce between conn1 and conn 2 use default copy
func bounceCopy(conn1 net.Conn, conn2 net.Conn) {
	_, err := io.Copy(conn2, conn1)
	if err != nil && !isNetConnClosedErr(err) {
		log.Err(err).Msgf("failed to write connection from %s to %s", conn1.RemoteAddr(), conn2.RemoteAddr())
		return
	}
}

func bounceDefault(conn1 net.Conn, conn2 net.Conn) {
	buffer := make([]byte, 1024)
	for {
		n, err := conn1.Read(buffer)
		if err != nil {
			if isNetConnClosedErr(err) {
				return
			}
		}

		data := buffer[:n]

		_, err = conn2.Write(data)
		if err != nil {
			if isNetConnClosedErr(err) {
				return
			}
		}
	}
}

// bounce between conn1 and conn 2 use zero copy optimization
func bounce(conn1 net.Conn, conn2 net.Conn) {

	pipe := make([]int, 2)
	// init kernel pipe
	if err := syscall.Pipe(pipe); err != nil {
		log.Err(err).Msgf("failed to create pipe")
		return
	}

	defer syscall.Close(pipe[0])
	defer syscall.Close(pipe[1])

	fileConn1, err := conn1.(*net.TCPConn).File()
	if err != nil {
		log.Err(err).Msgf("cannot get file descriptor of connection from %s", conn1.RemoteAddr())
		return
	}
	fileConn2, err := conn2.(*net.TCPConn).File()
	if err != nil {
		log.Err(err).Msgf("cannot get file descriptor of connection from %s", conn2.RemoteAddr())
		return
	}

	for {
		_, err := syscall.Splice(int(fileConn1.Fd()), nil, pipe[1], nil, 1024, 1)
		if err != nil {
			return
		}
		_, err = syscall.Splice(pipe[0], nil, int(fileConn2.Fd()), nil, 1024, 1)
		if err != nil {
			return
		}
	}
}

// close socket connection
func closeConn(conn net.Conn) {
	err := conn.Close()
	if err != nil && !isNetConnClosedErr(err) {
		log.Err(err).Msgf("failed to close connection %s", conn.RemoteAddr())
	} else {
		log.Info().Msgf("close connection to %s", conn.RemoteAddr())
	}
}

// check is error close
func isNetConnClosedErr(err error) bool {
	switch {
	case
		errors.Is(err, net.ErrClosed),
		errors.Is(err, io.EOF),
		errors.Is(err, syscall.EPIPE):
		return true
	default:
		return false
	}
}

// close network listener
func closeListener(ln net.Listener) {
	err := ln.Close()
	if err != nil {
		log.Err(err).Msgf("failed to close listener %s", ln.Addr())
	}
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
			log.Err(err).Msg("err when close listener")
			return
		}
	}(ln)
	if err != nil {
		log.Err(err)
		return
	}

	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Err(err).Msg("error when accept connection")
		}
		_, err = io.Copy(io.Discard, conn)
		if err != nil {
			log.Err(err).Msg("error when copy data")
		}
	}
}

type RingBuffer struct {
	data       [][]byte
	readIndex  int32
	writeIndex int32
}

func NewRingBuffer(size int) *RingBuffer {
	return &RingBuffer{
		data:       make([][]byte, size),
		readIndex:  0,
		writeIndex: 0,
	}
}

func (r *RingBuffer) Write(value []byte) error {
	if r.writeIndex-r.readIndex == int32(len(r.data)) {
		return errors.New("buffer is full")
	}

	r.data[r.writeIndex%int32(len(r.data))] = value
	atomic.AddInt32(&r.writeIndex, 1)

	return nil
}

func (r *RingBuffer) Read() ([]byte, error) {
	if r.writeIndex == r.readIndex {
		return nil, errors.New("buffer is empty")
	}

	value := r.data[r.readIndex%int32(len(r.data))]
	atomic.AddInt32(&r.readIndex, 1)

	return value, nil
}
