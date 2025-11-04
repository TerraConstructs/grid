# Data Model: Authentication & Authorization

**Feature**: 006-authz-authn-rbac
**Date**: 2025-10-11
**Status**: Complete

## Overview

This document defines the database schema and entity relationships for Grid's authentication and authorization system. All entities map to PostgreSQL tables using the existing Bun ORM.

**Authorization Approach**: Casbin enforces RBAC with label-scoped access using go-bexpr expressions. Label scope filters are stored as TEXT (go-bexpr expression strings like `env == "dev"`), evaluated in-memory at enforcement time via a custom `bexprMatch` function. Casbin's default union (OR) semantics handle multiple roles automatically - no custom scope intersection logic needed.

**Adapter**: msales/casbin-bun-adapter stores policies in the `casbin_rule` table, sharing the existing *bun.DB connection pool.

---

## Entity Relationship Diagram

```
┌─────────────┐       ┌───────────┐       ┌─────────────┐
│    User     │───────│ UserRole  │───────│    Role     │
│             │  1:N  │           │  N:1  │  (metadata) │
└─────────────┘       └───────────┘       └─────────────┘
                                                   ▲
                                                   │ N:1
                                          ┌────────┴────────┐
                                          │   GroupRole     │
                                          │   (group→role)  │
                                          └─────────────────┘

┌─────────────────┐
│ ServiceAccount  │───────┐
│                 │  1:N  │       ┌───────────┐
└─────────────────┘       └───────│ UserRole  │
                                  └───────────┘

┌─────────────┐
│  Session    │─────── (references User.id or ServiceAccount.id)
└─────────────┘

┌─────────────┐
│ CasbinRule  │─────── (Casbin policy storage, managed by Casbin enforcer)
└─────────────┘        (includes both policies and user/group-to-role groupings)
```

**Note**: Role entity stores metadata only (name, description, constraints). Actual authorization policies stored in `casbin_rule` table. Role assignments stored in:
- `user_roles` - Direct user-to-role assignments (rare, for overrides)
- `group_roles` - Group-to-role mappings (primary mechanism for enterprise SSO)
- `casbin_rule` (ptype='g') - Dynamic groupings for enforcement (user→group, group→role)

---

## Entities

### 1. User

Represents an authenticated human user identified by their SSO provider subject ID.

**Table**: `users`

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| id | UUID | PK, Default: gen_random_uuid() | Internal user ID |
| subject | VARCHAR(255) | UNIQUE, NOT NULL | OIDC subject (e.g., `auth0\|123`, `azuread\|abc`) |
| email | VARCHAR(255) | NOT NULL | User email from OIDC claims |
| name | VARCHAR(255) | | User display name |
| created_at | TIMESTAMP | NOT NULL, Default: NOW() | Account creation time |
| updated_at | TIMESTAMP | NOT NULL, Default: NOW() | Last update time |
| last_login_at | TIMESTAMP | | Last successful login |
| disabled | BOOLEAN | NOT NULL, Default: FALSE | Account disabled flag |

**Indexes**:
- Primary key: `id`
- Unique: `subject`
- Index: `email` (for admin search)

**Validation Rules**:
- `subject` format: `{provider}|{provider_user_id}` (e.g., `keycloak|123`)
- `email` format: Valid email address (RFC 5322)

**Lifecycle**:
- Created: On first successful OIDC login
- Updated: On subsequent logins (last_login_at)
- Deleted: Never (soft delete via `disabled` flag)

---

### 2. ServiceAccount

Represents a non-interactive authentication principal (e.g., CI/CD pipeline) with client credentials.

**Table**: `service_accounts`

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| id | UUID | PK, Default: gen_random_uuid() | Internal service account ID |
| client_id | VARCHAR(255) | UNIQUE, NOT NULL | OAuth2 client ID (public) |
| client_secret_hash | VARCHAR(255) | NOT NULL | Bcrypt hash of client secret |
| name | VARCHAR(255) | NOT NULL | Human-readable name |
| description | TEXT | | Optional description |
| scope_labels | JSONB | NOT NULL, Default: '{}'::jsonb | Label scope enforced for this service account |
| created_at | TIMESTAMP | NOT NULL, Default: NOW() | Creation time |
| created_by | UUID | NOT NULL, FK: users(id) | User who created it |
| last_used_at | TIMESTAMP | | Last authentication time |
| secret_rotated_at | TIMESTAMP | | Last secret rotation timestamp |
| disabled | BOOLEAN | NOT NULL, Default: FALSE | Account disabled flag |

