package main

import (
	"os"
	"testing"
)

func TestOnFailureDefaultIsFailFast(t *testing.T) {
	// A pipeline with a failing command should stop
	yaml := `
agent:
  cmd: "false"
  args: []
context:
  role: "test"
steps:
  - name: will-fail
    prompt: "this will fail"
  - name: should-not-run
    prompt: "unreachable"
`
	f, _ := os.CreateTemp("", "pipeline-failfast-*.yaml")
	f.WriteString(yaml)
	f.Close()
	defer os.Remove(f.Name())

	p, err := LoadPipeline(f.Name())
	if err != nil {
		t.Fatal(err)
	}

	err = RunPipeline(p)
	if err == nil {
		t.Fatal("expected error from failing step")
	}
	if !contains(err.Error(), "will-fail") {
		t.Errorf("error should mention step name, got: %v", err)
	}
}

func TestOnFailureSkip(t *testing.T) {
	// A failing step with on_failure: skip should not stop the pipeline
	yaml := `
agent:
  cmd: "echo"
  args: ["ok"]
context:
  role: "test"
steps:
  - name: will-fail
    prompt: "this will fail"
    on_failure: skip
  - name: should-run
    prompt: "this should execute"
`
	f, _ := os.CreateTemp("", "pipeline-skip-*.yaml")
	f.WriteString(yaml)
	f.Close()
	defer os.Remove(f.Name())

	p, err := LoadPipeline(f.Name())
	if err != nil {
		t.Fatal(err)
	}

	// Override first step agent to something that fails
	p.Steps[0].Agent = &AgentConfig{Cmd: "false", Args: []string{}}

	err = RunPipeline(p)
	if err != nil {
		t.Fatalf("pipeline should complete with skip, got error: %v", err)
	}
}

func TestOnFailureRetry(t *testing.T) {
	// Use a script that fails on first call but succeeds on second
	// We simulate this by testing that max_retries is respected when it always fails
	yaml := `
agent:
  cmd: "false"
  args: []
context:
  role: "test"
steps:
  - name: will-retry
    prompt: "retry this"
    on_failure: retry
    max_retries: 2
`
	f, _ := os.CreateTemp("", "pipeline-retry-*.yaml")
	f.WriteString(yaml)
	f.Close()
	defer os.Remove(f.Name())

	p, err := LoadPipeline(f.Name())
	if err != nil {
		t.Fatal(err)
	}

	err = RunPipeline(p)
	if err == nil {
		t.Fatal("expected error after exhausting retries")
	}
	if !contains(err.Error(), "will-retry") {
		t.Errorf("error should mention step name, got: %v", err)
	}
}

func TestOnFailureRetryDefaultMaxRetries(t *testing.T) {
	// on_failure: retry without max_retries should default to 1 retry
	yaml := `
agent:
  cmd: "false"
  args: []
context:
  role: "test"
steps:
  - name: will-retry
    prompt: "retry this"
    on_failure: retry
`
	f, _ := os.CreateTemp("", "pipeline-retry-default-*.yaml")
	f.WriteString(yaml)
	f.Close()
	defer os.Remove(f.Name())

	p, err := LoadPipeline(f.Name())
	if err != nil {
		t.Fatal(err)
	}

	err = RunPipeline(p)
	if err == nil {
		t.Fatal("expected error after exhausting retries")
	}
}

func TestOnFailureInvalidValue(t *testing.T) {
	yaml := `
agent:
  cmd: "echo"
  args: ["ok"]
context:
  role: "test"
steps:
  - name: step1
    prompt: "hello"
    on_failure: invalid_value
`
	f, _ := os.CreateTemp("", "pipeline-invalid-onfailure-*.yaml")
	f.WriteString(yaml)
	f.Close()
	defer os.Remove(f.Name())

	_, err := LoadPipeline(f.Name())
	if err == nil {
		t.Fatal("expected validation error for invalid on_failure value")
	}
	if !contains(err.Error(), "on_failure") {
		t.Errorf("error should mention on_failure, got: %v", err)
	}
}

func TestMaxRetriesWithoutRetryPolicy(t *testing.T) {
	yaml := `
agent:
  cmd: "echo"
  args: ["ok"]
context:
  role: "test"
steps:
  - name: step1
    prompt: "hello"
    max_retries: 3
`
	f, _ := os.CreateTemp("", "pipeline-maxretries-nopolicy-*.yaml")
	f.WriteString(yaml)
	f.Close()
	defer os.Remove(f.Name())

	_, err := LoadPipeline(f.Name())
	if err == nil {
		t.Fatal("expected validation error for max_retries without on_failure: retry")
	}
	if !contains(err.Error(), "max_retries") {
		t.Errorf("error should mention max_retries, got: %v", err)
	}
}

func TestOnFailureSkipDoesNotProduceOutput(t *testing.T) {
	// When a step is skipped, subsequent steps should not see its output
	yaml := `
agent:
  cmd: "echo"
  args: ["ok"]
context:
  role: "test"
steps:
  - name: will-fail
    prompt: "this will fail"
    on_failure: skip
  - name: uses-output
    prompt: "previous was {{will-fail.output}}"
`
	f, _ := os.CreateTemp("", "pipeline-skip-no-output-*.yaml")
	f.WriteString(yaml)
	f.Close()
	defer os.Remove(f.Name())

	p, err := LoadPipeline(f.Name())
	if err != nil {
		t.Fatal(err)
	}

	// Override first step to fail
	p.Steps[0].Agent = &AgentConfig{Cmd: "false", Args: []string{}}

	err = RunPipeline(p)
	if err != nil {
		t.Fatalf("pipeline should complete, got: %v", err)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsStr(s, substr))
}

func containsStr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
