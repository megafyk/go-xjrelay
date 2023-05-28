package main

import (
	"errors"
	"sync/atomic"
)

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