**Indexes**:
- Primary key: `id`
- Unique: `client_id`
- Index: `created_by` (for admin queries)
- Out of scope: GIN index: `scope_labels` (label scope filtering). Add only when SQL-side filtering becomes necessary; current design evaluates scopes in Go.

**Validation Rules**:
- `client_id` format: UUIDv4 string
- `client_secret_hash` format: Bcrypt hash (60 chars)
- `name` length: 3-100 characters
- `scope_labels` keys MUST align with label policy (validated in service layer)

**Lifecycle**:
- Created: Via `CreateServiceAccount` RPC by admin
- Updated: `last_used_at` on each authentication
- Deleted: Via `RevokeServiceAccount` RPC (soft delete via `disabled`)

**Security Notes**:
- Client secret NEVER stored in plain text
- Bcrypt cost: 10 (balance security/performance)
- Secret rotation: Create new service account, migrate, delete old

---

### 3. Role

Defines role metadata for admin UI and audit. Authorization policies live in `casbin_rule` table.

**Table**: `roles`

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| id | UUID | PK, Default: gen_random_uuid() | Internal role ID |
| name | VARCHAR(100) | UNIQUE, NOT NULL | Role name (e.g., `product-engineer`) |
| description | TEXT | | Human-readable description |
| scope_expr | TEXT | | Label scope filter as go-bexpr string (e.g., `env == "dev"`) |
| create_constraints | JSONB | | Label constraints for state creation |
| immutable_keys | TEXT[] | | Label keys that cannot be modified |
| created_at | TIMESTAMP | NOT NULL, Default: NOW() | Creation time |
| updated_at | TIMESTAMP | NOT NULL, Default: NOW() | Last update time |
| version | INTEGER | NOT NULL, Default: 1 | Optimistic locking version |

**Indexes**:
- Primary key: `id`
- Unique: `name` (mutable, but unique)

**scope_expr Format** (nullable TEXT):
- go-bexpr expression string evaluated against resource labels
- Examples: `env == "dev"`, `env == "dev" and team == "platform"`, `region == "us-west" or region == "us-east"`
- Empty/null = no scope restriction (all resources accessible)
- Evaluated at enforcement time via custom `bexprMatch` function in Casbin

> NOTE: go-bexpr does not support `in` operator; use `or` chains instead. Refer to specs/005-add-state-dimensions/bexpr-grammar.peg

**create_constraints** (nullable JSONB):
```json
{
  "env": {
    "allowed_values": ["dev"],
    "required": true
  },
  "team": {
    "allowed_values": ["platform", "infra"],
    "required": false
  }
}
```

**immutable_keys** (nullable TEXT[]):
```json
["env", "region"]
```

**Validation Rules**:
- `name` format: Lowercase, alphanumeric + dash (e.g., `product-engineer`)
- `name` length: 3-100 characters
- `scope_expr` must be valid go-bexpr syntax (validated when creating/updating role)
- `scope_expr` keys should exist in label policy (soft validation, not enforced)
- `create_constraints` keys must be referenced in Casbin policies with `state:create` action
- `immutable_keys` require corresponding Casbin policies with `state:update-labels` action

**Lifecycle**:
- Created: Via `CreateRole` RPC by admin (also creates corresponding Casbin policies in `casbin_rule`)
- Updated: Via `UpdateRole` RPC (increments `version`, updates Casbin policies)
- Deleted: Via `DeleteRole` RPC (removes Casbin policies, blocked if assigned to users)

**Relationship to Casbin**:
- Roles table is metadata only (for admin UI/audit)
- Actual enforcement uses `casbin_rule` table policies
- Admin RPCs keep both tables in sync

---

### 4. UserRole (Role Assignment)

Maps identities (human users or service accounts) to roles with optional label overrides.

