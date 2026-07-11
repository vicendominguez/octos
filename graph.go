package main

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
)

// GraphNode represents a step in the dependency graph with layout info.
type GraphNode struct {
	Name   string
	Level  int
	Column int
	Status StepStatus
}

// RenderGraph produces a styled ASCII representation of the pipeline DAG.
// If no step has needs, it renders a simple linear chain.
func RenderGraph(steps []Step, stepStates []StepState) string {
	if len(steps) == 0 {
		return ""
	}

	// Build adjacency: step → list of dependents (children)
	children := make(map[string][]string)
	parents := make(map[string][]string)
	for _, step := range steps {
		for _, dep := range step.Needs {
			children[dep] = append(children[dep], step.Name)
			parents[step.Name] = append(parents[step.Name], dep)
		}
	}

	// Assign topological levels
	levels := assignLevels(steps, parents)

	// Group nodes by level
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

	// Build status map
	statusMap := make(map[string]StepStatus)
	for _, st := range stepStates {
		statusMap[st.Name] = st.Status
	}

	// Assign column positions within each level
	columns := make(map[string]int)
	for _, group := range levelGroups {
		for col, name := range group {
			columns[name] = col
		}
	}

	// Render
	var out strings.Builder

	for lvl := 0; lvl <= maxLevel; lvl++ {
		group := levelGroups[lvl]

		// Draw edges from previous level to this level
		if lvl > 0 {
			edgeLine := renderEdges(levelGroups[lvl-1], group, children, columns)
			if edgeLine != "" {
				out.WriteString(edgeLine)
				out.WriteString("\n")
			}
		}

		// Draw nodes at this level
		nodeLine := renderNodes(group, statusMap)
		out.WriteString(nodeLine)
		out.WriteString("\n")
	}

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
		// Linear: each step is its own level
		for i, step := range steps {
			levels[step.Name] = i
		}
		return levels
	}

	// BFS/dynamic programming: level = max(parent levels) + 1
	for _, step := range steps {
		computeLevel(step.Name, steps, parents, levels)
	}

	return levels
}

func computeLevel(name string, steps []Step, parents map[string][]string, levels map[string]int) int {
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
		parentLvl := computeLevel(dep, steps, parents, levels)
		if parentLvl > maxParent {
			maxParent = parentLvl
		}
	}

	levels[name] = maxParent + 1
	return maxParent + 1
}

// renderNodes draws a row of nodes with their status styling.
func renderNodes(names []string, statusMap map[string]StepStatus) string {
	const nodeSpacing = 4

	parts := make([]string, len(names))
	for i, name := range names {
		status := statusMap[name]
		icon := GetStepIcon(status)
		style := GetStepStatusStyle(status)
		label := fmt.Sprintf(" %s %s ", icon, name)
		parts[i] = style.Render(label)
	}

	return strings.Join(parts, strings.Repeat(" ", nodeSpacing))
}

// renderEdges draws connection lines between parent and child levels.
func renderEdges(parentNames []string, childNames []string, children map[string][]string, columns map[string]int) string {
	if len(parentNames) == 0 || len(childNames) == 0 {
		return ""
	}

	// For simple cases, use centered pipe characters
	// Build a map of which parent connects to which child
	type edge struct {
		parentCol int
		childCol  int
	}

	var edges []edge
	for _, pName := range parentNames {
		pCol := columns[pName]
		for _, cName := range children[pName] {
			// Only draw edges to children in this level
			for _, cn := range childNames {
				if cn == cName {
					cCol := columns[cName]
					edges = append(edges, edge{pCol, cCol})
				}
			}
		}
	}

	if len(edges) == 0 {
		// Might be a linear sequence step with no explicit needs
		// Draw a simple pipe for each child that has a parent in the prev level
		if len(childNames) == 1 && len(parentNames) == 1 {
			return edgeStyle().Render("       │")
		}
		return ""
	}

	// Simple rendering: for each edge draw appropriate connector
	// Single parent → single child: │
	// Single parent → multiple children: fan out with ├──┼──┤
	// Multiple parents → single child: fan in with └──┴──┘

	if len(parentNames) == 1 && len(childNames) > 1 {
		// Fan-out: one parent splits to many
		return renderFanOut(childNames)
	}

	if len(parentNames) > 1 && len(childNames) == 1 {
		// Fan-in: many parents merge to one
		return renderFanIn(parentNames)
	}

	// Mixed or simple: just draw pipes
	var pipes []string
	for range childNames {
		pipes = append(pipes, edgeStyle().Render("│"))
	}
	return "       " + strings.Join(pipes, "          ")
}

