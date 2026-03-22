# Tasks — Animus_Prion

**Last Updated:** 2026-03-22

## All Tiers Complete

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

### Tier 4: Architecture — DONE
- [x] Unified core.Message — llm.Message is now a type alias
- [x] context.Context through Provider.Generate, Agent.Run
- [x] Structured logging with log/slog
- [x] Test coverage: llm, retrieval executor, chunker (108 tests, 19 files)

### Code Review P0 Fixes (2026-03-22) — DONE
- [x] Fix TrimMessages negative budget (system prompt > maxTokens edge case)
- [x] Fix repeat detection to hash all tool calls (not just first)
- [x] Propagate context.Context through planner (Decompose, ExecuteStep, Execute)
- [x] Write agent package tests (15 tests: repeat detection, trimming, error recovery, cancellation)

## Backlog (P1 — from 2026-03-22 review)
- [ ] Use errors.As instead of type assertion in IsRetryable
- [ ] Use sync.RWMutex for read-only methods (ExecutionBudget.Remaining, writeLog.Entries)
- [ ] Add Anthropic native tool calling (send tools field in API request)
- [ ] Make inferStepType deterministic (sorted slice instead of map)
- [ ] Inject write log instead of global
- [ ] Protect Agent.history with mutex or document non-concurrent

## Backlog (P2+)
- [ ] Round 3 benchmark with 14B model
- [ ] Benchmark harness (`prion bench`)
- [ ] MCP server mode (`prion serve`)
- [ ] GBNF grammar constraints
- [ ] Tree-sitter multi-language parsing
- [ ] CI/CD with GitHub Actions
- [ ] Add streaming (StreamProvider implementations)
- [ ] Add AMD GPU detection (rocm-smi)
- [ ] Add `prion setup` command
- [ ] Consider HNSW for vector store >10k chunks

## Completed
- [x] Phase 0-12: Core framework
- [x] Self-correction + Benchmarks R1/R2
- [x] 6 post-benchmark fixes
- [x] CLI UX + Native GGUF + Progress output
- [x] Code review (30 issues found)
- [x] Tiers 1-4: 22 issues fixed, 108 tests
- [x] Code Review P0: 4 bugs fixed, 129 tests
