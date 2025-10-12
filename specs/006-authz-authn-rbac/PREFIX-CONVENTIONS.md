# Casbin Prefix Conventions

**Date**: 2025-10-12
**Purpose**: Document consistent prefix usage across Casbin subjects, roles, objects, and actions

---

## Overview

Casbin uses free-form strings for subjects, objects, and actions. To avoid collisions and improve readability, Grid uses consistent prefixes to namespace these identifiers.

---

## Prefix Taxonomy

### 1. Subject Prefixes (in Grouping Rules, ptype='g')

Subjects represent authenticated principals or intermediate groupings.

| Prefix | Example | Usage |
|--------|---------|-------|
| `user:` | `user:alice` | Human users (from OIDC `sub` claim) |
| `group:` | `group:dev-team` | SSO groups (from OIDC `groups` claim) |
| `sa:` | `sa:ci-pipeline` | Service accounts (non-interactive) |

**Casbin Grouping Examples**:
```
# User-to-group assignment (dynamic, at authentication time)
g, user:alice, group:dev-team

# Group-to-role assignment (static, from group_roles table)
g, group:dev-team, role:product-engineer

# Direct user-to-role assignment (static, from user_roles table)
g, user:alice, role:admin

# Service account-to-role assignment (static, from user_roles table)
g, sa:ci-pipeline, role:service-account
```

### 2. Role Prefixes (in Grouping Rules, ptype='g')

Roles are intermediate groupings that bundle permissions.

| Prefix | Example | Usage |
|--------|---------|-------|
| `role:` | `role:product-engineer` | All role definitions |

**Note**: Role names in the `roles` table do NOT include the prefix (stored as `product-engineer`). The `role:` prefix is added when creating Casbin grouping rules.

### 3. Object/Resource Identifiers (in Policy Rules, ptype='p', v1)

Objects represent the resource types being accessed. **No prefix used** - just the bare resource type.

| Object Type | Example | Usage |
|-------------|---------|-------|
| `state` | `state` | State resources (applies to both control plane and data plane) |
| `policy` | `policy` | Label policy resources |
| `*` | `*` | Wildcard (all resource types) |

**Casbin Policy Examples**:
```
# No prefix - just resource type
p, role:product-engineer, state, state:read, env == "dev", allow
p, role:platform-engineer, *, *, , allow
p, role:product-engineer, policy, policy:read, , allow
```

**Why no prefix?**
- Simplicity: Object type is clear from context
- Aligns with Casbin conventions in examples/docs
- Reduces string length in policies

### 4. Action Format (in Policy Rules, ptype='p', v2)

Actions follow the format: `category:operation`

| Category | Operations | Examples |
|----------|-----------|----------|
| `state` | `create`, `read`, `list`, `update-labels`, `delete` | `state:create`, `state:read` |
| `tfstate` | `read`, `write`, `lock`, `unlock` | `tfstate:read`, `tfstate:write` |
| `dependency` | `create`, `read`, `list`, `delete` | `dependency:create` |
| `policy` | `read`, `write` | `policy:read`, `policy:write` |
| `admin` | `role-manage`, `user-assign`, `group-assign`, `service-account-manage`, `session-revoke` | `admin:group-assign` |

**Wildcards**:
- `state:*` - All state operations
- `tfstate:*` - All Terraform state operations
- `dependency:*` - All dependency operations
- `admin:*` - All admin operations
- `*:*` or `*` - All actions (both forms accepted)

**Casbin Policy Examples**:
```
p, role:service-account, state, tfstate:read, , allow
p, role:platform-engineer, *, *, , allow
p, role:product-engineer, state, state:create, env == "dev", allow
```

---

## Full Example: Permission Resolution Flow

```
User Authentication:
  JWT: sub="alice@company.com", groups=["dev-team", "contractors"]

Casbin Grouping Rules (g):
  # Dynamic user-to-group (created at auth time, not persisted)
  g, user:alice@company.com, group:dev-team
  g, user:alice@company.com, group:contractors

  # Static group-to-role (from group_roles table)
  g, group:dev-team, role:product-engineer
  g, group:contractors, role:limited-access

Casbin Policy Rules (p):
  # product-engineer permissions
  p, role:product-engineer, state, state:create, env == "dev", allow
  p, role:product-engineer, state, tfstate:read, env == "dev", allow

  # limited-access permissions
  p, role:limited-access, state, state:read, , allow

Enforcement Check:
  enforcer.Enforce("user:alice@company.com", "state", "state:create", {"env": "dev"})

  Casbin Resolution:
    user:alice@company.com → group:dev-team → role:product-engineer
      → state:create on state with env=="dev" → ALLOW
```

---

## Prefix Decision Rationale

### Service Account Prefix: `sa:` (not `service-account:`)

**Decision**: Use `sa:` for brevity.

**Rationale**:
- Shorter strings in Casbin rules (reduces storage, improves readability)
- Consistent with common abbreviations (e.g., `sa` in Kubernetes)
- Unambiguous (no other entity type uses `sa:` prefix)