**Table**: `user_roles`

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| id | UUID | PK, Default: gen_random_uuid() | Internal assignment ID |
| user_id | UUID | FK: users(id) | Set when assignment targets a human user |
| service_account_id | UUID | FK: service_accounts(id) | Set when assignment targets a service account |
| role_id | UUID | NOT NULL, FK: roles(id) ON DELETE CASCADE | Role ID |
| label_filter_override | JSONB | | Optional per-assignment label scope narrowing |
| assigned_at | TIMESTAMP | NOT NULL, Default: NOW() | Assignment timestamp |
| assigned_by | UUID | NOT NULL, FK: users(id) | Admin who created assignment |

**Check Constraint**:
- Exactly one of `user_id` or `service_account_id` MUST be non-null:
  ```sql
  (user_id IS NOT NULL)::int + (service_account_id IS NOT NULL)::int = 1
  ```

**Indexes**:
- Primary key: `id`
- Unique (partial): `(user_id, role_id)` WHERE `service_account_id IS NULL`
- Unique (partial): `(service_account_id, role_id)` WHERE `user_id IS NULL`
- Index: `user_id` (fast user lookup)
- Index: `service_account_id` (fast service account lookup)
- Index: `role_id` (fast role lookup)

**Validation Rules**:
- `label_filter_override` keys must exist in label policy (validated in service layer)
- Overrides MUST be subsets of the role's `label_scope`
- Cannot assign same role twice to same identity

**Lifecycle**:
- Created: Via `AssignRole` RPC by admin (enforces action `admin:user-assign`)
- Updated: Not supported (delete + recreate)
- Deleted: Via `RemoveRole` RPC by admin (cascade on role deletion)

**Note**: Direct user-to-role assignments are rare. Most assignments happen via `group_roles` (group-to-role mappings).

---

### 5. GroupRole (Group-to-Role Assignment)

Maps SSO groups to roles. Group membership extracted from JWT claims; role assignments managed by Grid admins.

**Table**: `group_roles`

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| id | UUID | PK, Default: gen_random_uuid() | Internal assignment ID |
| group_name | VARCHAR(255) | NOT NULL | SSO group name from JWT claim (e.g., "dev-team", "platform-engineers") |
| role_id | UUID | NOT NULL, FK: roles(id) ON DELETE CASCADE | Role ID |
| assigned_at | TIMESTAMP | NOT NULL, Default: NOW() | Assignment timestamp |
| assigned_by | UUID | NOT NULL, FK: users(id) | Admin who created assignment |

**Indexes**:
- Primary key: `id`
- Unique: `(group_name, role_id)` (prevent duplicate group-role pairs)
- Index: `group_name` (fast lookup by group)
- Index: `role_id` (fast role lookup)

**Validation Rules**:
- `group_name` must match format extracted from JWT claim (case-sensitive)
- Cannot assign same role twice to same group
- Group does not need to "exist" in Grid (membership defined by IdP)

**Lifecycle**:
- Created: Via `AssignGroupRole` RPC by admin (enforces action `admin:group-assign`)
- Updated: Not supported (delete + recreate)
- Deleted: Via `RemoveGroupRole` RPC by admin (cascade on role deletion)

**Relationship to Casbin**:
- Static mappings stored in `group_roles` table
- At authentication time, user's JWT groups are used to create dynamic Casbin groupings:
  - For each group in JWT: `enforcer.AddRoleForUser("user:alice", "group:dev-team")`
  - Casbin then resolves: user → group → role → policies
- Dynamic groupings are transient (not persisted in `casbin_rule`)

**Enterprise SSO Pattern**:
1. SSO admin adds user to "dev-team" group in Keycloak/Azure AD
2. Grid admin creates mapping: `group:dev-team` → `role:product-engineer`
3. User logs in, JWT contains `groups: ["dev-team"]`
4. Grid extracts groups, resolves to roles, user gets product-engineer permissions
5. No Grid admin intervention needed when new users join the group

---

### 6. Session

Tracks active sessions for human users (browser/CLI) and long-lived service account tokens that Grid issues.

