// Package main — executor.go implements the pipeline execution engine.
// It manages agent process lifecycle, step sequencing (serial and parallel),
// artifact loading/saving, retry logic, and event emission to the TUI.
package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// loadArtifact loads content from artifacts directory
func loadArtifact(filename string) (string, error) {
	path := filepath.Join(ArtifactsDir, filename)
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// saveArtifact saves content to artifacts directory
func saveArtifact(filename, content string) error {
	path := filepath.Join(ArtifactsDir, filename)
	return os.WriteFile(path, []byte(content), 0o644)
}





type PipelineContext struct {
	Global  map[string]any
	Outputs map[string]string
	mu      sync.Mutex
}



// PipelineRunner manages pipeline execution and exposes control over the active agent process.
type PipelineRunner struct {
	activeCmds  map[int]*exec.Cmd
	killedSteps map[int]bool
	mu          sync.Mutex
}

// KillStep kills a specific running step by index, causing it to be retried.
func (r *PipelineRunner) KillStep(index int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.killedSteps == nil {
		r.killedSteps = make(map[int]bool)
	}
	r.killedSteps[index] = true
	if cmd, ok := r.activeCmds[index]; ok && cmd.Process != nil {
		cmd.Process.Kill()
	}
}

// setActiveCmd stores the command for a specific step index.
func (r *PipelineRunner) setActiveCmd(index int, cmd *exec.Cmd) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.activeCmds == nil {
		r.activeCmds = make(map[int]*exec.Cmd)
	}
	r.activeCmds[index] = cmd
}

// clearActiveCmd removes the reference for a specific step.
func (r *PipelineRunner) clearActiveCmd(index int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.activeCmds, index)
}

// consumeKilled reports whether a manual kill was requested for a specific step and resets the flag.
func (r *PipelineRunner) consumeKilled(index int) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.killedSteps == nil {
		return false
	}
	k := r.killedSteps[index]
	delete(r.killedSteps, index)
	return k
}

// RunOptions configures pipeline execution behavior and event delivery.
type RunOptions struct {
	Ctx    context.Context
	Events chan<- Event
	Runner *PipelineRunner
	Resume bool
}

func RunPipeline(p *Pipeline) error {
	return RunPipelineWithOptions(p, RunOptions{})
}

func RunPipelineWithResume(p *Pipeline, resume bool) error {
	return RunPipelineWithOptions(p, RunOptions{Resume: resume})
}

