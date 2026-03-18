# Tasks — Animus_Prion

**Last Updated:** 2026-03-17

## Legend

- `[ ]` — Not started
- `[x]` — Complete
- `[BLOCKED: reason]` — Cannot proceed
- `[DECISION: topic]` — Awaiting human input

## Current Sprint — Security & Quality (from Code Review)

### Tier 1: Security (CRITICAL) — Do First
- [ ] Fix workspace boundary prefix collision (workspace.go:94,113) — append filepath.Separator
- [ ] Add permission checking to all git tools (git.go) — deny-list + metachar enforcement
- [ ] Fix workspace Contains() same prefix bug

### Tier 2: Correctness (HIGH) — Do Next
- [ ] Fix WriteFileTool blind \\n/\\t unescape (filesystem.go:176) — detect double-encoding
- [ ] Fix SplitConjunctions case destruction (decomposer.go:76) — split on original, not lowered
- [ ] Add exponential backoff for retryable errors (agent.go:92)
- [ ] Add io.LimitReader for HTTP responses (api.go:135,283)
- [ ] Escape SQL LIKE wildcards in GraphDB.SearchNodes (graphdb.go:181)

### Tier 3: Quality (MEDIUM) — Then These
- [ ] Fix NativeProvider.Shutdown double-Wait race
- [ ] Fix TrimMessages O(n^2) prepend → reverse approach
- [ ] Fix ChunkByFunction dead code path
- [ ] Wire ConfirmDangerous config to permission.IsDangerous()
- [ ] Add permission checking to ListFilesTool
- [ ] Fix detectVerifyCommand hardcoded "main.py"
- [ ] Sort tool names in Registry.List() for deterministic ordering

### Tier 4: Architecture — Plan for v0.3
- [ ] Thread context.Context through public APIs (agent.Run, planner.Execute, provider.Generate)
- [ ] Unify core.Message and llm.Message into single type
- [ ] Add structured logging (slog)
- [ ] Test coverage for: agent, llm, git tools, manifold, decomposer, retrieval executor, indexer

## Backlog (from Engineering Recommendations)
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
- [x] CLI UX overhaul — Entry #5
- [x] Native GGUF provider (subprocess) — Entry #5
- [x] Code fixes (containsError, health URL, Available) — Entry #5
- [x] Progress output + prompt engineering — Entry #5
- [x] Full code review (30 issues) — Entry #5
