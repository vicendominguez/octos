package main

import (
	"bytes"
	"testing"
)

func TestInterpolateResolvesContext(t *testing.T) {
	ctx := &PipelineContext{
		Global:  map[string]any{"name": "test", "items": []any{"a", "b"}},
		Outputs: map[string]string{"step1": "output1"},
	}

	result, _ := interpolate("{{context.name}} {{step1.output}}", ctx)
	if result != "test output1" {
		t.Errorf("got %q, want %q", result, "test output1")
	}

	result, _ = interpolate("{{context.items}}", ctx)
	if result != "- a\n- b" {
		t.Errorf("got %q, want %q", result, "- a\n- b")
	}
}

func TestInterpolateWarnsUnresolved(t *testing.T) {
	ctx := &PipelineContext{
		Global:  map[string]any{"name": "test"},
		Outputs: map[string]string{},
	}

	_, warnings := interpolate("{{context.typo}} and {{missing.output}}", ctx)

	// Verify warnings are returned
	if len(warnings) != 2 {
		t.Errorf("expected 2 warnings, got %d", len(warnings))
	}

	found := false
	for _, w := range warnings {
		if bytes.Contains([]byte(w), []byte("{{context.typo}}")) {
			found = true
		}
	}
	if !found {
		t.Errorf("expected warning about {{context.typo}}, got: %v", warnings)
	}

	found = false
	for _, w := range warnings {
		if bytes.Contains([]byte(w), []byte("{{missing.output}}")) {
			found = true
		}
	}
	if !found {
		t.Errorf("expected warning about {{missing.output}}, got: %v", warnings)
	}
}

func TestInterpolateNoWarningWhenResolved(t *testing.T) {
	ctx := &PipelineContext{
		Global:  map[string]any{"name": "test"},
		Outputs: map[string]string{"step1": "done"},
	}

	_, warnings := interpolate("{{context.name}} {{step1.output}}", ctx)

	if len(warnings) != 0 {
		t.Errorf("expected no warnings returned, got %d: %v", len(warnings), warnings)
	}
}

func TestInterpolateResolvesArtifacts(t *testing.T) {
	ctx := &PipelineContext{
		Global: map[string]any{},
		Outputs: map[string]string{
			"artifact.plan":     "the plan content",
			"artifact.analysis": "the analysis content",
			"step1":             "step1 output",
		},
	}

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"artifact.plan resolves", "Plan: {{artifact.plan}}", "Plan: the plan content"},
		{"artifact.analysis resolves", "Analysis: {{artifact.analysis}}", "Analysis: the analysis content"},
		{"step.output resolves", "{{step1.output}}", "step1 output"},
		{"step shorthand resolves", "{{step1}}", "step1 output"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, warnings := interpolate(tt.input, ctx)
			if result != tt.want {
				t.Errorf("interpolate(%q) = %q, want %q", tt.input, result, tt.want)
			}
			if len(warnings) != 0 {
				t.Errorf("expected no warnings, got %v", warnings)
			}
		})
	}
}
