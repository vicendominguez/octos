package main

import (
	"fmt"
	"strings"
)

// RenderGraph produces an ASCII representation of the pipeline DAG.
// If no step has needs, it renders a simple vertical chain.
func RenderGraph(steps []Step, stepStates []StepState) string {
	if len(steps) == 0 {
		return ""
	}

	// Build parent/children maps
	parents := make(map[string][]string)
	children := make(map[string][]string)
	for _, step := range steps {
		for _, dep := range step.Needs {
			children[dep] = append(children[dep], step.Name)
			parents[step.Name] = append(parents[step.Name], dep)
		}
	}

	// Build status map
	statusMap := make(map[string]StepStatus)
	for _, st := range stepStates {
		statusMap[st.Name] = st.Status
	}

	// Assign topological levels
	levels := assignLevels(steps, parents)

	// Group by level
	maxLevel := 0
	for _, lvl := range levels {
		if lvl > maxLevel {
			maxLevel = lvl
		}
	}

	levelGroups := make([][]string, maxLevel+1)
	for _, step := range steps {
		lvl := levels[step.Name]
		levelGroups[lvl] = append(levelGroups[lvl], step.Name)
	}

	var out strings.Builder

	for lvl := 0; lvl <= maxLevel; lvl++ {
		group := levelGroups[lvl]

		// Render connector lines from previous level
		if lvl > 0 {
			prevGroup := levelGroups[lvl-1]
			connector := renderConnector(prevGroup, group, children)
			if connector != "" {
				out.WriteString(connector)
			}
		}

		// Render nodes at this level
		out.WriteString(renderNodeRow(group, statusMap))
		out.WriteString("\n")
	}

	return out.String()
}

// renderNodeRow renders a row of nodes with status icons.
func renderNodeRow(names []string, statusMap map[string]StepStatus) string {
	if len(names) == 1 {
		icon := GetStepIcon(statusMap[names[0]])
		return fmt.Sprintf("  [%s %s]", icon, names[0])
	}

	// Multiple nodes on same level
	parts := make([]string, len(names))
	for i, name := range names {
		icon := GetStepIcon(statusMap[name])
		parts[i] = fmt.Sprintf("[%s %s]", icon, name)
	}
	return "  " + strings.Join(parts, "   ")
}

// renderConnector draws simple ASCII lines between levels.
func renderConnector(prevNames []string, currNames []string, children map[string][]string) string {
	// Single → Single: simple pipe
	if len(prevNames) == 1 && len(currNames) == 1 {
		return "    │\n"
	}

	// Single → Multiple (fan-out)
	if len(prevNames) == 1 && len(currNames) > 1 {
		return renderFanOutSimple(currNames)
	}

	// Multiple → Single (fan-in)
	if len(prevNames) > 1 && len(currNames) == 1 {
		return renderFanInSimple(prevNames)
	}

	// Multiple → Multiple or mixed: just pipes
	return "    │\n"
}

func renderFanOutSimple(names []string) string {
	n := len(names)
	if n == 0 {
		return ""
	}

	var out strings.Builder

	// Center pipe from parent
	out.WriteString("    │\n")

	// Branch line: ┌───┬───┐ or similar
	out.WriteString("  ")
	for i := 0; i < n; i++ {
		if i == 0 {
			out.WriteString("┌")
		} else if i == n-1 {
			out.WriteString("┐")
		} else {
			out.WriteString("┬")
		}
		if i < n-1 {
			out.WriteString("───────")
		}
	}
	out.WriteString("\n")

	// Down pipes
	out.WriteString("  ")
	for i := 0; i < n; i++ {
		out.WriteString("│")
		if i < n-1 {
			out.WriteString("       ")
		}
	}
	out.WriteString("\n")

	return out.String()
}

func renderFanInSimple(names []string) string {
	n := len(names)
	if n == 0 {
		return ""
	}

	var out strings.Builder

	// Up pipes
	out.WriteString("  ")
	for i := 0; i < n; i++ {
		out.WriteString("│")
		if i < n-1 {
			out.WriteString("       ")
		}
	}
	out.WriteString("\n")

	// Merge line: └───┴───┘
	out.WriteString("  ")
	for i := 0; i < n; i++ {
		if i == 0 {
			out.WriteString("└")
		} else if i == n-1 {
			out.WriteString("┘")
		} else {
			out.WriteString("┴")
		}
		if i < n-1 {
			out.WriteString("───────")
		}
	}
	out.WriteString("\n")

	// Center pipe to child
	out.WriteString("    │\n")

	return out.String()
}

// assignLevels computes the topological level of each step.
// Level 0 = no dependencies (or no needs declared).
// If no step has needs, each step gets its own level (linear).
func assignLevels(steps []Step, parents map[string][]string) map[string]int {
	hasAnyNeeds := false
	for _, step := range steps {
		if len(step.Needs) > 0 {
			hasAnyNeeds = true
			break
		}
	}

	levels := make(map[string]int)

	if !hasAnyNeeds {
		for i, step := range steps {
			levels[step.Name] = i
		}
		return levels
	}

	for _, step := range steps {
		computeLevel(step.Name, parents, levels)
	}

	return levels
}

func computeLevel(name string, parents map[string][]string, levels map[string]int) int {
	if lvl, ok := levels[name]; ok {
		return lvl
	}

	deps := parents[name]
	if len(deps) == 0 {
		levels[name] = 0
		return 0
	}

	maxParent := 0
	for _, dep := range deps {
		parentLvl := computeLevel(dep, parents, levels)
		if parentLvl > maxParent {
			maxParent = parentLvl
		}
	}

	levels[name] = maxParent + 1
	return maxParent + 1
}
