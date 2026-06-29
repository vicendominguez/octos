package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

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
	Name       string       `yaml:"name"`
	Prompt     string       `yaml:"prompt"`
	PromptFile string       `yaml:"prompt_file,omitempty"`
	SaveTo     string       `yaml:"save_to"`
	LoadFrom   string       `yaml:"load_from"`
	When       string       `yaml:"when"`
	Agent      *AgentConfig `yaml:"agent,omitempty"`
}

// expandEnvWithDefaults expands ${VAR}, $VAR, and ${VAR:-default} syntax.
// If VAR is unset or empty and a default is provided via :-, the default is used.
func expandEnvWithDefaults(s string) string {
	return os.Expand(s, func(key string) string {
		// Handle ${VAR:-default} syntax
		if idx := strings.Index(key, ":-"); idx >= 0 {
			envKey := key[:idx]
			defaultVal := key[idx+2:]
			if val, ok := os.LookupEnv(envKey); ok && val != "" {
				return val
			}
			return defaultVal
		}
		return os.Getenv(key)
	})
}

func LoadPipeline(path string) (*Pipeline, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	// Expand environment variables with default value support (${VAR:-default})
	expanded := expandEnvWithDefaults(string(data))

	var p Pipeline
	if err := yaml.Unmarshal([]byte(expanded), &p); err != nil {
		return nil, err
	}

	p.File = path

	// Resolve prompt_file references
	pipelineDir := filepath.Dir(path)
	for i, step := range p.Steps {
		if step.PromptFile != "" {
			promptPath := step.PromptFile
			if !filepath.IsAbs(promptPath) {
				promptPath = filepath.Join(pipelineDir, promptPath)
			}
			content, err := os.ReadFile(promptPath)
			if err != nil {
				return nil, fmt.Errorf("step %d (%s): cannot read prompt_file %q: %w", i+1, step.Name, step.PromptFile, err)
			}
			p.Steps[i].Prompt = expandEnvWithDefaults(string(content))
		}
	}

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
			return fmt.Errorf("step %d (%s): prompt or prompt_file is required", i+1, step.Name)
		}
	}
	
	return nil
}
