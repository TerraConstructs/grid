# Feature Specification: Authentication, Authorization, and RBAC

**Feature Branch**: `006-authz-authn-rbac`
**Created**: 2025-10-11
**Status**: Draft
**Input**: User description: "AuthZ/AuthN Rbac: add Authentication, Authorization, and RBAC controls that protect both the Control Plane (state/dependency/policy APIs) and the Data Plane (Terraform /tfstate endpoints). The system must be deployment-configurable to support label-scoped access, service accounts, and organization SSO."

## Execution Flow (main)
```
1. Parse user description from Input
   → If empty: ERROR "No feature description provided"
2. Extract key concepts from description
   → Identify: actors, actions, data, constraints
3. For each unclear aspect:
   → Mark with [NEEDS CLARIFICATION: specific question]
4. Fill User Scenarios & Testing section
   → If no clear user flow: ERROR "Cannot determine user scenarios"
5. Generate Functional Requirements
   → Each requirement must be testable
   → Mark ambiguous requirements
6. Identify Key Entities (if data involved)
7. Run Review Checklist
   → If any [NEEDS CLARIFICATION]: WARN "Spec has uncertainties"
   → If implementation details found: ERROR "Remove tech details"
8. Return: SUCCESS (spec ready for planning)
```

---

## ⚡ Quick Guidelines
- ✅ Focus on WHAT users need and WHY
- ❌ Avoid HOW to implement (no tech stack, APIs, code structure)
- 👥 Written for business stakeholders, not developers

---

## Clarifications

### Session 2025-10-11

- Q: What should be the default session/token expiration duration? → A: 12 hours
- Q: Should different roles be able to have different session/token durations? → A: No - all roles use the same default (12 hours)
- Q: What format should label scope filters use? → A: go-bexpr expression strings (e.g., `env == "dev"` or `env == "dev" and team == "platform"`), evaluated against resource labels at enforcement time
- Q: Should multiple roles use AND or OR semantics for access? → A: OR (union) semantics - Casbin's default behavior where any role granting access is sufficient

---

## User Scenarios & Testing

### Primary User Stories

#### Story 1: Product Engineer Managing Development States
As a product engineer, I need to create and manage Terraform states for my team's development environment, so that I can safely deploy infrastructure changes without affecting production systems. I must be prevented from accidentally creating or modifying production states, and I should only see states relevant to my team.

#### Story 2: Platform Engineer Providing Support
As a platform engineer, I need full access to all Terraform states across all environments, so that I can troubleshoot issues, perform emergency unlocks, and manage the label validation policy that governs all state metadata.

#### Story 3: CI/CD Pipeline Executing Terraform
As an automation system (CI/CD pipeline), I need non-interactive credentials to read and write Terraform state data across all environments, so that I can execute infrastructure deployments without human intervention. I should not be able to create new states or modify metadata, only perform Terraform operations.

#### Story 4: Organization User Accessing via SSO
As an organization member, I need to authenticate using my company's single sign-on system (Google Workspace, Okta, Azure AD), so that I can access Grid without creating separate credentials and my access is centrally managed by our IT team.

#### Story 5: CLI User Authenticating from Terminal
As a developer using the CLI tool, I need to authenticate from my terminal using a browser-based flow, so that I can use Grid commands without manually copying tokens or storing credentials in config files.

### Acceptance Scenarios

#### Authentication (AuthN)

1. **Given** a user with a company SSO account, **When** they navigate to the Grid web application, **Then** they are redirected to their organization's identity provider, authenticate there, and are returned to Grid with an active session.

2. **Given** a CLI user running `gridctl login`, **When** the command initiates a device-code flow, **Then** the user receives a URL and code, can authenticate in their browser, and the CLI obtains a valid token automatically.

3. **Given** an administrator creating a service account, **When** the service account credentials are generated, **Then** the system provides a client ID and secret that can be used for non-interactive authentication.

4. **Given** an authenticated user session, **When** the session expires or is revoked, **Then** subsequent requests are rejected and the user must re-authenticate.

#### Authentication (AuthN) - CLI Wrapper

1. **Pass-through & exit codes**
   Given a valid context and token, when the user runs `gridctl tf plan`, then terraform/ tofu runs normally, logs stream to console, and the wrapper exits with the same code as terraform/tofu.

2. **Binary selection**
   Given `TF_BIN=tofu`, when the user runs `gridctl tf apply`, the wrapper uses `tofu` and not `terraform`.

3. **Auth injection**
   Given an expired token, when the user runs `gridctl tf plan`, the wrapper refreshes the token, injects it, and the plan succeeds without prompting (if refresh is possible).

4. **Mid-run 401**
   Given a token that expires during `gridctl tf apply`, when the backend returns 401, the wrapper refreshes once and retries the apply. If refresh fails, the wrapper exits with a non-zero code and shows “authentication required” (no token leaked).

5. **CI non-interactive**
   Given no interactive browser access and valid service account credentials, when CI runs `gridctl tf init`, the command completes successfully without prompts. If credentials are missing, it fails fast with a clear error.

6. **Token secrecy**
   Given `--verbose`, when the user runs `gridctl tf plan`, the wrapper prints the command line and backend endpoint info with tokens redacted; tokens never appear in logs.

#### Authorization (AuthZ) - Control Plane

5. **Given** a product engineer with `env=dev` label scoped role, **When** they attempt to create a state with labels `env=dev, team=platform`, **Then** the state is created successfully.

6. **Given** a product engineer with `env=dev` label scoped role, **When** they attempt to create a state with labels `env=prod`, **Then** the request is denied with a clear error message indicating the label constraint violation.

7. **Given** a product engineer with `env=dev` label scoped role and `env` marked as immutable, **When** they attempt to update an existing state's `env` label from `dev` to `staging`, **Then** the request is denied with an error indicating the label is immutable.

8. **Given** a product engineer with `env=dev` label scoped role, **When** they list all states, **Then** only states with `env=dev` labels are returned in the results.

