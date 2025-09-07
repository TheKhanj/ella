package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"syscall"

	"github.com/thekhanj/ella/common"
	"github.com/thekhanj/ella/config"
)

// TODO: this file is becoming shit, clean it up
type Daemon struct {
	running atomic.Bool
	log     bool

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

func (this *Daemon) Run(
	ctx context.Context, c *config.Config, starts []string,
) int {
	if this.running.Load() {
		fmt.Println("error: daemon already running")
		return CODE_GENERAL_ERR
	}

	this.running.Store(true)

	pidFile := config.GetPidFile(c.PidFile)
	err := this.writePid(pidFile)
	if err != nil {
		fmt.Println("error: failed creating pid file:", err)
		return CODE_INITIALIZATION_FAILED
	}

	var code int
	this.services, code = this.getServices(c)
	if code != CODE_SUCCESS {
		return code
	}

	err = this.checkServicesToExist(c, starts)
	if err != nil {
		fmt.Println("error:", err)
		return CODE_INVALID_CONFIG
	}

	socket := SocketServer{this.Service}
	err = this.initVarDir()
	if err != nil {
		fmt.Println("error:", err)
		return CODE_GENERAL_ERR
	}

	go this.startServices(ctx, starts)

	common.WaitAny(
		ctx,
		socket.Listen,
	)
	err = this.deinitVarDir()
	if err != nil {
		fmt.Println("error:", err)
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

func (this *Daemon) checkServicesToExist(
	cfg *config.Config, services []string,
) error {
	for _, serviceName := range services {
		_, err := this.Service(serviceName)
		if err != nil {
			return err
		}
	}

	return nil
}

func (this *Daemon) startServices(ctx context.Context, services []string) {
	var wg sync.WaitGroup
	wg.Add(len(services))
	for _, sName := range services {
		go func() {
			defer wg.Done()

			service, err := this.Service(sName)
			if err != nil {
				panic("unreachable code")
			}
			this.runService(ctx, service)
		}()
	}
	wg.Wait()
}

func (this *Daemon) runService(ctx context.Context, s *Service) {
	var wg sync.WaitGroup
	if this.log {
		wg.Add(1)

		logs := s.Logs()
		go func() {
			<-ctx.Done()

			defer logs.Close()
		}()

		go func() {
			defer wg.Done()

			_, err := io.Copy(os.Stdout, logs)
			if err != nil {
				fmt.Println("daemon:", err)
			}
		}()
	}

	wg.Add(1)
	go func() {
		defer wg.Done()

		err := s.Start()
		if err != nil {
			fmt.Println("error:", err)
		}
	}()

	s.Run(ctx)
	wg.Wait()
}

func (this *Daemon) getServices(c *config.Config) ([]*Service, int) {
	services := make([]*Service, 0)
	for _, cfg := range c.Services {
		s, err := NewServiceFromConfig(&cfg)
		if err != nil {
			fmt.Println("error:", err)
			return nil, CODE_INITIALIZATION_FAILED
		}
		services = append(services, s)
	}

	return services, CODE_SUCCESS
}
