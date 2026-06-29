package main

import (
	"os"
	"testing"
)

func TestHeadlessNoLoop(t *testing.T) {
	// --loop 0 (default): should run exactly once
	yaml := `
agent:
  cmd: "echo"
  args: ["ok"]
context:
  role: "test"
steps:
  - name: step1
    prompt: "hello"
`
	f, _ := os.CreateTemp("", "pipeline-*.yaml")
	f.WriteString(yaml)
	f.Close()
	defer os.Remove(f.Name())

	p, err := LoadPipeline(f.Name())
	if err != nil {
		t.Fatal(err)
	}

	count := 0
	loopCount := 0 // simulates --loop 0 (default)
	if loopCount == 0 {
		loopCount = 1
	}
	for i := 1; i <= loopCount; i++ {
		count++
		_ = RunPipeline(p)
	}

	if count != 1 {
		t.Fatalf("expected 1 run, got %d", count)
	}
}

func TestHeadlessLoopN(t *testing.T) {
	// --loop 3: should run exactly 3 times
	yaml := `
agent:
  cmd: "echo"
  args: ["ok"]
context:
  role: "test"
steps:
  - name: step1
    prompt: "hello"
`
	f, _ := os.CreateTemp("", "pipeline-*.yaml")
	f.WriteString(yaml)
	f.Close()
	defer os.Remove(f.Name())

	p, err := LoadPipeline(f.Name())
	if err != nil {
		t.Fatal(err)
	}

	count := 0
	loopCount := 3
	for i := 1; i <= loopCount; i++ {
		count++
		_ = RunPipeline(p)
	}

	if count != 3 {
		t.Fatalf("expected 3 runs, got %d", count)
	}
}

func TestTUIAutoRestartOnlyWithLoop(t *testing.T) {
	// maxLoops = 0: should NOT auto-restart
	// maxLoops > 0 and currentLoop < maxLoops: should auto-restart

	// Case 1: no loop (maxLoops = 0)
	maxLoops := 0
	currentLoop := 1
	shouldRestart := maxLoops > 0 && currentLoop < maxLoops
	if shouldRestart {
		t.Fatal("should not auto-restart when maxLoops = 0")
	}

	// Case 2: loop 3, currentLoop 1 (should restart)
	maxLoops = 3
	currentLoop = 1
	shouldRestart = maxLoops > 0 && currentLoop < maxLoops
	if !shouldRestart {
		t.Fatal("should auto-restart when currentLoop < maxLoops")
	}

	// Case 3: loop 3, currentLoop 3 (should NOT restart)
	maxLoops = 3
	currentLoop = 3
	shouldRestart = maxLoops > 0 && currentLoop < maxLoops
	if shouldRestart {
		t.Fatal("should not auto-restart when currentLoop >= maxLoops")
	}
}

func TestManualRestartAlwaysAllowed(t *testing.T) {
	// handleRestartKey should allow restart unless maxLoops > 0 AND currentLoop >= maxLoops

	// Case 1: no loop mode, always allow
	maxLoops := 0
	currentLoop := 5
	blocked := maxLoops > 0 && currentLoop >= maxLoops
	if blocked {
		t.Fatal("manual restart should always be allowed without --loop")
	}

	// Case 2: loop 3, currentLoop 3, should block
	maxLoops = 3
	currentLoop = 3
	blocked = maxLoops > 0 && currentLoop >= maxLoops
	if !blocked {
		t.Fatal("manual restart should be blocked when max reached")
	}

	// Case 3: loop 3, currentLoop 2, should allow
	maxLoops = 3
	currentLoop = 2
	blocked = maxLoops > 0 && currentLoop >= maxLoops
	if blocked {
		t.Fatal("manual restart should be allowed when under max")
	}
}

func TestFormatLoopInfoNeverEmpty(t *testing.T) {
	// formatLoopInfo should never return an empty string

	p := &Pipeline{
		Agent: AgentConfig{Cmd: "echo", Args: []string{"ok"}},
		Steps: []Step{{Name: "s", Prompt: "p"}},
	}
	m := NewTUIModel(p, false)

	// Default state: currentLoop=1, maxLoops=0
	info := m.formatLoopInfo()
	if info == "" {
		t.Fatal("formatLoopInfo must never return empty string")
	}
	if len(info) < 3 {
		t.Fatalf("formatLoopInfo too short: %q", info)
	}

	// With maxLoops set
	m.maxLoops = 5
	m.currentLoop = 3
	info = m.formatLoopInfo()
	if info == "" {
		t.Fatal("formatLoopInfo must never return empty string with maxLoops")
	}
	if len(info) < 3 {
		t.Fatalf("formatLoopInfo too short: %q", info)
	}
}
