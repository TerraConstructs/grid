# Tasks: Authentication, Authorization, and RBAC

**Feature**: 006-authz-authn-rbac
**Input**: Design documents from `/Users/vincentdesmet/tcons/grid/specs/006-authz-authn-rbac/`
**Prerequisites**: plan.md (required), research.md, data-model.md, contracts/, quickstart.md

## Overview

This task list implements comprehensive authentication (AuthN), authorization (AuthZ), and role-based access control (RBAC) for Grid's API server. The implementation protects both the Connect RPC Control Plane and the Terraform HTTP Backend Data Plane using:

- **Casbin** for policy-based authorization with go-bexpr label filtering
- **OIDC** for SSO authentication (Keycloak dev, Azure Entra ID prod)
- **OAuth2 Client Credentials** for service accounts
- **PKCE + Loopback** for CLI authentication
- **Group-to-Role Mapping** for enterprise SSO integration

**Key Architectural Decisions**:
- Casbin enforces permissions with union (OR) semantics via casbin/chi-authz middleware
- JWT tokens validated against IdP JWKS (no custom signing)
- Database stores: roles, user_roles, group_roles, service_accounts, sessions, casbin_rule
- Label-scoped access via go-bexpr expressions evaluated at enforcement time
- Three default roles seeded: service-account, platform-engineer, product-engineer
- Terraform HTTP Backend uses Basic Auth pattern (bearer token in password field)

## Format: `[ID] [P?] Description`
- **[P]**: Can run in parallel (different files, no dependencies)
- Include exact file paths in descriptions

## Phase 3.1: Setup & Foundation

### Database Schema

- [ ] **T001** Create database migration `YYYYMMDDHHMMSS_auth_tables.go` in `cmd/gridapi/internal/migrations/`
  - Create tables: users, service_accounts, roles, user_roles, group_roles, sessions, casbin_rule
  - Reference: data-model.md §1-7 for complete schema
  - casbin_rule table: Standard Casbin schema (id, ptype, v0-v5) per research.md §7
  - Unique index on casbin_rule: (ptype, v0, v1, v2, v3, v4, v5)
  - Check constraints: user_roles and sessions must reference exactly one identity type
  - Partial unique indexes: user_roles (user_id, role_id) and (service_account_id, role_id)
  - GIN index on service_accounts.scope_labels: OUT OF SCOPE (evaluation in Go, not SQL)

- [ ] **T002** Create seed migration `YYYYMMDDHHMMSS_seed_auth_data.go` in `cmd/gridapi/internal/migrations/`
  - Seed 3 default roles into roles table: service-account, platform-engineer, product-engineer
  - Seed default Casbin policies into casbin_rule table with go-bexpr expressions
  - Reference: data-model.md §Seed Data (lines 449-526), research.md §7 (lines 493-543)
  - service-account policies: tfstate:* on state with empty scopeExpr
  - platform-engineer: wildcard (*:*) with empty scopeExpr
  - product-engineer: state:*/tfstate:*/dependency:* with scopeExpr `env == "dev"`

### Configuration

- [ ] **T003** Update config struct in `cmd/gridapi/internal/config/config.go`
  - Add OIDCConfig struct with fields: Issuer, ClientID, ClientSecret, RedirectURI
  - Add claim extraction config: GroupsClaimField (default: "groups"), GroupsClaimPath (optional, for nested)
  - Add UserIDClaimField (default: "sub"), EmailClaimField (default: "email")
  - Add CasbinModelPath string field (path to model.conf)
  - Reference: research.md §2 (lines 622-635), CLARIFICATIONS.md §1 (JWT claim config)

### Casbin Foundation

- [ ] **T004** Create Casbin model file `cmd/gridapi/casbin/model.conf`
  - Define RBAC model with custom bexprMatch function in matcher
  - Model: `r = sub, objType, act, labels` / `p = role, objType, act, scopeExpr, eft`
  - Matcher: `g(r.sub, p.role) && r.objType == p.objType && r.act == p.act && bexprMatch(p.scopeExpr, r.labels)`
  - Reference: research.md §1 (lines 66-86), plan.md §207

- [ ] **T005** Create Casbin enforcer initialization in `cmd/gridapi/internal/auth/casbin.go`
  - Function: `InitEnforcer(db *bun.DB, modelPath string) (*casbin.Enforcer, error)`
  - Use msales/casbin-bun-adapter with existing *bun.DB instance
  - Register custom bexprMatch function with sync.Map caching for compiled evaluators
  - Load policies from database via enforcer.LoadPolicy()
  - Reference: research.md §1 (lines 429-490), §7 (adapter usage)

- [ ] **T006** Create go-bexpr evaluator helper in `cmd/gridapi/internal/auth/bexpr.go`
  - Function: `evaluateBexpr(scopeExpr string, labels map[string]any) bool`
  - Cache compiled evaluators using sync.Map (expression string → *bexpr.Evaluator)
  - Empty scopeExpr returns true (no constraint)
  - Handle errors gracefully (invalid expressions return false)
  - Reference: research.md §1 (lines 443-482)

- [ ] **T007** Create Casbin identifier helpers in `cmd/gridapi/internal/auth/identifiers.go`
  - Define prefix constants: PrefixUser ("user:"), PrefixGroup ("group:"), PrefixServiceAccount ("sa:"), PrefixRole ("role:")
  - Helper functions: UserID(id string), GroupID(name string), ServiceAccountID(id string), RoleID(name string)
  - Parse functions: ExtractUserID(principal string), ExtractRoleID(principal string)
  - Reference: PREFIX-CONVENTIONS.md §2-3, §9 (helper function patterns)

