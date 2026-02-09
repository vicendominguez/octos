package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// UI Layout Constants
const (
	defaultProgressWidth = 40
	tickInterval         = 100 * time.Millisecond
	uiFixedLines         = 14
	minContentHeight     = 10
	panelBorderOverhead  = 8
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
	
	// Layout spacing
	contentVerticalPadding = 6
)

type StepStatus int

const (
	StatusPending StepStatus = iota
	StatusRunning
	StatusCompleted
	StatusFailed
)

type StepState struct {
	Name      string
	Status    StepStatus
	Duration  time.Duration
	StartTime time.Time
	Output    string
	Error     error
	Prompt    string
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
type stepCompleteMsg struct {
	index    int
	duration time.Duration
	err      error
}
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
		state, _ := LoadState(p.File)
		for i := 0; i <= state.LastCompletedStep && i < len(steps); i++ {
			steps[i].Status = StatusCompleted
			steps[i].Duration = time.Second // Placeholder
		}
	}

	prog := progress.New(
		progress.WithDefaultGradient(),
		progress.WithWidth(defaultProgressWidth),
	)

	// Get working directory
	wd, _ := os.Getwd()
	workingDir := filepath.Base(wd)

	// Get git branch (silent fail if not a repo)
	gitBranch := ""
	if out, err := exec.Command("git", "branch", "--show-current").Output(); err == nil {
		gitBranch = strings.TrimSpace(string(out))
	}

	return TUIModel{
		pipeline:    p,
		steps:       steps,
		progress:    prog,
		startTime:   time.Now(),
		resuming:    resume,
		workingDir:  workingDir,
		gitBranch:   gitBranch,
		maxLoops:    0,
		currentLoop: 1,
	}
}

func (m *TUIModel) Init() tea.Cmd {
	return tea.Batch(
		tickCmd(),
		tea.EnableMouseCellMotion,
	)
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

		contentWidth := m.width
		contentHeight := m.height
		
		// Update progress bar width responsively
		progressWidth := contentWidth / progressWidthDivisor
		if progressWidth < minProgressWidth {
			progressWidth = minProgressWidth
		}
		if progressWidth > maxProgressWidth {
			progressWidth = maxProgressWidth
		}
		m.progress.Width = progressWidth
		
		stepsWidth := m.calculateStepsWidth(contentWidth)
		panelWidth := contentWidth - stepsWidth - 2
		
		availableHeight := contentHeight - uiFixedLines
		if availableHeight < minContentHeight {
			availableHeight = minContentHeight
		}
		outputHeight := (availableHeight * OutputPanelPct) / 100
		diffHeight := (availableHeight * DiffPanelPct) / 100
		
		if outputHeight < MinPanelHeight {
			outputHeight = MinPanelHeight
		}
		if diffHeight < MinPanelHeight {
			diffHeight = MinPanelHeight
		}

		stepsHeight := availableHeight
		if stepsHeight < MinStepsHeight {
			stepsHeight = MinStepsHeight
		}

		m.stepsView = viewport.New(stepsWidth-PanelBorderPadding, stepsHeight-3)
		m.outputView = viewport.New(panelWidth-PanelBorderPadding, outputHeight)
		m.diffView = viewport.New(panelWidth-PanelBorderPadding, diffHeight)
		
		// Only trigger pipeline start on first window size event
		if m.currentStep == 0 && !m.pipelineEnded {
			return m, func() tea.Msg { return startPipelineMsg{} }
		}
		return m, nil

	case startPipelineMsg:
		// Start pipeline after window is ready
		if m.program != nil {
			m.statusMsg = "Starting pipeline..."
			go runPipelineWithProgram(m.pipeline, m.resuming, m.program)
		}
		return m, nil

	case tea.MouseMsg:
		switch msg.Type {
		case tea.MouseWheelUp:
			m.userScrolling = true
			if m.focusedPanel == FocusOutput {
				m.outputView.LineUp(ScrollLines)
			} else {
				m.diffView.LineUp(ScrollLines)
			}
			return m, nil
		case tea.MouseWheelDown:
			if m.focusedPanel == FocusOutput {
				m.outputView.LineDown(ScrollLines)
				if m.outputView.AtBottom() {
					m.userScrolling = false
				} else {
					m.userScrolling = true
				}
			} else {
				m.diffView.LineDown(ScrollLines)
				m.userScrolling = true
			}
			return m, nil
		}

	case tea.KeyMsg:
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

			if msg.index == len(m.steps)-1 {
				// Pipeline ended, enable navigation
				m.pipelineEnded = true
				m.endTime = time.Now()
				m.selectedStep = msg.index
				m.statusMsg = "Pipeline completed! Use ↑↓/jk to navigate steps"
			}
		}
		return m, nil
	}

	var cmd tea.Cmd
	m.outputView, cmd = m.outputView.Update(msg)
	return m, cmd
}

