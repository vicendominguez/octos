package main

import (
	"context"
	"os/exec"
	"sync"
	"testing"
	"time"
)

// TestKillCurrentStepRegistersIntent verifies that pressing R (KillCurrentStep)
// records the retry intent even when the process is already gone — the exact
// race that caused "R sometimes does nothing" under rate limits.
func TestKillCurrentStepRegistersIntent(t *testing.T) {
	r := &PipelineRunner{
		activeCmds:  make(map[int]*exec.Cmd),
		killedSteps: make(map[int]bool),
	}

	// Simulate the window where the process already finished and was cleared
	// (activeCmd == nil) right before the user presses R.
	r.clearActiveCmd()
	r.KillCurrentStep()

	if !r.consumeKilled() {
		t.Fatal("KillCurrentStep must register kill intent even when activeCmd is nil")
	}
	// consumeKilled must reset the flag so it does not leak into the next step.
	if r.consumeKilled() {
		t.Fatal("consumeKilled must reset the flag after reading it")
	}
}

// TestKillCurrentStepConcurrent reproduces the real scenario: a live process
// being killed from another goroutine (the TUI) while the pipeline goroutine
// is running the agent. Run with -race to exercise the lock coordination.
func TestKillCurrentStepConcurrent(t *testing.T) {
	r := &PipelineRunner{
		activeCmds:  make(map[int]*exec.Cmd),
		killedSteps: make(map[int]bool),
	}
	agent := AgentConfig{
		Cmd:  "sh",
		Args: []string{"-c", "sleep 2"},
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		// Give the process time to actually start before killing it.
		time.Sleep(50 * time.Millisecond)
		r.KillCurrentStep()
	}()

	// This blocks until the process is killed by the goroutine above.
	_, err := runAgentWithStreaming(context.Background(), agent, "", nil, r)
	wg.Wait()

	// The process was killed, so it must have exited with an error...
	if err == nil {
		t.Fatal("expected error from killed process, got nil")
	}
	// ...and the retry intent must be observable by the pipeline loop.
	if !r.consumeKilled() {
		t.Fatal("consumeKilled must return true after KillCurrentStep on a live process")
	}
}

// TestConsumeKilledDoesNotLeakBetweenSteps ensures a kill in one step does not
// trigger a phantom manual retry in the next.
func TestConsumeKilledDoesNotLeakBetweenSteps(t *testing.T) {
	r := &PipelineRunner{
		activeCmds:  make(map[int]*exec.Cmd),
		killedSteps: make(map[int]bool),
	}

	r.KillCurrentStep()
	if !r.consumeKilled() {
		t.Fatal("first consumeKilled should be true")
	}

	// Simulate the next step starting fresh.
	r.setActiveCmd(nil)
	if r.consumeKilled() {
		t.Fatal("kill intent leaked into the next step")
	}
}
