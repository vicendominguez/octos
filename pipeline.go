package main

import (
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

	// Ensure artifacts directory exists
	artifactsDir := filepath.Join(".octos", "artifacts")
	if err := os.MkdirAll(artifactsDir, 0755); err != nil {
		return nil, err
	}

	return &p, nil
}
