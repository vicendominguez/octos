// Package main — graph.go renders an ASCII DAG visualization of the pipeline.
// It computes topological levels from step dependencies and draws nodes with
// fan-out/fan-in connector lines.
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

	// Calculate node label widths and positions
	const gap = 3 // space between nodes on same level
	nodeWidth := make(map[string]int)
	for _, step := range steps {
		icon := GetStepIcon(statusMap[step.Name])
		label := fmt.Sprintf("[%s %s]", icon, step.Name)
		nodeWidth[step.Name] = len([]rune(label))
	}

	// Calculate the center position of each node
	nodeCenter := make(map[string]int)
	for _, group := range levelGroups {
		pos := 0
		for i, name := range group {
			w := nodeWidth[name]
			nodeCenter[name] = pos + w/2
			pos += w
			if i < len(group)-1 {
				pos += gap
			}
		}
	}

	var out strings.Builder

	for lvl := 0; lvl <= maxLevel; lvl++ {
		group := levelGroups[lvl]

		// Draw connectors from previous level
		if lvl > 0 {
			prevGroup := levelGroups[lvl-1]
			connector := buildConnector(prevGroup, group, children, nodeCenter)
			out.WriteString(connector)
		}

		// Draw node row
		out.WriteString(buildNodeRow(group, statusMap))
		out.WriteString("\n")
	}

	return out.String()
}

// buildNodeRow renders a row of nodes.
func buildNodeRow(names []string, statusMap map[string]StepStatus) string {
	const gap = 3
	parts := make([]string, len(names))
	for i, name := range names {
		icon := GetStepIcon(statusMap[name])
		parts[i] = fmt.Sprintf("[%s %s]", icon, name)
	}
	return strings.Join(parts, strings.Repeat(" ", gap))
}

// buildConnector draws lines between a parent level and child level.
func buildConnector(prevGroup []string, currGroup []string, children map[string][]string, nodeCenter map[string]int) string {
	// Single → Single
	if len(prevGroup) == 1 && len(currGroup) == 1 {
		center := nodeCenter[prevGroup[0]]
		return placeCh(center, '|') + "\n"
	}

	// Single → Multiple (fan-out)
	if len(prevGroup) == 1 && len(currGroup) > 1 {
		return buildFanOut(prevGroup[0], currGroup, nodeCenter)
	}

	// Multiple → Single (fan-in)
	if len(prevGroup) > 1 && len(currGroup) == 1 {
		return buildFanIn(prevGroup, currGroup[0], children, nodeCenter)
	}

	// Fallback: pipe from first parent
	center := nodeCenter[prevGroup[0]]
	return placeCh(center, '|') + "\n"
}

// buildFanOut: one parent splits to multiple children.
func buildFanOut(parent string, childNames []string, nodeCenter map[string]int) string {
	parentCenter := nodeCenter[parent]
	var out strings.Builder

	// Vertical pipe from parent center
	out.WriteString(placeCh(parentCenter, '|'))
	out.WriteString("\n")

	// Horizontal branch line: from parent center or leftmost child (whichever is left) to rightmost child
	leftmost := nodeCenter[childNames[0]]
	rightmost := nodeCenter[childNames[len(childNames)-1]]

	// Extend to include parent center
	start := leftmost
	if parentCenter < start {
		start = parentCenter
	}

	width := rightmost + 1
	line := make([]byte, width)
	for i := range line {
		line[i] = ' '
	}
	// Fill horizontal between start and rightmost
	for i := start; i <= rightmost; i++ {
		line[i] = '-'
	}
	// Place connector at parent center
	line[parentCenter] = '+'
	// Place connectors at each child center
	for _, name := range childNames {
		pos := nodeCenter[name]
		line[pos] = '+'
	}
	out.Write(line)
	out.WriteString("\n")

	// Down pipes at each child position
	pipeLine := make([]byte, width)
	for i := range pipeLine {
		pipeLine[i] = ' '
	}
	for _, name := range childNames {
		pos := nodeCenter[name]
		pipeLine[pos] = '|'
	}
	out.Write(pipeLine)
	out.WriteString("\n")

	return out.String()
}

// buildFanIn: multiple parents merge to one child.
func buildFanIn(parentNames []string, child string, children map[string][]string, nodeCenter map[string]int) string {
	childCenter := nodeCenter[child]
	var out strings.Builder

	leftmost := nodeCenter[parentNames[0]]
	rightmost := nodeCenter[parentNames[len(parentNames)-1]]

	width := rightmost + 1
	if childCenter >= width {
		width = childCenter + 1
	}

	// Up pipes at each parent position
	pipeLine := make([]byte, width)
	for i := range pipeLine {
		pipeLine[i] = ' '
	}
	for _, name := range parentNames {
		pos := nodeCenter[name]
		pipeLine[pos] = '|'
	}
	out.Write(pipeLine)
	out.WriteString("\n")

	// Horizontal merge line
	line := make([]byte, width)
	for i := range line {
		line[i] = ' '
	}
	// Extend to include child center
	start := leftmost
	if childCenter < start {
		start = childCenter
	}
	for i := start; i <= rightmost; i++ {
		line[i] = '-'
	}
	// Place connectors at parent centers
	for _, name := range parentNames {
		pos := nodeCenter[name]
		line[pos] = '+'
	}
	// Place connector at child center
	line[childCenter] = '+'
	out.Write(line)
	out.WriteString("\n")

	// Down pipe to child center
	out.WriteString(placeCh(childCenter, '|'))
	out.WriteString("\n")

	return out.String()
}

// placeCh returns a string with a character placed at a specific column position.
func placeCh(col int, ch rune) string {
	if col <= 0 {
		return string(ch)
	}
	return strings.Repeat(" ", col) + string(ch)
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
