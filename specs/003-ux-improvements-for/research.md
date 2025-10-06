# Research: CLI Context-Aware State Management

**Feature**: 003-ux-improvements-for
**Date**: 2025-10-03
**Status**: Complete

## 1. Interactive Terminal Libraries for Go

### Decision
Use **`github.com/pterm/pterm`**

### Rationale
- **Multi-Select support**: Built-in `DefaultInteractiveSelect` and `DefaultInteractiveMultiselect` components
- **Footprint**: Minimal without pulling in a full TUI framework

### Alternatives Considered
- **promptui**: Good for single-select, lacks native multi-select
- **survey**: Deprecated in favor of BubbleTea
- **go-prompt**: Too heavyweight (full TUI framework, not needed)
- **Bubbletea**: Overkill for simple prompts (requires full TUI rewrite)

### Implementation Notes

```golang
// Initialize an empty slice to hold the options
var options []string

// Generate 100 options and add them to the options slice
for i := 0; i < 100; i++ {
    options = append(options, fmt.Sprintf("Option %d", i))
}

// Generate 5 additional options with a specific message and add them to the options slice
for i := 0; i < 5; i++ {
    options = append(options, fmt.Sprintf("You can use fuzzy searching (%d)", i))
}

// Use PTerm's interactive select feature to present the options to the user and capture their selection
// The Show() method displays the options and waits for the user's input
selectedOption, _ := pterm.DefaultInteractiveSelect.WithOptions(options).Show()

// Display the selected option to the user with a green color for emphasis
pterm.Info.Printfln("Selected option: %s", pterm.Green(selectedOption))

// OR Create a new interactive multiselect printer with the options
// Disable the filter and define the checkmark symbols
printer := pterm.DefaultInteractiveMultiselect.
    WithOptions(options).
    WithFilter(false).
    WithCheckmark(&pterm.Checkmark{Checked: pterm.Green("x"), Unchecked: " "})

// Show the interactive multiselect and get the selected options
selectedOptions, _ := printer.Show()

// Print the selected options
pterm.Info.Printfln("Selected options: %s", pterm.Green(selectedOptions))
```

## 2. .grid File Format Design

### Decision
JSON format with the following schema:
```json
{
  "version": "1",
  "state_guid": "01JXXXXX-XXXX-XXXX-XXXX-XXXXXXXXXXXX",
  "state_logic_id": "my-app",
  "server_url": "http://localhost:8080",
  "created_at": "2025-10-03T14:22:00Z",
  "updated_at": "2025-10-03T14:22:00Z"
}
```

### Rationale
- **Human-readable**: JSON is familiar to developers, easy to inspect/debug
- **Git-friendly**: Deterministic field ordering (alphabetical), no whitespace variation
- **Versioning**: `version` field enables future schema migrations
- **Forward-compatible**: Additional fields can be added without breaking v1 readers
- **Logic-id tracking removed**: Simpler to just store current logic_id; changes detected by comparing to server response

### Alternatives Considered
- **YAML**: More human-readable but needs YAML parser dependency
- **TOML**: Less common in Go ecosystem, harder to parse atomically
- **Custom format**: Unnecessary complexity

### Implementation Notes
- Use `encoding/json` with `MarshalIndent` for pretty-printing
- Field order: Serialize to struct with alphabetical JSON tags
- Atomic writes: Write to `.grid.tmp`, then rename (atomic on POSIX)
- Read validation: Check `version == "1"`, validate GUID format

## 3. Output Caching Strategy

### Decision
**Implement server-side output caching in Phase 1** using PostgreSQL `state_outputs` table

### Rationale
- **Cross-state search requirement**: Need to search all outputs across all states quickly (e.g., "find all states with output named 'vpc_id'")
- **Performance**: Parsing every state's Terraform JSON on each search is prohibitively slow for 100+ states
- **PostgreSQL-native**: Use existing database, no additional infrastructure (Redis, etc.)
- **Repository pattern**: Fits existing Bun ORM architecture in `cmd/gridapi/internal/repository`
- **Automatic invalidation**: Update cache on Terraform state PUT via HTTP backend

