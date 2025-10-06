# Quickstart: CLI Context-Aware State Management

**Feature**: 003-ux-improvements-for
**Date**: 2025-10-03
**Purpose**: Validate end-to-end UX for directory context, interactive prompts, and enhanced state display

---

## Prerequisites

- Grid API server running at `http://localhost:8080`
- `gridctl` binary built from this feature branch
- Empty test directories for isolation

```bash
# ensure database is up and schemas migrated
make db-up
./bin/gridapi db init --db-url "postgres://grid:gridpass@localhost:5432/grid?sslmode=disable"
./bin/gridapi db migrate --db-url "postgres://grid:gridpass@localhost:5432/grid?sslmode=disable"

# Start Grid API server
./bin/gridapi serve --server-addr localhost:8080 --db-url "postgres://grid:gridpass@localhost:5432/grid?sslmode=disable"

# Build CLI from feature branch
make build
```

---

## Scenario 1: Create State with Directory Context

**Goal**: Verify `.grid` file creation and automatic context loading

```bash
# 1. Create fresh directory
mkdir -p /tmp/grid-test/frontend
cd /tmp/grid-test/frontend

# 2. Create state (auto-generates .grid file)
gridctl state create frontend-app

# Expected output:
# Created state: 01JXXX-XXXX-XXXX-XXXX-XXXXXXXXXXXX
# Logic ID: frontend-app
#
# Terraform HTTP Backend endpoints:
#   Address: http://localhost:8080/tfstate/01JXXX...
#   Lock:    http://localhost:8080/tfstate/01JXXX.../lock
#   Unlock:  http://localhost:8080/tfstate/01JXXX.../unlock

# 3. Verify .grid file created
cat .grid

# Expected output (JSON):
# {
#   "created_at": "2025-10-03T14:22:00Z",
#   "server_url": "http://localhost:8080",
#   "state_guid": "01JXXX-XXXX-XXXX-XXXX-XXXXXXXXXXXX",
#   "state_logic_id": "frontend-app",
#   "updated_at": "2025-10-03T14:22:00Z",
#   "version": "1"
# }

# 4. Test automatic context loading (no --logic-id flag needed)
gridctl state get

# Expected output:
# State: frontend-app (guid: 01JXXX...)
# Created: 2025-10-03 14:22:00
#
# Dependencies (consuming from): (none)
# Dependents (consumed by): (none)
# Outputs: (none - no Terraform state uploaded yet)

# 5. Try creating another state in same directory (should error)
gridctl state create another-app

# Expected error:
# Error: .grid exists for state frontend-app; use --force to overwrite

# 6. Force overwrite to change state
gridctl state create --force backend-app

# Expected: New .grid file with backend-app GUID

cat .grid | grep state_logic_id

# Expected: "state_logic_id": "backend-app"
```

**Test Assertions**:
- `.grid` file exists after `state create`
- `.grid` contains valid JSON with required fields (version, state_guid, state_logic_id)
- `state get` works without specifying `--logic-id` (uses .grid context)
- Second `state create` without `--force` returns error
- `--force` flag allows overwriting .grid

---

## Scenario 2: Interactive Output Selection for Dependencies

**Goal**: Verify multi-select prompt for outputs when creating dependencies

```bash
# Setup: Create networking state with mock Terraform outputs
cd /tmp/grid-test
mkdir networking
cd networking

gridctl state create networking

# Simulate Terraform state with outputs (manual API call for testing)
# In production, this would come from terraform apply uploading state
curl -X PUT http://localhost:8080/tfstate/$(cat .grid | jq -r .state_guid) \
  -H "Content-Type: application/json" \
  -d '{
    "version": 4,
    "terraform_version": "1.6.0",
    "serial": 1,
    "outputs": {
      "vpc_id": {"value": "vpc-abc123", "type": "string", "sensitive": false},
      "subnet_id": {"value": "subnet-xyz789", "type": "string", "sensitive": false},
      "security_group_id": {"value": "sg-def456", "type": "string", "sensitive": false},
      "db_endpoint": {"value": "db.example.com", "type": "string", "sensitive": false},
      "db_password": {"value": "secret123", "type": "string", "sensitive": true}
    }
  }'

# Now create frontend-app state and add dependency
cd /tmp/grid-test/frontend  # Directory with frontend-app .grid file

# Run deps add with only --from (will prompt for output selection)
gridctl deps add --from networking

# Expected interactive prompt:
# ? Select outputs to create dependencies:
#   ☐ db_endpoint
#   ☐ db_password (⚠️  sensitive)
#   ☐ security_group_id
#   ☐ subnet_id
#   ☐ vpc_id
# (Use arrow keys to navigate, space to select, enter to confirm)

# User selects: vpc_id, subnet_id (press space on each, then enter)

# Expected output:
# Dependency added: networking.vpc_id -> frontend-app as 'vpc_id' (edge ID: 1)
# Dependency added: networking.subnet_id -> frontend-app as 'subnet_id' (edge ID: 2)

# Verify dependencies created
gridctl deps list

# Expected output:
# From             Output      To            Input Name  Status  Edge ID
# networking       vpc_id      frontend-app  vpc_id      pending 1
# networking       subnet_id   frontend-app  subnet_id   pending 2
```

