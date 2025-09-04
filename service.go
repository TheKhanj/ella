package main

import (
	"context"
	"errors"
	"io"
	"log"
	"sync"
	"sync/atomic"

	"github.com/cskr/pubsub/v2"
	"github.com/thekhanj/ella/common"
	"github.com/thekhanj/ella/config"
)

// TODO: maybe have SignalChannel type? with .Close, and return
// it in Signal method?
//
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

//go:generate stringer -type=ServiceState
type ServiceState int

const (
	ServiceStateInactive ServiceState = iota
	ServiceStateActivating
	ServiceStateActive
	ServiceStateReloading
	ServiceStateDeactivating
	ServiceStateFailed
)

type ssmap = map[ServiceSignal][]ServiceState

var ServiceSignalOn ssmap = ssmap{
	ServiceSigStart:      {ServiceStateFailed, ServiceStateInactive},
	ServiceSigStartDone:  {ServiceStateActivating},
	ServiceSigStop:       {ServiceStateActive},
	ServiceSigStopDone:   {ServiceStateDeactivating},
	ServiceSigFail:       {ServiceStateActivating, ServiceStateActive, ServiceStateDeactivating},
	ServiceSigReload:     {ServiceStateActive},
	ServiceSigReloadDone: {ServiceStateReloading},
}

func (this ServiceState) IsStopped() bool {
	return this == ServiceStateInactive || this == ServiceStateFailed
}

func (this ServiceState) IsSignalAllowed(signal ServiceSignal) bool {
	for _, valid := range ServiceSignalOn[signal] {
		if valid == this {
			return true
		}
	}

	return false
}

type ServiceTopic int

const serviceTopic ServiceTopic = 0

var (
	ServiceErrAlreadyRunning     = errors.New("already running")
	ServiceErrNotRunning         = errors.New("not running")
	ServiceErrProcAlreadyRunning = errors.New("process already running")
	ServiceErrNotAProc           = errors.New("service is not a process")
	ServiceErrNeverStopped       = errors.New("service never stopped")
)

type Service struct {
	Name string

	createProc func() (*Proc, error)
	running    atomic.Bool
	signals    chan ServiceSignal

	state atomic.Int32
	bus   *pubsub.PubSub[ServiceTopic, ServiceState]

	mu     sync.RWMutex
	ctx    context.Context
	cancel func()
	proc   *Proc
	procs  *pubsub.PubSub[int, *Proc]
	wd     Watchdog

	stopAction   ProcAction
	reloadAction ProcAction
}

func (this *Service) Run(ctx context.Context, startHook func()) error {
	if this.running.Load() == true {
		return ServiceErrAlreadyRunning
	}
	this.running.Store(true)

	go startHook()

	signalsDone := make(chan struct{})
	go func() {
		defer close(signalsDone)

		for signal := range this.signals {
			this.handleSignal(signal)
		}
	}()

	<-ctx.Done()
	return this.shutdown(signalsDone)
}

func (this *Service) Sub() chan ServiceState {
	ret := this.bus.Sub(serviceTopic)
	return ret
}

func (this *Service) Unsub(ch chan ServiceState) {
	this.bus.Unsub(ch)
}

func (this *Service) StdoutPipe() (io.ReadCloser, error) {
	if this.createProc == nil {
		return nil, ServiceErrNotAProc
	}

	return this.pipe(func(proc *Proc) io.ReadCloser {
		return proc.StdoutPipe()
	}), nil
}

func (this *Service) StderrPipe() (io.ReadCloser, error) {
	if this.createProc == nil {
		return nil, ServiceErrNotAProc
	}

	return this.pipe(func(proc *Proc) io.ReadCloser {
		return proc.StderrPipe()
	}), nil
}

func (this *Service) Signal(signal ServiceSignal) error {
	if !this.running.Load() {
		return ServiceErrNotRunning
	}

	this.signals <- signal
	return nil
}

func (this *Service) shutdown(signalsDone chan struct{}) error {
	ch := this.Sub()
	states := common.ChWithInitial(ch, this.GetState())
	this.Signal(ServiceSigStop)

	state := common.WaitForFn(
		states, func() {
			this.Unsub(ch)
		},
		func(state ServiceState) bool {
			return state.IsStopped()
		},
	)
	if state == nil || !state.IsStopped() {
		// unreachable code
		return ServiceErrNeverStopped
	}

	this.running.Store(false)
	close(this.signals)
	<-signalsDone
	this.bus.Shutdown()
	this.procs.Shutdown()

	return nil
}

