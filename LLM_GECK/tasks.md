# Tasks — Animus_Prion

**Last Updated:** 2026-03-17

## Legend

- `[ ]` — Not started
- `[x]` — Complete
- `[BLOCKED: reason]` — Cannot proceed
- `[DECISION: topic]` — Awaiting human input

## Current Sprint — Security & Quality (from Code Review)

### Tier 1: Security (CRITICAL) — DONE
- [x] Fix workspace boundary prefix collision (workspace.go:94,113)
- [x] Add permission checking to all git tools (git.go)
- [x] Fix workspace Contains() same prefix bug

### Tier 2: Correctness (HIGH) — DONE
- [x] Fix WriteFileTool blind \\n/\\t unescape — detect double-encoding
- [x] Fix SplitConjunctions case destruction — split on original text
- [x] Add exponential backoff for retryable errors
- [x] Add io.LimitReader for HTTP responses (10MB cap)
- [x] Escape SQL LIKE wildcards in GraphDB.SearchNodes

### Tier 3: Quality (MEDIUM)
- [ ] Fix NativeProvider.Shutdown double-Wait race
- [ ] Fix TrimMessages O(n^2) prepend
- [ ] Fix ChunkByFunction dead code path
- [ ] Wire ConfirmDangerous config to permission.IsDangerous()
- [ ] Add permission checking to ListFilesTool
- [ ] Fix detectVerifyCommand hardcoded "main.py"
- [ ] Sort tool names in Registry.List() for deterministic ordering

### Tier 4: Architecture — Plan for v0.3
- [ ] Thread context.Context through public APIs
- [ ] Unify core.Message and llm.Message into single type
- [ ] Add structured logging (slog)
- [ ] Test coverage for: agent, llm, git tools, manifold, decomposer, retrieval executor, indexer

## Backlog
- [ ] Round 3 benchmark with 14B model
- [ ] Benchmark harness (`prion bench`)
- [ ] MCP server mode (`prion serve`)
- [ ] GBNF grammar constraints
- [ ] Tree-sitter multi-language parsing
- [ ] CI/CD with GitHub Actions

## Completed
- [x] Phase 0-12: Core framework — Entry #1-3
- [x] Self-correction + Benchmarks R1/R2 — Entry #4
- [x] 6 post-benchmark fixes — Entry #4
- [x] CLI UX + Native GGUF + Progress output — Entry #5
- [x] Code review (30 issues) — Entry #5
- [x] Tier 1 security fixes (workspace boundary, git permissions) — Entry #6
- [x] Tier 2 correctness fixes (5 HIGH issues) — Entry #6
