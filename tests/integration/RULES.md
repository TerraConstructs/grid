Integration Test Rules

This documents documents existing patterns inthe /Users/vincentdesmet/tcons/grid/tests/integration directory.

1. TestMain Setup Pattern (main_test.go)

The project uses a centralized TestMain setup pattern for managing the entire test lifecycle:

Key features:
- Pre-flight checks: Prevents leftover gridapi servers from interfering (checks port 8080 and kills stray processes)
- Server startup: Starts gridapi in background as separate process with exec.Command
- Server wait: Polls /health endpoint with 30-second timeout and 500ms intervals
- Bootstrap: Creates group→role mappings for Mode 1 authentication via exec of gridapi iam bootstrap
- Process management: Uses SIGTERM for graceful shutdown with 5-second timeout before force kill
- Environment inheritance: Passes through all parent environment variables including OIDC config

Signal handling:
- Uses syscall.SIGTERM for graceful shutdown
- Uses syscall.SIGHUP to trigger cache refresh after bootstrap (line 248)
- Waits with time.Sleep(500 * time.Millisecond) after SIGHUP to allow server to process signal

2. Test Interaction Patterns

Tests interact with the server through multiple approaches:

A. CLI Execution via helpers.go:
- runGridctl(): Executes gridctl CLI commands with timeout context
- runGridctlStdOut(): Captures only stdout output
- All gridctl invocations append --server http://localhost:8080
- Supports working directory override for test isolation

B. SDK Client (pkg/sdk):
- Created via newSDKClient() which instantiates sdk.NewClient(serverURL)
- Used for programmatic state creation, dependency management, output handling
- Example: client.CreateState(), client.ListStateOutputs(), client.AddDependency()

C. Direct HTTP Calls:
- Used for authentication/authorization testing (auth_mode1_test.go)
- Enables testing token validation at protocol level before business logic
- Examples: POST to Keycloak token endpoint, GET/POST to Connect RPC endpoints

D. Terraform/OpenTofu Execution:
- Via github.com/hashicorp/terraform-exec/tfexec package
- Tests full workflow: init → plan → apply → show → destroy
- Validates state persistence in remote backend

3. Cleanup & Isolation

Per-test isolation:
- Uses t.TempDir() for test-specific temporary directories (auto-cleanup)
- Each test generates unique logic IDs using UUID or Unix timestamps (lines 29, 64, 120, 165)
- Example: fmt.Sprintf("test-ctx-%d", time.Now().UnixNano())

No explicit per-test database cleanup:
- Tests rely on unique logic IDs to avoid conflicts
- No truncation tables between tests (unlike repository tests that use t.Cleanup())
- Tests are independent - each creates own state with unique identifier

Server-level lifecycle:
- Single gridapi instance runs for entire test suite
- Started in TestMain before any tests run
- Stopped after all tests complete
- PostgreSQL and Keycloak run for duration of test suite

Current directory handling:
- Tests save original directory and defer restore (line 35-37 in context_aware_test.go)
- Allows safe chdir to temp directories without affecting other tests

4. Time-Dependent Features Testing

Async/Race Condition Testing (output_inference_race_test.go):
- TestInferenceDoesNotResurrectRemovedOutput: Tests race between async inference and state removal
  - POST A (serial=10) → Sleep 50ms → POST B (serial=11) → Sleep 200ms
  - Validates serial monotonicity prevents late inference from overwriting newer state

Timed waits:
- time.Sleep(50 * time.Millisecond) - Let inference START but NOT complete
- time.Sleep(200 * time.Millisecond) - Wait for inference to complete
- time.Sleep(500 * time.Millisecond) - Full inference completion
- time.Sleep(10 * time.Millisecond) - Minimal delay for rapid POSTs

Timeout management:
- Tests use context.WithTimeout() for all operations (30s-90s typical)
- Lock conflict test (line 24): 90-second test timeout
- Individual operations: 5-10 second timeouts
- Lock acquisition wait: 3 seconds before attempting conflict scenario
- Lock release wait: 2 seconds before retry

