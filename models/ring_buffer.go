package models

import "sync"

type RingBuffer struct {
	mutex    sync.Mutex
	data     []string
	capacity int
	head     int
	full     bool
}

func NewRingBuffer(cap int) *RingBuffer {
	return &RingBuffer{data: make([]string, cap), capacity: cap}
}

func (rb *RingBuffer) Push(line string) {
	rb.mutex.Lock()
	defer rb.mutex.Unlock()
	rb.data[rb.head] = line
	rb.head = (rb.head + 1) % rb.capacity
	if rb.head == 0 {
		rb.full = true
	}
}

func (rb *RingBuffer) LastN(n int) []string {
	rb.mutex.Lock()
	defer rb.mutex.Unlock()
	if n <= 0 {
		return nil
	}
	var out []string
	size := rb.capacity
	if !rb.full {
		size = rb.head
	}
	if size == 0 {
		return nil
	}
	if n > size {
		n = size
	}
	for i := 0; i < n; i++ {
		idx := (rb.head - 1 - i + rb.capacity) % rb.capacity
		out = append(out, rb.data[idx])
	}
	// Reverse them to go from oldest to newest
	for i := 0; i < len(out)/2; i++ {
		j := len(out) - 1 - i
		out[i], out[j] = out[j], out[i]
	}
	return out
}
