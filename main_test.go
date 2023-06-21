package main

import (
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"net"
	"testing"
)

func BenchmarkBounceDefault(b *testing.B) {
	conn1, _ := net.Dial("tcp", addrA)
	conn2, _ := net.Dial("tcp", addrB)
	defer conn1.Close()
	defer conn2.Close()
	zerolog.SetGlobalLevel(zerolog.Disabled)

	go bounceDefault(conn1, conn2)

	msg := "Hello from Client"
	b.ResetTimer()
	for i := 0; i < 1000000; i++ {
		_, err := conn1.Write([]byte(msg))
		if err != nil {
			log.Err(err).Msgf("cannot write")
			return
		}
	}
}

func BenchmarkBounce(b *testing.B) {
	conn1, _ := net.Dial("tcp", addrA)
	conn2, _ := net.Dial("tcp", addrB)
	defer conn1.Close()
	defer conn2.Close()
	zerolog.SetGlobalLevel(zerolog.Disabled)

	go bounce(conn1, conn2)

	msg := "Hello from Client"
	b.ResetTimer()
	for i := 0; i < 1000000; i++ {
		_, err := conn1.Write([]byte(msg))
		if err != nil {
			log.Err(err).Msgf("cannot write")
			return
		}
	}
}