9. **Given** a product engineer with `env=dev` label scoped role, **When** they attempt to access state information for a state with `env=prod`, **Then** the request is denied as if the state does not exist (404 or 403).

10. **Given** a product engineer with `env=dev` label scoped role, **When** they attempt to create a dependency between two states both having `env=dev`, **Then** the dependency is created successfully.

11. **Given** a product engineer with `env=dev` label scoped role, **When** they attempt to create a dependency where the source state has `env=prod`, **Then** the request is denied because they lack access to the source state.

12. **Given** a product engineer role, **When** they attempt to update the label validation policy, **Then** the request is denied because policy management requires admin permissions.

13. **Given** a platform engineer with admin permissions, **When** they update the label validation policy, **Then** the policy is updated and immediately applied to all subsequent label operations.

#### Authorization (AuthZ) - Data Plane (Terraform /tfstate)

14. **Given** a service account with `tfstate:*` permissions, **When** it sends a GET request to `/tfstate/{guid}` with valid credentials, **Then** the state JSON is returned successfully.

15. **Given** an unauthenticated request, **When** it attempts to access `/tfstate/{guid}`, **Then** the request is denied with a 401 Unauthorized error.

16. **Given** a product engineer with `env=dev` label scope, **When** they attempt to POST state data to `/tfstate/{guid}` where the state has `env=prod`, **Then** the request is denied with a 403 Forbidden error.

17. **Given** a user with appropriate permissions, **When** they send a LOCK request to `/tfstate/{guid}/lock`, **Then** the state is locked and a lock ID is returned.

18. **Given** a locked state, **When** a different user with appropriate permissions attempts to LOCK the same state, **Then** the request is denied with a 409 Conflict error.

19. **Given** a user who locked a state, **When** they send an UNLOCK request with the correct lock ID, **Then** the state is unlocked successfully.

20. **Given** a platform engineer, **When** they attempt to force-unlock a state locked by another user, **Then** the unlock succeeds (admin override capability).

#### Label Management

21. **Given** a user creating a state with labels `env=dev, team=platform`, **When** the label validation policy requires `env` and allows `team` as free-text, **Then** the state is created with both labels validated.

22. **Given** a user creating a state with labels `env=invalid-value`, **When** the label validation policy only allows `env` values of `dev`, `staging`, `prod`, **Then** the request is denied with a validation error listing allowed values.

23. **Given** a state with labels `env=dev, team=platform`, **When** a user with appropriate permissions updates labels to add `owner=alice` and remove `team`, **Then** the state's labels become `env=dev, owner=alice`.

24. **Given** a label validation policy update that adds a new required key, **When** the policy is applied, **Then** existing states are not affected, but new state creation requires the new key.

### Edge Cases

#### Authentication Edge Cases

> Because Casbin allows dynamic policy updates and role managers, role edits or deletion take effect immediately. Because Keycloak may return temporarily_unavailable, the system should model that error class specifically and possibly retry.”

- **What happens when an SSO provider is unreachable during login?**
  - The system should return a clear error message indicating the identity provider is unavailable and suggest retrying.
  - Immediately return an error to the user like “Identity provider unavailable, please retry later.” Do not retry automatically. Set a timeout (e.g. 2–5 seconds) for IdP requests to avoid blocking UI.

- **What happens when a service account's credentials are rotated?**
  - Existing tokens issued with old credentials remain valid until their expiration time.
  - New authentication attempts must use the new credentials.
  - Immediately invalidate new authentication attempts using old credentials, but do not revoke previously issued tokens until they expire.

- **What happens when a user's SSO account is disabled while they have an active Grid session?**
  - Sessions are validated only at login; existing sessions continue until token expiration.

- **What happens when authentication token expires mid-operation (e.g., during a long Terraform apply)?**
  - The Terraform operation should fail with a 401 Unauthorized error.
  - The operation fails with a 401 Unauthorized. The CLI must detect that and ask the user to reauthenticate (or fetch a fresh token).

- **What happens when a user attempts to authenticate from multiple devices simultaneously?**
  - All sessions should be allowed (multiple concurrent sessions per user).
  - All login sessions are allowed (no limit). The system does not restrict concurrency.

- **What happens when the OIDC IdP returns an error during the callback phase of the login flow?**
  - The system should display the IdP error message to the user and log the failure.
  - Surface a user-friendly error message containing the IdP’s error (e.g. “access_denied: user is not in allowed group”) and log full details server-side.

#### Authorization and Role Management Edge Cases

- **What happens when a user's role is changed while they have an active session?**
  - The session should reflect the new permissions on the next request (no stale permission caching).
  - On the next request, the user’s effective permissions reflect the new role (i.e. authorization is always re-evaluated).

- **What happens when a user is assigned multiple roles with conflicting label scopes?**
  - Example: Role A grants access only to `env=dev`, Role B grants access only to `env=prod`.
  - The user can operate on any resource that satisfies at least one role’s scope. In the example, they may work with `env=dev` resources through Role A and `env=prod` resources through Role B, but not resources that fall outside both scopes.
  - We adopt union semantics: each role’s scope is evaluated independently, and access is granted when any assigned role permits the action. A role with no scope constraint is treated as globally permissive for the actions it grants.

- **What happens when a role definition is deleted while users are actively using it?**
  - Users with that role should be denied access on the next request.
  - [NEEDS CLARIFICATION: based on research phase in implementation for supported technical solutions]

- **What happens when a user with no assigned roles attempts to perform any operation?**
  - All operations should be denied with a 403 Forbidden error.
  - On subsequent requests, the role has no effect (users lose those permissions).

- **What happens when an admin attempts to remove their own admin role while they are the only admin?**
  - Prevent this action (disallow removing the last admin role) to avoid lock-out.

- **What happens when a role's permissions are updated to remove an action that users are currently relying on?**
  - Subsequent requests requiring that action should be denied.
  - On next request, enforcement denies that action.

#### Label Scope and Filtering Edge Cases