Concurrent operation testing (lock_conflict_test.go):
- Spawns goroutines with go func() to simulate concurrent Terraform operations
- Uses channels make(chan error, 1) to track completion
- Tests Terraform locking: slow operation holds lock, other operation blocked
- Verifies lock timeout (tfexec.LockTimeout("5s"))

5. Test File Organization

Core test files (32 files total):

| File                          | Purpose                                                |
|-------------------------------|--------------------------------------------------------|
| main_test.go                  | TestMain, server lifecycle, bootstrap                  |
| helpers.go                    | CLI/HTTP/SDK execution helpers                         |
| auth_mode1_auth_helpers.go    | Keycloak authentication flows                          |
| auth_mode1_infrastructure.go  | Keycloak discovery, health checks                      |
| auth_mode1_rbac_helpers.go    | Group-role mapping helpers                             |
| auth_mode1_test.go            | Mode 1 auth tests (token validation, SSO, device flow) |
| auth_mode2_test.go            | Mode 2 auth tests (internal IdP)                       |
| quickstart_test.go            | Terraform/OpenTofu workflows                           |
| context_aware_test.go         | .grid file context, directory-based state              |
| dependency_test.go            | State dependency graph, cycle prevention               |
| output_inference_race_test.go | Async inference race conditions                        |
| output_inference_test.go      | Schema inference for state outputs                     |
| output_schema_test.go         | Output schema validation                               |
| lock_conflict_test.go         | Terraform state locking                                |
| size_warning_test.go          | State size limits                                      |
| restart_persistence_test.go   | State persistence across server restart                |
| duplicate_logic_id_test.go    | Duplicate state prevention                             |
| not_found_test.go             | 404 error handling                                     |
| labels_test.go                | State labeling                                         |
| output_validation_test.go     | Output type validation                                 |

Helper files:
- Auth helpers split into 3 files for Mode 1 (9KB + 25KB + 2.6KB)
- Infrastructure, auth, and RBAC helpers separated by concern
- Each provides focused functionality for tests

6. Test Patterns Summary

Pattern 1: Short mode skipping
if testing.Short() {
    t.Skip("Skipping integration test in short mode")
}
All integration tests include this at the beginning.

Pattern 2: Unique ID generation
logicID := fmt.Sprintf("test-prefix-%s", uuid.New().String()[:8])
// or
logicID := fmt.Sprintf("test-ctx-%d", time.Now().UnixNano())

Pattern 3: Timeout contexts
ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
defer cancel()

Pattern 4: Test assertion style
- Uses github.com/stretchr/testify/require for fail-fast assertions
- Uses github.com/stretchr/testify/assert for non-fatal checks
- Example: require.NoError(t, err) vs assert.Error(t, err)

Pattern 5: Cleanup deferral
originalDir, err := os.Getwd()
require.NoError(t, err)
defer func() { _ = os.Chdir(originalDir) }()

Pattern 6: Best-effort cleanup
cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cleanupCancel()
_ = tf1.Destroy(cleanupCtx)  // Ignore errors
_ = tf2.Destroy(cleanupCtx)

---
File Paths (absolute):
- /Users/vincentdesmet/tcons/grid/tests/integration/main_test.go
- /Users/vincentdesmet/tcons/grid/tests/integration/helpers.go
- /Users/vincentdesmet/tcons/grid/tests/integration/auth_mode1_auth_helpers.go
- /Users/vincentdesmet/tcons/grid/tests/integration/auth_mode1_infrastructure.go
- /Users/vincentdesmet/tcons/grid/tests/integration/auth_mode1_test.go
- /Users/vincentdesmet/tcons/grid/tests/integration/output_inference_race_test.go
- /Users/vincentdesmet/tcons/grid/tests/integration/lock_conflict_test.go
- /Users/vincentdesmet/tcons/grid/tests/integration/context_aware_test.go (additional 20KB for context/dep patterns)
