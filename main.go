package main

import (
	"flag"
	"fmt"
	"log"

	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	useTUI := flag.Bool("tui", true, "Use TUI mode (default)")
	showVersion := flag.Bool("version", false, "Show version")
	resume := flag.Bool("resume", false, "Resume from last checkpoint")
	clean := flag.Bool("clean", false, "Clean state and start fresh")
	flag.Parse()

	if *showVersion {
		fmt.Printf("octos version %s\n", Version)
		return
	}

	args := flag.Args()
	if len(args) < 1 {
		log.Fatal("Usage: octos [--tui] [--resume] [--clean] <pipeline.yaml>")
	}

	pipelineFile := args[0]

	if *clean {
		if err := ClearState(pipelineFile); err != nil {
			log.Fatalf("Failed to clear state: %v", err)
		}
		fmt.Println("✓ State cleared")
		return
	}

	pipeline, err := LoadPipeline(pipelineFile)
	if err != nil {
		log.Fatal(err)
	}

	if *useTUI {
		// TUI mode
		m := NewTUIModel(pipeline, *resume)
		p := tea.NewProgram(&m, tea.WithAltScreen())
		m.program = p
		if _, err := p.Run(); err != nil {
			log.Fatal(err)
		}
	} else {
		// CLI mode
		if err := RunPipelineWithResume(pipeline, *resume); err != nil {
			log.Fatal(err)
		}
		fmt.Println("✓ Pipeline completed")
	}
}