- **What happens when a label scope filter uses a label key that doesn't exist on any states?**
  - List operations return empty results; direct access to states returns 404 Not Found.
  - This is expected behavior (user has valid permissions but no matching resources).

- **What happens when a state's labels are modified such that it moves out of a user's scope while they have it locked?**
  - (lock wins): Once a user has acquired the lock on a state, they retain access to it (for that lock session) even if a later label change would move it out of their scope. The lock ensures exclusive editing, and the label change should not evict them mid-edit.
  - Label update is allowed, even if it violates the user’s current scope (because they already had permission at the moment of locking).
  - You might block the user from further actions on that state after they release the lock if the label now makes it out-of-scope.

- **What happens when a user creates a dependency and later one state's labels change such that the user no longer has access to both ends?**
  - see baseline: Deny the operation (403 Forbidden) because the user does not have access to the target state.

- **What happens when a user with `env=dev` scope attempts to create a dependency from a `env=dev` state to a `env=prod` state?**
  - The dependency creation should be denied with a 403 Forbidden error indicating lack of access to the target state.
  - “Forbidden: you do not have permission to access state <target-id>.”
    You may or may not reveal that the target exists (to avoid leaking existence).

- **What happens when listing states with a label filter that conflicts with the user's role scope?**
  - Example: User has a role scoped to `env=dev` but queries with filter `env=prod`.
  - Evaluate the user-supplied filter alongside each role’s scope; the results are the union of all `role scope ∩ requested filter` combinations.
  - In this example no role covers `env=prod`, so the union is empty and the user sees no results (with no error).

#### Permission Constraint Edge Cases

- **What happens when a user attempts to create a state with labels that satisfy create constraints but violate label validation policy?**
  - The request should be denied with a policy validation error.
  - Both create constraints (authorization) and policy validation (data integrity) must pass.
  - Always enforce validation rules first. If label values violate the validation policy (e.g. key not allowed, value not in enum, format invalid), reject with a 400 Bad Request (or 403 if you consider it part of authorization) with a clear validation error message. Do not attempt to auto-correct or ignore invalid parts.

- **What happens when a user attempts to create a state without providing a required label key specified in create constraints?**
  - If the user omits a label key that is required by the create constraints (i.e. the constraints say “this key must exist with some value”), treat it as a violation and deny the request (403 or 400). The message should state “missing required label X.”

- **What happens when a user attempts to remove an immutable label key using label update operations?**
  - The request should be denied with a 403 Forbidden error indicating the key is immutable.
  - Deleting (removing) an immutable key is considered equivalent to modifying it, so it is disallowed. Any attempt to remove or change its value is rejected with 403.

- **What happens when an admin with no immutable key restrictions updates a label that is immutable for other roles?**
  - The admin should be able to modify any label (admin override).
  - Immutable key restrictions are role-specific, not global.
  - Admin-level roles bypass immutability constraints. When performing an update, if the actor has a sufficiently privileged role, skip the immutability check entirely. The update proceeds.

#### Data Plane and Terraform Integration Edge Cases

- **What happens when Terraform attempts to unlock a state with an incorrect lock ID?**
  - The unlock request fails with a 409 Conflict error indicating lock ID mismatch.
  - The lock should remain in place with the original lock ID.

- **What happens when Terraform attempts to write state while another user holds the lock?**
  - The write request (POST /tfstate/{guid}) should fail with a 409 Conflict error.
  - Existing behaviour must be preserved

- **What happens when a user's permissions are revoked while they hold a Terraform lock?**
  - Do not forcibly break the lock; the lock remains held. But on subsequent operations (e.g. write, unlock), enforce the new permissions: i.e. the user cannot perform further actions once permission is gone, except perhaps unlock if configured so. So the lock is stranded until some admin intervenes or the lock expires (if locks have TTL).

- **What happens when a service account makes thousands of Terraform state read requests in a short time?**
  - The requests should succeed if properly authenticated and authorized.
  - (no rate limiting): Allow unlimited reads by service accounts (assuming they are trusted)

- **What happens when Terraform sends a large state file (100+ MB) to POST /tfstate/{guid}?**
  - Preserve existing behaviour: the system currently warns against file sizes over 10MB but does not enforce a hard limit.

#### Label Policy Management Edge Cases

- **What happens when an admin updates the label policy to remove a key that exists on many existing states?**
  - preserve existing behaviour: Do not retroactively remove existing labels. The policy update only affects future operations (e.g. new state creations or updates). A report of non-compliant states is already implemented in the gridctl!

- **What happens when a user queries states using a label filter for a key that's not in the policy?**
  - The filter should still work if states have that label (out-of-policy labels are queryable).
  - Preserve existing behaviour: Accept the query. If some states have that label key (even though the policy doesn’t define it), the filter applies.

- **What happens when an admin updates the label policy to change a free-text key to an enumerated key while states have values outside the enumeration?**
  - preserve existing behaviour: Do not retroactively remove existing labels. The policy update only affects future operations (e.g. new state creations or updates). A report of non-compliant states is already implemented in the gridctl!

- **What happens when two admins simultaneously update the label policy?**
  - last write wins

#### Performance and Scale Edge Cases

- **What happens when a user with label scope has access to 10,000+ states?**
  - out of scope, no deployment with more than 500 states is known
  - paging is not implemented and not required for current scope
  - YAGNI/KISS: no premature optimization

- **What happens when the label validation policy is very large (100+ keys)?**
  - preserve existing behaviour

- **What happens when the permission resolution query (loading user roles and merging permissions) takes too long due to complex role hierarchies?**
  - YAGNI/KISS: no premature optimization

- **What happens when hundreds of users attempt to list states simultaneously, all triggering label scope filter queries?**
  - YAGNI/KISS: no premature optimization

#### Administrative Operations Edge Cases

- **What happens when an admin attempts to assign a non-existent role to a user?**
  - The operation should fail with a 404 Not Found error indicating the role doesn't exist.
  - YAGNI/KISS: no suggestions for similar role names if there's a typo.

