# Phase 7: Cache Refresh & Admin API

**Priority**: P2
**Effort**: 4-6 hours
**Risk**: Low
**Dependencies**: Phase 6 complete

## Objectives

- Automatic cache refresh (background goroutine)
- Manual refresh via admin API
- Operational flexibility

## Tasks

### Task 7.1: Background Refresh Goroutine

**File**: `cmd/serve.go`

```go
func runServer(cmd *cobra.Command, args []string) error {
    // ... initialization ...

    // Start background cache refresh
    refreshInterval := 5 * time.Minute // TODO: Make configurable
    refreshTicker := time.NewTicker(refreshInterval)
    defer refreshTicker.Stop()

    go func() {
        for range refreshTicker.C {
            log.Println("Refreshing group→role cache...")
            if err := iamService.RefreshGroupRoleCache(context.Background()); err != nil {
                log.Printf("ERROR: Cache refresh failed: %v", err)
            } else {
                snapshot := iamService.GetGroupRoleCacheSnapshot()
                log.Printf("INFO: Cache refreshed: version=%d, groups=%d", 
                    snapshot.Version, len(snapshot.Mappings))
            }
        }
    }()

    // ... start server ...
}
```

**Configuration**:
- Add `CACHE_REFRESH_INTERVAL` env var (default: 5m)
- Parse duration in config

### Task 7.2: Admin API for Manual Refresh

**File**: `server/admin_handlers.go` (new)

```go
// POST /admin/cache/refresh
func HandleCacheRefresh(iamService iam.Service) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        ctx := r.Context()

        // Check admin permission
        principal, ok := auth.GetUserFromContext(ctx)
        if !ok {
            http.Error(w, "Unauthorized", 401)
            return
        }

        allowed, _ := iamService.Authorize(ctx, principal, "admin", "cache:refresh", nil)
        if !allowed {
            http.Error(w, "Forbidden", 403)
            return
        }

        // Refresh cache
        if err := iamService.RefreshGroupRoleCache(ctx); err != nil {
            http.Error(w, fmt.Sprintf("Refresh failed: %v", err), 500)
            return
        }

        snapshot := iamService.GetGroupRoleCacheSnapshot()
        json.NewEncoder(w).Encode(map[string]interface{}{
            "status":  "success",
            "version": snapshot.Version,
            "groups":  len(snapshot.Mappings),
            "timestamp": snapshot.CreatedAt,
        })
    }
}
```

**Mount in router**:
```go
r.Post("/admin/cache/refresh", HandleCacheRefresh(iamService))
```

### Task 7.3: Automatic Refresh on Admin Operations

**Already implemented in Phase 2** - `AssignGroupRole()` and `RemoveGroupRole()` call `RefreshGroupRoleCache()` automatically.

Verify this works:
1. Call `AssignGroupRole()`
2. Verify cache version incremented
3. Verify new mapping visible immediately

## Deliverables

- [ ] Background refresh goroutine running
- [ ] Admin API endpoint created
- [ ] Cache refresh logs visible in server output
- [ ] Manual refresh tested via `curl`

## Testing

```bash
# Start server
./bin/gridapi serve

# Verify background refresh logs (every 5 minutes)
# Should see: "INFO: Cache refreshed: version=2, groups=..."

# Manual refresh
curl -X POST http://localhost:8080/admin/cache/refresh \
  -H "Authorization: Bearer <admin-token>"

# Should return: {"status":"success","version":3,"groups":5}
```

## Related Documents

- **Previous**: [phase-6-handler-refactor.md](phase-6-handler-refactor.md)
- **Next**: [phase-8-testing.md](phase-8-testing.md)