func renderFanOut(childNames []string) string {
	if len(childNames) == 0 {
		return ""
	}

	// Calculate spacing based on node label widths
	const nodeSpacing = 4
	avgWidth := 10 // approximate average label width

	totalWidth := len(childNames)*avgWidth + (len(childNames)-1)*nodeSpacing
	mid := totalWidth / 2

	// Build the fan-out line
	line1 := strings.Repeat(" ", mid) + edgeStyle().Render("│")

	// Build the branch line
	segments := make([]string, len(childNames))
	for i := range childNames {
		if i == 0 {
			segments[i] = "┌"
		} else if i == len(childNames)-1 {
			segments[i] = "┐"
		} else {
			segments[i] = "┬"
		}
	}

	branchFill := strings.Repeat("─", avgWidth+nodeSpacing-1)
	var branch strings.Builder
	for i, seg := range segments {
		branch.WriteString(seg)
		if i < len(segments)-1 {
			branch.WriteString(branchFill)
		}
	}

	// Build the down-pipes line
	pipes := make([]string, len(childNames))
	for i := range childNames {
		pipes[i] = "│"
	}
	pipeFill := strings.Repeat(" ", avgWidth+nodeSpacing-1)
	var pipeLine strings.Builder
	for i, p := range pipes {
		pipeLine.WriteString(p)
		if i < len(pipes)-1 {
			pipeLine.WriteString(pipeFill)
		}
	}

	return line1 + "\n" +
		edgeStyle().Render(branch.String()) + "\n" +
		edgeStyle().Render(pipeLine.String())
}

func renderFanIn(parentNames []string) string {
	if len(parentNames) == 0 {
		return ""
	}

	const nodeSpacing = 4
	avgWidth := 10

	totalWidth := len(parentNames)*avgWidth + (len(parentNames)-1)*nodeSpacing
	mid := totalWidth / 2

	// Build the up-pipes line
	pipes := make([]string, len(parentNames))
	for i := range parentNames {
		pipes[i] = "│"
	}
	pipeFill := strings.Repeat(" ", avgWidth+nodeSpacing-1)
	var pipeLine strings.Builder
	for i, p := range pipes {
		pipeLine.WriteString(p)
		if i < len(pipes)-1 {
			pipeLine.WriteString(pipeFill)
		}
	}

	// Build the merge line
	segments := make([]string, len(parentNames))
	for i := range parentNames {
		if i == 0 {
			segments[i] = "└"
		} else if i == len(parentNames)-1 {
			segments[i] = "┘"
		} else {
			segments[i] = "┴"
		}
	}
	branchFill := strings.Repeat("─", avgWidth+nodeSpacing-1)
	var branch strings.Builder
	for i, seg := range segments {
		branch.WriteString(seg)
		if i < len(segments)-1 {
			branch.WriteString(branchFill)
		}
	}

	// Down pipe
	line3 := strings.Repeat(" ", mid) + "│"

	return edgeStyle().Render(pipeLine.String()) + "\n" +
		edgeStyle().Render(branch.String()) + "\n" +
		edgeStyle().Render(line3)
}

func edgeStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(neonCyan).Faint(true)
}