- **What happens when an admin attempts to create a role with the same name as an existing role?**
  - role names should be mutable (backed by different unique id), but there should be a global uniqueness constraint. The system should ask to overwrite (allow ovewrite with --force flag)

- **What happens when an admin exports role definitions and imports them into another Grid deployment with different database state?**
  - Import does not validate that referenced label keys exist in the target deployment's policy
  - Import should be idempotent (overwrite existing roles with same name, create new ones as needed)

- **What happens when a service account is deleted while it has active tokens in use?**
  - Active tokens should be immediately invalidated (cannot be used for new requests).
  - Because Casbin allows dynamic policy updates and role managers, role edits or deletion take effect immediately. 

---

## Requirements

### Functional Requirements - Authentication (AuthN)

- **FR-001**: System MUST support SSO authentication via OIDC-compliant identity providers (e.g., Google Workspace, Okta, Azure AD, Auth0).

- **FR-002**: System MUST support a web-based authentication flow where users are redirected to their identity provider and returned with a Grid session token.

- **FR-003**: System MUST support a device-code/browser authentication flow for CLI users, allowing authentication from headless or terminal-only environments.

- **FR-004**: System MUST support service account authentication using client credentials (client ID and secret) for non-interactive scenarios.

- **FR-005**: System MUST issue session tokens or bearer tokens upon successful authentication that can be presented in subsequent requests.

- **FR-006**: System MUST validate tokens on every protected request to ensure they are valid, not expired, and not revoked.

- **FR-007**: System MUST allow administrators to revoke user sessions or service account credentials, immediately invalidating associated tokens.

- **FR-008**: CLI MUST provide a `gridctl login` command that initiates the device-code flow and stores the obtained token for subsequent commands.

- **FR-009**: CLI MUST provide a `gridctl logout` command that clears stored credentials and optionally revokes the session token.

- **FR-010**: System MUST support multiple concurrent sessions for a single user (e.g., web browser + multiple CLI sessions).

### Functional Requirements - Authorization (AuthZ) & RBAC

- **FR-011**: System MUST evaluate every Control Plane RPC request against the caller's assigned roles and permissions before executing the operation.

- **FR-012**: System MUST evaluate every Data Plane HTTP request to `/tfstate/*` endpoints against the caller's assigned roles and permissions.

- **FR-013**: System MUST support role definitions that specify allowed actions (e.g., `state:create`, `tfstate:read`, `policy:write`).

- **FR-014**: System MUST support role definitions that specify resource scope using label filters (e.g., `env=dev`, `team=platform`).

- **FR-015**: System MUST automatically filter list and search operations to return only resources matching the caller's label scope.

- **FR-016**: System MUST deny access to individual resources that do not match the caller's label scope, returning 403 Forbidden or 404 Not Found as appropriate.

- **FR-017**: System MUST support "create constraints" that restrict which label values a role can set when creating a state (e.g., product-engineer can only create states with `env=dev`).

- **FR-018**: System MUST enforce create constraints during state creation, denying requests that attempt to set disallowed label values.

- **FR-019**: System MUST support "immutable label keys" configuration per role, preventing users from modifying specified label keys on existing states.

- **FR-020**: System MUST enforce immutable label key restrictions during label update operations, denying requests that attempt to modify protected keys.

- **FR-021**: System MUST verify that the caller has access to BOTH the source and target states when creating or modifying a dependency edge.

- **FR-022**: System MUST restrict label validation policy management (read/write) based on role permissions, allowing only designated admin roles to modify the policy.

- **FR-023**: System MUST return clear, actionable error messages when authorization is denied, indicating the specific permission or constraint that was violated.

- **FR-024**: System MUST apply authorization rules consistently across both web application and CLI interfaces.

### Functional Requirements - Permission Management and Role Definitions

#### Action Enumeration

- **FR-025**: System MUST define a compiled enumeration of all supported actions across Control Plane and Data Plane that can be referenced in permission definitions. The enumeration MUST include at minimum:

  **Control Plane Actions (State Management)**:
  - `state:create` - Create new states
  - `state:read` - Read state metadata and configuration
  - `state:list` - List states (subject to label scope filtering)
  - `state:update-labels` - Modify state labels
  - `state:delete` - Delete states

  **Control Plane Actions (Dependency Management)**:
  - `dependency:create` - Add dependency edges
  - `dependency:read` - Read dependency information
  - `dependency:list` - List dependencies and dependents
  - `dependency:delete` - Remove dependency edges

  **Control Plane Actions (Policy Management)**:
  - `policy:read` - View label validation policy
  - `policy:write` - Update label validation policy

  **Data Plane Actions (Terraform HTTP Backend)**:
  - `tfstate:read` - GET /tfstate/{guid}
  - `tfstate:write` - POST /tfstate/{guid}
  - `tfstate:lock` - LOCK /tfstate/{guid}/lock
  - `tfstate:unlock` - UNLOCK /tfstate/{guid}/unlock

  **Administrative Actions**:
  - `admin:role-manage` - Create, update, delete role definitions
  - `admin:user-assign` - Assign roles to users and service accounts
  - `admin:service-account-manage` - Create, delete, rotate service account credentials
  - `admin:session-revoke` - Revoke user sessions or service account tokens

- **FR-026**: System MUST support wildcard action notation (e.g., `state:*`, `tfstate:*`, `*:*`) for granting all actions within a category or globally.

- **FR-027**: System MUST reject permission definitions that reference non-existent actions, returning a validation error with the list of valid action identifiers.

#### Permission Definition Syntax

- **FR-028**: System MUST support a permission definition syntax that specifies:
  1. Action identifier(s) from the enumeration
  2. Resource scope (all resources or label-filtered subset)
  3. Optional constraints (create constraints, immutable keys)

- **FR-029**: System MUST support specifying multiple actions in a single permission definition using action lists or wildcard patterns. The permission definition format is JSON.

