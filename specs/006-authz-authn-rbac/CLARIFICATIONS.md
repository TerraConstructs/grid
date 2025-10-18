# Authorization Implementation Clarifications

**Date**: 2025-10-12
**Purpose**: Address architectural questions and align implementation with enterprise SSO patterns

---

## 1. Group-to-Role Mapping vs Direct User-to-Role Assignment

### Current State Analysis

**Documents currently assume**: Direct user-to-role assignment via `AssignRole` RPC
- `user_roles` table: `(user_id, role_id)` mappings
- Casbin groupings: `g, user:alice, role:product-engineer`

**Problem**: This requires Grid admins to manage individual user assignments, which doesn't scale and doesn't integrate with enterprise SSO workflows.

### Recommended Approach: Group-to-Role Mapping

**Rationale**:
- **Standard enterprise pattern**: SSO providers (Keycloak, Azure AD, Okta) manage group membership
- **Separation of concerns**: SSO admins manage who is in what group; Grid admins manage what roles groups get
- **Scalability**: Adding a user to a group in IdP automatically grants them Grid roles
- **Audit trail**: Group membership changes tracked in IdP, role assignments tracked in Grid

### Proposed Architecture

#### Data Model Changes

**New Table**: `group_roles`
```sql
CREATE TABLE group_roles (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  group_name VARCHAR(255) NOT NULL,  -- From IdP claim (e.g., "dev-team", "platform-engineers")
  role_id UUID NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
  assigned_at TIMESTAMP NOT NULL DEFAULT NOW(),
  assigned_by UUID NOT NULL REFERENCES users(id),
  UNIQUE(group_name, role_id)
);

CREATE INDEX idx_group_roles_group ON group_roles(group_name);
CREATE INDEX idx_group_roles_role ON group_roles(role_id);
```

**Keep**: `user_roles` table for direct assignments (fallback/override scenarios)

#### Casbin Integration

**Grouping Rules in casbin_rule**:
```
# Group-to-role mappings (from group_roles table)
g, group:dev-team, role:product-engineer
g, group:platform-engineers, role:platform-engineer

# Direct user-to-role mappings (from user_roles table, rare)
g, user:alice, role:admin
```

**At Authentication Time** (JWT validation):
1. Extract user identity: `sub` claim → `user:alice`
2. Extract group membership: `groups` claim → `["dev-team", "contractors"]`
3. Add Casbin groupings dynamically:
   - `enforcer.AddRoleForUser("user:alice", "group:dev-team")`
   - `enforcer.AddRoleForUser("user:alice", "group:contractors")`
4. Casbin resolves: user → groups → roles → policies

**Note**: Dynamic groupings are transient (not persisted in `casbin_rule`). Only static group-to-role mappings persist.

#### OIDC Configuration

**New Config Fields** (`cmd/gridapi/internal/config/config.go`):
```go
type OIDCConfig struct {
    Issuer       string `yaml:"issuer" env:"OIDC_ISSUER"`
    ClientID     string `yaml:"client_id" env:"OIDC_CLIENT_ID"`
    ClientSecret string `yaml:"client_secret" env:"OIDC_CLIENT_SECRET"`
    RedirectURI  string `yaml:"redirect_uri" env:"OIDC_REDIRECT_URI"`

    // NEW: Configurable claim fields
    GroupsClaimField string `yaml:"groups_claim_field" env:"OIDC_GROUPS_CLAIM" default:"groups"`
    UserIDClaimField string `yaml:"user_id_claim_field" env:"OIDC_USER_ID_CLAIM" default:"sub"`
    EmailClaimField  string `yaml:"email_claim_field" env:"OIDC_EMAIL_CLAIM" default:"email"`
}
```

**Examples**:
- Keycloak: `groups_claim_field: "groups"`
- Azure AD: `groups_claim_field: "groups"` or `"roles"` (depends on configuration)
- Okta: `groups_claim_field: "groups"`
- Custom: `groups_claim_field: "custom_group_claim"`

#### Admin API Changes

**New RPCs** (add to `proto/state/v1/state.proto`):
```protobuf
// Group-to-role management
message AssignGroupRoleRequest {
  string group_name = 1;
  string role_name = 2;
}

message RemoveGroupRoleRequest {
  string group_name = 1;
  string role_name = 2;
}

message ListGroupRolesRequest {
  string group_name = 1;  // Optional: filter by group
}

message ListGroupRolesResponse {
  repeated GroupRoleAssignment assignments = 1;
}

message GroupRoleAssignment {
  string group_name = 1;
  string role_name = 2;
  google.protobuf.Timestamp assigned_at = 3;
  string assigned_by_user_id = 4;
}
```

