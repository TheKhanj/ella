package main

import (
	"context"
	"errors"
	"io"
	"log"
	"sync"
	"sync/atomic"

	"github.com/cskr/pubsub/v2"
	"github.com/thekhanj/ella/config"
)

//go:generate stringer -type=ServiceSignal
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

//go:generate stringer -type=ServiceSignal
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
	Name string

	createProc func() (*Proc, error)
	running    atomic.Bool
	signals    chan ServiceSignal

	state ServiceState
	bus   *pubsub.PubSub[ServiceTopic, ServiceState]

	// TODO: get a lock on this, it's needed considering pipes
	mu    sync.RWMutex
	proc  *Proc
	procs *pubsub.PubSub[int, *Proc]
	wd    Watchdog

	stopAction   ProcAction
	reloadAction ProcAction
}

func (this *Service) Run(ctx context.Context, startHook func()) error {
	defer func() {
		this.bus.Shutdown()
		this.procs.Shutdown()
	}()

	if this.running.Load() == true {
		return ServiceErrAlreadyRunning
	}

	this.running.Store(true)
	go startHook()

	go func() {
		for signal := range this.signals {
			this.handleSignal(ctx, signal)
		}
	}()

	<-ctx.Done()
	this.running.Store(false)
	this.signals <- ServiceSigStop
	close(this.signals)
	return nil
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

func (this *Service) StdoutPipe() io.ReadCloser {
	return this.pipe(func(proc *Proc) io.ReadCloser {
		return proc.StdoutPipe()
	})
}

func (this *Service) StderrPipe() io.ReadCloser {
	return this.pipe(func(proc *Proc) io.ReadCloser {
		return proc.StderrPipe()
	})
}

func (this *Service) Signal(signal ServiceSignal) error {
	if !this.running.Load() {
		return ServiceErrNotRunning
	}

	this.signals <- signal
	return nil
}

func (this *Service) pipe(factory func(proc *Proc) io.ReadCloser) io.ReadCloser {
	r, w := io.Pipe()
	go func() {
		defer w.Close()

		procs := this.procs.Sub(0)

		proc := this.getProc()
		if proc != nil {
			_, err := io.Copy(w, factory(proc))
			if err != nil {
				log.Println("service:", err)
				return
			}
		}

		for proc := range procs {
			_, err := io.Copy(w, factory(proc))
			if err != nil {
				log.Println("service:", err)
				return
			}
		}
	}()

	return r
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

func (this *Service) getProc() *Proc {
	this.mu.RLock()
	defer this.mu.RUnlock()

	return this.proc
}

func (this *Service) setProc(proc *Proc) {
	this.mu.Lock()
	defer this.mu.Unlock()

	this.proc = proc
	if proc != nil {
		this.procs.Pub(proc, 0)
	}
}

func (this *Service) start(ctx context.Context) {
	this.setState(ServiceStateActivating)
	// TODO: start dependencies

	if this.createProc == nil {
		this.Signal(ServiceSigStartDone)
		return
	}

	proc, err := this.createProc()
	if err != nil {
		log.Println("service:", err)
		this.Signal(ServiceSigFail)
		return
	}
	this.setProc(proc)
	ch := this.wd.Watch(proc)

	go func() {
		err := proc.Run(ctx)
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
	proc := this.getProc()
	this.wd.Unwatch(proc)
	// TODO: stop dependencies
	err := this.stopAction.Exec(proc)
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
	proc := this.getProc()
	if proc == nil {
		this.Signal(ServiceSigReloadDone)
		return
	}

	err := this.reloadAction.Exec(proc)
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
	this.setProc(nil)
}

func NewService(
	name string,
	stopAction ProcAction,
	reloadAction ProcAction,
	watchdog Watchdog,
	createProc func() (*Proc, error),
) *Service {
	return &Service{
		Name: name,

		createProc: createProc,
		running:    atomic.Bool{},
		signals:    make(chan ServiceSignal),

		state: ServiceStateInactive,
		bus:   pubsub.New[ServiceTopic, ServiceState](0),

		mu:    sync.RWMutex{},
		proc:  nil,
		procs: pubsub.New[int, *Proc](0),
		wd:    watchdog,

		stopAction:   stopAction,
		reloadAction: reloadAction,
	}
}

func NewServiceFromConfig(cfg *config.Service) (*Service, error) {
	parts, err := ParseCommandLine(string(cfg.Process.Exec))
	if err != nil {
		return nil, err
	}
	wdCfg, err := cfg.Process.GetWatchdog()
	if err != nil {
		return nil, err
	}
	wd, err := NewWatchdogFromConfig(wdCfg)
	if err != nil {
		return nil, err
	}
	stopCfg, err := cfg.Process.GetStop()
	if err != nil {
		return nil, err
	}
	stop, err := NewStopProcActionFromConfig(stopCfg)
	if err != nil {
		return nil, err
	}
	reloadCfg, err := cfg.Process.GetReload()
	if err != nil {
		return nil, err
	}
	reload, err := NewReloadProcActionFromConfig(reloadCfg)
	if err != nil {
		return nil, err
	}

	uid, err := cfg.Process.GetUid()
	if err != nil {
		return nil, err
	}
	gid, err := cfg.Process.GetGid()
	if err != nil {
		return nil, err
	}
	env, err := cfg.Process.GetEnv()
	if err != nil {
		return nil, err
	}

	return NewService(
		cfg.Name,
		stop, reload, wd,
		func() (*Proc, error) {
			stdin, err := cfg.Process.GetStdin()
			if err != nil {
				return nil, err
			}
			proc := NewProc(parts[0], parts[1:]...)
			proc.Cwd = string(cfg.Process.Cwd)
			proc.Uid = uid
			proc.Gid = gid
			if stdin != nil {
				proc.Stdin = stdin
			}
			proc.Env = env

			return proc, nil
		},
	), nil
}
