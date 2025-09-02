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
	config := f.String("c", "ella.json", "config file")
	hideLogs := f.Bool("l", false, "supress logs")

	f.Usage = func() {
		fmt.Println("Usage:")
		fmt.Println("  ella run -c ella.json")
		fmt.Println()
		fmt.Println("Flags:")
		f.PrintDefaults()
	}

	f.Parse(this.args)

	if len(f.Args()) != 0 {
		fmt.Println("error: extra arguments:", f.Args()[0])

		return CODE_INVALID_INVOKATION
	}

	ctx := common.NewSignalCtx(context.Background())
	d := Daemon{
		log: !*hideLogs,
	}

	return d.Run(ctx, *config)
}

type LogsCli struct {
	args []string
}

func (this *LogsCli) Exec() int {
	f := flag.NewFlagSet("ella", flag.ExitOnError)
	configPath := f.String("c", "ella.json", "config file")
	all := f.Bool("a", false, "show logs for all services")

	f.Usage = func() {
		fmt.Println("Usage:")
		fmt.Println("  ella logs -c ella.json -a [services...]")
		fmt.Println()
		fmt.Println("Flags:")
		f.PrintDefaults()
	}

	f.Parse(this.args)

	var c config.Config

	err := config.ReadConfig(*configPath, &c)
	if err != nil {
		fmt.Println("error: invalid config:", err)
		return CODE_INVALID_CONFIG
	}

	var serviceNames []string
	if *all {
		var err error
		serviceNames = make([]string, 0)
		services, err := c.GetServices()
		if err != nil {
			fmt.Println("error:", err)
			return CODE_GENERAL_ERR
		}
		for _, s := range services {
			serviceNames = append(serviceNames, s.Name)
		}
	} else {
		serviceNames = f.Args()
	}

	ctx := common.NewSignalCtx(context.Background())

	pid, code := this.getPid(c.PidFile)
	if code != CODE_SUCCESS {
		return code
	}
	socket := SocketClient{pid}
	err = socket.Logs(ctx, os.Stdout, serviceNames...)
	if err != nil {
		fmt.Println("error:", err)
		return CODE_GENERAL_ERR
	}

	return CODE_SUCCESS
}

func (this *LogsCli) getPid(pidFile *string) (int, int) {
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

func main() {
	c := Cli{args: os.Args[1:]}
	os.Exit(c.Exec())
}
