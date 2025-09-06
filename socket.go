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
	default:
		fmt.Fprintf(w, "error: invalid command: %s\n", cmd)
	}
}

func (this *SocketServer) showLogs(
	w io.Writer, serviceNames []string,
) error {
	services := make([]*Service, 0)
	for _, name := range serviceNames {
		s, err := this.getService(name)
		if err != nil {
			return err
		}
		services = append(services, s)
	}

	readers := make([]io.ReadCloser, 0)
	for _, s := range services {
		readers = append(readers, s.Logs())
	}
	logs := common.StreamLines(readers...)
	defer logs.Close()

	_, err := io.Copy(w, logs)
	return err
}

func (this *SocketServer) getSocketPath() string {
	return filepath.Join(common.GetVarDir(syscall.Getpid()), "ella.sock")
}

type SocketClient struct {
	pid int
}

func (this *SocketClient) Logs(
	ctx context.Context, w io.Writer, services ...string,
) error {
	conn, err := this.openConn()
	if err != nil {
		return err
	}
	defer conn.Close()

	cmd := this.getCmd(services)
	_, err = conn.Write([]byte(cmd))
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

func (this *SocketClient) getCmd(services []string) string {
	shellEscaped := make([]string, 0)
	for _, s := range services {
		shellEscaped = append(shellEscaped, common.ShellEscape(s))
	}

	return fmt.Sprintf("logs %s\n", strings.Join(shellEscaped, " "))
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
