package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"charm.land/bubbles/v2/progress"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// UI Layout Constants
const (
	defaultProgressWidth = 40
	tickInterval         = 100 * time.Millisecond
	minRightPanelWidth   = 30
	titleBorderPadding   = 4
	widthMargin          = 4
	
	// Popup dimensions
	popupWidthRatio     = 3.0 / 4.0  // 75% of screen width
	popupHeightRatio    = 2.0 / 3.0  // 66% of screen height
	popupTextPadding    = 10         // Padding for text wrapping
	popupViewportOffset = 8          // Offset for viewport height
	
	// Responsive breakpoints
	wideTerminalWidth      = 140
	extraWideTerminalWidth = 120
	mediumTerminalWidth    = 100
	narrowTerminalWidth    = 80
	compactTitleWidth      = 100
	minimalTitleWidth      = 80
	
	// Progress bar sizing
	progressWidthDivisor = 3
	minProgressWidth     = 20
	maxProgressWidth     = 60
)

type StepStatus int

const (
	StatusPending StepStatus = iota
	StatusRunning
	StatusCompleted
	StatusFailed
	StatusSkipped
)

type StepState struct {
	Name         string
	Status       StepStatus
	Duration     time.Duration
	StartTime    time.Time
	Output       string
	Error        error
	Prompt       string
	RetryAttempt int
	MaxRetries   int
}

type FocusedPanel int

const (
	FocusOutput FocusedPanel = iota
	FocusDiff
)

type TUIModel struct {
	pipeline       *Pipeline
	steps          []StepState
	currentStep    int
	selectedStep   int
	stepsView      viewport.Model
	outputView     viewport.Model
	diffView       viewport.Model
	promptView     viewport.Model
	progress       progress.Model
	width          int
	height         int
	startTime      time.Time
	endTime        time.Time
	filesChanged   []string
	quitting       bool
	resuming       bool
	program        *tea.Program
	pipelineEnded  bool
	statusMsg      string
	workingDir     string
	gitBranch      string
	userScrolling  bool
	showPrompt     bool
	focusedPanel   FocusedPanel
	maxLoops       int
	currentLoop    int
	runner         *PipelineRunner
}

type stepStartMsg struct {
	index  int
	prompt string
}
type stepOutputMsg struct {
	index  int
	output string
}
type stepStreamMsg struct {
	index int
	line  string
}
type stepRetryMsg struct {
	index      int
	attempt    int
	maxRetries int
}
type stepCompleteMsg struct {
	index    int
	duration time.Duration
	err      error
}
type stepSkipMsg struct {
	index int
}
type pipelineDoneMsg struct{}
type fileChangesMsg struct {
	index   int
	changes []string
}
type tickMsg time.Time
type startPipelineMsg struct{}

func NewTUIModel(p *Pipeline, resume bool) TUIModel {
	steps := make([]StepState, len(p.Steps))
	for i, step := range p.Steps {
		steps[i] = StepState{
			Name:   step.Name,
			Status: StatusPending,
		}
	}

	// Load state if resuming
	if resume && StateExists(p.File) {
		if state, err := LoadState(p.File); err == nil {
			for i := 0; i <= state.LastCompletedStep && i < len(steps); i++ {
				steps[i].Status = StatusCompleted
				steps[i].Duration = time.Second // Placeholder
			}
		}
	}

	prog := progress.New(
		progress.WithDefaultBlend(),
		progress.WithWidth(defaultProgressWidth),
	)

	// Get working directory
	wd, _ := os.Getwd()
	workingDir := filepath.Base(wd)

	m := TUIModel{
		pipeline:    p,
		steps:       steps,
		progress:    prog,
		startTime:   time.Now(),
		resuming:    resume,
		workingDir:  workingDir,
		maxLoops:    0,
		currentLoop: 1,
		runner:      &PipelineRunner{},
	}
	m.refreshGitBranch()

	return m
}

func (m *TUIModel) Init() tea.Cmd {
	return tickCmd()
}

