package main

import (
	"math/rand"
	"net"
	"testing"

	"github.com/rs/zerolog"
)

func BenchmarkBounceDefault(b *testing.B) {
	conn1, conn2 := net.Pipe()
	defer conn1.Close()
	defer conn2.Close()
	zerolog.SetGlobalLevel(zerolog.Disabled)
	go func() {
		buffer := make([]byte, 1024)
		for {
			conn2.Read(buffer)
		}
	}()

	go bounceDefault(conn1, conn2)
	data := make([]byte, 1024)
	rand.Read(data)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		conn1.Write(data)
	}
}

func BenchmarkBounce(b *testing.B) {
	conn1, conn2 := net.Pipe()
	defer conn1.Close()
	defer conn2.Close()
	zerolog.SetGlobalLevel(zerolog.Disabled)
	go func() {
		buffer := make([]byte, 1024)
		for {
			conn2.Read(buffer)
		}
	}()

	go bounce(conn1, conn2)
	data := make([]byte, 1024)
	rand.Read(data)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		conn1.Write(data)
	}
}

