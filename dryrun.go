package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// DryRun validates the pipeline without executing any agents.
// It resolves env vars and context, checks interpolation, evaluates conditions
// where possible, and reports the execution plan.
func DryRun(p *Pipeline) error {
	var errors []string
	var warnings []string

	// Header
	fmt.Printf("Pipeline: %s (%d steps)\n", p.File, len(p.Steps))
	fmt.Printf("Agent: %s %v", p.Agent.Cmd, p.Agent.Args)
	if p.Agent.Stdin {
		fmt.Print(" (stdin: true)")
	}
	fmt.Println()
	fmt.Println()

	// Check agent binary exists in PATH
	if _, err := lookupBinary(p.Agent.Cmd); err != nil {
		warnings = append(warnings, fmt.Sprintf("agent binary '%s' not found in PATH", p.Agent.Cmd))
	}

	// Build context for interpolation checking
	ctx := &Context{
		Global:  p.Context,
		Outputs: make(map[string]string),
	}

	// Track which steps produce artifacts and outputs
	producedArtifacts := make(map[string]string) // artifact file → step name
	producedOutputs := make(map[string]bool)     // step name → true

	fmt.Println("Steps:")
	for i, step := range p.Steps {
		status := "●"
		notes := []string{}

		// Check per-step agent
		agent := p.Agent
		if step.Agent != nil {
			agent = *step.Agent
			if _, err := lookupBinary(agent.Cmd); err != nil {
				warnings = append(warnings, fmt.Sprintf("step '%s': agent binary '%s' not found in PATH", step.Name, agent.Cmd))
			}
		}

		// Evaluate when condition
		if step.When != "" {
			canEvaluate := canEvaluateCondition(step.When, producedOutputs)
			if canEvaluate {
				// All referenced outputs are from prior steps — we can check syntax
				if !isValidConditionSyntax(step.When) {
					warnings = append(warnings, fmt.Sprintf("step '%s': unparseable 'when' condition: '%s'", step.Name, step.When))
				}
				notes = append(notes, fmt.Sprintf("when: \"%s\"", step.When))
			} else {
				status = "○"
				notes = append(notes, fmt.Sprintf("when: \"%s\"  [runtime: depends on prior output]", step.When))
			}
		}

		// Check load_from
		if step.LoadFrom != "" {
			artifactPath := filepath.Join(".octos", "artifacts", step.LoadFrom)
			if _, err := os.Stat(artifactPath); err != nil {
				if producer, ok := producedArtifacts[step.LoadFrom]; ok {
					notes = append(notes, fmt.Sprintf("load_from: %s (produced by '%s')", step.LoadFrom, producer))
				} else {
					warnings = append(warnings, fmt.Sprintf("step '%s': load_from '%s' not found and no prior step produces it", step.Name, step.LoadFrom))
				}
			}
		}

		// Check interpolation in prompt
		promptWarnings := checkInterpolation(step.Prompt, ctx, producedOutputs, producedArtifacts)
		for _, w := range promptWarnings {
			warnings = append(warnings, fmt.Sprintf("step '%s': %s", step.Name, w))
		}

		// Mark this step's outputs as available for subsequent steps
		producedOutputs[step.Name] = true
		if step.SaveTo != "" {
			producedArtifacts[step.SaveTo] = step.Name
		}

		// Print step line
		notesStr := ""
		if len(notes) > 0 {
			notesStr = "  " + strings.Join(notes, "  ")
		}
		fmt.Printf("  %d. %s %s%s\n", i+1, status, step.Name, notesStr)
	}

	// Context summary
	fmt.Println()
	if len(p.Context) > 0 {
		fmt.Println("Context:")
		for k, v := range p.Context {
			switch val := v.(type) {
			case string:
				if val == "" {
					warnings = append(warnings, fmt.Sprintf("context.%s is empty (env var not set?)", k))
					fmt.Printf("  ⚠ %s = [empty]\n", k)
				} else if strings.Contains(val, "${") {
					// Unexpanded env var
					errors = append(errors, fmt.Sprintf("context.%s contains unexpanded variable: %s", k, val))
					fmt.Printf("  ✗ %s = %s\n", k, val)
				} else {
					display := val
					if looksLikeSecret(k) {
						display = "[set]"
					}
					fmt.Printf("  ✓ %s = %s\n", k, display)
				}
			default:
				fmt.Printf("  ✓ %s = %v\n", k, v)
			}
		}
	}

	// Print warnings and errors
	if len(warnings) > 0 {
		fmt.Println()
		fmt.Println("Warnings:")
		for _, w := range warnings {
			fmt.Printf("  ⚠ %s\n", w)
		}
	}

	if len(errors) > 0 {
		fmt.Println()
		fmt.Println("Errors:")
		for _, e := range errors {
			fmt.Printf("  ✗ %s\n", e)
		}
		return fmt.Errorf("dry-run found %d error(s)", len(errors))
	}

	fmt.Println()
	fmt.Println("✓ Pipeline is valid. No errors found.")
	return nil
}

