package main

import (
	"strings"
	"sync"
	"testing"
)

func TestRunAgentWithStreamingNoRace(t *testing.T) {
	// Use a command that produces multiple lines on stdout to verify
	// concurrent writes to the shared builder are safe under -race.
	agent := AgentConfig{
		Cmd:  "sh",
		Args: []string{"-c", "echo line1; echo line2; echo line3; echo line4"},
	}

	var mu sync.Mutex
	var lines []string
	output, err := runAgentWithStreaming(agent, "", func(line string) {
		mu.Lock()
		lines = append(lines, line)
		mu.Unlock()
	}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, expected := range []string{"line1", "line2", "line3", "line4"} {
		if !strings.Contains(output, expected) {
			t.Errorf("output missing %q: %s", expected, output)
		}
	}

	mu.Lock()
	lineCount := len(lines)
	mu.Unlock()
	if lineCount < 4 {
		t.Errorf("expected at least 4 streamed lines, got %d", lineCount)
	}
}