func tickCmd() tea.Cmd {
	return tea.Tick(tickInterval, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func (m *TUIModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		// Update progress bar width responsively
		progressWidth := m.width / progressWidthDivisor
		if progressWidth < minProgressWidth {
			progressWidth = minProgressWidth
		}
		if progressWidth > maxProgressWidth {
			progressWidth = maxProgressWidth
		}
		m.progress.SetWidth(progressWidth)
		
		// Only trigger pipeline start on first window size event
		if m.currentStep == 0 && !m.pipelineEnded {
			return m, func() tea.Msg { return startPipelineMsg{} }
		}
		return m, nil

	case startPipelineMsg:
		// Start pipeline after window is ready
		if m.program != nil {
			m.statusMsg = "Starting pipeline..."
			go runPipelineWithProgram(m.pipeline, m.resuming, m.program, m.runner)
		}
		return m, nil

	case tea.MouseWheelMsg:
		m.userScrolling = true
		if msg.Button == tea.MouseWheelUp {
			if m.focusedPanel == FocusOutput {
				m.outputView.ScrollUp(ScrollLines)
			} else {
				m.diffView.ScrollUp(ScrollLines)
			}
		} else if msg.Button == tea.MouseWheelDown {
			if m.focusedPanel == FocusOutput {
				m.outputView.ScrollDown(ScrollLines)
			} else {
				m.diffView.ScrollDown(ScrollLines)
			}
		}
		return m, nil

	case tea.KeyPressMsg:
		return m.handleKeyPress(msg)

	case tickMsg:
		if !m.quitting {
			return m, tickCmd()
		}
		return m, nil

	case stepStartMsg:
		if m.isValidStepIndex(msg.index) {
			m.steps[msg.index].Status = StatusRunning
			m.steps[msg.index].StartTime = time.Now()
			m.steps[msg.index].Prompt = msg.prompt
			m.currentStep = msg.index
			m.statusMsg = fmt.Sprintf("Running step %d/%d: %s", msg.index+1, len(m.steps), m.steps[msg.index].Name)
			m.scrollToStep(msg.index)
			m.userScrolling = false
		}
		return m, nil

	case stepOutputMsg:
		if m.isValidStepIndex(msg.index) {
			m.steps[msg.index].Output = msg.output
		}
		return m, nil

	case stepStreamMsg:
		if m.isValidStepIndex(msg.index) {
			m.steps[msg.index].Output += msg.line + "\n"
			// Cap output to prevent memory leak in long-running pipelines
			const maxOutputBytes = 512 * 1024
			if len(m.steps[msg.index].Output) > maxOutputBytes {
				m.steps[msg.index].Output = m.steps[msg.index].Output[len(m.steps[msg.index].Output)-maxOutputBytes:]
			}
		}
		return m, nil

	case stepRetryMsg:
		if m.isValidStepIndex(msg.index) {
			m.steps[msg.index].RetryAttempt = msg.attempt
			m.steps[msg.index].MaxRetries = msg.maxRetries
			m.statusMsg = fmt.Sprintf("Step %s: retry %d/%d", m.steps[msg.index].Name, msg.attempt, msg.maxRetries)
		}
		return m, nil

	case fileChangesMsg:
		if m.isValidStepIndex(msg.index) {
			m.filesChanged = append(m.filesChanged, msg.changes...)
		}
		return m, nil

	case stepCompleteMsg:
		if m.isValidStepIndex(msg.index) {
			m.steps[msg.index].Duration = msg.duration
			if msg.err != nil {
				m.steps[msg.index].Status = StatusFailed
				m.steps[msg.index].Error = msg.err
				m.statusMsg = fmt.Sprintf("Step %d failed: %v", msg.index+1, msg.err)
			} else {
				m.steps[msg.index].Status = StatusCompleted
				m.statusMsg = fmt.Sprintf("Step %d/%d completed in %.1fs", msg.index+1, len(m.steps), msg.duration.Seconds())
			}
			m.refreshGitBranch()
		}
		return m, nil

	case stepSkipMsg:
		if m.isValidStepIndex(msg.index) {
			m.steps[msg.index].Status = StatusSkipped
		}
		return m, nil

	case pipelineDoneMsg:
		if !m.pipelineEnded {
			m.pipelineEnded = true
			m.endTime = time.Now()
			// Find last non-skipped step to select
			m.selectedStep = 0
			for i := len(m.steps) - 1; i >= 0; i-- {
				if m.steps[i].Status == StatusCompleted || m.steps[i].Status == StatusFailed {
					m.selectedStep = i
					break
				}
			}
			// Auto-restart if loop mode is active and loops remaining
			if m.maxLoops > 0 && m.currentLoop < m.maxLoops {
				return m.restartPipeline()
			}
			m.statusMsg = "Pipeline completed! Use ↑↓/jk to navigate steps"
		}
		return m, nil
	}

	var cmd tea.Cmd
	m.outputView, cmd = m.outputView.Update(msg)
	return m, cmd
}

func (m *TUIModel) handleKeyPress(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		m.quitting = true
		return m, tea.Quit
	
	case "tab":
		m.toggleFocusedPanel()
		return m, nil
	
	case "esc":
		if m.showPrompt {
			m.showPrompt = false
		}
		return m, nil
	
	case "enter":
		return m.handleEnterKey()
	
	case "r":
		return m.handleRestartKey()
	
	case "R":
		return m.handleRetryStepKey()
	
	case "j", "down":
		return m.handleDownKey()
	
	case "k", "up":
		return m.handleUpKey()
	
	case "ctrl+j":
		m.scrollPanelLines(1)
		return m, nil
	
	case "ctrl+k":
		m.scrollPanelLines(-1)
		return m, nil
	
	case "ctrl+d":
		m.scrollPanelHalfPage(true)
		return m, nil
	
	case "ctrl+u":
		m.scrollPanelHalfPage(false)
		return m, nil
	}
	
	return m, nil
}