// lookupBinary checks if a binary exists in PATH or is an absolute/relative path
func lookupBinary(name string) (string, error) {
	// If it's a path (absolute or relative), check directly
	if strings.Contains(name, "/") {
		if _, err := os.Stat(name); err != nil {
			return "", err
		}
		return name, nil
	}
	// Search in PATH
	path := os.Getenv("PATH")
	for _, dir := range filepath.SplitList(path) {
		full := filepath.Join(dir, name)
		if _, err := os.Stat(full); err == nil {
			return full, nil
		}
	}
	return "", fmt.Errorf("not found")
}

// canEvaluateCondition checks if all step references in a when condition are from prior steps
func canEvaluateCondition(condition string, producedOutputs map[string]bool) bool {
	// Extract step references: {{stepname.output}} patterns
	matches := unresolvedPlaceholderRegex.FindAllString(condition, -1)
	for _, m := range matches {
		// Strip {{ and }}
		ref := m[2 : len(m)-2]
		// Get the step name (before .output or .anything)
		parts := strings.SplitN(ref, ".", 2)
		stepName := parts[0]
		if stepName == "context" || stepName == "artifact" {
			continue
		}
		if !producedOutputs[stepName] {
			return false
		}
	}
	return true
}

// isValidConditionSyntax checks if a when condition uses supported syntax
func isValidConditionSyntax(condition string) bool {
	cond := strings.TrimSpace(condition)
	if strings.Contains(cond, " contains ") {
		return true
	}
	if strings.Contains(cond, " equals ") {
		return true
	}
	if cond == "not_empty" || strings.HasSuffix(cond, " not_empty") {
		return true
	}
	return false
}

// checkInterpolation validates placeholders in a prompt without resolving runtime values
func checkInterpolation(text string, ctx *Context, producedOutputs map[string]bool, producedArtifacts map[string]string) []string {
	var warnings []string
	matches := unresolvedPlaceholderRegex.FindAllString(text, -1)
	for _, m := range matches {
		ref := m[2 : len(m)-2] // strip {{ and }}

		// context.X — should be resolvable now
		if strings.HasPrefix(ref, "context.") {
			key := strings.TrimPrefix(ref, "context.")
			if _, ok := ctx.Global[key]; !ok {
				warnings = append(warnings, fmt.Sprintf("unresolved %s (not defined in context)", m))
			}
			continue
		}

		// artifact.X — check if a prior step produces it
		if strings.HasPrefix(ref, "artifact.") {
			artifactName := strings.TrimPrefix(ref, "artifact.")
			found := false
			for file, _ := range producedArtifacts {
				nameWithoutExt := strings.TrimSuffix(file, filepath.Ext(file))
				if nameWithoutExt == artifactName {
					found = true
					break
				}
			}
			if !found {
				// Check if the artifact file exists on disk
				for _, ext := range []string{".txt", ".md", ""} {
					path := filepath.Join(".octos", "artifacts", artifactName+ext)
					if _, err := os.Stat(path); err == nil {
						found = true
						break
					}
				}
			}
			if !found {
				warnings = append(warnings, fmt.Sprintf("%s — no prior step produces this artifact", m))
			}
			continue
		}

		// stepname.output or stepname — check if step exists
		parts := strings.SplitN(ref, ".", 2)
		stepName := parts[0]
		if !producedOutputs[stepName] {
			warnings = append(warnings, fmt.Sprintf("%s — referenced step has not run yet at this point", m))
		}
	}
	return warnings
}

// looksLikeSecret heuristic for masking values in dry-run output
func looksLikeSecret(key string) bool {
	lower := strings.ToLower(key)
	return strings.Contains(lower, "token") ||
		strings.Contains(lower, "secret") ||
		strings.Contains(lower, "password") ||
		strings.Contains(lower, "key") ||
		strings.Contains(lower, "api_key")
}