**Test Assertions**:
- `deps add --from <state>` fetches outputs from from-state
- Interactive multi-select prompt displays all output keys
- Sensitive outputs marked with "⚠️  sensitive" warning
- Selecting multiple outputs creates multiple dependency edges
- Default `--to` value inferred from `.grid` file

---

## Scenario 3: Single Output Auto-Select (No Prompt)

**Goal**: Verify automatic selection when from-state has only 1 output

```bash
# Setup: Create state with single output
cd /tmp/grid-test
mkdir database
cd database

gridctl state create database

# Upload Terraform state with 1 output
curl -X PUT http://localhost:8080/tfstate/$(cat .grid | jq -r .state_guid) \
  -H "Content-Type: application/json" \
  -d '{
    "version": 4,
    "outputs": {
      "connection_string": {"value": "postgres://...", "type": "string", "sensitive": true}
    }
  }'

# Add dependency from database to frontend-app (no prompt expected)
cd /tmp/grid-test/frontend

gridctl deps add --from database

# Expected output (NO PROMPT):
# Auto-selected single output: connection_string
# Dependency added: database.connection_string -> frontend-app as 'connection_string' (edge ID: 3)

# Verify
gridctl deps list | grep database

# Expected:
# database  connection_string  frontend-app  connection_string  pending  3
```

**Test Assertions**:
- No interactive prompt when from-state has exactly 1 output
- Auto-selection message displayed
- Dependency edge created successfully

---

## Scenario 4: Zero Outputs (Mock Dependency)

**Goal**: Verify mock dependency creation when from-state has no outputs

```bash
# Setup: Create state with no Terraform state uploaded
cd /tmp/grid-test
mkdir cache
cd cache

gridctl state create cache
# (Do NOT upload Terraform state - no outputs exist)

# Try adding dependency from cache (which has no outputs)
cd /tmp/grid-test/frontend

gridctl deps add --from cache

# Expected behavior (based on FR-015):
# Warning: State 'cache' has no outputs. Creating mock dependency.
# ? Enter mock value JSON (or press enter to skip):
# (User presses enter to skip)
# Dependency added: cache.(no-output) -> frontend-app (mock) (edge ID: 4)

# Alternative with explicit mock value:
gridctl deps add --from cache --mock '{"value": "redis://localhost"}'

# Expected:
# Dependency added: cache.(no-output) -> frontend-app (mock) (edge ID: 5)
```

**Test Assertions**:
- CLI allows creating dependency even when from-state has no outputs
- Mock value prompt displayed
- Mock dependency edge created with status "mock"

---

## Scenario 5: Enhanced State Info Display

**Goal**: Verify comprehensive state information includes dependencies, dependents, outputs

```bash
# View frontend-app state (now has 3 dependencies)
cd /tmp/grid-test/frontend

gridctl state get

# Expected output:
# State: frontend-app (guid: 01JXXX...)
# Created: 2025-10-03 14:22:00
# Updated: 2025-10-03 14:25:30
#
# Dependencies (consuming from):
#   networking.vpc_id → vpc_id
#   networking.subnet_id → subnet_id
#   database.connection_string → connection_string (⚠️  sensitive)
#
# Dependents (consumed by):
#   (none)
#
# Outputs:
#   (none - no Terraform state uploaded yet)

# Now upload Terraform state for frontend-app
curl -X PUT http://localhost:8080/tfstate/$(cat .grid | jq -r .state_guid) \
  -H "Content-Type: application/json" \
  -d '{
    "version": 4,
    "outputs": {
      "app_url": {"value": "https://app.example.com", "type": "string", "sensitive": false},
      "api_key": {"value": "sk-abc123", "type": "string", "sensitive": true}
    }
  }'

# View again with outputs populated
gridctl state get

# Expected output:
# State: frontend-app (guid: 01JXXX...)
# Created: 2025-10-03 14:22:00
# Updated: 2025-10-03 14:26:15
#
# Dependencies (consuming from):
#   networking.vpc_id → vpc_id
#   networking.subnet_id → subnet_id
#   database.connection_string → connection_string (⚠️  sensitive)
#
# Dependents (consumed by):
#   (none)
#
# Outputs:
#   app_url
#   api_key (⚠️  sensitive)

# Create dependent state to verify dependents display
cd /tmp/grid-test
mkdir monitoring
cd monitoring

gridctl state create monitoring

gridctl deps add --from frontend-app
# (Interactive prompt, select app_url)

# Now view networking state (should show dependents)
cd /tmp/grid-test/networking

gridctl state get

# Expected output:
# State: networking (guid: 01JYYY...)
# Created: 2025-10-03 14:20:00
#
# Dependencies (consuming from):
#   (none)
#
# Dependents (consumed by):
#   frontend-app.vpc_id
#   frontend-app.subnet_id
#
# Outputs:
#   vpc_id
#   subnet_id
#   security_group_id
#   db_endpoint
#   db_password (⚠️  sensitive)
```

