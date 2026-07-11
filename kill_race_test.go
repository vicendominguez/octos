package main

import (
	"context"
	"os/exec"
	"sync"
	"testing"
	"time"
)

// TestKillStepRegistersIntent verifies that pressing R (KillStep)
// records the retry intent even when the process is already gone — the exact
// race that caused "R sometimes does nothing" under rate limits.
func TestKillStepRegistersIntent(t *testing.T) {
	r := &PipelineRunner{
		activeCmds:  make(map[int]*exec.Cmd),
		killedSteps: make(map[int]bool),
	}

	// Simulate the window where the process already finished and was cleared
	// (activeCmd == nil) right before the user presses R.
	r.clearActiveCmd(0)
	r.KillStep(0)

	if !r.consumeKilled(0) {
		t.Fatal("KillStep must register kill intent even when activeCmd is nil")
	}
	// consumeKilled must reset the flag so it does not leak into the next step.
	if r.consumeKilled(0) {
		t.Fatal("consumeKilled must reset the flag after reading it")
	}
}

// TestKillStepConcurrent reproduces the real scenario: a live process
// being killed from another goroutine (the TUI) while the pipeline goroutine
// is running the agent. Run with -race to exercise the lock coordination.
func TestKillStepConcurrent(t *testing.T) {
	r := &PipelineRunner{
		activeCmds:  make(map[int]*exec.Cmd),
		killedSteps: make(map[int]bool),
	}
	agent := AgentConfig{
		Cmd:  "sh",
		Args: []string{"-c", "sleep 2"},
	}

	stepIdx := 0

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		// Give the process time to actually start before killing it.
		time.Sleep(50 * time.Millisecond)
		r.KillStep(stepIdx)
	}()

	// This blocks until the process is killed by the goroutine above.
	_, err := runAgentWithStreaming(context.Background(), agent, "", nil, r, stepIdx)
	wg.Wait()

	// The process was killed, so it must have exited with an error...
	if err == nil {
		t.Fatal("expected error from killed process, got nil")
	}
	// ...and the retry intent must be observable by the pipeline loop.
	if !r.consumeKilled(stepIdx) {
		t.Fatal("consumeKilled must return true after KillStep on a live process")
	}
}

// TestConsumeKilledDoesNotLeakBetweenSteps ensures a kill in one step does not
// trigger a phantom manual retry in the next.
func TestConsumeKilledDoesNotLeakBetweenSteps(t *testing.T) {
	r := &PipelineRunner{
		activeCmds:  make(map[int]*exec.Cmd),
		killedSteps: make(map[int]bool),
	}

	r.KillStep(0)
	if !r.consumeKilled(0) {
		t.Fatal("first consumeKilled should be true")
	}

	// Simulate the next step starting fresh — step 1 should not be affected.
	r.setActiveCmd(1, nil)
	if r.consumeKilled(1) {
		t.Fatal("kill intent leaked into the next step")
	}
}