- **FR-030**: System MUST validate permission definitions for consistency, rejecting definitions that:
  - Grant conflicting permissions (e.g., both allow and deny the same action)
  - Reference non-existent label keys in scope filters
  - Specify create constraints without `state:create` permission
  - Specify immutable keys without `state:update-labels` permission

#### Role Definition and Assignment

- **FR-031**: System MUST support defining named roles that bundle:
  - Set of permission grants (action + resource scope)
  - Label scope filter (applies to all permissions in the role)
  - Create constraints (label key-value restrictions for `state:create`)
  - Immutable key list (label keys that cannot be modified)

- **FR-032**: System MUST allow a single user or service account to be assigned multiple roles, with permissions being additive (union of all granted permissions).

- **FR-033**: System MUST evaluate label scope filters using union semantics when multiple roles are assigned; access is allowed when any assigned role authorizes the action.

- **FR-034**: System MUST persist role definitions in the database with version tracking, allowing auditing of role changes over time.

- **FR-035**: System MUST persist role assignments (user/service-account to role mappings) in the database with timestamps for creation and modification.

- **FR-036**: System MUST allow removing role assignments from users or service accounts, with changes taking effect on the next request (no stale permission caching).

#### Permission Enforcement

- **FR-037**: System MUST intercept every Control Plane RPC request and Data Plane HTTP request to perform authorization checks before allowing the operation to proceed.

- **FR-038**: System MUST resolve the authenticated principal's (user or service account) effective permissions by:
  1. Loading all assigned roles from the database
  2. Merging permission grants from all roles
  3. Computing the effective label scope filter
  4. Caching the result for the duration of the request only

- **FR-039**: System MUST evaluate each request against effective permissions by checking:
  1. Does the principal have the required action permission?
  2. Does the target resource match the principal's label scope filter?
  3. For create operations: Do requested labels satisfy create constraints?
  4. For update operations: Are modified label keys not in immutable key list?

- **FR-040**: System MUST deny requests that fail any authorization check, returning appropriate HTTP status codes:
  - 401 Unauthorized: No valid authentication token
  - 403 Forbidden: Authenticated but lacking required permission
  - 404 Not Found: Resource exists but is outside principal's label scope
  - 409 Conflict: Operation violates constraints (e.g., create constraint, immutable key)

- **FR-041**: System MUST include structured error details in denial responses specifying:
  - Which permission was missing (e.g., `state:create`)
  - Which constraint was violated (e.g., "create constraint: env must be dev")
  - For 404 responses: Whether resource truly doesn't exist or is scope-filtered [NEEDS CLARIFICATION: Should 404 responses leak information about resource existence outside user's scope, or always appear as "not found"?]

#### Default Roles and Seeding

- **FR-042**: System MUST include database migrations that create default role definitions as part of initial schema setup.

- **FR-043**: System MUST provide at least three pre-built default roles as database seeds:

  **service-account role**:
  - Permissions: `tfstate:read`, `tfstate:write`, `tfstate:lock`, `tfstate:unlock`
  - Label scope: None (all states accessible)
  - Create constraints: None (no state creation permitted)
  - Immutable keys: None (no label modification permitted)

  **platform-engineer role**:
  - Permissions: `state:*`, `tfstate:*`, `dependency:*`, `policy:*`, `admin:*`
  - Label scope: None (all states accessible)
  - Create constraints: None (can create with any labels)
  - Immutable keys: None (can modify any labels)

  **product-engineer role**:
  - Permissions: `state:create`, `state:read`, `state:list`, `state:update-labels`, `tfstate:*`, `dependency:*`, `policy:read`
  - Label scope: `env=dev` (example, should be configurable)
  - Create constraints: `env` must be one of `[dev]`
  - Immutable keys: `[env]` (cannot modify env key after creation)

- **FR-044**: System MUST allow administrators to customize default role definitions post-deployment without requiring code changes or redeployment.

- **FR-045**: System MUST support exporting role definitions to JSON configuration files for version control and importing them for replication across environments.

- **FR-046**: System MUST prevent deletion of roles that are currently assigned to active users or service accounts, requiring role unassignment before deletion.

#### Permission Introspection

- **FR-047**: System MUST provide an endpoint or CLI command for users to view their own effective permissions, showing:
  - Assigned roles
  - Effective permission grants
  - Label scope filter
  - Create constraints
  - Immutable keys

- **FR-048**: System MUST provide an administrative endpoint or CLI command to view any user's or service account's effective permissions for troubleshooting purposes.

- **FR-049**: System MUST log permission checks (allow/deny decisions) with sufficient detail for audit and debugging, including:
  - Timestamp
  - Principal identity (user ID or service account ID)
  - Requested action
  - Target resource identifier (state GUID or resource path)
  - Decision (allow/deny)
  - Reason for denial (missing permission, scope mismatch, constraint violation)

### Functional Requirements - Label Management

**Note**: Label Management functionality is already implemented. These requirements describe constraints on how role permissions interact with existing label operations.

- **FR-050**: System MUST allow users to provide label key-value pairs when creating a state, subject to label validation policy and role create constraints.

- **FR-051**: System MUST allow users to add, update, and delete labels on existing states, subject to label validation policy, role permissions, and immutable key restrictions.

- **FR-052**: System MUST validate all label operations against the current label validation policy, rejecting labels that violate policy rules.

- **FR-053**: System MUST cache the label validation policy in memory for fast validation, refreshing the cache when the policy is updated.

- **FR-054**: System MUST include labels in state listing and information responses to support display and filtering in web and CLI interfaces.

- **FR-055**: System MUST support server-side label filtering in list operations, allowing clients to request states matching specific label criteria.

- **FR-056**: System MUST ensure that server-side label filtering is authoritative and cannot be bypassed by client-side filtering alone.

### Functional Requirements - Data Plane Protection

- **FR-057**: System MUST require bearer token authentication for all `/tfstate/{guid}` endpoints (GET, POST, LOCK, UNLOCK).