**Table**: `sessions`

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| id | UUID | PK, Default: gen_random_uuid() | Internal session ID |
| user_id | UUID | FK: users(id) ON DELETE CASCADE | Set for human sessions |
| service_account_id | UUID | FK: service_accounts(id) ON DELETE CASCADE | Set for service account tokens |
| token_hash | VARCHAR(64) | UNIQUE, NOT NULL | SHA256 hash of bearer token |
| id_token | TEXT | | OIDC ID token (JWT) for human sessions |
| refresh_token | TEXT | | OIDC refresh token (nullable) |
| expires_at | TIMESTAMP | NOT NULL | Session expiry (12 hours from creation) |
| created_at | TIMESTAMP | NOT NULL, Default: NOW() | Login time |
| last_used_at | TIMESTAMP | NOT NULL, Default: NOW() | Last request time |
| user_agent | VARCHAR(255) | | Browser/CLI user agent |
| ip_address | INET | | Client IP address |
| revoked | BOOLEAN | NOT NULL, Default: FALSE | Manual revocation flag |

**Check Constraint**:
- Exactly one of `user_id` or `service_account_id` MUST be non-null (same pattern as `user_roles`).

**Indexes**:
- Primary key: `id`
- Unique: `token_hash` (fast lookup)
- Index: `user_id` (for user session list)
- Index: `service_account_id` (for service account audit)
- Index: `expires_at` (for cleanup job)

**Validation Rules**:
- `token_hash` format: SHA256 hex (64 chars)
- `id_token` required for human sessions; NULL for service accounts
- `expires_at` = `created_at` + 12 hours

**Lifecycle**:
- Created: On successful OIDC callback (user) or token issuance (service account)
- Updated: `last_used_at` on each authenticated request
- Deleted: On logout, expiry, revocation, or secret rotation (cleanup job)

**Security Notes**:
- Bearer token never stored (only hash)
- ID token stored for claims extraction on human sessions (not used after initial validation)
- Refresh token stored to support token refresh for human sessions (optional)

---

### 7. CasbinRule

Stores Casbin policy rules and user-role groupings. Managed by msales/casbin-bun-adapter.

**Table**: `casbin_rule`

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| id | SERIAL | PK | Auto-increment ID |
| ptype | VARCHAR(100) | NOT NULL | Policy type (`p` for policies, `g` for user-role groupings) |
| v0 | VARCHAR(255) | | For policies: role name. For groupings: user ID |
| v1 | VARCHAR(255) | | For policies: object type (e.g., "state"). For groupings: role name |
| v2 | VARCHAR(255) | | For policies: action (e.g., "state:read"). For groupings: unused |
| v3 | VARCHAR(255) | | For policies: scope expression (go-bexpr string). For groupings: unused |
| v4 | VARCHAR(255) | | For policies: effect (allow/deny). For groupings: unused |
| v5 | VARCHAR(255) | | Reserved for future use |

**Indexes**:
- Primary key: `id`
- Unique: `(ptype, v0, v1, v2, v3, v4, v5)` (no duplicate rules)

**Example Policy Rules** (ptype='p'):
```
ptype | v0                    | v1    | v2              | v3              | v4    | v5
------|----------------------|-------|-----------------|-----------------|-------|----
p     | role:service-account | state | tfstate:read    |                 | allow |
p     | role:platform-eng    | *     | *               |                 | allow |
p     | role:product-eng     | state | state:read      | env == "dev"    | allow |
p     | role:product-eng     | state | tfstate:write   | env == "dev"    | allow |
```

**Example Grouping Rules** (ptype='g'):
```
ptype | v0              | v1                    | v2 | v3 | v4 | v5
------|-----------------|----------------------|----|----|----|----|
g     | user:alice      | role:product-eng     |    |    |    |
g     | user:bob        | role:platform-eng    |    |    |    |
g     | sa:ci-pipeline  | role:service-account |    |    |    |
```

**Managed By**:
- Casbin enforcer (via msales/casbin-bun-adapter)
- Admin RPCs create/update/delete policies when managing roles
- User assignment RPCs create/delete grouping rules

**bexprMatch Evaluation**:
- v3 (scopeExpr) evaluated via custom Casbin function `bexprMatch(scopeExpr, labels)`
- Empty scopeExpr treated as "true" (no constraint)
- Expressions cached after first compilation for performance

---

## Relationships

### User ↔ Role (Many-to-Many, via UserRole OR via Group)

**Direct Assignment** (rare):
- Intermediary: `user_roles` (for metadata/audit)
- Casbin grouping rules: `casbin_rule` with ptype='g' (for enforcement)
- Used for edge cases/overrides

**Via Group Assignment** (primary):
- User has groups (from JWT claims)
- Groups mapped to roles via `group_roles` table
- At authentication: dynamic Casbin groupings created (user→group→role)
- No persistent storage of user-group relationship (IdP is source of truth)

