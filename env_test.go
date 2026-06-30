package main

import (
	"os"
	"testing"
)

func TestExpandEnvWithDefaults(t *testing.T) {
	os.Setenv("TEST_VAR", "hello")
	defer os.Unsetenv("TEST_VAR")
	defer os.Unsetenv("EMPTY_VAR")

	tests := []struct {
		name   string
		input  string
		setup  func()
		expect string
	}{
		{"simple var", "${TEST_VAR}", nil, "hello"},
		{"var with default, var set", "${TEST_VAR:-fallback}", nil, "hello"},
		{"var with default, var unset", "${UNSET_VAR:-fallback}", nil, "fallback"},
		{"var with default, var empty", "${EMPTY_VAR:-fallback}", func() { os.Setenv("EMPTY_VAR", "") }, "fallback"},
		{"no default, var unset", "${UNSET_VAR}", nil, ""},
		{"dollar without braces", "$TEST_VAR", nil, "hello"},
		{"multiple vars", "${TEST_VAR}-${UNSET_VAR:-world}", nil, "hello-world"},
		{"default with special chars", "${UNSET:-http://localhost:3000}", nil, "http://localhost:3000"},
		{"default with commas", "${UNSET:-a,b,c}", nil, "a,b,c"},
		{"no expansion needed", "plain text", nil, "plain text"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setup != nil {
				tt.setup()
			}
			got := expandEnvWithDefaults(tt.input)
			if got != tt.expect {
				t.Errorf("expandEnvWithDefaults(%q) = %q, want %q", tt.input, got, tt.expect)
			}
		})
	}
}
