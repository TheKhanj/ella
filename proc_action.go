// MIT License
// Copyright (c) 2025 Pooyan Khanjankhani

package main

import (
	"errors"
	"fmt"
	"syscall"
	"time"

	"github.com/thekhanj/ella/common"
	"github.com/thekhanj/ella/config"
)

var ProcActionErrNeverStopped error = errors.New("process never stopped")

type ProcAction interface {
	Exec(proc *Proc) error
}

type StopSignalProcAction struct {
	// TODO: don't forget to add this for exec proc action...
	// createProc CreateProc
	timeout time.Duration
	signal  syscall.Signal
}

func (this *StopSignalProcAction) Exec(proc *Proc) error {
	states := proc.Sub()
	process, err := proc.GetProcess()
	if err != nil {
		return err
	}

	err = process.Signal(this.signal)
	if err != nil {
		return err
	}

	stopped := make(chan struct{})
	go func() {
		defer close(stopped)

		common.WaitFor(
			states, func() { proc.Unsub(states) },
			ProcStateStopped,
		)
	}()

	select {
	case <-time.After(this.timeout):
		err := process.Kill()
		if err != nil {
			return err
		}

		<-stopped
		return nil
	case <-stopped:
		return nil
	}
}

func NewStopProcActionFromConfig(cfg config.StopProcAction) (ProcAction, error) {
	if stop, ok := cfg.(*config.StopSignalProcAction); ok {
		timeout, err := time.ParseDuration(string(stop.Timeout))
		if err != nil {
			return nil, err
		}
		return &StopSignalProcAction{
			timeout: timeout,
			signal:  stop.Code.GetSignal(),
		}, nil
	} else if signal, ok := cfg.(config.ProcActionSignalCode); ok {
		return &StopSignalProcAction{
			timeout: time.Second * 10,
			signal:  signal.GetSignal(),
		}, nil
	} else {
		return nil, fmt.Errorf("invalid stop action config: %v", cfg)
	}
}

var _ (ProcAction) = (*StopSignalProcAction)(nil)

type ReloadSignalProcAction struct {
	signal syscall.Signal
}

func (this *ReloadSignalProcAction) Exec(proc *Proc) error {
	process, err := proc.GetProcess()
	if err != nil {
		return err
	}

	return process.Signal(this.signal)
}

func NewReloadProcActionFromConfig(cfg config.ReloadProcAction) (ProcAction, error) {
	if signal, ok := cfg.(config.ProcActionSignalCode); ok {
		return &ReloadSignalProcAction{
			signal: signal.GetSignal(),
		}, nil
	} else {
		return nil, fmt.Errorf("invalid reload action config: %v", cfg)
	}
}

var _ (ProcAction) = (*ReloadSignalProcAction)(nil)
