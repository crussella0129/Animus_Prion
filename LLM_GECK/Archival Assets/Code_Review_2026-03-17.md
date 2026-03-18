# Code Review — Animus_Prion

**Date:** 2026-03-17
**Reviewer:** CodeRabbit AI (via Claude Code)
**Scope:** Full repository, 42 Go files, ~7,500 LOC

## Summary

| Severity | Count |
|----------|-------|
| CRITICAL | 5 |
| HIGH | 6 |
| MEDIUM | 11 |
| LOW | 8 |
| Test Gaps | 8 |
| Architecture | 4 |

## CRITICAL

1. **Workspace boundary bypass via path prefix collision** — workspace.go:94,113. `strings.HasPrefix("project-evil", "project")` passes. Fix: append path separator.
2. **Git tools have zero permission checking** — git.go:12. `gitExec()` runs `exec.Command` with no deny-list, no metachar check. LLM could run destructive git commands.
3. **Anthropic API key validation** — api.go:198. `Available()` returns true for `"not-needed"` placeholder.

## HIGH

4. **Unbounded HTTP response read** — api.go:135,283. `io.ReadAll` with no size limit → potential OOM.
5. **WriteFileTool blindly unescapes \\n/\\t** — filesystem.go:176. Corrupts source code containing escape sequences.
6. **No backoff on retryable errors** — agent.go:92. Agent hammers API on 429s.
7. **SplitConjunctions destroys case** — decomposer.go:76. Returns lowercase parts.
8. **SQL LIKE injection in SearchNodes** — graphdb.go:181. Unescaped `%` and `_`.
9. **Agent history grows unbounded** — agent.go:70. Memory never compacted.

## MEDIUM

10. Race condition in NativeProvider.Shutdown (double Wait)
11. TrimMessages O(n^2) prepend
12. Token estimation uses byte length not rune count
13. splitCommand ignores backslash escapes
14. ErrorCategory "parse" regex too broad
15. ChunkByFunction dead code path
16. VectorStore.Search loads entire DB
17. ConfirmDangerous config never checked
18. ListFilesTool lacks permission checking
19. Process orphaning on panic
20. freePort TOCTOU race

## LOW

21. Non-deterministic tool ordering (map iteration)
22. Register() errors silently ignored
23. Global writeLog singleton
24. Anthropic message conversion drops tool metadata
25. detectVerifyCommand hardcodes "main.py"
26. Scanner error not checked
27. Redefined min() shadows builtin
28. Should use math/rand/v2

## Test Coverage Gaps

- internal/agent/ — 0 tests (most critical package)
- internal/llm/ — 0 tests
- internal/tools/git.go — 0 tests
- internal/tools/manifold.go — 0 tests
- internal/planner/decomposer.go — 0 tests
- internal/retrieval/executor.go — 0 tests
- internal/memory/chunker.go — bug (#15) would have been caught
- internal/knowledge/indexer.go — 0 tests

## Architecture Concerns

1. Dual message types (core.Message vs llm.Message) — manual conversion, error-prone
2. No context.Context threading — can't cancel agent/plan execution
3. No structured logging
4. Tool execution during planning uses fullRegistry (misleading but correct)
