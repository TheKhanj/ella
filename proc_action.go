package main

import (
	"errors"
	"os"
	"syscall"
	"time"

	"github.com/thekhanj/ella/common"
)

var ProcActionErrNeverStopped error = errors.New("process never stopped")

type ProcAction interface {
	Exec(proc *os.Process) error
}

type StopProcAction struct {
	proc    *Proc
	timeout time.Duration
	signal  syscall.Signal
}

func (this *StopProcAction) Exec(process *os.Process) error {
	states := this.proc.Sub()
	defer this.proc.Unsub(states)

	err := process.Signal(this.signal)
	if err != nil {
		return err
	}

	stopped := make(chan struct{})
	go func() {
		defer close(stopped)

		neverStopped := !common.WaitFor(states, ProcStateStopped)
		if neverStopped {
			panic(ProcActionErrNeverStopped)
		}
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

var _ (ProcAction) = (*StopProcAction)(nil)

type ReloadProcAction struct {
	signal syscall.Signal
}

func (this *ReloadProcAction) Exec(process *os.Process) error {
	return process.Signal(this.signal)
}

var _ (ProcAction) = (*ReloadProcAction)(nil)
