
orquestador de prompts que:

Use un agente externo ya existente (opencode, kiro-cli, claude-code, etc.)

Le pase prompts secuenciales

Mantenga contexto común + premisas

Ejecute un pipeline determinista de pasos

Básicamente: “scriptear” al agente como si fuera un worker inteligente

Eso no existe “tal cual” como producto estándar, pero es muy fácil de construir en Rust o Go, porque estos agentes:

leen de stdin

escriben a stdout

aceptan flags o archivos de contexto

🧠 Modelo mental correcto
Piensa en esto como:

Un workflow engine que controla un CLI agent

NO como LangChain
NO como AutoGPT
SÍ como Makefile + contexto acumulado + prompts

🧱 Arquitectura mínima
pipeline.yaml
orchestrator (rust/go)
        ↓
[step 1 prompt] ─▶ agent
        ↓
[step 2 prompt + output step1]
        ↓
[step 3 prompt + contexto común]
Contexto compartido
Un objeto mutable que acumula:

premisas globales

outputs anteriores

decisiones del agente

📄 Ejemplo de definición de pipeline (YAML)
agent:
  cmd: "claude-code"
  args: ["--model", "sonnet"]

context:
  role: "Senior backend engineer"
  rules:
    - "No modificar tests"
    - "Preferir cambios pequeños"

steps:
  - name: analyze
    prompt: |
      Analiza este repo y describe los problemas principales.

  - name: plan
    prompt: |
      Usando el análisis anterior:
      {{analyze.output}}
      crea un plan de refactorización.

  - name: implement
    prompt: |
      Implementa el plan manteniendo estas reglas:
      {{context.rules}}


cmd := exec.Command("kiro-cli")
stdin, _ := cmd.StdinPipe()
stdout, _ := cmd.StdoutPipe()

cmd.Start()
stdin.Write([]byte(prompt))
stdin.Close()

out, _ := io.ReadAll(stdout)
cmd.Wait()

🧠 Detalles importantes (experiencia real)
1. El contexto NO debe crecer infinito
Usa:

resúmenes

extracción de decisiones

compresión entre pasos

2. Usa delimitadores duros
Siempre:

=== CONTEXTO GLOBAL ===
=== OUTPUT PASO ANTERIOR ===
=== NUEVA TAREA ===
Los agentes CLI responden mejor así.

3. El orquestador manda
El agente:

NO decide el flujo

NO crea nuevos pasos

SOLO ejecuta lo que tú defines

Esto es mucho más estable que AutoGPT-style.

📌 Conclusión clara
Lo que buscas es:

✔️ Un prompt pipeline runner
✔️ Agent-agnostic
✔️ Determinista
✔️ Hecho en Rust o Go
❌ NO un framework de LLMs

