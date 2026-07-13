package main

import (
	"os"
	"testing"
)

func TestDryRunPipelines(t *testing.T) {
	tests := []struct {
		name string
		yaml string
	}{
		{
			name: "valid pipeline",
			yaml: `
agent:
  cmd: "echo"
  args: ["ok"]
context:
  role: "test"
steps:
  - name: step1
    prompt: "hello"
  - name: step2
    prompt: "based on {{step1.output}}"
`,
		},
		{
			name: "with conditions",
			yaml: `
agent:
  cmd: "echo"
  args: ["ok"]
context:
  role: "test"
steps:
  - name: check
    prompt: "check something"
    save_to: check.txt
  - name: conditional
    when: "{{check.output}} contains yes"
    prompt: "do the thing"
`,
		},
		{
			name: "missing context var (warning only)",
			yaml: `
agent:
  cmd: "echo"
  args: ["ok"]
context:
  role: "test"
steps:
  - name: step1
    prompt: "use {{context.nonexistent}}"
`,
		},
		{
			name: "stdin agent",
			yaml: `
agent:
  cmd: "cat"
  args: []
  stdin: true
context:
  role: "test"
steps:
  - name: step1
    prompt: "hello via stdin"
`,
		},
		{
			name: "artifact chain",
			yaml: `
agent:
  cmd: "echo"
  args: ["ok"]
context:
  role: "test"
steps:
  - name: analyze
    prompt: "analyze code"
    save_to: analysis.txt
  - name: implement
    load_from: analysis.txt
    prompt: "implement based on {{artifact.analysis}}"
`,
		},
		{
			name: "invalid condition syntax (warning only)",
			yaml: `
agent:
  cmd: "echo"
  args: ["ok"]
context:
  role: "test"
steps:
  - name: step1
    prompt: "do something"
  - name: step2
    when: "some garbage condition"
    prompt: "conditional step"
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f, _ := os.CreateTemp("", "pipeline-dryrun-*.yaml")
			f.WriteString(tt.yaml)
			f.Close()
			defer os.Remove(f.Name())

			p, err := LoadPipeline(f.Name())
			if err != nil {
				t.Fatal(err)
			}

			err = DryRun(p)
			if err != nil {
				t.Fatalf("expected no error, got: %v", err)
			}
		})
	}
}

func TestLooksLikeSecret(t *testing.T) {
	tests := []struct {
		key    string
		expect bool
	}{
		{"token", true},
		{"api_token", true},
		{"GITEA_TOKEN", true},
		{"password", true},
		{"secret_key", true},
		{"role", false},
		{"repo", false},
		{"language", false},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			got := looksLikeSecret(tt.key)
			if got != tt.expect {
				t.Errorf("looksLikeSecret(%q) = %v, want %v", tt.key, got, tt.expect)
			}
		})
	}
}

func TestIsValidConditionSyntax(t *testing.T) {
	tests := []struct {
		cond   string
		expect bool
	}{
		{"{{step.output}} contains VALIDATED", true},
		{"{{step.output}} equals yes", true},
		{"{{step.output}} not_empty", true},
		{"not_empty", true},
		{"some random text", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.cond, func(t *testing.T) {
			got := isValidConditionSyntax(tt.cond)
			if got != tt.expect {
				t.Errorf("isValidConditionSyntax(%q) = %v, want %v", tt.cond, got, tt.expect)
			}
		})
	}
}
