package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"syscall"

	"github.com/thekhanj/ella/common"
	"github.com/thekhanj/ella/config"
)

type Daemon struct {
	log bool
}

func (this *Daemon) Run(ctx context.Context, cfgPath string) int {
	var c config.Config

	err := config.ReadConfig(cfgPath, &c)
	if err != nil {
		fmt.Println("error: invalid config:", err)
		return CODE_INVALID_CONFIG
	}

	pidFile := this.getPidFile(c.PidFile)
	err = this.writePid(pidFile)
	if err != nil {
		fmt.Println("error: failed creating pid file:", err)
		return CODE_INITIALIZATION_FAILED
	}

	services, code := this.getServices(&c)
	if code != CODE_SUCCESS {
		return code
	}

	this.runAllServices(ctx, services)

	return CODE_SUCCESS
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

func (this *Daemon) getPidFile(pidFile *string) string {
	if pidFile != nil {
		return *pidFile
	}

	if uid := syscall.Getuid(); uid == 0 {
		return "/var/run/ella/main.pid"
	} else {
		return fmt.Sprintf("/var/run/user/%d/ella/main.pid", uid)
	}
}

func (this *Daemon) runAllServices(
	ctx context.Context, services []*Service,
) {
	var wg sync.WaitGroup
	wg.Add(len(services))
	for _, s := range services {
		go func() {
			defer wg.Done()

			this.runService(ctx, s)
		}()
	}
	wg.Wait()
}

func (this *Daemon) runService(ctx context.Context, s *Service) {
	if this.log {
		go common.FlushWithContext(s.Name, os.Stdout, s.StdoutPipe())
		go common.FlushWithContext(s.Name, os.Stderr, s.StderrPipe())
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
