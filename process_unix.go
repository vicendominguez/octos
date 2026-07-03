//go:build !windows

package main

import (
	"os/exec"
	"syscall"
)

// detachFromTerminal creates a new session for the command so child processes
// cannot send SIGTTIN/SIGTTOU and suspend octos.
func detachFromTerminal(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
}
