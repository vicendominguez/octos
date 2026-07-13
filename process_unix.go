// Package main — process_unix.go provides platform-specific process management
// for Unix systems. It detaches child agent processes from the controlling terminal
// to prevent SIGTTIN/SIGTTOU signals from suspending the TUI.

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
