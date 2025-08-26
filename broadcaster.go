package main

import (
	"io"
	"sync"
)

type Broadcaster struct {
	mu      sync.RWMutex
	writers map[io.Writer]chan error
}

func NewBroadcaster() *Broadcaster {
	return &Broadcaster{
		writers: make(map[io.Writer]chan error),
	}
}

func (this *Broadcaster) Add(w io.Writer) <-chan error {
	errs := make(chan error)
	this.mu.Lock()
	this.writers[w] = errs
	this.mu.Unlock()

	return errs
}

func (this *Broadcaster) Remove(w io.Writer) {
	this.mu.Lock()
	errs := this.writers[w]
	close(errs)
	delete(this.writers, w)
	this.mu.Unlock()
}

func (this *Broadcaster) Write(p []byte) (n int, err error) {
	this.mu.RLock()
	defer this.mu.RUnlock()

	l := len(p)
	for w, errs := range this.writers {
		n, e := w.Write(p)
		if e != nil {
			errs <- err
		} else if n < l {
			errs <- io.ErrShortWrite
		}
	}

	return l, nil
}

func (this *Broadcaster) Run(r io.Reader) error {
	_, err := io.Copy(this, r)
	return err
}
