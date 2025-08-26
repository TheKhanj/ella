package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"sync/atomic"
	"syscall"
)

type ExecProcState int32

const (
	EXEC_PROC_STATE_NOT_STARTED ExecProcState = iota
	EXEC_PROC_STATE_STARTING
	EXEC_PROC_STATE_STARTED
	EXEC_PROC_STATE_STOPPED
	EXEC_PROC_STATE_DONE
)

// TODO: add subscription to exit code or state changes
type ExecProc struct {
	Name string
	Args []string

	Stdin io.Reader

	Cwd string
	Uid uint32
	Gid uint32
	Env []string

	state atomic.Int32

	stdout *Broadcaster
	stderr *Broadcaster

	cmd *exec.Cmd
}

func (this *ExecProc) Run(ctx context.Context) error {
	err := this.setState(EXEC_PROC_STATE_STARTING)
	if err != nil {
		return err
	}

	this.setCmd(ctx)
	stdout, err := this.cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := this.cmd.StderrPipe()
	if err != nil {
		return err
	}

	err = this.cmd.Start()
	if err != nil {
		return err
	}

	err = this.setState(EXEC_PROC_STATE_STARTED)
	if err != nil {
		return err
	}

	err = this.waitForCmd(stdout, stderr)
	if err != nil {
		return err
	}

	return nil
}

func (this *ExecProc) StdoutPipe() io.ReadCloser {
	return this.pipe(this.stdout)
}

func (this *ExecProc) StderrPipe() io.ReadCloser {
	return this.pipe(this.stderr)
}

func (this *ExecProc) pipe(b *Broadcaster) io.ReadCloser {
	r, w := io.Pipe()
	b.Add(w)
	return r
}

func (this *ExecProc) setCmd(ctx context.Context) {
	cmd := exec.CommandContext(ctx, this.Name, this.Args...)

	if this.Cwd != "" {
		cmd.Dir = this.Cwd
	}
	if this.Env != nil {
		cmd.Env = this.Env
	}
	if this.Stdin != nil {
		cmd.Stdin = this.Stdin
	}

	if this.Uid != uint32(syscall.Getuid()) ||
		this.Gid != uint32(syscall.Getgid()) {
		cmd.SysProcAttr = &syscall.SysProcAttr{
			Credential: &syscall.Credential{
				Uid: this.Uid,
				Gid: this.Gid,
			},
		}
	}

	this.cmd = cmd
}

func (this *ExecProc) waitForCmd(stdout, stderr io.ReadCloser) error {
	var wg sync.WaitGroup
	// 1: stdout broadcaster
	// 2: stderr broadcaster
	// 3: command execution
	wg.Add(3)

	go func() {
		defer wg.Done()

		this.flush(this.stdout, stdout)
	}()
	go func() {
		defer wg.Done()

		this.flush(this.stderr, stderr)
	}()
	go func() {
		defer wg.Done()

		err := this.cmd.Wait()
		if err != nil {
			fmt.Println("process:", err)
		}

		this.setState(EXEC_PROC_STATE_STOPPED)
	}()

	wg.Wait()
	return this.setState(EXEC_PROC_STATE_DONE)
}

func (this *ExecProc) setState(state ExecProcState) error {
	curr := this.state.Load()
	if curr >= int32(state) {
		return fmt.Errorf(
			"process state can only go forward: current: %d, desired: %d",
			curr, state,
		)
	}

	this.state.Store(int32(state))
	return nil
}

func (this *ExecProc) flush(b *Broadcaster, out io.ReadCloser) {
	err := b.Run(out)
	if err == os.ErrClosed {
		return
	}

	if err != nil {
		fmt.Println("process: broadcaster:", err)
	}

	out.Close()
}

var _ Proc = (*ExecProc)(nil)

func NewExecProc(name string, args ...string) *ExecProc {
	return &ExecProc{
		Name:  name,
		Args:  args,
		Stdin: nil,
		Cwd:   "",
		Uid:   uint32(syscall.Getuid()),
		Gid:   uint32(syscall.Getgid()),
		Env:   nil,

		state:  atomic.Int32{},
		stdout: NewBroadcaster(),
		stderr: NewBroadcaster(),
	}
}
