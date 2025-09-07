package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/user"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"syscall"
)

func GetPidFile(pidFile *string) string {
	if pidFile != nil {
		return *pidFile
	}

	if uid := syscall.Getuid(); uid == 0 {
		return "/var/run/ella/main.pid"
	} else {
		return fmt.Sprintf("/var/run/user/%d/ella/main.pid", uid)
	}
}

func ReadConfig(path string, cfg *Config) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	return cfg.UnmarshalJSON(b)
}

func ReadParsedConfig(path string, cfg *Config) error {
	var raw Config
	err := ReadConfig(path, &raw)
	if err != nil {
		return err
	}

	return raw.IncludeAll(cfg)
}

func (this *Config) IncludeAll(cfg *Config) error {
	return this.includeAll(cfg, []string{})
}

func (this *Config) includeAll(cfg *Config, included []string) error {
	cfg.PidFile = this.PidFile
	services := make([]Service, 0)

	for _, glob := range this.Include {
		cfgs, err := this.globReadConfig(included, glob)
		if err != nil {
			return err
		}

		for _, cfg := range cfgs {
			services = append(services, cfg.Services...)
		}
	}

	services = append(services, this.Services...)
	cfg.Services = services

	err := this.checkDuplicateServices(cfg)
	if err != nil {
		return err
	}

	return nil
}

func (this *Config) checkDuplicateServices(cfg *Config) error {
	cfgs := make(map[string]bool)
	for _, s := range cfg.Services {
		if cfgs[s.Name] == true {
			return fmt.Errorf("duplicate service name: %s", s.Name)
		}
		cfgs[s.Name] = true
	}

	return nil
}

func (this *Config) globReadConfig(
	included []string, pattern string,
) ([]*Config, error) {
	files, err := filepath.Glob(pattern)
	if err != nil {
		return nil, err
	}

	cfgs := make([]*Config, 0)
	for _, file := range files {
		if slices.Contains(included, file) {
			return nil, fmt.Errorf(
				"circular inclusion: %s -> %s",
				strings.Join(included, " -> "), file,
			)
		}

		var cfg Config
		err = ReadConfig(file, &cfg)
		if err != nil {
			return nil, err
		}

		included = append(included, file)
		var subCfg Config
		err := cfg.includeAll(&subCfg, included)
		included = included[:len(included)-1]
		if err != nil {
			return nil, err
		}

		cfgs = append(cfgs, &subCfg)
	}

	return cfgs, nil
}

func (this *Proc) GetStop() (StopProcAction, error) {
	stop := this.Stop
	if stopSignal, ok := stop.(string); ok {
		return ProcActionSignalCode(stopSignal), nil
	} else if m, ok := stop.(map[string]any); ok {
		switch m["type"] {
		case "signal":
			var c StopSignalProcAction
			b, err := json.Marshal(stop)
			if err != nil {
				return nil, err
			}
			err = c.UnmarshalJSON(b)
			if err != nil {
				return nil, err
			}
			return &c, nil
		case "exec":
			return nil, errors.New("not implemented")
		default:
			return nil, fmt.Errorf("invalid action type: %s", m["type"])
		}
	} else {
		return nil, fmt.Errorf("invalid stop action: %v", stop)
	}
}

func (this *Proc) GetReload() (ReloadProcAction, error) {
	reload := this.Reload
	if reloadSignal, ok := reload.(string); ok {
		return ProcActionSignalCode(reloadSignal), nil
	} else if m, ok := reload.(map[string]any); ok {
		switch m["type"] {
		case "exec":
			return nil, errors.New("not implemented")
		default:
			return nil, fmt.Errorf("invalid action type: %s", m["type"])
		}
	} else {
		return nil, fmt.Errorf("invalid reload action: %v", reload)
	}
}

func (this *Proc) GetWatchdog() (ProcWatchdog, error) {
	if this.Watchdog == nil {
		return &SimpleWatchdog{
			Strategy: "simple",
		}, nil
	}

	var m map[string]any
	m, ok := this.Watchdog.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("invalid watchdog config: %v", this.Watchdog)
	}

	switch m["strategy"] {
	case "simple":
		return &SimpleWatchdog{
			Strategy: "simple",
		}, nil
	default:
		return nil, fmt.Errorf("invalid watchdog strategy: %s", m["strategy"])
	}
}

