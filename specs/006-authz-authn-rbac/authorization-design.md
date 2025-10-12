# Grid Authorization Design

## Overview

This document defines the authorization model for Grid's dual API surface: Connect RPC (control plane) and Terraform HTTP Backend (data plane).

## Actions Taxonomy

### State Management (Control Plane)
- `state:create` - CreateState RPC
- `state:read` - GetStateInfo, GetStateConfig, GetStateLock, ListStates, ListStateOutputs
- `state:update-labels` - UpdateStateLabels RPC
- `state:delete` - (future) DeleteState RPC

### Terraform State Data (Data Plane)
- `tfstate:read` - GET /tfstate/{guid}
- `tfstate:write` - POST /tfstate/{guid}
- `tfstate:lock` - LOCK /tfstate/{guid}/lock
- `tfstate:unlock` - UNLOCK /tfstate/{guid}/unlock (HTTP) + UnlockState RPC

### Dependencies
- `dependency:read` - ListDependencies, ListDependents, SearchByOutput, GetTopologicalOrder, GetStateStatus, GetDependencyGraph, ListAllEdges
- `dependency:write` - AddDependency, RemoveDependency

### Label Policy Management
- `policy:read` - GetLabelPolicy
- `policy:write` - SetLabelPolicy

## Resource Model

Resources follow the pattern: `resource:type:identifier`

- `resource:state:*` - All states
- `resource:state:{guid}` - Specific state by GUID
- `resource:state:labels:{filter}` - States matching label filter (e.g., `env=dev`)
- `resource:policy:*` - Label policy (singleton)

## Roles and Permissions

### 1. service-account (Automation)

**Purpose**: CI/CD pipelines, automation that performs Terraform operations across all states

**Permissions**:
```yaml
permissions:
  - action: "tfstate:read"
    resource: "resource:state:*"
  - action: "tfstate:write"
    resource: "resource:state:*"
  - action: "tfstate:lock"
    resource: "resource:state:*"
  - action: "tfstate:unlock"
    resource: "resource:state:*"
```

**Characteristics**:
- Data plane only (no control plane access)
- No filtering by labels (unrestricted scope)
- Cannot create states or modify labels
- Cannot manage dependencies or policies

---

### 2. platform-engineer (Admin + Full Access)

**Purpose**: System administrators who define label policy, provide support, and run Terraform against all states

**Permissions**:
```yaml
permissions:
  - action: "state:*"
    resource: "resource:state:*"
  - action: "tfstate:*"
    resource: "resource:state:*"
  - action: "dependency:*"
    resource: "resource:state:*"
  - action: "policy:*"
    resource: "resource:policy:*"
```

**Characteristics**:
- Full access to control plane and data plane
- Can set/update label policy (SetLabelPolicy)
- No label-based filtering (unrestricted scope)
- Can modify any state's labels including protected keys

---

### 3. product-engineer (Scoped by Labels)

**Purpose**: Product teams that create and manage states for their environment (e.g., dev, staging)

**Permissions**:
```yaml
permissions:
  - action: "state:create"
    resource: "resource:state:*"
    constraints:
      create_label_filter:
        env: ["dev"]  # Can only create states with env=dev

  - action: "state:read"
    resource: "resource:state:labels:env=dev"  # Filtered by env=dev

  - action: "state:update-labels"
    resource: "resource:state:labels:env=dev"
    constraints:
      immutable_keys: ["env"]  # Cannot modify env key

  - action: "tfstate:*"
    resource: "resource:state:labels:env=dev"

  - action: "dependency:*"
    resource: "resource:state:labels:env=dev"

  - action: "policy:read"
    resource: "resource:policy:*"
```

**Characteristics**:
- Label-scoped access (e.g., only states where `env=dev`)
- Can create states but only with specific label values
- Cannot modify protected label keys (e.g., `env`) on existing states
- Can view label policy but cannot set it
- Full Terraform operations within their label scope
- Can manage dependencies but only for states they have access to

---

## Label Constraints Model

### Create Constraints
When creating a state, users can only set labels that match their `create_label_filter`:

```json
{
  "role": "product-engineer",
  "create_label_filter": {
    "env": ["dev", "test"],       // Can only set env=dev or env=test
    "team": ["team-a", "team-b"]   // Can only set team=team-a or team-team-b
  }
}
```

**Behavior**:
- CreateState request with `env=prod` → **DENIED** (403 Forbidden)
- CreateState request with `env=dev` → **ALLOWED**
- CreateState request without `env` label → Policy-dependent (if env is required by label policy, request fails validation)

### Update Constraints
When updating labels, users cannot modify keys listed in `immutable_keys`:

```json
{
  "role": "product-engineer",
  "immutable_keys": ["env", "region"]
}
```

**Behavior**:
- UpdateStateLabels adding/modifying `team` label → **ALLOWED**
- UpdateStateLabels modifying `env` label → **DENIED** (403 Forbidden)
- UpdateStateLabels removing `env` label → **DENIED** (403 Forbidden)

### Access Filtering
Users only see states matching their `access_filter`:

```json
{
  "role": "product-engineer",
  "access_filter": {
    "env": "dev",
    "team": "team-a"
  }
}
```

**Behavior**:
- ListStates → Returns only states where `env=dev AND team=team-a`
- GetStateInfo for state with `env=prod` → **DENIED** (404 Not Found or 403 Forbidden)
- POST /tfstate/{guid} for state with `env=staging` → **DENIED** (403 Forbidden)

