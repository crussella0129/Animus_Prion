# Session Log — Animus_Prion

*Append only. Do not edit existing entries.*

---

## Entry #0 — 2026-03-17

### Summary
Project initialized. GECK structure created. Go module initialized. GitHub repo created.

### Understood Goals
- Rewrite Animus (Python) in 100% Go for performance
- Preserve all architecture: agent loop, plan-then-execute, Manifold retrieval, security hardening
- Single static binary deployment
- Local-first, edge-hardware optimized

### Questions/Ambiguities
- Native llama.cpp binding: CGo or subprocess? (CGo adds build complexity but better performance)
- SQLite driver: mattn/go-sqlite3 (CGo) vs modernc.org/sqlite (pure Go)? Pure Go preferred for cross-compilation

### Initial Tasks
- Set up project scaffold with Go conventions
- Implement core packages (config, workspace, permissions)
- Implement tool framework (registry, shell, filesystem, git)
- Implement LLM provider abstraction
- Implement agent loop

### Checkpoint
**Status:** CONTINUE — Project scaffold in progress.

---

## Entry #1 — 2026-03-17

### Summary
Full project framework implemented: 35 source files across 8 packages, 27 tests passing, binary builds and runs. Pushed to GitHub.

### Actions
- Created Go module with cobra and yaml.v3 dependencies
- Implemented internal/config: YAML config with save/load and chmod 600
- Implemented internal/core: workspace (boundary enforcement, symlink-safe), context (tier-aware budgets), errors (classification + recovery), toolparse (3-strategy parser), message types
- Implemented internal/permission: deny-lists, injection regex, metachar rejection, network blocking
- Implemented internal/tools: registry (validation, coercion, OpenAI schema), shell (list-based exec, execution budget), filesystem (read/write/list with audit log), git (8 operations)
- Implemented internal/llm: provider interface, OpenAI and Anthropic API clients, factory with fallback
- Implemented internal/agent: full agentic loop (tool calling, reflection, repeat detection, graceful degradation)
- Implemented internal/planner: regex parser (max 7 steps), LLM decomposer, chunked executor (scope enforcement, tool filtering)
- Implemented cmd/prion: cobra CLI (chat REPL, run, version, config init/show)
- Resolved import cycle: extracted agent into its own package
- Fixed 2 test failures (TrimMessages budget, Windows workspace boundary)
- Built binary: `bin/prion.exe` — `Prion v0.1.0`

### Files Changed
- `go.mod`, `go.sum` — Module definition + dependencies
- `cmd/prion/main.go` — CLI entrypoint
- `internal/agent/agent.go` — Core agentic loop
- `internal/config/config.go` + `_test.go` — Configuration system
- `internal/core/workspace.go` + `_test.go` — Workspace boundary
- `internal/core/errors.go` + `_test.go` — Error classification
- `internal/core/context.go` + `_test.go` — Token budgeting
- `internal/core/toolparse.go` + `_test.go` — Tool call parsing
- `internal/core/message.go` — Chat message types
- `internal/permission/checker.go` + `_test.go` — Security checks
- `internal/tools/registry.go` + `_test.go` — Tool framework
- `internal/tools/shell.go` — Shell execution with safety
- `internal/tools/filesystem.go` — File I/O with audit
- `internal/tools/git.go` — Git operations
- `internal/llm/provider.go` — Provider interface
- `internal/llm/api.go` — OpenAI + Anthropic clients
- `internal/llm/factory.go` — Provider factory
- `internal/planner/parser.go` + `_test.go` — Plan parsing
- `internal/planner/decomposer.go` — LLM task decomposition
- `internal/planner/executor.go` — Step execution with scope enforcement

### Commits
- `22fac35` — feat: initial Go rewrite of Animus agent — project scaffold with all core packages

### Findings
- Go's import cycle enforcement caught an architectural issue (core↔tools↔llm cycle) that Python's circular imports would have silently allowed. Resolved by extracting agent to its own package.
- Go 1.26.1 builds the entire project in <3 seconds. The compiled binary is a single static file — no runtime dependencies.
- The `[]tools.OpenAISchema` to `[]any` conversion is needed because Go doesn't implicitly convert typed slices. This is a deliberate language design choice for type safety.

