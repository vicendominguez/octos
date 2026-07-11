package main

import (
	"strings"
	"testing"
)

func TestNeedsValidDependencies(t *testing.T) {
	p := &Pipeline{
		Agent: AgentConfig{Cmd: "echo"},
		Steps: []Step{
			{Name: "implement", Prompt: "do stuff"},
			{Name: "review", Prompt: "review", Needs: []string{"implement"}},
			{Name: "release", Prompt: "release", Needs: []string{"review"}},
		},
	}
	if err := p.Validate(); err != nil {
		t.Fatalf("expected valid pipeline, got: %v", err)
	}
}

func TestNeedsDiamondGraph(t *testing.T) {
	p := &Pipeline{
		Agent: AgentConfig{Cmd: "echo"},
		Steps: []Step{
			{Name: "implement", Prompt: "do stuff"},
			{Name: "review", Prompt: "review", Needs: []string{"implement"}},
			{Name: "security", Prompt: "audit", Needs: []string{"implement"}},
			{Name: "docs", Prompt: "docs", Needs: []string{"implement"}},
			{Name: "release", Prompt: "release", Needs: []string{"review", "security", "docs"}},
		},
	}
	if err := p.Validate(); err != nil {
		t.Fatalf("expected valid diamond graph, got: %v", err)
	}
}

func TestNeedsNoNeeds(t *testing.T) {
	p := &Pipeline{
		Agent: AgentConfig{Cmd: "echo"},
		Steps: []Step{
			{Name: "step1", Prompt: "one"},
			{Name: "step2", Prompt: "two"},
			{Name: "step3", Prompt: "three"},
		},
	}
	if err := p.Validate(); err != nil {
		t.Fatalf("pipeline without needs should be valid, got: %v", err)
	}
}

func TestNeedsReferenceUndefined(t *testing.T) {
	p := &Pipeline{
		Agent: AgentConfig{Cmd: "echo"},
		Steps: []Step{
			{Name: "step1", Prompt: "one"},
			{Name: "step2", Prompt: "two", Needs: []string{"nonexistent"}},
		},
	}
	err := p.Validate()
	if err == nil {
		t.Fatal("expected error for undefined needs reference")
	}
	if !strings.Contains(err.Error(), "nonexistent") {
		t.Fatalf("error should mention 'nonexistent', got: %v", err)
	}
}

func TestNeedsReferenceAfterCurrentStep(t *testing.T) {
	p := &Pipeline{
		Agent: AgentConfig{Cmd: "echo"},
		Steps: []Step{
			{Name: "step1", Prompt: "one", Needs: []string{"step2"}},
			{Name: "step2", Prompt: "two"},
		},
	}
	err := p.Validate()
	if err == nil {
		t.Fatal("expected error for forward reference in needs")
	}
	if !strings.Contains(err.Error(), "not defined before") {
		t.Fatalf("error should mention forward reference, got: %v", err)
	}
}

func TestNeedsSelfReference(t *testing.T) {
	p := &Pipeline{
		Agent: AgentConfig{Cmd: "echo"},
		Steps: []Step{
			{Name: "step1", Prompt: "one", Needs: []string{"step1"}},
		},
	}
	err := p.Validate()
	if err == nil {
		t.Fatal("expected error for self-reference in needs")
	}
}

func TestNeedsCycleDetection(t *testing.T) {
	// A cycle requires forward references, which are caught earlier.
	// But let's test the cycle detection directly by checking a valid
	// looking structure that would cycle if forward refs were allowed.
	// Since our validation catches forward refs first, a true cycle
	// through needs alone is impossible (you can't reference a step
	// declared after you). This test verifies that invariant holds.
	p := &Pipeline{
		Agent: AgentConfig{Cmd: "echo"},
		Steps: []Step{
			{Name: "a", Prompt: "a", Needs: []string{"c"}},
			{Name: "b", Prompt: "b", Needs: []string{"a"}},
			{Name: "c", Prompt: "c", Needs: []string{"b"}},
		},
	}
	err := p.Validate()
	if err == nil {
		t.Fatal("expected error for cycle (caught as forward reference)")
	}
}

func TestNeedsDuplicateStepNames(t *testing.T) {
	p := &Pipeline{
		Agent: AgentConfig{Cmd: "echo"},
		Steps: []Step{
			{Name: "step1", Prompt: "one"},
			{Name: "step1", Prompt: "two"},
		},
	}
	err := p.Validate()
	if err == nil {
		t.Fatal("expected error for duplicate step names")
	}
	if !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("error should mention 'duplicate', got: %v", err)
	}
}

func TestNeedsMultipleDeps(t *testing.T) {
	p := &Pipeline{
		Agent: AgentConfig{Cmd: "echo"},
		Steps: []Step{
			{Name: "a", Prompt: "a"},
			{Name: "b", Prompt: "b"},
			{Name: "c", Prompt: "c"},
			{Name: "d", Prompt: "d", Needs: []string{"a", "b", "c"}},
		},
	}
	if err := p.Validate(); err != nil {
		t.Fatalf("expected valid pipeline with multiple deps, got: %v", err)
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