---

## Authorization Decision Flow

### For Control Plane (Connect RPC)

```
1. Extract user identity from request (e.g., JWT claims, mTLS cert)
2. Load user's role and permissions
3. Determine action (e.g., "state:create", "dependency:write")
4. Determine resource (e.g., "resource:state:{guid}")
5. Check permission grants:
   a. Does role have action on resource?
   b. If label-scoped, does state match access_filter?
   c. If state:create, do requested labels match create_label_filter?
   d. If state:update-labels, are modified keys not in immutable_keys?
6. ALLOW or DENY
```

### For Data Plane (Terraform HTTP Backend)

```
1. Extract user identity from request (e.g., HTTP header, token)
2. Load user's role and permissions
3. Determine action from HTTP method:
   - GET → "tfstate:read"
   - POST → "tfstate:write"
   - LOCK → "tfstate:lock"
   - UNLOCK → "tfstate:unlock"
4. Extract GUID from URL path: /tfstate/{guid}
5. Fetch state labels from database
6. Check permission:
   a. Does role have tfstate:{action} on resource:state:*?
   b. If label-scoped, do state labels match access_filter?
7. ALLOW or DENY
```

---

## Implementation Considerations

### 1. Middleware Architecture
- **Connect RPC**: Implement as Connect interceptor (unary/streaming)
- **HTTP Backend**: Implement as Chi middleware for `/tfstate/*` routes

### 2. Identity Propagation
- Use context.Context to propagate user identity through request lifecycle
- Define `AuthContext` type containing user ID, role, permissions

### 3. Label Query Optimization
- For ListStates with access_filter, push filter to SQL WHERE clause
- Use bexpr library (already in use for filter parameter) for label matching
- Index label JSONB column for performance

### 4. Policy Storage
- Store role definitions in database table: `roles`
- Store user-role assignments in table: `user_roles`
- Cache role definitions in-memory with TTL refresh

### 5. Audit Logging
- Log authorization decisions (ALLOW/DENY) with:
  - User identity
  - Action attempted
  - Resource targeted
  - Decision reason
  - Timestamp

---

## Example Scenarios

### Scenario 1: Product Engineer Creates State

**Request**:
```protobuf
CreateStateRequest {
  guid: "01JXXX..."
  logic_id: "app-db-dev"
  labels: {"env": "dev", "team": "platform"}
}
```

**Authorization Check**:
1. User has role `product-engineer`
2. Action: `state:create`
3. Check `create_label_filter`: env=dev ✓ allowed
4. Result: **ALLOW**

---

### Scenario 2: Product Engineer Tries to Modify Env Label

**Request**:
```protobuf
UpdateStateLabelsRequest {
  state_id: "01JXXX..."
  adds: {"env": "prod"}  // Trying to change env
}
```

**Authorization Check**:
1. User has role `product-engineer`
2. Action: `state:update-labels`
3. Target state has labels: {"env": "dev"}
4. Check `access_filter`: env=dev ✓ state is accessible
5. Check `immutable_keys`: ["env"] ✗ env cannot be modified
6. Result: **DENY** (403 Forbidden: "Cannot modify immutable label key: env")

---

### Scenario 3: Service Account Writes State

**Request**:
```http
POST /tfstate/01JXXX... HTTP/1.1
Authorization: Bearer <token>

{terraform state JSON}
```

**Authorization Check**:
1. User has role `service-account`
2. Action: `tfstate:write`
3. Permission grants `tfstate:write` on `resource:state:*` (no label filtering)
4. Result: **ALLOW**

---

### Scenario 4: Product Engineer Lists States

**Request**:
```protobuf
ListStatesRequest {
  filter: ""  // No user-provided filter
}
```

**Authorization Check**:
1. User has role `product-engineer`
2. Action: `state:read` (implied by ListStates)
3. Apply `access_filter`: {"env": "dev"}
4. Execute SQL: `SELECT * FROM states WHERE labels->>'env' = 'dev'`
5. Result: **ALLOW** (returns filtered results)

---

## Security Considerations

### 1. Bypass Prevention
- Authorization checks MUST occur before business logic
- Never rely on client-provided filters for access control
- Always re-fetch state labels from database (don't trust request data)

### 2. Privilege Escalation Prevention
- `immutable_keys` prevents users from changing their access scope (e.g., changing env=dev to env=prod)
- `create_label_filter` prevents users from creating states outside their scope
- Policy management (`policy:write`) restricted to platform-engineer role

### 3. Dependency Authorization
- When creating dependency A→B, user must have access to BOTH states
- ListDependencies/ListDependents filtered by user's access scope
- Prevent information disclosure via dependency graph traversal

### 4. Lock Authorization
- User who locks state must have `tfstate:lock` permission
- Unlock can be performed by:
  - User who created the lock (if they provide correct lock_id)
  - platform-engineer (force unlock for operational support)

---

## Next Steps

1. **Define Auth Provider Interface**: Abstract identity/role resolution
2. **Implement Middleware**: Connect interceptor + Chi middleware
3. **Add Role Configuration**: YAML/JSON config for role definitions
4. **Database Schema**: Add `roles`, `user_roles`, `role_permissions` tables
5. **Testing**: Unit tests for authz logic + integration tests per role
6. **Documentation**: API docs with permission requirements per endpoint
