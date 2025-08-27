package main

import (
	"context"
	"io"
	"strings"
	"sync"
	"testing"
	"time"
)

type ProcPipesTest struct {
	t      *testing.T
	shell  string
	script string
	stdout string
	stderr string
}

func (this *ProcPipesTest) Run() {
	p := NewProc(this.shell)
	p.Stdin = strings.NewReader(this.script)
	stdout := p.StdoutPipe()
	defer stdout.Close()
	stderr := p.StderrPipe()
	defer stderr.Close()

	ctx, cancel := context.WithTimeout(this.t.Context(), time.Second)
	defer cancel()

	done := make(chan struct{})
	go func() {
		defer close(done)

		err := p.Start(ctx)
		if err != nil {
			this.t.Error(err)
			this.t.Fail()
		}
	}()

	this.assertOutput(stdout, this.stdout)
	this.assertOutput(stderr, this.stderr)

	<-done
}

func (this *ProcPipesTest) assertOutput(
	out io.Reader, expected string,
) {
	b, err := io.ReadAll(out)
	if err != nil {
		this.t.Error(err)
		this.t.Fail()
	}
	if string(b) != expected {
		this.t.Errorf("unexpected standard output: %s", string(b))
		this.t.Fail()
	}
}

func TestProcPipes(t *testing.T) {
	pt := ProcPipesTest{
		t,
		"/usr/bin/sh",
		`echo -n stdout
		echo -n stderr >&2`,
		"stdout",
		"stderr",
	}
	pt.Run()
}

type ProcStatesTest struct {
	t       *testing.T
	shell   string
	script  string
	timeout time.Duration
}

func (this *ProcStatesTest) Run() {
	p := NewProc(this.shell)
	p.Stdin = strings.NewReader(this.script)

	ctx, cancel := context.WithTimeout(this.t.Context(), this.timeout)
	defer cancel()

	states := p.Sub()

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()

		this.assertEvents(states)
	}()
	go func() {
		defer wg.Done()

		err := p.Start(ctx)
		if err != nil {
			this.t.Error(err)
			this.t.Fail()
		}

		p.Unsub(states)
	}()

	wg.Wait()
}

func (this *ProcStatesTest) assertEvents(states chan ProcState) {
	shouldBe := func(expected, received ProcState) {
		if received != expected {
			this.t.Errorf(
				"unexpected state: expected: %d received: %d",
				expected, received,
			)
			this.t.Fail()
		}
	}

	i := 0

	for state := range states {
		switch i {
		case 0:
			shouldBe(ProcStateStarting, state)
		case 1:
			shouldBe(ProcStateStarted, state)
		case 2:
			shouldBe(ProcStateStopped, state)
		case 3:
			shouldBe(ProcStateWaitDone, state)
		case 4:
			this.t.Error("should not receive shutted down state change event!")
			this.t.Fail()
		}

		i++
	}

	if i != 4 {
		this.t.Errorf("expected 5 state changes received %d", i)
		this.t.Fail()
	}
}

func TestProcStates(t *testing.T) {
	pt1 := ProcStatesTest{
		t,
		"/usr/bin/sh",
		"sleep 1",
		time.Second * 5,
	}
	pt1.Run()

	var wg sync.WaitGroup
	n := 100
	wg.Add(n)

	for i := 0; i < n; i++ {
		pt := ProcStatesTest{
			t,
			"/usr/bin/sh",
			"sleep 1",
			time.Second * 10,
		}

		go func() {
			defer wg.Done()

			pt.Run()
		}()
	}

	wg.Wait()
}
