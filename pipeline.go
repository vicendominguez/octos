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
	Cmd   string   `yaml:"cmd"`
	Args  []string `yaml:"args"`
	Stdin bool     `yaml:"stdin,omitempty"`
}

type Step struct {
	Name       string       `yaml:"name"`
	Prompt     string       `yaml:"prompt"`
	PromptFile string       `yaml:"prompt_file,omitempty"`
	SaveTo     string       `yaml:"save_to"`
	LoadFrom   string       `yaml:"load_from"`
	When       string       `yaml:"when"`
	Agent      *AgentConfig `yaml:"agent,omitempty"`
	OnFailure  string       `yaml:"on_failure,omitempty"`
	MaxRetries int          `yaml:"max_retries,omitempty"`
	Needs      []string     `yaml:"needs,omitempty"`
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
		return nil, fmt.Errorf("error: invalid YAML in %s: %w\n  hint: check indentation and quoting around the reported line", path, err)
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
				return nil, fmt.Errorf("error: step '%s' references prompt_file '%s' which does not exist\n  hint: path is relative to pipeline YAML location (%s)", step.Name, step.PromptFile, pipelineDir)
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
	if err := os.MkdirAll(artifactsDir, 0o755); err != nil {
		return nil, err
	}

	return &p, nil
}

// Validate checks if the pipeline configuration is valid
func (p *Pipeline) Validate() error {
	if p.Agent.Cmd == "" {
		return fmt.Errorf("error: agent.cmd is required\n  hint: add 'cmd: <agent-binary>' under 'agent:'")
	}
	
	if len(p.Steps) == 0 {
		return fmt.Errorf("error: at least one step is required\n  hint: add a 'steps:' section with at least one '- name:' entry")
	}

	stepNames := make(map[string]int) // name → position index
	
	for i, step := range p.Steps {
		if step.Name == "" {
			return fmt.Errorf("error: step %d has no name\n  hint: every step needs a unique 'name:' field", i+1)
		}
		if _, exists := stepNames[step.Name]; exists {
			return fmt.Errorf("error: duplicate step name '%s'\n  hint: each step must have a unique name", step.Name)
		}
		if step.Prompt == "" {
			return fmt.Errorf("error: step '%s' has no prompt\n  hint: add 'prompt:' or 'prompt_file:' to the step", step.Name)
		}
		if step.OnFailure != "" && step.OnFailure != "retry" && step.OnFailure != "skip" && step.OnFailure != "fail_fast" {
			return fmt.Errorf("error: step '%s' has invalid on_failure value '%s'\n  hint: must be 'retry', 'skip', or 'fail_fast'", step.Name, step.OnFailure)
		}
		if step.MaxRetries > 0 && step.OnFailure != "retry" {
			return fmt.Errorf("error: step '%s' has max_retries without on_failure: retry\n  hint: add 'on_failure: retry' or remove 'max_retries'", step.Name)
		}

		// Validate needs references
		for _, dep := range step.Needs {
			depIdx, exists := stepNames[dep]
			if !exists {
				return fmt.Errorf("error: step '%s' needs '%s' which is not defined before it\n  hint: referenced steps must appear earlier in the pipeline", step.Name, dep)
			}
			_ = depIdx
		}

		stepNames[step.Name] = i
	}

	// Detect cycles (only if any step uses needs)
	if p.hasNeeds() {
		if err := p.detectCycles(stepNames); err != nil {
			return err
		}
	}
	
	return nil
}

// hasNeeds returns true if any step in the pipeline declares needs
func (p *Pipeline) hasNeeds() bool {
	for _, step := range p.Steps {
		if len(step.Needs) > 0 {
			return true
		}
	}
	return false
}

// detectCycles checks for dependency cycles using DFS
func (p *Pipeline) detectCycles(stepNames map[string]int) error {
	// Build adjacency list
	adj := make(map[string][]string)
	for _, step := range p.Steps {
		adj[step.Name] = step.Needs
	}

	const (
		white = 0 // unvisited
		gray  = 1 // in current path
		black = 2 // fully processed
	)

	color := make(map[string]int)
	for _, step := range p.Steps {
		color[step.Name] = white
	}

	var visit func(name string) error
	visit = func(name string) error {
		color[name] = gray
		for _, dep := range adj[name] {
			switch color[dep] {
			case gray:
				return fmt.Errorf("error: dependency cycle detected: '%s' → '%s'\n  hint: remove the circular dependency", name, dep)
			case white:
				if err := visit(dep); err != nil {
					return err
				}
			}
		}
		color[name] = black
		return nil
	}

	for _, step := range p.Steps {
		if color[step.Name] == white {
			if err := visit(step.Name); err != nil {
				return err
			}
		}
	}

	return nil
}