func RunPipelineWithOptions(p *Pipeline, opts RunOptions) error {
	runCtx := opts.Ctx
	if runCtx == nil {
		runCtx = context.Background()
	}
	events := opts.Events
	runner := opts.Runner
	resume := opts.Resume

	// Ensure artifacts directory exists before execution begins
	if err := os.MkdirAll(ArtifactsDir, 0o755); err != nil {
		return fmt.Errorf("failed to create artifacts directory: %w", err)
	}

	ctx := &PipelineContext{
		Global:  p.Context,
		Outputs: make(map[string]string),
	}

	startStep := 0
	startTime := time.Now()
	silent := events != nil // Silent mode if events channel is set
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

	// Use parallel execution when dependencies are declared
	if p.hasNeeds() && startStep == 0 {
		err := runPipelineByLevels(runCtx, p, opts, ctx, artifacts, silent, startTime)
		emit(events, Event{Kind: EventDone})
		if err == nil {
			if clearErr := ClearState(p.File); clearErr != nil {
				fmt.Fprintf(os.Stderr, "warning: failed to clear state: %v\n", clearErr)
			}
		}
		return err
	}

	for i := startStep; i < len(p.Steps); i++ {
		step := p.Steps[i]

		// Check condition
		if step.When != "" && !isValidConditionSyntax(step.When) {
			msg := fmt.Sprintf("warning: step '%s' has unparseable 'when' condition: '%s'\n  hint: supported: 'X contains Y', 'X equals Y', 'X not_empty'", step.Name, step.When)
			if events != nil {
				emit(events, Event{Kind: EventStepStream, StepIndex: i, Line: msg})
			} else if !silent {
				fmt.Fprintln(os.Stderr, msg)
			}
		}
		if !evaluateCondition(step.When, ctx.Outputs, artifacts) {
			if !silent {
				fmt.Printf("⊘ Skipping step: %s (condition not met)\n", step.Name)
			}
			emit(events, Event{Kind: EventStepSkip, StepIndex: i})
			continue
		}

		// Load artifacts if specified
		for _, loadFile := range step.LoadFrom {
			content, err := loadArtifact(loadFile)
			if err != nil {
				msg := fmt.Sprintf("warning: step '%s' cannot load artifact '%s' (file not found in %s/)\n  hint: was the producing step skipped or is this the first run?", step.Name, loadFile, ArtifactsDir)
				if events != nil {
					emit(events, Event{Kind: EventStepStream, StepIndex: i, Line: msg})
				} else if !silent {
					fmt.Fprintln(os.Stderr, msg)
				}
			} else {
				artifactName := strings.TrimSuffix(loadFile, filepath.Ext(loadFile))
				artifacts[artifactName] = content
				ctx.Outputs[ArtifactKeyPrefix+artifactName] = content
			}
		}

		// Build prompt before callback
		prompt, warnings := interpolate(step.Prompt, ctx)
		fullPrompt := buildPrompt(ctx, prompt)

		// Route warnings: to event stream if available, otherwise stderr
		for _, w := range warnings {
			if events != nil {
				emit(events, Event{Kind: EventStepStream, StepIndex: i, Line: w})
			} else {
				fmt.Fprintln(os.Stderr, w)
			}
		}

		emit(events, Event{Kind: EventStepStart, StepIndex: i, Prompt: prompt})

		start := time.Now()
		if !silent {
			fmt.Printf("→ Running step: %s\n", step.Name)
		}

		// Snapshot files before execution (only when TUI is active)
		var beforeFiles map[string]time.Time
		if events != nil {
			beforeFiles = scanDirectory(".")
		}

		// Use step-specific agent or fallback to pipeline agent
		agent := p.Agent
		if step.Agent != nil {
			agent = *step.Agent
		}

		var output string
		var err error

		// Determine failure policy
		onFailure := step.OnFailure
		if onFailure == "" {
			onFailure = FailurePolicyFailFast
		}
		maxRetries := step.MaxRetries
		if maxRetries <= 0 && onFailure == FailurePolicyRetry {
			maxRetries = 1
		}

		// Execute with retry logic
		attempts := 0
		retryPrompt := fullPrompt
		for {
			attempts++
			if events != nil {
				output, err = runAgentWithStreaming(runCtx, agent, retryPrompt, func(line string) {
					emit(events, Event{Kind: EventStepStream, StepIndex: i, Line: line})
				}, runner, i)
			} else {
				output, err = runAgent(runCtx, agent, retryPrompt, runner, i)
			}

			if err == nil {
				break
			}

			// If killed via R key (manual retry), restart immediately
			if runner != nil && runner.consumeKilled(i) {
				if events != nil {
					emit(events, Event{Kind: EventStepStream, StepIndex: i, Line: "⟳ Manual retry requested — restarting step..."})
				}
				retryPrompt = fullPrompt
				attempts = 0
				continue
			}

			// Agent failed
			if onFailure == FailurePolicyRetry && attempts <= maxRetries {
				if !silent {
					fmt.Printf("⚠ Step %s failed (attempt %d/%d), retrying...\n", step.Name, attempts, maxRetries+1)
				}
				emit(events, Event{Kind: EventStepRetry, StepIndex: i, Attempt: attempts, MaxRetries: maxRetries + 1})
				if events != nil {
					emit(events, Event{Kind: EventStepStream, StepIndex: i, Line: fmt.Sprintf("⚠ Attempt %d/%d failed: %v — retrying...", attempts, maxRetries+1, err)})
				}
				// Inject error into retry prompt
				retryPrompt = fmt.Sprintf("[RETRY %d/%d] Previous attempt failed with: %v\n\n%s", attempts+1, maxRetries+1, err, fullPrompt)
				continue
			}

			break
		}

		duration := time.Since(start)

		if err != nil {
			switch onFailure {
			case FailurePolicySkip:
				if !silent {
					fmt.Printf("⊘ Step %s failed, skipping (on_failure: skip): %v\n", step.Name, err)
				}
				if events != nil {
					emit(events, Event{Kind: EventStepStream, StepIndex: i, Line: fmt.Sprintf("⊘ Step skipped due to failure: %v", err)})
				}
				emit(events, Event{Kind: EventStepComplete, StepIndex: i, Duration: duration})
				// Save state and continue to next step
				state := &PipelineState{
					PipelineFile:      p.File,
					LastCompletedStep: i,
					Outputs:           ctx.Outputs,
					StartTime:         startTime.Format(time.RFC3339),
				}
				if saveErr := SaveState(state); saveErr != nil {
					msg := fmt.Sprintf("warning: failed to save state: %v", saveErr)
					if events != nil {
						emit(events, Event{Kind: EventStepStream, StepIndex: i, Line: msg})
					} else {
						fmt.Fprintln(os.Stderr, msg)
					}
				}
				continue
			default: // fail_fast or retry exhausted
				emit(events, Event{Kind: EventStepComplete, StepIndex: i, Duration: duration, Err: err})
				emit(events, Event{Kind: EventDone})
				return fmt.Errorf("error: step '%s' failed (duration %s)\n  agent: %s %v\n  cause: %w", step.Name, duration.Round(time.Millisecond), agent.Cmd, agent.Args, err)
			}
		}

		ctx.Outputs[step.Name] = output

		// Detect file changes (only when TUI is active)
		if events != nil {
			changes := detectFileChanges(beforeFiles)
			if len(changes) > 0 {
				emit(events, Event{Kind: EventFileChanges, StepIndex: i, Changes: changes})
			}
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

		emit(events, Event{Kind: EventStepOutput, StepIndex: i, Output: output})
		emit(events, Event{Kind: EventStepComplete, StepIndex: i, Duration: duration})

		// Save state after each successful step
		state := &PipelineState{
			PipelineFile:      p.File,
			LastCompletedStep: i,
			Outputs:           ctx.Outputs,
			StartTime:         startTime.Format(time.RFC3339),
		}
		if saveErr := SaveState(state); saveErr != nil {
			msg := fmt.Sprintf("warning: failed to save state: %v", saveErr)
			if events != nil {
				emit(events, Event{Kind: EventStepStream, StepIndex: i, Line: msg})
			} else {
				fmt.Fprintln(os.Stderr, msg)
			}
		}

		if !silent {
			fmt.Printf("✓ Step %s completed\n\n", step.Name)
		}
	}

	// Clear state on completion
	if clearErr := ClearState(p.File); clearErr != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to clear state: %v\n", clearErr)
	}
	emit(events, Event{Kind: EventDone})
	return nil
}