func (this *Proc) GetUid() (uint32, error) {
	u := this.User
	if uStr, ok := u.(string); ok {
		if uStr == "!inherit" {
			return uint32(syscall.Getuid()), nil
		}

		user, err := user.Lookup(uStr)
		if err != nil {
			return 0, err
		}

		uid, err := strconv.Atoi(user.Uid)
		if err != nil {
			return 0, err
		}

		return uint32(uid), nil
	} else if uid, ok := u.(int); ok {
		return uint32(uid), nil
	} else {
		return 0, fmt.Errorf("invalid user: %v", u)
	}
}

func (this *Proc) GetGid() (uint32, error) {
	g := this.Group
	if gStr, ok := g.(string); ok {
		if gStr == "!inherit" {
			return uint32(syscall.Getgid()), nil
		}

		group, err := user.LookupGroup(gStr)
		if err != nil {
			return 0, err
		}

		gid, err := strconv.Atoi(group.Gid)
		if err != nil {
			return 0, err
		}

		return uint32(gid), nil
	} else if uid, ok := g.(int); ok {
		return uint32(uid), nil
	} else {
		return 0, fmt.Errorf("invalid group: %v", g)
	}
}

func (this *Proc) GetStdin() (io.ReadCloser, error) {
	if this.Stdin == nil {
		return nil, nil
	}

	if path, ok := this.Stdin.(string); ok {
		file, err := os.Open(path)
		if err != nil {
			return nil, err
		}
		return file, nil
	} else {
		return nil, fmt.Errorf("invalid stdin: %v", this.Stdin)
	}
}

func (this *Proc) GetEnv() ([]string, error) {
	env := this.Environments

	if inherit, ok := env.(string); ok && inherit == "!inherit" {
		return os.Environ(), nil
	} else if mp, ok := env.(map[string]any); ok {
		envs := make([]string, 0)
		for key, val := range mp {
			parsed, err := this.parseEnvVal(key, val)
			if err != nil {
				return nil, err
			}
			envs = append(envs, fmt.Sprintf("%s=%s", key, parsed))
		}

		return envs, nil
	} else {
		return nil, fmt.Errorf("invalid environments: %v", env)
	}
}

func (this *Proc) parseEnvVal(key string, val any) (string, error) {
	if str, ok := val.(string); ok {
		if str == "!inherit" {
			v, found := os.LookupEnv(key)
			if !found {
				return "", fmt.Errorf("environment variable %s is not set", key)
			}

			return v, nil
		}

		return str, nil
	} else if literalObj, ok := val.(map[string]any); ok {
		var literal LiteralEnvValue
		b, err := json.Marshal(literalObj)
		if err != nil {
			return "", err
		}
		err = literal.UnmarshalJSON(b)
		if err != nil {
			return "", err
		}

		return literal.Value, nil
	} else {
		return "", fmt.Errorf("invalid environment value: %s: %v", key, val)
	}
}

func (this *ProcActionSignalCode) GetSignal() syscall.Signal {
	switch *this {
	case ProcActionSignalCodeSIGABRT:
		return syscall.SIGABRT
	case ProcActionSignalCodeSIGALRM:
		return syscall.SIGALRM
	case ProcActionSignalCodeSIGCHLD:
		return syscall.SIGCHLD
	case ProcActionSignalCodeSIGCONT:
		return syscall.SIGCONT
	case ProcActionSignalCodeSIGFPE:
		return syscall.SIGFPE
	case ProcActionSignalCodeSIGHUP:
		return syscall.SIGHUP
	case ProcActionSignalCodeSIGILL:
		return syscall.SIGILL
	case ProcActionSignalCodeSIGINT:
		return syscall.SIGINT
	case ProcActionSignalCodeSIGKILL:
		return syscall.SIGKILL
	case ProcActionSignalCodeSIGPIPE:
		return syscall.SIGPIPE
	case ProcActionSignalCodeSIGQUIT:
		return syscall.SIGQUIT
	case ProcActionSignalCodeSIGSEGV:
		return syscall.SIGSEGV
	case ProcActionSignalCodeSIGSTOP:
		return syscall.SIGSTOP
	case ProcActionSignalCodeSIGTERM:
		return syscall.SIGTERM
	case ProcActionSignalCodeSIGTSTP:
		return syscall.SIGTSTP
	case ProcActionSignalCodeSIGTTIN:
		return syscall.SIGTTIN
	case ProcActionSignalCodeSIGTTOU:
		return syscall.SIGTTOU
	case ProcActionSignalCodeSIGUSR1:
		return syscall.SIGUSR1
	case ProcActionSignalCodeSIGUSR2:
		return syscall.SIGUSR2
	default:
		panic(fmt.Sprintf("not implemented signal: %s", string(*this)))
	}
}