func (m *TUIModel) handleKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
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
		if step.Status == StatusCompleted {
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

func (m *TUIModel) handleDownKey() (tea.Model, tea.Cmd) {
	if m.showPrompt {
		m.promptView.LineDown(1)
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
		m.promptView.LineUp(1)
		return m, nil
	}
	if m.pipelineEnded && m.selectedStep > 0 {
		m.selectedStep--
		m.scrollToStep(m.selectedStep)
	}
	return m, nil
}

func (m *TUIModel) scrollPanelLines(lines int) {
	if m.focusedPanel == FocusOutput {
		if lines > 0 {
			m.outputView.LineDown(lines)
		} else {
			m.outputView.LineUp(-lines)
		}
		if m.outputView.AtBottom() {
			m.userScrolling = false
		} else {
			m.userScrolling = true
		}
	} else {
		if lines > 0 {
			m.diffView.LineDown(lines)
		} else {
			m.diffView.LineUp(-lines)
		}
		m.userScrolling = true
	}
}

func (m *TUIModel) scrollPanelHalfPage(down bool) {
	if m.focusedPanel == FocusOutput {
		if down {
			m.outputView.HalfViewDown()
		} else {
			m.outputView.HalfViewUp()
		}
		if m.outputView.AtBottom() {
			m.userScrolling = false
		} else {
			m.userScrolling = true
		}
	} else {
		if down {
			m.diffView.HalfViewDown()
		} else {
			m.diffView.HalfViewUp()
		}
		m.userScrolling = true
	}
}

func (m *TUIModel) View() string {
	if m.width == 0 || m.height == 0 {
		return "Initializing..."
	}

	// Use stacked layout for narrow terminals
	if m.isNarrowMode() {
		return m.renderNarrowView()
	}

	// Render header (fixed height)
	header := m.renderHeader()
	
	// Render footer (fixed height)
	footer := m.renderFooter()
	
	// Calculate content area height based on ACTUAL rendered header/footer heights
	headerLines := strings.Count(header, "\n") + 1
	footerLines := strings.Count(footer, "\n") + 1
	
	contentHeight := m.height - headerLines - footerLines - contentVerticalPadding
	if contentHeight < 5 {
		contentHeight = 5
	}
	
	// Render content panels
	content := m.renderContent(m.width, contentHeight)
	
	// Stack header, content, footer
	result := lipgloss.JoinVertical(lipgloss.Left, header, content, footer)
	
	// Truncate to fit terminal height exactly
	lines := strings.Split(result, "\n")
	if len(lines) > m.height {
		lines = lines[:m.height]
		result = strings.Join(lines, "\n")
	}
	
	// Render popup if showing prompt
	if m.showPrompt && m.pipelineEnded && m.selectedStep < len(m.steps) && m.steps[m.selectedStep].Prompt != "" {
		result = m.renderPromptPopup(result)
	}
	
	return result
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
			return "⌨  [Tab] Switch panel │ [Ctrl+j/k] Scroll │ [Ctrl+d/u] Page │ [Mouse wheel] Scroll │ [q] Quit"
		} else if m.width >= narrowTerminalWidth {
			return "⌨  [Tab] Panel │ [Ctrl+j/k] Scroll │ [Ctrl+d/u] Page │ [q] Quit"
		} else {
			return "⌨  [Tab] Panel │ [j/k] Scroll │ [q] Quit"
		}
	}
}

func (m *TUIModel) renderContent(width, contentHeight int) string {
	panelOverhead := panelBorderOverhead
	availableWidth := width - panelOverhead
	
	stepsContentWidth := (availableWidth * StepsWidthPct) / 100
	if stepsContentWidth < MinStepsWidth {
		stepsContentWidth = MinStepsWidth
	}
	
	rightPanelContentWidth := availableWidth - stepsContentWidth
	if rightPanelContentWidth < minRightPanelWidth {
		rightPanelContentWidth = minRightPanelWidth
	}

	m.stepsView.SetContent(m.buildStepsView(true, true))

	// Current step output
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
	
	// Wrap output content to viewport width (subtract 2 for safety margin)
	wrapWidth := m.outputView.Width - 2
	if wrapWidth < 10 {
		wrapWidth = 10
	}
	wrappedOutput := wrapText(outputContent, wrapWidth)
	m.outputView.SetContent(wrappedOutput)
	
	// Auto-scroll to bottom only if user hasn't manually scrolled and step is running
	if !m.pipelineEnded && !m.userScrolling && displayStep == m.currentStep && m.steps[displayStep].Status == StatusRunning {
		m.outputView.GotoBottom()
	}
	
	// Add focus indicator to panel titles
	outputTitle := fmt.Sprintf("OUTPUT: %s", currentStepName)
	if m.focusedPanel == FocusOutput {
		outputTitle += " ◀"
	}
	
	outputPanelHeight := (contentHeight * OutputPanelPct) / 100
	diffPanelHeight := contentHeight - outputPanelHeight
	if outputPanelHeight < MinPanelHeight {
		outputPanelHeight = MinPanelHeight
	}
	if diffPanelHeight < MinPanelHeight {
		diffPanelHeight = MinPanelHeight
	}
	
	outputPanel := panelStyle.Width(rightPanelContentWidth).Height(outputPanelHeight).Render(
		magentaBoldStyle.Render(outputTitle) + "\n\n" +
			m.outputView.View(),
	)

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
	
	// Add focus indicator to diff panel title
	diffTitle := "FILE CHANGES"
	if m.focusedPanel == FocusDiff {
		diffTitle += " ◀"
	}
	
	diffPanel := panelStyle.Width(rightPanelContentWidth).Height(diffPanelHeight).Render(
		magentaBoldStyle.Render(diffTitle) + "\n\n" +
			m.diffView.View(),
	)

	rightPanels := lipgloss.JoinVertical(lipgloss.Left, outputPanel, diffPanel)
	totalRightHeight := lipgloss.Height(rightPanels)
	
	// Subtract 2 to account for steps panel's own border (top+bottom)
	stepsPanel := panelStyle.Width(stepsContentWidth).Height(totalRightHeight - 2).Render(m.stepsView.View())

	return lipgloss.JoinHorizontal(
		lipgloss.Top,
		stepsPanel,
		rightPanels,
	)
}

