# PRD: Mejoras de caso práctico y usabilidad para Octos

## 1. Validación en seco: `--dry-run` vs `octos plan`

### Análisis

| Criterio | Opción A: `--dry-run` | Opción B: `octos plan` |
|----------|----------------------|------------------------|
| Consistencia con CLI actual | ✅ Todo son flags hoy (`--tui`, `--resume`, `--clean`, `--loop`) | ❌ Introduce subcomandos; rompe el patrón |
| Descubribilidad | Media — requiere `--help` | Alta — aparece como verbo en el uso |
| Complejidad de implementación | Baja — un flag más, un `if` en `main.go` | Media — requiere reestructurar el parsing CLI (subcommands o detectar primer arg) |
| Futuro | Si luego añades `octos init`, `octos list`, etc., el flag queda inconsistente | Escala natural a más subcomandos |
| KISS/YAGNI | ✅ Mínimo cambio, resuelve el problema | ❌ Infraestructura para un futuro incierto |

### Recomendación: Opción A — `--dry-run`

**Razones:**
1. Octos es un binario single-purpose. No hay señales de necesitar subcomandos a corto plazo.
2. No hay `init`, `list`, `validate` ni nada que justifique un router de subcomandos.
3. El flag encaja perfectamente con el estilo actual: `octos --dry-run pipeline.yaml`.
4. Si en el futuro necesitas subcomandos, la migración es trivial (deprecar flag → añadir subcommand).

### Comportamiento propuesto

```
$ octos --dry-run pipeline.yaml

Pipeline: pipeline.yaml (7 steps)
Agent: goose [run, -i, -] (stdin: true)

Steps:
  1. ● pick-issue
  2. ● analyze-issue        when: "{{pick-issue.output}} not_empty"
  3. ● plan
  4. ● implement
  5. ● validate
  6. ○ update-docs          when: "{{validate.output}} contains VALIDATED"  [skipped: cannot evaluate]
  7. ○ create-mr            when: "{{validate.output}} contains VALIDATED"  [skipped: cannot evaluate]

Interpolation check:
  ✓ All context variables resolved
  ⚠ {{pick-issue.output}} — will be resolved at runtime (step dependency)
  ⚠ {{artifact.current-issue}} — requires pick-issue.save_to

Env vars:
  ✓ GITEA_URL = http://pluto:3000
  ✓ GITEA_TOKEN = [set]

No errors found. Pipeline is valid.
```

**Reglas:**
- Resuelve env vars y `context.*` en tiempo de dry-run.
- Los `{{step.output}}` y `{{artifact.*}}` se marcan como "runtime dependency" (no se pueden resolver sin ejecutar).
- Las condiciones `when` que dependen de outputs no resueltos se marcan como "cannot evaluate".
- Si hay un error real (env var vacía usada en context, `prompt_file` que no existe, `load_from` referenciando fichero inexistente), falla con exit code 1.

---

## 2. Gestión de fallos por paso (`on_failure`)

### Diseño

```yaml
steps:
  - name: analyze
    prompt: "..."
    on_failure: retry      # retry | skip | fail_fast (default)
    max_retries: 2         # solo con retry
```

### Interacción con checkpoints/resume

- **`fail_fast` (default):** Comportamiento actual — guarda state antes de fallar, `--resume` retoma desde ese step.
- **`retry`:** Se reintenta `max_retries` veces antes de fallar definitivamente. El state no se guarda hasta que el step tenga éxito o se agoten los reintentos.
- **`skip`:** El step se marca como skipped, el pipeline continúa. Se guarda en state como completed (para que `--resume` no lo reintente).

### ¿Inyectar error en retry?

**Recomendación: Sí, inyectar el error del intento anterior.**

El prompt del reintento incluiría un prefijo:

```
[RETRY 2/3] Previous attempt failed with: <error message>

<prompt original>
```

Esto da al agente contexto para autocorregirse. Es lo que haría un humano.

### ¿Qué pasa con artefactos de un step skipped?

