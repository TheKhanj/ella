package config

import (
	"fmt"
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
