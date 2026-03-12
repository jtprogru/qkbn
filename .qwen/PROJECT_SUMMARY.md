The user wants me to create a comprehensive project summary in markdown format based on the conversation history. Let me analyze what happened:

1. I was asked to analyze the directory and generate QWEN.md
2. I explored the project structure, read multiple files (go.mod, Taskfile.yml, main.go, .golangci.yaml, .goreleaser.yaml, .gitignore)
3. I discovered this is a Go project called "qkbn" - a local Kanban board server for Qwen-code tasks
4. I created QWEN.md with comprehensive project documentation
5. I was asked to write README.md, which I did

Key findings:
- Project name: qkbn (Qwen-code Kanban Board)
- Language: Go 1.25.0
- Purpose: Local web server that displays Qwen-code tasks from ~/.qwen/todos/ as a Kanban board
- No external dependencies (standard library only)
- Uses Taskfile for build commands
- Has golangci-lint configuration
- Has GoReleaser configuration (but with outdated references to "jtpost")
- No tests currently exist
- There's a discrepancy: Taskfile.yml and .goreleaser.yaml mention "jtpost" but the actual code uses "qkbn"

Let me create a comprehensive summary.# Project Summary

## Overall Goal
Create documentation for **qkbn** — a local Go-based Kanban board web server that visualizes Qwen-code tasks from `~/.qwen/todos/` in a browser.

## Key Knowledge

### Project Facts
| Aspect | Details |
|--------|---------|
| **Name** | qkbn (Qwen-code Kanban Board) |
| **Language** | Go 1.25.0 |
| **Dependencies** | None (standard library only) |
| **Main file** | `cmd/qkbn/main.go` |
| **Server** | http://localhost:9090 |
| **Data source** | `~/.qwen/todos/*.json` |

### Architecture
- Reads latest modified JSON file from `~/.qwen/todos/`
- Auto-refreshes page every 5 seconds
- Three columns: TODO (pending), IN PROGRESS, DONE (completed)
- Uses `net/http` + `html/template`

### Build Commands (Taskfile.yml)
```bash
task run              # go run cmd/qkbn/main.go
task build            # CGO_ENABLED=0 go build
task install          # go install
task lint             # golangci-lint run
task test             # go test -v ./...
task test:coverage    # Tests with HTML report
task clean            # Remove artifacts
task release          # GoReleaser snapshot
```

### Known Issues
- **Inconsistency**: `Taskfile.yml` and `.goreleaser.yaml` reference `jtpost` (old name), but actual binary is `qkbn`
- **No tests**: Test tasks exist but no test files present
- **Go version**: Requires Go 1.25.0 (bleeding edge)

### User Preferences
- Output language: **Russian** (mandatory)
- Concise, direct communication style
- No chitchat or unnecessary explanations

## Recent Actions

| Action | Result |
|--------|--------|
| Explored project structure | Single Go file, no tests, Taskfile-based workflow |
| Read 6 config files | go.mod, Taskfile.yml, .golangci.yaml, .goreleaser.yaml, .gitignore, main.go |
| Created `QWEN.md` | Comprehensive context file with build commands, structure, conventions |
| Created `README.md` | User-facing documentation with quick start, installation options, JSON format |

## Current Plan

| # | Task | Status |
|---|------|--------|
| 1 | Analyze project structure and configs | [DONE] |
| 2 | Create QWEN.md for agent context | [DONE] |
| 3 | Create README.md for users | [DONE] |
| 4 | Fix `jtpost` → `qkbn` inconsistency in Taskfile.yml | [TODO] |
| 5 | Fix `jtpost` → `qkbn` in .goreleaser.yaml | [TODO] |
| 6 | Add basic unit tests | [TODO] |

---

## Summary Metadata
**Update time**: 2026-03-12T15:07:51.497Z 
