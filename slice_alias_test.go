package main

import (
	"context"
	"testing"
)

func TestBuildArgsDoesNotCorruptOriginal(t *testing.T) {
	agent := AgentConfig{
		Cmd:  "echo",
		Args: []string{"fixed-arg"},
	}

	// First call
	args1 := buildArgs(agent, "prompt1")
	// Second call with same agent
	args2 := buildArgs(agent, "prompt2")

	// Original args must be untouched
	if len(agent.Args) != 1 || agent.Args[0] != "fixed-arg" {
		t.Fatalf("agent.Args corrupted: %v", agent.Args)
	}

	// Each result must be independent
	if args1[len(args1)-1] != "prompt1" {
		t.Errorf("args1 last element should be 'prompt1', got %q", args1[len(args1)-1])
	}
	if args2[len(args2)-1] != "prompt2" {
		t.Errorf("args2 last element should be 'prompt2', got %q", args2[len(args2)-1])
	}
}

func TestBuildArgsStdinReturnsOriginal(t *testing.T) {
	agent := AgentConfig{
		Cmd:   "cat",
		Args:  []string{},
		Stdin: true,
	}

	args := buildArgs(agent, "ignored prompt")
	if len(args) != 0 {
		t.Errorf("stdin agent should return original args, got %v", args)
	}
}

func TestRunAgentReuseSameConfig(t *testing.T) {
	agent := AgentConfig{
		Cmd:  "echo",
		Args: []string{"hello"},
	}

	// Run twice with same agent
	out1, err := runAgent(context.Background(), agent, "world1", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	out2, err := runAgent(context.Background(), agent, "world2", nil, 0)
	if err != nil {
		t.Fatal(err)
	}

	// agent.Args must still be ["hello"]
	if len(agent.Args) != 1 || agent.Args[0] != "hello" {
		t.Fatalf("agent.Args corrupted after reuse: %v", agent.Args)
	}

	if out1 == out2 {
		t.Errorf("outputs should differ: %q vs %q", out1, out2)
	}
}
