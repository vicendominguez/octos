package main

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"
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
	output, err := runAgentWithStreaming(context.Background(), agent, "", func(line string) {
		mu.Lock()
		lines = append(lines, line)
		mu.Unlock()
	}, nil, 0)
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

func TestRunAgentWithStreamingContextCancel(t *testing.T) {
	agent := AgentConfig{
		Cmd:  "sh",
		Args: []string{"-c", "sleep 10; echo done"},
	}

	ctx, cancel := context.WithCancel(context.Background())

	// Cancel after 100ms
	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	_, err := runAgentWithStreaming(ctx, agent, "", nil, nil, 0)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected error from cancelled context")
	}
	if elapsed > 2*time.Second {
		t.Fatalf("cancellation took too long: %v", elapsed)
	}
}
