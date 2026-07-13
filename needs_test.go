package main

import (
	"strings"
	"testing"
)

func TestNeedsValidPipelines(t *testing.T) {
	tests := []struct {
		name  string
		steps []Step
	}{
		{
			name: "linear dependencies",
			steps: []Step{
				{Name: "implement", Prompt: "do stuff"},
				{Name: "review", Prompt: "review", Needs: []string{"implement"}},
				{Name: "release", Prompt: "release", Needs: []string{"review"}},
			},
		},
		{
			name: "diamond graph",
			steps: []Step{
				{Name: "implement", Prompt: "do stuff"},
				{Name: "review", Prompt: "review", Needs: []string{"implement"}},
				{Name: "security", Prompt: "audit", Needs: []string{"implement"}},
				{Name: "docs", Prompt: "docs", Needs: []string{"implement"}},
				{Name: "release", Prompt: "release", Needs: []string{"review", "security", "docs"}},
			},
		},
		{
			name: "no needs (sequential)",
			steps: []Step{
				{Name: "step1", Prompt: "one"},
				{Name: "step2", Prompt: "two"},
				{Name: "step3", Prompt: "three"},
			},
		},
		{
			name: "multiple deps on one step",
			steps: []Step{
				{Name: "a", Prompt: "a"},
				{Name: "b", Prompt: "b"},
				{Name: "c", Prompt: "c"},
				{Name: "d", Prompt: "d", Needs: []string{"a", "b", "c"}},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &Pipeline{
				Agent: AgentConfig{Cmd: "echo"},
				Steps: tt.steps,
			}
			if err := p.Validate(); err != nil {
				t.Fatalf("expected valid pipeline, got: %v", err)
			}
		})
	}
}

func TestNeedsInvalidPipelines(t *testing.T) {
	tests := []struct {
		name      string
		steps     []Step
		wantInErr string
	}{
		{
			name: "undefined reference",
			steps: []Step{
				{Name: "step1", Prompt: "one"},
				{Name: "step2", Prompt: "two", Needs: []string{"nonexistent"}},
			},
			wantInErr: "nonexistent",
		},
		{
			name: "forward reference",
			steps: []Step{
				{Name: "step1", Prompt: "one", Needs: []string{"step2"}},
				{Name: "step2", Prompt: "two"},
			},
			wantInErr: "not defined before",
		},
		{
			name: "self reference",
			steps: []Step{
				{Name: "step1", Prompt: "one", Needs: []string{"step1"}},
			},
			wantInErr: "step1",
		},
		{
			name: "cycle (caught as forward reference)",
			steps: []Step{
				{Name: "a", Prompt: "a", Needs: []string{"c"}},
				{Name: "b", Prompt: "b", Needs: []string{"a"}},
				{Name: "c", Prompt: "c", Needs: []string{"b"}},
			},
			wantInErr: "not defined before",
		},
		{
			name: "duplicate step names",
			steps: []Step{
				{Name: "step1", Prompt: "one"},
				{Name: "step1", Prompt: "two"},
			},
			wantInErr: "duplicate",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &Pipeline{
				Agent: AgentConfig{Cmd: "echo"},
				Steps: tt.steps,
			}
			err := p.Validate()
			if err == nil {
				t.Fatal("expected validation error")
			}
			if !strings.Contains(err.Error(), tt.wantInErr) {
				t.Fatalf("error should contain %q, got: %v", tt.wantInErr, err)
			}
		})
	}
}

func TestHasNeeds(t *testing.T) {
	pWithout := &Pipeline{
		Agent: AgentConfig{Cmd: "echo"},
		Steps: []Step{
			{Name: "a", Prompt: "a"},
			{Name: "b", Prompt: "b"},
		},
	}
	if pWithout.hasNeeds() {
		t.Fatal("pipeline without needs should return false")
	}

	pWith := &Pipeline{
		Agent: AgentConfig{Cmd: "echo"},
		Steps: []Step{
			{Name: "a", Prompt: "a"},
			{Name: "b", Prompt: "b", Needs: []string{"a"}},
		},
	}
	if !pWith.hasNeeds() {
		t.Fatal("pipeline with needs should return true")
	}
}