func (this *Service) pipe(factory func(proc *Proc) io.ReadCloser) io.ReadCloser {
	r, w := io.Pipe()
	go func() {
		defer w.Close()

		ch := this.procs.Sub(0)
		defer this.procs.Unsub(ch)
		procs := common.ChWithInitial(ch, this.getProc())

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

func (this *Service) GetState() ServiceState {
	return ServiceState(this.state.Load())
}

func (this *Service) setState(state ServiceState) {
	this.state.Store(int32(state))
	this.bus.Pub(state, serviceTopic)
}

// Sometimes we do intentionally block the runtime to prevent other
// signals from taking precedence.
func (this *Service) handleSignal(signal ServiceSignal) {
	state := this.GetState()
	if !state.IsSignalAllowed(signal) {
		log.Printf(
			"service: signal \"%s\" not allowed on state \"%s\"\n",
			signal, state,
		)
		return
	}

	switch signal {
	case ServiceSigStart:
		this.start()
	case ServiceSigStartDone:
		this.startDone()
	case ServiceSigStop:
		this.stop()
	case ServiceSigStopDone:
		this.stopDone()
	case ServiceSigFail:
		this.fail()
	case ServiceSigReload:
		this.reload()
	case ServiceSigReloadDone:
		this.reloadDone()
	}
}

func (this *Service) getProc() *Proc {
	this.mu.RLock()
	defer this.mu.RUnlock()

	return this.proc
}

func (this *Service) setProc(proc *Proc) {
	this.mu.Lock()
	if proc != nil && this.proc != nil {
		panic(ServiceErrProcAlreadyRunning)
	}
	this.proc = proc
	this.mu.Unlock()

	if proc != nil {
		this.procs.Pub(proc, 0)
	}
}

func (this *Service) cleanProc() {
	this.ctx = nil
	this.cancel()
	this.setProc(nil)
}

func (this *Service) handleWatchdogEvents(ch chan WatchdogSignal) {
	for sig := range ch {
		// TODO: maybe think a bit more about this :3
		switch sig {
		case WatchdogSigStarted:
			this.Signal(ServiceSigStartDone)
		case WatchdogSigFailed:
			this.Signal(ServiceSigFail)
		}
	}
}

func (this *Service) start() {
	this.setState(ServiceStateActivating)

	if this.createProc == nil {
		this.handleSignal(ServiceSigStartDone)
		return
	}

	proc, err := this.createProc()
	if err != nil {
		log.Println("service:", err)
		this.handleSignal(ServiceSigFail)
		return
	}
	this.setProc(proc)
	ch := this.wd.Watch(proc)

	// TODO: should i use context.background?
	// it's just here to not need to get a lock on it
	this.ctx, this.cancel = context.WithCancel(context.Background())
	go func() {
		err := proc.Run(this.ctx)
		if err != nil {
			log.Println("service:", err)
		}
	}()

	go this.handleWatchdogEvents(ch)
}

func (this *Service) startDone() {
	this.setState(ServiceStateActive)
}

func (this *Service) stop() {
	this.setState(ServiceStateDeactivating)

	proc := this.getProc()
	if proc == nil {
		this.handleSignal(ServiceSigStopDone)
		return
	}

	this.wd.Unwatch(proc)

	err := this.stopAction.Exec(proc)
	if err != nil {
		log.Println("service:", err)
		this.handleSignal(ServiceSigFail)
	} else {
		this.handleSignal(ServiceSigStopDone)
	}
}

func (this *Service) stopDone() {
	this.setState(ServiceStateInactive)
	this.cleanProc()
}

func (this *Service) reload() {
	this.setState(ServiceStateReloading)

	proc := this.getProc()
	if proc == nil {
		this.handleSignal(ServiceSigReloadDone)
		return
	}

	err := this.reloadAction.Exec(proc)
	if err != nil {
		log.Println("service:", err)
	}

	this.handleSignal(ServiceSigReloadDone)
}

func (this *Service) reloadDone() {
	this.setState(ServiceStateActive)
}

func (this *Service) fail() {
	this.setState(ServiceStateFailed)
	this.cleanProc()
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

		state: atomic.Int32{},
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