### Issues
None

### Checkpoint
**Status:** CONTINUE — Framework complete. Ready for Phase 7 (additional tests) and Phase 8 (README).

### Next
- Add tests for filesystem tools, shell tool, git tools
- Create README.md with usage documentation
- Begin Manifold retrieval implementation (backlog)

---

## Entry #2 — 2026-03-17

### Summary
Implemented Manifold retrieval (hardcoded router + RRF executor), knowledge graph (Go AST parser + SQLite graph DB + incremental indexer), and comprehensive tool tests. 70 tests passing across 8 packages.

### Actions
- Added 16 filesystem/shell tool tests (read, write, boundary, audit, metachar, timeout, budget)
- Implemented retrieval router: 100% hardcoded query classification (semantic, structural, hybrid, keyword)
- Implemented retrieval executor: strategy dispatch, RRF fusion, result deduplication
- Implemented knowledge/parser: Go AST extraction (functions, methods, types, imports, call edges)
- Implemented knowledge/graphdb: SQLite graph DB with BFS transitive callers, blast radius, inheritance
- Implemented knowledge/indexer: SHA-256 content-hashed incremental indexing
- Added modernc.org/sqlite pure-Go dependency (no CGo required)

### Files Changed
- `internal/tools/filesystem_test.go` — 9 filesystem tool tests
- `internal/tools/shell_test.go` — 7 shell tool tests
- `internal/retrieval/router.go` — Hardcoded query classifier
- `internal/retrieval/executor.go` — Strategy executor with RRF
- `internal/retrieval/router_test.go` — 13 retrieval tests
- `internal/knowledge/parser.go` — Go AST parser
- `internal/knowledge/graphdb.go` — SQLite graph database
- `internal/knowledge/indexer.go` — Incremental file indexer
- `internal/knowledge/parser_test.go` — 7 parser tests
- `internal/knowledge/graphdb_test.go` — 7 graph DB tests

### Commits
- `b6d0117` — test: add comprehensive tool tests
- `4cbd066` — feat: add Manifold retrieval system
- `cd28510` — feat: add knowledge graph

### Findings
- Go's `go/ast` standard library is remarkably complete — extracts all the entities we need with zero external dependencies
- `modernc.org/sqlite` pure-Go driver adds ~15MB to binary but eliminates CGo entirely, keeping cross-compilation simple
- The two-pass AST strategy (declarations first, then call edges) mirrors compiler name-resolution order
- RRF fusion with k=60 is the same constant used in academic IR literature; works well here

### Issues
None

### Checkpoint
**Status:** CONTINUE — Knowledge graph complete. Vector store and README next.

### Next
- Implement vector store (SQLite-backed embedding storage with KNN)
- Create README.md
- Wire knowledge graph + retrieval into the agent as a tool

---

## Entry #3 — 2026-03-17

### Summary
Completed vector store, ManifoldSearchTool, and README. All 12 phases done. 77 tests passing, 7 commits pushed.

### Actions
- Implemented memory/vectorstore: SQLite BLOB embeddings, cosine KNN brute-force, batch insert, source deletion
- Implemented memory/chunker: line-aware text chunking with overlap, per-function Go chunking
- Implemented tools/manifold: ManifoldSearchTool wiring retrieval router + graph DB into agent tool framework
- Created README.md: architecture, quick start, config, security model, Manifold docs

### Files Changed
- `internal/memory/vectorstore.go` — SQLite vector store with KNN search
- `internal/memory/chunker.go` — Text and function-level chunking
- `internal/memory/vectorstore_test.go` — 7 vector store tests
- `internal/tools/manifold.go` — ManifoldSearchTool agent tool
- `README.md` — Project documentation

### Commits
- `b76e3d0` — feat: add vector store
- `00f77af` — feat: wire Manifold search tool + add README

### Findings
- Brute-force cosine KNN is adequate for codebases under ~50K chunks (<100ms search time)
- Pure-Go sqlite driver eliminates CGo but adds ~15MB to binary — acceptable tradeoff for cross-compilation
- The ManifoldSearchTool gracefully degrades when backends are nil — critical for first-run experience

### Issues
None

