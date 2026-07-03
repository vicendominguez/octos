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

	// {{artifact.plan}} should resolve directly
	result, warnings := interpolate("Plan: {{artifact.plan}}", ctx)
	if result != "Plan: the plan content" {
		t.Errorf("artifact.plan: got %q, want %q", result, "Plan: the plan content")
	}
	if len(warnings) != 0 {
		t.Errorf("expected no warnings, got %v", warnings)
	}

	// {{artifact.analysis}} should resolve directly
	result, warnings = interpolate("Analysis: {{artifact.analysis}}", ctx)
	if result != "Analysis: the analysis content" {
		t.Errorf("artifact.analysis: got %q, want %q", result, "Analysis: the analysis content")
	}
	if len(warnings) != 0 {
		t.Errorf("expected no warnings, got %v", warnings)
	}

	// {{step1.output}} still works
	result, _ = interpolate("{{step1.output}}", ctx)
	if result != "step1 output" {
		t.Errorf("step1.output: got %q, want %q", result, "step1 output")
	}

	// {{step1}} also resolves directly (step name without .output)
	result, _ = interpolate("{{step1}}", ctx)
	if result != "step1 output" {
		t.Errorf("direct step1: got %q, want %q", result, "step1 output")
	}
}
