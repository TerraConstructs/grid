# Casbin Bun Adapter: Implementation Notes

## Overview

Grid uses a custom Casbin adapter based on `github.com/msales/casbin-bun-adapter` v1.0.7, modified to work with schema-less database configurations (SQLite and PostgreSQL public schema).

**Location:** `cmd/gridapi/internal/auth/bunadapter/adapter.go`

## Key Modifications from Upstream

### 1. Removed Schema Qualifiers
- **Reason:** Original adapter hard-coded PostgreSQL schema names, incompatible with SQLite
- **Change:** Removed schema prefixes from table references
- **Impact:** Adapter now works with both PostgreSQL and SQLite

### 2. Removed ID Field and Random Number Generator
- **Reason:** Simplified primary key strategy
- **Change:** Use composite primary key on all fields (ptype, v0, v1, v2, v3, v4, v5)
- **Impact:** Idempotent policy insertion with `ON CONFLICT DO NOTHING`

### 4. Support Empty Fields in Policies

- **Reason:** Original adapter did not support empty fields in policies
- **Change:** Modified `String()` to include empty strings up to last non-empty field:
- **Impact:** Policies with empty fields are now correctly stored and retrieved

References:
- [casbin/casbin Issue 316](https://github.com/casbin/casbin/issues/316)

### 3. Fixed LoadPolicy for Scope Expressions (Critical Bug Fix)

**Problem:** Original implementation serialized policies to CSV strings, then parsed them back:

```go
// BUGGY: Original code
for _, r := range rules {
    persist.LoadPolicyLine(r.String(), model)  // CSV serialization roundtrip
}
```

This broke when scope expressions contained special characters like quotes:
- Policy: `role:product-engineer, state, state:create, env == "dev", allow`
- CSV serialization: `p, role:product-engineer, state, state:create, env == "dev", allow`
- CSV parsing: Chokes on quotes in `env == "dev"`, corrupts policy

**Fix:** Pass value slice directly to Casbin, bypassing CSV:

```go
// FIXED: Current code (adapter.go:44-50)
for _, r := range rules {
    values, lastNonEmpty := r.toValueSlice()
    if lastNonEmpty == -1 {
        continue // skip empty rule
    }
    model.AddPolicy(r.Ptype, r.Ptype, values[:lastNonEmpty+1])  // Direct slice
}
```

**Impact:**
- ✅ Correctly handles bexpr scope expressions with quotes, commas, operators
- ✅ No string parsing errors
- ✅ Better performance (no string allocation/parsing)

## AutoSave Behavior

### Enabled in Production
**Location:** `cmd/gridapi/cmd/serve.go:111`

```go
enforcer.EnableAutoSave(true)
```

### What AutoSave Does
When enabled, every policy modification immediately persists to the database:
- `enforcer.AddGroupingPolicy()` → `adapter.AddPolicy()` → `INSERT INTO casbin_rules`
- `enforcer.DeleteRoleForUser()` → `adapter.RemovePolicy()` → `DELETE FROM casbin_rules`
- No manual `SavePolicy()` calls required

### AutoSave Implementation
The adapter implements Casbin's AutoSave interface:

```go
// adapter.go:68-77
func (a *Adapter) AddPolicy(_ string, ptype string, rule []string) error {
    r := newCasbinRule(ptype, rule)
    if err := a.save(false, r); err != nil {  // false = append, don't truncate
        return fmt.Errorf("failed to add adapter policy rule: %w", err)
    }
    return nil
}

// adapter.go:93-102
func (a *Adapter) RemovePolicy(_ string, ptype string, rule []string) error {
    r := newCasbinRule(ptype, rule)
    if err := a.delete(r); err != nil {
        return fmt.Errorf("failed to remove adapter policy rule: %w", err)
    }
    return nil
}
```

## No Watcher Implemented

Currently when multiple Grid API instances are running, policy changes in one instance are not propagated to others.
- **Reason:** Simplicity, low change frequency
- **Impact:** Changes to policies (e.g., role assignments) may take effect only after a server restart in other instances
- **Future Work:** Implement a Casbin Watcher using Redis or similar pub/sub mechanism to notify other instances of policy changes.

## Dynamic Groupings and Database Synchronization

### Current Behavior
Dynamic user→group→role groupings (extracted from JWT claims) are automatically persisted to `casbin_rules` table via AutoSave.

**Example:**
```sql
-- After Alice authenticates with JWT containing groups: [platform-engineers, product-engineers]
g | user:5bc8f962-b5ef-4ad7-9336-6b646619c955 | group:platform-engineers
g | user:5bc8f962-b5ef-4ad7-9336-6b646619c955 | group:product-engineers
```

### Synchronization Pattern

**On every authenticated request:**

1. **Clear previous groupings** (`groups.go:clearUserGroupings()`):
   - Queries existing roles for user
   - Deletes all dynamic groupings (user→group, user→role)
   - AutoSave immediately removes from database

2. **Apply current groupings** (`groups.go:ApplyDynamicGroupings()`):
   - Extracts groups from JWT claims
   - Adds user→group groupings
   - Adds group→role mappings (from repository)
   - AutoSave immediately inserts to database

**Result:** Database reflects "last authenticated state" for each user.

### Known Behavior: Stale Data Between Sessions

**Scenario:**
1. Alice authenticates → JWT has `groups: [platform-engineers, product-engineers]`
2. Grid persists `user:alice → group:platform-engineers` to database
3. Admin removes Alice from `platform-engineers` in Keycloak
4. Alice's JWT token hasn't expired yet, so she doesn't re-authenticate
5. Database still shows `user:alice → group:platform-engineers` (stale)

**When it syncs:**
- Alice's next API call **after getting a fresh JWT token** from Keycloak
- Fresh token has updated groups claim: `[product-engineers]` (no platform-engineers)
- `clearUserGroupings()` removes old groupings
- `ApplyDynamicGroupings()` adds only current groupings

**Synchronization delay:** Equals JWT token lifetime (typically 5-15 minutes)

### Design Trade-offs

**Current approach (AutoSave enabled for dynamic groupings):**
- ✅ Database shows authorization state for debugging/auditing
- ✅ Automatic persistence, no manual SavePolicy() calls
- ✅ Self-healing on next authentication
- ⚠️ Stale data visible between user sessions
- ⚠️ More database writes (full clear+reapply per auth)

**Alternative approaches considered:**
1. **Memory-only dynamic groupings** (disable AutoSave temporarily during ApplyDynamicGroupings)
   - Pros: No stale data, cleaner database, faster
   - Cons: Lost on server restart, can't audit from DB

2. **Delta-based sync** (compute diff between JWT groups and DB groups, only update changes)
   - Pros: Fewer DB writes, better performance
   - Cons: More complex code, still has stale data

3. **Separate table with TTL** (store dynamic groupings in `dynamic_groupings` table with timestamps)
   - Pros: Clean separation, explicit staleness tracking, can add cleanup job
   - Cons: More infrastructure, additional repository layer

**Decision:** Keep current approach for simplicity. Stale data is cosmetic (doesn't affect authorization correctness since JWT is source of truth during request processing). Can revisit if high-volume deployments need optimization.

## Testing Considerations

### Integration Tests
Tests verify the adapter correctly handles:
- ✅ Policies with bexpr scope expressions (e.g., `env == "dev"`)
- ✅ Dynamic groupings with AutoSave enabled
- ✅ Label-based access control enforcement
- ✅ Union semantics for multiple group memberships

**Test Location:** `tests/integration/auth_mode1_test.go`

### Database Inspection
After integration tests, the `casbin_rules` table will contain both:
- **Static policies** (p rules): Permission grants with scope expressions
- **Static group→role mappings** (g rules): `group:product-engineers → role:product-engineer`
- **Dynamic user groupings** (g rules): `user:UUID → group:NAME` (reflects last authenticated state)

Dynamic entries persist until:
- User authenticates again (groupings updated)
- Server restart (all policies reloaded from database)
- Manual cleanup (if implemented)

## References

- Casbin AutoSave documentation: https://casbin.org/docs/adapters
- Original adapter: https://github.com/msales/casbin-bun-adapter
- Bug fix commit: See git history for `adapter.go` LoadPolicy changes
- Related specs: `specs/006-authz-authn-rbac/` (AuthN/AuthZ design docs)