// runPipelineByLevels executes steps grouped by topological level.
// Steps in the same level run in parallel.
func runPipelineByLevels(runCtx context.Context, p *Pipeline, opts RunOptions, ctx *PipelineContext, artifacts map[string]string, silent bool, startTime time.Time) error {
	events := opts.Events
	runner := opts.Runner

	// Build parent map and assign levels
	parents := make(map[string][]string)
	for _, step := range p.Steps {
		for _, dep := range step.Needs {
			parents[step.Name] = append(parents[step.Name], dep)
		}
	}
	levels := assignLevels(p.Steps, parents)

	// Find max level
	maxLevel := 0
	for _, lvl := range levels {
		if lvl > maxLevel {
			maxLevel = lvl
		}
	}

	// Build step index map
	stepIndex := make(map[string]int)
	for i, step := range p.Steps {
		stepIndex[step.Name] = i
	}

	// Execute level by level
	for lvl := 0; lvl <= maxLevel; lvl++ {
		// Collect steps at this level
		var levelSteps []int
		for _, step := range p.Steps {
			if levels[step.Name] == lvl {
				levelSteps = append(levelSteps, stepIndex[step.Name])
			}
		}

		if len(levelSteps) == 1 {
			// Single step: run sequentially (simpler, same as before)
			i := levelSteps[0]
			if err := runSingleStep(runCtx, p, i, opts, ctx, artifacts, silent, startTime); err != nil {
				return err
			}
		} else {
			// Multiple steps: run in parallel
			type stepResult struct {
				index  int
				output string
				err    error
			}

			results := make(chan stepResult, len(levelSteps))
			var wg sync.WaitGroup

			for _, i := range levelSteps {
				step := p.Steps[i]

				// Check condition before launching
				if !evaluateCondition(step.When, ctx.Outputs, artifacts) {
					if !silent {
						fmt.Printf("⊘ Skipping step: %s (condition not met)\n", step.Name)
					}
					emit(events, Event{Kind: EventStepSkip, StepIndex: i})
					continue
				}

				wg.Add(1)
				go func(idx int, s Step) {
					defer wg.Done()

					// Load artifacts if specified
					for _, loadFile := range s.LoadFrom {
						content, err := loadArtifact(loadFile)
						if err != nil {
							msg := fmt.Sprintf("warning: step '%s' cannot load artifact '%s'", s.Name, loadFile)
							emit(events, Event{Kind: EventStepStream, StepIndex: idx, Line: msg})
						} else {
							artifactName := strings.TrimSuffix(loadFile, filepath.Ext(loadFile))
							// Note: artifacts map access is safe here since parallel steps at same level
							// don't produce artifacts that siblings need
							ctx.mu.Lock()
							artifacts[artifactName] = content
							ctx.Outputs[ArtifactKeyPrefix+artifactName] = content
							ctx.mu.Unlock()
						}
					}

					// Build prompt
					ctx.mu.Lock()
					prompt, warnings := interpolate(s.Prompt, ctx)
					fullPrompt := buildPrompt(ctx, prompt)
					ctx.mu.Unlock()

					for _, w := range warnings {
						emit(events, Event{Kind: EventStepStream, StepIndex: idx, Line: w})
					}

					emit(events, Event{Kind: EventStepStart, StepIndex: idx, Prompt: prompt})

					start := time.Now()

					var beforeFiles map[string]time.Time
					if events != nil {
						beforeFiles = scanDirectory(".")
					}

					agent := p.Agent
					if s.Agent != nil {
						agent = *s.Agent
					}

					var output string
					var err error
					onFailure := s.OnFailure
					if onFailure == "" {
						onFailure = FailurePolicyFailFast
					}
					maxRetries := s.MaxRetries
					if maxRetries <= 0 && onFailure == FailurePolicyRetry {
						maxRetries = 1
					}

					attempts := 0
					retryPrompt := fullPrompt
					for {
						attempts++
						if events != nil {
							output, err = runAgentWithStreaming(runCtx, agent, retryPrompt, func(line string) {
								emit(events, Event{Kind: EventStepStream, StepIndex: idx, Line: line})
							}, runner, idx)
						} else {
							output, err = runAgent(runCtx, agent, retryPrompt, runner, idx)
						}

						if err == nil {
							break
						}

						if runner != nil && runner.consumeKilled(idx) {
							emit(events, Event{Kind: EventStepStream, StepIndex: idx, Line: "⟳ Manual retry requested — restarting step..."})
							retryPrompt = fullPrompt
							attempts = 0
							continue
						}

						if onFailure == FailurePolicyRetry && attempts <= maxRetries {
							emit(events, Event{Kind: EventStepRetry, StepIndex: idx, Attempt: attempts, MaxRetries: maxRetries + 1})
							emit(events, Event{Kind: EventStepStream, StepIndex: idx, Line: fmt.Sprintf("⚠ Attempt %d/%d failed: %v — retrying...", attempts, maxRetries+1, err)})
							retryPrompt = fmt.Sprintf("[RETRY %d/%d] Previous attempt failed with: %v\n\n%s", attempts+1, maxRetries+1, err, fullPrompt)
							continue
						}
						break
					}

					duration := time.Since(start)

					if err != nil && onFailure == FailurePolicySkip {
						emit(events, Event{Kind: EventStepStream, StepIndex: idx, Line: fmt.Sprintf("⊘ Step skipped due to failure: %v", err)})
						emit(events, Event{Kind: EventStepComplete, StepIndex: idx, Duration: duration})
						results <- stepResult{idx, "", nil}
						return
					}

					if err != nil {
						emit(events, Event{Kind: EventStepComplete, StepIndex: idx, Duration: duration, Err: err})
						results <- stepResult{idx, "", err}
						return
					}

					// Detect file changes (only when TUI is active)
					if events != nil {
						changes := detectFileChanges(beforeFiles)
						if len(changes) > 0 {
							emit(events, Event{Kind: EventFileChanges, StepIndex: idx, Changes: changes})
						}
					}

					// Save artifact
					if s.SaveTo != "" {
						saveArtifact(s.SaveTo, output)
					}

					emit(events, Event{Kind: EventStepOutput, StepIndex: idx, Output: output})
					emit(events, Event{Kind: EventStepComplete, StepIndex: idx, Duration: duration})

					results <- stepResult{idx, output, nil}
				}(i, step)
			}

			// Wait for all parallel steps to finish
			go func() {
				wg.Wait()
				close(results)
			}()

			for res := range results {
				if res.err != nil {
					return fmt.Errorf("error: step '%s' failed\n  cause: %w", p.Steps[res.index].Name, res.err)
				}
				// Store output
				ctx.mu.Lock()
				ctx.Outputs[p.Steps[res.index].Name] = res.output
				ctx.mu.Unlock()
			}

			// Save state after level completion
			lastIdx := levelSteps[len(levelSteps)-1]
			state := &PipelineState{
				PipelineFile:      p.File,
				LastCompletedStep: lastIdx,
				Outputs:           ctx.Outputs,
				StartTime:         startTime.Format(time.RFC3339),
			}
			if saveErr := SaveState(state); saveErr != nil {
				msg := fmt.Sprintf("warning: failed to save state: %v", saveErr)
				if events != nil {
					emit(events, Event{Kind: EventStepStream, StepIndex: lastIdx, Line: msg})
				} else {
					fmt.Fprintln(os.Stderr, msg)
				}
			}
		}
	}

	return nil
}

