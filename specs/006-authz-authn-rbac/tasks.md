# Tasks: Authentication, Authorization, and RBAC

**Feature**: 006-authz-authn-rbac
**Input**: Design documents from `/Users/vincentdesmet/tcons/grid/specs/006-authz-authn-rbac/`
**Prerequisites**: plan.md (required), research.md, data-model.md, contracts/, quickstart.md

## Overview

This task list implements comprehensive authentication (AuthN), authorization (AuthZ), and role-based access control (RBAC) for Grid's API server. The implementation protects both the Connect RPC Control Plane and the Terraform HTTP Backend Data Plane using:

- **Casbin** for policy-based authorization with go-bexpr label filtering
- **OIDC** for SSO authentication (Keycloak dev, Azure Entra ID prod)
- **zitadel/oidc** for relaying party on client/server side, as well as internal IdP hosting authorization, device, token, discovery, and revocation endpoints (for service accounts only)
- **go-oidc-middleware** for token extraction/validation on Chi routes (JWKS-backed)
- **OAuth2 Client Credentials** for service accounts
- **OIDC Device Code flow (RFC 8628)** for CLI authentication
- **Group-to-Role Mapping** for enterprise SSO integration

**Key Architectural Decisions**:
- **Mode-Based Authentication**: Deployment chooses ONE of two modes (External IdP Only OR Internal IdP Only). Hybrid NOT supported.
- Casbin enforces permissions with union (OR) semantics via custom Chi middleware layered on casbin/v2
- JWT tokens validated via mode-based verifier (auto discovery + JWKS)
- Terraform HTTP Backend uses Basic Auth pattern (bearer token in password field converted to Authorization header before validation)
- Database stores: roles, user_roles, group_roles, service_accounts, sessions, casbin_rules
- Label-scoped access via go-bexpr expressions evaluated at enforcement time
- Three default roles seeded: service-account, platform-engineer, product-engineer

## Short-Path Landing Adjustments (v1)

To ship the initial AuthN/AuthZ feature quickly and keep Terraform stable:

- [x] **T009C** Increase Internal IdP access-token TTL default to 120 minutes
  - Change `defaultAccessTokenTTL` in `cmd/gridapi/internal/auth/oidc.go` to `120 * time.Minute`.
  - Keep refresh tokens at 24h; ID tokens remain at 15m.

- [x] **T099** Documentation: External IdP TTL guidance
  - Update `specs/006-authz-authn-rbac/spec.md` with FR-010a (Interim) and edge-case note.
  - Update `specs/006-authz-authn-rbac/plan.md` with a Short-Path Landing section and plan adjustments.
  - Update `specs/006-authz-authn-rbac/quickstart.md` with a Token TTL Setup and Guidance section including Auth0/Okta/Entra notes.

- [→] **T003** Add TerraformTokenTTL (Defer to run-token branch)
  - Defer endpoint + signer wiring to `006-authz-authn-rbac-run-tokens` feature branch.
  - Future tasks: exchange endpoint, dual-issuer verifier for dataplane, scope enforcement in middleware, CLI wrapper to request run-token.

## Format: `[ID] [P?] Description`
- **[P]**: Can run in parallel (different files, no dependencies)
- Include exact file paths in descriptions

## Phase 3.1: Setup & Foundation

### Database Schema

> Follow existing migrations and model patterns `cmd/gridapi/internal/migrations/*`

- [x] **T001** Create database migration `YYYYMMDDHHMMSS_auth_tables.go` in `cmd/gridapi/internal/migrations/`
  - Define Bunx Models and Relations in `cmd/gridapi/internal/db/models`
  - Use Model pointers to create tables: users, service_accounts, roles, user_roles, group_roles, sessions, casbin_rules
  - Fork https://github.com/msales/casbin-bun-adapter/blob/master/adapter.go#L404-L416 to create casbin_rules table and initial seeding - see cmd/gridapi/internal/auth/bunadapter/README.md
  - Reference: data-model.md §17-445 for complete schema (plan differred from implementation)
  - casbin_rules table: Standard Casbin schema (id, ptype, v0-v5) per research.md §8
  - Unique index on casbin_rules: (ptype, v0, v1, v2, v3, v4, v5) (actual implementation: composed pk to avoid additional uniqueness constraint, dropped id column)
  - Check constraints: user_roles and sessions must reference exactly one identity type
  - Partial unique indexes: user_roles (user_id, role_id) and (service_account_id, role_id)
  - GIN index on service_accounts.scope_labels: OUT OF SCOPE (evaluation in Go, not SQL)

- [x] **T002** Create seed migration `YYYYMMDDHHMMSS_seed_auth_data.go` in `cmd/gridapi/internal/migrations/`
  - Seed 3 default roles into roles table: service-account, platform-engineer, product-engineer
  - Seed default Casbin policies into casbin_rules table with go-bexpr expressions
  - Reference: data-model.md §Seed Data (lines 449-526), research.md §8 (lines 493-543)
  - Define System User Id ("00000000-0000-0000-0000-000000000000") as constant in `cmd/gridapi/internal/auth/system.go` for seeding and service account management
  - Seed system user to manage service accounts created via `gridapi sa` command (divergence from plan due to implementation clarifications)
  - service-account policies: tfstate:* on state with empty scopeExpr
  - platform-engineer: wildcard (*:*) with empty scopeExpr
  - product-engineer: state:*/tfstate:*/dependency:* with scopeExpr `env == "dev"`

- [x] **T001A** Create revoked_jti table migration in `cmd/gridapi/internal/migrations/`
  - Table: revoked_jti (jti TEXT PRIMARY KEY, subject TEXT, exp TIMESTAMP, revoked_at TIMESTAMP, revoked_by TEXT)
  - Index on exp for cleanup queries: `CREATE INDEX idx_revoked_jti_exp ON revoked_jti(exp)`
  - Used for JWT revocation denylist (check after signature validation)
  - **DEFERRED Cleanup job**: periodic deletion of rows where `exp < NOW() - interval '1 hour'`
  - Reference: implementation-adjustments.md §57-64 (revocation model), §486-490 (revoked_jti table structure)

- [x] **T001B** Create users table migration in `cmd/gridapi/internal/migrations/` (Internal IdP Mode)
  - Table: users (id UUID PRIMARY KEY, email TEXT UNIQUE NOT NULL, password_hash TEXT, created_at TIMESTAMP, disabled_at TIMESTAMP)
  - Used for local username/password authentication in Mode 2 (Internal IdP)
  - Password stored as bcrypt hash with cost 10
  - Reference: implementation-adjustments.md §361-363 (internal mode data), quickstart.md Scenario 1 Mode 2 Alternative

### Configuration

- [x] **T003** Update config struct in `cmd/gridapi/internal/config/config.go`
  - Add OIDCConfig struct with fields: Issuer, ClientID, ClientSecret, RedirectURI
  - Add claim extraction config: GroupsClaimField (default: "groups"), GroupsClaimPath (optional, for nested)
  - Add UserIDClaimField (default: "sub"), EmailClaimField (default: "email")
  - Add CasbinModelPath string field (path to model.conf)
  - DEFERRED (vNext): Do NOT add `TerraformTokenTTL` in v1. The run-token exchange and its TTL knob move to a dedicated branch (`006-authz-authn-rbac-run-tokens`).
  - Reference: research.md §2 (lines 622-635), CLARIFICATIONS.md §1 (JWT claim config)

- [x] **T003A** Document and automate OIDC signing key setup (FR-110)
  - Add `make oidc-dev-keys` (or similar) to generate ed25519 and RSA key pairs for local development (`cmd/gridapi/internal/auth/keys/`)
  - Update `docs/local-dev.md` (or new section) with instructions for loading dev keys and configuring environment variables
  - Document production guidance for loading keys from secure key vault / secrets manager (reference deployment guide)
  - Reference: FR-110 (spec.md line 662), research.md §7 (Key Management TODO), plan.md §33-41 (library overview)

