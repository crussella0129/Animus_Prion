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

### Code Review P1 Fixes (2026-03-22) — DONE
- [x] Use errors.As instead of type assertion in IsRetryable
- [x] Use sync.RWMutex for read-only methods (ExecutionBudget.Remaining, writeLog.Entries)
- [x] Add Anthropic native tool calling (tools in request, tool_use parsing in response)
- [x] Make inferStepType deterministic (sorted slices instead of maps)
- [x] Inject write log instead of global (defaultWriteLog + NewWriteFileToolWithLog)
- [x] Document Agent.history as non-concurrent (YAGNI — REPL is single-threaded)

### Code Review P2/P3 Fixes (2026-03-22) — DONE
- [x] Add --verbose/-v flag for slog debug output
- [x] Add AMD GPU detection (rocm-smi fallback after nvidia-smi)
- [x] Add `prion setup` command (environment validation + diagnostics)
- [x] Add file size limit to read_file (10MB max, clear error message)
- [x] Add max_entries to list_files (default 200, truncation message)
- [x] Document splitCommand escape limitation

### Streaming (2026-03-22) — DONE
- [x] LocalProvider.GenerateStream — OpenAI-compatible SSE parsing
- [x] AnthropicProvider.GenerateStream — Anthropic SSE parsing with tool_use support
- [x] Agent.SetStreaming — onChunk callback, auto-detects StreamProvider
- [x] REPL wired for token-by-token output

### CI/CD (2026-03-22) — DONE
- [x] GitHub Actions: test (ubuntu + windows), vet, build, gofmt lint
- [x] gofmt applied to all source files

### Skeleton Tree Decomposition (2026-03-22) — DONE
- [x] TaskNode tree, parseSkeletonLevel, isLeafHeuristic, similar(), summarizeBranch
- [x] SkeletonPlanner: recursive expansion, restatement detection, tree execution
- [x] Session checkpoint/resume: SaveSession, LoadSession, FindNextPending, auto-resume
- [x] GBNF grammar constraints for local models (ToolCallGrammar, ToolCallOrTextGrammar)
- [x] Agent uses GBNF grammar when provider lacks native tool support
- [x] Wired into REPL and CLI (replaces PlanExecutor)

## Backlog
- [ ] Round 3 benchmark with 14B model
- [ ] Benchmark harness (`prion bench`)
- [ ] MCP server mode (`prion serve`)
- [ ] Tree-sitter multi-language parsing
- [ ] Consider HNSW for vector store >10k chunks

## Completed
- [x] Phase 0-12: Core framework
- [x] Self-correction + Benchmarks R1/R2
- [x] 6 post-benchmark fixes
- [x] CLI UX + Native GGUF + Progress output
- [x] Code review (30 issues found)
- [x] Tiers 1-4: 22 issues fixed, 108 tests
- [x] Code Review P0: 4 bugs fixed, 129 tests
- [x] Code Review P1: 6 issues fixed, 134 tests
- [x] Code Review P2/P3: 6 items, 136 tests
- [x] CI/CD + gofmt: GitHub Actions, 136 tests
- [x] Streaming: LocalProvider + Anthropic + agent wiring, 142 tests
- [x] Round 2 review: Anthropic message merging, input_json_delta, cancellable sleeps, 145 tests
- [x] Skeleton tree decomposition + GBNF grammar + session persistence, 170 tests
