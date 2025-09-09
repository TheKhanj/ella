// MIT License
// Copyright (c) 2025 Pooyan Khanjankhani

package main

import (
	"io"
	"sync"
)

type Broadcaster struct {
	mu      sync.RWMutex
	writers map[io.WriteCloser]struct{}
}

func NewBroadcaster() *Broadcaster {
	return &Broadcaster{
		writers: make(map[io.WriteCloser]struct{}),
	}
}

func (this *Broadcaster) Add(w io.WriteCloser) {
	this.mu.Lock()
	this.writers[w] = struct{}{}
	this.mu.Unlock()
}

func (this *Broadcaster) Remove(w io.WriteCloser) {
	this.mu.Lock()
	this.remove(w)
	this.mu.Unlock()
}

func (this *Broadcaster) Write(p []byte) (n int, err error) {
	this.mu.RLock()
	removing := make([]io.WriteCloser, 0)
	l := len(p)
	for w := range this.writers {
		n, e := w.Write(p)
		if e != nil || n < l {
			removing = append(removing, w)
		}
	}
	this.mu.RUnlock()

	for _, w := range removing {
		this.Remove(w)
	}

	return l, nil
}

func (this *Broadcaster) Run(r io.Reader) error {
	_, err := io.Copy(this, r)
	this.removeAll()
	return err
}

func (this *Broadcaster) removeAll() {
	this.mu.Lock()
	defer this.mu.Unlock()
	for w := range this.writers {
		this.remove(w)
	}
}

func (this *Broadcaster) remove(w io.WriteCloser) {
	w.Close()
	delete(this.writers, w)
}
