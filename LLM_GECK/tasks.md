# Tasks — Animus_Prion

**Last Updated:** 2026-03-17

## Legend

- `[ ]` — Not started
- `[x]` — Complete
- `[BLOCKED: reason]` — Cannot proceed
- `[DECISION: topic]` — Awaiting human input

## Current Sprint — Post-Benchmark Fixes

- [ ] FIX-1: Route `prion run` through PlanExecutor for complex tasks (ROOT CAUSE of missing main.py)
- [ ] FIX-2: Smarter repeat detection — allow retries after failures, increase threshold
- [ ] FIX-3: Filesystem-based verification detection (check for Cargo.toml/go.mod/\*.py, not just step descriptions)
- [ ] FIX-4: Requirements completeness check — extract expected files from prompt, verify they exist
- [ ] FIX-5: Improve tool call parser for nested JSON arguments (brace-counting fallback)
- [ ] FIX-6: Add platform info to planner's per-step system prompt

## Backlog

- [ ] GBNF grammar generation for native models
- [ ] Native llama.cpp provider (CGo or subprocess)
- [ ] Cross-platform testing (Linux, macOS)
- [ ] CI/CD with GitHub Actions
- [ ] Streaming support for API providers
- [ ] Session persistence and transcript recording
- [ ] Integration test with real LLM (gauntlet)
- [ ] Embedding provider implementation

## Completed (Recent)

- [x] Phase 0-6 complete — Entry #1
- [x] Phase 7-9 complete — Entry #2
- [x] Phase 10-12 complete — Entry #3
- [x] Self-correction (platform prompt, verify loop, robust shell) — Entry #4 (benchmark)
