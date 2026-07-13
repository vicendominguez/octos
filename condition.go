package main

import (
	"errors"
	"fmt"
	"strings"
)

// conditionOp represents the operator in a parsed condition.
type conditionOp int

const (
	opContains conditionOp = iota
	opEquals
	opNotEmpty
)

// parsedCondition is the structured representation of a when condition.
type parsedCondition struct {
	op    conditionOp
	left  string
	right string // empty for opNotEmpty
}

// errUnparseableCondition is returned when a condition string uses unsupported syntax.
var errUnparseableCondition = errors.New("unparseable condition")

// parseCondition parses a condition string into a structured representation.
// It searches from the END of the string to avoid false matches when
// interpolated output contains operator keywords like " contains ".
//
// Supported forms:
//   - "<expr> contains <value>"
//   - "<expr> equals <value>"
//   - "<expr> not_empty"
//   - "not_empty"  (bare form, left is empty)
func parseCondition(cond string) (*parsedCondition, error) {
	cond = strings.TrimSpace(cond)
	if cond == "" {
		return nil, fmt.Errorf("%w: empty condition", errUnparseableCondition)
	}

	// Check not_empty first (bare or suffixed)
	if cond == "not_empty" {
		return &parsedCondition{op: opNotEmpty, left: ""}, nil
	}
	if strings.HasSuffix(cond, " not_empty") {
		left := cond[:len(cond)-len(" not_empty")]
		return &parsedCondition{op: opNotEmpty, left: left}, nil
	}

	// Check " contains " from the end to avoid false matches
	if idx := strings.LastIndex(cond, " contains "); idx >= 0 {
		left := cond[:idx]
		right := cond[idx+len(" contains "):]
		return &parsedCondition{op: opContains, left: left, right: right}, nil
	}

	// Check " equals " from the end
	if idx := strings.LastIndex(cond, " equals "); idx >= 0 {
		left := cond[:idx]
		right := cond[idx+len(" equals "):]
		return &parsedCondition{op: opEquals, left: left, right: right}, nil
	}

	return nil, fmt.Errorf("%w: %q", errUnparseableCondition, cond)
}

// evaluateCondition checks if a when condition is met.
// It replaces step output and artifact variables, then parses and evaluates the condition.
func evaluateCondition(condition string, outputs map[string]string, artifacts map[string]string) bool {
	if condition == "" {
		return true
	}

	// Replace variables
	cond := condition
	for name, output := range outputs {
		cond = strings.ReplaceAll(cond, fmt.Sprintf("{{%s.output}}", name), output)
	}
	for name, content := range artifacts {
		cond = strings.ReplaceAll(cond, fmt.Sprintf("{{artifact.%s}}", name), content)
	}

	parsed, err := parseCondition(cond)
	if err != nil {
		// Unparseable conditions default to true (matches original behavior)
		return true
	}

	switch parsed.op {
	case opNotEmpty:
		value := strings.TrimSpace(parsed.left)
		return value != ""
	case opContains:
		haystack := strings.Trim(parsed.left, "' \"")
		needle := strings.Trim(parsed.right, "' \"")
		return strings.Contains(strings.ToLower(haystack), strings.ToLower(needle))
	case opEquals:
		left := strings.Trim(parsed.left, "' \"")
		right := strings.Trim(parsed.right, "' \"")
		return left == right
	}

	return true
}

// isValidConditionSyntax checks if a when condition uses supported syntax.
// It delegates to parseCondition — valid if parseable.
func isValidConditionSyntax(condition string) bool {
	_, err := parseCondition(strings.TrimSpace(condition))
	return err == nil
}