**Keep existing**: `AssignRole` / `RemoveRole` for direct user assignments (edge cases)

### Functional Requirements Updates

**Add**:
- **FR-104**: System MUST extract group membership from configurable JWT claim field (default: `groups`)
- **FR-105**: System MUST support group-to-role mappings managed via Admin RPC (`AssignGroupRole`, `RemoveGroupRole`)
- **FR-106**: System MUST resolve user permissions via group membership: user → groups → roles → policies (transitive via Casbin)
- **FR-107**: System MUST support direct user-to-role assignments as fallback/override mechanism
- **FR-108**: System MUST allow deployers to configure OIDC claim field names for groups, user ID, and email

**Update**:
- **FR-035** (user-role assignments): Clarify this is for direct assignments; group-role assignments managed separately

### Implementation Impact

**Phase 2 Tasks to Add**:
1. Create `group_roles` table migration
2. Add OIDC config fields for claim customization
3. Implement JWT claim extraction with configurable field names
4. Implement dynamic Casbin grouping at authentication time
5. Add `AssignGroupRole` / `RemoveGroupRole` / `ListGroupRoles` RPCs
6. Update authentication middleware to resolve user → groups → roles
7. CLI commands: `gridctl role assign-group`, `gridctl role list-groups`

**Complexity**: Medium (adds one table, 3 RPCs, dynamic Casbin grouping logic)

---

## 2. Create Constraints vs Label Validation Policy

### Definitions

**Label Validation Policy** (`label_policy` table, already implemented):
- **Purpose**: Define **global data integrity rules** for all state labels
- **Scope**: Applies to ALL users (including admins)
- **Examples**:
  - `allowed_keys`: Which label keys can exist (e.g., `env`, `team`, `region`)
  - `allowed_values`: Valid values for specific keys (e.g., `env: [dev, staging, prod]`)
  - `reserved_prefixes`: Prefixes that cannot be used (e.g., `internal-`)
  - `max_keys`: Maximum number of labels per state (e.g., 32)
  - `max_value_len`: Maximum length of label values (e.g., 256)
- **Enforcement**: Validated at `CreateState` and `UpdateStateLabels` operations
- **Managed by**: `platform-engineer` role via `SetLabelPolicy` RPC
- **Example policy**:
  ```json
  {
    "allowed_keys": {"env": {}, "team": {}, "region": {}},
    "allowed_values": {
      "env": ["dev", "staging", "prod"],
      "region": ["us-west", "us-east", "eu-central"]
    },
    "max_keys": 32,
    "max_value_len": 256
  }
  ```

**Create Constraints** (role metadata, `roles.create_constraints` column):
- **Purpose**: Define **authorization restrictions** on what label values a role can SET during state creation
- **Scope**: Role-specific (e.g., product-engineer can only set `env=dev`)
- **Examples**:
  - Product-engineer: Can only create states with `env=dev` (even though policy allows dev/staging/prod)
  - Contractor: Can only create states with `team=external` (even though policy allows any team)
- **Enforcement**: Checked during `CreateState` after label policy validation
- **Managed by**: `platform-engineer` role via `CreateRole` / `UpdateRole` RPC
- **Example constraint**:
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

### Relationship and Interaction

**Two-Stage Validation**:
1. **Label Policy Validation** (global data integrity):
   - Is `env` in `allowed_keys`? ✓
   - Is `prod` in `allowed_values[env]`? ✓
   - Does label count exceed `max_keys`? ✗
   - Result: If any fail → **400 Bad Request** ("Label validation failed")

2. **Create Constraint Check** (role authorization):
   - Does role's `create_constraints[env]` allow `prod`? ✗
   - Is `env` required but missing? ✗
   - Result: If any fail → **403 Forbidden** ("Create constraint violated: env must be dev")

**Example Scenario**:
```
Label Policy:
  allowed_values.env = [dev, staging, prod]

product-engineer role:
  create_constraints.env.allowed_values = [dev]
  create_constraints.env.required = true

User tries to create state with env=prod:
  1. Label Policy: ✓ "prod" is a valid value (passes)
  2. Create Constraint: ✗ "prod" not in role's allowed list (fails)
  Result: 403 Forbidden
```

