package main

import (
	"context"
	"errors"
	"log"
	"sync/atomic"

	"github.com/cskr/pubsub/v2"
)

type ServiceSignal int

const (
	ServiceSigStart ServiceSignal = iota
	ServiceSigStartDone
	ServiceSigStop
	ServiceSigStopDone
	ServiceSigFail
	ServiceSigReload
	ServiceSigReloadDone
)

type ServiceState int

const (
	ServiceStateInactive ServiceState = iota
	ServiceStateActive
	ServiceStateFailed
	ServiceStateActivating
	ServiceStateDeactivating
	ServiceStateReloading
)

type ServiceTopic int

const serviceTopic ServiceTopic = 0

var (
	ServiceErrAlreadyRunning = errors.New("already running")
	ServiceErrNotRunning     = errors.New("not running")
)

type Service struct {
	createProc func() *Proc
	running    atomic.Bool
	signals    chan ServiceSignal

	state ServiceState
	bus   *pubsub.PubSub[ServiceTopic, ServiceState]

	proc *Proc
	wd   Watchdog

	stopAction   ProcAction
	reloadAction ProcAction
}

func (this *Service) Run(ctx context.Context) error {
	if this.running.Load() == true {
		return ServiceErrAlreadyRunning
	}

	this.running.Store(true)

	for {
		select {
		case <-ctx.Done():
			this.running.Store(false)
			this.signals <- ServiceSigStop
			close(this.signals)
			for sig := range this.signals {
				this.handleSignal(ctx, sig)
			}
			return nil
		case signal, ok := <-this.signals:
			if !ok {
				return nil
			}

			this.handleSignal(ctx, signal)
		}
	}
}

func (this *Service) Sub() (chan ServiceState, error) {
	if !this.running.Load() {
		return nil, ServiceErrNotRunning
	}

	return this.bus.Sub(serviceTopic), nil
}

func (this *Service) Unsub(ch chan ServiceState) error {
	if !this.running.Load() {
		return ServiceErrNotRunning
	}

	this.bus.Unsub(ch)
	return nil
}

func (this *Service) Signal(signal ServiceSignal) error {
	if !this.running.Load() {
		return ServiceErrNotRunning
	}

	this.signals <- signal
	return nil
}

// Shouldn't be called concurrently!
func (this *Service) setState(state ServiceState) {
	this.state = state
}

func (this *Service) getState() ServiceState {
	return this.state
}

func (this *Service) handleSignal(ctx context.Context, signal ServiceSignal) {
	switch signal {
	case ServiceSigStart:
		this.start(ctx)
	case ServiceSigStartDone:
		this.startDone(ctx)
	case ServiceSigStop:
		this.stop(ctx)
	case ServiceSigStopDone:
		this.stopDone(ctx)
	case ServiceSigFail:
		this.fail(ctx)
	case ServiceSigReload:
		this.reload(ctx)
	case ServiceSigReloadDone:
		this.reloadDone(ctx)
	}
}

func (this *Service) start(ctx context.Context) {
	this.setState(ServiceStateActivating)
	// TODO: start dependencies

	if this.createProc == nil {
		this.Signal(ServiceSigStartDone)
		return
	}

	this.proc = this.createProc()
	ch := this.wd.Watch(this.proc)

	go func() {
		err := this.proc.Run(ctx)
		if err != nil {
			log.Println("service:", err)
		}
	}()

	go func() {
		for sig := range ch {
			switch sig {
			case WatchdogSigStarted:
				this.Signal(ServiceSigStartDone)
			case WatchdogSigFailed:
				this.Signal(ServiceSigFail)
			}
		}
	}()
}

func (this *Service) startDone(ctx context.Context) {
	this.setState(ServiceStateActive)
}

func (this *Service) stop(ctx context.Context) {
	this.setState(ServiceStateDeactivating)
	this.wd.Unwatch(this.proc)
	// TODO: stop dependencies
	process, err := this.proc.GetProcess()
	if err != nil {
		panic(err)
	}
	err = this.stopAction.Exec(process)
	if err != nil {
		log.Println("serivce:", err)
		this.Signal(ServiceSigFail)
	} else {
		this.Signal(ServiceSigStopDone)
	}
}

func (this *Service) stopDone(ctx context.Context) {
	this.setState(ServiceStateInactive)
	this.proc = nil
}

func (this *Service) reload(ctx context.Context) {
	if this.getState() != ServiceStateActive {
		return
	}

	this.setState(ServiceStateReloading)
	// TODO: reload dependencies
	if this.proc == nil {
		this.Signal(ServiceSigReloadDone)
		return
	}

	process, err := this.proc.GetProcess()
	if err != nil {
		panic(err)
	}
	err = this.reloadAction.Exec(process)
	if err != nil {
		log.Println("service:", err)
	}
	this.Signal(ServiceSigReloadDone)
}

func (this *Service) reloadDone(ctx context.Context) {
	if this.getState() != ServiceStateReloading {
		return
	}

	this.setState(ServiceStateActive)
}

func (this *Service) fail(ctx context.Context) {
	this.setState(ServiceStateFailed)
	this.proc = nil
}
