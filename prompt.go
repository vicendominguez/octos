// Package main — prompt.go contains pure functions for building and interpolating
// prompts. buildPrompt assembles the final prompt sent to the agent, interpolate
// resolves {{placeholders}} in prompt text, and buildArgs constructs CLI arguments.
package main

import (
	"bytes"
	"fmt"
	"regexp"
	"slices"
	"strings"
)

var unresolvedPlaceholderRegex = regexp.MustCompile(`\{\{[a-zA-Z_][a-zA-Z0-9_.]*\}\}`)

func buildPrompt(ctx *PipelineContext, newTask string) string {
	var buf bytes.Buffer

	if len(ctx.Global) > 0 {
		buf.WriteString("=== CONTEXT ===\n")
		for k, v := range ctx.Global {
			buf.WriteString(fmt.Sprintf("%s: %v\n", k, v))
		}
		buf.WriteString("\n")
	}

	buf.WriteString("=== TASK ===\n")
	buf.WriteString(newTask)

	return buf.String()
}

func interpolate(text string, ctx *PipelineContext) (string, []string) {
	result := text
	var warnings []string

	for name, output := range ctx.Outputs {
		// {{step_name.output}} for step outputs
		placeholder := fmt.Sprintf("{{%s.output}}", name)
		result = strings.ReplaceAll(result, placeholder, output)
		// {{artifact.X}} directly (when stored as ctx.Outputs["artifact.X"])
		direct := fmt.Sprintf("{{%s}}", name)
		result = strings.ReplaceAll(result, direct, output)
	}

	// Interpolate all context values
	for key, value := range ctx.Global {
		placeholder := fmt.Sprintf("{{context.%s}}", key)
		switch v := value.(type) {
		case []any:
			var lines []string
			for _, item := range v {
				lines = append(lines, fmt.Sprintf("- %v", item))
			}
			result = strings.ReplaceAll(result, placeholder, strings.Join(lines, "\n"))
		default:
			result = strings.ReplaceAll(result, placeholder, fmt.Sprintf("%v", v))
		}
	}

	// Collect unresolved placeholders as warnings
	if matches := unresolvedPlaceholderRegex.FindAllString(result, -1); len(matches) > 0 {
		for _, m := range matches {
			warnings = append(warnings, fmt.Sprintf("warning: unresolved placeholder %s\n  hint: check spelling or ensure the referenced step runs before this one", m))
		}
	}

	return result, warnings
}

func buildArgs(agent AgentConfig, prompt string) []string {
	if agent.Stdin {
		return agent.Args
	}
	return append(slices.Clone(agent.Args), prompt)
}
