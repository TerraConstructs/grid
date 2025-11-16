# Repository Guidelines

## Issue Tracking with bd (beads)

**IMPORTANT**: This project uses **bd (beads)** for ALL issue tracking. Do NOT use markdown TODOs, task lists, or other tracking methods.

### Why bd?

- Dependency-aware: Track blockers and relationships between issues
- Git-friendly: Auto-syncs to JSONL for version control
- Agent-optimized: JSON output, ready work detection, discovered-from links
- Prevents duplicate tracking systems and confusion

### Quick Start

**Check for ready work:**
```bash
bd ready --json
```

**Create new issues:**
```bash
bd create "Issue title" -t bug|feature|task -p 0-4 --json
bd create "Issue title" -p 1 --deps discovered-from:bd-123 --json
```

**Claim and update:**
```bash
bd update bd-42 --status in_progress --json
bd update bd-42 --priority 1 --json
```

**Complete work:**
```bash
bd close bd-42 --reason "Completed" --json
```

### Issue Types

- `bug` - Something broken
- `feature` - New functionality
- `task` - Work item (tests, docs, refactoring)
- `epic` - Large feature with subtasks
- `chore` - Maintenance (dependencies, tooling)

### Priorities

- `0` - Critical (security, data loss, broken builds)
- `1` - High (major features, important bugs)
- `2` - Medium (default, nice-to-have)
- `3` - Low (polish, optimization)
- `4` - Backlog (future ideas)

### Workflow for AI Agents

1. **Check ready work**: `bd ready` shows unblocked issues
2. **Claim your task**: `bd update <id> --status in_progress`
3. **Work on it**: Implement, test, document
4. **Discover new work?** Create linked issue:
   - `bd create "Found bug" -p 1 --deps discovered-from:<parent-id>`
5. **Complete**: `bd close <id> --reason "Done"`
6. **Commit together**: Always commit the `.beads/issues.jsonl` file together with the code changes so issue state stays in sync with code state

### Auto-Sync

bd automatically syncs with git:
- Exports to `.beads/issues.jsonl` after changes (5s debounce)
- Imports from JSONL when newer (e.g., after `git pull`)
- No manual export/import needed!

### Important Rules

- ✅ Use bd for ALL task tracking
- ✅ Always use `--json` flag for programmatic use
- ✅ Link discovered work with `discovered-from` dependencies
- ✅ Check `bd ready` before asking "what should I work on?"
- ❌ Do NOT create markdown TODO lists
- ❌ Do NOT use external issue trackers
- ❌ Do NOT duplicate tracking systems

For more details, see README.md and QUICKSTART.md.

## Project Structure & Module Organization
Grid uses a Go workspace with module roots under `cmd`, `pkg`, `api`, and `tests`. `cmd/gridapi` hosts the API server with internal subpackages for config, db, repository, server, and state logic, while `cmd/gridctl` contains the CLI command tree. Shared Go libraries live in `pkg/sdk`, generated stubs in `api`, and TypeScript bindings under `js/sdk/gen`. Example Terraform projects sit in `examples`, demo assets in `demo`, and specs or UX notes in `specs/`. Keep generated code out of reviews; source changes belong in the proto or non-gen directories.

## Build, Test, and Development Commands
Use `make build` to compile `gridapi` and `gridctl` into `bin/`. Start the local Postgres dependency with `make db-up`, or reset everything via `make db-reset`. `./bin/gridapi serve --db-url "postgres://grid:gridpass@localhost:5432/grid?sslmode=disable"` runs the API; pair it with `./bin/gridctl state list --server http://localhost:8080` to exercise endpoints. Run `buf generate` whenever you touch files beneath `proto/`, and commit the updated Go and TypeScript artifacts.

## Coding Style & Naming Conventions
Go code follows standard `gofmt` output, tabs over spaces, and exports only when cross-package reuse is required. Keep package paths rooted under `cmd/.../internal` for private logic and `pkg/sdk` for reusable APIs. Name migrations `YYYYMMDDHHMMSS_description.go`, Terraform samples using hyphenated directories, and flags or config keys in kebab-case. TypeScript modules in `js/sdk` should remain ESLint-friendly with camelCase identifiers and default export avoidance.

## Testing Guidelines
Prefer `make test-unit` for fast feedback; it runs race-detected Go tests that avoid external services. Repository tests that hit Postgres live under `cmd/gridapi/internal/repository` and require `make test-unit-db` (which auto-starts Docker). End-to-end coverage is in `tests/integration`; run it with `make test-integration` after building binaries. Name new Go tests with the `TestXxx` pattern in `*_test.go`, and clean up transient fixtures using `t.Cleanup` or `make test-clean`. Contract tests in `tests/contract` are placeholders—note TODOs before adding new cases.

## Commit & Pull Request Guidelines
Follow the existing conventional prefixes (`feat:`, `fix:`, `chore:`, `docs:`) and keep messages imperative and under 72 characters. Reference related issues in the body and mention migrations, proto changes, or new binaries explicitly. PRs should include a concise summary, testing evidence (`make test-unit`, `make test-integration`, etc.), and screenshots or CLI transcripts when behavior changes. Request reviews from domain owners (`gridapi`, `gridctl`, SDK) and ensure generated artifacts and docs stay in sync with code changes.
