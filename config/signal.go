package config

import (
	"fmt"
	"syscall"
)

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
