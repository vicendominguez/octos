package main

import (
	"os"
	"strings"
	"testing"
)

func TestRunAgentWithStdin(t *testing.T) {
	// cat reads from stdin and outputs it
	agent := AgentConfig{
		Cmd:   "cat",
		Args:  []string{},
		Stdin: true,
	}

	output, err := runAgent(agent, "hello from stdin", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if strings.TrimSpace(output) != "hello from stdin" {
		t.Errorf("got %q, want %q", strings.TrimSpace(output), "hello from stdin")
	}
}

func TestRunAgentWithoutStdin(t *testing.T) {
	// echo appends prompt as argument (original behavior)
	agent := AgentConfig{
		Cmd:   "echo",
		Args:  []string{"prefix"},
		Stdin: false,
	}

	output, err := runAgent(agent, "the prompt", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if strings.TrimSpace(output) != "prefix the prompt" {
		t.Errorf("got %q, want %q", strings.TrimSpace(output), "prefix the prompt")
	}
}

func TestRunAgentWithStreamingStdin(t *testing.T) {
	agent := AgentConfig{
		Cmd:   "cat",
		Args:  []string{},
		Stdin: true,
	}

	var lines []string
	output, err := runAgentWithStreaming(agent, "line1\nline2\nline3", func(line string) {
		lines = append(lines, line)
	}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(output, "line1") || !strings.Contains(output, "line2") || !strings.Contains(output, "line3") {
		t.Errorf("output missing expected lines: %q", output)
	}

	if len(lines) != 3 {
		t.Errorf("expected 3 streamed lines, got %d: %v", len(lines), lines)
	}
}

func TestRunAgentWithStreamingNoStdin(t *testing.T) {
	agent := AgentConfig{
		Cmd:   "echo",
		Args:  []string{"-n"},
		Stdin: false,
	}

	var lines []string
	output, err := runAgentWithStreaming(agent, "hello streaming", func(line string) {
		lines = append(lines, line)
	}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if strings.TrimSpace(output) != "hello streaming" {
		t.Errorf("got %q, want %q", strings.TrimSpace(output), "hello streaming")
	}
}

func TestStdinWithLargePrompt(t *testing.T) {
	// Test that stdin handles large prompts that would fail as CLI arguments
	agent := AgentConfig{
		Cmd:   "cat",
		Args:  []string{},
		Stdin: true,
	}

	// Generate a 200KB prompt (would exceed ARG_MAX per-argument limits)
	largePrompt := strings.Repeat("x", 200*1024)

	output, err := runAgent(agent, largePrompt, nil)
	if err != nil {
		t.Fatalf("unexpected error with large prompt: %v", err)
	}

	if len(strings.TrimSpace(output)) != len(largePrompt) {
		t.Errorf("output length %d, want %d", len(strings.TrimSpace(output)), len(largePrompt))
	}
}

func TestPipelineLoadWithStdin(t *testing.T) {
	yaml := `
agent:
  cmd: "cat"
  args: []
  stdin: true
context:
  role: "test"
steps:
  - name: step1
    prompt: "hello via stdin"
`
	f, _ := os.CreateTemp("", "pipeline-stdin-*.yaml")
	f.WriteString(yaml)
	f.Close()
	defer os.Remove(f.Name())

	p, err := LoadPipeline(f.Name())
	if err != nil {
		t.Fatal(err)
	}

	if !p.Agent.Stdin {
		t.Error("expected agent.stdin to be true")
	}
}

func TestPipelineLoadWithoutStdin(t *testing.T) {
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
	f, _ := os.CreateTemp("", "pipeline-nostdin-*.yaml")
	f.WriteString(yaml)
	f.Close()
	defer os.Remove(f.Name())

	p, err := LoadPipeline(f.Name())
	if err != nil {
		t.Fatal(err)
	}

	if p.Agent.Stdin {
		t.Error("expected agent.stdin to be false when omitted")
	}
}

func TestPipelineRunWithStdin(t *testing.T) {
	yaml := `
agent:
  cmd: "cat"
  args: []
  stdin: true
context:
  role: "test"
steps:
  - name: step1
    prompt: "hello from pipeline stdin"
`
	f, _ := os.CreateTemp("", "pipeline-run-stdin-*.yaml")
	f.WriteString(yaml)
	f.Close()
	defer os.Remove(f.Name())

	p, err := LoadPipeline(f.Name())
	if err != nil {
		t.Fatal(err)
	}

	err = RunPipeline(p)
	if err != nil {
		t.Fatalf("pipeline run failed: %v", err)
	}
}

func TestPerStepAgentStdinOverride(t *testing.T) {
	yaml := `
agent:
  cmd: "echo"
  args: ["default"]
context:
  role: "test"
steps:
  - name: step1
    agent:
      cmd: "cat"
      args: []
      stdin: true
    prompt: "overridden to stdin"
`
	f, _ := os.CreateTemp("", "pipeline-step-stdin-*.yaml")
	f.WriteString(yaml)
	f.Close()
	defer os.Remove(f.Name())

	p, err := LoadPipeline(f.Name())
	if err != nil {
		t.Fatal(err)
	}

	if p.Agent.Stdin {
		t.Error("default agent should not have stdin")
	}
	if !p.Steps[0].Agent.Stdin {
		t.Error("step agent should have stdin: true")
	}

	err = RunPipeline(p)
	if err != nil {
		t.Fatalf("pipeline run failed: %v", err)
	}
}
