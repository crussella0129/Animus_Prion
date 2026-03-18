# Tasks — Animus_Prion

**Last Updated:** 2026-03-17

## Legend

- `[ ]` — Not started
- `[x]` — Complete

## Current Sprint — Security & Quality (from Code Review)

### Tier 1: Security (CRITICAL) — DONE
- [x] Fix workspace boundary prefix collision
- [x] Add permission checking to all git tools
- [x] Fix workspace Contains() same prefix bug

### Tier 2: Correctness (HIGH) — DONE
- [x] Fix WriteFileTool blind \\n/\\t unescape
- [x] Fix SplitConjunctions case destruction
- [x] Add exponential backoff for retryable errors
- [x] Add io.LimitReader for HTTP responses
- [x] Escape SQL LIKE wildcards in SearchNodes

### Tier 3: Quality (MEDIUM) — DONE
- [x] Fix NativeProvider.Shutdown double-Wait race
- [x] Fix TrimMessages O(n^2) prepend
- [x] Fix ChunkByFunction dead code path
- [x] Wire ConfirmDangerous to permission.IsDangerous()
- [x] Add permission checking to ListFilesTool
- [x] Fix detectVerifyCommand hardcoded "main.py"
- [x] Sort tool names in Registry.List()

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
- [x] Phase 0-12: Core framework
- [x] Self-correction + Benchmarks R1/R2
- [x] 6 post-benchmark fixes
- [x] CLI UX + Native GGUF + Progress output
- [x] Code review (30 issues)
- [x] Tier 1: 3 CRITICAL security fixes
- [x] Tier 2: 5 HIGH correctness fixes
- [x] Tier 3: 7 MEDIUM quality fixes
