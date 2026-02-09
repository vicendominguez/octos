package main

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var ansiRegex = regexp.MustCompile(`\x1b(\[[0-9;]*[a-zA-Z]|\(B|\)0)`)
var spinnerRegex = regexp.MustCompile(`[⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏]\s*`)
var controlCharsRegex = regexp.MustCompile(`[\x00-\x08\x0B-\x0C\x0E-\x1F\x7F]`)
var cursorMovementRegex = regexp.MustCompile(`\x1b\[[0-9]*[ABCDEFGHJKST]`)

// evaluateCondition checks if a when condition is met
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

	// Simple condition evaluation
	if strings.Contains(cond, " contains ") {
		parts := strings.SplitN(cond, " contains ", 2)
		if len(parts) == 2 {
			haystack := strings.Trim(parts[0], "' \"")
			needle := strings.Trim(parts[1], "' \"")
			return strings.Contains(strings.ToLower(haystack), strings.ToLower(needle))
		}
	}
	if strings.Contains(cond, " equals ") {
		parts := strings.SplitN(cond, " equals ", 2)
		if len(parts) == 2 {
			left := strings.Trim(parts[0], "' \"")
			right := strings.Trim(parts[1], "' \"")
			return left == right
		}
	}
	if cond == "not_empty" || strings.HasSuffix(cond, " not_empty") {
		return strings.TrimSpace(cond) != "" && cond != "not_empty"
	}

	return true
}

// loadArtifact loads content from artifacts directory
func loadArtifact(filename string) (string, error) {
	path := filepath.Join(".octos", "artifacts", filename)
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// saveArtifact saves content to artifacts directory
func saveArtifact(filename, content string) error {
	path := filepath.Join(".octos", "artifacts", filename)
	return os.WriteFile(path, []byte(content), 0644)
}

// detectFileChanges compares directory state before/after to find changes
func detectFileChanges(beforeFiles map[string]time.Time) []string {
	afterFiles := scanDirectory(".")
	var changes []string

	// Check for new or modified files
	for path, afterTime := range afterFiles {
		if beforeTime, exists := beforeFiles[path]; !exists {
			changes = append(changes, "+ "+path)
		} else if afterTime.After(beforeTime) {
			changes = append(changes, "M "+path)
		}
	}

	// Check for deleted files
	for path := range beforeFiles {
		if _, exists := afterFiles[path]; !exists {
			changes = append(changes, "- "+path)
		}
	}

	return changes
}

// scanDirectory recursively scans directory and returns file paths with mod times
func scanDirectory(root string) map[string]time.Time {
	files := make(map[string]time.Time)
	
	filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		
		// Skip hidden dirs and common ignore patterns
		if strings.Contains(path, "/.") || 
		   strings.Contains(path, "/node_modules/") ||
		   strings.Contains(path, "/.octos/") {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		
		if !info.IsDir() {
			files[path] = info.ModTime()
		}
		return nil
	})
	
	return files
}

func stripANSI(s string) string {
	// Remove ANSI escape codes (colors, styles)
	s = ansiRegex.ReplaceAllString(s, "")
	// Remove cursor movement codes
	s = cursorMovementRegex.ReplaceAllString(s, "")
	// Remove spinner characters
	s = spinnerRegex.ReplaceAllString(s, "")
	// Remove other control characters (except \n and \t)
	s = controlCharsRegex.ReplaceAllString(s, "")
	// Remove carriage returns
	s = strings.ReplaceAll(s, "\r", "")
	return s
}

type Context struct {
	Global  map[string]any
	Outputs map[string]string
}

type ProgressCallback func(stepIndex int, output string)
type StepCallback func(stepIndex int, duration time.Duration, err error)
type StreamCallback func(stepIndex int, line string)
type FileChangesCallback func(stepIndex int, changes []string)

func RunPipeline(p *Pipeline) error {
	return RunPipelineWithResume(p, false)
}

func RunPipelineWithResume(p *Pipeline, resume bool) error {
	return RunPipelineWithCallbacks(p, nil, nil, nil, nil, nil, resume)
}

