package main

import (
	"strings"
	"testing"
)

func TestRenderGraphLinear(t *testing.T) {
	steps := []Step{
		{Name: "step1"},
		{Name: "step2"},
		{Name: "step3"},
	}
	states := []StepState{
		{Name: "step1", Status: StatusCompleted},
		{Name: "step2", Status: StatusRunning},
		{Name: "step3", Status: StatusPending},
	}

	result := RenderGraph(steps, states)
	if result == "" {
		t.Fatal("expected non-empty graph for linear pipeline")
	}
	if !strings.Contains(result, "step1") {
		t.Fatal("graph should contain step1")
	}
	if !strings.Contains(result, "step2") {
		t.Fatal("graph should contain step2")
	}
	if !strings.Contains(result, "step3") {
		t.Fatal("graph should contain step3")
	}
}

func TestRenderGraphDiamond(t *testing.T) {
	steps := []Step{
		{Name: "implement"},
		{Name: "review", Needs: []string{"implement"}},
		{Name: "security", Needs: []string{"implement"}},
		{Name: "docs", Needs: []string{"implement"}},
		{Name: "release", Needs: []string{"review", "security", "docs"}},
	}
	states := []StepState{
		{Name: "implement", Status: StatusCompleted},
		{Name: "review", Status: StatusCompleted},
		{Name: "security", Status: StatusRunning},
		{Name: "docs", Status: StatusPending},
		{Name: "release", Status: StatusPending},
	}

	result := RenderGraph(steps, states)
	if result == "" {
		t.Fatal("expected non-empty graph for diamond pipeline")
	}
	if !strings.Contains(result, "implement") {
		t.Fatal("graph should contain implement")
	}
	if !strings.Contains(result, "release") {
		t.Fatal("graph should contain release")
	}
}

func TestRenderGraphEmpty(t *testing.T) {
	result := RenderGraph(nil, nil)
	if result != "" {
		t.Fatal("expected empty graph for nil steps")
	}
}

func TestRenderGraphSingleStep(t *testing.T) {
	steps := []Step{{Name: "only"}}
	states := []StepState{{Name: "only", Status: StatusRunning}}

	result := RenderGraph(steps, states)
	if !strings.Contains(result, "only") {
		t.Fatal("graph should contain the single step")
	}
}

func TestAssignLevelsLinear(t *testing.T) {
	steps := []Step{
		{Name: "a"},
		{Name: "b"},
		{Name: "c"},
	}
	parents := make(map[string][]string)

	levels := assignLevels(steps, parents)
	if levels["a"] != 0 || levels["b"] != 1 || levels["c"] != 2 {
		t.Fatalf("expected linear levels 0,1,2, got %v", levels)
	}
}

func TestAssignLevelsDiamond(t *testing.T) {
	steps := []Step{
		{Name: "root"},
		{Name: "left", Needs: []string{"root"}},
		{Name: "right", Needs: []string{"root"}},
		{Name: "merge", Needs: []string{"left", "right"}},
	}
	parents := map[string][]string{
		"left":  {"root"},
		"right": {"root"},
		"merge": {"left", "right"},
	}

	levels := assignLevels(steps, parents)
	if levels["root"] != 0 {
		t.Fatalf("root should be level 0, got %d", levels["root"])
	}
	if levels["left"] != 1 {
		t.Fatalf("left should be level 1, got %d", levels["left"])
	}
	if levels["right"] != 1 {
		t.Fatalf("right should be level 1, got %d", levels["right"])
	}
	if levels["merge"] != 2 {
		t.Fatalf("merge should be level 2, got %d", levels["merge"])
	}
}