- [x] **T003B** Add helper scripts to manage Keycloak dev environment (FR-111, FR-112)
  - Create Make targets: `make keycloak-up` (docker compose up -d), `make keycloak-down`, `make keycloak-logs`, ensuring reliance on updated `docker-compose.yml` and `initdb/01-init-keycloak-db.sql`
  - Provide wrapper script `scripts/dev/keycloak-reset.sh` to stop stack, prune volumes (with confirmation), and restart for clean state
  - Update `docs/local-dev.md` (or quickstart.md) with instructions for using the make targets, admin credentials, and expected URLs (http://localhost:8443)
  - Reference: FR-111 (spec.md line 664), FR-112 (spec.md line 666), plan.md §356-426 (compose details), research.md §2 (Keycloak setup)

### Casbin Foundation

- [x] **T004** Create Casbin model file `cmd/gridapi/casbin/model.conf`
  - Define RBAC model with custom bexprMatch function in matcher
  - Model: `r = sub, objType, act, labels` / `p = role, objType, act, scopeExpr, eft`
  - Matcher: `g(r.sub, p.role) && r.objType == p.objType && r.act == p.act && bexprMatch(p.scopeExpr, r.labels)`
  - Reference: research.md §1 (lines 66-86), plan.md §207

- [x] **T005** Create Casbin enforcer initialization in `cmd/gridapi/internal/auth/casbin.go`
  - Function: `InitEnforcer(db *bun.DB, modelPath string) (*casbin.Enforcer, error)`
  - Use forked msales/casbin-bun-adapter with reference to our existing *bun.DB instance
  - Register custom bexprMatch function with sync.Map caching for compiled evaluators
  - Load policies from database via enforcer.LoadPolicy()
  - Reference: research.md §1 (lines 429-490), §7 (adapter usage)

- [x] **T006** Create go-bexpr evaluator helper in `cmd/gridapi/internal/auth/bexpr.go`
  - Function: `evaluateBexpr(scopeExpr string, labels map[string]any) bool`
  - Cache compiled evaluators using sync.Map (expression string → *bexpr.Evaluator)
  - Empty scopeExpr returns true (no constraint)
  - Handle errors gracefully (invalid expressions return false)
  - Reference: research.md §1 (lines 443-482)

- [x] **T007** Create Casbin identifier helpers in `cmd/gridapi/internal/auth/identifiers.go`
  - Define prefix constants: PrefixUser ("user:"), PrefixGroup ("group:"), PrefixServiceAccount ("sa:"), PrefixRole ("role:")
  - Helper functions: UserID(id string), GroupID(name string), ServiceAccountID(id string), RoleID(name string)
  - Parse functions: ExtractUserID(principal string), ExtractRoleID(principal string)
  - Reference: PREFIX-CONVENTIONS.md §2-3, §9 (helper function patterns)

- [x] **T008** Create action constants file in `cmd/gridapi/internal/auth/actions.go`
  - Define all action constants from authorization-design.md
  - Control Plane: StateCreate, StateRead, StateList, StateUpdateLabels, StateDelete
  - Data Plane: TfstateRead, TfstateWrite, TfstateLock, TfstateUnlock
  - Admin: AdminRoleManage, AdminUserAssign, AdminGroupAssign, AdminServiceAccountManage, AdminSessionRevoke
  - Wildcards: StateWildcard, TfstateWildcard, AdminWildcard, AllWildcard
  - Reference: plan.md §315-331, `specs/006-authz-authn-rbac/authorization-design.md` (Actions Taxonomy section) for canonical action definitions and role mappings.

## Phase 3.2: Auth Core (Token Validation & Session Management)

- [⛔] **T009** [P] Configure OIDC provider endpoints in `cmd/gridapi/internal/auth/oidc.go` (Grid as Provider)
  - **Status**: ⛔️ DIVERGED - Issues opaque tokens instead of JWTs (BLOCKING)
  - **Divergence**: The `zitadel/oidc` library was found to issue stateful, opaque tokens by default for the internal IdP, not JWTs as planned. This has led to a core architectural issue where tokens are stored in-memory and are invalidated on server restart.
  - **Blocker**: Must complete T009A before proceeding with any auth features
  - Instantiate zitadel/oidc `op.Provider` with config from `config.OIDC` (issuer, endpoints)
  - Implement storage adapters backed by repositories (clients, users, sessions, device codes)
  - Mount authorization, token, device, discovery, revocation handlers onto Chi router helper
  - Hook handler callbacks to persist Grid session rows (FR-005/FR-007) and emit audit logs (FR-098/FR-099)
  - **Purpose**: Issues first-party tokens for service accounts + CLI device flow
  - **Sessions**: Persisted at issuance time in `CreateAccessToken()` callback
  - Reference: research.md §7 (provider wiring), plan.md §201-214, implementation-adjustments.md §119-128

- [x] **T009A** ✅ COMPLETE: Configure JWT Access Tokens (`cmd/gridapi/internal/auth/oidc.go`)
  - **Status**: ✅ Verified working - JWT tokens generated successfully
  - **What was done**:
    1. ✅ Line 726: `AccessTokenType()` already returns `op.AccessTokenTypeJWT` (was already correct)
    2. ✅ Line 507: `GetAudience()` already returns `[]string{"gridapi"}` (was already correct)
    3. ✅ **NEW**: Added signing key persistence (`config.OIDCConfig.SigningKeyPath`)
    4. ✅ **NEW**: Implemented `loadOrGenerateSigningKey()` in oidc.go to load/save persistent RSA keys
  - **Discovery**: Our `createJWT()` method is **NOT dead code**
    - It's part of the required `op.Storage` interface
    - zitadel/oidc library may call it for certain grant types
    - Kept as-is (working correctly)
  - **Verification Completed**:
    - ✅ Token has 3 parts (header.payload.signature)
    - ✅ Claims include: iss, sub, aud (gridapi), exp, iat, jti
    - ✅ Signing key persists across server restarts at `tmp/keys/signing-key.pem`
  - Reference: implementation-adjustments.md "JWT Creation Architecture Clarification"

- [x] **T009B** [P] Configure external IdP integration in `cmd/gridapi/internal/auth/relying_party.go` (Grid as Relying Party)
  - Use github.com/zitadel/oidc/v3/pkg/client/rp to handles OIDC authentication against an external IdP (from `config.OIDC.ExternalIdP`)
  - Configure authorization code flow to use `rp.WithPKCE`
  - Generate and validate state/nonce for CSRF/replay protection (use crypto/rand, store in session cookie using `zitadelhttp.NewCookieHandler`)
  - Implement token exchange logic (code → tokens via IdP's /token endpoint)
  - Verify ID token signature and claims using go-oidc library (coreos/go-oidc)
  - **Purpose**: Integrates with external SSO (Keycloak, Entra ID, Okta) for web users
  - **Sessions**: Created on first request after token validation (see T017 update)
  - Reference: spec.md FR-001/FR-002
  - **Note**: This is the missing piece for SSO user authentication

- [⛔] **T010** [P] Create mode-based token verifier in `cmd/gridapi/internal/auth/jwt.go`
  - **Status**: ⛔️ DIVERGED - Bypasses JWT validation for internal IdP (BLOCKING)
  - **Divergence**: Due to the internal IdP issuing opaque tokens (see T009), the verifier was modified. JWT verification is now disabled in internal IdP mode, and the middleware simply passes through the opaque token for later validation. This deviates from the original plan of universal JWT validation.
  - **Blocker**: Must complete T010A after T009A is fixed
  - **Function**: `NewVerifier(cfg config.OIDCConfig, opts ...VerifierOption) (func(http.Handler) http.Handler, error)`
  - **Purpose**: Validate tokens based on deployment mode (single issuer per deployment)
  - **Implementation**:
    1. Mode detection:
       - If `cfg.ExternalIdP != nil` → Mode 1 (External IdP Only)
       - If `cfg.Issuer != ""` → Mode 2 (Internal IdP Only)
       - Else → Auth disabled (development mode)
    2. Create single go-oidc verifier based on mode:
       - **Mode 1**: Verifier for `cfg.ExternalIdP.Issuer` (IdP's URL)
       - **Mode 2**: Verifier for `cfg.Issuer` (Grid's URL)
    3. Build Chi middleware that:
       - Extracts JWT from Authorization header
       - Validates token with configured verifier
       - Stores validated claims in request context
  - Expose helpers: `ClaimsFromContext()`, `TokenStringFromContext()`, `TokenHashFromContext()`
  - Provide skipper for public routes (`/health`, `/auth/*`, OPTIONS)
  - Reference: Mode-based architecture (plan.md §35-90), FR-001/FR-002, implementation-adjustments.md §166-178
  - **Note**: Each deployment validates tokens from ONE trusted issuer only

- [x] **T010A** CRITICAL: Revert Verifier to Universal JWT Validation
  - **PRIORITY: BLOCKING** - Depends on T009A completion
  - **Description**: Simplify the token verifier in `cmd/gridapi/internal/auth/jwt.go` to remove the special handling for opaque tokens. The verifier should always expect and validate a JWT, regardless of the issuer.
  - **Actions**:
    1. In `NewVerifier`, remove the logic branch that returns a no-op middleware for the internal provider. The function should always configure and return a real JWT-validating middleware.
    2. The logic should only differentiate between providers to set the correct `issuer` and `clientID` for the verifier.
    3. Remove the error-handling logic that bypasses a `ParseToken` failure. A token that cannot be parsed as a JWT should now result in a hard authentication failure.
    4. After JWT signature and claims validation, extract `jti` claim and pass to context for revocation check in T017A
  - Reference: implementation-adjustments.md §171-178 (universal validation), §205-207 (JWT validation flow)   

- [x] **T011** [P] Create JWT claim extraction in `cmd/gridapi/internal/auth/claims.go`
  - Function: `ExtractGroups(claims map[string]interface{}, claimField, claimPath string) ([]string, error)`
  - Support flat arrays: `["dev-team", "contractors"]`
  - Support nested objects with mapstructure: `[{"name": "dev-team"}]` with path="name"
  - Handle missing/invalid claims gracefully
  - Reference: research.md §9 (lines 619-694), CLARIFICATIONS.md §1

- [x] **T012** [P] Create dynamic Casbin grouping at auth time in `cmd/gridapi/internal/auth/grouping.go`
  - Function: `ApplyDynamicGroupings(enforcer *casbin.Enforcer, userID string, groups []string, groupRoles map[string][]string)`
  - For each group in JWT: create transient grouping `user:alice → group:dev-team`
  - For each group-role mapping: create grouping `group:dev-team → role:product-engineer`
  - Groupings are transient (not persisted in casbin_rules table)
  - Reference: data-model.md §5 (lines 290-317), CLARIFICATIONS.md §1 (lines 59-67)

### Repository Layer

- [x] **T013** Create session repository in `cmd/gridapi/internal/repository/session_repository.go`
  - Interface methods: CreateSession, GetSessionByTokenHash, UpdateLastUsed, RevokeSession, RevokeAllSessionsForUser, RevokeAllSessionsForServiceAccount
  - Schema: data-model.md §3 (lines 174-199)
  - Token hash stored as SHA256 hex (64 chars)
  - Check sessions.revoked column on validation (FR-102a)
  - Cascade revocation: RevokeAllSessionsForServiceAccount sets revoked=true for all sessions (FR-070b)
  - Reference: data-model.md §3, plan.md §632-637

- [x] **T014** Create service account repository in `cmd/gridapi/internal/repository/service_account_repository.go`
  - Interface methods: CreateServiceAccount, GetByClientID, UpdateLastUsed, SetDisabled, RotateSecret
  - Bcrypt hash client secrets with cost 10
  - Schema: data-model.md §2 (lines 141-172)
  - Rotation behavior (FR-070a): Updating secret does NOT invalidate existing tokens (JWT self-contained)
  - Update secret_rotated_at timestamp on rotation
  - Reference: research.md §4 (lines 221-285), plan.md §638-644

- [x] **T015** [P] Create group-role repository in `cmd/gridapi/internal/repository/group_role_repository.go`
  - Interface methods: CreateGroupRole, DeleteGroupRole, ListGroupRoles, GetRolesByGroup
  - Schema: data-model.md §5 (lines 235-265)
  - Unique constraint: (group_name, role_id)
  - Used for static group→role mappings (IdP manages group membership)
  - Reference: CLARIFICATIONS.md §1 (lines 48-68)

- [x] **T016** [P] Create auth repository interface in `cmd/gridapi/internal/repository/auth_repository.go`
  - Combine all auth-related repository interfaces
  - SessionRepository, ServiceAccountRepository, GroupRoleRepository, RoleRepository, UserRoleRepository
  - Follow existing repository/interface.go pattern
  - Reference: plan.md §645

## Phase 3.3: Middleware (Request Interception)

- [⛔] **T017** [P] Update authentication middleware in `cmd/gridapi/internal/middleware/authn.go`
  - **Status**: ⛔️ DIVERGED - Handles opaque tokens with complex dual-path logic (BLOCKING)
  - **Divergence**: The middleware was updated to handle opaque tokens from the internal IdP. Instead of relying solely on JWT validation, it now handles bearer tokens directly from headers for the internal IdP mode.
  - **Blocker**: Must complete T017A after T009A and T010A are fixed
  - **Original plan** (for reference):
    1. Use mode-based single-issuer verifier from T010
    2. After token validation, check if session exists:
       - **Mode 1 (External IdP)**: Session may not exist (first request after SSO callback)
       - **Mode 2 (Internal IdP)**: Session already exists (created at issuance in T009)
    3. Mode-specific session handling:
       - **Mode 1**: If no session exists, create new session:
         - Store token hash for revocation support (FR-007)
         - Extract `sub` and `iss` from claims to identify user uniquely
         - Look up or create user record in users table based on claims
       - **Mode 2**: Session must exist (fail if missing)
    4. Continue existing flow: session revocation check, dynamic Casbin grouping, context injection
  - Normalize Terraform Basic Auth before verification (FR-057–FR-059) - keep existing
  - Extract groups and apply dynamic Casbin groupings - keep existing
  - Reference: Mode-based architecture (plan.md §35-90, research.md §212-225), implementation-adjustments.md §242-256

- [x] **T017A** CRITICAL: Simplify AuthN Middleware and Add JWT Revocation Check
  - **PRIORITY: BLOCKING** - Depends on T009A and T010A completion
  - **Description**: Refactor the authentication middleware to remove the complex dual-path logic for opaque/JWT tokens and implement a mandatory revocation check for all JWTs.
  - **Actions**:
    1. In `cmd/gridapi/internal/middleware/authn.go`, simplify the logic. Assume that a valid set of claims will always be in the context for an authenticated request (per T010A).
    2. Remove the fallback logic that handles raw token strings for session lookup.
    3. **Implement Revocation Check**: After successfully getting claims from the context, extract the `jti` claim.
    4. Query `revoked_jti` table (from T001A) to check if token is revoked: `SELECT 1 FROM revoked_jti WHERE jti = $1`
    5. If revoked OR associated user/service account is disabled, deny the request with a 401 Unauthorized error.
    6. Continue existing flow: extract groups, apply dynamic Casbin grouping, context injection
  - Reference: implementation-adjustments.md §247-256 (simplified authn flow), §205-207 (JWT validation → JTI denylist → RBAC)       

- [x] **T018** [P] Create authorization middleware in `cmd/gridapi/internal/middleware/authz.go`
  - Build custom Chi middleware around casbin/v2 enforcer (avoid casbin's example chi-authz library due to v1 dependency)
  - Custom subject extraction from context (set by authn middleware)
  - Ensure Terraform routes reuse preprocessed authorization (no duplicate Basic Auth handling)
  - Augment tfstate route handlers to capture Authenticated Principal in Terraform Lock Info (FR-061a)
  - Lock-aware bypass (FR-061a): If principal holds lock, bypass label scope for tfstate:write/unlock
  - Use enforcer.Enforce(subject, objType, action, labels) for authorization
  - Reference: research.md §1 (lines 13-46, custom middleware notes), plan.md §657-665, research.md §6 (Basic Auth shim)
  - **Status**: ✅ Implemented in `cmd/gridapi/internal/middleware/authz.go`

- [x] **T018B** [P] Create Connect RPC authorization middleware in `cmd/gridapi/internal/middleware/authz_interceptor.go`
  - Build AuthZ Interceptor combining `cmd/gridapi/internal/auth/actions.go` with casbin/v2 enforcer
  - Leverage T018 subject extraction from context (set by authn middleware)
  - Use enforcer.Enforce(subject, objType, action, labels) for authorization
  - Reference: research.md §1 (lines 13-46, custom middleware notes), plan.md §657-665, research.md §6 (Basic Auth shim)
  - **Status**: ✅ Implemented in `cmd/gridapi/internal/middleware/authz_interceptor.go`

- [x] **T019** [P] Create context helpers in `cmd/gridapi/internal/auth/context.go`
  - Functions: SetUserContext, GetUserFromContext, SetGroupsContext, GetGroupsFromContext
  - Store user ID, groups, and roles in request context
  - Type-safe context keys (avoid string keys)
  - Reference: Standard Go context.WithValue patterns
  - **Status**: ✅ Implemented in `cmd/gridapi/internal/auth/context.go`

## Phase 3.4: HTTP Endpoints (OIDC Flow)

**Dual-Mode Architecture**: Grid either operates as:
1. **OIDC Provider**  endpoints auto-mounted by zitadel/oidc
2. **OIDC Relying Party** custom handlers redirect to external IdP

### Grid as Provider (Auto-Mounted Endpoints)

**zitadel/oidc provider automatically mounts standard OIDC endpoints via `op.CreateRouter()` at `/`:**
- `/device_authorization` - CLI initiates device code flow (RFC 8628)
- `/oauth/token` - Token endpoint (handles device_code, client_credentials, refresh_token grants)
- `/keys` - Grid's JWKS for validating Grid-issued tokens
- `/.well-known/openid-configuration` - Discovery for Grid-as-provider

Implementation divergence:
- oidc-middleware required to use LazyLoad Kws Keyset to avoid race condition during server startup (endpoint not yet serving keys).

### Custom Handlers (Grid as Provider + Relying Party)

- [N/A] **T020A** [P] Create device verification UI in `cmd/gridapi/internal/server/auth_handlers.go` (function: HandleDeviceVerify)
  - **Status**: NOT NEEDED - Internal IdP mode only supports service account authentication
  - **Note**: This was based on an incorrect assumption. The internal IdP mode only supports service account authentication (client credentials), not interactive user flows like device verification. Therefore, no user-facing verification UI is needed for this mode.
  - Reference: implementation-adjustments.md §306-308 (incorrect assumption note), quickstart.md Scenario 2 (device flow via external IdP only)

- [x] **T020B** [P] Create SSO login handler in `cmd/gridapi/internal/server/auth_handlers.go` (function: HandleSSOLogin)
  - **Grid as Relying Party**: Redirect to external IdP (Keycloak, Entra ID, Okta)
  - GET /auth/sso/login - Initiate OAuth2 authorization code flow
  - Generate CSRF state token (crypto/rand), store in secure HTTPOnly cookie
  - Generate PKCE code_challenge (S256) for additional security
  - Build authorization URL using oauth2.Config from T009B
  - Redirect user to external IdP's authorization page
  - Reference: golang.org/x/oauth2 authorization code flow, spec.md FR-001/FR-002
  - **Note**: External IdP hosts login page, not Grid

- [x] **T020C** [P] Create SSO callback handler in `cmd/gridapi/internal/server/auth_handlers.go` (function: HandleSSOCallback)
  - **Grid as Relying Party**: Receive tokens from external IdP
  - GET /auth/sso/callback?code=<code>&state=<state> - OIDC callback from external IdP
  - Validate state parameter against cookie (CSRF protection)
  - Exchange authorization code for tokens via IdP's token endpoint (oauth2.Config.Exchange)
  - Verify ID token signature and claims using go-oidc verifier
  - Extract user claims (sub, email, name, groups)
  - Session will be created on first API request (see T017 update)
  - Redirect user to Grid web UI (post-login page)
  - Reference: golang.org/x/oauth2 token exchange, go-oidc ID token verification
  - **Note**: Session persistence deferred to T017 middleware (first request after callback)

- [ ] **T021** [P] Verify Grid provider endpoints work (integration test)
  - **NOT NEEDED** Verify `/device_authorization` initiates device flow for CLI (auto-mounted by zitadel/oidc) - N/A SA only for internal IdP
  - **NOT NEEDED** Verify `/oauth/token` endpoint handles device_code grant (CLI polling) - N/A SA only for internal IdP
  - Verify `/oauth/token` endpoint handles client_credentials grant (service accounts)
  - Verify `/.well-known/openid-configuration` returns Grid's discovery document
  - Verify `/keys` returns Grid's JWKS
  - Reference: zitadel/oidc provider auto-routing, RFC 8628, OAuth2 client credentials
  - Implementation detail: The AuthN middleware's skipper must allow these endpoints see `publicPrefixes := []string{` in cmd/gridapi/internal/auth/jwt.go
  - **Status**: Endpoints auto-mounted via `r.Mount("/", opts.OIDCRouter)` in server/connect.go

- [ ] **T022** [P] Verify SSO flow endpoints work (integration test)
  - Verify `/auth/sso/login` redirects to external IdP (T020B)
  - Verify `/auth/sso/callback` handles IdP redirect with auth code (T020C)
  - Verify state validation prevents CSRF attacks
  - Verify external IdP tokens are validated by dynamic verifier (T010)
  - Verify sessions created on first request for external IdP tokens (T017)
  - Reference: T009B, T020B, T020C, spec.md FR-001/FR-002

- [ ] **T020D** [P] Create local login handler in `cmd/gridapi/internal/server/auth_handlers.go` (function: HandleLocalLogin)
  - **Grid as Provider (Internal Mode)**: Local username/password authentication
  - POST /auth/login - Verify credentials against users table (from T001B)
  - Accept JSON: `{"email": "user@example.com", "password": "plaintext"}`
  - Verify password using bcrypt.CompareHashAndPassword against users.password_hash
  - Check users.disabled_at is NULL (account not disabled)
  - Create server-side session via session repository, set HttpOnly cookie (`grid_session`)
  - Return success response with user info (id, email, roles)
  - Reference: implementation-adjustments.md §351-356 (internal mode endpoints), quickstart.md Scenario 1 Mode 2 Alternative

- [x] **T023** [P] Create unified logout handler in `cmd/gridapi/internal/server/auth_handlers.go` (function: HandleLogout)
  - **Dual-mode logout**: Handle sessions from both Grid provider and external IdP
  - POST /auth/logout - Revoke session and optionally notify external IdP
  - Implementation:
    1. Extract session from context (set by authn middleware)
    2. Determine session origin (Grid provider vs external IdP):
       - Check session.UserID (external IdP) vs session.ServiceAccountID (Grid provider)
       - Or inspect original token's `iss` claim if still available
    3. Revoke session in Grid's database (session repository.Revoke) OR insert jti into revoked_jti table
    4. Clear session cookies (if using cookie-based sessions)
    5. **Optional** (future): If external IdP session, initiate IdP logout:
       - OIDC Front-Channel Logout: Redirect to IdP's end_session_endpoint
       - OIDC Back-Channel Logout: POST to IdP's revocation endpoint
  - Return success response or redirect to post-logout page
  - Reference: Dual-mode architecture, OIDC session management, spec.md FR-087

## Phase 3.5: Proto Updates & Code Generation

- [x] **T026** Update proto definitions in `proto/state/v1/state.proto`
  - Add all auth-related RPCs from contracts/auth-rpc-additions.proto
  - Service Account Management: CreateServiceAccount, ListServiceAccounts, RevokeServiceAccount, RotateServiceAccount
  - Role Management: CreateRole, ListRoles, UpdateRole, DeleteRole, ExportRoles, ImportRoles
  - Role Assignment: AssignRole, RemoveRole, ListUserRoles
  - Group-to-Role: AssignGroupRole, RemoveGroupRole, ListGroupRoles
  - Permission Introspection: GetEffectivePermissions
  - Session Management: ListSessions, RevokeSession
  - Reference: contracts/auth-rpc-additions.proto (complete RPC definitions), plan.md lines 563-564 (export/import RPCs)

- [x] **T027** Run buf generate to generate Go and TypeScript code
  - Command: `buf generate` from repo root
  - Generates Go code in api/state/v1/
  - Generates TypeScript code in js/sdk/gen/
  - Verify generated code compiles
  - Reference: CLAUDE.md "Code Generation" section

## Phase 3.6: Connect RPC Handlers (Admin Operations)

**Note on file locations**: Due to the number of handlers, the Connect RPC implementations were split across multiple files for better organization. Core state handlers remain in `connect_handlers.go`, new authentication handlers are in `connect_handlers_auth.go`, and dependency-related handlers are in `connect_handlers_deps.go`. This is a structural refactoring from the original plan.

- [x] **T028** [P] Implement CreateServiceAccount RPC (business logic only)
  - Generate UUIDv7 for client_id 
  - Generate random 32-byte client_secret (crypto/rand)
  - Hash secret with bcrypt (cost 10)
  - Use service account repository (not direct table access)
  - Return client_id and plaintext secret (only shown once)
  - Note: Authorization guard to be added in a subsequent task.
  - Reference: contracts/auth-rpc-additions.proto (lines 12-23), existing State creation pattern for UUIDv7

- [x] **T029** [P] Implement ListServiceAccounts RPC (business logic only)
  - Use service account repository to list accounts
  - Return list without client_secret_hash
  - Note: Authorization guard to be added in a subsequent task.
  - Reference: contracts/auth-rpc-additions.proto (lines 25-41)

- [x] **T030** [P] Implement RevokeServiceAccount RPC (business logic only)
  - Use service account repository.SetDisabled(true)
  - Use session repository.RevokeAllSessionsForServiceAccount (FR-070b)
  - Use enforcer.DeleteRoleForUser to remove Casbin groupings
  - Note: Authorization guard to be added in a subsequent task.
  - Reference: contracts/auth-rpc-additions.proto (lines 43-49)

- [x] **T031** [P] Implement RotateServiceAccount RPC (business logic only)
  - Generate new client_secret (UUIDv7 or crypto/rand), hash with bcrypt
  - Use service account repository.RotateSecret()
  - DO NOT invalidate existing sessions (FR-070a: tokens are self-contained)
  - Return new plaintext secret (only shown once)
  - Note: Authorization guard to be added in a subsequent task.
  - Reference: contracts/auth-rpc-additions.proto (lines 51-59)

- [x] **T032** [P] Implement AssignRole RPC (business logic only)
  - Use user role repository.CreateUserRole() or service account assignment method
  - Use enforcer.AddRoleForUser("user:X" or "sa:X", "role:Y") for Casbin grouping
  - Call enforcer.SavePolicy() to persist
  - Note: Authorization guard to be added in a subsequent task.
  - Reference: contracts/auth-rpc-additions.proto (lines 133-143), Casbin RBAC API

- [x] **T033** [P] Implement RemoveRole RPC (business logic only)
  - Use user role repository.DeleteUserRole()
  - Use enforcer.DeleteRoleForUser() to remove Casbin grouping
  - Call enforcer.SavePolicy()
  - Note: Authorization guard to be added in a subsequent task - Require `auth.AdminUserAssign` permission
  - Reference: contracts/auth-rpc-additions.proto (lines 145-153), Casbin RBAC API

- [x] **T034** [P] Implement AssignGroupRole RPC (business logic only)
  - Use group-role repository.CreateGroupRole()
  - Use enforcer.AddRoleForUser(auth.GroupID(X), auth.RoleID(Y)) for static Casbin grouping
  - Call enforcer.SavePolicy()
  - Note: Authorization guard to be added in a subsequent task - Require `auth.AdminGroupAssign` permission
  - Reference: contracts/auth-rpc-additions.proto (lines 172-180), CLARIFICATIONS.md §1

- [x] **T035** [P] Implement RemoveGroupRole RPC (business logic only)
  - Use group-role repository.DeleteGroupRole()
  - Use enforcer.DeleteRoleForUser(auth.GroupID(X), auth.RoleID(Y)) to remove Casbin grouping
  - Call enforcer.SavePolicy()
  - Note: Authorization guard to be added in a subsequent task - Require `auth.AdminGroupAssign` permission
  - Reference: contracts/auth-rpc-additions.proto (lines 182-189)

- [x] **T036** [P] Implement ListGroupRoles RPC (business logic only)
  - Use group-role repository.ListGroupRoles(optional filter by group_name)
  - Return list of group-role assignments with metadata
  - Note: Authorization guard to be added in a subsequent task - Require `auth.AdminGroupAssign` permission
  - Reference: contracts/auth-rpc-additions.proto (lines 191-204)

- [x] **T037** [P] Implement GetEffectivePermissions RPC (business logic only)
  - Use enforcer.GetRolesForUser(principal) to get all roles (includes transitive via groups)
  - Use enforcer.GetFilteredPolicy(0, role) to get policies for each role
  - Aggregate permissions from all roles (union/OR semantics)
  - Return aggregated actions, label_scope_exprs, create_constraints, immutable_keys
  - Reference: contracts/auth-rpc-additions.proto (lines 206-223), Casbin RBAC API

- [x] **T038** [P] Implement CreateRole RPC (business logic only)
  - Use role repository.CreateRole()
  - Use enforcer.AddPolicy() for each action to create Casbin policies
  - Validate label_scope_expr as valid go-bexpr syntax
  - Call enforcer.SavePolicy()
  - Note: Authorization guard to be added in a subsequent task - Require `auth.AdminRoleManage` permission
  - Reference: contracts/auth-rpc-additions.proto (lines 63-100)

- [x] **T039** [P] Implement ListRoles RPC (business logic only)
  - Use role repository.ListRoles()
  - Return role metadata
  - No specific permission required for listing (information is public)
  - Reference: contracts/auth-rpc-additions.proto (lines 102-108)

- [x] **T040** [P] Implement UpdateRole RPC (business logic only)
  - Use role repository.UpdateRole() with optimistic locking (version check)
  - Use enforcer.RemoveFilteredPolicy() to delete old policies, enforcer.AddPolicy() for new
  - Validate label_scope_expr syntax
  - Call enforcer.SavePolicy()
  - Note: Authorization guard to be added in a subsequent task - Require `auth.AdminRoleManage` permission
  - Reference: contracts/auth-rpc-additions.proto (lines 110-122)

- [x] **T041** [P] Implement DeleteRole RPC (business logic only)
  - Use enforcer.GetUsersForRole(role) to check if assigned (block if has users)
  - Use role repository.DeleteRole() (cascade to user_roles via FK)
  - Use enforcer.RemoveFilteredPolicy() to delete all Casbin policies for role
  - Call enforcer.SavePolicy()
  - Note: Authorization guard to be added in a subsequent task - Require `auth.AdminRoleManage` permission
  - Reference: contracts/auth-rpc-additions.proto (lines 124-130), Casbin RBAC API

- [x] **T042** [P] Implement ListSessions RPC (business logic only)
  - **Status**: ✅ Implemented, but authorization is deferred.
  - [x] Use session repository.ListSessionsForUser(user_id)
  - [x] Return active sessions (not revoked, not expired)
  - [x] Include metadata: created_at, last_used_at, user_agent, ip_address
  - [DEFERRED] User can list their own sessions; admin can list for any user. Authorization guard to be added in a subsequent task.
  - Reference: contracts/auth-rpc-additions.proto (lines 225-242)

- [x] **T043** [P] Implement RevokeSession RPC (business logic only)
  - **Status**: ✅ Implemented, but authorization is deferred.
  - [x] Use session repository.RevokeSession(session_id)
  - [DEFERRED] User can revoke their own sessions, admin can revoke any session. Authorization guard to be added in a subsequent task.
  - Reference: contracts/auth-rpc-additions.proto (lines 244-250)

- [ ] **T043A** [P] Implement ExportRoles RPC (FR-045)
  - Accept optional list of role names to export (empty = export all)
  - Use role repository.ListRoles() with optional filter
  - For each role, use enforcer.GetFilteredPolicy(0, role) to get all policies
  - Serialize roles with metadata (name, permissions, label_scope, create_constraints, immutable_keys) to JSON
  - Return JSON string
  - Require admin:role-manage permission
  - Reference: FR-045 (spec.md line 493), plan.md line 563

- [ ] **T043B** [P] Implement ImportRoles RPC (FR-045)
  - Parse JSON string into role definitions
  - For each role:
    - Check if role exists: if yes and overwrite=false, skip with warning
    - If overwrite=true or role doesn't exist: create/update role via repository
    - Use enforcer.RemoveFilteredPolicy() + AddPolicy() to update Casbin policies
    - Validate label_scope_expr as valid go-bexpr syntax
  - Call enforcer.SavePolicy() after all updates
  - Return summary: imported_count, skipped_count, errors[]
  - Idempotent: same import can run multiple times with same result (when overwrite=true)
  - Require admin:role-manage permission
  - Reference: FR-045 (spec.md line 493), plan.md line 564

## Phase 3.7: SDK Auth Helpers (pkg/sdk)

- [x] **T044** Define CredentialStore interface in `pkg/sdk/credentials.go`
  - Interface methods: SaveCredentials, LoadCredentials, DeleteCredentials, IsValid
  - Structure: Credentials{AccessToken, TokenType, ExpiresAt, RefreshToken optional}
  - Interface allows different storage backends (file, in-memory for tests, keychain future)
  - Used by pkg/sdk auth helpers and terraform wrapper
  - Reference: plan.md Constitutional Principle II (pkg/sdk Note on exceptions)

- [x] **T045** [P] Create OIDC login helper in `pkg/sdk/auth.go` (function: LoginWithDeviceCode)
  - Accept CredentialStore interface as parameter (dependency injection)
  - Call `/device_authorization` through zitadel/oidc client helpers to obtain verification URI + user code (FR-003/FR-008)
  - Print/display instructions, optionally attempt `xdg-open`/`open` to launch browser (fall back to manual steps)
  - Poll `/oauth/token` on the provider-specified interval until authorized, denied, or expired (respect `slow_down`)
  - Persist tokens via CredentialStore interface and return metadata for CLI messaging
  - Reference: research.md §3 (device flow), plan.md Constitutional note on HTTP exception

- [x] **T046** [P] Create service account login helper in `pkg/sdk/auth.go` (function: LoginWithServiceAccount)
  - Accept CredentialStore interface and client credentials
  - Use zitadel/oidc client utilities to execute the OAuth2 client credentials flow (no hand-rolled HTTP)
  - Save tokens via CredentialStore interface
  - Return error if auth fails
  - Reference: research.md §4 (client credentials flow)

## Phase 3.8: CLI Authentication Commands

**Note on file locations**: The CLI commands were implemented under command groups for better organization. For example, `login`, `logout`, and `status` are under the `auth` group (e.g., `cmd/gridctl/cmd/auth/login.go`), and role management commands are under the `role` group (e.g., `cmd/gridctl/cmd/role/assign_group.go`). This diverges from the flat file structure initially planned but improves usability.

- [x] **T047** Implement CredentialStore in CLI `cmd/gridctl/internal/auth/credential_store.go`
  - Concrete implementation of pkg/sdk CredentialStore interface
  - Store tokens in ~/.grid/credentials.json with mode 0600
  - Functions implement interface: SaveCredentials, LoadCredentials, DeleteCredentials, IsValid
  - Validate expiry on load
  - Reference: pkg/sdk/credentials.go interface definition

- [x] **T048** Create login command in `cmd/gridctl/cmd/auth/login.go`
  - Create CLI credential store instance
  - Call pkg/sdk.LoginWithDeviceCode(credStore) helper
  - Echo verification URI + user code, show progress while polling, and print success message with token expiry
  - Handle errors (browser open failed, device flow denied/expired, throttling)
  - Reference: quickstart.md §2, pkg/sdk/auth.go LoginWithDeviceCode

- [x] **T049** Create logout command in `cmd/gridctl/cmd/auth/logout.go`
  - Delete local credentials via credential store.DeleteCredentials()
  - Optionally call /auth/logout HTTP endpoint to revoke server-side session
  - Print confirmation message
  - Reference: plan.md §728-729

- [x] **T050** Create auth status command in `cmd/gridctl/cmd/auth/status.go`
  - Load credentials from credential store
  - Call GetEffectivePermissions RPC via generated Connect client (with current user's ID)
  - Display: user identity, roles, token expiry, effective permissions
  - Reference: plan.md §730-732

- [x] **T050A** Create role inspect command in `cmd/gridctl/cmd/role/inspect.go` (function: inspectCmd) (FR-048)
  - Cobra command: `gridctl role inspect <principal_id>`
  - Accept principal ID (user:alice, sa:ci-pipeline, or group:platform-engineers)
  - Call GetEffectivePermissions RPC via generated Connect client with specified principal_id
  - Display: principal type, assigned roles, effective permissions, label scope expressions, create constraints, immutable keys
  - Require admin:role-manage permission (admin-only troubleshooting)
  - Reference: FR-048 (spec.md line 506), existing T037 RPC implementation

- [x] **T051** Create role assign-group command in `cmd/gridctl/cmd/role/assign_group.go` (function: assignGroupCmd)
  - Cobra command: `gridctl role assign-group <group> <role>`
  - Call AssignGroupRole RPC via generated Connect client
  - Print confirmation with assignment details
  - Require admin:group-assign permission
  - Reference: CLARIFICATIONS.md §1 (primary SSO pattern)

- [x] **T052** Create role remove-group command in `cmd/gridctl/cmd/role/remove_group.go` (function: removeGroupCmd)
  - Cobra command: `gridctl role remove-group <group> <role>`
  - Call RemoveGroupRole RPC via generated Connect client
  - Print confirmation

- [x] **T053** Create role list-groups command in `cmd/gridctl/cmd/role/list_groups.go` (function: listGroupsCmd)
  - Cobra command: `gridctl role list-groups [group]`
  - Call ListGroupRoles RPC via generated Connect client (optional filter)
  - Display tab-delimited table: group_name, role_name, assigned_at

- [x] **T053A** Create role export command in `cmd/gridctl/cmd/role/export.go` (function: exportCmd) (FR-045)
  - Cobra command: `gridctl role export [role_names...] --output=roles.json`
  - Call ExportRoles RPC via generated Connect client (optional filter by role names)
  - Write JSON response to file specified by --output flag (default: stdout)
  - Print confirmation with exported count
  - Require admin:role-manage permission
  - Reference: FR-045, plan.md lines 763-766

- [x] **T053B** Create role import command in `cmd/gridctl/cmd/role/import.go` (function: importCmd) (FR-045)
  - Cobra command: `gridctl role import --file=roles.json [--force]`
  - Read JSON from file specified by --file flag
  - Call ImportRoles RPC via generated Connect client with file contents and overwrite=force
  - Print summary: imported_count, skipped_count, errors
  - Require admin:role-manage permission
  - Reference: FR-045, plan.md lines 767-771

## Phase 3.9: SDK Terraform Wrapper (Token Injection & Process Management)

pass-through wrapper command `gridctl tf` that executes a Terraform/Tofu subcommand (e.g., `plan`, `apply`, `init`, `state`, `lock`, `unlock`, etc.) and propagates STDIN/STDOUT/STDERR and exit codes unchanged. `gridctl tf` MUST inject a valid bearer token for the Grid HTTP backend so that all `/tfstate/*` requests sent by the terraform/tofu process include authorization

- [x] **T054** [P] Create binary discovery in `pkg/sdk/terraform/binary.go`
  - Precedence: --tf-bin flag → TERRAFORM_BINARY_NAME env var → auto-detect (terraform, then tofu)
  - Check binary exists and is executable
  - Return clear error if neither terraform nor tofu found
  - Reference: plan.md §743-747, FR-097b, FR-097h

- [x] **T055** [P] Create process spawner in `pkg/sdk/terraform/wrapper.go`
  - Function: Run(ctx, binary, args, credStore CredentialStore) (exitCode int, err error)
  - Pipe STDIN/STDOUT/STDERR to/from subprocess unchanged
  - Preserve exact exit code from subprocess
  - Pass through all args verbatim
  - Run in current working directory (no --cwd flag)
  - Reference: plan.md §748-752, FR-097f, FR-097h

- [x] **T056** [P] Create token injection in `pkg/sdk/terraform/wrapper.go`
  - Accept CredentialStore interface parameter (dependency injection)
  - Define GridContext interface in pkg/sdk for cmd/gridctl to inject its concrete implementation (providing state info such as backend endpoints, etc. as required to wrap TF runs)
  - Set TF_HTTP_USERNAME="gridapi" (can't be blank, gridapi ignores it...) and TF_HTTP_PASSWORD=<bearer_token> environment variables
  - Load credentials from CredentialStore.LoadCredentials()
  - Fail fast if no credentials available in non-interactive mode
  - Never persist token to disk, only pass via env vars
  - Reference: plan.md §753-757, FR-097c, FR-097j, FR-097k

- [ ] **T057** [P] Create 401 detection and guidance in `pkg/sdk/terraform/output.go`
  - Parse stderr for "401" or "Unauthorized" strings
  - On 401: print concise guidance to re-authenticate (e.g., `gridctl auth login`), then exit non-zero. Do not retry automatically in v1.
  - Refresh-token path is not required by v1 integration tests; unit coverage may be added later if implemented.
  - Reference: plan.md §758-762, FR-097e (v1)

- [ ] **T058** [P] Create secret redaction in `pkg/sdk/terraform/output.go`
  - Mask bearer tokens in all console output, logs, crash reports
  - When --verbose: print command line with tokens redacted (show "[REDACTED]")
  - Never print: bearer token values, TF_HTTP_PASSWORD, Authorization headers
  - Reference: plan.md §763-766, FR-097g

- [ ] **T059** [P] Create CI vs interactive detection in `pkg/sdk/terraform/auth.go`
  - Check for TTY, CI env vars (CI=true, GITHUB_ACTIONS, GITLAB_CI, etc.)
  - Non-interactive mode: obtain a token using service account credentials provided via environment variables or a configured CredentialStore; fail fast with a clear message if unavailable
  - Interactive mode: use existing access token from CredentialStore (no client-secret persisted by default)
  - Reference: plan.md §767-770, FR-097j

- [x] **T060** Create gridctl tf command in `cmd/gridctl/cmd/tf/tf.go`
  - Cobra command: `gridctl tf [flags] -- <terraform args>`
  - Flags: --tf-bin (fall back to binary override "TERRAFORM_BINARY_NAME" env var like cdktf), --verbose (debug output)
  - Read .grid file via cmd/gridctl/internal/context (DirCtx) for backend endpoint
  - Pass CLI credential store to pkg/sdk/terraform wrapper
  - Delegate to pkg/sdk/terraform wrapper with credentials and context
  - Hint: If backend.tf missing, print non-blocking suggestion to run `gridctl state init`
  - Reference: plan.md §771-776, FR-097i, FR-097l (CLI-only hint)

- [ ] **T061** Update context file reader in `cmd/gridctl/internal/context/reader.go`
  - Extend existing DirCtx to extract GUID and backend endpoints from .grid file
  - Provide context to pkg/sdk/terraform for validation
  - Note: Context reading is CLI responsibility, not pkg/sdk
  - Reference: plan.md §777-781

## Phase 3.9.5: CLI Bootstrap Commands (Service Account Management)

### Bootstrap Pattern (gridapi sa) - Direct Database Access

- [x] **T091** Create gridapi sa create command in `cmd/gridapi/cmd/sa/create.go`
  - **Bootstrap pattern**: Direct database write using Repository pattern, NO authentication required
  - **Purpose**: Initial setup when no authentication exists yet (similar to `gridapi db init`)
  - **Security**: Assumes local server access (not exposed via HTTP)
  - Cobra command: `gridapi sa create <name> --role <role>`
  - Generate client_id (UUIDv7) and client_secret (32-byte crypto/rand hex)
  - Bcrypt hash client_secret with cost 10
  - Insert into service_accounts table: (id, client_id, secret_hash, name, description, created_at, created_by=system_user_id)
  - Print to stdout (machine-readable format):
    ```
    Client ID: <client_id>
    Client Secret: <secret>
    (Save this secret - it will not be shown again)
    ```
  - Reference: Bootstrap pattern clarification, quickstart.md Scenario 3 Mode 2 (first SA creation)

- [ ] **T092** Create gridapi sa list command in `cmd/gridapi/cmd/sa/list.go` (optional helper)
  - **Bootstrap pattern**: Direct database read, NO authentication required
  - List all service accounts with client_id, name, created_at (no secrets)
  - Tab-delimited output for scripting
  - Reference: Bootstrap pattern (operational helper)

### Production Pattern (gridctl sa) - Authenticated RPC Calls

- [ ] **T093** Create gridctl sa create command in `cmd/gridctl/cmd/sa/create.go`
  - **Production pattern**: Calls CreateServiceAccount RPC via authenticated connection
  - Requires valid auth token with `auth.AdminServiceAccountManage` permission
  - Cobra command: `gridctl sa create --name <name> [--description <desc>]`
  - Call CreateServiceAccount RPC from T028 via generated Connect client
  - Print client_id and plaintext secret (shown once)
  - Reference: T028 (RPC handler), quickstart.md Scenario 3 Mode 2 (lines 305-314)

- [ ] **T094** Create gridctl sa list command in `cmd/gridctl/cmd/sa/list.go`
  - Call ListServiceAccounts RPC from T029 via generated Connect client
  - Display tab-delimited table: client_id, name, created_at, disabled
  - Requires `auth.AdminServiceAccountManage` permission
  - Reference: T029 (RPC handler)

- [ ] **T095** Create gridctl sa revoke command in `cmd/gridctl/cmd/sa/revoke.go`
  - Call RevokeServiceAccount RPC from T030 via generated Connect client
  - Cascade revocation: all active sessions invalidated (FR-070b)
  - Print confirmation with count of revoked sessions
  - Requires `auth.AdminServiceAccountManage` permission
  - Reference: T030 (RPC handler), quickstart.md Scenario 6 (lines 572-587)

- [ ] **T096** Create gridctl sa rotate command in `cmd/gridctl/cmd/sa/rotate.go`
  - Call RotateServiceAccount RPC from T031 via generated Connect client
  - Print new client_secret (shown once)
  - Note: Existing tokens remain valid (JWT self-contained, FR-070a)
  - Requires `auth.AdminServiceAccountManage` permission
  - Reference: T031 (RPC handler)

## Phase 3.10: JS/SDK Auth Helpers (Browser)

- [ ] **T062** Create auth helpers in `js/sdk/auth.ts`
  - Functions: initLogin(redirectUri), handleCallback(code, state), logout(), refreshToken()
  - Use direct HTTP fetch to /auth/* endpoints (NOT Connect RPC - Constitutional exception)
  - Set credentials: 'include' for httpOnly cookies
  - Export TypeScript interfaces: UserInfo, AuthState, AuthError
  - Reference: research.md §13 (lines 942-974), plan.md Constitutional note on HTTP auth endpoints

## Phase 3.11: Webapp Authentication (React Integration)

- [ ] **T063** [P] Create AuthContext provider in `webapp/src/context/AuthContext.tsx`
  - Import auth helpers from js/sdk/auth.ts
  - AuthState: user (UserInfo | null), loading (boolean), error (string | null)
  - Functions: login, logout, refreshToken (delegate to js/sdk/auth)
  - useEffect on mount: attempt token refresh from httpOnly cookie
  - Mirror existing GridContext.tsx structure
  - Reference: research.md §13 (lines 885-940)

- [ ] **T065** [P] Create usePermissions hook in `webapp/src/hooks/usePermissions.ts`
  - Use generated Connect client to call GetEffectivePermissions RPC from js/sdk/gen
  - Cache permissions in state
  - Helper: hasPermission(action: string) → boolean

- [ ] **T066** [P] Create AuthGuard component in `webapp/src/components/AuthGuard.tsx`
  - Use useAuth hook from AuthContext
  - Conditional rendering: redirect to login if unauthenticated
  - Preserve return_to parameter in login redirect
  - Show loading spinner while auth state loading
  - Reference: research.md §13 (lines 976-997)

- [ ] **T067** [P] Create LoginCallback component in `webapp/src/components/LoginCallback.tsx`
  - Parse code/state from URL query params
  - Call js/sdk/auth.handleCallback()
  - Redirect to return_to on success, show error message on failure

- [ ] **T068** [P] Create UserMenu component in `webapp/src/components/UserMenu.tsx`
  - Display user email/name from useAuth
  - Logout button calling useAuth().logout()
  - Follow existing webapp component patterns

- [ ] **T069** [P] Create ProtectedAction component in `webapp/src/components/ProtectedAction.tsx`
  - Conditional rendering based on usePermissions().hasPermission() check
  - Props: requiredPermission (string), children, fallback (optional)
  - Note: Prepared for future use; dashboard is currently READ ONLY
  - Reference: research.md §13 (lines 999-1023)

- [ ] **T070** Update App.tsx in `webapp/src/App.tsx`
  - Wrap existing GridProvider with AuthProvider (auth wraps grid)
  - Import and use AuthProvider from context/AuthContext

- [ ] **T071** Create login page in `webapp/src/pages/Login.tsx`
  - Button to initiate login via useAuth().login()
  - Uses js/sdk/auth.initLogin() under the hood
  - Simple conditional rendering (no routing library needed)

- [ ] **T072** Update gridApi.ts with 401 interceptor in `webapp/src/services/gridApi.ts`
  - Add Connect interceptor to catch unauthenticated errors
  - On 401: redirect to /login?return_to=<current_path>
  - Reference: research.md §13 (lines 1025-1048)

- [ ] **T073** [P] Create user profile page in `webapp/src/pages/Profile.tsx` (FR-079)
  - Display user's effective permissions via GetEffectivePermissions RPC (js/sdk/gen Connect client)
  - Show assigned roles, label scope expressions, create constraints, immutable keys
  - Display in user-friendly format (not raw JSON)
  - Add route and link from UserMenu component
  - Reference: FR-079

- [ ] **T074** [P] Create label policy viewer in `webapp/src/components/PolicyViewer.tsx` (FR-082)
  - Display current label validation policy
  - Fetch policy via existing policy API (js/sdk/gen Connect client)
  - Show: allowed_keys, allowed_values, max_keys, max_value_len
  - Implement as modal or dedicated page
  - Reference: FR-082

- [ ] **T075** [P] Add browser console logging in `js/sdk/auth.ts` and webapp error handlers (FR-089)
  - Log authentication errors to console.error with sanitized details in js/sdk/auth.ts
  - Log authorization errors (403) to console.warn with action/resource info in webapp error boundaries
  - Never log tokens or credentials
  - Display user-friendly error messages in UI (FR-085)
  - Reference: FR-085, FR-089

## Phase 3.12: Integration (Wire Everything Together)

- [x] **T076** Apply auth middleware to Chi router in `cmd/gridapi/cmd/serve.go`
  - Order: Authentication middleware → Authorization middleware → Business logic
  - Attached to Chi router globally to protect all subsequent handlers.
  - Reference: research.md §1 (lines 13-46)

- [x] **T077** Apply auth middleware to Connect RPC service in `cmd/gridapi/internal/server/router.go`
  - Note: Completed via global middleware in T076. All Connect handlers are now protected.

- [x] **T078** Update tfstate endpoints with auth in `cmd/gridapi/internal/server/tfstate.go`
  - Note: Completed via global middleware in T076. All tfstate handlers are now protected.
  - Middleware extracts token from HTTP Basic Auth password field (task T018).
  - Authorization actions used: auth.TfstateRead, auth.TfstateWrite, auth.TfstateLock, auth.TfstateUnlock

- [ ] **T079** Add structured auth/authz audit logging in `cmd/gridapi/internal/middleware/audit.go` (FR-098, FR-099)
  - Create audit middleware using Chi logger
  - Log authentication attempts: timestamp, user/service_account identity, source IP, success/failure
  - Log authorization decisions: timestamp, principal, action, resource, labels, decision (allow/deny), reason
  - Use structured logging (JSON format) for parsing/analysis
  - Never log bearer tokens or credentials
  - Reference: FR-098, FR-099

## Phase 3.12.5: Apply Authorization Guards

- [ ] **T082** Apply authz guards to Service Account RPCs in `connect_handlers_auth.go`
  - Handlers: CreateServiceAccount, ListServiceAccounts, RevokeServiceAccount, RotateServiceAccount
  - Permission: `auth.AdminServiceAccountManage`

- [ ] **T083** Apply authz guards to direct Role Assignment RPCs in `connect_handlers_auth.go`
  - Handlers: AssignRole, RemoveRole
  - Permission: `auth.AdminUserAssign`

- [ ] **T084** Apply authz guards to Group Role Assignment RPCs in `connect_handlers_auth.go`
  - Handlers: AssignGroupRole, RemoveGroupRole, ListGroupRoles
  - Permission: `auth.AdminGroupAssign`

- [ ] **T085** Apply authz guards to Role Management RPCs in `connect_handlers_auth.go`
  - Handlers: CreateRole, UpdateRole, DeleteRole, ExportRoles, ImportRoles
  - Permission: `auth.AdminRoleManage`

- [DEFERRED] **T086** Apply authz guard to RevokeSession RPC in `connect_handlers_auth.go`
  - Note: Allow user to revoke their own session, but require `auth.AdminSessionRevoke` to revoke others' sessions.

- [ ] **T087** Implement authorization logic within the authorization middleware (`authz.go`)
  - The middleware should resolve the object and action from the request path.
  - It should then call the Casbin enforcer: `enforcer.Enforce(principal, object, action, labels)`.
  - This will protect all non-admin Connect RPCs (State, Dependency, Policy) and `tfstate` endpoints.

- [x] **T080** Update Docker Compose with Keycloak in `docker-compose.yml`
  - Add Keycloak service (quay.io/keycloak/keycloak:22.0)
  - Add PostgreSQL init script for Keycloak database
  - Environment: KEYCLOAK_ADMIN, KC_DB settings
  - Ports: 8443:8080 (Keycloak HTTP)
  - Local dev configuration: KC_HTTP_ENABLED=true, KC_HOSTNAME_STRICT_HTTPS=false (FR-100 Local Development Exception)
  - NOTE: Plain HTTP is permitted for local dev; production MUST use HTTPS/TLS
  - Health check for startup synchronization
  - Reference: plan.md §356-408, FR-100

- [x] **T081** Create PostgreSQL init script in `initdb/01-init-keycloak-db.sql`
  - Create keycloak user and database (idempotent)
  - Runs on first PostgreSQL container start
  - Reference: plan.md §410-426

## Phase 3.13: Testing

### Repository Tests

- [ ] **T078** Create auth repository unit tests in `cmd/gridapi/internal/repository/auth_repository_test.go`
  - Test session CRUD, revocation checks (sessions.revoked column)
  - Test service account CRUD, bcrypt secret hashing, rotation
  - Test group-role CRUD, unique constraints
  - Test cascade revocation: RevokeAllSessionsForServiceAccount
  - Use real PostgreSQL database (not mocks), run migrations before tests
  - Follow existing *_test.go patterns
  - Reference: plan.md §843-845

### Auth Core Tests

- [ ] **T079** Create JWT validation unit tests in `cmd/gridapi/internal/auth/jwt_test.go`
  - Mock JWKS endpoint using httptest.Server
  - Test cases: valid JWT, expired JWT, invalid signature, wrong issuer, missing jti claim
  - **NEW**: Test JWT creation from T009A: verify token structure (header.payload.signature), jti presence, correct signing algorithm
  - **NEW**: Test revoked JWT: insert jti into revoked_jti table, verify 401 on next request
  - Reference: plan.md §846-849, implementation-adjustments.md §798-1010 (debugging patterns)

- [ ] **T080** Create JWT claim extraction tests in `cmd/gridapi/internal/auth/claims_test.go`
  - Flat array: `["dev-team", "contractors"]`
  - Nested objects: `[{"name": "dev-team"}]` with path="name"
  - Edge cases: empty groups, nil groups, invalid types
  - Reference: research.md §9 (test examples), plan.md §850-854

### Middleware Tests

- [ ] **T081** Create middleware unit tests in `cmd/gridapi/internal/middleware/middleware_test.go`
  - Use httptest.NewRequest to test in isolation
  - Scenarios: no token (401), invalid token (401), valid token (pass through)
  - Lock-aware bypass: test lock holder bypasses label scope for tfstate:write/unlock
  - Reference: plan.md §855-858

- [ ] **T082** Create audit logging tests in `cmd/gridapi/internal/middleware/audit_test.go`
  - Test authentication attempt logging (success/failure with IP)
  - Test authorization decision logging (allow/deny with reason)
  - Verify no tokens/credentials logged
  - Verify structured JSON format
  - Reference: FR-098, FR-099

### Contract Tests

- [ ] **T083** Create auth RPC contract tests in `tests/contract/auth_contract_test.go`
  - Test all RPCs from tasks T028-T043B (proto contract verification)
  - CreateServiceAccount_Success, CreateServiceAccount_Unauthorized
  - AssignRole_ValidRole, AssignRole_NonexistentRole
  - AssignGroupRole, RemoveGroupRole, ListGroupRoles (group-role RPCs)
  - RevokeServiceAccount cascades to sessions (FR-070b)
  - GetEffectivePermissions with multiple roles
  - CreateRole_ValidDefinition, CreateRole_InvalidActionEnum
  - **NEW**: Test hash comparison pattern from implementation-adjustments.md:
    - Client token SHA256 vs database token_hash (for sessions)
    - Verify token stored correctly at issuance
  - Follow existing tests/contract/ patterns
  - Reference: plan.md §859-862, implementation-adjustments.md §823-877 (hash verification scripts)

### Integration Tests

- [ ] **T084** Create integration tests with Keycloak in `tests/integration/auth_test.go`
  - **UPDATED**: Follow verification patterns from quickstart.md Scenarios 1-12
  - Use Keycloak in docker-compose for Mode 1 (External IdP) tests
  - Test Mode 2 (Internal IdP) with Grid-issued JWTs
  - **Key scenarios to implement**:
    - Scenario 1: SSO user login (web flow) - Mode 1
    - Scenario 2: CLI device code flow - Mode 1 vs Mode 2
    - Scenario 3: Service account authentication - Mode 1 (IdP client) vs Mode 2 (Grid entity)
    - Scenario 4: Role assignment and permission checks (RBAC, label scope, constraints)
    - Scenario 6: Service account revocation with cascade (FR-070b)
    - Scenario 9: Group-based authorization (JWT groups claim, union semantics)
    - Scenario 11: Terraform wrapper with token injection
    - Scenario 12: Lock-aware authorization bypass
  - **JWT verification patterns** (from implementation-adjustments.md §798-1010):
    - Verify JWT structure: header.payload.signature (3 parts separated by dots)
    - Verify jti claim presence in token
    - Compare client token SHA256 hash vs database token_hash
    - Test revocation: insert jti into revoked_jti, verify 401 on next request
  - Test /tfstate endpoints extract token from HTTP Basic Auth password field
  - Reference: quickstart.md §1-1280 (complete scenarios), implementation-adjustments.md §798-1010 (debugging scripts)

### CLI Tests

- [ ] **T085** Create CLI E2E tests in `cmd/gridctl/cmd/auth_test.go`
  - Mock HTTP server for /auth/login, /auth/callback endpoints
  - Test login, logout, status commands
  - Test role assign-group, remove-group, list-groups commands
  - Reference: plan.md §868-870

### Terraform Wrapper Tests

- [ ] **T086** Create Terraform wrapper unit tests in `pkg/sdk/terraform/wrapper_test.go`
  - Binary discovery: test precedence (--tf-bin → TERRAFORM_BINARY_NAME → auto-detect)
  - STDIO pass-through: verify output streaming, exit codes preserved
  - Token injection: verify TF_HTTP_PASSWORD set correctly, never logged
  - 401 retry: test single retry on 401, then fail
  - Secret redaction: test --verbose masks tokens
  - Reference: plan.md §871-876

- [ ] **T087** Create Terraform wrapper E2E tests in `tests/integration/terraform_wrapper_test.go`
  - Mock HTTP backend using httptest.Server with 401 scenarios
  - Test commands: gridctl tf plan, gridctl tf apply with auth
  - CI mode: test non-interactive service account auth
  - Reference: plan.md §877-880

### Webapp Tests

- [ ] **T088** Create webapp auth flow tests in `webapp/src/__tests__/auth.test.tsx`
  - Use React Testing Library, mock useAuth hook
  - Test AuthGuard rendering (loading, unauthenticated, authenticated states)
  - Test ProtectedAction visibility based on permissions
  - Test login flow, 401 handling in gridApi interceptor
  - Follow existing dashboard_*.test.tsx patterns
  - Reference: research.md §13 (lines 1075-1090), plan.md §881-884

- [ ] **T089** Create webapp profile/policy viewer tests in `webapp/src/__tests__/profile.test.tsx`
  - Test Profile page displays effective permissions correctly
  - Test PolicyViewer displays label validation policy
  - Test browser console logging for auth/authz errors
  - Reference: FR-079, FR-082, FR-089

## Dependencies

**Sequential Dependencies** (must complete in order):
- Database (T001-T002) → Config (T003) → Casbin Foundation (T004-T008)
- Config (T003) → Auth Core (T009-T012)
- Casbin Foundation (T005) → Dynamic Grouping (T012)
- Repositories (T013-T016) before Middleware (T017-T019)
- Auth Core (T009-T012) before Middleware (T017-T019)
- Middleware (T017-T019, T075) before Integration (T072-T077)
- HTTP Endpoints (T020-T025) before Integration (T072)
- RPC Handlers (T026-T041) before Integration (T073)
- Webapp core (T058-T068) before Webapp profile/policy (T069-T071)
- Tests (T078-T089) should be written BEFORE implementing features (TDD)

**Parallel Execution Groups** (can run concurrently):
- [P] Auth Core: T009 (provider) ✅, T009B (relying party), T010 (dynamic verifier), T011 (claims) ✅ (different files in auth/)
- [P] Repositories: T015 (group_role_repository.go can be parallel with session/service_account repos)
- [P] Middleware: T017 (authn - needs update), T018 ✅, T019 ✅ (authn.go, authz.go, context.go)
- [P] HTTP Endpoints: T020A (device UI), T020B (SSO login), T020C (SSO callback), T023 (logout); T021, T022 (integration tests)
- [P] RPC Handlers: T028-T043B (all independent RPCs in connect_handlers.go)
- [P] CLI TF Wrapper Core: T054-T059 (pkg/sdk/terraform/ files)
- [P] Webapp Components: T063-T069 (context, services, hooks, components)
- [P] Webapp Profile/Policy: T073-T075 (Profile.tsx, PolicyViewer.tsx, authApi.ts logging)

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
Task 1: "Implement login UI handler in cmd/gridapi/internal/server/auth_handlers.go per T020"
Task 2: "Write integration tests for OIDC endpoints per T021"
Task 3: "Implement device verification UI handler per T022"
Task 4: "Implement logout handler per T023"
Task 5: "Write device authorization integration tests per T024"
Task 6: "Write device token polling integration tests per T025"

# Note: Most OIDC endpoints are auto-mounted by zitadel/oidc provider via op.CreateRouter()
# Only custom handlers needed: login UI (T020), device verification UI (T022), logout (T023)
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

### Critical JWT Architecture (NEW)
- [ ] Internal IdP issues JWTs (not opaque tokens) - T009A
- [ ] JWTs contain jti claim for revocation tracking
- [ ] JWT signature validated via JWKS - T010A
- [ ] JTI denylist checked after JWT validation - T017A
- [ ] revoked_jti table exists and is used - T001A

### Database Schema (NEW)
- [ ] revoked_jti table created with jti, subject, exp, revoked_at, revoked_by - T001A
- [ ] users table created for internal mode (id, email, password_hash, disabled_at) - T001B
- [ ] All migrations run successfully (auth schema + seed data)

### Bootstrap vs Production (NEW)
- [ ] gridapi sa create works without authentication (bootstrap) - T091
- [ ] gridctl sa create requires authentication (production) - T093
- [ ] Bootstrap creates first service account for auth system initialization

### Core Authentication
- [ ] OIDC login flow works (web and CLI)
- [ ] Service account authentication works (Mode 1: IdP client, Mode 2: Grid entity)
- [ ] JWT validation mode-based (single issuer per deployment)
- [ ] Token expiry enforced (configurable TTL for Terraform tokens)

### Core Authorization
- [ ] Casbin enforcer loads policies from database
- [ ] Role assignment and group-role mapping work
- [ ] Create constraints enforced (product-engineer can only create env=dev)
- [ ] Immutable keys enforced (product-engineer cannot modify env label)
- [ ] Label scope filtering works (product-engineer only sees env=dev states)
- [ ] Admin has full access (platform-engineer wildcard permissions)
- [ ] Service account limited to Data Plane (tfstate:* only)

### Revocation
- [ ] JTI revocation works (insert jti into revoked_jti → immediate 401) - T001A
- [ ] Service account revocation cascades to JWT invalidation (FR-070b)
- [ ] Terraform long-lived tokens (90-120 min) work without mid-run expiry

### Integration
- [ ] Terraform HTTP Backend auth via Basic Auth password field works
- [ ] Lock holders bypass label scope for tfstate:write/unlock
- [ ] Dependency authorization checks both source and target states
- [ ] Clear error messages on authorization denial

### Testing
- [ ] All contract tests pass
- [ ] All integration tests pass (follow quickstart.md scenarios)
- [ ] JWT verification tests pass (structure, jti, revocation)
- [ ] Webapp auth flow works (login, logout, AuthGuard, 401 handling)
- [ ] CLI tf wrapper works (token injection, 401 retry, secret redaction)

---

**Status**: Major AuthN/AuthZ implementation complete for Mode 1 & Mode 2

**Total Tasks**: 103
- **Completed**: 75 tasks (73% complete)
- **Remaining**: 28 tasks (primarily webapp UI, terraform wrapper, testing infrastructure)

**Recent Implementation** (as of db274cbcb):
- ✅ Mode 1 (External IdP) with Keycloak integration and JIT user provisioning
- ✅ Mode 2 (Internal IdP) with JWT signing and service account authentication
- ✅ Casbin model embedded in binary (no external config file)
- ✅ JWT revocation infrastructure (revoked_jti table + repository)
- ✅ Bexpr label scope expressions working correctly
- ✅ Dynamic group-to-role mapping via JWT claims
- ✅ IAM bootstrap commands for initial setup
- ✅ Integration test suites for both modes

**Breakdown**:
- Foundation: 10/10 ✅ COMPLETE (T001-T008, T001A ✅, T001B ✅)
- Config: 3/3 ✅ COMPLETE (T003 ✅, T003A ✅, T003B ✅) *TerraformTokenTTL deferred to run-token branch*
- Auth Core: 6/6 ✅ COMPLETE (T009 ✅, T009A ✅, T009B ✅, T010 ✅, T010A ✅, T011 ✅, T012 ✅) *All CRITICAL blockers resolved*
- Repositories: 4/4 ✅ COMPLETE (T013-T016)
- Middleware: 4/4 ✅ COMPLETE (T017 ✅, T017A ⚠️ *see note*, T018 ✅, T018B ✅, T019 ✅)
- HTTP Endpoints: 3/6 ⚠️ PARTIAL (T020B ✅, T020C ✅, T020D ❌, T021 ❌, T022 ❌, T023 ✅) *T020A N/A*
- Proto/Codegen: 2/2 ✅ COMPLETE (T026-T027)
- RPC Handlers: 16/18 ✅ MOSTLY COMPLETE (T028-T043 ✅, T043A ❌, T043B ❌) *Authz guards pending T082-T087*
- pkg/sdk Auth: 3/3 ✅ COMPLETE (T044-T046)
- CLI Auth: 10/10 ✅ COMPLETE (T047-T053, T053A, T053B)
- Bootstrap/Production SA: 4/6 ⚠️ PARTIAL (T091 ✅, T092 ✅, T093-T096 ❌) *gridapi commands exist, gridctl commands TODO*
- pkg/sdk Terraform: 0/8 ❌ TODO (T054-T061)
- js/sdk Auth: 0/1 ❌ TODO (T062)
- Webapp: 0/13 ❌ TODO (T063-T075)
- Integration Wiring: 6/7 ✅ MOSTLY COMPLETE (T076-T081 ✅, T079 ❌ *audit logging*)
- Authorization Guards: 0/6 ❌ TODO (T082-T087)
- Testing: 2/12 ⚠️ PARTIAL (Mode 1 & Mode 2 integration tests ✅, remaining unit/contract tests ❌)

## Implementation Notes

### ✅ **Resolved Critical Blockers**

1. **T009A - JWT Token Issuance** ✅ COMPLETE
   - Mode 2 internal IdP now issues JWTs (not opaque tokens)
   - Signing keys auto-generated and persisted at `tmp/keys/signing-key.pem`
   - Tokens include `jti` claim for revocation tracking
   - Implementation: `cmd/gridapi/internal/auth/oidc.go:726` (`AccessTokenTypeJWT`)

2. **T010A - JWT Validation** ✅ COMPLETE
   - Universal JWT validation for both Mode 1 and Mode 2
   - Single-issuer verifier configured based on deployment mode
   - Signature and claims validated via JWKS
   - Implementation: `cmd/gridapi/internal/auth/jwt.go`

3. **T017A - Revocation Check** ✅ COMPLETE
   - JWT revocation check wired in authn middleware (line 69)
   - `revoked_jti` repository properly querying database
   - Test fixed: Now uses protected endpoint (`/state.v1.StateService/ListStates`) instead of `/health`
   - All Mode 2 integration tests passing (5/5)
   - Implementation: `cmd/gridapi/internal/middleware/authn.go:69-79`

### ✅ **Infrastructure Additions**

4. **T001A - Revoked JTI Table** ✅ COMPLETE
   - Table created: `revoked_jti (jti TEXT PRIMARY KEY, subject, exp, revoked_at, revoked_by)`
   - Repository implemented: `cmd/gridapi/internal/repository/bun_revoked_jti_repository.go`
   - Used for JWT revocation denylist (check after signature validation)

5. **T001B - Users Table** ✅ COMPLETE (Mode 1 JIT provisioning)
   - Table created: `users (id UUID, subject TEXT, email TEXT, name TEXT, disabled_at TIMESTAMP)`
   - Used for JIT user provisioning in Mode 1 (external IdP)
   - Mode 2 (internal IdP) currently only supports service accounts
   - Implementation: `cmd/gridapi/internal/middleware/authn.go:196-287` (createUserFromExternalJWT)

6. **T020D - Local Login Handler** ❌ TODO (Mode 2 web users)
   - POST /auth/login endpoint NOT YET IMPLEMENTED
   - Mode 2 currently supports service account authentication only
   - Future: Add username/password authentication for web users

### 🔧 **Configuration Updates**

7. **T003 - TerraformTokenTTL (Deferred)**
   - Do NOT add in v1. Will be introduced with the run-token exchange implementation.
   - Interim: Internal IdP access-token TTL defaults to 120 minutes (code-level default) and External IdP guidance recommends 120–180 minutes.

### 🛠️ **Bootstrap Pattern**

8. **T091-T092 - gridapi sa commands** ✅ COMPLETE
   - Bootstrap commands: `gridapi sa create`, `gridapi sa list`, `gridapi sa assign`, `gridapi sa unassign`
   - Direct database access via repositories (no authentication required)
   - Used for initial Mode 2 setup (bootstrap first service account)
   - **Note**: Should be refactored to use shared services layer (not direct repository access)
   - Implementation: `cmd/gridapi/cmd/sa/` (create.go, list.go, assign.go, unassign.go)
   - Used in Mode 2 integration tests: `tests/integration/auth_mode2_test.go`

9. **T093-T096 - gridctl sa commands** ❌ TODO
   - Production service account management via authenticated RPC calls
   - Requires valid auth token with `AdminServiceAccountManage` permission
   - Commands: create, list, revoke, rotate
   - Will call RPC handlers from T028-T031

**Key Architectural Decisions**:
- **JWT-First Architecture**: All tokens are JWTs, revocation via `jti` denylist
- **Mode-Based Authentication**:
  - **Mutually Exclusive Modes**: Deployment chooses ONE authentication mode. Hybrid mode NOT supported.
  - **Mode 1 (External IdP Only)**: Grid validates tokens from external IdP (Keycloak, Entra ID). Service accounts are IdP clients. Sessions created on first request.
  - **Mode 2 (Internal IdP Only)**: Grid issues and validates its own tokens. Service accounts managed in Grid. Sessions persisted at issuance time.
  - **Single-Issuer Validation**: Middleware configures verifier for ONE issuer based on deployment mode (no dynamic issuer selection)
- **UUIDv7** for all entity IDs (service accounts, users) - consistent with existing State GUID pattern
- **Repository Pattern** throughout - no direct table access in handlers
- **Casbin Enforcer Methods** - use enforcer.AddRoleForUser(), GetRolesForUser(), etc. instead of manual casbin_rules queries
- **zitadel/oidc Provider** (T009) - Mode 2 only: hosts full OIDC authorization server, auto-mounts `/device_authorization`, `/token`, `/keys`, `/.well-known/openid-configuration`
- **golang.org/x/oauth2 + coreos/go-oidc** (T009B) - Mode 1 only: relying party integration for external IdP SSO
- **Mode-Based Verifier** (T010) - single verifier configured for deployment mode (Mode 1: external IdP issuer; Mode 2: Grid issuer)
- **CredentialStore Interface** - defined in pkg/sdk, implemented in CLI, injected into SDK helpers and terraform wrapper
- **pkg/sdk Auth Helpers** - reusable OIDC/service account login flows (LoginWithDeviceCode, LoginWithServiceAccount) using zitadel/oidc client utilities
- **js/sdk Auth Module** - browser auth helpers (initLogin, handleCallback) used by webapp, keeps auth logic DRY
- **HTTP Auth Endpoints** - direct HTTP fetch (not RPC) per Constitutional exception for /auth/* routes
- **Generated Connect Clients** - webapp uses js/sdk/gen for all RPC calls (GetEffectivePermissions, etc.)
- **Custom Chi Middleware** - built around casbin/v2 enforcer (no chi-authz v1 dependency), includes lock-aware bypass and Terraform Basic Auth shim
