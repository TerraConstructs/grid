# E2E Testing Notes

## ✅ Cache Refresh Signal Handling (IMPLEMENTED)

**Context:**
E2E tests may need to update group-to-role mappings during test execution. The gridapi IAM service has a cache for these mappings that refreshes every 5 minutes.

**Solution:**
The `gridapi serve` command now listens for SIGHUP and triggers an immediate IAM cache refresh.

**Implementation:**
- `cmd/gridapi/cmd/serve.go`: Added SIGHUP signal handler alongside existing shutdown signals
- When SIGHUP received, calls `iamService.RefreshGroupRoleCache(ctx)`
- Logs the cache refresh event with version and group count

**Usage in E2E Tests:**
```bash
# Test helpers that modify group-to-role mappings can send SIGHUP to gridapi
kill -HUP $(cat /tmp/grid-e2e-gridapi.pid)

# Or use the helper function (to be created in tests/e2e/helpers/keycloak.helpers.ts)
```

**Example Scenario:**
1. Test starts with alice@example.com in product-engineers group (env=dev access)
2. Test needs to verify permission change
3. Helper function adds alice to platform-engineers group via Keycloak API
4. Helper sends `kill -HUP` to gridapi PID
5. Cache refreshes immediately (logs show version increment)
6. Test continues with new permissions active

**Related Files:**
- Signal handler: `cmd/gridapi/cmd/serve.go:277-300`
- IAM service cache: `cmd/gridapi/internal/services/iam/group_role_cache.go`
- HTTP refresh endpoint: `cmd/gridapi/internal/server/handlers/auth.go`
- E2E setup: `tests/e2e/setup/start-services.sh` (stores PID in `/tmp/grid-e2e-gridapi.pid`)