func (m *TUIModel) countCompletedSteps() int {
	completed := 0
	for _, step := range m.steps {
		if step.Status == StatusCompleted || step.Status == StatusSkipped {
			completed++
		}
	}
	return completed
}

func (m *TUIModel) calculateStepsPerMinute(completed int, elapsed time.Duration) float64 {
	if elapsed.Minutes() > 0 && completed > 0 {
		return float64(completed) / elapsed.Minutes()
	}
	return 0.0
}

func (m *TUIModel) formatLoopInfo() string {
	if m.maxLoops > 0 {
		return fmt.Sprintf(" [Loop %d/%d]", m.currentLoop, m.maxLoops)
	}
	return fmt.Sprintf(" [Loop %d]", m.currentLoop)
}

func (m *TUIModel) buildStepsView(showTitle bool, showDuration bool) string {
	var stepsView strings.Builder
	
	if showTitle {
		stepsView.WriteString(magentaBoldStyle.Render("PIPELINE STEPS"))
		stepsView.WriteString("\n\n")
	}
	
	for i, step := range m.steps {
		icon := GetStepIcon(step.Status)
		stepStyle := GetStepStatusStyle(step.Status)
		
		line := fmt.Sprintf("%s %s", icon, step.Name)
		if step.Status == StatusRunning && step.RetryAttempt > 0 {
			retryInfo := yellowStyle.Render(fmt.Sprintf("(retry %d/%d)", step.RetryAttempt, step.MaxRetries))
			line = fmt.Sprintf("%s %s %s", icon, step.Name, retryInfo)
		}
		if showDuration && step.Status == StatusCompleted {
			duration := fmt.Sprintf("%.1fs", step.Duration.Seconds())
			line = stepStyle.Render(line) + " " + statsStyle.Render(duration)
		} else {
			line = stepStyle.Render(line)
		}
		
		stepsView.WriteString(line)
		
		if !m.pipelineEnded && i == m.currentStep && step.Status == StatusRunning {
			stepsView.WriteString(" ◀")
		} else if m.pipelineEnded && i == m.selectedStep {
			stepsView.WriteString(" ◀")
		}
		stepsView.WriteString("\n")
	}
	
	return stepsView.String()
}

func (m *TUIModel) isValidStepIndex(index int) bool {
	return index >= 0 && index < len(m.steps)
}

// refreshGitBranch updates the displayed git branch from the working directory.
func (m *TUIModel) refreshGitBranch() {
	if out, err := exec.Command("git", "branch", "--show-current").Output(); err == nil {
		m.gitBranch = strings.TrimSpace(string(out))
	}
}

func (m *TUIModel) getDisplayStep() int {
	if m.pipelineEnded {
		return m.selectedStep
	}
	return m.currentStep
}

func (m *TUIModel) toggleFocusedPanel() {
	if m.focusedPanel == FocusOutput {
		m.focusedPanel = FocusDiff
	} else {
		m.focusedPanel = FocusOutput
	}
}

func (m *TUIModel) handleEnterKey() (tea.Model, tea.Cmd) {
	if m.pipelineEnded {
		if !m.showPrompt {
			m.showPrompt = true
			m.initPromptView()
		} else {
			m.showPrompt = false
		}
	}
	return m, nil
}

func (m *TUIModel) handleRestartKey() (tea.Model, tea.Cmd) {
	if m.pipelineEnded {
		if m.maxLoops > 0 && m.currentLoop >= m.maxLoops {
			m.statusMsg = fmt.Sprintf("Max loops reached (%d/%d)", m.currentLoop, m.maxLoops)
			return m, nil
		}
		return m.restartPipeline()
	}
	return m, nil
}

func (m *TUIModel) handleRetryStepKey() (tea.Model, tea.Cmd) {
	if m.pipelineEnded || m.runner == nil {
		return m, nil
	}
	if m.currentStep < len(m.steps) && m.steps[m.currentStep].Status == StatusRunning {
		m.statusMsg = fmt.Sprintf("Retrying step: %s", m.steps[m.currentStep].Name)
		m.runner.KillCurrentStep()
	}
	return m, nil
}

