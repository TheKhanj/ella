package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/thekhanj/ella/config"
)

var VERSION = "dev"

const (
	CODE_SUCCESS int = iota
	CODE_GENERAL_ERR
	CODE_INVALID_CONFIG
	CODE_INVALID_INVOKATION
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

	return this.runDaemon(*config)
}

func (this *RunCli) runDaemon(cfgPath string) int {
	var c config.Config

	err := config.ReadConfig(cfgPath, &c)
	if err != nil {
		fmt.Println("error: invalid config:", err)
		return CODE_INVALID_CONFIG
	}

	// TODO: find entry service and use that instead
	var sCfg config.Service
	b, err := json.Marshal(c.Services[0])
	if err != nil {
		log.Println("error:", err)
		return CODE_GENERAL_ERR
	}
	err = sCfg.UnmarshalJSON(b)
	if err != nil {
		log.Println("error:", err)
		return CODE_GENERAL_ERR
	}
	s, err := NewServiceFromConfig(&sCfg)
	if err != nil {
		log.Println("error:", err)
		return CODE_GENERAL_ERR
	}

	// TODO: handle sigterm and sigint
	err = s.Run(context.Background(), func() {
		err = s.Signal(ServiceSigStart)
		if err != nil {
			log.Println("error:", err)
		}
	})
	if err != nil {
		log.Println("error:", err)
	}

	return CODE_SUCCESS
}

func main() {
	c := Cli{args: os.Args[1:]}
	os.Exit(c.Exec())
}
