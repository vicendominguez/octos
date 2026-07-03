package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	tea "charm.land/bubbletea/v2"
)

func main() {
	useTUI := flag.Bool("tui", true, "Use TUI mode (default)")
	showVersion := flag.Bool("version", false, "Show version")
	resume := flag.Bool("resume", false, "Resume from last checkpoint")
	clean := flag.Bool("clean", false, "Clean state and start fresh")
	dryRun := flag.Bool("dry-run", false, "Validate pipeline without executing agents")
	loop := flag.Int("loop", 0, "Number of times to run pipeline (0 = no loop, N = repeat N times)")
	flag.Parse()

	if *showVersion {
		fmt.Printf("octos version %s\n", Version)
		return
	}

	args := flag.Args()
	if len(args) < 1 {
		log.Fatal("Usage: octos [--tui] [--resume] [--clean] [--loop N] <pipeline.yaml>")
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

	if *dryRun {
		if err := DryRun(pipeline); err != nil {
			os.Exit(1)
		}
		return
	}

	if *useTUI {
		// TUI mode
		m := NewTUIModel(pipeline, *resume)
		m.maxLoops = *loop
		p := tea.NewProgram(&m)
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
