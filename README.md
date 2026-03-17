# Animus Prion

A local-first LLM agent with plan-then-execute architecture, rewritten in Go for lightning speed.

Prion is the Go rewrite of [Animus](https://github.com/crussella0129/Animus) — same architecture, radically different substrate. Single static binary, goroutine concurrency, zero runtime dependencies.

## Architecture

```
cmd/prion/          CLI entrypoint (cobra)
internal/
  agent/            Agentic loop — tool calling, reflection, repeat detection
  config/           YAML configuration with safe file permissions
  core/             Workspace boundary, context management, error classification, tool parsing
  knowledge/        Code intelligence — Go AST parser, SQLite graph DB, incremental indexer
  llm/              Provider interface — OpenAI, Anthropic, native (planned)
  memory/           Vector store — SQLite BLOB embeddings, cosine KNN, text chunking
  permission/       Security — deny-lists, injection detection, network blocking
  planner/          Plan-then-execute — regex parser, LLM decomposer, chunked executor
  retrieval/        Manifold — hardcoded query router, RRF fusion executor
  tools/            Tool framework — registry, shell, filesystem, git, manifold search
```

## Quick Start

```bash
# Build
go build -o prion ./cmd/prion

# Initialize config
./prion config init

# Interactive chat
./prion chat

# Single task
./prion run "read main.go and explain what it does"

# Version
./prion version
```

## Configuration

Config lives at `~/.animus_prion/config.yaml`:

```yaml
model:
  provider: openai          # openai, anthropic, native
  model_name: gpt-4
  temperature: 0.7
  context_length: 8192
  base_url: ""              # custom endpoint (vLLM, LM Studio)
  api_key: ""               # or set OPENAI_API_KEY / ANTHROPIC_API_KEY

agent:
  max_turns: 20
  confirm_dangerous: true
  workspace_root: "."

rag:
  chunk_size: 512
  embedding_model: all-MiniLM-L6-v2
  top_k: 5
```

## Security

All security hardening from Animus Phase 3 is preserved:

- **No shell interpretation** — commands execute via `os/exec.Command` (list-based args)
- **Metacharacter rejection** — `; | & < > $ ()` are blocked outright
- **Workspace boundary** — all file/shell/git operations enforce project root via symlink-safe resolution
- **Permission deny-lists** — dangerous commands, directories, and files are blocked
- **Injection detection** — regex patterns catch `$()`, backticks, chaining operators
- **Execution budget** — 300s cumulative limit prevents runaway processes
- **Scope enforcement** — planner restricts each step to its allowed tool set; 2nd violation terminates

## Manifold Retrieval

Query classification is 100% hardcoded (no LLM, <1ms):

| Query Pattern | Strategy | Example |
|---|---|---|
| "how does X work?" | Semantic | Vector similarity search |
| "who calls X()?" | Structural | Graph BFS traversal |
| "find X and what depends on it" | Hybrid | RRF fusion of both |
| "find TODO comments" | Keyword | Pattern matching |

## Knowledge Graph

The Go AST parser extracts:
- Functions, methods, structs, interfaces
- Import relationships
- Call edges (who calls what)
- Doc comments and signatures

Stored in SQLite with BFS traversal for callers, callees, blast radius, and inheritance queries.

## Tests

```bash
go test ./... -v
```

77 tests across 9 packages covering config, workspace, permissions, tools, retrieval, knowledge graph, vector store, planner, and error classification.

## License

MIT