**Examples**:
```
# Grouping
g, sa:ci-pipeline, role:service-account
g, sa:terraform-cloud, role:service-account

# Not this:
g, service-account:ci-pipeline, role:service-account  (too long)
```

### User Identity Format: Use Full Subject

**Decision**: Use the full OIDC `sub` claim value, with `user:` prefix.

**Examples**:
```
# Email-based
user:alice@company.com

# UUID-based (Azure AD)
user:00000000-0000-0000-0000-000000000000

# Custom format (Keycloak)
user:keycloak|123456
```

**Note**: Do NOT normalize or transform the subject. Use it exactly as received from IdP.

### Group Name Format: Use Claim Value Directly

**Decision**: Use the group name from JWT claim, with `group:` prefix.

**Examples**:
```
# Flat group names
group:dev-team
group:platform-engineers

# Nested/hierarchical (if IdP uses this format)
group:/engineering/platform
group:Company/Teams/DevTeam
```

**Normalization**: Case-sensitive, no transformation. Grid admins must type group names exactly as they appear in JWT claims.

---

## Implementation Guidelines

### Go Code: Prefix Constants

```go
// cmd/gridapi/internal/auth/identifiers.go

package auth

import "fmt"

// Subject prefixes
const (
    PrefixUser           = "user:"
    PrefixGroup          = "group:"
    PrefixServiceAccount = "sa:"
)

// Role prefix
const (
    PrefixRole = "role:"
)

// Helper functions
func UserSubject(sub string) string {
    return PrefixUser + sub
}

func GroupSubject(groupName string) string {
    return PrefixGroup + groupName
}

func ServiceAccountSubject(clientID string) string {
    return PrefixServiceAccount + clientID
}

func RoleObject(roleName string) string {
    return PrefixRole + roleName
}
```

### Database Storage

**group_roles table**:
- `group_name` column: Store WITHOUT prefix (e.g., `dev-team`)
- When creating Casbin grouping: Add prefix (e.g., `group:dev-team`)

**user_roles table**:
- `user_id` column: Store user UUID (Grid's internal user ID)
- When creating Casbin grouping: Lookup user's OIDC subject, add prefix (e.g., `user:alice@company.com`)

**roles table**:
- `name` column: Store WITHOUT prefix (e.g., `product-engineer`)
- When creating Casbin policies/groupings: Add prefix (e.g., `role:product-engineer`)

### JWT Claim Extraction

```go
// Extract groups from JWT
func extractGroups(token *oidc.IDToken, claimField string) ([]string, error) {
    var claims map[string]interface{}
    if err := token.Claims(&claims); err != nil {
        return nil, err
    }

    // Support flat arrays: ["dev-team", "contractors"]
    if groups, ok := claims[claimField].([]interface{}); ok {
        result := make([]string, 0, len(groups))
        for _, g := range groups {
            if str, ok := g.(string); ok {
                result = append(result, str)  // Use as-is, no prefix here
            }
        }
        return result, nil
    }

    // Support nested objects: [{"name": "dev-team"}, {"name": "contractors"}]
    // (Implementation uses jsonpath or pointerstructure - see research.md)

    return nil, fmt.Errorf("groups claim not found or invalid format")
}

// At authentication time
func createDynamicGroupings(enforcer *casbin.Enforcer, userSub string, groups []string) error {
    userSubject := auth.UserSubject(userSub)

    for _, groupName := range groups {
        groupSubject := auth.GroupSubject(groupName)
        _, err := enforcer.AddRoleForUser(userSubject, groupSubject)
        if err != nil {
            return err
        }
    }
    return nil
}
```

---

## Consistency Checklist

When adding new features, ensure:

- [ ] User identifiers use `user:` prefix in Casbin
- [ ] Group identifiers use `group:` prefix in Casbin
- [ ] Service account identifiers use `sa:` prefix in Casbin
- [ ] Role identifiers use `role:` prefix in Casbin
- [ ] Object types have NO prefix (just bare type name)
- [ ] Actions use `category:operation` format
- [ ] Database tables store names WITHOUT prefixes
- [ ] Casbin API calls ADD prefixes when creating rules
- [ ] Helper functions defined in `auth/identifiers.go`
- [ ] Documentation examples use correct prefixes

---

## Migration Notes

If existing systems use different prefixes, create a migration that:

1. Reads all Casbin grouping rules (ptype='g')
2. Transforms subject identifiers (add/change prefixes)
3. Updates `casbin_rule` table
4. Reloads enforcer policies

Example migration:
```sql
-- Transform old user format to new format
UPDATE casbin_rule
SET v0 = 'user:' || v0
WHERE ptype = 'g' AND v0 NOT LIKE 'user:%' AND v0 NOT LIKE 'group:%' AND v0 NOT LIKE 'sa:%';

-- Transform role format (add role: prefix)
UPDATE casbin_rule
SET v1 = 'role:' || v1
WHERE ptype = 'g' AND v1 NOT LIKE 'role:%';
```

---

**Status**: Conventions documented. Ready for implementation in Phase 2.
