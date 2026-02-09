# Octos - LLM Agent Pipeline Orchestrator

![Octos](images/octos.png)

Minimalist orchestrator that executes YAML pipelines with CLI agents (kiro-cli, claude-code, etc.)

Automate multi-step LLM workflows with checkpoints, artifacts, and conditional execution.

## Installation

### Homebrew (macOS)

```bash
brew tap vicendominguez/tap
brew install octos
```

### Debian/Ubuntu

```bash
wget https://github.com/vicendominguez/octos/releases/download/v0.1.0/octos_0.1.0_amd64.deb
sudo dpkg -i octos_0.1.0_amd64.deb
```

### From source

```bash
go build -o octos
```

## Quick Start

```bash
# Run with interactive TUI
./octos pipeline.yaml

# Run in headless mode (CI/CD)
./octos --tui=false pipeline.yaml

# Resume from last checkpoint
./octos --resume pipeline.yaml

# Loop mode - run pipeline multiple times (Ralph loop pattern)
./octos --loop 3 pipeline.yaml        # Run 3 times
./octos --loop 0 pipeline.yaml        # Loop until Ctrl+C
```

## Loop Mode (Ralph Pattern)

Octos supports the **Ralph Wiggum loop** pattern - running your pipeline multiple times with fresh context each iteration, allowing the agent to iteratively refine its work.

**Use cases:**
- Iterative refinement: Agent fixes its own mistakes
- Complex tasks: Break work into multiple passes
- Self-healing: Retry failed steps in next iteration

**TUI controls:**
- Press `r` to restart pipeline after completion
- Loop counter shown in title bar
- Automatic loop limit enforcement

