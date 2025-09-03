package main

import (
	"fmt"
	"sync"

	"github.com/thekhanj/ella/config"
)

type WatchdogSignal int

const (
	WatchdogSigStarted WatchdogSignal = iota
	WatchdogSigFailed
)

type Watchdog interface {
	Watch(proc *Proc) chan WatchdogSignal
	Unwatch(proc *Proc)
}

func NewWatchdogFromConfig(cfg config.Watchdog) (Watchdog, error) {
	if _, ok := cfg.(*config.SimpleWatchdog); ok {
		return NewSimpleWatchdog(), nil
	} else {
		return nil, fmt.Errorf("invalid watchdog config: %v", cfg)
	}
}

type SimpleWatchdog struct {
	mu    sync.RWMutex
	procs map[*Proc]chan ProcState
}

func (this *SimpleWatchdog) Watch(
	proc *Proc,
) chan WatchdogSignal {
	states := proc.Sub()
	ch := make(chan WatchdogSignal)
	this.setStates(proc, states)

	go func() {
		defer close(ch)

		for state := range states {
			if state == ProcStateStarted {
				ch <- WatchdogSigStarted
			}
			if state == ProcStateStopped {
				ch <- WatchdogSigFailed
				this.Unwatch(proc)
			}
		}
	}()

	return ch
}

func (this *SimpleWatchdog) Unwatch(proc *Proc) {
	states := this.getStates(proc)
	if states == nil {
		return
	}

	proc.Unsub(states)
	this.unsetStates(proc)
}

func (this *SimpleWatchdog) setStates(proc *Proc, states chan ProcState) {
	this.mu.Lock()
	defer this.mu.Unlock()

	this.procs[proc] = states
}

func (this *SimpleWatchdog) getStates(proc *Proc) chan ProcState {
	this.mu.RLock()
	defer this.mu.RUnlock()

	states, ok := this.procs[proc]
	if !ok {
		return nil
	}

	return states
}

func (this *SimpleWatchdog) unsetStates(proc *Proc) {
	this.mu.Lock()
	defer this.mu.Unlock()

	delete(this.procs, proc)
}

var _ Watchdog = (*SimpleWatchdog)(nil)

func NewSimpleWatchdog() *SimpleWatchdog {
	return &SimpleWatchdog{
		mu:    sync.RWMutex{},
		procs: make(map[*Proc]chan ProcState),
	}
}