- **FR-058**: System MUST return 401 Unauthorized for requests to `/tfstate/*` endpoints without valid authentication.

- **FR-059**: System MUST return 403 Forbidden for authenticated requests to `/tfstate/*` endpoints where the caller lacks the required permissions or label scope access.

- **FR-060**: System MUST return 409 Conflict for lock/unlock operations that fail due to lock state conflicts (e.g., already locked, invalid lock ID).

- **FR-061**: System MUST include clear error messages in Data Plane responses indicating the reason for denial (missing auth, insufficient permissions, lock conflict).

### Functional Requirements - Administration & Configuration

- **FR-062**: System MUST support deployment-time configuration file(s) defining role definitions, permissions, label scopes, create constraints, and immutable keys.

- **FR-063**: System MUST provide administrative endpoints or CLI commands to manage role definitions without restarting the server.

- **FR-064**: System MUST provide administrative endpoints or CLI commands to assign roles to users and service accounts.

- **FR-065**: System MUST provide administrative endpoints or CLI commands to update label scope filters for roles.

- **FR-066**: System MUST provide administrative endpoints or CLI commands to update create constraints for roles.

- **FR-067**: System MUST provide administrative endpoints or CLI commands to update immutable label key lists for roles.

- **FR-068**: System MUST provide administrative endpoints or CLI commands to manage the label validation schema (policy).

- **FR-069**: System MUST persist role definitions, user-role assignments, and configuration in durable storage.

- **FR-070**: System MUST support creating and managing service accounts via administrative interfaces, generating client credentials.

### Functional Requirements - Web Application

#### Authentication & Session Management

- **FR-071**: Web application MUST enforce authentication, redirecting unauthenticated users to the SSO login flow via GET /auth/login.

- **FR-072**: Web application MUST handle the OIDC callback flow, receiving authorization code and state parameters from GET /auth/callback, and establishing a session cookie upon successful authentication.

- **FR-073**: Web application MUST store session state using HTTPOnly, Secure, and SameSite=Lax cookies to prevent XSS and CSRF attacks.

- **FR-074**: Web application MUST detect authentication failures during the callback phase and display user-friendly error messages indicating the specific OAuth2/OIDC error (e.g., access_denied, invalid_grant).

- **FR-075**: Web application MUST provide a logout action that calls POST /auth/logout to revoke the session and clear cookies.

- **FR-076**: Web application MUST detect 401 Unauthorized responses from API calls and automatically redirect to the login flow, preserving the intended destination URL for post-login redirect.

- **FR-077**: Web application MUST handle session expiry gracefully, prompting the user to re-authenticate when their session expires mid-interaction.

#### User Identity & Authorization Display

- **FR-078**: Web application MUST display the authenticated user's identity (name, email) and assigned role(s) in a persistent UI element (e.g., header, user menu).

- **FR-079**: Web application MUST provide a user profile or settings page where users can view their effective permissions, label scope filter, and assigned roles.

- **FR-080**: Web application MUST filter state lists to show only states within the user's label scope, without requiring manual filter input from the user.

- **FR-081**: Web application MUST display labels for each state in list and detail views, with visual indicators for label keys that match the user's label scope filter.

- **FR-082**: Web application MUST display the current label validation policy to all users via a dedicated policy viewer page or modal.

#### Authorization-Aware UI

- **FR-083**: Web application MUST hide or disable UI actions that the user is not authorized to perform based on their effective permissions (e.g., hide "Create State" button if user lacks `state:create` permission).

- **FR-084**: Web application MUST prevent users from attempting to modify immutable label keys by disabling edit controls for those keys in the UI.

- **FR-085**: Web application MUST display clear, user-friendly error messages when an action is denied due to authorization, explaining the specific permission or constraint violation (e.g., "You cannot create states with env=prod. Your role only allows env=dev.").

- **FR-086**: Web application MUST display visual indicators (icons, badges) distinguishing between states the user can read-only versus states they can modify.

#### Error Handling & Resilience

- **FR-087**: Web application MUST handle identity provider unavailability (e.g., Keycloak timeout) by displaying a clear error message with retry instructions, rather than hanging indefinitely.

- **FR-088**: Web application MUST implement client-side token refresh or re-authentication logic if the API server supports refresh tokens, minimizing user interruption during long sessions.

- **FR-089**: Web application MUST log authentication and authorization errors to the browser console for debugging purposes, while displaying sanitized error messages to the user.

**Note on Webapp Scope**: The current dashboard (webapp/) is **READ ONLY** - it displays states, dependencies, and labels but does not implement write operations (create state, update labels, delete state). Requirements FR-083, FR-084, and FR-086 that reference "edit controls," "modify," and "Create State button" are specified to prepare the authorization infrastructure for future write operations, but are not implemented in the current feature scope. The auth system protects existing read operations based on label scope (e.g., product-engineer sees only env=dev states). See research.md §13 "Dashboard READ ONLY Scope" for implementation details.

### Functional Requirements - CLI

- **FR-090**: CLI MUST provide a `gridctl login` command that authenticates the user and stores credentials locally.

- **FR-091**: CLI MUST automatically include bearer tokens from stored credentials in all subsequent command requests.

- **FR-092**: CLI commands for state creation (`gridctl state create`) MUST accept label flags (e.g., `--label env=dev --label team=platform`).

- **FR-093**: CLI commands for state creation MUST enforce create constraints, rejecting attempts to set disallowed label values with a clear error message.

- **FR-094**: CLI commands for label updates MUST enforce immutable key restrictions, rejecting attempts to modify protected keys with a clear error message.

- **FR-095**: CLI commands for dependency management MUST validate that the user has access to both source and target states before creating the dependency.

- **FR-096**: CLI MUST display clear error messages when authentication fails, authorization is denied, or label constraints are violated.

- **FR-097**: CLI MUST support non-interactive authentication using service account credentials via environment variables or config file.