### Group ↔ Role (Many-to-Many)
- Intermediary: `group_roles` (persistent mapping)
- A group can be assigned multiple roles
- A role can be assigned to multiple groups
- Group membership managed by IdP; role assignments managed by Grid admins

### ServiceAccount ↔ Role (Many-to-Many)
- Intermediary: `user_roles` (rows with `service_account_id` set)
- Casbin grouping rules: `casbin_rule` with ptype='g'
- Check constraint guarantees each row targets exactly one identity type

### Role ↔ CasbinRule Policies (Logical, not FK)
- Role metadata stored in `roles` table
- Authorization policies stored in `casbin_rule` with ptype='p', v0=role_name
- Admin RPCs keep both in sync (not enforced by database constraints)
- Deleting role must also delete corresponding Casbin policies

### User ↔ Session (One-to-Many)
- A user can have multiple active sessions (browser + CLI)
- A session belongs to one identity (user or service account)
- Cascade delete: Deleting identity deletes sessions

### Permission Resolution Flow
```
User Login (JWT with groups claim)
    ↓
Extract groups: ["dev-team", "contractors"]
    ↓
Lookup group_roles: dev-team → product-engineer, contractors → limited-access
    ↓
Create dynamic Casbin groupings:
  - g, user:alice, group:dev-team
  - g, user:alice, group:contractors
  - g, group:dev-team, role:product-engineer  (from group_roles)
  - g, group:contractors, role:limited-access (from group_roles)
    ↓
Casbin resolves: user → groups → roles → policies
    ↓
Union of all roles' permissions (OR semantics)
```

---

## Seed Data (Default Roles)

Three default roles seeded in database migrations (roles table + casbin_rule table):

### 1. service-account

**Role Metadata** (roles table):
```json
{
  "name": "service-account",
  "description": "Automation/CI-CD pipeline access (Data Plane only)",
  "scope_expr": null,
  "create_constraints": null,
  "immutable_keys": []
}
```

**Casbin Policies** (casbin_rule table):
```
p, role:service-account, state, tfstate:read, , allow
p, role:service-account, state, tfstate:write, , allow
p, role:service-account, state, tfstate:lock, , allow
p, role:service-account, state, tfstate:unlock, , allow
```

### 2. platform-engineer

**Role Metadata** (roles table):
```json
{
  "name": "platform-engineer",
  "description": "Full admin access (Control + Data Plane)",
  "scope_expr": null,
  "create_constraints": null,
  "immutable_keys": []
}
```

**Casbin Policies** (casbin_rule table):
```
p, role:platform-engineer, *, *, , allow
```

### 3. product-engineer

**Role Metadata** (roles table):
```json
{
  "name": "product-engineer",
  "description": "Label-scoped access for product teams (dev environment)",
  "scope_expr": "env == \"dev\"",
  "create_constraints": {
    "env": {
      "allowed_values": ["dev"],
      "required": true
    }
  },
  "immutable_keys": ["env"]
}
```

**Casbin Policies** (casbin_rule table):
```
p, role:product-engineer, state, state:create, env == "dev", allow
p, role:product-engineer, state, state:read, env == "dev", allow
p, role:product-engineer, state, state:list, env == "dev", allow
p, role:product-engineer, state, state:update-labels, env == "dev", allow
p, role:product-engineer, state, tfstate:read, env == "dev", allow
p, role:product-engineer, state, tfstate:write, env == "dev", allow
p, role:product-engineer, state, tfstate:lock, env == "dev", allow
p, role:product-engineer, state, tfstate:unlock, env == "dev", allow
p, role:product-engineer, state, dependency:create, env == "dev", allow
p, role:product-engineer, state, dependency:read, env == "dev", allow
p, role:product-engineer, state, dependency:list, env == "dev", allow
p, role:product-engineer, state, dependency:delete, env == "dev", allow
p, role:product-engineer, policy, policy:read, , allow
```

---

## Database Migrations

### Migration File: `YYYYMMDDHHMMSS_auth_schema.go`

