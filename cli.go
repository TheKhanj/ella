package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/thekhanj/ella/common"
	"github.com/thekhanj/ella/config"
)

var VERSION = "dev"

const (
	CODE_SUCCESS int = iota
	CODE_GENERAL_ERR
	CODE_INVALID_CONFIG
	CODE_INVALID_INVOKATION
	CODE_INITIALIZATION_FAILED
)

type Cli struct {
	args []string
}

func (this *Cli) Exec() int {
	f := flag.NewFlagSet("ella", flag.ExitOnError)
	help := f.Bool("h", false, "show help")
	version := f.Bool("v", false, "show version")

	f.Usage = func() {
		fmt.Println("Usage:")
		fmt.Println("  ella -h")
		fmt.Println()
		fmt.Println("Available Commands:")
		fmt.Println("  run       Run the daemon")
		fmt.Println("  logs      Run the daemon")
		fmt.Println("  start     Start services")
		fmt.Println("  stop      Stop services")
		fmt.Println("  restart   Restart services")
		fmt.Println("  reload    Reload services")
		fmt.Println()
		fmt.Println("Flags:")
		f.PrintDefaults()
	}

	f.Parse(this.args)

	if *help {
		f.Usage()

		return CODE_SUCCESS
	}

	if *version {
		fmt.Println(VERSION)

		return CODE_SUCCESS
	}

	if len(f.Args()) == 0 {
		f.Usage()
		fmt.Println("error: not enough arguments")

		return CODE_INVALID_INVOKATION
	}

	cmd := f.Args()[0]
	switch cmd {
	case "run":
		c := RunCli{args: f.Args()[1:]}
		return c.Exec()
	case "logs":
		c := LogsCli{args: f.Args()[1:]}
		return c.Exec()
	case "start":
		c := StartCli{args: f.Args()[1:]}
		return c.Exec()
	case "stop":
		c := StopCli{args: f.Args()[1:]}
		return c.Exec()
	case "restart":
		c := RestartCli{args: f.Args()[1:]}
		return c.Exec()
	case "reload":
		c := ReloadCli{args: f.Args()[1:]}
		return c.Exec()
	default:
		fmt.Printf("error: invalid command \"%s\"\n", cmd)
		return CODE_INVALID_INVOKATION
	}
}

type RunCli struct {
	args []string
}

func (this *RunCli) Exec() int {
	f := flag.NewFlagSet("ella", flag.ExitOnError)
	cfgPath := f.String("c", "ella.json", "config file")
	hideLogs := f.Bool("l", false, "supress logs")
	all := f.Bool("a", false, "start all services")

	f.Usage = func() {
		fmt.Println("Usage:")
		fmt.Println("  ella run -c ella.json [services...]")
		fmt.Println()
		fmt.Println("Flags:")
		f.PrintDefaults()
	}

	f.Parse(this.args)

	ctx := common.NewSignalCtx(context.Background())
	d := Daemon{
		log: !*hideLogs,
	}

	var c config.Config
	err := config.ReadParsedConfig(*cfgPath, &c)
	if err != nil {
		fmt.Println("error: invalid config:", err)
		return CODE_INVALID_CONFIG
	}

	var serviceNames []string
	if *all {
		for _, s := range c.Services {
			serviceNames = append(serviceNames, s.Name)
		}
	} else {
		serviceNames = f.Args()
	}

	return d.Run(ctx, &c, serviceNames)
}

func getDaemonPid(pidFile *string) (int, int) {
	b, err := os.ReadFile(config.GetPidFile(pidFile))
	if err != nil {
		fmt.Println("error:", err)
		return 0, CODE_GENERAL_ERR
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(b)))
	if err != nil {
		fmt.Println("error:", err)
		return 0, CODE_GENERAL_ERR
	}

	return pid, CODE_SUCCESS
}

func runCliAction(
	args []string,
	action string,
	allMsg string,
) int {
	f := flag.NewFlagSet("ella", flag.ExitOnError)
	configPath := f.String("c", "ella.json", "config file")
	all := f.Bool("a", false, allMsg)

	f.Usage = func() {
		fmt.Println("Usage:")
		fmt.Printf("  ella %s -c ella.json -a\n", action)
		fmt.Printf("  ella %s -c ella.json [services...]\n", action)
		fmt.Println()
		fmt.Println("Flags:")
		f.PrintDefaults()
	}

	f.Parse(args)

	var c config.Config

	err := config.ReadParsedConfig(*configPath, &c)
	if err != nil {
		fmt.Println("error: invalid config:", err)
		return CODE_INVALID_CONFIG
	}

	var serviceNames []string
	if *all {
		for _, s := range c.Services {
			serviceNames = append(serviceNames, s.Name)
		}
	} else {
		serviceNames = f.Args()
	}
	if len(serviceNames) == 0 {
		fmt.Println("error: no service name specified")

		return CODE_INVALID_INVOKATION
	}

	ctx := common.NewSignalCtx(context.Background())

	pid, code := getDaemonPid(c.PidFile)
	if code != CODE_SUCCESS {
		return code
	}
	socket := SocketClient{pid}
	err = socket.Action(ctx, os.Stdout, action, serviceNames...)
	if err != nil {
		fmt.Println("error:", err)
		return CODE_GENERAL_ERR
	}

	return CODE_SUCCESS
}

type LogsCli struct {
	args []string
}

func (this *LogsCli) Exec() int {
	return runCliAction(this.args, "logs", "show logs for all services")
}

type StartCli struct {
	args []string
}

func (this *StartCli) Exec() int {
	return runCliAction(this.args, "start", "start all services")
}

type StopCli struct {
	args []string
}

func (this *StopCli) Exec() int {
	return runCliAction(this.args, "stop", "stop all services")
}

type RestartCli struct {
	args []string
}

func (this *RestartCli) Exec() int {
	return runCliAction(this.args, "restart", "restart all services")
}

type ReloadCli struct {
	args []string
}

func (this *ReloadCli) Exec() int {
	return runCliAction(this.args, "reload", "reload all services")
}

func main() {
	c := Cli{args: os.Args[1:]}
	os.Exit(c.Exec())
}