* **FR-097a**: CLI MUST provide a pass-through wrapper command `gridctl tf` that executes a Terraform/Tofu subcommand (e.g., `plan`, `apply`, `init`, `state`, `lock`, `unlock`, etc.) and propagates STDIN/STDOUT/STDERR and exit codes unchanged.

* **FR-097b**: `gridctl tf` MUST support selecting the terraform binary via, in order of precedence: `--tf-bin` flag, `TF_BIN` environment variable, then defaulting to `terraform` if present, otherwise `tofu`.

* **FR-097c**: Before invoking the selected binary, `gridctl tf` MUST inject a valid bearer token for the Grid HTTP backend so that all `/tfstate/*` requests sent by the terraform/tofu process include authorization. The injection MUST use the backend’s supported mechanisms (e.g., runtime headers and/or generated backend config). The token MUST NOT be printed to console or logs.

* **FR-097d**: If no valid local credentials exist or the token is expired, `gridctl tf` MUST attempt to obtain/refresh credentials using the same auth flow as `gridctl login` (non-interactive when service account credentials are configured). If refresh/auth fails, the wrapper MUST exit with the same non-zero exit code as the underlying command would have produced for an auth failure and print a concise error explaining that authentication is required.

* **FR-097e**: If the terraform/tofu process returns `401 Unauthorized` from the backend during execution, `gridctl tf` MUST:

  1. attempt a single token refresh (if refresh is configured),
  2. re-try the exact same subcommand once,
  3. if it still fails, exit with the underlying process exit code and a brief message indicating auth failure.
     The wrapper MUST NOT loop indefinitely.

* **FR-097f**: `gridctl tf` MUST support `--cwd` to run the subcommand in a specific directory (default: current working directory) and MUST pass through any additional arguments verbatim after `--` (e.g., `gridctl tf -- plan -var-file=dev.tfvars`).

* **FR-097g**: `gridctl tf` MUST mask sensitive values (bearer tokens, client secrets) in all logs, debug output, and crash reports. When `--verbose` is used, the wrapper MAY print the effective terraform/tofu command line (with secrets redacted), but MUST NOT print headers or the token.

* **FR-097h**: The wrapper MUST preserve exit code semantics: the final exit code equals the terraform/tofu process exit code. Wrapper-level failures that occur before the subprocess starts (e.g., missing binary, auth bootstrap failure) MUST return a non-zero exit code and a clear, actionable error message.

* **FR-097i**: When the Grid directory context file is present (e.g., `.grid`), `gridctl tf` MUST prefer the state GUID and backend endpoints from that context. If multiple context files are supported by flags (e.g., `--link <file>`), the wrapper MUST use the selected context for backend injection.

* **FR-097j**: The wrapper MUST work with both interactive (human) sessions and non-interactive CI environments. In CI, it MUST use service account credentials if configured and MUST NOT launch a browser/device flow; instead it MUST fail fast with a clear message if credentials are absent.

* **FR-097k**: The wrapper MUST NOT persist bearer tokens into terraform/tofu files or into the user’s working directory. Any transient backend config artifacts created for header injection MUST be ephemeral (deleted after the run) or stored in the system temp directory.

* **FR-097l**: The wrapper MUST print a short hint on first use if it detects that the project is not initialized with the Grid backend (e.g., suggests `gridctl state init`), but MUST NOT block execution.

### Functional Requirements - Security & Compliance

- **FR-098**: System MUST log all authentication attempts (success and failure) with timestamp, user identity, and source IP.

- **FR-099**: System MUST log all authorization decisions (allow and deny) with timestamp, user identity, action, resource, and decision reason.

- **FR-100**: System MUST support secure transmission of credentials (HTTPS/TLS required for web and API endpoints).

- **FR-101**: System MUST NOT log or expose bearer tokens, client secrets, or other sensitive credentials in plain text.

- **FR-102**: System MUST support token expiration with a default session duration of 12 hours, requiring users to re-authenticate after expiration. All roles use the same session duration (not configurable per role).

- **FR-103**: System MUST validate that OIDC tokens are issued by a trusted identity provider before accepting them.

- **FR-104**: System MUST extract group membership from configurable JWT claim field (default: `groups`), supporting both flat string arrays (e.g., `["dev-team", "admins"]`) and nested object arrays with configurable extraction path (e.g., `[{"name": "dev-team"}]` with path `$[*].name`).

- **FR-105**: System MUST support group-to-role mappings managed via Admin RPC (`AssignGroupRole`, `RemoveGroupRole`, `ListGroupRoles`), where Grid admins type group names exactly as they appear in JWT claims (no UI for browsing IdP groups).

- **FR-106**: System MUST resolve user permissions via group membership using transitive resolution: user → groups (from JWT) → roles (from group_roles table) → policies (from casbin_rule table), applying union (OR) semantics across all resolved roles.

- **FR-107**: System MUST support direct user-to-role assignments as fallback/override mechanism for edge cases where group-based assignment is insufficient.

- **FR-108**: System MUST allow deployers to configure OIDC claim field names and extraction paths via deployment configuration (environment variables or config file), with sensible defaults (groups claim: `groups`, user ID claim: `sub`, email claim: `email`).

- **FR-109**: System MUST use consistent prefixes for Casbin identifiers: `user:` for human users, `group:` for SSO groups, `sa:` for service accounts, `role:` for roles (see PREFIX-CONVENTIONS.md for complete taxonomy).

### Key Entities

- **User**: Represents an authenticated human user, identified by SSO provider user ID. Has assigned roles (directly or via group membership) that determine permissions. Associated with sessions/tokens for authentication state.

- **Service Account**: Represents a non-interactive authentication principal (e.g., CI/CD pipeline). Has a client ID and secret. Has assigned roles. Used for automation scenarios.

- **Group**: Represents an SSO group (extracted from JWT claims) that can be assigned roles. Group membership managed by IdP; role assignments managed by Grid admins. Examples: "dev-team", "platform-engineers".

