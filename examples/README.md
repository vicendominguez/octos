# Octos Pipeline Examples

## Available Examples

### 1. `example.yaml` - Basic Pipeline
Simple pipeline demonstrating core features:
- Sequential step execution
- Variable interpolation from previous steps
- Context usage

```bash
octos examples/example.yaml
```

### 2. `example-artifacts.yaml` - Artifacts & Conditionals
Advanced pipeline showcasing:
- Saving outputs to artifacts (`save_to`)
- Loading artifacts in later steps (`load_from`)
- Conditional execution (`when`)
- Reducing context accumulation

```bash
octos examples/example-artifacts.yaml
```

### 3. `example-parallel.yaml` - Parallel Execution & Multi-Artifact Loading
Demonstrates the dependency graph features:
- **Parallel execution** via `needs` (steps without shared deps run simultaneously)
- **Multi-artifact loading** (`load_from` as list)
- DAG visualization with `g` key in TUI
- Conditional execution based on parallel outputs

```
Dependency graph:
  analyze-code ──┐
                 ├── plan ── implement ── verify
  analyze-tests ─┘
```

```bash
octos examples/example-parallel.yaml
```

### 4. `example-prompt-file.yaml` - External Prompt Files
Mixing inline prompts with external markdown files:
- Long prompts loaded from `prompts/` directory
- Short prompts stay inline
- Environment variables expanded in external files

```bash
octos examples/example-prompt-file.yaml
```

### 5. `cost-optimization.yaml` - Mixed Agent Models
Cost-optimized pipeline demonstrating:
- Default cheap model (Haiku) for simple tasks
- Expensive model (Sonnet) only for complex steps
- Strategic model selection per step

```bash
octos examples/cost-optimization.yaml
```

### 6. `goose-stdin.yaml` - Goose with Stdin Prompt Delivery
Pipeline using `stdin: true` for agents that read prompts from stdin:
- Avoids "argument list too long" errors
- Uses `goose run -i -` which reads from stdin

```bash
octos examples/goose-stdin.yaml
```

### 7. `gitea-issue-to-mr.yaml` - Automated Issue → Merge Request
Full issue resolution pipeline with parallel discovery:
- **Parallel phase**: picks issue + scans codebase simultaneously
- **Multi-artifact loading**: plan loads both issue and codebase map
- Retry on implementation failures
- Labels as state machine: (none) → in-progress → in-review
- Designed for ralph loop: `octos --loop N`

```bash
GITEA_URL=http://gitea:3000 GITEA_TOKEN=xxx octos examples/gitea-issue-to-mr.yaml
```

### 8. `code-review-to-gitea-issues.yaml` - Multi-Repo Code Review
Automated code review that creates labeled Gitea issues:
- **Parallel setup**: clone repos + create labels simultaneously
- **Multi-artifact dedup**: loads findings + existing issues to avoid duplicates
- Follows KISS/YAGNI principles

```bash
GITEA_URL=http://gitea:3000 GITEA_TOKEN=xxx REPOS=project-a,project-b octos examples/code-review-to-gitea-issues.yaml
```

## Key Features Demonstrated

| Feature | Example |
|---------|---------|
| Sequential steps | `example.yaml` |
| Artifacts & conditions | `example-artifacts.yaml` |
| **Parallel execution (`needs`)** | `example-parallel.yaml`, `gitea-issue-to-mr.yaml` |
| **Multi `load_from`** | `example-parallel.yaml`, `code-review-to-gitea-issues.yaml` |
| Per-step agent override | `cost-optimization.yaml` |
| External prompt files | `example-prompt-file.yaml` |
| Stdin prompt delivery | `goose-stdin.yaml` |
| Retry & failure handling | `gitea-issue-to-mr.yaml` |
| Environment variables | `code-review-to-gitea-issues.yaml` |

## Running Examples

```bash
# Interactive TUI (press g for dependency graph)
octos examples/example-parallel.yaml

# Headless mode (CI/CD)
octos --tui=false examples/example.yaml

# Validate without running
octos --dry-run examples/gitea-issue-to-mr.yaml

# Resume from checkpoint
octos --resume examples/cost-optimization.yaml

# Loop mode (autonomous iteration)
octos --loop 5 examples/gitea-issue-to-mr.yaml
```

## Creating Your Own Pipeline

Start simple and layer features:

1. `steps:` with sequential execution
2. `save_to` / `load_from` for artifact passing (less tokens than `{{step.output}}`)
3. `when` for conditional execution
4. `needs` for parallel execution where steps are independent
5. `load_from: [a.txt, b.txt]` when a step needs multiple artifacts
6. `on_failure: retry` for flaky steps
7. `--dry-run` to validate before running