func (m *TUIModel) handleDownKey() (tea.Model, tea.Cmd) {
	if m.showPrompt {
		m.promptView.ScrollDown(1)
		return m, nil
	}
	if m.pipelineEnded && m.selectedStep < len(m.steps)-1 {
		m.selectedStep++
		m.scrollToStep(m.selectedStep)
	}
	return m, nil
}

func (m *TUIModel) handleUpKey() (tea.Model, tea.Cmd) {
	if m.showPrompt {
		m.promptView.ScrollUp(1)
		return m, nil
	}
	if m.pipelineEnded && m.selectedStep > 0 {
		m.selectedStep--
		m.scrollToStep(m.selectedStep)
	}
	return m, nil
}

func (m *TUIModel) scrollPanelLines(lines int) {
	m.userScrolling = true
	
	if m.focusedPanel == FocusOutput {
		if lines > 0 {
			m.outputView.ScrollDown(lines)
		} else {
			m.outputView.ScrollUp(-lines)
		}
	} else {
		if lines > 0 {
			m.diffView.ScrollDown(lines)
		} else {
			m.diffView.ScrollUp(-lines)
		}
	}
}

func (m *TUIModel) scrollPanelHalfPage(down bool) {
	m.userScrolling = true
	
	if m.focusedPanel == FocusOutput {
		if down {
			m.outputView.HalfPageDown()
		} else {
			m.outputView.HalfPageUp()
		}
	} else {
		if down {
			m.diffView.HalfPageDown()
		} else {
			m.diffView.HalfPageUp()
		}
	}
}

func (m *TUIModel) View() tea.View {
	if m.width == 0 || m.height == 0 {
		v := tea.NewView("Initializing...")
		v.AltScreen = true
		v.MouseMode = tea.MouseModeCellMotion
		return v
	}

	var content string

	// Use stacked layout for narrow terminals
	if m.isNarrowMode() {
		content = m.renderNarrowView()
	} else {
		// Render header and footer first to measure their actual heights
		header := m.renderHeader()
		footer := m.renderFooter()
		
		headerHeight := strings.Count(header, "\n") + 1
		footerHeight := strings.Count(footer, "\n") + 1
		
		// Content gets ALL remaining vertical space - no arbitrary padding
		contentHeight := m.height - headerHeight - footerHeight
		if contentHeight < 5 {
			contentHeight = 5
		}
		
		// Render content panels with exact height budget
		contentPanels := m.renderContent(m.width, contentHeight)
		
		// Stack: header + content + footer = exactly m.height lines
		result := lipgloss.JoinVertical(lipgloss.Left, header, contentPanels, footer)
		
		// Safety truncation - should not normally trigger with correct math
		lines := strings.Split(result, "\n")
		if len(lines) > m.height {
			lines = lines[:m.height]
			result = strings.Join(lines, "\n")
		}
		
		// Render popup if showing prompt
		if m.showPrompt && m.pipelineEnded && m.selectedStep < len(m.steps) && m.steps[m.selectedStep].Prompt != "" {
			result = m.renderPromptPopup(result)
		}
		
		content = result
	}

	v := tea.NewView(content)
	v.AltScreen = true
	v.MouseMode = tea.MouseModeCellMotion
	return v
}

func (m *TUIModel) renderHeader() string {
	titleText := "OCTOS PIPELINE ORCHESTRATOR"
	if m.width < compactTitleWidth {
		titleText = "OCTOS PIPELINE"
	}
	if m.width < minimalTitleWidth {
		titleText = "OCTOS"
	}
	
	titleBorder := strings.Repeat("═", len(titleText)+titleBorderPadding)
	titleBox := titleStyle.Render(fmt.Sprintf("╔%s╗\n║  %s  ║\n╚%s╝", titleBorder, titleText, titleBorder))
	
	running := 0
	completed := m.countCompletedSteps()
	// Count current step as running if pipeline hasn't ended
	if !m.pipelineEnded && m.isValidStepIndex(m.currentStep) {
		running = 1
	}

	// Progress bar with cyberpunk style - cyan text (original color)
	percent := float64(completed+running) / float64(len(m.steps))
	progressBar := m.progress.ViewAs(percent)
	progressText := fmt.Sprintf("▓▒░ %d/%d steps (%.0f%%) ░▒▓", completed+running, len(m.steps), percent*100)
	
	loopInfo := m.formatLoopInfo()
	
	// Build progress line
	progressTextStyled := boldCyanStyle.Render(progressText)
	loopInfoStyled := cyanStyle.Render(loopInfo)
	
	progressLine := progressBarStyle.Render(progressBar) + " " + progressTextStyled + loopInfoStyled
	
	titleBoxWidth := lipgloss.Width(titleBox)
	progressLineWidth := lipgloss.Width(progressLine)
	
	targetWidth := m.width - widthMargin
	padding := targetWidth - titleBoxWidth - progressLineWidth
	if padding < 1 {
		padding = 1
	}
	titleLines := strings.Split(titleBox, "\n")
	if len(titleLines) >= 2 {
		titleLines[1] = titleLines[1] + strings.Repeat(" ", padding) + progressLine
		return strings.Join(titleLines, "\n")
	}
	
	return titleBox
}

