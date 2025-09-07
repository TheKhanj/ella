package main

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"

	"github.com/thekhanj/ella/config"
)

type WatchdogSignal int

const (
	WatchdogSigStarted WatchdogSignal = iota
	WatchdogSigStopped
	WatchdogSigFailed
)

var WatchdogErrAlreadyRunning = errors.New("an active process is already running")

type Watchdog interface {
	Start() (chan WatchdogSignal, error)
	Stop() error
	Reload() error
	Procs() *Procs
}

func NewWatchdogFromConfig(
	cfg config.Watchdog,
	exec func() (*Proc, error),
	stop, reload ProcAction,
) (Watchdog, error) {
	// TODO: make this watchdog config simpler, no need for this complexity
	if _, ok := cfg.(*config.SimpleWatchdog); ok {
		return NewSimpleWatchdog(exec, stop, reload), nil
	} else {
		return nil, fmt.Errorf("invalid watchdog config: %v", cfg)
	}
}

type SimpleWatchdog struct {
	procs  *Procs
	exec   func() (*Proc, error)
	stop   ProcAction
	reload ProcAction

	running atomic.Bool
	cancel  func()
}

func (this *SimpleWatchdog) Start() (chan WatchdogSignal, error) {
	if this.running.Load() {
		return nil, WatchdogErrAlreadyRunning
	}
	proc, err := this.exec()
	if err != nil {
		return nil, err
	}

	this.running.Store(true)
	go this.procs.Push(proc)

	signals := make(chan WatchdogSignal)
	ctx, cancel := context.WithCancel(context.Background())
	this.cancel = cancel

	go this.run(ctx, proc, signals)

	return signals, nil
}

func (this *SimpleWatchdog) Stop() error {
	proc, err := this.procs.Last()
	if err != nil {
		return err
	}
	this.cancel()
	this.running.Store(false)

	return this.stop.Exec(proc)
}

func (this *SimpleWatchdog) Reload() error {
	proc, err := this.procs.Last()
	if err != nil {
		return err
	}

	return this.reload.Exec(proc)
}

func (this *SimpleWatchdog) Procs() *Procs {
	return this.procs
}

func (this *SimpleWatchdog) run(
	ctx context.Context,
	proc *Proc, signals chan WatchdogSignal,
) {
	states := proc.Sub()
	defer func() {
		proc.Unsub(states)
		close(signals)
		this.running.Store(false)
	}()

	go func() {
		err := proc.Run(ctx)
		if err != nil {
			fmt.Println("watchdog: process:", err)
		}
	}()

	for state := range states {
		if state == ProcStateStarted {
			// TODO: think about coroutine or not
			this.signal(signals, WatchdogSigStarted)
		}
		if state == ProcStateStopped {
			code, err := proc.GetExitCode()
			if err != nil {
				panic("unreachable code")
			}

			if code == 0 || this.running.Load() == false {
				this.signal(signals, WatchdogSigStopped)
			} else {
				this.signal(signals, WatchdogSigFailed)
			}
		}
	}
}

func (this *SimpleWatchdog) signal(
	sigs chan WatchdogSignal, sig WatchdogSignal,
) {
	sigs <- sig
}

var _ Watchdog = (*SimpleWatchdog)(nil)

func NewSimpleWatchdog(
	exec func() (*Proc, error),
	stop, reload ProcAction,
) *SimpleWatchdog {
	return &SimpleWatchdog{
		procs:  NewProcs(),
		exec:   exec,
		stop:   stop,
		reload: reload,

		running: atomic.Bool{},
	}
}
