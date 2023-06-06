package main

import (
	"errors"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"io"
	"net"
	"syscall"
)

func main() {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	go relay()
	done := make(chan bool)
	log.Info().Msgf("relay server started")
	<-done
	close(done)
}

func relay() {
	// init A listener
	addrA := "localhost:8080"
  addrB := "192.168.112.18:22"
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
	}
}

// bounce between conn1 and conn 2 use disruptor
func bounceDisruptor(conn1 net.Conn, conn2 net.Conn) {
	defer closeConn(conn1)
	defer closeConn(conn2)

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
	defer closeConn(conn1)
	defer closeConn(conn2)
	_, err := io.Copy(conn2, conn1)
	if err != nil && !isNetConnClosedErr(err) {
		log.Err(err).Msgf("failed to write connection from %s to %s", conn1.RemoteAddr(), conn2.RemoteAddr())
		return
	}
}

func bounceDefault(conn1 net.Conn, conn2 net.Conn) {
	defer closeConn(conn1)
	defer closeConn(conn2)
	buffer := make([]byte, 1024)
	for {
		n, err := conn1.Read(buffer)
		if err != nil {
			if isNetConnClosedErr(err) {
				log.Err(err).Msgf("failed to read data from closed connection %s", conn1.RemoteAddr())
        return
			}
		}

		data := buffer[:n]

		_, err = conn2.Write(data)
		if err != nil {
			if !isNetConnClosedErr(err) {
				log.Err(err).Msgf("failed to write data to closed connection %s", conn2.RemoteAddr())
				return
			}
		}
	}
}

// bounce between conn1 and conn 2 use zero copy optimization
func bounce(conn1 net.Conn, conn2 net.Conn) {
	defer conn1.Close()
	defer conn2.Close()

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
		  log.Err(err).Msgf("failed to read fd %d %s", fileConn1, pipe[1])	
			return
		}
		_, err = syscall.Splice(pipe[0], nil, int(fileConn2.Fd()), nil, 1024, 1)
		if err != nil {
      log.Err(err).Msgf("failed to write fd %d %s", fileConn2, pipe[0])
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