- [ ] **T008** Create action constants file in `cmd/gridapi/internal/auth/actions.go`
  - Define all action constants from authorization-design.md
  - Control Plane: StateCreate, StateRead, StateList, StateUpdateLabels, StateDelete
  - Data Plane: TfstateRead, TfstateWrite, TfstateLock, TfstateUnlock
  - Admin: AdminRoleManage, AdminUserAssign, AdminGroupAssign, AdminServiceAccountManage, AdminSessionRevoke
  - Wildcards: StateWildcard, TfstateWildcard, AdminWildcard, AllWildcard
  - Reference: plan.md §288-330

## Phase 3.2: Auth Core (Token Validation & Session Management)

- [ ] **T009** [P] Create OIDC client wrapper in `cmd/gridapi/internal/auth/oidc.go`
  - Function: `InitOIDCProvider(ctx context.Context, issuer, clientID string) (*oidc.Provider, *oidc.IDTokenVerifier, error)`
  - Use go-oidc to fetch discovery metadata and JWKS
  - Return provider and configured verifier for token validation
  - Cache JWKS automatically via go-oidc internals
  - Reference: research.md §2 (lines 112-165)

- [ ] **T010** [P] Create JWT validation function in `cmd/gridapi/internal/auth/jwt.go`
  - Function: `ValidateJWT(ctx context.Context, rawToken string, verifier *oidc.IDTokenVerifier) (*oidc.IDToken, error)`
  - Verify signature against JWKS
  - Validate claims: iss, aud, exp, nbf, iat
  - Return parsed IDToken for claim extraction
  - Reference: research.md §6 (lines 313-375)

- [ ] **T011** [P] Create JWT claim extraction in `cmd/gridapi/internal/auth/claims.go`
  - Function: `ExtractGroups(claims map[string]interface{}, claimField, claimPath string) ([]string, error)`
  - Support flat arrays: `["dev-team", "contractors"]`
  - Support nested objects with mapstructure: `[{"name": "dev-team"}]` with path="name"
  - Handle missing/invalid claims gracefully
  - Reference: research.md §8 (lines 619-694), CLARIFICATIONS.md §1

- [ ] **T012** [P] Create dynamic Casbin grouping at auth time in `cmd/gridapi/internal/auth/grouping.go`
  - Function: `ApplyDynamicGroupings(enforcer *casbin.Enforcer, userID string, groups []string, groupRoles map[string][]string)`
  - For each group in JWT: create transient grouping `user:alice → group:dev-team`
  - For each group-role mapping: create grouping `group:dev-team → role:product-engineer`
  - Groupings are transient (not persisted in casbin_rule table)
  - Reference: data-model.md §5 (lines 290-317), CLARIFICATIONS.md §1 (lines 59-67)

### Repository Layer

- [ ] **T013** Create session repository in `cmd/gridapi/internal/repository/session_repository.go`
  - Interface methods: CreateSession, GetSessionByTokenHash, UpdateLastUsed, RevokeSession, RevokeAllSessionsForUser, RevokeAllSessionsForServiceAccount
  - Schema: data-model.md §3 (lines 174-199)
  - Token hash stored as SHA256 hex (64 chars)
  - Check sessions.revoked column on validation (FR-102a)
  - Cascade revocation: RevokeAllSessionsForServiceAccount sets revoked=true for all sessions (FR-070b)
  - Reference: data-model.md §3, plan.md §632-637

- [ ] **T014** Create service account repository in `cmd/gridapi/internal/repository/service_account_repository.go`
  - Interface methods: CreateServiceAccount, GetByClientID, UpdateLastUsed, SetDisabled, RotateSecret
  - Bcrypt hash client secrets with cost 10
  - Schema: data-model.md §2 (lines 141-172)
  - Rotation behavior (FR-070a): Updating secret does NOT invalidate existing tokens (JWT self-contained)
  - Update secret_rotated_at timestamp on rotation
  - Reference: research.md §4 (lines 221-285), plan.md §638-644

- [ ] **T015** [P] Create group-role repository in `cmd/gridapi/internal/repository/group_role_repository.go`
  - Interface methods: CreateGroupRole, DeleteGroupRole, ListGroupRoles, GetRolesByGroup
  - Schema: data-model.md §5 (lines 235-265)
  - Unique constraint: (group_name, role_id)
  - Used for static group→role mappings (IdP manages group membership)
  - Reference: CLARIFICATIONS.md §1 (lines 48-68)

- [ ] **T016** [P] Create auth repository interface in `cmd/gridapi/internal/repository/auth_repository.go`
  - Combine all auth-related repository interfaces
  - SessionRepository, ServiceAccountRepository, GroupRoleRepository, RoleRepository, UserRoleRepository
  - Follow existing repository/interface.go pattern
  - Reference: plan.md §645

## Phase 3.3: Middleware (Request Interception)

- [ ] **T017** [P] Create authentication middleware in `cmd/gridapi/internal/middleware/authn.go`
  - Extract JWT from Authorization header or session cookie
  - Validate JWT using auth/jwt.go functions
  - Extract user ID and groups from claims
  - Apply dynamic Casbin groupings (user→groups→roles)
  - Check session revocation status (sessions.revoked=false)
  - Inject user identity into request context
  - Reference: research.md §6 (JWT flow), plan.md §649-656

