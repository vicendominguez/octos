package main

import (
	"bytes"
	"testing"
)

func TestInterpolateResolvesContext(t *testing.T) {
	ctx := &Context{
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
	ctx := &Context{
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
	ctx := &Context{
		Global:  map[string]any{"name": "test"},
		Outputs: map[string]string{"step1": "done"},
	}

	_, warnings := interpolate("{{context.name}} {{step1.output}}", ctx)

	if len(warnings) != 0 {
		t.Errorf("expected no warnings returned, got %d: %v", len(warnings), warnings)
	}
}
