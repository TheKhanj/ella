package main

import (
	"errors"
	"io"
	"log"
	"sync"

	"github.com/cskr/pubsub/v2"
)

var ProcsErrEmpty = errors.New("no proc has been pushed")

type Procs struct {
	mu   sync.RWMutex
	last *Proc

	procs *pubsub.PubSub[int, *Proc]
}

func (this *Procs) Push(proc *Proc) {
	this.procs.Pub(proc, 0)
}

func (this *Procs) Shutdown() {
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
		// TODO: I'm quite sure this wouldn't handle killing the process
		// and still receiving the new processes logs...
		// write couple of tests for it bitch 😠
		if err == nil {
			_, err := io.Copy(w, getPipe(last))
			if err != nil {
				log.Println("service:", err)
				return
			}
		}

		ch := this.procs.Sub(0)
		defer this.procs.Unsub(ch)

		for proc := range ch {
			_, err := io.Copy(w, getPipe(proc))
			if err != nil {
				log.Println("service:", err)
				return
			}
		}
	}()

	return r
}

func NewProcs() *Procs {
	return &Procs{
		mu:   sync.RWMutex{},
		last: nil,

		procs: pubsub.New[int, *Proc](0),
	}
}
