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

	switch cmd {
	case "logs":
		err := this.showLogs(w, parts[1:])
		if err != nil {
			fmt.Fprintf(w, "error: %s\n", err)
		}
	case "start":
		err := this.start(w, parts[1:])
		if err != nil {
			fmt.Fprintf(w, "error: %s\n", err)
		}
	case "stop":
		err := this.stop(w, parts[1:])
		if err != nil {
			fmt.Fprintf(w, "error: %s\n", err)
		}
	case "restart":
		err := this.restart(w, parts[1:])
		if err != nil {
			fmt.Fprintf(w, "error: %s\n", err)
		}
	case "reload":
		err := this.reload(w, parts[1:])
		if err != nil {
			fmt.Fprintf(w, "error: %s\n", err)
		}
	default:
		fmt.Fprintf(w, "error: invalid command: %s\n", cmd)
	}
}

func (this *SocketServer) start(w io.Writer, services []string) error {
	return this.action(
		w, services, func(s *Service) error {
			return s.Start()
		},
	)
}

func (this *SocketServer) stop(w io.Writer, services []string) error {
	return this.action(
		w, services, func(s *Service) error {
			return s.Stop()
		},
	)
}

func (this *SocketServer) restart(w io.Writer, services []string) error {
	return this.action(
		w, services, func(s *Service) error {
			return s.Restart()
		},
	)
}

func (this *SocketServer) reload(w io.Writer, services []string) error {
	return this.action(
		w, services, func(s *Service) error {
			return s.Reload()
		},
	)
}

func (this *SocketServer) action(
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
				fmt.Fprintf(w, "service(%s): %s\n", s.Name, err)
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

func (this *SocketClient) Action(
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