### Database Schema
```sql
CREATE TABLE state_outputs (
    state_guid UUID NOT NULL REFERENCES states(guid) ON DELETE CASCADE,
    output_key TEXT NOT NULL,
    sensitive BOOLEAN NOT NULL DEFAULT FALSE,
    state_serial BIGINT NOT NULL,  -- TF state version/serial for invalidation
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (state_guid, output_key)
);

CREATE INDEX idx_state_outputs_guid ON state_outputs(state_guid);
CREATE INDEX idx_state_outputs_key ON state_outputs(output_key);  -- For cross-state search
```

### Implementation Pattern
**Repository Interface** (`internal/repository/interface.go`):
```go
type StateOutputRepository interface {
    // UpsertOutputs replaces all outputs for a state (atomic update)
    UpsertOutputs(ctx context.Context, stateGUID string, serial int64, outputs []OutputKey) error

    // GetOutputsByState returns all outputs for a state
    GetOutputsByState(ctx context.Context, stateGUID string) ([]OutputKey, error)

    // SearchOutputsByKey finds all states with output matching key
    SearchOutputsByKey(ctx context.Context, outputKey string) ([]StateOutputRef, error)
}
```

**Bun ORM Model** (`internal/db/models/state_output.go`):
```go
type StateOutput struct {
    bun.BaseModel `bun:"table:state_outputs"`

    StateGUID   string    `bun:"state_guid,pk,type:uuid"`
    OutputKey   string    `bun:"output_key,pk,type:text"`
    Sensitive   bool      `bun:"sensitive,notnull"`
    StateSerial int64     `bun:"state_serial,notnull"`
    CreatedAt   time.Time `bun:"created_at,notnull,default:now()"`
    UpdatedAt   time.Time `bun:"updated_at,notnull,default:now()"`
}
```

**Migration** (`cmd/gridapi/internal/migrations/YYYYMMDDHHMMSS_add_state_outputs_cache.go`):
```go
func (m *Migration_YYYYMMDDHHMMSS) Up(ctx context.Context, db *bun.DB) error {
    _, err := db.ExecContext(ctx, `
        CREATE TABLE state_outputs (
            state_guid UUID NOT NULL REFERENCES states(guid) ON DELETE CASCADE,
            output_key TEXT NOT NULL,
            sensitive BOOLEAN NOT NULL DEFAULT FALSE,
            state_serial BIGINT NOT NULL,
            created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
            updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
            PRIMARY KEY (state_guid, output_key)
        );
        CREATE INDEX idx_state_outputs_guid ON state_outputs(state_guid);
        CREATE INDEX idx_state_outputs_key ON state_outputs(output_key);
    `)
    return err
}
```

### Invalidation Strategy
When Terraform HTTP backend PUT updates state:
1. Parse new state JSON serial/version
2. If `serial != cached_serial`: Delete old outputs, insert new outputs
3. Transaction: Update `states.data` + refresh `state_outputs` atomically

**Hook in HTTP backend handler**:
```go
func (s *Server) HandleTerraformStatePUT(w http.ResponseWriter, r *http.Request) {
    // ... existing state write logic ...

    // Parse outputs from new state JSON
    outputs, _ := tfstate.ParseOutputKeys(newStateJSON)

    // Update cache (transactionally with state write)
    s.repo.UpsertOutputs(ctx, stateGUID, newSerial, outputs)
}
```

### Alternatives Considered
- **On-demand parsing**: Too slow for cross-state searches (100+ states × parse time)
- **Redis cache**: Adds infrastructure complexity, Grid prefers PostgreSQL-only stack
- **In-memory cache**: Doesn't persist across server restarts, complicates multi-instance deployments
- **Materialized views**: Harder to keep synchronized with JSONB state data

