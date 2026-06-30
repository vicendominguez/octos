package main

import (
	"bytes"
	"os"
	"testing"
)

func TestInterpolateResolvesContext(t *testing.T) {
	ctx := &Context{
		Global:  map[string]any{"name": "test", "items": []any{"a", "b"}},
		Outputs: map[string]string{"step1": "output1"},
	}

	result := interpolate("{{context.name}} {{step1.output}}", ctx)
	if result != "test output1" {
		t.Errorf("got %q, want %q", result, "test output1")
	}

	result = interpolate("{{context.items}}", ctx)
	if result != "- a\n- b" {
		t.Errorf("got %q, want %q", result, "- a\n- b")
	}
}

func TestInterpolateWarnsUnresolved(t *testing.T) {
	ctx := &Context{
		Global:  map[string]any{"name": "test"},
		Outputs: map[string]string{},
	}

	// Capture stderr
	old := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	_ = interpolate("{{context.typo}} and {{missing.output}}", ctx)

	w.Close()
	os.Stderr = old

	var buf bytes.Buffer
	buf.ReadFrom(r)
	stderr := buf.String()

	if !bytes.Contains([]byte(stderr), []byte("{{context.typo}}")) {
		t.Errorf("expected warning about {{context.typo}}, got: %q", stderr)
	}
	if !bytes.Contains([]byte(stderr), []byte("{{missing.output}}")) {
		t.Errorf("expected warning about {{missing.output}}, got: %q", stderr)
	}
}

func TestInterpolateNoWarningWhenResolved(t *testing.T) {
	ctx := &Context{
		Global:  map[string]any{"name": "test"},
		Outputs: map[string]string{"step1": "done"},
	}

	old := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	_ = interpolate("{{context.name}} {{step1.output}}", ctx)

	w.Close()
	os.Stderr = old

	var buf bytes.Buffer
	buf.ReadFrom(r)

	if buf.Len() > 0 {
		t.Errorf("expected no warnings, got: %q", buf.String())
	}
}