**Up**:
1. Create `users` table
2. Create `service_accounts` table
3. Create `roles` table (with scope_expr TEXT instead of label_scope JSONB)
4. Create `user_roles` table (with dual FK + check constraint)
5. Create `group_roles` table (group-to-role mappings)
6. Create `sessions` table (with dual FK + check constraint)
7. Create `casbin_rule` table (standard Casbin schema)
8. Create unique index on `casbin_rule` (ptype, v0, v1, v2, v3, v4, v5)

**Down**:
1. Drop all tables in reverse order

### Migration File: `YYYYMMDDHHMMSS_seed_auth_data.go`

**Up**:
1. Insert 3 default roles into `roles` table
2. Insert default Casbin policies into `casbin_rule` table (with go-bexpr expressions)

**Down**:
1. Delete seeded rows from `casbin_rule` and `roles`

**Note**: Policies are seeded directly into `casbin_rule`, not via Casbin API, to ensure they exist before server startup.

---

## Query Patterns

### Get User Effective Roles and Metadata
```sql
SELECT r.name,
       r.scope_expr,
       r.create_constraints,
       r.immutable_keys,
       ur.label_filter_override
FROM user_roles ur
JOIN roles r ON ur.role_id = r.id
WHERE ur.user_id = $1;
```

**Note**: Actual authorization policies queried from `casbin_rule` by Casbin enforcer. This query is for admin UI/audit only.

### Get User Casbin Policies (via Casbin enforcer)
```go
// Get all policies for user's roles
roles := enforcer.GetRolesForUser(userID)
for _, role := range roles {
    policies := enforcer.GetFilteredPolicy(0, role)
    // policies contains: [role, objType, act, scopeExpr, eft]
}
```

### Validate Session
```sql
SELECT s.id,
       COALESCE(s.user_id::text, s.service_account_id::text) AS identity_id,
       s.expires_at,
       s.revoked,
       COALESCE(u.disabled, sa.disabled, FALSE) AS disabled
FROM sessions s
LEFT JOIN users u ON s.user_id = u.id
LEFT JOIN service_accounts sa ON s.service_account_id = sa.id
WHERE s.token_hash = $1
  AND s.expires_at > NOW()
  AND s.revoked = FALSE
  AND COALESCE(u.disabled, sa.disabled, FALSE) = FALSE;
```

### Check Permission (Casbin enforcer)
```go
// Enforcement call with labels
allowed, err := enforcer.Enforce(userID, "state", "read", stateLabels)
// Casbin queries casbin_rule table and evaluates bexprMatch(scopeExpr, stateLabels)
```

### List Policies for Role (Casbin API)
```go
// Get all policies for a role
policies := enforcer.GetFilteredPolicy(0, "role:product-engineer")
// Returns: [[role:product-engineer, state, state:read, env == "dev", allow], ...]
```

---

## Constraints & Invariants

1. **User Subject Uniqueness**: Each OIDC subject maps to exactly one user
2. **Role Assignment Uniqueness**: Partial unique indexes prevent duplicate role pairs per identity
3. **Group-Role Assignment Uniqueness**: Unique index on `group_roles` (group_name, role_id) prevents duplicate group-role pairs
4. **Single Identity Constraint**: Check constraint ensures each user_role row references exactly one identity type
5. **Token Hash Uniqueness**: No two sessions with same token hash
6. **Role Name Uniqueness**: Role names globally unique
7. **Casbin Policy Uniqueness**: Unique index on casbin_rule (ptype, v0, v1, v2, v3, v4, v5) prevents duplicate policies
8. **Role-Policy Sync**: Admin RPCs must keep roles table and casbin_rule policies in sync (application-level invariant, not DB constraint)
9. **Group-Role Sync**: Static group-to-role mappings in `group_roles` table; dynamic user-to-group groupings created at authentication time (not persisted)
10. **Session Identity Constraint**: Check constraint ensures each session row references exactly one identity type
11. **Session Expiry**: `expires_at` always >= `created_at`
12. **Soft Deletes**: Users and service accounts use `disabled` flag, not hard delete
13. **bexpr Syntax**: scope_expr must be valid go-bexpr syntax (validated at role creation/update)
14. **Union Semantics**: Multiple roles grant access via OR logic (Casbin's default behavior)
15. **Group Source of Truth**: Group membership defined by IdP (JWT claims), not stored in Grid database

---

**Status**: Data model complete. Ready for Phase 1 contract generation.
