// Package main — process_windows.go provides a no-op implementation of
// detachFromTerminal for Windows where session management is not needed.

//go:build windows

package main

import "os/exec"

// detachFromTerminal is a no-op on Windows.
func detachFromTerminal(cmd *exec.Cmd) {}