- [ ] **T018** [P] Create authorization middleware in `cmd/gridapi/internal/middleware/authz.go`
  - Integrate casbin/chi-authz middleware
  - Custom subject extraction from context (set by authn middleware)
  - For /tfstate/* endpoints: extract bearer token from HTTP Basic Auth password field (username ignored)
  - Lock-aware bypass (FR-061a): If principal holds lock, bypass label scope for tfstate:write/unlock
  - Use enforcer.Enforce(subject, objType, action, labels) for authorization
  - Reference: research.md §1 (lines 13-46), plan.md §657-665

- [ ] **T019** [P] Create context helpers in `cmd/gridapi/internal/middleware/context.go`
  - Functions: SetUserContext, GetUserFromContext, SetGroupsContext, GetGroupsFromContext
  - Store user ID, groups, and roles in request context
  - Type-safe context keys (avoid string keys)
  - Reference: Standard Go context.WithValue patterns

## Phase 3.4: HTTP Endpoints (OIDC Flow)

- [ ] **T020** [P] Create login handler in `cmd/gridapi/internal/server/auth_handlers.go` (function: HandleLogin)
  - GET /auth/login - Initiate OIDC authorization code flow
  - Generate CSRF state token (crypto/rand)
  - Build authorization URL with state and redirect_uri
  - Redirect to IdP's authorization endpoint
  - Reference: research.md §2 (OIDC flow), contracts/http-endpoints.yaml (lines 30-58)

- [ ] **T021** [P] Create callback handler in `cmd/gridapi/internal/server/auth_handlers.go` (function: HandleCallback)
  - GET /auth/callback - OIDC callback handler
  - Validate state parameter (CSRF protection)
  - Exchange authorization code for tokens
  - Create session in database
  - Set HTTPOnly session cookie or return token JSON (CLI flow)
  - Reference: research.md §2, contracts/http-endpoints.yaml (lines 60-120)

- [ ] **T022** [P] Create token handler in `cmd/gridapi/internal/server/auth_handlers.go` (function: HandleToken)
  - POST /auth/token - OAuth2 client credentials flow
  - Validate client_id and client_secret against service_accounts table
  - Issue access token (12-hour expiry)
  - Create session record for service account
  - Reference: research.md §4 (lines 232-262), contracts/http-endpoints.yaml (lines 122-171)

- [ ] **T023** [P] Create logout handler in `cmd/gridapi/internal/server/auth_handlers.go` (function: HandleLogout)
  - POST /auth/logout - Revoke session
  - Delete session from database or set revoked=true
  - Clear session cookie
  - Reference: contracts/http-endpoints.yaml (lines 173-202)

- [ ] **T024** [P] Create device code init handler in `cmd/gridapi/internal/server/auth_handlers.go` (function: HandleDeviceCodeInit)
  - POST /auth/device/code - Initiate device authorization flow (RFC 8628)
  - Generate device_code and user_code
  - Store in temporary storage with expiry (e.g., Redis or in-memory map)
  - Return verification_uri and codes
  - Reference: contracts/http-endpoints.yaml (lines 204-238), quickstart.md §2 (device flow note)

- [ ] **T025** [P] Create device token poll handler in `cmd/gridapi/internal/server/auth_handlers.go` (function: HandleDeviceTokenPoll)
  - POST /auth/device/token - Poll for device authorization completion
  - Check device_code status (pending, authorized, denied, expired)
  - If authorized: exchange for access token, create session
  - Return appropriate error codes: authorization_pending, slow_down, access_denied, expired_token
  - Reference: contracts/http-endpoints.yaml (lines 240-279)

## Phase 3.5: Connect RPC Handlers (Admin Operations)

- [ ] **T026** [P] Implement CreateServiceAccount RPC in `cmd/gridapi/internal/server/connect_handlers.go`
  - Generate UUIDv4 for client_id
  - Generate random 32-byte client_secret (crypto/rand)
  - Hash secret with bcrypt (cost 10)
  - Store in service_accounts table
  - Return client_id and plaintext secret (only shown once)
  - Require admin:service-account-manage permission
  - Reference: contracts/auth-rpc-additions.proto (lines 12-23), plan.md §677-681

- [ ] **T027** [P] Implement ListServiceAccounts RPC in `cmd/gridapi/internal/server/connect_handlers.go`
  - Query service_accounts table (exclude disabled if needed)
  - Return list without client_secret_hash
  - Require admin:service-account-manage permission
  - Reference: contracts/auth-rpc-additions.proto (lines 25-41)

- [ ] **T028** [P] Implement RevokeServiceAccount RPC in `cmd/gridapi/internal/server/connect_handlers.go`
  - Set service_accounts.disabled=true
  - Call repository.RevokeAllSessionsForServiceAccount to invalidate all sessions (FR-070b)
  - Remove Casbin grouping rules for this service account
  - Require admin:service-account-manage permission
  - Reference: contracts/auth-rpc-additions.proto (lines 43-49), plan.md §683-688

- [ ] **T029** [P] Implement RotateServiceAccount RPC in `cmd/gridapi/internal/server/connect_handlers.go`
  - Generate new client_secret, hash with bcrypt
  - Update service_accounts.client_secret_hash and secret_rotated_at
  - DO NOT invalidate existing sessions (FR-070a: tokens are self-contained)
  - Return new plaintext secret (only shown once)
  - Require admin:service-account-manage permission
  - Reference: contracts/auth-rpc-additions.proto (lines 51-59), plan.md §638-644

- [ ] **T030** [P] Implement AssignRole RPC in `cmd/gridapi/internal/server/connect_handlers.go`
  - Insert into user_roles table (user_id or service_account_id based on principal_type)
  - Create Casbin grouping rule: `g, user:X, role:Y` (or `g, sa:X, role:Y`)
  - Call enforcer.SavePolicy() to persist
  - Require admin:user-assign permission
  - Reference: contracts/auth-rpc-additions.proto (lines 133-143), plan.md §689-693

- [ ] **T031** [P] Implement RemoveRole RPC in `cmd/gridapi/internal/server/connect_handlers.go`
  - Delete from user_roles table
  - Remove Casbin grouping rule
  - Call enforcer.SavePolicy()
  - Require admin:user-assign permission
  - Reference: contracts/auth-rpc-additions.proto (lines 145-153)

- [ ] **T032** [P] Implement AssignGroupRole RPC in `cmd/gridapi/internal/server/connect_handlers.go`
  - Insert into group_roles table (group_name, role_id)
  - Create static Casbin grouping: `g, group:X, role:Y` (persisted in casbin_rule)
  - Call enforcer.SavePolicy()
  - Require admin:group-assign permission
  - Reference: contracts/auth-rpc-additions.proto (lines 172-180), CLARIFICATIONS.md §1 (lines 94-121)

- [ ] **T033** [P] Implement RemoveGroupRole RPC in `cmd/gridapi/internal/server/connect_handlers.go`
  - Delete from group_roles table
  - Remove Casbin grouping rule
  - Call enforcer.SavePolicy()
  - Require admin:group-assign permission
  - Reference: contracts/auth-rpc-additions.proto (lines 182-189)

- [ ] **T034** [P] Implement ListGroupRoles RPC in `cmd/gridapi/internal/server/connect_handlers.go`
  - Query group_roles table (optionally filter by group_name)
  - Return list of group-role assignments with metadata
  - Require admin:group-assign permission (read-only operation)
  - Reference: contracts/auth-rpc-additions.proto (lines 191-204)

- [ ] **T035** [P] Implement GetEffectivePermissions RPC in `cmd/gridapi/internal/server/connect_handlers.go`
  - Resolve user→groups (from JWT or direct assignments)→roles→policies
  - Query Casbin for roles: enforcer.GetRolesForUser(principal)
  - Aggregate permissions from all roles (union/OR semantics)
  - Return aggregated actions, label_scope_exprs, create_constraints, immutable_keys
  - Reference: contracts/auth-rpc-additions.proto (lines 206-223), plan.md §706-709

- [ ] **T036** [P] Implement CreateRole RPC in `cmd/gridapi/internal/server/connect_handlers.go`
  - Insert into roles table (name, description, scope_expr, create_constraints, immutable_keys)
  - Create Casbin policies for each action in actions list
  - Validate label_scope_expr as valid go-bexpr syntax
  - Call enforcer.SavePolicy()
  - Require admin:role-manage permission
  - Reference: contracts/auth-rpc-additions.proto (lines 63-100), plan.md §710-714

- [ ] **T037** [P] Implement ListRoles RPC in `cmd/gridapi/internal/server/connect_handlers.go`
  - Query roles table (all roles)
  - Return role metadata (no need to join Casbin policies for basic list)
  - Reference: contracts/auth-rpc-additions.proto (lines 102-108)

- [ ] **T038** [P] Implement UpdateRole RPC in `cmd/gridapi/internal/server/connect_handlers.go`
  - Update roles table with optimistic locking (version check)
  - Update Casbin policies (delete old, insert new)
  - Validate label_scope_expr syntax
  - Call enforcer.SavePolicy()
  - Require admin:role-manage permission
  - Reference: contracts/auth-rpc-additions.proto (lines 110-122)

- [ ] **T039** [P] Implement DeleteRole RPC in `cmd/gridapi/internal/server/connect_handlers.go`
  - Check if role is assigned to any users/service accounts/groups (block if assigned)
  - Delete from roles table (cascade to user_roles via FK)
  - Delete all Casbin policies for this role
  - Call enforcer.SavePolicy()
  - Require admin:role-manage permission
  - Reference: contracts/auth-rpc-additions.proto (lines 124-130)

- [ ] **T040** [P] Implement ListSessions RPC in `cmd/gridapi/internal/server/connect_handlers.go`
  - Query sessions table for given user_id
  - Return active sessions (not revoked, not expired)
  - Include metadata: created_at, last_used_at, user_agent, ip_address
  - Reference: contracts/auth-rpc-additions.proto (lines 225-242)

- [ ] **T041** [P] Implement RevokeSession RPC in `cmd/gridapi/internal/server/connect_handlers.go`
  - Set sessions.revoked=true for given session_id
  - User can revoke their own sessions, admin can revoke any session
  - Require admin:session-revoke for revoking other users' sessions
  - Reference: contracts/auth-rpc-additions.proto (lines 244-250)

## Phase 3.6: CLI Authentication (Device Flow & Credential Storage)

- [ ] **T042** Create PKCE + loopback device flow in `cmd/gridctl/internal/auth/device_flow.go`
  - Generate PKCE code verifier (43-128 char random string)
  - Derive code challenge: SHA256(verifier) → Base64URL
  - Start local HTTP server on random port: net.Listen("127.0.0.1:0")
  - Build authorization URL with challenge and redirect_uri=http://127.0.0.1:<port>/callback
  - Open browser (or print URL if browser open fails)
  - Wait for callback with authorization code
  - Exchange code + verifier for tokens via /auth/callback
  - Shutdown local server after exchange
  - Reference: research.md §3 (lines 167-214), quickstart.md §2

- [ ] **T043** Create credential store in `cmd/gridctl/internal/auth/credential_store.go`
  - Store tokens in ~/.grid/credentials.json with mode 0600
  - Structure: {access_token, token_type, expires_at}
  - Functions: SaveCredentials, LoadCredentials, DeleteCredentials
  - Validate expiry on load, prompt re-login if expired
  - Reference: research.md §11 (lines 840-843)

- [ ] **T044** Create login command in `cmd/gridctl/cmd/login.go`
  - Initiate PKCE + loopback flow (call device_flow.go)
  - Save tokens to credential store on success
  - Print success message with token expiry
  - Reference: quickstart.md §2 (lines 114-181), plan.md §725-727

- [ ] **T045** Create logout command in `cmd/gridctl/cmd/logout.go`
  - Delete local credentials file (~/.grid/credentials.json)
  - Optionally call /auth/logout RPC to revoke server-side session
  - Print confirmation message
  - Reference: plan.md §728-729

- [ ] **T046** Create auth status command in `cmd/gridctl/cmd/auth.go`
  - Load credentials from store
  - Call GetEffectivePermissions RPC
  - Display: user identity, roles, token expiry, effective permissions
  - Reference: plan.md §730-732

- [ ] **T047** Create role assign-group command in `cmd/gridctl/cmd/role.go` (function: assignGroupCmd)
  - Cobra command: `gridctl role assign-group <group> <role>`
  - Call AssignGroupRole RPC
  - Print confirmation with assignment details
  - Require admin:group-assign permission
  - Reference: plan.md §733-736, CLARIFICATIONS.md §1 (primary SSO pattern)

- [ ] **T048** Create role remove-group command in `cmd/gridctl/cmd/role.go` (function: removeGroupCmd)
  - Cobra command: `gridctl role remove-group <group> <role>`
  - Call RemoveGroupRole RPC
  - Print confirmation
  - Reference: plan.md §737-738

- [ ] **T049** Create role list-groups command in `cmd/gridctl/cmd/role.go` (function: listGroupsCmd)
  - Cobra command: `gridctl role list-groups [group]`
  - Call ListGroupRoles RPC (optionally filter by group_name)
  - Display tab-delimited table: group_name, role_name, assigned_at
  - Reference: plan.md §739-742

## Phase 3.7: CLI Terraform Wrapper (Token Injection & Process Management)

- [ ] **T050** [P] Create binary discovery in `pkg/sdk/terraform/binary.go`
  - Precedence: --tf-bin flag → TF_BIN env var → auto-detect (terraform, then tofu)
  - Check binary exists and is executable
  - Return clear error if neither terraform nor tofu found
  - Reference: plan.md §743-747, FR-097b, FR-097h

- [ ] **T051** [P] Create process spawner in `pkg/sdk/terraform/wrapper.go`
  - Function: Run(ctx context.Context, args []string, env map[string]string) (exitCode int, err error)
  - Pipe STDIN/STDOUT/STDERR to/from subprocess unchanged
  - Preserve exact exit code from subprocess
  - Pass through all args after `--` verbatim
  - Run in current working directory (no --cwd flag)
  - Reference: plan.md §748-752, FR-097f, FR-097h

- [ ] **T052** [P] Create token injection in `pkg/sdk/terraform/auth.go`
  - Set TF_HTTP_USERNAME="" and TF_HTTP_PASSWORD=<bearer_token> environment variables
  - Load credentials from pkg/sdk credential source (not CLI-specific)
  - Fail fast if no credentials available in non-interactive mode
  - Never persist token to disk, only pass via env vars
  - Reference: plan.md §753-757, FR-097c, FR-097j, FR-097k

- [ ] **T053** [P] Create 401 detection and retry in `pkg/sdk/terraform/output.go`
  - Parse stderr for "401" or "Unauthorized" strings
  - On 401: attempt single token refresh, re-run exact same command once, then fail
  - Maximum 1 retry attempt (no infinite loops)
  - Clear auth failure message if retry fails
  - Reference: plan.md §758-762, FR-097e

- [ ] **T054** [P] Create secret redaction in `pkg/sdk/terraform/output.go`
  - Mask bearer tokens in all console output, logs, crash reports
  - When --verbose: print command line with tokens redacted (show "[REDACTED]")
  - Never print: bearer token values, TF_HTTP_PASSWORD, Authorization headers
  - Reference: plan.md §763-766, FR-097g

- [ ] **T055** [P] Create CI vs interactive detection in `pkg/sdk/terraform/auth.go`
  - Check for TTY, CI env vars (CI=true, GITHUB_ACTIONS, GITLAB_CI, etc.)
  - Non-interactive mode: use service account credentials, fail fast if missing
  - Interactive mode: allow device flow or existing credentials
  - Reference: plan.md §767-770, FR-097j

- [ ] **T056** Create gridctl tf command in `cmd/gridctl/cmd/tf.go`
  - Cobra command: `gridctl tf [flags] -- <terraform args>`
  - Flags: --tf-bin (binary override), --verbose (debug output)
  - Read .grid file via cmd/gridctl/internal/context (DirCtx) for backend endpoint
  - Delegate to pkg/sdk/terraform wrapper with credentials and context
  - Hint: If backend.tf missing, print non-blocking suggestion to run `gridctl state init`
  - Reference: plan.md §771-776, FR-097i, FR-097l (CLI-only hint)

- [ ] **T057** Create context file integration in `cmd/gridctl/internal/context/reader.go` (update existing)
  - Extend existing DirCtx to extract GUID and backend endpoints from .grid file
  - Provide context to pkg/sdk/terraform for validation
  - Note: Context reading is CLI responsibility, not pkg/sdk
  - Reference: plan.md §777-781

## Phase 3.8: Webapp Authentication (React Integration)

- [ ] **T058** [P] Create AuthContext provider in `webapp/src/context/AuthContext.tsx`
  - AuthState: user (UserInfo | null), loading (boolean), error (string | null)
  - Functions: login (redirect to /auth/login), logout, refreshToken
  - useEffect on mount: attempt token refresh from httpOnly cookie
  - Mirror existing GridContext.tsx structure
  - Reference: research.md §13 (lines 885-940), plan.md §785-790

- [ ] **T059** [P] Create auth service in `webapp/src/services/authApi.ts`
  - Functions: initLogin, handleCallback, logout, refreshToken
  - Use direct HTTP fetch (not Connect RPC) per Constitution exception
  - Set credentials: 'include' for httpOnly cookies
  - Reference: research.md §13 (lines 942-974), plan.md §791-795

- [ ] **T060** [P] Create useAuth hook in `webapp/src/context/AuthContext.tsx` (export from AuthContext)
  - Export: useAuth() providing user, loading, error, login, logout
  - Follow existing useGrid() pattern
  - Reference: research.md §13, plan.md §796-799

- [ ] **T061** [P] Create usePermissions hook in `webapp/src/hooks/usePermissions.ts`
  - Call GetEffectivePermissions RPC, cache in state
  - Helper: hasPermission(action: string) → boolean
  - Reference: plan.md §800-802

- [ ] **T062** [P] Create AuthGuard component in `webapp/src/components/AuthGuard.tsx`
  - Conditional rendering: redirect to login if unauthenticated
  - Preserve return_to parameter in login redirect
  - Show loading spinner while auth state loading
  - Reference: research.md §13 (lines 976-997), plan.md §803-807

- [ ] **T063** [P] Create LoginCallback component in `webapp/src/components/LoginCallback.tsx`
  - Parse code/state from URL query params
  - Call authApi.handleCallback
  - Redirect to return_to on success, show error message on failure
  - Reference: plan.md §808-810

- [ ] **T064** [P] Create UserMenu component in `webapp/src/components/UserMenu.tsx`
  - Display user email/name from useAuth
  - Logout button calling logout function
  - Follow existing webapp component patterns
  - Reference: plan.md §811-813

- [ ] **T065** [P] Create ProtectedAction component in `webapp/src/components/ProtectedAction.tsx`
  - Conditional rendering based on hasPermission check
  - Props: requiredPermission (string), children, fallback (optional)
  - Note: Prepared for future use; dashboard is READ ONLY
  - Reference: research.md §13 (lines 999-1023), plan.md §814-817

- [ ] **T066** Update App.tsx in `webapp/src/App.tsx`
  - Wrap existing GridProvider with AuthProvider (auth wraps grid)
  - Import and use AuthProvider from context/AuthContext
  - Reference: plan.md §818-820

- [ ] **T067** Create login page/route in `webapp/src/pages/Login.tsx` (or inline in App.tsx)
  - Button to initiate authApi.initLogin(window.location.origin + '/callback')
  - Simple conditional rendering (no routing library needed unless adding React Router)
  - Reference: plan.md §821-823

- [ ] **T068** Update gridApi.ts with 401 interceptor in `webapp/src/services/gridApi.ts`
  - Add Connect interceptor to catch unauthenticated errors
  - On 401: redirect to /login?return_to=<current_path>
  - Reference: research.md §13 (lines 1025-1048), plan.md §824-828

## Phase 3.9: Integration (Wire Everything Together)

- [ ] **T069** Apply auth middleware to Chi router in `cmd/gridapi/cmd/serve.go` (or router setup file)
  - Order: Authentication middleware → Authorization middleware → Business logic
  - Attach to Chi router before other handlers
  - Reference: research.md §1 (lines 13-46), plan.md §830-832

- [ ] **T070** Apply auth middleware to Connect RPC service in `cmd/gridapi/internal/server/connect_handlers.go`
  - Selective: Some endpoints public (e.g., health check), most require auth
  - Use Connect interceptors or wrap service registration
  - Reference: plan.md §833-835

- [ ] **T071** Update tfstate endpoints with auth in `cmd/gridapi/internal/server/tfstate.go`
  - Apply auth middleware to /tfstate/{guid}, /tfstate/{guid}/lock, /tfstate/{guid}/unlock
  - Middleware extracts token from HTTP Basic Auth password field (task T018)
  - Authorization: tfstate:read, tfstate:write, tfstate:lock, tfstate:unlock
  - Lock-aware bypass: principal holding lock bypasses label scope for tfstate:write/unlock
  - Reference: authorization-design.md, plan.md §836-841

- [ ] **T072** Update Docker Compose with Keycloak in `docker-compose.yml`
  - Add Keycloak service (quay.io/keycloak/keycloak:22.0)
  - Add PostgreSQL init script for Keycloak database
  - Environment: KEYCLOAK_ADMIN, KC_DB settings
  - Ports: 8443:8080 (Keycloak HTTP)
  - Health check for startup synchronization
  - Reference: plan.md §356-408 (complete docker-compose config)

- [ ] **T073** Create PostgreSQL init script in `initdb/01-init-keycloak-db.sql`
  - Create keycloak user and database (idempotent)
  - Runs on first PostgreSQL container start
  - Reference: plan.md §410-426

## Phase 3.10: Testing (TDD - Tests First, Implementation After)

### Repository Tests

- [ ] **T074** Create auth repository unit tests in `cmd/gridapi/internal/repository/auth_repository_test.go`
  - Test session CRUD, revocation checks (sessions.revoked column)
  - Test service account CRUD, bcrypt secret hashing, rotation
  - Test group-role CRUD, unique constraints
  - Test cascade revocation: RevokeAllSessionsForServiceAccount
  - Use real PostgreSQL database (not mocks), run migrations before tests
  - Follow existing *_test.go patterns
  - Reference: plan.md §843-845

### Auth Core Tests

- [ ] **T075** Create JWT validation unit tests in `cmd/gridapi/internal/auth/jwt_test.go`
  - Mock JWKS endpoint using httptest.Server
  - Test cases: valid token, expired token, invalid signature, wrong issuer
  - Reference: plan.md §846-849

- [ ] **T076** Create JWT claim extraction tests in `cmd/gridapi/internal/auth/claims_test.go`
  - Flat array: `["dev-team", "contractors"]`
  - Nested objects: `[{"name": "dev-team"}]` with path="name"
  - Edge cases: empty groups, nil groups, invalid types
  - Reference: research.md §8 (test examples), plan.md §850-854

### Middleware Tests

- [ ] **T077** Create middleware unit tests in `cmd/gridapi/internal/middleware/middleware_test.go`
  - Use httptest.NewRequest to test in isolation
  - Scenarios: no token (401), invalid token (401), valid token (pass through)
  - Lock-aware bypass: test lock holder bypasses label scope for tfstate:write/unlock
  - Reference: plan.md §855-858

### Contract Tests

- [ ] **T078** Create auth RPC contract tests in `tests/contract/auth_contract_test.go`
  - Test all RPCs from tasks T026-T041 (proto contract verification)
  - CreateServiceAccount_Success, CreateServiceAccount_Unauthorized
  - AssignRole_ValidRole, AssignRole_NonexistentRole
  - AssignGroupRole, RemoveGroupRole, ListGroupRoles (group-role RPCs)
  - RevokeServiceAccount cascades to sessions (FR-070b)
  - GetEffectivePermissions with multiple roles
  - CreateRole_ValidDefinition, CreateRole_InvalidActionEnum
  - Follow existing tests/contract/ patterns
  - Reference: plan.md §859-862

### Integration Tests

- [ ] **T079** Create integration tests with Keycloak in `tests/integration/auth_test.go`
  - Use Keycloak in docker-compose, real OIDC flow
  - Test scenarios from quickstart.md: Product Engineer (label scope), Platform Engineer (full access), CI/CD Pipeline (service account)
  - Test user with multiple groups gets union of permissions
  - Test /tfstate endpoints extract token from HTTP Basic Auth password field
  - Reference: quickstart.md §1-8, plan.md §863-867

### CLI Tests

- [ ] **T080** Create CLI E2E tests in `cmd/gridctl/cmd/auth_test.go`
  - Mock HTTP server for /auth/login, /auth/callback endpoints
  - Test login, logout, status commands
  - Test role assign-group, remove-group, list-groups commands
  - Reference: plan.md §868-870

### Terraform Wrapper Tests

- [ ] **T081** Create Terraform wrapper unit tests in `pkg/sdk/terraform/wrapper_test.go`
  - Binary discovery: test precedence (--tf-bin → TF_BIN → auto-detect)
  - STDIO pass-through: verify output streaming, exit codes preserved
  - Token injection: verify TF_HTTP_PASSWORD set correctly, never logged
  - 401 retry: test single retry on 401, then fail
  - Secret redaction: test --verbose masks tokens
  - Reference: plan.md §871-876

- [ ] **T082** Create Terraform wrapper E2E tests in `tests/integration/terraform_wrapper_test.go`
  - Mock HTTP backend using httptest.Server with 401 scenarios
  - Test commands: gridctl tf plan, gridctl tf apply with auth
  - CI mode: test non-interactive service account auth
  - Reference: plan.md §877-880

### Webapp Tests

- [ ] **T083** Create webapp auth flow tests in `webapp/src/__tests__/auth.test.tsx`
  - Use React Testing Library, mock useAuth hook
  - Test AuthGuard rendering (loading, unauthenticated, authenticated states)
  - Test ProtectedAction visibility based on permissions
  - Test login flow, 401 handling in gridApi interceptor
  - Follow existing dashboard_*.test.tsx patterns
  - Reference: research.md §13 (lines 1075-1090), plan.md §881-884

## Dependencies

**Sequential Dependencies** (must complete in order):
- Database (T001-T002) → Config (T003) → Casbin Foundation (T004-T008)
- Config (T003) → Auth Core (T009-T012)
- Casbin Foundation (T005) → Dynamic Grouping (T012)
- Repositories (T013-T016) before Middleware (T017-T019)
- Auth Core (T009-T012) before Middleware (T017-T019)
- Middleware (T017-T019) before Integration (T069-T073)
- HTTP Endpoints (T020-T025) before Integration (T069)
- RPC Handlers (T026-T041) before Integration (T070)
- Tests (T074-T083) should be written BEFORE implementing features (TDD)

**Parallel Execution Groups** (can run concurrently):
- [P] Auth Core: T009, T010, T011 (different files in auth/)
- [P] Repositories: T015 (group_role_repository.go can be parallel with session/service_account repos)
- [P] Middleware: T017, T018, T019 (authn.go, authz.go, context.go)
- [P] HTTP Endpoints: T020, T021, T022, T023, T024, T025 (all handlers in auth_handlers.go)
- [P] RPC Handlers: T026-T041 (all independent RPCs in connect_handlers.go)
- [P] CLI TF Wrapper Core: T050, T051, T052, T053, T054, T055 (pkg/sdk/terraform/ files)
- [P] Webapp Components: T058, T059, T060, T061, T062, T063, T064, T065 (context, services, hooks, components)

## Parallel Execution Examples

**Launch Auth Core Tasks Together** (after T003 complete):
```bash
# In Claude Code, create multiple tasks in one message:
Task 1: "Implement OIDC client wrapper in cmd/gridapi/internal/auth/oidc.go per T009"
Task 2: "Implement JWT validation in cmd/gridapi/internal/auth/jwt.go per T010"
Task 3: "Implement JWT claim extraction in cmd/gridapi/internal/auth/claims.go per T011"
```

**Launch HTTP Endpoint Handlers Together** (after auth core complete):
```bash
Task 1: "Implement HandleLogin in cmd/gridapi/internal/server/auth_handlers.go per T020"
Task 2: "Implement HandleCallback in cmd/gridapi/internal/server/auth_handlers.go per T021"
Task 3: "Implement HandleToken in cmd/gridapi/internal/server/auth_handlers.go per T022"
Task 4: "Implement HandleLogout in cmd/gridapi/internal/server/auth_handlers.go per T023"
Task 5: "Implement HandleDeviceCodeInit in cmd/gridapi/internal/server/auth_handlers.go per T024"
Task 6: "Implement HandleDeviceTokenPoll in cmd/gridapi/internal/server/auth_handlers.go per T025"
```

**Launch RPC Handlers Together** (after repositories complete):
```bash
# All T026-T041 can run in parallel (16 RPC handlers)
Task 1: "Implement CreateServiceAccount RPC per T026"
Task 2: "Implement ListServiceAccounts RPC per T027"
# ... (continue for all RPC handlers)
```

**Launch Terraform Wrapper Components Together**:
```bash
Task 1: "Implement binary discovery in pkg/sdk/terraform/binary.go per T050"
Task 2: "Implement process spawner in pkg/sdk/terraform/wrapper.go per T051"
Task 3: "Implement token injection in pkg/sdk/terraform/auth.go per T052"
Task 4: "Implement 401 retry in pkg/sdk/terraform/output.go per T053"
Task 5: "Implement secret redaction in pkg/sdk/terraform/output.go per T054"
Task 6: "Implement CI detection in pkg/sdk/terraform/auth.go per T055"
```

## Notes

- **TDD Approach**: All tests (T074-T083) should be written BEFORE implementing corresponding features
- **Commit After Each Task**: Create git commit after completing each task
- **Avoid**: Vague tasks, same file conflicts when running parallel tasks
- **File Path Convention**: All paths in tasks are absolute or relative to repo root
- **Testing**: Use real PostgreSQL for repository tests (not mocks), Keycloak in docker-compose for integration tests
- **Constitution Compliance**: No new modules introduced, follows existing patterns, uses existing dependencies
- **Group-Role Priority**: Group-to-role mappings (T032-T034) are the PRIMARY pattern for enterprise SSO; direct user-role assignments (T030-T031) are fallback for edge cases
- **Terraform Basic Auth**: Server extracts bearer token from HTTP Basic Auth password field (username ignored, GitHub API pattern)
- **Lock-Aware Bypass**: Lock holders retain tfstate:write/unlock permissions even if label scope changes
- **Cascade Revocation**: RevokeServiceAccount invalidates all active sessions via RevokeAllSessionsForServiceAccount

## Validation Checklist

After completing all tasks, verify:

- [ ] All migrations run successfully (auth schema + seed data)
- [ ] Casbin enforcer loads policies from database
- [ ] OIDC login flow works (web and CLI)
- [ ] Service account authentication works
- [ ] Role assignment and group-role mapping work
- [ ] Create constraints enforced (product-engineer can only create env=dev)
- [ ] Immutable keys enforced (product-engineer cannot modify env label)
- [ ] Label scope filtering works (product-engineer only sees env=dev states)
- [ ] Admin has full access (platform-engineer wildcard permissions)
- [ ] Service account limited to Data Plane (tfstate:* only)
- [ ] Token expiry enforced (12 hours)
- [ ] Session revocation works (sessions.revoked checked on every request)
- [ ] Service account revocation cascades to sessions
- [ ] Terraform HTTP Backend auth via Basic Auth password field works
- [ ] Lock holders bypass label scope for tfstate:write/unlock
- [ ] Dependency authorization checks both source and target states
- [ ] Clear error messages on authorization denial
- [ ] All contract tests pass
- [ ] All integration tests pass
- [ ] Webapp auth flow works (login, logout, AuthGuard, 401 handling)
- [ ] CLI tf wrapper works (token injection, 401 retry, secret redaction)

---

**Status**: Tasks ready for execution. Total: 83 tasks (Foundation: 8, Auth Core: 8, Middleware: 3, HTTP Endpoints: 6, RPC Handlers: 16, CLI Auth: 8, CLI TF Wrapper: 8, Webapp: 11, Integration: 5, Testing: 10)
