package main

import (
	"flag"
	"fmt"
	"log"

	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	noTUI := flag.Bool("no-tui", false, "Disable TUI (headless mode)")
	showVersion := flag.Bool("version", false, "Show version")
	resume := flag.Bool("resume", false, "Resume from last checkpoint")
	clean := flag.Bool("clean", false, "Clean state and start fresh")
	loop := flag.Int("loop", 0, "Number of times to run pipeline (0 = infinite in TUI, 1 in headless)")
	flag.Parse()

	if *showVersion {
		fmt.Printf("octos version %s\n", Version)
		return
	}

	args := flag.Args()
	if len(args) < 1 {
		fmt.Println("Usage: octos [options] <pipeline.yaml>")
		fmt.Println("\nOptions:")
		flag.PrintDefaults()
		return
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

	if !*noTUI {
		// TUI mode
		m := NewTUIModel(pipeline, *resume)
		m.maxLoops = *loop
		p := tea.NewProgram(&m, tea.WithAltScreen())
		m.program = p
		if _, err := p.Run(); err != nil {
			log.Fatal(err)
		}
	} else {
		// Headless mode - loop must be finite (default to 1 if 0)
		loopCount := *loop
		if loopCount == 0 {
			loopCount = 1
		}
		
		for i := 1; i <= loopCount; i++ {
			if loopCount > 1 {
				fmt.Printf("\n→ Loop iteration %d/%d\n", i, loopCount)
			}
			
			if err := RunPipelineWithResume(pipeline, *resume && i == 1); err != nil {
				log.Fatal(err)
			}
		}
		
		fmt.Println("✓ Pipeline completed")
	}
}
