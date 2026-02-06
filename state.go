package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

type PipelineState struct {
	PipelineFile      string            `json:"pipeline_file"`
	LastCompletedStep int               `json:"last_completed_step"`
	Outputs           map[string]string `json:"outputs"`
	StartTime         string            `json:"start_time"`
	LastUpdate        string            `json:"last_update"`
}

func getStateDir() string {
	return filepath.Join(".octos", "state")
}

func getStateFile(pipelineFile string) string {
	return filepath.Join(getStateDir(), filepath.Base(pipelineFile)+".json")
}

func SaveState(state *PipelineState) error {
	stateDir := getStateDir()
	if err := os.MkdirAll(stateDir, 0755); err != nil {
		return err
	}

	state.LastUpdate = time.Now().Format(time.RFC3339)

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(getStateFile(state.PipelineFile), data, 0644)
}

func LoadState(pipelineFile string) (*PipelineState, error) {
	data, err := os.ReadFile(getStateFile(pipelineFile))
	if err != nil {
		return nil, err
	}

	var state PipelineState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, err
	}

	return &state, nil
}

func StateExists(pipelineFile string) bool {
	_, err := os.Stat(getStateFile(pipelineFile))
	return err == nil
}

func ClearState(pipelineFile string) error {
	stateFile := getStateFile(pipelineFile)
	if _, err := os.Stat(stateFile); err == nil {
		return os.Remove(stateFile)
	}
	return nil
}