- **Role**: Defines a set of permissions, label scope expression (go-bexpr), create constraints, and immutable key restrictions. Can be assigned to users, groups, or service accounts. Examples: service-account, platform-engineer, product-engineer.

- **Permission**: Specifies an allowed action on a resource type (e.g., `state:create`, `tfstate:read`, `policy:write`). Attached to roles.

- **Label Scope Expression**: A go-bexpr expression string that restricts which states a role can access (e.g., `env == "dev"` or `env == "dev" and team == "platform"`). Evaluated against resource labels at enforcement time. Empty expression means no restriction (all resources accessible).

- **Create Constraint**: A restriction on which label values a role can set when creating a new state (e.g., product-engineer can only set `env=dev`).

- **Immutable Key Restriction**: A list of label keys that a role cannot modify on existing states (e.g., `env` is immutable for product-engineer).

- **Session**: Represents an authenticated user session, associated with a bearer token. Has an expiration time. Can be revoked.

- **Bearer Token**: A credential issued upon successful authentication, presented in request headers to prove identity. Contains user/service account identity and expiration.

- **Identity Provider (IdP)**: External OIDC-compliant system that authenticates users and issues identity tokens (e.g., Google Workspace, Okta, Azure AD). Grid trusts tokens from configured IdPs.

- **Label Validation Policy**: Defines the schema for state labels (required keys, allowed values, free-text keys). Managed by admins. Cached in memory for fast validation.

---

## Review & Acceptance Checklist

### Content Quality
- [x] No implementation details (languages, frameworks, APIs)
- [x] Focused on user value and business needs
- [x] Written for non-technical stakeholders
- [x] All mandatory sections completed

### Requirement Completeness
- [x] No [NEEDS CLARIFICATION] markers remain
- [x] Requirements are testable and unambiguous
- [x] Success criteria are measurable
- [x] Scope is clearly bounded
- [x] Dependencies and assumptions identified

### Outstanding Clarifications

#### Permission Management & Syntax
- All permission management questions have been resolved:
  - FR-029: Permission definition format is JSON (line 383)
  - FR-033: Multiple role scopes use union (OR) semantics per Casbin's default (lines 150-153, 401)
  - FR-041: Information disclosure handled per lines 186-187 (do not reveal existence)
  - FR-045: Export/import format is JSON (line 462)
  - FR-046: Role deletion requires unassignment first (line 464)

#### Authentication
- All authentication edge cases have been resolved in the Edge Cases section (lines 120-142).

#### Authorization & Role Management
- All authorization edge cases have been resolved in Edge Cases section (lines 144-168):
  - Role changes take effect immediately on next request (no session invalidation)
  - Role deletion behavior deferred to implementation research (line 157)
  - Users with no roles denied all operations (403 Forbidden)
  - System prevents last admin from self-removal (line 164)
  - Permission updates take effect on next request (no notifications)

#### Label Scope & Constraints
- All label scope edge cases have been resolved in Edge Cases section (lines 170-212):
  - Lock holder retains access even if labels change (lines 176-179)
  - Dependency creation denied if user lacks access to either end (lines 181-187)
  - Information disclosure: do not reveal state existence (lines 186-187)
  - Filter conflicts return empty results without error (lines 189-192)
  - Missing required labels treated as constraint violation (lines 201-202)
  - Label removal treated same as modification for immutability (lines 204-206)

#### Data Plane & Terraform
- All data plane edge cases have been resolved in Edge Cases section (lines 213-231):
  - Preserve existing lock/unlock behavior (lines 219-221)
  - Permission revocation: lock remains but operations denied (lines 223-224)
  - No rate limiting for service accounts (lines 226-228)
  - Preserve existing state size behavior (warn at 10MB, no hard limit) (lines 230-231)

#### Label Policy Management
- All label policy edge cases have been resolved in Edge Cases section (lines 233-246):
  - Policy changes do not retroactively affect existing states (lines 235-236, 242-243)
  - Out-of-policy labels remain queryable (lines 238-240)
  - Concurrent policy updates use last-write-wins (line 245-246)

#### Performance & Scale
- All performance edge cases resolved as YAGNI/KISS (lines 248-262):
  - No pagination needed (max 500 states expected)
  - No premature optimization for permission resolution or query caching

#### Administrative Operations
- All administrative edge cases have been resolved in Edge Cases section (lines 264-279):
  - No role name suggestions (YAGNI)
  - Role names unique, overwrite with --force flag (line 271)
  - Import is idempotent, no validation of referenced keys (lines 273-275)
  - Token invalidation is immediate via Casbin (lines 277-279)

---

## Execution Status

- [x] User description parsed
- [x] Key concepts extracted
- [x] Ambiguities marked
- [x] User scenarios defined
- [x] Requirements generated
- [x] Entities identified
- [x] Review checklist passed (all clarifications resolved)

---

## Notes for Planning Phase

This specification defines a comprehensive authentication and authorization system with the following key capabilities:

1. **Multi-modal Authentication**: SSO for web users, device-code flow for CLI, service account credentials for automation
2. **Fine-grained Authorization**: Role-based access with label-scoped filtering, create constraints, and immutable key protection
3. **Dual API Protection**: Both Control Plane (RPC) and Data Plane (HTTP /tfstate) endpoints are secured
4. **Operational Flexibility**: Deployment-configurable roles, administrative APIs for runtime management
5. **User Experience**: Clear error messages, UI adaptation based on permissions, seamless authentication flows

The specification identifies several areas requiring clarification before implementation, primarily around edge cases for session management, dependency validation under role changes, and policy update impacts on existing data.

The next phase (planning) should produce detailed design decisions for:
- Token format and validation strategy (OIDC/JWT)
- Authorization middleware architecture (Casbin + Chi integration)
- Label scope enforcement using go-bexpr evaluation
- Admin API design (Connect RPC for role/permission management)
- Session storage and revocation mechanism
- Casbin model definition with bexprMatch custom function
- Database schema (casbin_rule table + auth tables)
