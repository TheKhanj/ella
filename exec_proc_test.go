package main

import (
	"context"
	"io"
	"strings"
	"testing"
	"time"
)

type ExecProcPipesTest struct {
	t      *testing.T
	shell  string
	script string
	stdout string
	stderr string
}

func (this *ExecProcPipesTest) Run() {
	p := NewExecProc(this.shell)
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

		err := p.Run(ctx)
		if err != nil {
			this.t.Error(err)
			this.t.Fail()
		}
	}()

	this.assertOutput(stdout, this.stdout)
	this.assertOutput(stderr, this.stderr)

	<-done
}

func (this *ExecProcPipesTest) assertOutput(
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

func TestExecProcPipes(t *testing.T) {
	pt := ExecProcPipesTest{
		t,
		"/usr/bin/sh",
		`echo -n stdout
		echo -n stderr >&2`,
		"stdout",
		"stderr",
	}
	pt.Run()
}
