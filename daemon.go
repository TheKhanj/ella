package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"syscall"

	"github.com/thekhanj/ella/common"
	"github.com/thekhanj/ella/config"
)

type Daemon struct {
	running  atomic.Bool
	log      bool
	services []*Service
}

func (this *Daemon) Service(name string) (*Service, error) {
	for _, s := range this.services {
		if s.Name == name {
			return s, nil
		}
	}

	return nil, fmt.Errorf("service not found: %s", name)
}

func (this *Daemon) Run(ctx context.Context, cfgPath string) int {
	if this.running.Load() {
		fmt.Println("error: daemon already running")
		return CODE_GENERAL_ERR
	}

	this.running.Store(true)

	var c config.Config

	err := config.ReadConfig(cfgPath, &c)
	if err != nil {
		fmt.Println("error: invalid config:", err)
		return CODE_INVALID_CONFIG
	}

	pidFile := config.GetPidFile(c.PidFile)
	err = this.writePid(pidFile)
	if err != nil {
		fmt.Println("error: failed creating pid file:", err)
		return CODE_INITIALIZATION_FAILED
	}

	var code int
	this.services, code = this.getServices(&c)
	if code != CODE_SUCCESS {
		return code
	}

	socket := SocketServer{this.Service}
	err = this.initVarDir()
	if err != nil {
		log.Println("error:", err)
		return CODE_GENERAL_ERR
	}
	common.WaitAny(
		ctx,
		socket.Listen,
		func(ctx context.Context) error {
			this.runAllServices(ctx)
			return nil
		},
	)
	err = this.deinitVarDir()
	if err != nil {
		log.Println("error:", err)
		return CODE_GENERAL_ERR
	}

	return CODE_SUCCESS
}

func (this *Daemon) initVarDir() error {
	return os.MkdirAll(common.GetVarDir(os.Getpid()), 0755)
}

func (this *Daemon) deinitVarDir() error {
	return os.Remove(common.GetVarDir(os.Getpid()))
}

func (this *Daemon) writePid(pidFile string) error {
	err := os.MkdirAll(filepath.Dir(pidFile), 0755)
	if err != nil {
		return err
	}

	return os.WriteFile(
		pidFile,
		[]byte(fmt.Sprintf("%d\n", syscall.Getpid())),
		0655,
	)
}

func (this *Daemon) runAllServices(ctx context.Context) {
	var wg sync.WaitGroup
	wg.Add(len(this.services))
	for _, s := range this.services {
		go func() {
			defer wg.Done()

			this.runService(ctx, s)
		}()
	}
	wg.Wait()
}

func (this *Daemon) runService(ctx context.Context, s *Service) {
	if this.log {
		go common.FlushWithContext(
			s.Name, os.Stdout, s.StdoutPipe(),
		)
		go common.FlushWithContext(
			s.Name, os.Stderr, s.StderrPipe(),
		)
	}

	err := s.Run(ctx, func() {
		err := s.Signal(ServiceSigStart)
		if err != nil {
			log.Println("error:", err)
		}
	})
	if err != nil {
		log.Println("error:", err)
	}
}

func (this *Daemon) getServices(c *config.Config) ([]*Service, int) {
	// TODO: find entry service and run that instead of all services
	serviceCfgs, err := c.GetServices()
	if err != nil {
		log.Println("error:", err)

		return nil, CODE_INVALID_CONFIG
	}
	services := make([]*Service, 0)
	for _, cfg := range serviceCfgs {
		s, err := NewServiceFromConfig(cfg)
		if err != nil {
			log.Println("error:", err)
			return nil, CODE_INITIALIZATION_FAILED
		}
		services = append(services, s)
	}

	return services, CODE_SUCCESS
}
