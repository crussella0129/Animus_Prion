# Tasks — Animus_Prion

**Last Updated:** 2026-03-17

## Legend

- `[ ]` — Not started
- `[x]` — Complete
- `[BLOCKED: reason]` — Cannot proceed
- `[DECISION: topic]` — Awaiting human input

## Current Sprint

- [x] Phase 0: Project scaffold — Go module, directory structure, GECK init
- [x] Phase 1: Core packages — config, workspace, permissions, errors, context, message, toolparse
- [x] Phase 2: Tool framework — registry, shell, filesystem, git tools
- [x] Phase 3: LLM provider abstraction — provider interface, OpenAI/Anthropic API clients, factory
- [x] Phase 4: Agent loop — agentic loop with tool calling, reflection, repeat detection
- [x] Phase 5: Planner — parser, decomposer, executor with scope enforcement
- [x] Phase 6: CLI — cobra-based CLI with chat REPL, run, config commands
- [ ] Phase 7: Additional tests — filesystem tools, shell tool, git tools, agent integration
- [ ] Phase 8: README.md with usage documentation

## Backlog

- [ ] Manifold retrieval: router (hardcoded classification), executor (RRF fusion)
- [ ] Knowledge graph: AST parsing, SQLite graph DB, incremental indexing
- [ ] Vector store: SQLite-backed embedding storage with KNN search
- [ ] GBNF grammar generation for native models
- [ ] Native llama.cpp provider (CGo or subprocess)
- [ ] Audio/TTS system (if desired in Go)
- [ ] Cross-platform testing (Linux, macOS)
- [ ] CI/CD with GitHub Actions
- [ ] Streaming support for API providers
- [ ] Session persistence and transcript recording

## Completed (Recent)

- [x] Phase 0-6 complete — Entry #1