**For full Ralph loop implementation**, check out [Chief](https://github.com/MiniCodeMonkey/chief) - a dedicated tool for autonomous multi-iteration agent workflows.

## TUI Features

**Cyberpunk-themed dashboard** with real-time updates:
- 📋 **Steps Panel**: Visual progress with status indicators (⏳ → ✓ → ✗)
- 📺 **Output Panel**: Live streaming output from current step
- 📁 **File Changes**: Real-time tracking of modified/created/deleted files
- ⚡ **Stats Bar**: Elapsed time, steps/min speed, completion rate
- 🎯 **Status Bar**: Current time, working directory, git branch, pipeline status
- ⌨️ **Navigation**: Vim-style keys (j/k) to review completed steps

## Pipeline Format

### Basic Example

```yaml
agent:
  cmd: "kiro-cli"
  args: ["chat", "--no-interactive", "--trust-all-tools"]

context:
  role: "Senior backend engineer"
  rules:
    - "Don't modify tests"
    - "Keep changes minimal"

steps:
  - name: analyze
    prompt: "Analyze this codebase and list main issues"
  
  - name: fix
    prompt: |
      Fix the issues found in the analysis:
      {{analyze.output}}
      
      Follow these rules:
      {{context.rules}}
```

### Advanced Example with Artifacts & Conditions

```yaml
agent:
  cmd: "kiro-cli"
  args: ["chat", "--no-interactive", "--trust-all-tools"]

context:
  project: "E-commerce API"
  standards:
    - "Use TypeScript strict mode"
    - "Add JSDoc comments"
    - "Write unit tests for new functions"

steps:
  # Step 1: Analyze and save to artifact
  - name: analyze-structure
    prompt: |
      Analyze the project structure and identify:
      1. Main entry points
      2. Missing documentation
      3. Test coverage gaps
    save_to: analysis.txt

  # Step 2: Check if tests exist
  - name: check-tests
    prompt: "Do test files exist in this project? Answer yes or no."
    save_to: test-status.txt

  # Step 3: Conditional - only runs if no tests found
  - name: create-tests
    when: "{{check-tests.output}} contains no"
    load_from: analysis.txt
    prompt: |
      Based on this analysis:
      {{artifact.analysis}}
      
      Create unit tests following:
      {{context.standards}}

  # Step 4: Add documentation
  - name: add-docs
    load_from: analysis.txt
    prompt: |
      Add JSDoc comments to undocumented functions found in:
      {{artifact.analysis}}

  # Step 5: Final summary
  - name: summary
    prompt: |
      Summarize all changes made in this pipeline.
      Reference the initial analysis if needed.
```

### Real-World Example: Refactoring Pipeline

```yaml
agent:
  cmd: "kiro-cli"
  args: ["chat", "--no-interactive", "--trust-all-tools", "--wrap=always"]

context:
  role: "Code reviewer and refactoring expert"
  constraints:
    - "Maintain backward compatibility"
    - "Don't change public APIs"
    - "Add deprecation warnings for old code"

steps:
  - name: identify-code-smells
    prompt: |
      Scan the codebase for:
      - Duplicate code
      - Long functions (>50 lines)
      - Complex conditionals
      - Missing error handling
    save_to: code-smells.txt

  - name: check-test-coverage
    prompt: "What's the current test coverage? List untested modules."
    save_to: coverage-report.txt

  - name: prioritize-refactoring
    load_from: code-smells.txt
    prompt: |
      Based on these code smells:
      {{artifact.code-smells}}
      
      Create a prioritized refactoring plan (high/medium/low priority).
    save_to: refactor-plan.txt

  - name: refactor-high-priority
    load_from: refactor-plan.txt
    prompt: |
      Implement ONLY the high-priority refactorings from:
      {{artifact.refactor-plan}}
      
      Constraints:
      {{context.constraints}}

  - name: add-missing-tests
    when: "{{check-test-coverage.output}} contains untested"
    load_from: coverage-report.txt
    prompt: |
      Add tests for untested modules mentioned in:
      {{artifact.coverage-report}}

  - name: verify-changes
    prompt: |
      Review all changes made and verify:
      1. No breaking changes
      2. All tests pass
      3. Code is cleaner than before
```

## Features

### 🔗 Variable Interpolation

Access data from previous steps and context:

```yaml
{{stepname.output}}           # Output from a previous step
{{context.role}}              # Global context values
{{context.rules}}             # Lists from context
{{artifact.filename}}         # Loaded artifact content
```

### 💾 Artifacts

Save and reuse outputs to reduce context size:

```yaml
steps:
  - name: analyze
    prompt: "Analyze the code"
    save_to: analysis.txt       # Saves to .octos/artifacts/analysis.txt
  
  - name: implement
    load_from: analysis.txt     # Loads artifact as {{artifact.analysis}}
    prompt: "Implement based on {{artifact.analysis}}"
```

**Benefits:**
- Reduce token usage by not accumulating all outputs
- Reuse artifacts across different pipelines
- Persist important outputs for later review

### ⚡ Conditional Execution

Skip steps based on previous outputs:

```yaml
steps:
  - name: check-deps
    prompt: "Are dependencies up to date?"
  
  - name: update-deps
    when: "{{check-deps.output}} contains outdated"
    prompt: "Update outdated dependencies"
  
  - name: verify-security
    when: "{{check-deps.output}} not_empty"
    prompt: "Run security audit"
```

**Supported conditions:**
- `contains text` - Case-insensitive substring match
- `equals value` - Exact match
- `not_empty` - Output is not empty

### 🔀 Per-Step Agent Override

Use different agents for different steps to optimize costs or capabilities:

```yaml
agent:
  cmd: "kiro-cli"
  args: ["chat", "--no-interactive", "--model", "haiku"]  # Default: cheap model

steps:
  - name: analyze
    prompt: "List all functions and their complexity"
    # Uses default agent (haiku - cheap)
  
  - name: complex-refactor
    agent:
      cmd: "kiro-cli"
      args: ["chat", "--no-interactive", "--model", "sonnet"]
    prompt: "Refactor the authentication system with best practices"
    # Uses sonnet only for this step (expensive but powerful)
  
  - name: add-tests
    prompt: "Add unit tests for the refactored code"
    # Back to default agent (haiku - cheap)
  
  - name: review
    agent:
      cmd: "claude-code"
      args: ["--no-interactive"]
    prompt: "Review all changes and suggest improvements"
    # Uses different CLI tool entirely
```

**Benefits:**
- Optimize costs by using cheaper models for simple tasks
- Use powerful models only when needed
- Mix different CLI agents in the same pipeline
- Each step can have its own agent configuration

### 🔄 Resume & Checkpoints

Automatically saves state after each successful step:

```bash
# Pipeline fails at step 3
./octos pipeline.yaml

# Resume from step 3 (skips completed steps 1-2)
./octos --resume pipeline.yaml

# Start fresh (clear saved state)
./octos --clean pipeline.yaml
```

State files stored in `.octos/state/`

### 📁 File Change Tracking

Real-time detection of file modifications:
- `+` New files created
- `M` Files modified
- `-` Files deleted

Automatically tracks changes during each step execution.

## CLI Options

```bash
./octos [options] <pipeline.yaml>

Options:
  --version          Show version
  --tui=false        Disable TUI (headless mode)
  --resume           Resume from last checkpoint
  --clean            Clear saved state before running
```

## Examples

```bash
# Interactive mode with TUI
./octos example.yaml

# Artifacts and conditional execution
./octos example-artifacts.yaml

# Headless mode for CI/CD
./octos --tui=false pipeline.yaml

# Resume failed pipeline
./octos --resume pipeline.yaml
```

## Directory Structure

```
.octos/
├── state/              # Checkpoint files
│   └── pipeline.yaml.json
└── artifacts/          # Saved outputs
    ├── analysis.txt
    └── plan.txt
```

## Tips for LLMs

When creating pipelines:

1. **Break down complex tasks** into small, focused steps
2. **Use artifacts** to reduce context accumulation
3. **Add conditions** to skip unnecessary steps
4. **Save important outputs** for reuse across pipelines
5. **Use descriptive step names** for better navigation
6. **Reference context** to maintain consistency across steps

Example pattern:
```yaml
steps:
  - name: analyze      # Understand the problem
  - name: plan         # Create solution approach
  - name: implement    # Execute the plan
  - name: verify       # Check the results
```
