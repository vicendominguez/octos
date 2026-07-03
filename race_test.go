package main

import (
	"strings"
	"sync"
	"testing"
)

func TestRunAgentWithStreamingNoRace(t *testing.T) {
	// sh -c produces interleaved stdout and stderr concurrently
	agent := AgentConfig{
		Cmd:  "sh",
		Args: []string{"-c", "echo out1; echo err1 >&2; echo out2; echo err2 >&2"},
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

	// All four lines must appear in the output
	for _, expected := range []string{"out1", "out2", "err1", "err2"} {
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
