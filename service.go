// MIT License
// Copyright (c) 2025 Pooyan Khanjankhani

package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"sync"
	"sync/atomic"

	"github.com/cskr/pubsub/v2"
	"github.com/thekhanj/ella/common"
	"github.com/thekhanj/ella/config"
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

func (this ServiceState) IsStopped() bool {
	return this == ServiceStateInactive || this == ServiceStateFailed
}

var (
	ServiceErrAlreadyRunning = errors.New("service already running")
	ServiceErrAlreadyStopped = errors.New("service already stopped")
	ServiceErrFailed         = errors.New("service failed")
	ServiceErrNotActive      = errors.New("service is not active")
)

type Service struct {
	Name     string
	Watchdog Watchdog

	logB      *Broadcaster
	logStdout bool
	logStderr bool
	log       *log.Logger

	running atomic.Bool
	state   atomic.Int32
	bus     *pubsub.PubSub[int, ServiceState]

	// Ensure the watchdog doesn't leave the service in an inconsistent state,
	// for example when the process crashes in the middle of reload operation.
	atomicAction sync.Mutex
}

func (this *Service) Run(ctx context.Context) {
	var wg sync.WaitGroup
	wg.Add(1)

	r, w := io.Pipe()

	this.log = log.New(
		w, fmt.Sprintf("%s: ", this.Name), 0,
	)

	go func() {
		defer wg.Done()
		defer w.Close()

		err := this.logB.Run(r)
		if err != nil {
			fmt.Printf("%s: logger stopped: %s\n", this.Name, err)
		}
	}()

	<-ctx.Done()
	r.Close()
	wg.Wait()
	if this.Watchdog != nil {
		this.Watchdog.Procs().Shutdown()
	}
}

func (this *Service) Start() error {
	this.atomicAction.Lock()
	defer this.atomicAction.Unlock()

	return this.start()
}

func (this *Service) Stop() error {
	this.atomicAction.Lock()
	defer this.atomicAction.Unlock()

	return this.stop()
}

func (this *Service) Reload() error {
	this.atomicAction.Lock()
	defer this.atomicAction.Unlock()

	return this.reload()
}

func (this *Service) Restart() error {
	this.atomicAction.Lock()
	defer this.atomicAction.Unlock()

	if !this.GetState().IsStopped() {
		err := this.stop()
		if err != nil {
			return err
		}
	}

	return this.start()
}

func (this *Service) Logs() io.ReadCloser {
	r, w := io.Pipe()
	this.logB.Add(w)

	readers := []io.ReadCloser{r}
	var stdout, stderr io.ReadCloser = nil, nil
	if this.logStdout {
		stdout = this.Watchdog.Procs().StdoutPipe()
		r, w := io.Pipe()
		go func() {
			common.FlushWithContext(
				fmt.Sprintf("%s[stdout]:", this.Name),
				w, stdout,
			)
		}()
		readers = append(readers, r)
	}
	if this.logStderr {
		stderr = this.Watchdog.Procs().StderrPipe()
		r, w := io.Pipe()
		go func() {
			common.FlushWithContext(
				fmt.Sprintf("%s[stderr]:", this.Name),
				w, stderr,
			)
		}()
		readers = append(readers, r)
	}

	return common.StreamLines(readers...)
}

func (this *Service) GetState() ServiceState {
	return ServiceState(this.state.Load())
}

func (this *Service) setState(state ServiceState) {
	this.state.Store(int32(state))
	go this.bus.Pub(state, 0)
}

func (this *Service) handleWatchdogSignals(
	sigs chan WatchdogSignal,
) error {
	sig, ok := <-sigs
	if !ok {
		panic("unreachable code")
	}
	err := this.handleSingleWdSignal(sig)
	if err != nil {
		return err
	}

	go func() {
		for sig := range sigs {
			this.handleSingleWdSignal(sig)
		}
	}()

	return nil
}

func (this *Service) handleSingleWdSignal(sig WatchdogSignal) error {
	this.atomicAction.Lock()
	defer this.atomicAction.Unlock()

	switch sig {
	case WatchdogSigStarted:
		this.startDone()
		return nil
	case WatchdogSigStopped:
		// TODO: think?
		return nil
	case WatchdogSigFailed:
		this.fail()
		return ServiceErrFailed
	default:
		return errors.ErrUnsupported
	}
}

func (this *Service) start() error {
	if !this.GetState().IsStopped() {
		return ServiceErrAlreadyRunning
	}
	this.log.Print("starting")

	this.setState(ServiceStateActivating)

	if this.Watchdog == nil {
		this.startDone()
		return nil
	}

	sigs, err := this.Watchdog.Start()
	if err != nil {
		this.fail()
		return err
	}

	go this.handleWatchdogSignals(sigs)

	return nil
}

func (this *Service) startDone() {
	this.log.Print("started")
	this.setState(ServiceStateActive)
}

func (this *Service) stop() error {
	if this.GetState().IsStopped() {
		return ServiceErrAlreadyStopped
	}
	this.log.Print("stopping")

	this.setState(ServiceStateDeactivating)

	if this.Watchdog == nil {
		this.stopDone()
		return nil
	}

	err := this.Watchdog.Stop()
	if err != nil {
		this.fail()
		return err
	}

	this.stopDone()
	return nil
}

func (this *Service) stopDone() {
	this.log.Print("stopped")
	this.setState(ServiceStateInactive)
}

func (this *Service) reload() error {
	if this.GetState() != ServiceStateActive {
		return ServiceErrNotActive
	}
	this.log.Print("reloading")

	this.setState(ServiceStateReloading)

	if this.Watchdog == nil {
		this.reloadDone()
		return nil
	}

	err := this.Watchdog.Reload()
	this.reloadDone()
	if err != nil {
		return err
	}

	return nil
}

func (this *Service) reloadDone() {
	this.log.Print("reloaded")
	this.setState(ServiceStateActive)
}

func (this *Service) fail() {
	this.log.Print("failed")
	this.setState(ServiceStateFailed)
}

func NewService(
	name string,
	watchdog Watchdog,
	logStdout, logStderr bool,
) *Service {
	return &Service{
		Name:     name,
		Watchdog: watchdog,

		logB:      NewBroadcaster(),
		logStdout: logStdout,
		logStderr: logStderr,
		log:       nil,

		running: atomic.Bool{},
		state:   atomic.Int32{},
		bus:     pubsub.New[int, ServiceState](0),

		atomicAction: sync.Mutex{},
	}
}

func NewServiceFromConfig(cfg *config.Service) (*Service, error) {
	parts, err := ParseCommandLine(string(cfg.Process.Exec))
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

	createProc := func(path string, args ...string) *Proc {
		proc := NewProc(path, args...)

		proc.Cwd = string(cfg.Process.Cwd)
		proc.Uid = uid
		proc.Gid = gid
		proc.Env = env

		return proc
	}
	exec := func() (*Proc, error) {
		stdin, err := cfg.Process.GetStdin()
		if err != nil {
			return nil, err
		}
		proc := createProc(parts[0], parts[1:]...)
		if stdin != nil {
			proc.Stdin = stdin
		}

		return proc, nil
	}

	// TODO: handle empty watchdog, target files you know...
	wdCfg, err := cfg.Process.GetWatchdog()
	if err != nil {
		return nil, err
	}
	wd, err := NewWatchdogFromConfig(wdCfg, exec, stop, reload)
	if err != nil {
		return nil, err
	}

	return NewService(
		cfg.Name, wd,
		// TODO: handle target files...
		bool(cfg.Process.Stdout), bool(cfg.Process.Stderr),
	), nil
}
