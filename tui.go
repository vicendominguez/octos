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
}

type TUIModel struct {
	pipeline      *Pipeline
	steps         []StepState
	currentStep   int
	selectedStep  int
	outputView    viewport.Model
	diffView      viewport.Model
	progress      progress.Model
	width         int
	height        int
	startTime     time.Time
	endTime       time.Time
	filesChanged  []string
	quitting      bool
	resuming      bool
	program       *tea.Program
	pipelineEnded bool
	statusMsg     string
	workingDir    string
	gitBranch     string
}

type stepStartMsg struct{ index int }
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

var (
	// Cyberpunk color scheme
	neonCyan    = lipgloss.Color("51")  // Bright cyan
	neonMagenta = lipgloss.Color("201") // Bright magenta
	neonGreen   = lipgloss.Color("46")  // Bright green
	neonYellow  = lipgloss.Color("226") // Bright yellow
	neonRed     = lipgloss.Color("196") // Bright red
	darkBg      = lipgloss.Color("235") // Dark background
	darkerBg    = lipgloss.Color("233") // Darker background

	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(neonCyan).
			Background(darkerBg).
			Padding(0, 1).
			MarginBottom(1)

	statusBarStyle = lipgloss.NewStyle().
			Foreground(neonCyan).
			Background(darkBg).
			Padding(0, 1).
			Bold(true)

	progressBarStyle = lipgloss.NewStyle().
				Foreground(neonMagenta).
				Bold(true)

	stepPendingStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("240")).
				Faint(true)

	stepRunningStyle = lipgloss.NewStyle().
				Foreground(neonYellow).
				Bold(true).
				Blink(true)

	stepCompletedStyle = lipgloss.NewStyle().
				Foreground(neonGreen).
				Bold(true)

	stepFailedStyle = lipgloss.NewStyle().
			Foreground(neonRed).
			Bold(true)

	panelStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(neonCyan).
			Padding(0, 1)

	statsStyle = lipgloss.NewStyle().
			Foreground(neonMagenta).
			Italic(true)
)

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
		progress.WithWidth(40),
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
		pipeline:   p,
		steps:      steps,
		progress:   prog,
		startTime:  time.Now(),
		resuming:   resume,
		workingDir: workingDir,
		gitBranch:  gitBranch,
	}
}

func (m *TUIModel) Init() tea.Cmd {
	return tea.Batch(
		tickCmd(),
	)
}