### Functional Requirements Mapping

**Relevant FRs**:
- **FR-017**: System MUST support "create constraints" that restrict which label values a role can set
- **FR-018**: System MUST enforce create constraints during state creation
- **FR-030**: System MUST validate permission definitions (create constraints require `state:create`)
- **FR-039**: System MUST evaluate requests by checking create constraints for create operations
- **FR-050**: Users provide labels subject to **both** policy validation **and** role create constraints
- **FR-052**: All label operations validated against label validation policy
- **FR-093**: CLI enforces create constraints with clear error messages

### Implementation: Constraint Validation Order

```go
// cmd/gridapi/internal/state/service.go

func (s *Service) CreateState(ctx context.Context, req *CreateStateRequest) error {
    // 1. Label Policy Validation (global data integrity)
    if err := s.labelPolicy.Validate(req.Labels); err != nil {
        return status.Errorf(codes.InvalidArgument,
            "label validation failed: %v", err)
    }

    // 2. Authorization Check (Casbin: does user have state:create?)
    allowed, err := s.enforcer.Enforce(userID, "state", "state:create", req.Labels)
    if err != nil || !allowed {
        return status.Error(codes.PermissionDenied, "insufficient permissions")
    }

    // 3. Create Constraint Validation (role-specific restrictions)
    role := s.getUserRole(ctx, userID)
    if err := validateCreateConstraints(role.CreateConstraints, req.Labels); err != nil {
        return status.Errorf(codes.PermissionDenied,
            "create constraint violated: %v", err)
    }

    // 4. Business logic (create state in DB)
    // ...
}

func validateCreateConstraints(constraints map[string]Constraint, labels map[string]string) error {
    for key, constraint := range constraints {
        value, exists := labels[key]

        // Check required
        if constraint.Required && !exists {
            return fmt.Errorf("required label %s is missing", key)
        }

        // Check allowed values
        if exists && len(constraint.AllowedValues) > 0 {
            if !contains(constraint.AllowedValues, value) {
                return fmt.Errorf("%s must be one of %v, got %s",
                    key, constraint.AllowedValues, value)
            }
        }
    }
    return nil
}
```

### Does LabelPolicy Need Modifications?

**Answer**: **No modifications needed to existing LabelPolicy implementation**.

**Reasoning**:
- LabelPolicy already validates at the right time (before create constraints)
- Create constraints are **additional restrictions** on top of policy, not replacements
- The two systems are orthogonal:
  - Policy: "What label values are valid in the system?"
  - Constraints: "What subset can this role use?"

**Data Flow**:
```
User Request
    ↓
Label Policy Validation (existing: label_policy.go)
    ↓ (if valid)
Casbin Authorization Check (new: enforcer.Enforce)
    ↓ (if allowed)
Create Constraint Validation (new: role.CreateConstraints)
    ↓ (if satisfied)
Create State in Database
```

### TypeScript Interface Impact

**Webapp `useLabelPolicy.ts`**: **No changes needed**.

The existing `PolicyDefinition` interface represents the global label policy. Create constraints are a separate concern managed via role configuration (not exposed to label policy hook).

**New Hook Needed**: `useRoleConstraints.ts`
```typescript
interface RoleConstraints {
  create_constraints?: Record<string, {
    allowed_values?: string[];
    required?: boolean;
  }>;
  immutable_keys?: string[];
}

export function useRoleConstraints(): {
  constraints: RoleConstraints | null;
  loading: boolean;
  error: Error | null;
} {
  // Fetch from GetEffectivePermissions RPC
}
```

---

## 3. Action Constants Source Reference

### Current State

**plan.md** references: `cmd/gridapi/internal/auth/actions.go` (to be created)

**Missing**: Link to source taxonomy in `authorization-design.md`

### Update Required

**File**: `specs/006-authz-authn-rbac/plan.md`

**Section**: "Action Taxonomy & Constants"