Si un step con `on_failure: skip` no genera su artefacto:
1. El `load_from` de pasos posteriores ya maneja esto como warning (no falla).
2. Los `{{artifact.x}}` se resolverán como cadena vacía.
3. **Mejora propuesta:** Si un step posterior tiene `load_from: X` y X viene de un step con `skip`, emitir warning explícito: `"⚠ step 'Y' loads artifact from 'X' which was skipped"`.

### Campos nuevos en Step

```go
type Step struct {
    // ... existentes ...
    OnFailure  string `yaml:"on_failure,omitempty"`  // retry | skip | fail_fast
    MaxRetries int    `yaml:"max_retries,omitempty"` // default: 1 si on_failure=retry
}
```

### Validación

- `on_failure` solo acepta: `retry`, `skip`, `fail_fast` (o vacío = fail_fast).
- `max_retries` > 0 solo si `on_failure: retry`.
- `max_retries` sin `on_failure: retry` → warning en validación.

---

## 3. Mensajes de error más claros

### Inventario de puntos de fallo y propuesta de formato

| # | Punto de fallo | Mensaje actual | Propuesta |
|---|----------------|----------------|-----------|
| 1 | YAML parse error | Error crudo de yaml.v3 | `"error: invalid YAML in pipeline.yaml: <detalle>\n  hint: check indentation and quoting"` |
| 2 | agent.cmd vacío | `"agent.cmd is required"` | `"error: agent.cmd is required in pipeline.yaml\n  hint: add 'cmd: <agent-binary>' under 'agent:'"` |
| 3 | Step sin nombre | `"step %d: name is required"` | OK, es claro. Añadir hint: `"hint: every step needs a unique 'name:' field"` |
| 4 | Step sin prompt | `"step %d (%s): prompt or prompt_file is required"` | OK. |
| 5 | prompt_file no existe | `"cannot read prompt_file %q: %w"` | `"error: step '%s' references prompt_file '%s' which does not exist\n  hint: path is relative to pipeline YAML location (%s)"` |
| 6 | Artefacto no encontrado en load_from | Warning a stderr, continúa | `"warning: step '%s' cannot load artifact '%s' (file not found in .octos/artifacts/)\n  hint: was the producing step skipped or is this the first run?"` |
| 7 | Placeholder no resuelto | `"⚠ unresolved placeholder: {{xxx}}"` | `"warning: step '%s' has unresolved placeholder {{xxx}}\n  hint: check spelling or ensure the referenced step runs before this one"` |
| 8 | Agent falla (exit ≠ 0) | `"step %s failed: %w"` | `"error: step '%s' failed (exit code %d, duration %s)\n  agent: %s %v\n  stderr: <últimas 5 líneas>"` |
| 9 | Condición when inválida | Evalúa true silenciosamente | `"warning: step '%s' has unparseable 'when' condition: '%s'\n  hint: supported: 'X contains Y', 'X equals Y', 'X not_empty'"` |
| 10 | Env var no definida usada en context | Se expande a "" silenciosamente | Esto ya se gestiona con `${VAR:-default}`. No cambiar — es intencional. |

### Formato estándar

```
<level>: <qué falló> in step '<step_name>'
  hint: <sugerencia de arreglo>
```

Levels: `error` (fatal), `warning` (continúa).

---

## 4. Quick start del README

### Propuesta

Añadir al inicio del README, justo después de la descripción y antes de "Installation":

```markdown
## 30-Second Quick Start

```yaml
# hello.yaml
agent:
  cmd: "echo"
  args: ["-e"]

steps:
  - name: greet
    prompt: "Hello from Octos!"
```

```bash
brew install vicendominguez/tap/octos
octos hello.yaml
```

That's it. Replace `echo` with any CLI agent (kiro-cli, goose, claude...) and you have a real pipeline.
```

**Razones:**
- Usa `echo` como agente → funciona sin instalar nada más.
- Muestra el formato mínimo del YAML.
- 7 líneas de YAML + 2 de CLI = resultado inmediato.
- Tras este bloque, enlazar a "see full examples below" para el detalle.

---

## 5. Análisis de verbosidad (NO implementar)

### Combinaciones posibles

