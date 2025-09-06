package main

import (
	"errors"
	"io"
	"log"
	"sync"
	"sync/atomic"

	"github.com/cskr/pubsub/v2"
)

var ProcsErrEmpty = errors.New("no proc has been pushed")

type Procs struct {
	mu   sync.RWMutex
	last *Proc

	procs   *pubsub.PubSub[int, *Proc]
	running atomic.Bool
}

func (this *Procs) Push(proc *Proc) {
	if this.running.Load() {
		this.procs.Pub(proc, 0)
	}
	this.mu.Lock()
	this.last = proc
	this.mu.Unlock()
}

func (this *Procs) Shutdown() {
	this.running.Store(false)
	this.procs.Shutdown()
}

func (this *Procs) Last() (*Proc, error) {
	this.mu.RLock()
	defer this.mu.RUnlock()

	if this.last == nil {
		return nil, ProcsErrEmpty
	}

	return this.last, nil
}

func (this *Procs) StdoutPipe() io.ReadCloser {
	return this.pipe(
		func(proc *Proc) io.ReadCloser { return proc.StdoutPipe() },
	)
}

func (this *Procs) StderrPipe() io.ReadCloser {
	return this.pipe(
		func(proc *Proc) io.ReadCloser { return proc.StderrPipe() },
	)
}

func (this *Procs) pipe(
	getPipe func(proc *Proc) io.ReadCloser,
) io.ReadCloser {
	r, w := io.Pipe()
	go func() {
		defer w.Close()

		last, err := this.Last()
		if err == nil {
			_, err := io.Copy(w, getPipe(last))
			if err != nil {
				log.Println("procs:", err)
				return
			}
		}

		if !this.running.Load() {
			log.Println("procs: pubsub not running")
			return
		}

		ch := this.procs.Sub(0)
		defer func() {
			if this.running.Load() {
				this.procs.Unsub(ch)
			}
		}()

		for proc := range ch {
			_, err := io.Copy(w, getPipe(proc))
			if err != nil {
				log.Println("procs:", err)
				return
			}
		}
	}()

	return r
}

func NewProcs() *Procs {
	running := atomic.Bool{}
	running.Store(true)

	return &Procs{
		mu:   sync.RWMutex{},
		last: nil,

		procs:   pubsub.New[int, *Proc](0),
		running: running,
	}
}