**Change**:
```markdown
### Action Taxonomy & Constants

**Decision**: Define Go constants for all actions in spec.md FR-025 taxonomy to prevent typos and enable IDE autocomplete.

**Source**: See `specs/006-authz-authn-rbac/authorization-design.md` (Actions Taxonomy section) for canonical action definitions and role mappings.

**Implementation**: See `cmd/gridapi/internal/auth/actions.go` (to be created during Phase 2 task generation).

**Actions include**:
- Control Plane: `state:{create|read|list|update-labels|delete}`, `dependency:{read|write}`, `policy:{read|write}`
- Data Plane: `tfstate:{read|write|lock|unlock}`
- Admin: `admin:{role-manage|user-assign|service-account-manage|session-revoke}`
- Wildcards: `state:*`, `tfstate:*`, `admin:*`, `*:*`

**Validation**: Action strings validated when creating/updating policies via Connect RPC to prevent typos. Casbin itself does not enforce action validation (uses free-form string matching).

**Mapping to Endpoints**: See authorization-design.md for which RPC methods and HTTP routes require which actions.
```

---

## Recommendations

### Immediate Updates Required

1. **spec.md**: Add FR-104 through FR-108 (group-to-role mapping requirements)
2. **data-model.md**: Add `group_roles` table schema
3. **plan.md**:
   - Add reference to authorization-design.md in Action Constants section
   - Add group-to-role tasks to Phase 2 task list
4. **research.md**: Add section on OIDC claim extraction and group resolution

### Implementation Priority

**High Priority** (essential for enterprise adoption):
- Group-to-role mapping (without this, SSO integration is cumbersome)
- Configurable OIDC claim fields (different IdPs use different claim names)

**Medium Priority** (can defer to later iteration):
- Direct user-to-role assignments (most orgs will use groups exclusively)

### Testing Considerations

**New Test Scenarios**:
1. User with multiple groups gets union of all group-role mappings
2. Group membership changes in JWT reflected on next request
3. Create constraint validation rejects values outside role's allowed list
4. Label policy validation runs before create constraint validation
5. Admin can bypass immutable key restrictions

---

## Final Decisions (2025-10-12)

### ✅ 1. Group Management UI
**Decision**: Type-only (no IdP browsing UI)

**Rationale**:
- Simpler implementation (no IdP API integration)
- Group names must match JWT claims exactly (no transformation)
- Admins type group name when creating mapping
- Error messages guide admins if typos occur

**Impact**:
- No additional IdP client libraries needed
- No OAuth scopes for group listing
- Admin UX: simple text input field

### ✅ 2. Group Claim Format
**Decision**: Support both flat arrays and simple nested objects using mapstructure

**Rationale**:
- Flat arrays cover 90% of cases (Keycloak, Okta default)
- Simple nested extraction (one level) handles Azure AD custom claims
- Leverage mapstructure (already in dep tree via go-bexpr)
- YAGNI: defer complex JSONPath until demand proven

**Implementation**:
- Flat: `["dev-team", "contractors"]` → extract directly
- Nested: `[{"name": "dev-team"}]` + path="name" → extract via mapstructure
- Complex paths deferred (document as future enhancement)

**Impact**:
- Zero new dependencies (mapstructure already imported)
- Config: add `groups_claim_path` field (optional, default empty)
- See research.md section 8 for full implementation

### ✅ 3. Default Claim Field
**Decision**: KISS/YAGNI - default to `groups`, make it configurable

**Rationale**:
- All major IdPs support "groups" as standard claim name
- Configurable via environment variable if IdP uses different name
- No auto-detection magic (explicit config better than implicit behavior)

**Configuration**:
```yaml
oidc:
  groups_claim_field: "groups"  # Default, override if needed
  groups_claim_path: ""         # Optional, for nested extraction
  user_id_claim_field: "sub"    # Standard OIDC
  email_claim_field: "email"    # Standard OIDC
```

**Impact**:
- Simple default works for most deployments
- Admins can override if IdP uses custom claim names
- No complex auto-detection logic

### ✅ 4. Group Prefix Convention
**Decision**: Use consistent prefixes across all Casbin identifiers

**Prefixes**:
- Users: `user:` (e.g., `user:alice@company.com`)
- Groups: `group:` (e.g., `group:dev-team`)
- Service Accounts: `sa:` (e.g., `sa:ci-pipeline`)
- Roles: `role:` (e.g., `role:product-engineer`)

**Rationale**:
- Prevents collisions (user "admin" vs role "admin")
- Self-documenting in Casbin rules
- Consistent with Casbin best practices

**Documentation**: See PREFIX-CONVENTIONS.md for complete taxonomy

**Impact**:
- Helper functions in `auth/identifiers.go`
- Database stores names WITHOUT prefixes
- Casbin API calls ADD prefixes when creating rules
- All examples updated for consistency