| Modo | `--tui` | Verbosidad | Tiene sentido | Notas |
|------|---------|------------|---------------|-------|
| TUI + normal | true | default | ✅ Actual | Dashboard completo |
| TUI + quiet | true | `-q` | ❌ | No tiene sentido suprimir output en una UI visual |
| Headless + normal | false | default | ✅ Actual | Imprime progress a stdout |
| Headless + quiet | false | `-q` | ✅ | Solo output final del último step (o JSON) |
| Headless + silent | false | `-s` | 🤔 | Sin ningún output — solo exit code |

### Recomendación

- **Solo implementar `-q`/`--quiet` en modo headless** (`--tui=false`).
- En TUI no tiene sentido (¿para qué lanzas la TUI si no quieres ver nada?).
- Comportamiento de `--quiet`: suprimir los mensajes de progreso (`→ Running`, `✓ Completed`, warnings). Solo imprimir el output del último step + exit code.
- **No hacer JSON output todavía** — es una feature separada que añade complejidad sin caso de uso claro hoy.

### Complejidad estimada

- `main.go`: añadir flag `--quiet`, pasarlo al executor.
- `executor.go`: respetar `quiet` en los `fmt.Printf` del modo headless (ya existe `silent` basado en callbacks — se puede reutilizar).
- `tui.go`: no tocar.
- Total: ~15 líneas. Complejidad muy baja.

---

## 6. README agent-agnostic

### Propuesta

No es una sección separada. Integrar el concepto agent-agnostic en la **descripción principal** del README (primeras líneas). Quien lea la intro ya debe entender la ventaja diferencial sin inferirla de los ejemplos.

**Descripción actual:**
> Minimalist orchestrator that executes YAML pipelines with CLI agents (kiro-cli, claude-code, etc.)

**Propuesta de descripción:**
> Minimalist orchestrator that executes YAML pipelines with any CLI agent. No adapters, no plugins — if it accepts a prompt and returns text to stdout, it works. Swap agents per-step, mix providers in the same pipeline, or use a shell script as your "agent".

Complementar en la sección "Pipeline Format" con un comentario inline que lo refuerce:

```yaml
# Works with any CLI that takes a prompt:
agent:
  cmd: "goose"           # or kiro-cli, claude, ./my-script.sh...
  args: ["run", "-i", "-"]
  stdin: true            # pass prompt via stdin instead of CLI arg
```

---

## TODO de implementación (orden de commits)

### Commit 1: docs — improve README intro, quick start, and agent-agnostic messaging
- Reescribir la descripción principal para dejar claro que es agent-agnostic.
- Añadir Quick Start section al inicio del README (antes del detalle).
- Sin cambios de código.

### Commit 2: feat — add `--dry-run` flag
- Añadir flag `--dry-run` a `main.go`.
- Crear función `DryRun(pipeline)` en nuevo fichero `dryrun.go`.
- Resuelve env vars y context, marca runtime dependencies.
- Tests: `dryrun_test.go`.
- Bump version a 0.11.0.

### Commit 3: feat — add `on_failure` field to steps
- Añadir campos `OnFailure` y `MaxRetries` a `Step` en `pipeline.go`.
- Validar valores permitidos en `Validate()`.
- Implementar lógica en `RunPipelineWithCallbacks` (retry loop, skip logic).
- Inyectar error del intento anterior en retry prompt.
- Tests: `on_failure_test.go`.

### Commit 4: feat — improve error messages
- Añadir nombre del step a todos los mensajes de error/warning.
- Añadir hints a errores de validación.
- Warning para condiciones `when` no parseables (en vez de evaluar `true`).
- Incluir exit code y stderr tail en fallos de agente.
- Tests: actualizar tests existentes que comprueben mensajes.

### Commit 5: docs — add verbosity analysis to docs (no code)
- Crear `docs/verbosity-analysis.md` con el análisis del punto 5.
- Referencia para futura implementación si se decide.

### Versiones
- Commit 1: no bump (solo docs)
- Commit 2: 0.11.0 (nueva feature)
- Commit 3: 0.12.0 (nueva feature)
- Commit 4: 0.12.1 (mejora UX, no rompe API)
- Commit 5: no bump (solo docs)