func (m *TUIModel) renderFooter() string {
	// Enhanced status bar with context
	currentTime := time.Now().Format("15:04")
	
	statusParts := []string{
		cyanStyle.Render("⏰ " + currentTime),
		yellowStyle.Render("📁 " + m.workingDir),
	}
	
	if m.gitBranch != "" {
		statusParts = append(statusParts, 
			greenStyle.Render("🌿 " + m.gitBranch))
	}
	
	if m.statusMsg != "" {
		statusParts = append(statusParts, m.statusMsg)
	}
	
	statusBar := statusBarStyle.Width(m.width).Render(
		strings.Join(statusParts, " │ "),
	)

	// Stats
	elapsed := time.Since(m.startTime)
	if m.pipelineEnded && !m.endTime.IsZero() {
		elapsed = m.endTime.Sub(m.startTime)
	}
	
	running := 0
	completed := m.countCompletedSteps()
	// Count current step as running if pipeline hasn't ended
	if !m.pipelineEnded && m.isValidStepIndex(m.currentStep) {
		running = 1
	}
	
	stats := statsStyle.Render(
		fmt.Sprintf("⚡ Elapsed: %s │ Steps: %d/%d │ Speed: %.1f steps/min",
			elapsed.Round(time.Second),
			completed+running,
			len(m.steps),
			m.calculateStepsPerMinute(completed, elapsed),
		),
	)

	help := m.buildHelpText()
	
	return lipgloss.JoinVertical(lipgloss.Left, statusBar, stats, cyanFaintStyle.Render(help))
}

func (m *TUIModel) buildHelpText() string {
	if m.pipelineEnded {
		if m.width >= wideTerminalWidth {
			return "⌨  [↑↓/jk] Navigate │ [Enter] View prompt │ [r] Restart │ [Tab] Switch panel │ [Ctrl+j/k] Scroll │ [Ctrl+d/u] Page │ [Mouse wheel] Scroll │ [q] Quit"
		} else if m.width >= mediumTerminalWidth {
			return "⌨  [↑↓/jk] Navigate │ [Enter] Prompt │ [r] Restart │ [Tab] Panel │ [Ctrl+j/k] Scroll │ [q] Quit"
		} else {
			return "⌨  [↑↓/jk] Nav │ [Enter] Prompt │ [r] Restart │ [Tab] Panel │ [q] Quit"
		}
	} else {
		if m.width >= extraWideTerminalWidth {
			return "⌨  [R] Retry step │ [Tab] Switch panel │ [Ctrl+j/k] Scroll │ [Ctrl+d/u] Page │ [Mouse wheel] Scroll │ [q] Quit"
		} else if m.width >= narrowTerminalWidth {
			return "⌨  [R] Retry │ [Tab] Panel │ [Ctrl+j/k] Scroll │ [Ctrl+d/u] Page │ [q] Quit"
		} else {
			return "⌨  [R] Retry │ [Tab] Panel │ [q] Quit"
		}
	}
}

