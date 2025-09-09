// MIT License
// Copyright (c) 2025 Pooyan Khanjankhani

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
		fmt.Fprintln(os.Stderr, "Usage:")
		fmt.Fprintln(os.Stderr, "  ella -h")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Available Commands:")
		fmt.Fprintln(os.Stderr, "  run       run the daemon")
		fmt.Fprintln(os.Stderr, "  logs      run the daemon")
		fmt.Fprintln(os.Stderr, "  start     start services")
		fmt.Fprintln(os.Stderr, "  stop      stop services")
		fmt.Fprintln(os.Stderr, "  restart   restart services")
		fmt.Fprintln(os.Stderr, "  reload    reload services")
		fmt.Fprintln(os.Stderr, "  list      list services")
		fmt.Fprintln(os.Stderr, "  schema    show http address of config's json schema")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Flags:")
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
		fmt.Fprintln(os.Stderr, "error: not enough arguments")

		return CODE_INVALID_INVOKATION
	}

	cmd := f.Args()[0]
	switch cmd {
	case "run":
		c := RunCli{args: f.Args()[1:]}
		return c.Exec()
	case "logs", "start", "stop", "restart", "reload":
		msg := map[string]string{
			"logs":    "show logs for all services",
			"start":   "start all services",
			"stop":    "stop all services",
			"restart": "restart all services",
			"reload":  "reload all services",
		}
		return runCliAction(this.args[1:], cmd, msg[cmd])
	case "list":
		c := ListCli{args: f.Args()[1:]}
		return c.Exec()
	case "schema":
		c := SchemaCli{args: f.Args()[1:]}
		return c.Exec()
	default:
		fmt.Fprintf(os.Stderr, "error: invalid command \"%s\"\n", cmd)
		return CODE_INVALID_INVOKATION
	}
}

type RunCli struct {
	args []string
}

func (this *RunCli) Exec() int {
	f := flag.NewFlagSet("ella", flag.ExitOnError)
	cfgPath := f.String("c", "ella.json", "config file")
	hideLogs := f.Bool("l", false, "suppress logs")
	all := f.Bool("a", false, "start all services")
	help := f.Bool("h", false, "show help")

	f.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage:")
		fmt.Fprintln(os.Stderr, "  ella run -c ella.json [services...]")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Flags:")
		f.PrintDefaults()
	}

	f.Parse(this.args)

	if *help {
		f.Usage()

		return CODE_SUCCESS
	}

	ctx := common.NewSignalCtx(context.Background())
	d := Daemon{
		log: !*hideLogs,
	}

	var c config.Config
	err := config.ReadParsedConfig(*cfgPath, &c)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error: invalid config:", err)
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
		fmt.Fprintln(os.Stderr, "error:", err)
		return 0, CODE_GENERAL_ERR
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(b)))
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
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
	help := f.Bool("h", false, "show help")

	f.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage:")
		fmt.Fprintf(os.Stderr, "  ella %s -c ella.json -a\n", action)
		fmt.Fprintf(os.Stderr, "  ella %s -c ella.json [services...]\n", action)
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Flags:")
		f.PrintDefaults()
	}

	f.Parse(args)

	if *help {
		f.Usage()

		return CODE_SUCCESS
	}

	var c config.Config

	err := config.ReadParsedConfig(*configPath, &c)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error: invalid config:", err)
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
		fmt.Fprintln(os.Stderr, "error: no service name specified")

		return CODE_INVALID_INVOKATION
	}

	ctx := common.NewSignalCtx(context.Background())

	pid, code := getDaemonPid(c.PidFile)
	if code != CODE_SUCCESS {
		return code
	}
	socket := SocketClient{pid}
	err = socket.RunCommand(ctx, os.Stdout, action, serviceNames...)
	if err != nil {
		fmt.Sprintln(os.Stderr, "error:", err)
		return CODE_GENERAL_ERR
	}

	return CODE_SUCCESS
}

type ListCli struct {
	args []string
}

func (this *ListCli) Exec() int {
	f := flag.NewFlagSet("ella", flag.ExitOnError)
	configPath := f.String("c", "ella.json", "config file")
	help := f.Bool("h", false, "show help")

	f.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage:")
		fmt.Fprintln(os.Stderr, "  ella list -c ella.json")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Flags:")
		f.PrintDefaults()
	}

	f.Parse(this.args)

	if *help {
		f.Usage()

		return CODE_SUCCESS
	}

	var c config.Config

	err := config.ReadParsedConfig(*configPath, &c)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error: invalid config:", err)
		return CODE_INVALID_CONFIG
	}

	if len(f.Args()) != 0 {
		fmt.Fprintf(os.Stderr, "error: extra argument: %s\n", f.Args()[0])

		return CODE_INVALID_INVOKATION
	}

	ctx := common.NewSignalCtx(context.Background())

	pid, code := getDaemonPid(c.PidFile)
	if code != CODE_SUCCESS {
		return code
	}
	socket := SocketClient{pid}
	err = socket.RunCommand(ctx, os.Stdout, "list")
	if err != nil {
		fmt.Sprintln(os.Stderr, "error:", err)
		return CODE_GENERAL_ERR
	}

	return CODE_SUCCESS
}

type SchemaCli struct {
	args []string
}

func (this *SchemaCli) Exec() int {
	f := flag.NewFlagSet("ella", flag.ExitOnError)
	f.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage:")
		fmt.Fprintln(os.Stderr, "  ella schema")
	}

	f.Parse(this.args)

	if len(f.Args()) != 0 {
		fmt.Fprintf(os.Stderr, "error: extra argument: %s\n", f.Args()[0])

		return CODE_INVALID_INVOKATION
	}

	fmt.Println(common.GetJsonSchemaAddress(VERSION))

	return CODE_SUCCESS
}

func main() {
	c := Cli{args: os.Args[1:]}
	os.Exit(c.Exec())
}
