package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"sync/atomic"
	"syscall"

	"github.com/cskr/pubsub/v2"
	"github.com/google/shlex"
)

//go:generate stringer -type=ServiceSignal
type ProcState int

const (
	ProcStateNotStarted ProcState = iota
	ProcStateStarting
	ProcStateStarted
	ProcStateStopped
	ProcStateWaitDone
	ProcStateBusShuttedDown
)

type ProcTopic int

const procTopic ProcTopic = 0

var (
	ProcErrNotStarted = errors.New("not started yet")
	ProcErrNotStopped = errors.New("not stopped yet")
)

type Proc struct {
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

	cmd      *exec.Cmd
	exitCode atomic.Int32

	bus *pubsub.PubSub[ProcTopic, ProcState]
}

func (this *Proc) Run(ctx context.Context) error {
	defer this.Shutdown()

	err := this.setState(ProcStateStarting)
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

	err = this.setState(ProcStateStarted)
	if err != nil {
		return err
	}

	err = this.wait(stdout, stderr)
	if err != nil {
		return err
	}

	return nil
}

func (this *Proc) StdoutPipe() io.ReadCloser {
	return this.pipe(this.stdout)
}

func (this *Proc) StderrPipe() io.ReadCloser {
	return this.pipe(this.stderr)
}

func (this *Proc) GetProcess() (*os.Process, error) {
	if this.GetState() < ProcStateStarted {
		return nil, ProcErrNotStarted
	}

	return this.cmd.Process, nil
}

func (this *Proc) Signal(signal os.Signal) error {
	if this.GetState() < ProcStateStarted {
		return ProcErrNotStarted
	}

	return this.cmd.Process.Signal(signal)
}

func (this *Proc) GetExitCode() (int, error) {
	if this.GetState() < ProcStateStopped {
		return 0, ProcErrNotStopped
	}

	return int(this.exitCode.Load()), nil
}

func (this *Proc) GetState() ProcState {
	return ProcState(this.state.Load())
}

// Subscribe to process's state changes
func (this *Proc) Sub() chan ProcState {
	if !this.checkBus() {
		ch := make(chan ProcState)
		close(ch)

		return ch
	}

	return this.bus.Sub(procTopic)
}

// Unsubscribe from process's state changes
func (this *Proc) Unsub(ch chan ProcState) {
	if this.checkBus() {
		this.bus.Unsub(ch)
	}
}

func (this *Proc) Shutdown() {
	if !this.checkBus() {
		return
	}

	this.bus.Shutdown()
	this.setState(ProcStateBusShuttedDown)
}

func (this *Proc) pipe(b *Broadcaster) io.ReadCloser {
	r, w := io.Pipe()
	b.Add(w)
	return r
}

func (this *Proc) setCmd(ctx context.Context) {
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

func (this *Proc) wait(stdout, stderr io.ReadCloser) error {
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

		this.waitForCmd()
	}()

	wg.Wait()
	return this.setState(ProcStateWaitDone)
}

func (this *Proc) waitForCmd() {
	err := this.cmd.Wait()
	if err != nil {
		fmt.Println("process:", err)

		if exitErr, ok := err.(*exec.ExitError); ok {
			this.exitCode.Store(int32(exitErr.ExitCode()))
		}
	}

	this.setState(ProcStateStopped)
}

func (this *Proc) setState(state ProcState) error {
	curr := this.state.Load()
	if curr >= int32(state) {
		return fmt.Errorf(
			"process state can only go forward: current: %d, desired: %d",
			curr, state,
		)
	}

	this.state.Store(int32(state))
	if this.checkBus() {
		this.bus.Pub(state, procTopic)
	}

	return nil
}

func (this *Proc) flush(b *Broadcaster, out io.ReadCloser) {
	err := b.Run(out)
	if err == os.ErrClosed {
		return
	}

	if err != nil {
		fmt.Println("process: broadcaster:", err)
	}

	out.Close()
}

func (this *Proc) checkBus() bool {
	// The bus has already shutted down, so adding another cmd into it's
	// cmd channel would block the program.
	if this.GetState() >= ProcStateBusShuttedDown {
		return false
	}

	return true
}

func NewProc(name string, args ...string) *Proc {
	return &Proc{
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

		bus: pubsub.New[ProcTopic, ProcState](0),
	}
}

func ParseCommandLine(cmd string) ([]string, error) {
	parts, err := shlex.Split(cmd)
	if err != nil {
		return nil, err
	}
	if len(parts) == 0 {
		return nil, errors.New("empty command line")
	}

	return parts, nil
}
