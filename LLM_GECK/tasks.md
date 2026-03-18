# Tasks — Animus_Prion

**Last Updated:** 2026-03-17

## Legend

- `[ ]` — Not started
- `[x]` — Complete
- `[BLOCKED: reason]` — Cannot proceed
- `[DECISION: topic]` — Awaiting human input

## Current Sprint — Engineering Overhaul

### Priority 1: Immediate Code Fixes (30 min)
- [ ] 5.1: Replace containsError() string matching with regex patterns (eliminate false positives)
- [ ] 5.2: Fix health check URL construction in LocalProvider.Available()
- [ ] 5.3: Fix Available() fallback logic — return false when server unreachable

### Priority 2: Native GGUF via Subprocess (Path A)
- [ ] Create internal/llm/native.go — NativeProvider with process lifecycle
- [ ] Implement freePort() for dynamic port allocation
- [ ] Implement findLlamaServer() with search order ($PRION_LLAMA_SERVER → ~/.animus_prion/bin → PATH)
- [ ] Health-check loop with timeout
- [ ] Graceful shutdown (SIGTERM + Wait)
- [ ] Compose with LocalProvider for Generate() calls
- [ ] Add "native" provider to factory
- [ ] Update config defaults for native provider
- [ ] Add `prion setup` command for llama-server download

### Priority 3: Round 3 Benchmark
- [ ] Re-run benchmark with all fixes applied

### Priority 4: Benchmark Harness
- [ ] Create benchmarks/ directory structure
- [ ] Implement `prion bench` subcommand
- [ ] Machine-readable JSON output

### Priority 5: MCP Server Mode
- [ ] Add mcp-go dependency
- [ ] Create cmd/prion/mcp.go — `prion serve`
- [ ] Expose Manifold search, knowledge graph, vector store as MCP tools

## Backlog
- [ ] GBNF grammar constraints (needs native GGUF first)
- [ ] Tree-sitter multi-language parsing (Python, Rust, TypeScript)
- [ ] CI/CD with GitHub Actions
- [ ] Streaming support for API providers
- [ ] Session persistence

## Completed
- [x] Phase 0-12: Core framework — Entry #1-3
- [x] Self-correction v1 + Benchmarks R1/R2 — Entry #4
- [x] 6 post-benchmark fixes — Entry #4
- [x] CLI UX overhaul (banner, auto-server, interactive default)
