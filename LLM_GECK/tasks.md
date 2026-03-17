# Tasks — Animus_Prion

**Last Updated:** 2026-03-17

## Legend

- `[ ]` — Not started
- `[x]` — Complete
- `[BLOCKED: reason]` — Cannot proceed
- `[DECISION: topic]` — Awaiting human input

## Current Sprint — Post-Benchmark Fixes

- [x] FIX-1: Route `prion run` through PlanExecutor for complex tasks
- [x] FIX-2: Smarter repeat detection — allow retries after failures, increase threshold
- [x] FIX-3: Filesystem-based verification detection (Cargo.toml/go.mod/*.py probing)
- [x] FIX-4: Requirements completeness check — extract expected files, verify they exist
- [x] FIX-5: Brace-counting JSON parser fallback (Strategy 4)
- [x] FIX-6: Platform info in planner's per-step system prompt
- [ ] Round 3 benchmark to validate all 6 fixes

## Backlog

- [ ] GBNF grammar generation for native models
- [ ] Native llama.cpp provider (CGo or subprocess)
- [ ] Cross-platform testing (Linux, macOS)
- [ ] CI/CD with GitHub Actions
- [ ] Streaming support for API providers
- [ ] Session persistence and transcript recording
- [ ] Embedding provider implementation

## Completed (Recent)

- [x] Phase 0-6 complete — Entry #1
- [x] Phase 7-9 complete — Entry #2
- [x] Phase 10-12 complete — Entry #3
- [x] Self-correction v1 + Benchmarks R1/R2 — Entry #4
- [x] 6 post-benchmark fixes (FIX-1 through FIX-6) — Entry #4
