package main

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type Pipeline struct {
	File    string            `json:"-"`
	Agent   AgentConfig       `yaml:"agent"`
	Context map[string]any    `yaml:"context"`
	Steps   []Step            `yaml:"steps"`
}

type AgentConfig struct {
	Cmd  string   `yaml:"cmd"`
	Args []string `yaml:"args"`
}

type Step struct {
	Name     string       `yaml:"name"`
	Prompt   string       `yaml:"prompt"`
	SaveTo   string       `yaml:"save_to"`
	LoadFrom string       `yaml:"load_from"`
	When     string       `yaml:"when"`
	Agent    *AgentConfig `yaml:"agent,omitempty"`
}

func LoadPipeline(path string) (*Pipeline, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var p Pipeline
	if err := yaml.Unmarshal(data, &p); err != nil {
		return nil, err
	}

	p.File = path

	// Validate pipeline
	if err := p.Validate(); err != nil {
		return nil, err
	}

	// Ensure artifacts directory exists
	artifactsDir := filepath.Join(".octos", "artifacts")
	if err := os.MkdirAll(artifactsDir, 0755); err != nil {
		return nil, err
	}

	return &p, nil
}

// Validate checks if the pipeline configuration is valid
func (p *Pipeline) Validate() error {
	if p.Agent.Cmd == "" {
		return fmt.Errorf("agent.cmd is required")
	}
	
	if len(p.Steps) == 0 {
		return fmt.Errorf("at least one step is required")
	}
	
	for i, step := range p.Steps {
		if step.Name == "" {
			return fmt.Errorf("step %d: name is required", i+1)
		}
		if step.Prompt == "" {
			return fmt.Errorf("step %d (%s): prompt is required", i+1, step.Name)
		}
	}
	
	return nil
}