### Checkpoint
**Status:** CONTINUE — All planned phases complete. Backlog items remain.

### Next
- CI/CD with GitHub Actions
- Embedding provider implementation
- Integration test with real LLM

---

## Entry #4 — 2026-03-17

### Summary
Benchmarked Animus vs Prion (2 rounds), implemented self-correction (platform prompt, verify loop, robust shell), then analyzed benchmark results and implemented 6 targeted fixes addressing every failure mode.

### Actions
- Round 1 benchmark: Prion 7.25 vs Animus 5.85 (Prion Rust compiles, Animus #![no_std] error)
- Implemented self-correction v1: platform prompt, verify-and-repair loop, robust shell tool
- Round 2 benchmark: Prion 7.70 vs Animus 4.80 (Prion auto-compiled Rust, 79s vs 117s)
- Analyzed all 4 experiment runs, traced 6 root causes to source code
- FIX-1: Route `prion run` through PlanExecutor for complex tasks
- FIX-2: Smarter repeat detection (only count repeats on success, threshold 1→2)
- FIX-3: Filesystem-based verify detection (probe Cargo.toml/go.mod/*.py)
- FIX-4: Requirements completeness check (extract expected files from prompt, verify existence)
- FIX-5: Brace-counting JSON parser fallback for nested tool calls
- FIX-6: Platform info in planner per-step prompt

### Files Changed
- `cmd/prion/main.go` — setupEnv refactor, planner routing for complex tasks
- `internal/agent/agent.go` — platform prompt, smarter repeat detection
- `internal/tools/shell.go` — robust arg handling (string/array/object/any)
- `internal/planner/executor.go` — verify loop, completeness check, filesystem probing, platform prompt
- `internal/core/toolparse.go` — brace-counting JSON extractor (Strategy 4)
- `internal/planner/executor_test.go` — filesystem probing, extractExpectedFiles tests
- `internal/core/toolparse_test.go` — nested JSON, balanced extraction tests
- `LLM_GECK/Archival Assets/Benchmark_Animus_vs_Prion_2026-03-17.md` — 2-round comparison

### Commits
- `a665ef9` — refactor: rename OpenAI to LocalProvider
- `1cdf890` — docs: Round 1 benchmark
- `c614503` — feat: self-correction v1
- `4a7174c` — docs: Round 2 benchmark
- `7c777dd` — feat: 6 post-benchmark fixes

### Findings
- Prion auto-compiled Rust via verify loop — first self-verifying behavior across both agents
- Speed flipped: Prion 79s vs Animus 117s (57% faster than Prion R1)
- Root cause of missing main.py: `prion run` bypassed the planner entirely
- 7B models are stochastic: Animus fixed #![no_std] in R2 but introduced a Python syntax error
- Filesystem probing is more reliable than step-description keyword matching

### Issues
None blocking. Known: `containsError()` may false-positive on strings containing "error" in variable names.

### Checkpoint
**Status:** CONTINUE — All 6 benchmark-identified issues fixed. 97 tests passing.

### Next
- Round 3 benchmark to validate all 6 fixes
- CI/CD with GitHub Actions
- Embedding provider implementation

---

## Entry #5 — 2026-03-17

### Summary
Engineering overhaul session: implemented native GGUF provider (subprocess), fixed 3 code bugs (containsError false positives, health check URL, Available() logic), added live progress output, fixed prompt engineering for 7B tool use, increased timeout for 14B models, ran full code review (30 issues found across 4 severity levels). Bumped to v0.2.0.

### Actions
- Implemented NativeProvider: managed llama-server subprocess with auto-port, health-check, graceful shutdown
- Fixed containsError() with regex patterns (eliminates false positives)
- Fixed health check URL construction and Available() fallback logic
- Added live progress output (Planning..., [1/3] step, Verifying...)
- Improved prompt engineering: explicit write_file instruction, JSON format example for steps
- Increased HTTP timeout to 300s for 14B models
- Ran comprehensive code review: 5 critical, 6 high, 11 medium, 8 low, 8 test gaps
- Installed prion to ~/.local/bin for PowerShell PATH access
- Updated config default to native provider, version to 0.2.0

### Files Changed
- `internal/llm/native.go` — NEW: NativeProvider with process lifecycle
- `internal/llm/factory.go` — "native" provider, Shutdowner interface
- `internal/llm/api.go` — Health check fix, Available() fix, 300s timeout
- `internal/planner/executor.go` — Progress callback, containsError regex, filesystem probing, prompt engineering
- `internal/agent/agent.go` — Platform prompt, write_file instructions, repeat detection
- `cmd/prion/main.go` — v0.2.0, env refactor, progress wiring, provider shutdown

### Commits
- `ed04124` — feat: proper CLI UX
- `2df6a40` — feat: native GGUF provider + code fixes — v0.2.0
- `fdcece9` — feat: live progress output
- `bd0e308` — fix: prompt engineering for 7B tool use
- `e959125` — fix: 300s timeout for 14B models

### Findings (Code Review)
- 2 CRITICAL security issues: workspace boundary bypass via path prefix collision, git tools have zero permission checking
- WriteFileTool blindly unescapes \\n/\\t (corrupts source code with escape sequences)
- Agent has no backoff on retryable errors (hammers API on 429s)
- SplitConjunctions destroys case (returns lowercase)
- No context.Context threading — can't cancel long operations
- 8 packages have zero test coverage (agent, llm, git tools, manifold, decomposer, retrieval executor, chunker, indexer)

### Issues
- CRITICAL: Workspace boundary prefix collision needs immediate fix
- CRITICAL: Git tools need permission checking
- HIGH: WriteFileTool escape handling corrupts code
- 14B model works on RTX 2080 Ti but needs 300s timeout

### Checkpoint
**Status:** CONTINUE — v0.2.0 shipped. Code review complete. Security fixes next.

### Next
- Fix workspace boundary (append path separator before prefix check)
- Add permission checking to git tools
- Fix WriteFileTool escape handling
- Add context.Context to public APIs
- Backoff on retryable errors
- Test coverage for critical packages

---

## Entry #6 — 2026-03-17

### Summary
Tiers 1-4 code review fixes: 22 issues resolved across security, correctness, quality, and architecture. All 4 tiers complete. 108 tests passing across 19 test files.

### Checkpoint
**Status:** CONTINUE — All tiers done.

---

## Entry #7 — 2026-03-22

### Summary
External code review P0 fixes: 4 bugs fixed, 15 agent tests added. 129 tests passing.

### Actions
- Fixed TrimMessages negative budget: clamped to zero when system prompt exceeds maxTokens, returns system + last message
- Fixed repeat detection: hash ALL tool calls into the key, not just toolCalls[0] — prevents multi-tool loop evasion
- Propagated context.Context through planner: Decompose, ExecuteStep, Execute, checkCompleteness, verifyAndRepair all accept ctx now. Ctrl+C cancels LLM calls during plan execution
- Created agent_test.go: 15 tests covering Run flow, tool execution, repeat detection (single + multi-tool), context cancellation, error recovery (retryable + non-retryable), evaluateToolResult, context trimming, history growth, Reset

### Files Changed
- `internal/core/context.go` — TrimMessages negative budget fix
- `internal/agent/agent.go` — Repeat detection hashes all tool calls
- `internal/planner/decomposer.go` — Decompose accepts context.Context
- `internal/planner/executor.go` — ExecuteStep, Execute, checkCompleteness, verifyAndRepair accept context.Context
- `internal/agent/agent_test.go` — NEW: 15 agent tests
- `cmd/prion/main.go` — Updated callers to pass context.Background() to planner
- `LLM_GECK/tasks.md` — Updated with P0 done, P1 backlog items

### Commits
- (pending)

### Findings
- Review item 2.1 (go.mod version) was INVALID — Go 1.26.1 is the actual installed version
- Review item 2.7 (math/rand seed) was INVALID — Go 1.26.1 auto-seeds since Go 1.20
- 8 of 22 review items verified as real bugs, 2 rejected, remainder deferred to P1/P2

### Issues
None blocking.

### Checkpoint
**Status:** CONTINUE — P0 fixes complete. P1 backlog documented in tasks.md.

### Next
- P1 fixes: errors.As, RWMutex, Anthropic tool calling, deterministic inferStepType, inject write log, Agent.history mutex
