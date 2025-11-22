# E2E Testing TODOs

## Cache Refresh Signal Handling (Priority: Medium)

**Context:**
E2E tests may need to update group-to-role mappings during test execution. The gridapi IAM service has a cache for these mappings that refreshes every 5 minutes.

Currently, tests would need to either:
1. Wait 5 minutes for automatic refresh
2. Hit the `/auth/cache/refresh` endpoint (requires credentials)

**Proposed Solution:**
Add Unix signal handling to `gridapi serve` command to trigger IAM cache refresh on demand.

**Implementation:**
1. In `cmd/gridapi/cmd/serve.go`, set up signal channel for SIGHUP
2. When SIGHUP received, call the IAM service's cache refresh method (same logic as the HTTP endpoint handler)
3. Log the cache refresh event

**Usage in E2E Tests:**
```bash
# Test helpers that modify group-to-role mappings can send SIGHUP to gridapi
kill -HUP $(cat /tmp/grid-e2e-gridapi.pid)
```

**Related Files:**
- IAM service cache: `cmd/gridapi/internal/services/iam/group_role_cache.go`
- HTTP refresh endpoint: `cmd/gridapi/internal/server/handlers/auth.go` (CacheRefreshHandler)
- E2E setup: `tests/e2e/setup/start-services.sh` (stores PID in `/tmp/grid-e2e-gridapi.pid`)

**Workaround Until Implemented:**
For e2e tests that need to change permissions, either:
- Set up all permission variations before starting gridapi
- Accept 5-minute cache TTL and design tests accordingly
- Use the HTTP refresh endpoint with test credentials