## 4. Concurrency Control for .grid File Writes

### Decision
**Atomic write with error-on-conflict** (no file locking)

### Rationale
- **Cross-platform**: File locking (`flock`) behaves differently on Windows vs POSIX
- **Simplicity**: Atomic writes via temp file + rename is standard Go pattern
- **Conflict detection**: If .grid exists and has different GUID, return error per FR-006d ("first write wins")
- **Race window**: Tiny race between read-check and write, acceptable for local CLI use

### Implementation Pattern
```go
func WriteGridContext(ctx DirectoryContext) error {
    // 1. Check if .grid exists with different GUID
    existing, err := ReadGridContext()
    if err == nil && existing.StateGUID != ctx.StateGUID {
        return fmt.Errorf(".grid exists for different state; use --force to overwrite")
    }

    // 2. Write to temp file
    tmpPath := ".grid.tmp"
    data, _ := json.MarshalIndent(ctx, "", "  ")
    os.WriteFile(tmpPath, data, 0644)

    // 3. Atomic rename
    os.Rename(tmpPath, ".grid")
}
```

### Alternatives Considered
- **File locking (flock)**: Platform-specific, complex error handling
- **Lock file (.grid.lock)**: Leftover locks if process crashes
- **Optimistic locking**: Requires version/ETag in .grid file, over-engineered

### Edge Case Handling
- **Concurrent creates**: Second process gets error, must use `--force`
- **Stale .grid**: If state deleted from server, CLI detects on next command and prompts to re-run `state create`

## 5. Non-Interactive Mode Patterns

### Decision
Global `--non-interactive` flag that:
- Suppresses all prompts
- Errors immediately if prompt would occur
- Exit code 1 with clear message: "Cannot prompt in non-interactive mode. Specify --output explicitly."

### Rationale
Surveyed patterns from:
- **Terraform**: Uses `TF_INPUT=false` env var + `-input=false` flag
- **kubectl**: No built-in prompts; requires explicit confirmation flags (e.g., `--yes`)
- **Ansible**: `--non-interactive` flag for playbooks
- **GitHub CLI**: `--yes` flag to skip confirmations

### Implementation
- Add `--non-interactive` to root command (`gridctl --non-interactive deps add ...`)
- Store in cobra persistent flag, check before calling `survey.AskOne`
- Environment variable fallback: `GRID_NON_INTERACTIVE=1`

```go
var nonInteractive bool

func init() {
    rootCmd.PersistentFlags().BoolVar(&nonInteractive, "non-interactive", false, "Disable interactive prompts")
    if os.Getenv("GRID_NON_INTERACTIVE") == "1" {
        nonInteractive = true
    }
}

func promptMultiSelect(options []string) ([]string, error) {
    if nonInteractive {
        return nil, fmt.Errorf("cannot prompt in non-interactive mode")
    }
    // ... survey.MultiSelect logic
}
```

### Exit Codes
- **0**: Success
- **1**: User error (missing required flag in non-interactive mode)
- **2**: Server error (network, not found, etc.)

### Alternatives Considered
- **Assume defaults**: Dangerous (might create wrong dependencies)
- **Require --yes flag**: Doesn't help with multi-select (no single default)
- **Silent failure**: Terrible UX, no feedback

## Summary of Decisions

| Research Area | Decision | Key Trade-Off |
|---------------|----------|---------------|
| Terminal UI Library | survey/v2 | Native multi-select vs simpler promptui |
| .grid File Format | JSON (version 1 schema) | Human-readable vs binary performance |
| Output Caching | On-demand parsing (Phase 1) | Simplicity vs <200ms latency goal |
| Concurrency Control | Atomic write + error-on-conflict | Simplicity vs robust locking |
| Non-Interactive Mode | `--non-interactive` flag + env var | Explicit errors vs implicit defaults |

All decisions align with Constitution Principle VII (Simplicity & Pragmatism). Caching deferred until performance pain demonstrated.
