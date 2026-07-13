# Research: Estado actual de Octos

## Arquitectura

### Ficheros principales

| Fichero | Responsabilidad | LOC aprox |
|---------|----------------|-----------|
| `main.go` | CLI entry point: parsea flags (`--tui`, `--resume`, `--clean`, `--loop`, `--version`) y arranca TUI o headless | 60 |
| `pipeline.go` | Carga YAML, expande env vars, valida estructura, resuelve `prompt_file` | 100 |
| `executor.go` | Motor de ejecución: interpolación, condiciones `when`, artefactos, invocación de agentes, file tracking | 440 |
| `tui.go` | Modelo Bubble Tea: UI cyberpunk con panels, streaming, controles | 750 |
| `styles.go` | Estilos lipgloss, constantes de layout | 120 |
| `state.go` | Checkpoints: save/load/clear estado JSON | 65 |
| `version.go` | Constante de versión | 3 |

### Flujo de ejecución

```
main.go
  └─ LoadPipeline()          [pipeline.go]
  └─ useTUI?
       ├─ TUI: NewTUIModel → tea.Run → callbacks → RunPipelineWithCallbacks
       └─ Headless: RunPipelineWithResume → RunPipelineWithCallbacks
                                              └─ for each step:
                                                   evaluateCondition()
                                                   loadArtifact()
                                                   interpolate()
                                                   buildPrompt()
                                                   runAgent() / runAgentWithStreaming()
                                                   detectFileChanges()
                                                   saveArtifact()
                                                   SaveState()
```

### CLI actual

```
octos [flags] <pipeline.yaml>

Flags:
  --tui       bool   (default: true)
  --version   bool
  --resume    bool
  --clean     bool
  --loop      int    (0 = no loop)
```

No hay subcomandos. Todo son flags del binario principal. El parsing usa `flag` estándar de Go.

### Sistema de checkpoints (state.go)

- Guarda en `.octos/state/<pipeline>.yaml.json` después de cada step OK.
- Contiene: `last_completed_step`, `outputs` (mapa step→output), timestamps.
- `--resume` carga el state y salta los steps completados.
- `--clean` borra el state file.
- Al completar el pipeline, se borra automáticamente.

### Interpolación y errores (executor.go)

**Puntos de fallo identificados:**

| Punto | Código actual | Mensaje actual |
|-------|---------------|----------------|
| YAML inválido | `yaml.Unmarshal` en `LoadPipeline` | Error genérico de yaml.v3 (no indica línea del pipeline) |
| `agent.cmd` vacío | `Validate()` | `"agent.cmd is required"` |
| Sin steps | `Validate()` | `"at least one step is required"` |
| Step sin nombre | `Validate()` | `"step %d: name is required"` |
| Step sin prompt | `Validate()` | `"step %d (%s): prompt or prompt_file is required"` |
| `prompt_file` no existe | `LoadPipeline` | `"step %d (%s): cannot read prompt_file %q: %w"` |
| Artefacto no encontrado | `loadArtifact` en loop | Warning a stderr, NO falla (continúa sin el artefacto) |
| Placeholder no resuelto | `interpolate()` | Warning `"⚠ unresolved placeholder: {{xxx}}"` — NO falla, pasa literal |
| Agent falla (exit ≠ 0) | `runAgent` | `"step %s failed: %w"` con el stderr del proceso |
| `when` mal formada | `evaluateCondition` | NO falla — evalúa a `true` silenciosamente si no matchea ningún patrón |

**Problemas identificados:**
1. Los placeholders no resueltos son solo warnings, no errores fatales → el agente recibe `{{xxx}}` literal.
2. Los artefactos no encontrados son warnings → pasos posteriores con `{{artifact.x}}` reciben cadena vacía.
3. Una condición `when` con sintaxis incorrecta evalúa a `true` → el paso se ejecuta cuando no debería.
4. No hay información del step actual en muchos errores (por ejemplo, errores de YAML).

### Condiciones `when` soportadas

- `contains text` — case-insensitive substring
- `equals value` — exact match  
- `not_empty` — output no vacío

No hay validación de sintaxis: cualquier otra cadena evalúa `true`.

### Agentes: cómo se invocan

```go
// Sin stdin (default)
args := append(agent.Args, prompt)
cmd := exec.Command(agent.Cmd, args...)

// Con stdin (nuevo en v0.10.0)
cmd := exec.Command(agent.Cmd, agent.Args...)
cmd.Stdin = strings.NewReader(prompt)
```

El prompt incluye siempre el contexto global prepended (`buildPrompt`). No hay forma de desactivar eso.

### TUI: interacción con ejecución

- La TUI lanza `RunPipelineWithCallbacks` en una goroutine.
- Recibe eventos via callbacks: `onStart`, `onStream`, `onComplete`, `onFileChanges`.
- Los callbacks envían `tea.Msg` al programa para actualizar la UI.
- La TUI tiene: panel de steps, panel de output (streaming), panel de file changes, barra de stats.
- No tiene concepto de "verbosidad" — siempre muestra todo.

### Modo headless (--tui=false)

- Usa `RunPipelineWithResume` sin callbacks → `silent = false`.
- Imprime directamente a stdout: `→ Running step`, `✓ Step completed`, warnings a stderr.
- No tiene opción de suprimir output ni de formatear como JSON.
