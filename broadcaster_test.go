package main

import (
	"io"
	"strings"
	"sync"
	"testing"
)

type BroadcasterTest struct {
	t   *testing.T
	b   *Broadcaster
	str string
}

func (this *BroadcasterTest) Run() {
	input := strings.NewReader(this.str)

	var wg sync.WaitGroup
	// 1: boardcaster run
	// 2,3,4,...,100: check pipe
	pipesCnt := 3
	wg.Add(1 + pipesCnt)
	for i := 0; i < pipesCnt; i++ {
		r, w := io.Pipe()
		this.b.Add(w)
		go this.checkOutput(r, w, &wg)
	}

	go func() {
		defer wg.Done()

		err := this.b.Run(input)
		if err != nil {
			this.t.Error(err)
			this.t.Fail()
		}
	}()

	wg.Wait()
}

func (this *BroadcasterTest) checkOutput(
	r *io.PipeReader, w *io.PipeWriter,
	wg *sync.WaitGroup,
) {
	defer wg.Done()
	defer this.b.Remove(w)
	defer r.Close()

	b, err := io.ReadAll(r)
	if err != nil {
		this.t.Error(err)
		this.t.Fail()
	}
	if string(b) != this.str {
		this.t.Errorf(
			"unexpected received value: expected: %s, received: %s",
			this.str, string(b),
		)
		this.t.Fail()
	}
}

func TestBroadcaster(t *testing.T) {
	bt := BroadcasterTest{t, NewBroadcaster(), "0123456789"}
	bt.Run()
}
