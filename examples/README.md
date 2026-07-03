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

### 5. `example-prompt-file.yaml` - External Prompt Files
Demonstrates mixing inline prompts with external markdown files:
- Long prompts loaded from `prompts/` directory
- Short prompts stay inline
- Environment variables expanded in external files

**Use case:** Complex pipelines with large, reusable prompts

```bash
./octos examples/example-prompt-file.yaml
```

### 6. `gitea-issue-to-mr.yaml` - Automated Issue → Merge Request
Full issue resolution pipeline with ralph loop support:
- Picks oldest open issue (or resumes in-progress)
- Reads issue + all comments for context
- Analyzes, plans, implements, validates
- Creates merge request on success
- Labels as state machine: (none) → in-progress → in-review

**Use case:** Autonomous issue resolution with `octos --loop N`

```bash
GITEA_URL=http://your-gitea:3000 GITEA_TOKEN=xxx octos examples/gitea-issue-to-mr.yaml
```

### 7. `goose-stdin.yaml` - Goose with Stdin Prompt Delivery
Pipeline demonstrating `stdin: true` for agents that read prompts from stdin:
- Avoids "argument list too long" errors with large interpolated prompts
- Uses `goose run -i -` which reads instructions from stdin
- Required for agents with CLI argument length limitations

**Use case:** Goose pipelines with large artifacts/context

```bash
./octos examples/goose-stdin.yaml
```

## Running Examples

```bash
# Interactive mode with TUI
./octos examples/example.yaml

# Headless mode (CI/CD)
./octos --tui=false examples/example.yaml

# Resume from checkpoint
./octos --resume examples/example.yaml

# Validate without running
./octos --dry-run examples/example.yaml
```

## Creating Your Own Pipeline

Start with `example.yaml` and gradually add:
1. Artifacts for large outputs
2. Conditionals for decision points
3. Context for shared configuration
4. `on_failure: retry` for flaky steps
5. `--dry-run` to validate before running
6. Multiple steps for complex workflows