// isNarrowMode returns true if terminal is too narrow for side-by-side layout
func (m *TUIModel) isNarrowMode() bool {
	return m.width < NarrowModeWidth
}

// renderNarrowView renders stacked layout for narrow terminals
func (m *TUIModel) renderNarrowView() string {
	contentWidth := m.width
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

	// Calculate available height for panels
	// Fixed: title(1) + progress(1) + help(1) = 3 lines
	fixedHeight := 3
	availableHeight := contentHeight - fixedHeight
	if availableHeight < 10 {
		availableHeight = 10
	}

	// Steps (30% of available height, min 3)
	stepsHeight := (availableHeight * 30) / 100
	if stepsHeight < 3 {
		stepsHeight = 3
	}
	
	m.stepsView.SetContent(m.buildStepsView(false, false))
	stepsPanel := panelStyle.Width(contentWidth).Height(stepsHeight).Render(m.stepsView.View())

	// Output (remaining height)
	outputHeight := availableHeight - stepsHeight
	if outputHeight < 5 {
		outputHeight = 5
	}
	
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
	outputPanel := panelStyle.Width(contentWidth).Height(outputHeight).Render(
		PanelTitleStyle().Render(outputTitle) + "\n" + m.outputView.View(),
	)

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

// calculateStepsWidth returns the width for the steps panel based on terminal width
func (m *TUIModel) calculateStepsWidth(contentWidth int) int {
	stepsWidth := (contentWidth * StepsWidthPct) / 100
	if stepsWidth < MinStepsWidth {
		stepsWidth = MinStepsWidth
	}
	return stepsWidth
}

// wrapText hard-wraps text to fit within width
func wrapText(text string, width int) string {
	if width <= 0 {
		return text
	}
	
	var result strings.Builder
	lines := strings.Split(text, "\n")
	
	for i, line := range lines {
		currentWidth := 0
		for _, r := range line {
			runeWidth := lipgloss.Width(string(r))
			if currentWidth+runeWidth > width && currentWidth > 0 {
				result.WriteString("\n")
				currentWidth = 0
			}
			result.WriteRune(r)
			currentWidth += runeWidth
		}
		if i < len(lines)-1 {
			result.WriteString("\n")
		}
	}
	
	return result.String()
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
	
	wrappedPrompt := wrapText(prompt, popupWidth-popupTextPadding)
	m.promptView = viewport.New(popupWidth-popupTextPadding, popupHeight-popupViewportOffset)
	m.promptView.SetContent(wrappedPrompt)
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
	
	// Reset progress bar
	m.progress.SetPercent(0)
	
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
	halfHeight := m.stepsView.Height / 2
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
		lipgloss.WithWhitespaceForeground(lipgloss.Color("0")),
	)
}

func runPipelineWithProgram(p *Pipeline, resume bool, program *tea.Program) {
	RunPipelineWithCallbacks(p,
		func(stepIndex int, prompt string) {
			if program != nil {
				program.Send(stepStartMsg{index: stepIndex, prompt: prompt})
			}
		},
		func(stepIndex int, output string) {
			if program != nil {
				program.Send(stepOutputMsg{index: stepIndex, output: output})
			}
		},
		func(stepIndex int, duration time.Duration, err error) {
			if program != nil {
				program.Send(stepCompleteMsg{index: stepIndex, duration: duration, err: err})
			}
		},
		func(stepIndex int, line string) {
			if program != nil {
				program.Send(stepStreamMsg{index: stepIndex, line: line})
			}
		},
		func(stepIndex int, changes []string) {
			if program != nil {
				program.Send(fileChangesMsg{index: stepIndex, changes: changes})
			}
		},
		resume,
	)
}