func tickCmd() tea.Cmd {
	return tea.Tick(time.Millisecond*100, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func (m *TUIModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		// Calculate panel dimensions with margins
		contentWidth := m.width - 4
		contentHeight := m.height - 4
		
		// Steps panel takes 30% of width, rest for output/diff
		stepsWidth := 30
		if contentWidth < 100 {
			stepsWidth = contentWidth / 3
		}
		
		panelWidth := contentWidth - stepsWidth - 2
		panelHeight := (contentHeight - 16) / 2
		
		if panelHeight < 5 {
			panelHeight = 5
		}

		m.outputView = viewport.New(panelWidth-4, panelHeight)
		m.diffView = viewport.New(panelWidth-4, panelHeight)
		
		// Trigger pipeline start
		return m, func() tea.Msg { return startPipelineMsg{} }

	case startPipelineMsg:
		// Start pipeline after window is ready
		if m.program != nil {
			m.statusMsg = "Starting pipeline..."
			go runPipelineWithProgram(m.pipeline, m.resuming, m.program)
		}
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			m.quitting = true
			return m, tea.Quit
		
		case "j", "down":
			if m.pipelineEnded && m.selectedStep < len(m.steps)-1 {
				m.selectedStep++
			}
			return m, nil
		
		case "k", "up":
			if m.pipelineEnded && m.selectedStep > 0 {
				m.selectedStep--
			}
			return m, nil
		
		case "ctrl+j":
			m.outputView.LineDown(1)
			return m, nil
		
		case "ctrl+k":
			m.outputView.LineUp(1)
			return m, nil
		
		case "ctrl+d":
			m.outputView.HalfViewDown()
			return m, nil
		
		case "ctrl+u":
			m.outputView.HalfViewUp()
			return m, nil
		}

	case tickMsg:
		if !m.quitting {
			return m, tickCmd()
		}
		return m, nil

	case stepStartMsg:
		if msg.index < len(m.steps) {
			m.steps[msg.index].Status = StatusRunning
			m.steps[msg.index].StartTime = time.Now()
			m.currentStep = msg.index
			m.statusMsg = fmt.Sprintf("Running step %d/%d: %s", msg.index+1, len(m.steps), m.steps[msg.index].Name)
		}
		return m, nil

	case stepOutputMsg:
		if msg.index < len(m.steps) {
			m.steps[msg.index].Output = msg.output
			if msg.index == m.currentStep {
				m.outputView.SetContent(msg.output)
				m.outputView.GotoBottom()
			}
		}
		return m, nil

	case stepStreamMsg:
		if msg.index < len(m.steps) {
			m.steps[msg.index].Output += msg.line + "\n"
			if msg.index == m.currentStep {
				m.outputView.SetContent(m.steps[msg.index].Output)
				m.outputView.GotoBottom()
			}
		}
		return m, nil

	case fileChangesMsg:
		if msg.index < len(m.steps) {
			m.filesChanged = append(m.filesChanged, msg.changes...)
		}
		return m, nil

	case stepCompleteMsg:
		if msg.index < len(m.steps) {
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

func (m *TUIModel) View() string {
	if m.width == 0 || m.height == 0 {
		return "Initializing..."
	}

	// Add margins
	contentWidth := m.width - 4
	contentHeight := m.height - 4

	// Title with ASCII art style
	titleText := "OCTOS PIPELINE ORCHESTRATOR"
	titleBorder := strings.Repeat("═", len(titleText)+4)
	title := titleStyle.Render(fmt.Sprintf("╔%s╗\n║  %s  ║\n╚%s╝", titleBorder, titleText, titleBorder))

	// Count completed steps
	completed := 0
	for _, step := range m.steps {
		if step.Status == StatusCompleted {
			completed++
		}
	}

	// Progress bar with gradient effect
	percent := float64(completed) / float64(len(m.steps))
	progressBar := m.progress.ViewAs(percent)
	progressText := fmt.Sprintf("▓▒░ %d/%d (%.0f%%) ░▒▓", completed, len(m.steps), percent*100)
	progressLine := progressBarStyle.Render(progressBar) + " " + lipgloss.NewStyle().Foreground(neonCyan).Bold(true).Render(progressText)

	// Calculate responsive dimensions early
	stepsWidth := 30
	if contentWidth < 100 {
		stepsWidth = contentWidth / 3
	}
	panelWidth := contentWidth - stepsWidth - 2

	// Steps list with cyberpunk style
	var stepsView strings.Builder
	stepsView.WriteString(lipgloss.NewStyle().Foreground(neonCyan).Bold(true).Render("PIPELINE STEPS"))
	stepsView.WriteString("\n\n")
	for i, step := range m.steps {
		var icon, style string
		switch step.Status {
		case StatusPending:
			icon = "○"
			style = stepPendingStyle.Render(fmt.Sprintf("%s %s", icon, step.Name))
		case StatusRunning:
			icon = "⚙"
			style = stepRunningStyle.Render(fmt.Sprintf("%s %s", icon, step.Name))
		case StatusCompleted:
			icon = "✓"
			duration := fmt.Sprintf("%.1fs", step.Duration.Seconds())
			style = stepCompletedStyle.Render(fmt.Sprintf("%s %s", icon, step.Name)) + " " + statsStyle.Render(duration)
		case StatusFailed:
			icon = "✗"
			style = stepFailedStyle.Render(fmt.Sprintf("%s %s", icon, step.Name))
		}
		stepsView.WriteString(style)
		
		// Show indicator for current running step or selected step
		if !m.pipelineEnded && i == m.currentStep && step.Status == StatusRunning {
			stepsView.WriteString(" ◀")
		} else if m.pipelineEnded && i == m.selectedStep {
			stepsView.WriteString(" ◀")
		}
		stepsView.WriteString("\n")
	}

	// Current step output
	currentStepName := "Waiting..."
	outputContent := "Pipeline starting..."
	
	// Show output from selected step (when pipeline ended) or current running step
	displayStep := m.currentStep
	if m.pipelineEnded {
		displayStep = m.selectedStep
	}
	
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
	outputPanel := panelStyle.Width(panelWidth).Render(
		lipgloss.NewStyle().Foreground(neonCyan).Bold(true).Render(fmt.Sprintf("OUTPUT: %s", currentStepName)) + "\n\n" +
			m.outputView.View(),
	)

	// File changes
	diffContent := "No changes yet"
	if len(m.filesChanged) > 0 {
		var dc strings.Builder
		for _, file := range m.filesChanged {
			dc.WriteString(lipgloss.NewStyle().Foreground(neonGreen).Render("+ " + file))
			dc.WriteString("\n")
		}
		diffContent = dc.String()
	}
	m.diffView.SetContent(diffContent)
	
	diffPanel := panelStyle.Width(panelWidth).Render(
		lipgloss.NewStyle().Foreground(neonMagenta).Bold(true).Render("FILE CHANGES") + "\n\n" +
			m.diffView.View(),
	)

	// Stats
	elapsed := time.Since(m.startTime)
	if m.pipelineEnded && !m.endTime.IsZero() {
		elapsed = m.endTime.Sub(m.startTime)
	}
	
	// Count running steps as in progress
	running := 0
	for _, step := range m.steps {
		if step.Status == StatusRunning {
			running = 1
			break
		}
	}
	
	stats := statsStyle.Render(
		fmt.Sprintf("⚡ Elapsed: %s │ Steps: %d/%d │ Speed: %.1f steps/min",
			elapsed.Round(time.Second),
			completed+running,
			len(m.steps),
			func() float64 {
				if elapsed.Minutes() > 0 && completed > 0 {
					return float64(completed) / elapsed.Minutes()
				}
				return 0.0
			}(),
		),
	)

	// Enhanced status bar with context
	currentTime := time.Now().Format("15:04")
	
	statusParts := []string{
		lipgloss.NewStyle().Foreground(neonCyan).Render("⏰ " + currentTime),
		lipgloss.NewStyle().Foreground(neonYellow).Render("📁 " + m.workingDir),
	}
	
	if m.gitBranch != "" {
		statusParts = append(statusParts, 
			lipgloss.NewStyle().Foreground(neonGreen).Render("🌿 " + m.gitBranch))
	}
	
	if m.statusMsg != "" {
		statusParts = append(statusParts, m.statusMsg)
	}
	
	statusBar := statusBarStyle.Width(contentWidth).Render(
		strings.Join(statusParts, " │ "),
	)

	// Calculate responsive dimensions
	panelHeight := contentHeight - 16
	if panelHeight < 10 {
		panelHeight = 10
	}

	// Layout with margins
	content := lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		progressLine,
		"",
		lipgloss.JoinHorizontal(
			lipgloss.Top,
			panelStyle.Width(stepsWidth).Height(panelHeight).Render(stepsView.String()),
			lipgloss.JoinVertical(
				lipgloss.Left,
				outputPanel,
				diffPanel,
			),
		),
		"",
		statusBar,
		stats,
		"",
		lipgloss.NewStyle().Foreground(neonCyan).Faint(true).Render(
			func() string {
				if m.pipelineEnded {
					return "⌨  [↑↓/jk] Navigate steps │ [Ctrl+j/k] Scroll output │ [Ctrl+d/u] Page │ [q] Quit"
				}
				return "⌨  [Ctrl+j/k] Scroll output │ [Ctrl+d/u] Page │ [q] Quit"
			}(),
		),
	)

	// Add margins
	return lipgloss.NewStyle().Margin(1, 2).Render(content)
}

func (m *TUIModel) updateDiffView() {
	var content strings.Builder
	for _, file := range m.filesChanged {
		content.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("42")).Render("+ " + file))
		content.WriteString("\n")
	}
	m.diffView.SetContent(content.String())
}

func runPipelineWithProgram(p *Pipeline, resume bool, program *tea.Program) {
	RunPipelineWithCallbacks(p,
		func(stepIndex int, _ string) {
			if program != nil {
				program.Send(stepStartMsg{index: stepIndex})
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