func (m *TUIModel) renderContent(width, contentHeight int) string {
	// panelStyle: Border(RoundedBorder) + Padding(0,1)
	// In lipgloss v2, .Width(W) is the TOTAL outer width including border and padding.
	// So text area = W - border(2) - padding(2) = W - 4
	const frameTotal = 4  // border(1+1) + padding(1+1)
	const borderV = 2     // border top + bottom
	const titleLines = 2  // "TITLE\n\n"

	// Width budget: stepsOuter + rightOuter = width
	stepsOuter := (width * StepsWidthPct) / 100
	if stepsOuter < MinStepsWidth + frameTotal {
		stepsOuter = MinStepsWidth + frameTotal
	}
	rightOuter := width - stepsOuter
	if rightOuter < minRightPanelWidth + frameTotal {
		rightOuter = minRightPanelWidth + frameTotal
	}

	// Text widths (what the viewport/content can actually use)
	stepsTextWidth := stepsOuter - frameTotal
	rightTextWidth := rightOuter - frameTotal

	// Height budget: outputPanel + diffPanel = contentHeight
	outputOuter := (contentHeight * OutputPanelPct) / 100
	diffOuter := contentHeight - outputOuter
	if outputOuter < MinPanelHeight {
		outputOuter = MinPanelHeight
	}
	if diffOuter < MinPanelHeight {
		diffOuter = MinPanelHeight
	}

	outputTextHeight := outputOuter - borderV - titleLines
	diffTextHeight := diffOuter - borderV - titleLines
	stepsTextHeight := contentHeight - borderV
	if outputTextHeight < 2 {
		outputTextHeight = 2
	}
	if diffTextHeight < 2 {
		diffTextHeight = 2
	}
	if stepsTextHeight < MinStepsHeight {
		stepsTextHeight = MinStepsHeight
	}

	// Configure viewports - use SoftWrap so viewport handles line breaking
	m.stepsView.SetWidth(stepsTextWidth)
	m.stepsView.SetHeight(stepsTextHeight)
	m.outputView.SetWidth(rightTextWidth)
	m.outputView.SetHeight(outputTextHeight)
	m.outputView.SoftWrap = true
	m.diffView.SetWidth(rightTextWidth)
	m.diffView.SetHeight(diffTextHeight)
	m.diffView.SoftWrap = true

	// Set content (NO pre-wrapping - viewport handles it)
	m.stepsView.SetContent(m.buildStepsView(true, true))

	currentStepName := "Waiting..."
	outputContent := "Pipeline starting..."
	displayStep := m.getDisplayStep()
	if displayStep < len(m.steps) {
		currentStepName = m.steps[displayStep].Name
		if m.steps[displayStep].Output != "" {
			outputContent = m.steps[displayStep].Output
		} else if m.steps[displayStep].Status == StatusRunning {
			outputContent = "Running..."
		} else if m.steps[displayStep].Status == StatusCompleted {
			outputContent = "Completed (no output)"
		}
	}
	m.outputView.SetContent(outputContent)

	// Auto-scroll
	if !m.pipelineEnded && displayStep == m.currentStep && m.steps[displayStep].Status == StatusRunning {
		if m.userScrolling && m.outputView.AtBottom() {
			m.userScrolling = false
		}
		if !m.userScrolling {
			m.outputView.GotoBottom()
		}
	}

	// File changes
	diffContent := "No changes yet"
	if len(m.filesChanged) > 0 {
		var dc strings.Builder
		for _, file := range m.filesChanged {
			dc.WriteString(greenStyle.Render("+ " + file))
			dc.WriteString("\n")
		}
		diffContent = dc.String()
	}
	m.diffView.SetContent(diffContent)

	// Build titles (truncate if needed)
	outputTitle := fmt.Sprintf("OUTPUT: %s", currentStepName)
	if m.focusedPanel == FocusOutput {
		outputTitle += " ◀"
	}
	if lipgloss.Width(outputTitle) > rightTextWidth {
		outputTitle = outputTitle[:rightTextWidth-1] + "…"
	}
	diffTitle := "FILE CHANGES"
	if m.focusedPanel == FocusDiff {
		diffTitle += " ◀"
	}

	// Render panels simply: title + viewport.View() produces exact dimensions
	// viewport.View() already outputs exactly textWidth × textHeight
	outputPanel := panelStyle.Width(rightOuter).Render(
		magentaBoldStyle.Render(outputTitle) + "\n\n" + m.outputView.View())
	diffPanel := panelStyle.Width(rightOuter).Render(
		magentaBoldStyle.Render(diffTitle) + "\n\n" + m.diffView.View())
	stepsPanel := panelStyle.Width(stepsOuter).Render(m.stepsView.View())

	// Force exact heights by truncating/padding
	outputPanel = fixedHeightContent(outputPanel, outputOuter)
	diffPanel = fixedHeightContent(diffPanel, diffOuter)
	stepsPanel = fixedHeightContent(stepsPanel, contentHeight)

	rightPanels := lipgloss.JoinVertical(lipgloss.Left, outputPanel, diffPanel)

	result := lipgloss.JoinHorizontal(lipgloss.Top, stepsPanel, rightPanels)
	return fixedHeightContent(result, contentHeight)
}

// isNarrowMode returns true if terminal is too narrow for side-by-side layout
func (m *TUIModel) isNarrowMode() bool {
	return m.width < NarrowModeWidth
}

