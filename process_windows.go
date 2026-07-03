//go:build windows

package main

import "os/exec"

// detachFromTerminal is a no-op on Windows.
func detachFromTerminal(cmd *exec.Cmd) {}
