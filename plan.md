# Plan de Ejecución - Orquestador de Prompts para Agentes LLM

## Fase 1: Diseño y Arquitectura
- [ ] Definir estructura del proyecto Go (cmd/, internal/, pkg/)
- [ ] Diseñar esquema YAML para definición de pipelines
- [ ] Especificar formato de contexto compartido entre pasos
- [ ] Definir interfaz para comunicación con agentes CLI externos
- [ ] Diseñar sistema de delimitadores para contexto (CONTEXTO GLOBAL, OUTPUT PASO ANTERIOR, NUEVA TAREA)

## Fase 2: Implementación Core
- [ ] Crear parser YAML para configuración de pipelines
- [ ] Implementar gestor de contexto mutable con acumulación de outputs
- [ ] Desarrollar executor de comandos CLI (stdin/stdout)
- [ ] Implementar sistema de paso de prompts secuenciales
- [ ] Crear mecanismo de interpolación de variables ({{analyze.output}}, {{context.rules}})

## Fase 3: Gestión de Contexto
- [ ] Implementar sistema de resúmenes para evitar crecimiento infinito
- [ ] Crear extractor de decisiones clave entre pasos
- [ ] Desarrollar compresor de contexto histórico
- [ ] Implementar límites de tamaño de contexto por paso

## Fase 4: Integración con Agentes
- [ ] Implementar soporte para kiro-cli
- [ ] Implementar soporte para claude-code
- [ ] Implementar soporte para opencode
- [ ] Crear interfaz genérica agent-agnostic
- [ ] Manejar flags y argumentos específicos por agente

## Fase 5: Control de Flujo
- [ ] Implementar ejecución secuencial determinista de pasos
- [ ] Crear sistema de dependencias entre pasos
- [ ] Implementar captura y almacenamiento de outputs
- [ ] Desarrollar sistema de logs por paso
- [ ] Implementar manejo de errores y rollback

## Fase 6: Testing
- [ ] Escribir tests unitarios para parser YAML
- [ ] Crear tests table-driven para executor
- [ ] Implementar tests de integración con agentes mock
- [ ] Añadir tests de race condition con -race flag
- [ ] Validar manejo de contexto en pipelines largos

## Fase 7: Optimización
- [ ] Optimizar gestión de memoria en contexto
- [ ] Implementar timeouts con context.Context
- [ ] Añadir cancelación de pipelines
- [ ] Optimizar comunicación stdin/stdout
- [ ] Implementar pool de workers si es necesario

## Fase 8: CLI y UX
- [ ] Crear CLI principal con cobra/flag
- [ ] Implementar comando run para ejecutar pipelines
- [ ] Añadir comando validate para verificar YAML
- [ ] Crear comando dry-run para simular ejecución
- [ ] Implementar output formateado y progress indicators

## Fase 9: Documentación
- [ ] Documentar formato YAML de pipelines
- [ ] Crear ejemplos de uso común
- [ ] Documentar sistema de contexto y variables
- [ ] Escribir guía de integración de nuevos agentes
- [ ] Documentar best practices y limitaciones

## Fase 10: Release
- [ ] Configurar go mod con dependencias mínimas
- [ ] Crear Makefile para build y test
- [ ] Configurar golangci-lint
- [ ] Preparar binarios para múltiples plataformas
- [ ] Crear README con quick start
