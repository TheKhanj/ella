package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/thekhanj/ella/common"
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

func main() {
	c := Cli{args: os.Args[1:]}
	os.Exit(c.Exec())
}