func RunPipelineWithCallbacks(p *Pipeline, onStart, onOutput ProgressCallback, onComplete StepCallback, onStream StreamCallback, onFileChanges FileChangesCallback, resume bool) error {
	ctx := &Context{
		Global:  p.Context,
		Outputs: make(map[string]string),
	}

	startStep := 0
	startTime := time.Now()
	silent := onStart != nil || onComplete != nil // Silent mode if callbacks are set
	artifacts := make(map[string]string)

	// Load state if resuming
	if resume && StateExists(p.File) {
		state, err := LoadState(p.File)
		if err == nil {
			startStep = state.LastCompletedStep + 1
			ctx.Outputs = state.Outputs
			if !silent {
				fmt.Printf("→ Resuming from step %d (%s)\n", startStep+1, p.Steps[startStep].Name)
			}
		}
	}

	for i := startStep; i < len(p.Steps); i++ {
		step := p.Steps[i]

		// Check condition
		if !evaluateCondition(step.When, ctx.Outputs, artifacts) {
			if !silent {
				fmt.Printf("⊘ Skipping step: %s (condition not met)\n", step.Name)
			}
			continue
		}

		// Load artifact if specified
		if step.LoadFrom != "" {
			content, err := loadArtifact(step.LoadFrom)
			if err != nil {
				if !silent {
					fmt.Printf("⚠ Warning: could not load artifact %s: %v\n", step.LoadFrom, err)
				}
			} else {
				artifactName := strings.TrimSuffix(step.LoadFrom, filepath.Ext(step.LoadFrom))
				artifacts[artifactName] = content
				ctx.Outputs["artifact."+artifactName] = content
			}
		}

		// Build prompt before callback
		prompt := interpolate(step.Prompt, ctx)
		fullPrompt := buildPrompt(ctx, prompt)

		if onStart != nil {
			onStart(i, prompt)
		}

		start := time.Now()
		if !silent {
			fmt.Printf("→ Running step: %s\n", step.Name)
		}

		// Snapshot files before execution
		beforeFiles := scanDirectory(".")

		// Use step-specific agent or fallback to pipeline agent
		agent := p.Agent
		if step.Agent != nil {
			agent = *step.Agent
		}

		var output string
		var err error

		if onStream != nil {
			output, err = runAgentWithStreaming(agent, fullPrompt, func(line string) {
				onStream(i, line)
			})
		} else {
			output, err = runAgent(agent, fullPrompt)
		}

		duration := time.Since(start)

		if err != nil {
			if onComplete != nil {
				onComplete(i, duration, err)
			}
			return fmt.Errorf("step %s failed: %w", step.Name, err)
		}

		ctx.Outputs[step.Name] = output

		// Detect file changes
		changes := detectFileChanges(beforeFiles)
		if onFileChanges != nil && len(changes) > 0 {
			onFileChanges(i, changes)
		}

		// Save artifact if specified
		if step.SaveTo != "" {
			if err := saveArtifact(step.SaveTo, output); err != nil {
				if !silent {
					fmt.Printf("⚠ Warning: could not save artifact %s: %v\n", step.SaveTo, err)
				}
			} else if !silent {
				fmt.Printf("💾 Saved artifact: %s\n", step.SaveTo)
			}
		}

		if onOutput != nil {
			onOutput(i, output)
		}

		if onComplete != nil {
			onComplete(i, duration, nil)
		}

		// Save state after each successful step
		state := &PipelineState{
			PipelineFile:      p.File,
			LastCompletedStep: i,
			Outputs:           ctx.Outputs,
			StartTime:         startTime.Format(time.RFC3339),
		}
		SaveState(state)

		if !silent {
			fmt.Printf("✓ Step %s completed\n\n", step.Name)
		}
	}

	// Clear state on completion
	ClearState(p.File)
	return nil
}

func buildPrompt(ctx *Context, newTask string) string {
	var buf bytes.Buffer

	buf.WriteString("=== CONTEXTO GLOBAL ===\n")
	for k, v := range ctx.Global {
		buf.WriteString(fmt.Sprintf("%s: %v\n", k, v))
	}

	if len(ctx.Outputs) > 0 {
		buf.WriteString("\n=== OUTPUT PASOS ANTERIORES ===\n")
		for name, output := range ctx.Outputs {
			buf.WriteString(fmt.Sprintf("[%s]:\n%s\n\n", name, output))
		}
	}

	buf.WriteString("=== NUEVA TAREA ===\n")
	buf.WriteString(newTask)

	return buf.String()
}

func interpolate(text string, ctx *Context) string {
	result := text

	for name, output := range ctx.Outputs {
		placeholder := fmt.Sprintf("{{%s.output}}", name)
		result = strings.ReplaceAll(result, placeholder, output)
	}

	if rules, ok := ctx.Global["rules"].([]any); ok {
		var rulesList []string
		for _, r := range rules {
			rulesList = append(rulesList, fmt.Sprintf("- %v", r))
		}
		result = strings.ReplaceAll(result, "{{context.rules}}", strings.Join(rulesList, "\n"))
	}

	return result
}

func runAgent(agent AgentConfig, prompt string) (string, error) {
	args := append(agent.Args, prompt)
	cmd := exec.Command(agent.Cmd, args...)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%w: %s", err, string(output))
	}

	return stripANSI(string(output)), nil
}

func runAgentWithStreaming(agent AgentConfig, prompt string, onLine func(string)) (string, error) {
	args := append(agent.Args, prompt)
	cmd := exec.Command(agent.Cmd, args...)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", err
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return "", err
	}

	if err := cmd.Start(); err != nil {
		return "", err
	}

	var output strings.Builder
	scanner := bufio.NewScanner(stdout)

	go func() {
		for scanner.Scan() {
			line := scanner.Text()
			cleanLine := stripANSI(line)
			output.WriteString(cleanLine + "\n")
			if onLine != nil {
				onLine(cleanLine)
			}
		}
	}()

	// Also capture stderr
	stderrScanner := bufio.NewScanner(stderr)
	go func() {
		for stderrScanner.Scan() {
			line := stderrScanner.Text()
			cleanLine := stripANSI(line)
			output.WriteString(cleanLine + "\n")
			if onLine != nil {
				onLine(cleanLine)
			}
		}
	}()

	if err := cmd.Wait(); err != nil {
		return output.String(), err
	}

	return output.String(), nil
}

