# Environment — Animus_Prion

**Captured:** 2026-03-17
**Last Updated:** 2026-03-17

## Development Machine

- **OS:** Windows 11 Pro 10.0.26200 (amd64)
- **Shell:** bash (Git Bash / MSYS2)

## Runtime Versions

| Tool | Version |
|------|---------|
| Go | 1.26.1 windows/amd64 |
| Git | 2.52.0.windows.1 |
| GitHub CLI | installed (gh) |

## Package State

- See `go.mod` / `go.sum`

## Environment Variables

| Variable | Purpose |
|----------|---------|
| `GOPATH` | Go workspace root |
| `GOROOT` | Go installation directory |
| `PATH` | Must include Go bin directory |

## Target Platforms

- [x] Windows
- [ ] macOS
- [ ] Linux

## Notes

- Go not on default bash PATH; requires `export PATH="/c/Program Files/Go/bin:$PATH"`
- Primary development on Windows, cross-compilation for Linux/macOS via `GOOS`/`GOARCH`