**Test Assertions**:
- `state get` displays dependencies (incoming edges)
- `state get` displays dependents (outgoing edges)
- `state get` displays outputs (from Terraform state JSON)
- Sensitive outputs/dependencies marked with warning icon
- Output includes timestamps (created_at, updated_at)

---

## Scenario 6: Non-Interactive Mode (CI/Automation)

**Goal**: Verify `--non-interactive` flag behavior

```bash
# Create new test directory
cd /tmp/grid-test
mkdir ci-test
cd ci-test

gridctl state create ci-app

# Try deps add without --output in non-interactive mode (should fail)
gridctl deps add --from networking --non-interactive

# Expected error:
# Error: Cannot prompt in non-interactive mode. Specify --output explicitly or provide single-output state.

# Correct usage with explicit --output
gridctl deps add --from networking --output vpc_id --non-interactive

# Expected output:
# Dependency added: networking.vpc_id -> ci-app as 'vpc_id' (edge ID: 6)

# Alternative: Use environment variable
export GRID_NON_INTERACTIVE=1

gridctl deps add --from networking --output subnet_id

# Expected: Same as --non-interactive flag
```

**Test Assertions**:
- `--non-interactive` flag suppresses all prompts
- Commands requiring prompts error with clear message
- Explicit flags (`--output`) bypass prompt requirement
- `GRID_NON_INTERACTIVE=1` env var works as flag alternative
- Exit code 1 for user errors, 2 for server errors

---

## Scenario 7: Corrupted .grid File Handling

**Goal**: Verify graceful handling of corrupted directory context

```bash
cd /tmp/grid-test/frontend

# Corrupt .grid file
echo "not valid json{{{" > .grid

# Try to use context
gridctl state get

# Expected:
# Warning: .grid file corrupted or invalid, ignoring
# Error: State identifier required (no context found)

# Should prompt for explicit --logic-id
gridctl state get --logic-id frontend-app

# Expected: Works normally (queries server directly)
```

**Test Assertions**:
- Corrupted `.grid` triggers warning log
- CLI continues without crashing
- Commands require explicit parameters when `.grid` invalid

---

## Scenario 8: Write Permission Denied

**Goal**: Verify handling when directory is read-only

```bash
cd /tmp/grid-test
mkdir readonly
cd readonly

# Make directory read-only
chmod 555 .

# Try to create state
gridctl state create readonly-app

# Expected:
# Warning: Cannot write .grid file (permission denied), state context will not be saved
# Created state: 01JZZZ...
# (Backend config printed normally)

# Verify state created on server despite .grid failure
gridctl state list | grep readonly-app

# Expected:
# readonly-app  01JZZZ...  false  2025-10-03 14:30:00

# Restore permissions
chmod 755 /tmp/grid-test/readonly
```

**Test Assertions**:
- Write permission detected early
- Warning displayed to user
- State creation succeeds despite .grid failure
- Subsequent commands require explicit `--logic-id`

---

## Cleanup

```bash
# Stop Grid API server
# Ctrl+C in server terminal

# Remove test directories
rm -rf /tmp/grid-test

# Reset database (optional)
make db-reset
```

---

## Integration Test Automation

These scenarios should be automated as integration tests in `tests/integration/context_aware_test.go`:

```go
func TestDirectoryContextCreation(t *testing.T) { ... }
func TestInteractiveOutputSelection(t *testing.T) { ... }  // Mock terminal I/O
func TestSingleOutputAutoSelect(t *testing.T) { ... }
func TestEnhancedStateInfoDisplay(t *testing.T) { ... }
func TestNonInteractiveMode(t *testing.T) { ... }
func TestCorruptedGridFile(t *testing.T) { ... }
func TestWritePermissionDenied(t *testing.T) { ... }
```

**Test Isolation**: Each test creates isolated temp directory, generates unique logic-ids, and cleans up after completion.

**Mock Terminal**: Use `github.com/Netflix/go-expect` for simulating terminal I/O in interactive prompt tests.

---

**Validation Criteria**: All scenarios complete successfully without errors. Integration tests pass in CI.
