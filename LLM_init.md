# Project: Animus_Prion

**Repository:** https://github.com/crussella0129/Animus_Prion
**Local Path:** C:/Users/charl/Animus_Prion
**Created:** 2026-03-17

## Goal

Rewrite the Animus local-first LLM agent in 100% Go for lightning speed. Animus_Prion is a compiled, single-binary agent with plan-then-execute architecture, Manifold multi-strategy retrieval, hardcoded orchestration, and LLM-powered reasoning. It preserves all security hardening (workspace boundaries, injection detection, execution budgets, permission deny-lists) while gaining Go's concurrency, compilation speed, and zero-dependency deployment.

## Success Criteria

- [ ] Core agent loop with tool calling, reflection, and max-turns enforcement
- [ ] Plan-then-execute pipeline (parser, decomposer, executor) with scope enforcement
- [ ] Workspace boundary enforcement with symlink-safe path resolution
- [ ] Permission checker with deny-lists, injection detection, network command blocking
- [ ] Tool framework: shell (list-based exec, no shell=true), filesystem (audit log), git
- [ ] LLM provider abstraction: OpenAI-compatible API, Anthropic API, native (llama.cpp via CGo)
- [ ] YAML configuration system with safe file permissions
- [ ] Context window management with tier-aware budgeting
- [ ] CLI with interactive REPL and slash commands
- [ ] Manifold retrieval: router (hardcoded classification), executor (RRF fusion)
- [ ] Knowledge graph: AST parsing, SQLite graph DB, incremental indexing
- [ ] Vector store: SQLite-backed embedding storage with KNN search
- [ ] All tests passing with `go test ./...`
- [ ] Single binary build: `go build ./cmd/prion`

## Constraints

- **Language:** Go 1.26+
- **Must use:** Standard library where possible, cobra (CLI), gopkg.in/yaml.v3 (config), mattn/go-sqlite3 (storage)
- **Must avoid:** Shell execution via sh -c, any CGo unless absolutely necessary (llama.cpp binding), external process managers
- **Target platforms:** Windows (primary), Linux, macOS

## Context

Animus is a Python 3.11+ local-first LLM agent with 596 passing tests, plan-then-execute architecture, and Manifold multi-strategy retrieval. The Python version works but suffers from startup latency, dependency hell (llama-cpp-python wheels, sqlite-vec), and GIL limitations for concurrent retrieval. Go eliminates all of these: single static binary, goroutine concurrency, and native compilation.

The name "Prion" reflects the self-replicating, minimal-structure nature of the rewrite — same function, radically different substrate.

## Initial Task

Set up the complete Go project structure mirroring Animus architecture, implement core packages (config, workspace, permissions, tool framework, agent loop), and verify with tests.