// renderNarrowView renders stacked layout for narrow terminals
func (m *TUIModel) renderNarrowView() string {
	// In lipgloss v2, Width(W) is the TOTAL outer width (border+padding+text)
	const frameTotal = 4 // border(2) + padding(2)
	const borderV = 2

	contentHeight := m.height

	// Title (compact)
	titleText := "OCTOS"
	if m.maxLoops != 1 {
		titleText += fmt.Sprintf(" [%d", m.currentLoop)
		if m.maxLoops > 0 {
			titleText += fmt.Sprintf("/%d", m.maxLoops)
		}
		titleText += "]"
	}
	title := titleStyle.Render(titleText)

	// Progress
	completed := m.countCompletedSteps()
	percent := float64(completed) / float64(len(m.steps))
	progressBar := m.progress.ViewAs(percent)
	progressText := fmt.Sprintf("%d/%d", completed, len(m.steps))
	progressLine := progressBarStyle.Render(progressBar) + " " + progressText

	// Fixed: title(1) + progress(1) + help(1) = 3 lines
	fixedHeight := 3
	availableHeight := contentHeight - fixedHeight
	if availableHeight < 10 {
		availableHeight = 10
	}

	// Panel outer = terminal width, text = width - frameTotal
	panelOuter := m.width
	textWidth := panelOuter - frameTotal
	if textWidth < 10 {
		textWidth = 10
	}

	// Steps panel (30% of available height)
	stepsOuter := (availableHeight * 30) / 100
	if stepsOuter < 3 + borderV {
		stepsOuter = 3 + borderV
	}
	stepsTextHeight := stepsOuter - borderV

	m.stepsView.SetWidth(textWidth)
	m.stepsView.SetHeight(stepsTextHeight)
	m.stepsView.SetContent(m.buildStepsView(false, false))
	stepsPanel := panelStyle.Width(panelOuter).Render(m.stepsView.View())
	stepsPanel = fixedHeightContent(stepsPanel, stepsOuter)

	// Output panel (remaining height)
	outputOuter := availableHeight - stepsOuter
	if outputOuter < 5 + borderV {
		outputOuter = 5 + borderV
	}
	outputTextHeight := outputOuter - borderV - 1 // -1 for title line
	if outputTextHeight < 3 {
		outputTextHeight = 3
	}

	m.outputView.SetWidth(textWidth)
	m.outputView.SetHeight(outputTextHeight)
	m.outputView.SoftWrap = true

	displayStep := m.getDisplayStep()
	currentStepName := "Waiting..."
	outputContent := "Pipeline starting..."
	if displayStep < len(m.steps) {
		currentStepName = m.steps[displayStep].Name
		if m.steps[displayStep].Output != "" {
			outputContent = m.steps[displayStep].Output
		}
	}
	m.outputView.SetContent(outputContent)

	outputTitle := fmt.Sprintf("OUT: %s", currentStepName)
	if m.focusedPanel == FocusOutput {
		outputTitle += " ◀"
	}
	outputPanel := panelStyle.Width(panelOuter).Render(
		PanelTitleStyle().Render(outputTitle) + "\n" + m.outputView.View())
	outputPanel = fixedHeightContent(outputPanel, outputOuter)

	// Help (compact)
	help := "[Tab] Switch │ [j/k] Scroll │ [q] Quit"

	content := lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		progressLine,
		stepsPanel,
		outputPanel,
		cyanFaintStyle.Render(help),
	)

	return content
}

// fixedHeightContent truncates or pads content to exactly `height` lines.
// This guarantees panels have a fixed size regardless of content length.
func fixedHeightContent(content string, height int) string {
	if height <= 0 {
		return ""
	}
	lines := strings.Split(content, "\n")
	if len(lines) > height {
		lines = lines[:height]
	}
	for len(lines) < height {
		lines = append(lines, "")
	}
	return strings.Join(lines, "\n")
}

// initPromptView initializes the prompt viewport with current step's prompt
func (m *TUIModel) initPromptView() {
	if m.selectedStep >= len(m.steps) {
		return
	}
	
	prompt := m.steps[m.selectedStep].Prompt
	popupWidth := int(float64(m.width) * popupWidthRatio)
	if popupWidth > NarrowModeWidth {
		popupWidth = NarrowModeWidth
	}
	popupHeight := int(float64(m.height) * popupHeightRatio)
	if popupHeight > PopupMaxHeight {
		popupHeight = PopupMaxHeight
	}
	
	m.promptView = viewport.New(viewport.WithWidth(popupWidth-popupTextPadding), viewport.WithHeight(popupHeight-popupViewportOffset))
	m.promptView.SoftWrap = true
	m.promptView.SetContent(prompt)
}

