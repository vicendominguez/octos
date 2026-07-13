package main

import (
	"errors"
	"testing"
)

func TestParseCondition(t *testing.T) {
	tests := []struct {
		name    string
		cond    string
		wantOp  conditionOp
		wantL   string
		wantR   string
		wantErr bool
	}{
		{
			name:   "contains operator",
			cond:   "hello world contains hello",
			wantOp: opContains,
			wantL:  "hello world",
			wantR:  "hello",
		},
		{
			name:   "equals operator",
			cond:   "yes equals yes",
			wantOp: opEquals,
			wantL:  "yes",
			wantR:  "yes",
		},
		{
			name:   "not_empty with value",
			cond:   "some text not_empty",
			wantOp: opNotEmpty,
			wantL:  "some text",
		},
		{
			name:   "bare not_empty",
			cond:   "not_empty",
			wantOp: opNotEmpty,
			wantL:  "",
		},
		{
			name:    "empty string",
			cond:    "",
			wantErr: true,
		},
		{
			name:    "no recognized operator",
			cond:    "some random text",
			wantErr: true,
		},
		{
			name:   "contains from end when output has contains keyword",
			cond:   "the output contains stuff contains error",
			wantOp: opContains,
			wantL:  "the output contains stuff",
			wantR:  "error",
		},
		{
			name:   "equals from end when output has equals keyword",
			cond:   "x equals y equals z",
			wantOp: opEquals,
			wantL:  "x equals y",
			wantR:  "z",
		},
		{
			name:   "whitespace is trimmed",
			cond:   "  hello contains world  ",
			wantOp: opContains,
			wantL:  "hello",
			wantR:  "world",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseCondition(tt.cond)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("parseCondition(%q) expected error, got nil", tt.cond)
				}
				if !errors.Is(err, errUnparseableCondition) {
					t.Fatalf("parseCondition(%q) error = %v, want errUnparseableCondition", tt.cond, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseCondition(%q) unexpected error: %v", tt.cond, err)
			}
			if got.op != tt.wantOp {
				t.Errorf("parseCondition(%q).op = %d, want %d", tt.cond, got.op, tt.wantOp)
			}
			if got.left != tt.wantL {
				t.Errorf("parseCondition(%q).left = %q, want %q", tt.cond, got.left, tt.wantL)
			}
			if got.right != tt.wantR {
				t.Errorf("parseCondition(%q).right = %q, want %q", tt.cond, got.right, tt.wantR)
			}
		})
	}
}

func TestEvaluateCondition(t *testing.T) {
	outputs := map[string]string{
		"check": "yes there are outdated packages",
	}
	artifacts := map[string]string{
		"report": "detailed analysis here",
	}

	tests := []struct {
		name      string
		condition string
		want      bool
	}{
		{"empty condition", "", true},
		{"contains match", "{{check.output}} contains outdated", true},
		{"contains no match", "{{check.output}} contains nothing", false},
		{"equals match", "yes equals yes", true},
		{"equals no match", "yes equals no", false},
		{"not_empty with value", "{{check.output}} not_empty", true},
		{"not_empty empty value", " not_empty", false},
		{"unparseable defaults true", "random gibberish", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := evaluateCondition(tt.condition, outputs, artifacts)
			if got != tt.want {
				t.Errorf("evaluateCondition(%q) = %v, want %v", tt.condition, got, tt.want)
			}
		})
	}
}