// runSingleStep executes a single step (used within level-based execution).
func runSingleStep(runCtx context.Context, p *Pipeline, i int, opts RunOptions, ctx *PipelineContext, artifacts map[string]string, silent bool, startTime time.Time) error {
	step := p.Steps[i]
	events := opts.Events
	runner := opts.Runner

	if !evaluateCondition(step.When, ctx.Outputs, artifacts) {
		if !silent {
			fmt.Printf("⊘ Skipping step: %s (condition not met)\n", step.Name)
		}
		emit(events, Event{Kind: EventStepSkip, StepIndex: i})
		return nil
	}

	for _, loadFile := range step.LoadFrom {
		content, err := loadArtifact(loadFile)
		if err != nil {
			msg := fmt.Sprintf("warning: step '%s' cannot load artifact '%s'", step.Name, loadFile)
			emit(events, Event{Kind: EventStepStream, StepIndex: i, Line: msg})
		} else {
			artifactName := strings.TrimSuffix(loadFile, filepath.Ext(loadFile))
			artifacts[artifactName] = content
			ctx.Outputs[ArtifactKeyPrefix+artifactName] = content
		}
	}

	prompt, warnings := interpolate(step.Prompt, ctx)
	fullPrompt := buildPrompt(ctx, prompt)

	for _, w := range warnings {
		emit(events, Event{Kind: EventStepStream, StepIndex: i, Line: w})
	}

	emit(events, Event{Kind: EventStepStart, StepIndex: i, Prompt: prompt})

	start := time.Now()
	if !silent {
		fmt.Printf("→ Running step: %s\n", step.Name)
	}

	var beforeFiles map[string]time.Time
	if events != nil {
		beforeFiles = scanDirectory(".")
	}

	agent := p.Agent
	if step.Agent != nil {
		agent = *step.Agent
	}

	var output string
	var err error
	onFailure := step.OnFailure
	if onFailure == "" {
		onFailure = FailurePolicyFailFast
	}
	maxRetries := step.MaxRetries
	if maxRetries <= 0 && onFailure == FailurePolicyRetry {
		maxRetries = 1
	}

	attempts := 0
	retryPrompt := fullPrompt
	for {
		attempts++
		if events != nil {
			output, err = runAgentWithStreaming(runCtx, agent, retryPrompt, func(line string) {
				emit(events, Event{Kind: EventStepStream, StepIndex: i, Line: line})
			}, runner, i)
		} else {
			output, err = runAgent(runCtx, agent, retryPrompt, runner, i)
		}

		if err == nil {
			break
		}

		if runner != nil && runner.consumeKilled(i) {
			emit(events, Event{Kind: EventStepStream, StepIndex: i, Line: "⟳ Manual retry requested — restarting step..."})
			retryPrompt = fullPrompt
			attempts = 0
			continue
		}

		if onFailure == FailurePolicyRetry && attempts <= maxRetries {
			emit(events, Event{Kind: EventStepRetry, StepIndex: i, Attempt: attempts, MaxRetries: maxRetries + 1})
			emit(events, Event{Kind: EventStepStream, StepIndex: i, Line: fmt.Sprintf("⚠ Attempt %d/%d failed: %v — retrying...", attempts, maxRetries+1, err)})
			retryPrompt = fmt.Sprintf("[RETRY %d/%d] Previous attempt failed with: %v\n\n%s", attempts+1, maxRetries+1, err, fullPrompt)
			continue
		}
		break
	}

	duration := time.Since(start)

	if err != nil {
		switch onFailure {
		case FailurePolicySkip:
			emit(events, Event{Kind: EventStepStream, StepIndex: i, Line: fmt.Sprintf("⊘ Step skipped due to failure: %v", err)})
			emit(events, Event{Kind: EventStepComplete, StepIndex: i, Duration: duration})
		default:
			emit(events, Event{Kind: EventStepComplete, StepIndex: i, Duration: duration, Err: err})
			return fmt.Errorf("error: step '%s' failed\n  cause: %w", step.Name, err)
		}
		return nil
	}

	ctx.Outputs[step.Name] = output

	if events != nil {
		changes := detectFileChanges(beforeFiles)
		if len(changes) > 0 {
			emit(events, Event{Kind: EventFileChanges, StepIndex: i, Changes: changes})
		}
	}

	if step.SaveTo != "" {
		saveArtifact(step.SaveTo, output)
	}

	emit(events, Event{Kind: EventStepOutput, StepIndex: i, Output: output})
	emit(events, Event{Kind: EventStepComplete, StepIndex: i, Duration: duration})

	state := &PipelineState{
		PipelineFile:      p.File,
		LastCompletedStep: i,
		Outputs:           ctx.Outputs,
		StartTime:         startTime.Format(time.RFC3339),
	}
	if saveErr := SaveState(state); saveErr != nil {
		msg := fmt.Sprintf("warning: failed to save state: %v", saveErr)
		if events != nil {
			emit(events, Event{Kind: EventStepStream, StepIndex: i, Line: msg})
		} else {
			fmt.Fprintln(os.Stderr, msg)
		}
	}

	if !silent {
		fmt.Printf("✓ Step %s completed\n\n", step.Name)
	}
	return nil
}