// restartPipeline resets the pipeline state and starts again
func (m *TUIModel) restartPipeline() (tea.Model, tea.Cmd) {
	m.currentLoop++
	
	// Reset all steps to pending
	for i := range m.steps {
		m.steps[i].Status = StatusPending
		m.steps[i].Output = ""
		m.steps[i].Error = nil
		m.steps[i].Duration = 0
	}
	
	// Reset state
	m.currentStep = 0
	m.selectedStep = 0
	m.pipelineEnded = false
	m.filesChanged = []string{}
	m.startTime = time.Now()
	m.endTime = time.Time{}
	m.userScrolling = false
	
	// Reset progress bar (ViewAs is used for rendering, no animation state needed)
	_ = m.progress.SetPercent(0)
	
	m.statusMsg = fmt.Sprintf("Restarting pipeline (loop %d", m.currentLoop)
	if m.maxLoops > 0 {
		m.statusMsg += fmt.Sprintf("/%d", m.maxLoops)
	}
	m.statusMsg += ")..."
	
	// Trigger pipeline start
	return m, func() tea.Msg { return startPipelineMsg{} }
}

func (m *TUIModel) scrollToStep(stepIndex int) {
	if !m.isValidStepIndex(stepIndex) {
		return
	}
	
	// Each step takes 1 line, plus 2 lines for header
	lineHeight := 1
	targetLine := stepIndex * lineHeight
	
	// Center the target step in viewport
	halfHeight := m.stepsView.Height() / 2
	scrollTo := targetLine - halfHeight
	if scrollTo < 0 {
		scrollTo = 0
	}
	
	m.stepsView.SetYOffset(scrollTo)
}

func (m *TUIModel) renderPromptPopup(baseContent string) string {
	stepName := m.steps[m.selectedStep].Name
	
	popupWidth := int(float64(m.width) * popupWidthRatio)
	if popupWidth > NarrowModeWidth {
		popupWidth = NarrowModeWidth
	}
	popupHeight := int(float64(m.height) * popupHeightRatio)
	if popupHeight > PopupMaxHeight {
		popupHeight = PopupMaxHeight
	}
	
	// Create popup style
	popupStyle := lipgloss.NewStyle().
		Border(lipgloss.DoubleBorder()).
		BorderForeground(neonMagenta).
		Background(darkerBg).
		Padding(1, 2).
		Width(popupWidth).
		Height(popupHeight)
	
	// Title
	popupTitle := magentaBoldStyle.Render(fmt.Sprintf("PROMPT: %s", stepName))
	
	// Footer with scroll hint
	scrollPercent := m.promptView.ScrollPercent()
	scrollInfo := fmt.Sprintf("%.0f%%", scrollPercent*100)
	footer := lipgloss.NewStyle().
		Foreground(neonYellow).
		Faint(true).
		Render(fmt.Sprintf("[j/k] Scroll │ [Enter/Esc] Close │ %s", scrollInfo))
	
	popupContent := lipgloss.JoinVertical(
		lipgloss.Left,
		popupTitle,
		"",
		m.promptView.View(),
		"",
		footer,
	)
	
	popup := popupStyle.Render(popupContent)
	
	// Overlay popup on base content
	return lipgloss.Place(
		m.width,
		m.height,
		lipgloss.Center,
		lipgloss.Center,
		popup,
		lipgloss.WithWhitespaceChars(" "),
		lipgloss.WithWhitespaceStyle(lipgloss.NewStyle().Foreground(lipgloss.Color("0"))),
	)
}

func runPipelineWithProgram(p *Pipeline, resume bool, program *tea.Program, runner *PipelineRunner) {
	RunPipelineWithOptions(p, RunOptions{
		OnStart: func(stepIndex int, prompt string) {
			if program != nil {
				program.Send(stepStartMsg{index: stepIndex, prompt: prompt})
			}
		},
		OnOutput: func(stepIndex int, output string) {
			if program != nil {
				program.Send(stepOutputMsg{index: stepIndex, output: output})
			}
		},
		OnComplete: func(stepIndex int, duration time.Duration, err error) {
			if program != nil {
				program.Send(stepCompleteMsg{index: stepIndex, duration: duration, err: err})
			}
		},
		OnStream: func(stepIndex int, line string) {
			if program != nil {
				program.Send(stepStreamMsg{index: stepIndex, line: line})
			}
		},
		OnFileChanges: func(stepIndex int, changes []string) {
			if program != nil {
				program.Send(fileChangesMsg{index: stepIndex, changes: changes})
			}
		},
		OnRetry: func(stepIndex int, attempt int, maxRetries int) {
			if program != nil {
				program.Send(stepRetryMsg{index: stepIndex, attempt: attempt, maxRetries: maxRetries})
			}
		},
		OnSkip: func(stepIndex int) {
			if program != nil {
				program.Send(stepSkipMsg{index: stepIndex})
			}
		},
		OnDone: func() {
			if program != nil {
				program.Send(pipelineDoneMsg{})
			}
		},
		Runner: runner,
		Resume: resume,
	})
}
