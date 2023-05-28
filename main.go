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
	addrB := "192.168.122.118:22"
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

// bounce between conn1 and conn 2
func bounce(conn1 net.Conn, conn2 net.Conn) {
	defer closeConn(conn1)
	defer closeConn(conn2)

	//_, err := io.Copy(conn2, conn1)
	//if err != nil && !isNetConnClosedErr(err) {
	//	log.Err(err).Msgf("failed to write connection from %s to %s", conn1.RemoteAddr(), conn2.RemoteAddr())
	//	return
	//}

	//
	//go func() {
	//	for {
	//		data := make([]byte, 1024)
	//		n, err := conn1.Read(data)
	//		if err != nil {
	//			log.Err(err).Msgf("failed to read conn1")
	//			return
	//		}
	//
	//		for i := 0; i < n; i++ {
	//			err := buffer.Write(int(data[i]))
	//			if err != nil {
	//				log.Err(err).Msgf("failed to write conn2")
	//				return
	//			}
	//		}
	//	}
	//}()
	//
	//for {
	//	data := make([]byte, 1024)
	//	n, err := conn2.Read(data)
	//	if err != nil {
	//		log.Err(err).Msgf("failed to read conn2")
	//		return
	//	}
	//
	//	for i := 0; i < n; i++ {
	//		value, _ := buffer.Read()
	//		data[i] = byte(value)
	//	}
	//
	//	_, err = conn2.Write(data[:n])
	//	if err != nil {
	//		return
	//	}
	//}

	buffer := NewRingBuffer(1024)
	done := make(chan bool)
	go func() {
		for {
			data := make([]byte, 1024)
			_, err := conn1.Read(data)
			log.Info().Msg(string(data))
			if err != nil {
				if err != io.EOF {
					log.Err(err).Msgf("failed to read conn1")
				}
				done <- true
				return
			}
			err = buffer.Write(data)
			if err != nil {
				log.Err(err).Msgf("failed to write data from conn1 to buffer")
				return
			}
		}
	}()
	for {
		if <-done {
			break
		}
		data, _ := buffer.Read()
		_, err := conn2.Write(data)
		if err != nil {
			log.Err(err).Msgf("failed to write data from buffer to conn2")
		}
	}

	//buf := make([]byte, 32*1024)
	//for {
	//	n, err := conn1.Read(buf)
	//	if err != nil {
	//		return
	//	}
	//	_, err = conn2.Write(buf[:n])
	//	if err != nil {
	//		return
	//	}
	//}
}

// close socket connection
func closeConn(conn net.Conn) {
	err := conn.Close()
	if err != nil && !isNetConnClosedErr(err) {
		log.Err(err).Msgf("failed to close connection %s", conn.RemoteAddr())
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