func runAgent(ctx context.Context, agent AgentConfig, prompt string, runner *PipelineRunner, stepIdx int) (string, error) {
	var cmd *exec.Cmd
	if agent.Stdin {
		cmd = exec.CommandContext(ctx, agent.Cmd, agent.Args...)
		cmd.Stdin = strings.NewReader(prompt)
	} else {
		cmd = exec.CommandContext(ctx, agent.Cmd, buildArgs(agent, prompt)...)
	}

	if runner != nil {
		runner.setActiveCmd(stepIdx, cmd)
	}

	output, err := cmd.CombinedOutput()
	if runner != nil {
		runner.clearActiveCmd(stepIdx)
	}
	if err != nil {
		return "", fmt.Errorf("%w: %s", err, string(output))
	}

	return stripANSI(string(output)), nil
}

func runAgentWithStreaming(ctx context.Context, agent AgentConfig, prompt string, onLine func(string), runner *PipelineRunner, stepIdx int) (string, error) {
	var cmd *exec.Cmd
	if agent.Stdin {
		cmd = exec.CommandContext(ctx, agent.Cmd, agent.Args...)
		cmd.Stdin = strings.NewReader(prompt)
	} else {
		cmd = exec.CommandContext(ctx, agent.Cmd, buildArgs(agent, prompt)...)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", err
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return "", err
	}

	// Detach from controlling terminal when running in TUI mode to prevent
	// child processes from suspending octos via SIGTTIN/SIGTTOU.
	if runner != nil {
		detachFromTerminal(cmd)
	}

	if err := cmd.Start(); err != nil {
		return "", err
	}

	if runner != nil {
		runner.setActiveCmd(stepIdx, cmd)
	}

	var (
		mu     sync.Mutex
		output strings.Builder
	)
	writeLine := func(line string) {
		mu.Lock()
		output.WriteString(line + "\n")
		mu.Unlock()
		if onLine != nil {
			onLine(line)
		}
	}

	var wg sync.WaitGroup
	wg.Add(2)
	scan := func(r io.Reader) {
		defer wg.Done()
		s := bufio.NewScanner(r)
		for s.Scan() {
			writeLine(stripANSI(s.Text()))
		}
	}
	go scan(stdout)
	go scan(stderr)

	cmdErr := cmd.Wait()
	if runner != nil {
		runner.clearActiveCmd(stepIdx)
	}
	wg.Wait()

	mu.Lock()
	result := output.String()
	mu.Unlock()

	return result, cmdErr
}
