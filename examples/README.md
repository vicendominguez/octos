# Octos Pipeline Examples

## Available Examples

### 1. `example.yaml` - Basic Pipeline
Simple pipeline demonstrating core features:
- Sequential step execution
- Variable interpolation from previous steps
- Context usage

**Use case:** Quick start, learning the basics

```bash
./octos examples/example.yaml
```

### 2. `example-artifacts.yaml` - Artifacts & Conditionals
Advanced pipeline showcasing:
- Saving outputs to artifacts (`save_to`)
- Loading artifacts in later steps (`load_from`)
- Conditional execution (`when`)
- Reducing context accumulation

**Use case:** Complex workflows with decision points

```bash
./octos examples/example-artifacts.yaml
```

### 3. `cost-optimization.yaml` - Mixed Agent Models
Cost-optimized pipeline demonstrating:
- Default cheap model (Haiku) for simple tasks
- Expensive model (Sonnet) only for complex steps
- Strategic model selection per step
- Maximizing value while minimizing costs

**Use case:** Production pipelines where cost matters

```bash
./octos examples/cost-optimization.yaml
```

### 4. `code-review-to-gitea-issues.yaml` - Automated Code Review → Gitea Issues
Multi-repo code review pipeline that:
- Clones multiple repos from Gitea
- Analyzes code for updates, improvements, best practices, and concerns
- Creates labeled issues in Gitea automatically
- Follows KISS/YAGNI principles

**Use case:** Automated tech debt detection and issue tracking

```bash
./octos examples/code-review-to-gitea-issues.yaml
```

## Running Examples

```bash
# Interactive mode with TUI
./octos examples/example.yaml

# Headless mode (CI/CD)
./octos -no-tui examples/example.yaml

# Resume from checkpoint
./octos --resume examples/example.yaml
```

## Creating Your Own Pipeline

Start with `example.yaml` and gradually add:
1. Artifacts for large outputs
2. Conditionals for decision points
3. Context for shared configuration
4. Multiple steps for complex workflows
