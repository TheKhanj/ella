package main

import (
	"context"
	"io"
	"os"
	"strconv"
	"sync"
	"testing"
	"time"
)

type ProcsTest struct {
	t     *testing.T
	count int
	sleep time.Duration
}

func (this *ProcsTest) Run() {
	procs := NewProcs()

	this.assertLast(procs)

	readerStopped := this.runReader(procs)
	processStarted := this.runProcesses(procs)

loop:
	for {
		select {
		case <-readerStopped:
			this.t.Log("reader closed when process exited")
			this.t.FailNow()

			break loop
		case _, ok := <-processStarted:
			if !ok {
				break loop
			}

			continue
		}
	}

	procs.Shutdown()

	select {
	case <-readerStopped:
		break
	case <-time.After(time.Second * 2):
		this.t.Log("reader did not close")
		this.t.FailNow()
	}
}

func (this *ProcsTest) runReader(procs *Procs) chan struct{} {
	ret := make(chan struct{})

	go func() {
		defer close(ret)

		r := procs.StdoutPipe()
		defer r.Close()

		_, err := io.Copy(os.Stdout, r)
		if err != nil {
			this.t.Log(err)
		}
	}()

	return ret
}

func (this *ProcsTest) runProcesses(procs *Procs) chan struct{} {
	ret := make(chan struct{})

	go func() {
		defer close(ret)

		for i := 0; i < this.count; i++ {
			ctx, cancel := context.WithCancel(this.t.Context())
			var wg sync.WaitGroup
			wg.Add(1)
			go func() {
				defer wg.Done()

				ret <- struct{}{}

				proc := NewProc("/usr/bin/echo", strconv.Itoa(i+1))
				procs.Push(proc)
				proc.Run(ctx)
			}()

			time.Sleep(this.sleep)

			cancel()
			wg.Wait()
		}
	}()

	return ret
}

func (this *ProcsTest) assertLast(procs *Procs) {
	_, err := procs.Last()
	if err == nil {
		this.t.Error("expected to return an error")
		this.t.FailNow()
	}
}

func TestProcs(t *testing.T) {
	pt := ProcsTest{t, 50, time.Millisecond * 50}
	pt.Run()
}
