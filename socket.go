// MIT License
// Copyright (c) 2025 Pooyan Khanjankhani

package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"

	"github.com/google/shlex"
	"github.com/thekhanj/ella/common"
)

type SocketServer struct {
	getService func(name string) (*Service, error)
	services   []*Service
}

func (this *SocketServer) Listen(ctx context.Context) error {
	socketPath := this.getSocketPath()
	os.Remove(socketPath)

	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		return err
	}

	go func() {
		<-ctx.Done()
		listener.Close()
	}()

	for {
		conn, err := listener.Accept()
		if err != nil {
			return err
		}

		go this.handleConnection(conn)
	}
}

func (this *SocketServer) handleConnection(conn net.Conn) {
	defer conn.Close()
	scanner := bufio.NewScanner(conn)

	for scanner.Scan() {
		cmd := scanner.Text()
		this.handleCommand(conn, cmd)
	}
}

func (this *SocketServer) handleCommand(w io.Writer, cmdLine string) {
	parts, err := shlex.Split(cmdLine)
	if err != nil {
		fmt.Fprintf(w, "error: parsing command line failed: %s\n", err)
		return
	}

	cmd := parts[0]
	args := parts[1:]

	handlers := []func(io.Writer, string, []string) (error, bool){
		this.handleLogsCommand,
		this.handleServicesCommand,
		this.handleListCommand,
	}

	for _, h := range handlers {
		err, handled := h(w, cmd, args)
		if !handled {
			continue
		}

		if err != nil {
			fmt.Fprintf(w, "error: %s\n", err)
		}
		return
	}

	fmt.Fprintf(w, "error: invalid command: %s\n", cmd)
}

func (this *SocketServer) handleListCommand(
	w io.Writer, cmd string, extraArgs []string,
) (error, bool) {
	if cmd != "list" {
		return nil, false
	}

	if len(extraArgs) != 0 {
		return fmt.Errorf("extra argument: %s", extraArgs[0]), true
	}

	for _, s := range this.services {
		fmt.Fprintln(w, s.Name)
	}
	return nil, true
}

var socketServiceActions = map[string]func(*Service) error{
	"start":   func(s *Service) error { return s.Start() },
	"stop":    func(s *Service) error { return s.Stop() },
	"restart": func(s *Service) error { return s.Restart() },
	"reload":  func(s *Service) error { return s.Reload() },
}

func (this *SocketServer) handleServicesCommand(
	w io.Writer, cmd string, services []string,
) (error, bool) {
	fn, ok := socketServiceActions[cmd]
	if !ok {
		return nil, false
	}

	return this.runServicesAction(w, services, fn), true
}

func (this *SocketServer) handleLogsCommand(
	w io.Writer, cmd string, services []string,
) (error, bool) {
	if cmd != "logs" {
		return nil, false
	}

	return this.showLogs(w, services), true
}

func (this *SocketServer) runServicesAction(
	w io.Writer, services []string,
	actionFn func(s *Service) error,
) error {
	ss, err := this.getServices(services)
	if err != nil {
		return err
	}

	var wg sync.WaitGroup
	wg.Add(len(ss))
	for _, s := range ss {
		go func() {
			defer wg.Done()

			err := actionFn(s)
			if err != nil {
				fmt.Fprintf(w, "%s: %s\n", s.Name, err)
			}
		}()
	}
	wg.Wait()
	return nil
}

func (this *SocketServer) showLogs(
	w io.Writer, serviceNames []string,
) error {
	services, err := this.getServices(serviceNames)
	if err != nil {
		return err
	}

	readers := make([]io.ReadCloser, 0)
	for _, s := range services {
		readers = append(readers, s.Logs())
	}
	logs := common.StreamLines(readers...)
	defer logs.Close()

	_, err = io.Copy(w, logs)
	return err
}

func (this *SocketServer) getServices(
	services []string,
) ([]*Service, error) {
	ss := make([]*Service, 0)
	for _, name := range services {
		s, err := this.getService(name)
		if err != nil {
			return nil, err
		}
		ss = append(ss, s)
	}

	return ss, nil
}

func (this *SocketServer) getSocketPath() string {
	return filepath.Join(common.GetVarDir(syscall.Getpid()), "ella.sock")
}

type SocketClient struct {
	pid int
}

func (this *SocketClient) RunCommand(
	ctx context.Context, w io.Writer, cmd string, services ...string,
) error {
	conn, err := this.openConn()
	if err != nil {
		return err
	}
	defer conn.Close()

	cmdLine := this.getCmd(cmd, services)
	_, err = conn.Write([]byte(cmdLine))
	if err != nil {
		return err
	}
	conn.(*net.UnixConn).CloseWrite()

	copied := make(chan struct{})
	go func() {
		defer close(copied)

		io.Copy(w, conn)
	}()

	select {
	case <-ctx.Done():
		return nil
	case <-copied:
		return nil
	}
}

func (this *SocketClient) getCmd(cmd string, services []string) string {
	shellEscaped := make([]string, 0)
	for _, s := range services {
		shellEscaped = append(shellEscaped, common.ShellEscape(s))
	}

	return fmt.Sprintf("%s %s\n", cmd, strings.Join(shellEscaped, " "))
}

func (this *SocketClient) openConn() (net.Conn, error) {
	conn, err := net.Dial("unix", this.getSocketPath())
	if err != nil {
		return nil, err
	}

	return conn, nil
}

func (this *SocketClient) getSocketPath() string {
	return filepath.Join(common.GetVarDir(this.pid), "ella.sock")
}
