# Implementation Plan: Authentication, Authorization, and RBAC

**Branch**: `006-authz-authn-rbac` | **Date**: 2025-10-11 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/Users/vincentdesmet/tcons/grid/specs/006-authz-authn-rbac/spec.md`

## Execution Flow (/speckit.plan command scope)
```
1. Load feature spec from Input path
   → ✅ COMPLETE: Spec loaded with clarifications
2. Fill Technical Context (scan for NEEDS CLARIFICATION)
   → ✅ COMPLETE: All clarifications resolved
3. Fill Constitution Check section
   → ✅ COMPLETE: Evaluated against Constitution v2.0.0
4. Evaluate Constitution Check
   → ✅ PASS: No violations, follows existing patterns
5. Execute Phase 0 → research.md
   → ✅ COMPLETE: Technology decisions documented
6. Execute Phase 1 → contracts, data-model.md, quickstart.md, CLAUDE.md
   → ✅ COMPLETE: Design artifacts generated
7. Re-evaluate Constitution Check
   → ✅ PASS: Design follows constitutional principles
8. Plan Phase 2 → Describe task generation approach
   → ✅ COMPLETE: Task strategy documented
9. STOP - Ready for /speckit.tasks command
```

**IMPORTANT**: The /speckit.plan command STOPS at step 9. Phases 2-4 are executed by other commands:
- Phase 2: /speckit.tasks command creates tasks.md
- Phase 3-4: Implementation execution (manual or via tools)

## Summary

This feature adds comprehensive authentication (AuthN), authorization (AuthZ), and role-based access control (RBAC) to Grid's API server, protecting both the Connect RPC Control Plane (state/dependency/policy APIs) and the Terraform HTTP Backend Data Plane (/tfstate endpoints).

### Deployment Mode Architecture

Grid supports **two mutually exclusive authentication modes**. A deployment chooses exactly ONE mode. Hybrid mode is NOT supported.

#### Mode 1: External IdP Only (Recommended)
**Principle**: Grid is a Resource Server - validates tokens, never issues them.

**Configuration**:
- `EXTERNAL_IDP_ISSUER`, `EXTERNAL_IDP_CLIENT_ID`, `EXTERNAL_IDP_CLIENT_SECRET`, `EXTERNAL_IDP_REDIRECT_URI`
- `OIDC_ISSUER` must be empty

**Token Flow**:
- **Human SSO (Web/CLI)**: User authenticates at external IdP → IdP issues JWT (`iss` = IdP URL) → Grid validates via JWKS
- **Service Accounts**: Created as IdP clients (not Grid entities) → machine calls IdP's `/token` with client credentials → IdP issues JWT → Grid validates

**Session Creation**: On first API request after token validation

**Implementation**:
- `coreos/go-oidc/v3` for token validation
- `golang.org/x/oauth2` for authorization code flow (SSO)
- No Grid signing keys, no Grid OIDC provider endpoints
- Custom handlers: `/auth/sso/login`, `/auth/sso/callback`

**Benefits**:
- Simplest deployment (no key management)
- Offloads all identity concerns to enterprise IdP
- Standard OAuth2/OIDC flows

#### Mode 2: Internal IdP Only (Air-Gapped)
**Principle**: Grid is a self-contained IdP - issues and validates its own tokens.

**Configuration**:
- `OIDC_ISSUER` set to Grid's URL (e.g., `https://grid.example.com`)
- `EXTERNAL_IDP_*` must be empty

**Token Flow**:
- **Service Accounts**: Created in Grid → machine calls Grid's `/token` with client credentials → Grid issues JWT (`iss` = Grid URL)
- **Human Users**: Grid manages credentials (username/password) → authentication via Grid's login forms → Grid issues tokens

**Session Creation**: At issuance time in `CreateAccessToken()` callback

**Implementation**:
- `zitadel/oidc/v3/pkg/op` for Grid as OIDC provider
- Auto-mounted endpoints: `/device_authorization`, `/token`, `/keys`, `/.well-known/openid-configuration`
- Grid-managed signing keys (RSA 2048)
- Immediate revocation support via `sessions.revoked`

**Tradeoffs**:
- Full autonomy (no external dependencies)
- Operational burden: key rotation, user credential management, login UI

#### Token Validation Strategy
Authentication middleware uses **single-issuer validation** based on deployment mode:
- **Mode 1**: Verifier configured for `ExternalIdP.Issuer` (IdP's URL)
- **Mode 2**: Verifier configured for `OIDC.Issuer` (Grid's URL)
- Middleware checks `cfg.OIDC.ExternalIdP != nil` to determine mode

### Implementation Components

- **Casbin** for policy-based authorization with custom Chi middleware integration (no external chi-authz wrapper)
- **Mode 1 (External IdP)**: `golang.org/x/oauth2` + `coreos/go-oidc/v3` for token validation
- **Mode 2 (Internal IdP)**: `zitadel/oidc/v3/pkg/op` for Grid as OIDC provider, `zitadel/oidc/v3/pkg/client` for CLI device flow
- **OAuth2 Client Credentials** flow for service accounts (Mode 1: IdP-issued; Mode 2: Grid-issued)
- **OIDC Device Code flow (RFC 8628)** for CLI authentication (Mode 2 only)
- **OIDC Authorization Code flow** for web SSO (Mode 1 only)
- **Group-to-Role Mapping** for enterprise SSO integration (IdP manages group membership, Grid manages role assignments)

Key architectural decisions:
1. **Mutually exclusive deployment modes**: Mode 1 (External IdP Only) OR Mode 2 (Internal IdP Only). Hybrid mode NOT supported.
2. **Mode-based token validation**: Middleware selects single verifier based on `cfg.OIDC.ExternalIdP != nil`
3. **Session persistence strategies**:
   - Mode 1: Sessions created on first request after token validation
   - Mode 2: Sessions created at issuance time in `CreateAccessToken()` callback
   - Both support immediate revocation via `sessions.revoked` column (FR-007/FR-102a)
4. Casbin enforces permissions via purpose-built Chi middleware layered over casbin/v2 with union (OR) semantics
5. Database stores: roles, user-role assignments, group-role mappings, service accounts, sessions, casbin_rule (Casbin policies)
6. Label-scoped access enforced via go-bexpr expressions evaluated at enforcement time using custom bexprMatch function
7. Three default roles seeded: service-account, platform-engineer, product-engineer
8. No scope intersection logic needed - Casbin's default union semantics handle multiple roles automatically
9. msales/casbin-bun-adapter shares existing *bun.DB connection pool (no separate DB connection)
10. Group membership extracted from JWT claims (configurable field, supports flat and nested formats via mapstructure)
11. Dynamic Casbin grouping at authentication time: user → groups (JWT) → roles (database) → policies (Casbin)
12. Consistent prefix conventions: user:, group:, sa:, role: for all Casbin identifiers
13. **Terraform HTTP Backend uses Basic Auth**: Server extracts bearer token from HTTP Basic Auth password field (username ignored, similar to [GitHub API pattern](https://docs2.lfe.io/guides/getting-started/#:~:text=The%20easiest%20way%20to%20authenticate,prompt%20you%20for%20the%20password.)). Client injects via TF_HTTP_PASSWORD. Lock-aware bypass: principals holding locks retain tfstate:write/unlock even if label scope changes.
14. **gridctl tf wrapper** in pkg/sdk handles process spawning, token injection, 401 retry, and secret redaction. CLI provides .grid context and missing backend.tf hints.

## Technical Context

**Language/Version**: Go 1.24.4 (existing workspace)
**Primary Dependencies**:
- casbin/casbin/v2 - Policy enforcement engine (wrapped by custom Chi middleware)
- github.com/msales/casbin-bun-adapter - Bun ORM adapter for Casbin policy storage (uses existing *bun.DB)
- github.com/hashicorp/go-bexpr - Boolean expression evaluation for label scope filtering
- github.com/mitchellh/mapstructure - JWT claim extraction (already in dep tree via go-bexpr)
- github.com/zitadel/oidc/v3 - OIDC provider/device authorization toolkit
- github.com/XenitAB/go-oidc-middleware - Chi middleware for token validation (wraps coreos/go-oidc)
- golang.org/x/oauth2 - OAuth2 flows (client credentials, device code)
- go-chi/chi/v5 - HTTP router (existing)
- uptrace/bun - ORM for auth tables (existing)

**Storage**: PostgreSQL (existing) - New tables: roles, user_roles, group_roles, service_accounts, sessions, casbin_rule. Label scope filters stored as go-bexpr expression strings, evaluated in-memory against resource labels.

**Testing**: Go testing framework with testify assertions, contract tests using httptest

**Target Platform**: Linux/macOS server (existing: darwin 24.6.0 noted in spec)

**Project Type**: Go workspace monorepo with Connect RPC + HTTP endpoints

**Performance Goals**:
- Token validation: <10ms (cached JWKS)
- Authorization check: <5ms (Casbin in-memory policy)
- Session lookup: <5ms (database index on token hash)

**Constraints**:
- 12-hour token lifetime (uniform across roles)
- No rate limiting (YAGNI for <500 states scope)
- Preserve existing Terraform HTTP Backend behavior
- No pagination (max 500 states expected)
- **Terraform HTTP Backend Auth Limitation**: Terraform only supports Basic Auth via `TF_HTTP_USERNAME`/`TF_HTTP_PASSWORD` environment variables (no Bearer token support). OpenTofu supports `headers` in backend config but not via env vars. Server-side must extract token from Basic Auth password field (GitHub API pattern: username can be blank, password contains bearer token).

**Scale/Scope**:
- Expected users: <100 humans, ~10 service accounts
- Expected states: <500 (documented in spec edge cases)
- Expected roles: 5-10 custom roles beyond 3 defaults
- Expected policies: ~20 Casbin rules

## Constitution Check
*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

### Principle I: Go Workspace Architecture ✅ PASS
- Uses existing go.work with 5 modules: api, cmd/gridapi, cmd/gridctl, pkg/sdk, tests
- New code lives in cmd/gridapi/internal/auth/ (not a new module)
- Database migrations in cmd/gridapi/internal/migrations/ (existing pattern)
- No new modules introduced

### Principle II: Contract-Centric SDKs ✅ PASS (with noted exceptions)
- Auth RPCs will be added to proto/state/v1/state.proto as admin operations
- Service account management via Connect RPC (e.g., CreateServiceAccount)
- Token introspection via Connect RPC (e.g., GetEffectivePermissions)
- **Exception 1**: Login flow uses HTTP endpoints (not Connect RPC) for OIDC redirect compatibility
  - `/auth/login` - Initiates OIDC flow (browser redirect)
  - `/auth/callback` - OIDC callback handler
  - `/auth/token` - OAuth2 token exchange for service accounts
  - Rationale: OIDC protocol mandates specific HTTP redirect flows incompatible with Connect RPC
- **Exception 2**: Terraform HTTP Backend endpoints (not Connect RPC) for protocol compatibility
  - `/tfstate/{guid}` - GET/POST state data (Terraform HTTP Backend spec)
  - `/tfstate/{guid}/lock` - LOCK endpoint
  - `/tfstate/{guid}/unlock` - UNLOCK endpoint
  - Rationale: Terraform HTTP Backend protocol requires specific HTTP methods and paths
- SDK wrappers will expose ergonomic auth functions backed by generated clients
- **pkg/sdk Note**: The SDK provides helper functions for OIDC flows (LoginWithDeviceCode, LoginWithServiceAccount) that use HTTP endpoints directly, and Terraform wrapper (pkg/sdk/terraform/) for process spawning and token injection. These are exceptions to "depend only on protobuf/Connect RPC". The SDK's auth.go and terraform/ package document these exceptions.

### Principle III: Dependency Flow Discipline ✅ PASS
- cmd/gridapi depends on internal packages (auth, middleware) only
- pkg/sdk will import api (generated Connect clients) for auth RPCs
- pkg/sdk auth OIDC flow helpers use direct HTTP calls (exception for /auth/* endpoints only)
- cmd/gridctl depends on pkg/sdk for auth operations (both RPC and OIDC flow)
- webapp uses generated TypeScript SDK from js/sdk/gen for Connect RPC
- webapp uses direct fetch/axios for OIDC HTTP endpoints (/auth/login, /auth/callback)
- No circular dependencies introduced

### Principle IV: Cross-Language Parity via Connect RPC ⚠️ PARTIAL
- Admin auth operations (role management, user assignment) via Connect RPC
- Token validation and permission checks server-side only (not SDK-exposed)
- Login flows language-specific (Go CLI uses device authorization flow, webapp uses browser)
- Documented exceptions:
  - OIDC login endpoints are HTTP-only (see Principle II Exception 1)
  - Terraform HTTP Backend endpoints are HTTP-only (see Principle II Exception 2)
  - Terraform wrapper (pkg/sdk/terraform/) is Go-only (process spawning, no cross-language equivalent needed)

### Principle V: Test Strategy ✅ PASS
- Contract tests for auth RPCs against proto definitions
- Integration tests with real Keycloak in Docker Compose
- Unit tests for Casbin policy evaluation
- CLI E2E tests with mocked token server
- TDD: All tests written before implementation

### Principle VI: Versioning & Releases ✅ PASS
- Auth RPCs versioned with existing proto/state/v1
- No breaking changes to existing state management APIs
- Database migrations versioned (existing pattern)
- Backward-compatible: Auth optional initially (can be enabled via flag)

### Principle VII: Simplicity & Pragmatism ✅ PASS
- Uses existing router (Chi), ORM (Bun), database (PostgreSQL)
- Casbin chosen over custom RBAC (battle-tested library)
- No new abstraction layers beyond Casbin enforcer wrapper
- YAGNI: No caching beyond Casbin's in-memory policy and JWKS standard cache
- Standard OAuth2 flows (no custom protocols)

**Overall Assessment**: ✅ PASS - No constitutional violations. Auth implementation follows existing patterns and uses proven libraries.

### SDK Architecture Note

**pkg/sdk (Go)**: Principle II allows HTTP auth endpoints as exception. The SDK structure is:
- **`pkg/sdk/credentials.go`**: CredentialStore interface (storage abstraction)
- **`pkg/sdk/auth.go`**: LoginWithDeviceCode (RFC 8628 helper backed by zitadel/oidc endpoints), LoginWithServiceAccount (client credentials) - HTTP-based helpers
- **`pkg/sdk/terraform/`**: Terraform wrapper accepting CredentialStore interface for token injection

**CLI Implementation**: CLI provides concrete CredentialStore (~/.grid/credentials.json), calls pkg/sdk helpers with dependency injection

**js/sdk (TypeScript)**: Generated code (js/sdk/gen/) from buf generate, plus:
- **`js/sdk/auth.ts`**: Browser auth helpers (initLogin, handleCallback, logout) - HTTP fetch to /auth/* endpoints
- **Webapp**: Uses js/sdk/auth for authentication, js/sdk/gen Connect clients for all RPC calls

This pattern keeps auth logic reusable across CLI and webapp while respecting the Constitutional exception for HTTP auth endpoints.

## Project Structure

### Documentation (this feature)
```
specs/006-authz-authn-rbac/
├── plan.md              # This file (/speckit.plan command output)
├── spec.md              # Feature specification (input)
├── research.md          # Phase 0 output (/speckit.plan command)
├── data-model.md        # Phase 1 output (/speckit.plan command)
├── quickstart.md        # Phase 1 output (/speckit.plan command)
├── contracts/           # Phase 1 output (/speckit.plan command)
│   ├── auth-service.proto   # Auth RPC additions to state.proto
│   └── http-endpoints.yaml  # OpenAPI for OIDC HTTP endpoints
└── tasks.md             # Phase 2 output (/speckit.tasks command - NOT created by /speckit.plan)
```

### Source Code (repository root)
```
cmd/gridapi/
├── internal/
│   ├── auth/                    # NEW: Auth subsystem
│   │   ├── actions.go           # Action constants file
│   │   ├── oidc.go              # OIDC provider wiring (zitadel/oidc handlers + config)
│   │   ├── casbin.go            # Casbin enforcer initialization + bexprMatch function
│   │   ├── jwt.go               # go-oidc-middleware configuration + middleware adapters
│   │   ├── claims.go            # JWT claim extraction (ExtractGroups with mapstructure)
│   │   ├── identifiers.go       # Casbin identifier prefix helpers (user:, group:, sa:, role:)
│   │   ├── session.go           # Session management
│   │   └── service_account.go   # Service account credential handling
│   ├── db/                      # Updated: define Bun Models
│   │   └── auth.go              # NEW: Auth table models (users, service_accounts, roles, user_roles, group_roles, sessions, casbin_rule)
│   ├── middleware/              # NEW: Auth middleware
│   │   ├── authn.go             # Authentication middleware (wraps go-oidc-middleware, revocation, Casbin grouping)
│   │   ├── authz.go             # Casbin Chi middleware wrapper
│   │   └── context.go           # Auth context propagation
│   ├── server/
│   │   ├── auth_handlers.go     # NEW: OIDC HTTP endpoints
│   │   ├── connect_handlers.go  # UPDATED: Add auth RPC handlers (incl. group-role RPCs)
│   │   └── tfstate.go           # UPDATED: Apply auth middleware
│   ├── repository/
│   │   ├── auth_repository.go   # NEW: Auth table queries (roles, user_roles, group_roles, etc.)
│   │   └── interface.go         # UPDATED: Add auth repo interface
│   ├── migrations/
│   │   └── YYYYMMDDHHMMSS_auth_schema.go  # NEW: Auth tables migration (incl. group_roles)
│   └── config/
│       └── config.go            # UPDATED: Add OIDC config (groups_claim_field, groups_claim_path)
├── cmd/
│   └── serve.go                 # UPDATED: Initialize auth system
└── casbin/                      # NEW: Casbin model file
    └── model.conf               # RBAC model definition (policies in database)

cmd/gridctl/
├── cmd/
│   ├── login.go                 # NEW: OIDC device code flow
│   ├── logout.go                # NEW: Clear local credentials
│   ├── auth.go                  # NEW: Auth status commands
│   ├── role.go                  # NEW: Role management (assign-group, remove-group, list-groups)
│   └── tf.go                    # NEW: Terraform wrapper command (delegates to pkg/sdk/terraform)
└── internal/
    ├── auth/
    │   ├── device_flow.go       # NEW: Device-code UX helper (code display + polling)
    │   └── credential_store.go  # NEW: Local token storage
    └── context/
        └── reader.go            # EXISTING: .grid directory context file reading (DirCtx)

pkg/sdk/
├── client.go                    # UPDATED: Add auth client methods
├── auth.go                      # NEW: Auth SDK wrapper (OIDC helpers, RPC wrappers)
│                                # Note: OIDC helpers use HTTP endpoints directly (exception)
├── auth_test.go                 # NEW: Auth SDK contract tests
└── terraform/                   # NEW: Terraform wrapper (exception: not using Connect RPC)
    ├── wrapper.go               # Process spawning, token injection, 401 retry
    ├── binary.go                # Binary discovery (--tf-bin, TF_BIN, auto-detect)
    ├── auth.go                  # Token injection via TF_HTTP_PASSWORD
    ├── output.go                # Secret redaction, 401 detection
    └── wrapper_test.go          # Wrapper unit tests

proto/state/v1/
└── state.proto                  # UPDATED: Add admin auth RPCs

api/state/v1/                    # GENERATED: Connect clients/servers
└── (autogenerated files)

tests/
├── integration/
│   └── auth_test.go             # NEW: E2E auth flow tests
└── contract/
    └── auth_contract_test.go    # NEW: Auth RPC contract tests

webapp/                          # EXISTING: React webapp
├── src/
│   ├── context/
│   │   ├── AuthContext.tsx      # NEW: Auth context provider
│   │   └── GridContext.tsx      # EXISTING: Grid data context
│   ├── hooks/
│   │   ├── useAuth.ts           # NEW: Auth hook (login, logout, session)
│   │   ├── usePermissions.ts    # NEW: Permission checking hook
│   │   └── useGridData.ts       # EXISTING: Data fetching hook
│   ├── services/
│   │   ├── authApi.ts           # NEW: Auth HTTP endpoint calls
│   │   └── gridApi.ts           # EXISTING: Connect RPC client
│   ├── components/
│   │   ├── AuthGuard.tsx        # NEW: Route protection component
│   │   ├── LoginCallback.tsx    # NEW: OIDC callback handler
│   │   ├── UserMenu.tsx         # NEW: User identity display
│   │   └── ProtectedAction.tsx  # NEW: Permission-gated button wrapper
│   └── App.tsx                  # UPDATED: Wrap with AuthProvider

docker-compose.yml               # UPDATED: Add Keycloak service
```

**Structure Decision**: Existing Go workspace monorepo structure. Auth implementation lives in cmd/gridapi/internal/auth/ following established internal package pattern. No new modules introduced per Constitutional Principle I. Middleware lives in cmd/gridapi/internal/middleware/ (new directory, not new module). CLI auth code in cmd/gridctl/internal/auth/ (new directory). SDK wrappers in pkg/sdk/ (existing module). **Terraform wrapper** lives in pkg/sdk/terraform/ (process spawning, token injection, 401 retry) - pkg/sdk exception for non-RPC functionality. CLI tf command (cmd/gridctl/cmd/tf.go) delegates to pkg/sdk/terraform, reads .grid context via cmd/gridctl/internal/context. Webapp auth code in webapp/src with new context, hooks, and components following React patterns.

## Phase 0: Outline & Research

**Status**: ✅ COMPLETE

### Research Tasks Completed:

1. **Casbin Integration with Chi Router**
   - Approach: Build bespoke Chi middleware around casbin/v2’s API (AddPolicy/Enforce) to avoid v1/v2 adapter mismatch
   - Pattern: Initialize enforcer → custom middleware extracts subject from context → evaluates `(subject, objectType, action, labels)`
   - Subject extraction: Uses custom resolver function to get user ID from JWT claims

2. **OIDC Provider Setup**
   - Dev: Keycloak 22+ in Docker Compose (official image)
   - Prod: Azure Entra ID (formerly Azure AD)
   - Library: github.com/zitadel/oidc/v3 to host authorization, device, token, discovery, and revocation endpoints
   - Discovery: Auto-fetch .well-known/openid-configuration
   - JWKS: Cached signing keys from jwks_uri

3. **CLI Authentication Flow**
   - Pattern: OIDC Device Authorization Grant (RFC 8628)
   - Library usage: Use github.com/zitadel/oidc/v3 device helpers to start the flow and poll the token endpoint (no custom HTTP server)
   - UX contract: CLI prints verification URI + user code, opens browser when possible, and polls on the documented interval until approved/denied
   - Resilience: Respect retry intervals returned by the provider; surface throttling/errors per RFC 8628

4. **Service Account Authentication**
   - Flow: OAuth2 Client Credentials (RFC 6749 Section 4.4)
   - Keycloak: Client credentials enabled on service client
   - Azure AD: App registration with client secret
   - Token endpoint: POST /token with grant_type=client_credentials

5. **Resource-Server Token Validation**
   - Library: github.com/XenitAB/go-oidc-middleware (wraps coreos/go-oidc)
   - Process: Middleware performs discovery + JWKS caching → validates signature/claims → stores claims in context
   - Grid adds follow-up middleware for session revocation and Casbin dynamic grouping

6. **Casbin Policy Management**
   - Go: Casbin API (e.g., enforcer.AddPolicy)
   - JavaScript: @casbin/casbin (Node.js library)
   - Evaluated: casbin-dashboard (too heavy), custom admin UI (defer to Phase 2)

**Output**: research.md (to be generated in next step)

### Action Taxonomy & Constants

**Decision**: Define Go constants for all actions in spec.md FR-025 taxonomy to prevent typos and enable IDE autocomplete.

**Source**: See `specs/006-authz-authn-rbac/authorization-design.md` (Actions Taxonomy section) for canonical action definitions and role mappings.

**Implementation**: See `cmd/gridapi/internal/auth/actions.go` (to be created during Phase 2 task generation).

**Actions include**:
- Control Plane: `state:{create|read|list|update-labels|delete}`, `dependency:{read|write}`, `policy:{read|write}`
- Data Plane: `tfstate:{read|write|lock|unlock}`
- Admin: `admin:{role-manage|user-assign|group-assign|service-account-manage|session-revoke}`
- Wildcards: `state:*`, `tfstate:*`, `admin:*`, `*:*`

**Validation**: Action strings validated when creating/updating policies via Connect RPC to prevent typos. Casbin itself does not enforce action validation (uses free-form string matching).

**Mapping to Endpoints**: See authorization-design.md for which RPC methods and HTTP routes require which actions.

### Database Seed Setup for Casbin Policies

**Decision**: Seed default Casbin policies directly into `casbin_rule` table via database migration.

**Implementation**: Migration file `cmd/gridapi/internal/migrations/YYYYMMDDHHMMSS_seed_casbin_policies.go`

**Default Policies** (using model: `p = role, objType, act, scopeExpr, eft`):

1. **service-account role**: Data plane access with no label scope constraint
   - Policies: `tfstate:{read|write|lock|unlock}` on `state` with empty scopeExpr

2. **platform-engineer role**: Full wildcard access
   - Policy: `*` on `*` with empty scopeExpr (allow)

3. **product-engineer role**: Label-scoped dev access
   - Policies: `state:{create|read|list|update-labels}`, `tfstate:*`, `dependency:*` on `state` with scopeExpr `env == "dev"`
   - Policy: `policy:read` on `policy` with empty scopeExpr

**Implementation details**: See research.md section 7 for complete seed data code example using go-bexpr expressions.

### Keycloak Development Bootstrap Steps

**Docker Compose Configuration**:

```yaml
# docker-compose.yml (additions)

services:
  postgres:
    image: postgres:17-alpine
    container_name: grid-postgres
    environment:
      POSTGRES_USER: grid
      POSTGRES_PASSWORD: gridpass
      POSTGRES_DB: grid
    ports:
      - "5432:5432"
    volumes:
      - grid-postgres-data:/var/lib/postgresql/data
      - ./initdb:/docker-entrypoint-initdb.d:ro  # Init scripts for keycloak DB
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U grid"]
      interval: 5s
      timeout: 5s
      retries: 5

  keycloak:
    image: quay.io/keycloak/keycloak:22.0
    container_name: grid-keycloak
    environment:
      KEYCLOAK_ADMIN: admin
      KEYCLOAK_ADMIN_PASSWORD: admin
      KC_DB: postgres
      KC_DB_URL: jdbc:postgresql://postgres:5432/keycloak
      KC_DB_USERNAME: keycloak
      KC_DB_PASSWORD: keycloak
      KC_HOSTNAME: localhost
      KC_HOSTNAME_PORT: 8443
      # Local dev configuration (FR-100 Local Development Exception)
      # Plain HTTP allowed for localhost/docker-compose to simplify setup
      # Production MUST use HTTPS/TLS
      KC_HOSTNAME_STRICT_HTTPS: "false"
      KC_HTTP_ENABLED: "true"
    ports:
      - "8443:8080"
    depends_on:
      postgres:
        condition: service_healthy
    command: start-dev  # Development mode - NOT for production
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:8080/health/ready"]
      interval: 10s
      timeout: 5s
      retries: 5

volumes:
  grid-postgres-data:
    driver: local
```

**PostgreSQL Init Script**: `initdb/01-init-keycloak-db.sql`

```sql
-- Create keycloak user and database on first init
-- This script runs once when the postgres container starts with an empty data directory

DO $$
BEGIN
  IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'keycloak') THEN
    CREATE ROLE keycloak LOGIN PASSWORD 'keycloak';
  END IF;
END $$;

-- Create keycloak database (idempotent)
SELECT 'CREATE DATABASE keycloak OWNER keycloak'
WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname = 'keycloak')\gexec
```

**Manual Setup Steps** (after `docker compose up`):

1. **Access Keycloak Admin Console**:
   - URL: http://localhost:8443
   - Username: `admin`
   - Password: `admin`

2. **Create Realm**:
   - Click "Create Realm" in top-left dropdown
   - Realm name: `grid`
   - Click "Create"

3. **Create OIDC Client for API Server**:
   - Navigate to Clients → Create Client
   - Client type: OpenID Connect
   - Client ID: `grid-api`
   - Click "Next"
   - Client authentication: ON (confidential)
   - Authentication flow: Check "Standard flow", "Direct access grants"
   - Click "Next"
   - Valid redirect URIs: `http://localhost:8080/auth/callback`, `http://localhost:3000/callback` (webapp)
   - Web origins: `http://localhost:3000` (for CORS)
   - Click "Save"
   - Go to "Credentials" tab, copy Client Secret

4. **Create Service Account Client (for CI/CD)**:
   - Navigate to Clients → Create Client
   - Client ID: `grid-service-account`
   - Client authentication: ON
   - Authentication flow: Check "Service accounts roles"
   - Click "Save"
   - Go to "Service Account Roles" tab
   - Assign roles (can map to Grid roles later)

5. **Create Test Users**:
   - Navigate to Users → Add User
   - Username: `alice@example.com`
   - Email: `alice@example.com`
   - Email verified: ON
   - Click "Create"
   - Go to "Credentials" tab, set password: `password`, Temporary: OFF
   - Repeat for `bob@example.com` (different persona for testing)

6. **Create Groups (for role mapping)**:
   - Navigate to Groups → Create Group
   - Name: `platform-engineers`
   - Click "Create"
   - Repeat for `product-engineers`
   - Assign users to groups: Users → alice@example.com → Groups → Join Group

7. **Configure Group/Role Claims in Token**:
   - Navigate to Clients → grid-api → Client Scopes → grid-api-dedicated
   - Add Mapper → By Configuration → Group Membership
   - Name: `groups`
   - Token Claim Name: `groups`
   - Full group path: OFF
   - Add to ID token: ON
   - Add to access token: ON
   - Click "Save"

**gridapi Configuration** (after Keycloak setup):

```yaml
# config.yaml or environment variables

oidc:
  issuer: "http://localhost:8443/realms/grid"
  client_id: "grid-api"
  client_secret: "<copied from Keycloak console>"
  redirect_uri: "http://localhost:8080/auth/callback"
  groups_claim_field: "groups"        # Default: extract groups from "groups" claim
  groups_claim_path: ""               # Optional: for nested extraction (e.g., "name" for [{name:"dev"}])
  user_id_claim_field: "sub"          # Default: OIDC standard subject claim
  email_claim_field: "email"          # Default: OIDC standard email claim
```

**Automated Setup Script** (optional, for CI/CD):

Create `scripts/keycloak-setup.sh` using Keycloak Admin REST API to automate realm creation, client registration, and user provisioning. This script would be called after `docker compose up` in CI environments.

**Output**: research.md (to be generated in next step)

## Phase 1: Design & Contracts

**Status**: ✅ COMPLETE

### 1. Data Model (data-model.md)

Entities extracted from spec Key Entities section:

- **User** (maps to OIDC subject)
- **ServiceAccount** (client_id/secret pairs)
- **Role** (bundled permissions + constraints)
- **GroupRole** (group_name → role mappings for SSO integration)
- **RoleAssignment** (user/service-account → role, direct assignments)
- **Session** (token hash → user mapping)
- **CasbinRule** (Casbin policy storage with go-bexpr expressions)

See data-model.md for full schema details.

### 2. API Contracts (contracts/)

**Connect RPC additions** to proto/state/v1/state.proto:
- CreateServiceAccount(name) → (client_id, client_secret)
- ListServiceAccounts() → (service_accounts[])
- RevokeServiceAccount(client_id) → ()
- AssignRole(user_id, role_name) → ()
- RemoveRole(user_id, role_name) → ()
- AssignGroupRole(group_name, role_name) → ()
- RemoveGroupRole(group_name, role_name) → ()
- ListGroupRoles(group_name?) → (group_role_assignments[])
- GetEffectivePermissions(user_id) → (roles[], permissions[], label_scope)
- ListRoles() → (roles[])
- CreateRole(name, permissions, label_scope, create_constraints, immutable_keys) → (role)
- UpdateRole(name, ...) → (role)
- DeleteRole(name) → ()
- ExportRoles(role_names[]?) → (roles_json string) - Export role definitions as JSON (all roles if no filter)
- ImportRoles(roles_json string, overwrite bool) → (imported_count, skipped_count, errors[]) - Import roles from JSON, idempotent

**HTTP endpoints** (non-RPC, OIDC flow):
- GET /auth/login?redirect_uri=... - Initiate OIDC flow
- GET /auth/callback?code=...&state=... - OIDC callback handler
- POST /auth/token - OAuth2 token exchange (service accounts)
- POST /auth/logout - Revoke session

See contracts/ directory for OpenAPI spec.

### 3. Contract Tests

Tests generated in tests/contract/auth_contract_test.go:
- TestCreateServiceAccount_Success
- TestCreateServiceAccount_Unauthorized (no admin role)
- TestAssignRole_ValidRole
- TestAssignRole_NonexistentRole (404)
- TestGetEffectivePermissions_MultipleRoles
- TestCreateRole_ValidDefinition
- TestCreateRole_InvalidActionEnum (validation error)

All tests initially FAIL (no implementation yet).

### 4. Integration Test Scenarios

From user stories in spec.md:
- Story 1 (Product Engineer): Test create state with env=dev, denied with env=prod
- Story 2 (Platform Engineer): Test unlock state locked by another user
- Story 3 (CI/CD Pipeline): Test service account auth + state write
- Story 4 (SSO User): Test OIDC login flow (mocked IdP)
- Story 5 (CLI User): Test device code flow (mocked server)

See quickstart.md for step-by-step validation.

### 5. Agent Context Update

Running update script...

**Output**: data-model.md, contracts/, failing tests, quickstart.md, CLAUDE.md updated

## Phase 2: Task Planning Approach
*This section describes what the /speckit.tasks command will do - DO NOT execute during /speckit.plan*

### Task Generation Strategy

The /speckit.tasks command will generate tasks from Phase 1 artifacts in the following order:

**Foundation Tasks** (database, config, auth setup):
1. Database migration for casbin_rule table (schema + unique index)
   - **Schema**: data-model.md §7 (lines 342-366)
   - **Adapter**: research.md §1 (msales/casbin-bun-adapter pattern)
2. Database migration for auth tables (roles, user_roles, group_roles, service_accounts, sessions)
   - **Role schema**: data-model.md §4 (lines 201-233)
   - **GroupRole schema**: data-model.md §5 (lines 235-265)
   - **UserRole schema**: data-model.md §6 (lines 267-288)
   - **ServiceAccount schema**: data-model.md §2 (lines 141-172)
   - **Session schema**: data-model.md §3 (lines 174-199)
3. Seed migration for default Casbin policies (insert into casbin_rule with go-bexpr expressions)
   - **Seed data**: research.md §8, plan.md §310-323 (default role policies)
   - **go-bexpr syntax**: research.md §1 (bexprMatch function examples)
4. Config struct updates (OIDC issuer, client ID, JWKS URLs, Casbin model path, groups_claim_field, groups_claim_path)
   - **OIDC fields**: research.md §2 (lines 112-165), CLARIFICATIONS.md §1 (lines 72-90)
   - **Claim config**: CLARIFICATIONS.md §1 (lines 72-90), research.md §9 (JWT claim extraction)
5. Casbin model file creation (model.conf with bexprMatch matcher)
   - **Model syntax**: research.md §1 (lines 48-77, includes bexprMatch custom function)
6. Casbin enforcer initialization (msales/casbin-bun-adapter + bexprMatch function registration)
   - **Initialization**: research.md §1 (lines 79-110, enforcer setup + custom function)
   - **Adapter**: research.md §5 (lines 575-580, database-backed pattern)
7. go-bexpr evaluator helper with caching (for bexprMatch)
   - **Implementation**: research.md §1 (lines 79-110, sync.Map caching pattern)
8. Casbin identifier helper functions (auth/identifiers.go with prefix constants: user:, group:, sa:, role:)
   - **Prefix taxonomy**: PREFIX-CONVENTIONS.md §2-3 (lines 15-99)
   - **Helper functions**: PREFIX-CONVENTIONS.md §9 (lines 195-232)

**Auth Core** (token validation, session management):
9. OIDC provider configuration (zitadel/oidc handlers)
   - **Pattern**: research.md §7 (provider wiring + storage adapters)
   - **Device flow**: research.md §7 (device authorization support)
10. go-oidc-middleware verifier setup (Chi middleware)
    - **Validation flow**: research.md §6 (middleware chain, discovery + JWKS caching)
    - **Post-validation hooks**: research.md §6 (revocation + Casbin grouping)
11. JWT claim extraction helper (ExtractGroups function with mapstructure for nested claims)
    - **Flat array extraction**: research.md §9 (lines 619-686, direct array handling)
    - **Nested extraction**: research.md §9 (lines 688-765, mapstructure pattern)
    - **Config**: CLARIFICATIONS.md §1 (groups_claim_field, groups_claim_path)
12. Dynamic Casbin grouping at authentication time (user→group, group→role transient mappings)
    - **Flow**: data-model.md §5 (lines 290-317, permission resolution diagram)
    - **Transient groupings**: CLARIFICATIONS.md §1 (lines 59-67, AddRoleForUser pattern)
    - **Prefix usage**: PREFIX-CONVENTIONS.md §5 (lines 274-288, authentication-time grouping)
13. Session repository (CRUD operations + cascade revocation)
    - **Schema**: data-model.md §3 (lines 174-199)
    - **Hash pattern**: research.md §5 (token_hash for lookup)
    - **Cascade revocation**: RevokeAllSessionsForServiceAccount(service_account_id) - updates sessions.revoked=true for FR-070b
    - **Revocation check**: ValidateSession must check sessions.revoked column for FR-102a
14. Service account repository (CRUD + secret hashing)
    - **Schema**: data-model.md §2 (lines 141-172)
    - **Secret handling**: research.md §4 (lines 221-285, bcrypt hashing)
    - **Rotation behavior (FR-070a)**: Updating client_secret_hash does NOT invalidate existing tokens (JWT tokens are self-contained, validated against JWKS not secret). Update secret_rotated_at timestamp. Document this behavior in repository comments.
15. Group-role repository (CRUD for group_roles table)
    - **Schema**: data-model.md §5 (lines 235-265)
    - **Casbin integration**: CLARIFICATIONS.md §1 (lines 48-68, static group→role mappings)
16. Auth repository interface implementation
    - **Pattern**: Follow existing repository/interface.go pattern in cmd/gridapi/internal/repository/

**Middleware** (request interception):
17. Authentication middleware (wrap go-oidc-middleware + revocation) [P]
    - **Pattern**: research.md §6 (middleware chain, discovery + JWKS caching)
    - **Subject extraction**: research.md §1 (lines 13-46, custom resolver function)
    - **Dynamic grouping**: Call task #12 logic to create transient user→group→role mappings
    - **Revocation check (FR-102a)**: Session repository validation must check sessions.revoked=FALSE (data-model.md §3 line 597)
18. Authorization middleware (custom Casbin Chi middleware + lock-aware bypass) [P]
    - **Integration**: research.md §1 (lines 13-46, custom middleware notes)
    - **Enforcer**: Use enforcer from task #6
    - **Basic Auth extraction**: For /tfstate/* endpoints, extract bearer token from HTTP Basic Auth password field (username ignored, GitHub API pattern per FR-057, FR-097c). Token passed to authentication middleware.
    - **Lock-aware bypass (FR-061a)**: Before enforcing tfstate:write/tfstate:unlock, check if principal holds the lock (compare authenticated principal with state.LockInfo.Who). If lock holder matches, bypass label scope check for these actions only. All other operations: normal authorization.
19. Context propagation (inject user identity into request context) [P]
    - **Pattern**: Standard Go context.WithValue for user identity storage

**HTTP Endpoints** (OIDC flow):
20. Login handler (GET /auth/login - redirect to IdP) [P]
    - **Flow**: research.md §7 (provider routing, auth code handler)
    - **State generation**: Use crypto/rand for CSRF protection
21. Callback handler (GET /auth/callback - exchange code) [P]
    - **Exchange**: research.md §7 (handler hooks for session issuance)
    - **Session creation**: Insert into sessions table (task #13 repository)
22. Token handler (POST /auth/token - service account exchange) [P]
    - **Flow**: research.md §4 (lines 221-285, OAuth2 client credentials)
    - **Validation**: Verify client_id/secret against service_accounts table
23. Logout handler (POST /auth/logout - revoke session) [P]
    - **Revocation**: Delete from sessions table
    - **Pattern**: quickstart.md §5 (session management examples)

**Connect RPC Handlers** (admin operations):
24. CreateServiceAccount RPC implementation [P]
    - **Proto contract**: contracts/auth-rpc-additions.proto
    - **Repository**: Task #14 (service account repository)
    - **Authorization**: Requires admin:service-account-manage (authorization-design.md)
25. ListServiceAccounts RPC implementation [P]
    - **Proto contract**: contracts/auth-rpc-additions.proto
    - **Repository**: Task #14
25a. RevokeServiceAccount RPC implementation [P]
    - **Proto contract**: contracts/auth-rpc-additions.proto (RevokeServiceAccount RPC)
    - **Repository**: Task #14 (set service_accounts.disabled=true)
    - **Cascade revocation (FR-070b)**: Call task #13 RevokeAllSessionsForServiceAccount to invalidate all active sessions
    - **Authorization**: Requires admin:service-account-manage
26. AssignRole RPC implementation [P]
    - **Proto contract**: contracts/auth-rpc-additions.proto
    - **Purpose**: Direct user→role assignments (fallback, see CLARIFICATIONS.md §1)
    - **Casbin sync**: Update casbin_rule with new grouping (g, user:X, role:Y)
27. RemoveRole RPC implementation [P]
    - **Proto contract**: contracts/auth-rpc-additions.proto
    - **Casbin sync**: Remove grouping from casbin_rule
28. AssignGroupRole RPC implementation [P]
    - **Proto contract**: CLARIFICATIONS.md §1 (lines 94-121, new RPC definition)
    - **Primary pattern**: Enterprise SSO group→role mapping
    - **Repository**: Task #15 (group-role repository)
    - **Casbin sync**: Insert into casbin_rule (g, group:X, role:Y)
29. RemoveGroupRole RPC implementation [P]
    - **Proto contract**: CLARIFICATIONS.md §1 (lines 94-121)
    - **Casbin sync**: Delete from casbin_rule
30. ListGroupRoles RPC implementation [P]
    - **Proto contract**: CLARIFICATIONS.md §1 (lines 94-121)
    - **Filter**: Optional group_name parameter
31. GetEffectivePermissions RPC implementation [P]
    - **Proto contract**: contracts/auth-rpc-additions.proto
    - **Logic**: Resolve user→groups→roles→policies (data-model.md §5, permission flow)
32. CreateRole RPC implementation [P]
    - **Proto contract**: contracts/auth-rpc-additions.proto
    - **Schema**: data-model.md §4 (create_constraints, immutable_keys, scope_expr)
    - **Validation**: CLARIFICATIONS.md §2 (create constraints vs label policy)
33. ListRoles RPC implementation [P]
    - **Proto contract**: contracts/auth-rpc-additions.proto
    - **Returns**: All roles with embedded constraints

**CLI Auth** (device flow):
34. Device code flow implementation (library-backed)
    - **Flow**: research.md §3 (lines 167-214, updated for RFC 8628 device authorization)
    - **Handlers**: Use zitadel/oidc provider endpoints (no custom code exchange server)
    - **Client helper**: Rely on zitadel/oidc device client to poll using provider-issued interval
35. Credential store (local token storage)
    - **Pattern**: quickstart.md §2-3 (CLI authentication examples)
    - **Security**: Store in ~/.grid/credentials with 0600 permissions
36. Login command (gridctl login)
    - **Flow**: research.md §3 (initiate device authorization, show verification URI/code, poll until approved)
    - **User stories**: quickstart.md §1 (Product Engineer auth scenario)
37. Logout command (gridctl logout)
    - **Action**: Delete local credentials file, optionally call /auth/logout RPC
38. Auth status command (gridctl auth status)
    - **Display**: User identity, roles, token expiry, effective permissions
    - **RPC**: Call GetEffectivePermissions for current user (task #31)
38a. Role inspect command (gridctl role inspect <principal_id>) [FR-048]
    - **Purpose**: Admin troubleshooting tool to inspect any user/service account/group's effective permissions
    - **Input**: Principal ID (user:alice, sa:ci-pipeline, group:platform-engineers)
    - **RPC**: Call GetEffectivePermissions with specified principal_id
    - **Display**: Principal type, assigned roles, effective permissions, label scope, constraints
    - **Authorization**: Requires admin:role-manage permission
39. Role assign-group command (gridctl role assign-group <group> <role>)
    - **RPC**: Call AssignGroupRole (task #28)
    - **Authorization**: Requires admin:group-assign permission
    - **Purpose**: CLARIFICATIONS.md §1 (primary enterprise SSO pattern)
40. Role remove-group command (gridctl role remove-group <group> <role>)
    - **RPC**: Call RemoveGroupRole (task #29)
41. Role list-groups command (gridctl role list-groups [group])
    - **RPC**: Call ListGroupRoles (task #30)
    - **Output**: Tab-delimited table (group_name, role_name, assigned_at)
42. Role export command (gridctl role export [role_names...] --output=roles.json)
    - **RPC**: Call ExportRoles (optional filter by role names)
    - **Output**: JSON file with role definitions
    - **Authorization**: Requires admin:role-manage permission
43. Role import command (gridctl role import --file=roles.json [--force])
    - **RPC**: Call ImportRoles with file contents
    - **Options**: --force to overwrite existing roles with same name
    - **Output**: Summary of imported/skipped/errored roles
    - **Authorization**: Requires admin:role-manage permission

**CLI Terraform Wrapper** (pkg/sdk + gridctl):
44. Binary discovery and selection (pkg/sdk/terraform/binary.go) [P]
    - **Precedence**: --tf-bin flag → TF_BIN env var → auto-detect (terraform, then tofu)
    - **Validation**: Check binary exists and is executable
    - **Error handling**: Clear message if neither terraform nor tofu found (FR-097b, FR-097h)
45. Process spawner with I/O pass-through (pkg/sdk/terraform/wrapper.go) [P]
    - **STDIO**: Pipe STDIN/STDOUT/STDERR to/from terraform/tofu process unchanged
    - **Exit codes**: Preserve exact exit code from subprocess (FR-097h)
    - **Arguments**: Pass through all args after `--` verbatim (FR-097f)
    - **Working directory**: Run in current directory (no --cwd flag)
46. Token injection via TF_HTTP_PASSWORD (pkg/sdk/terraform/auth.go) [P]
    - **Pattern**: Set TF_HTTP_USERNAME="" and TF_HTTP_PASSWORD=<bearer_token> environment variables (FR-097c)
    - **Credential source**: Use stored credentials from pkg/sdk auth (task #35 credential store)
    - **Validation**: Fail fast if no credentials available in non-interactive mode (FR-097j)
    - **Security**: Never persist token to disk, only pass via env vars (FR-097k)
47. Mid-run 401 detection and retry (pkg/sdk/terraform/output.go) [P]
    - **Detection**: Parse terraform/tofu stderr for "401" or "Unauthorized" strings
    - **Retry logic (FR-097e)**: On 401, attempt single token refresh, re-run exact same command once, then fail
    - **No infinite loops**: Maximum 1 retry attempt
    - **Error message**: Clear auth failure message if retry fails
48. Secret redaction for logs (pkg/sdk/terraform/output.go) [P]
    - **Redaction (FR-097g)**: Mask bearer tokens in all console output, logs, crash reports
    - **Verbose mode**: When --verbose, print command line with tokens redacted (show "[REDACTED]")
    - **Never print**: Bearer token values, TF_HTTP_PASSWORD, Authorization headers
49. CI vs interactive detection (pkg/sdk/terraform/auth.go) [P]
    - **Detection**: Check for TTY, CI env vars (CI=true, GITHUB_ACTIONS, GITLAB_CI, etc.)
    - **Non-interactive (FR-097j)**: Use service account credentials, fail fast if missing
    - **Interactive**: Allow device flow or existing credentials
50. gridctl tf command (cmd/gridctl/cmd/tf.go)
    - **Cobra wiring**: `gridctl tf [flags] -- <terraform args>`
    - **Flags**: --tf-bin (binary override), --verbose (debug output)
    - **Context**: Read .grid file via cmd/gridctl/internal/context (DirCtx) for backend endpoint (FR-097i)
    - **Delegation**: Call pkg/sdk/terraform wrapper with credentials and context
    - **Hint**: If backend.tf missing, print non-blocking suggestion to run `gridctl state init` (FR-097l, CLI-only)
51. Context file integration (.grid reading, CLI-only)
    - **Location**: cmd/gridctl/internal/context (existing DirCtx pattern)
    - **Extract**: GUID, backend endpoints from .grid file
    - **Pass to wrapper**: Provide context to pkg/sdk/terraform for validation
    - **Note**: Context reading is CLI responsibility, not pkg/sdk

**Webapp Auth** (React integration):
52. Auth context provider (AuthContext.tsx) [P]
    - **Pattern**: research.md §14 (lines 885-940, mirror GridContext.tsx structure)
    - **Existing reference**: webapp/src/context/GridContext.tsx (React Context API pattern)
    - **State**: user, loading, error (useState pattern)
53. Auth service for HTTP endpoints (authApi.ts) [P]
    - **Implementation**: research.md §14 (lines 942-974, HTTP fetch with credentials)
    - **OIDC exception**: Uses direct fetch (not Connect RPC) per Constitution Check
    - **Security**: credentials: 'include' for httpOnly cookies
54. useAuth hook (login, logout, session state) [P]
    - **Pattern**: research.md §14 (exported from AuthContext.tsx)
    - **Existing reference**: webapp/src/hooks/useGridData.ts (custom hook pattern)
55. usePermissions hook (permission checking) [P]
    - **Logic**: Call GetEffectivePermissions RPC, cache in state
    - **Helper**: hasPermission(action) function for component use
56. AuthGuard route protection component [P]
    - **Implementation**: research.md §14 (lines 976-997, conditional rendering)
    - **Behavior**: Redirect to login if unauthenticated, preserve return_to parameter
57. LoginCallback component (OIDC callback handler) [P]
    - **Flow**: Parse code/state from URL, call authApi.handleCallback, redirect to return_to
    - **Error handling**: Display error message if callback fails
58. UserMenu component (user identity display) [P]
    - **Display**: User email/name from useAuth, logout button
    - **Existing patterns**: Follow webapp/src/components/ structure
59. ProtectedAction component (permission-gated buttons) [P]
    - **Implementation**: research.md §14 (lines 999-1023, conditional rendering based on permissions)
    - **Note**: Prepared for future use; dashboard is READ ONLY (research.md §14, lines 1061-1073)
60. Update App.tsx to wrap with AuthProvider
    - **Pattern**: Wrap existing GridProvider with AuthProvider (auth wraps grid for dependency order)
    - **Existing file**: webapp/src/App.tsx
61. Add login page/route
    - **Action**: Button to initiate authApi.initLogin(window.location.origin + '/callback')
    - **No routing library**: Use simple conditional rendering or add React Router if needed
62. Handle 401 responses in gridApi.ts (redirect to login)
    - **Implementation**: research.md §14 (lines 1025-1048, Connect interceptor pattern)
    - **Existing file**: webapp/src/services/gridApi.ts (add auth interceptor)

**Integration** (wire everything together):
63. Apply auth middleware to Chi router
    - **Pattern**: research.md §1 (lines 13-46, Chi middleware attachment)
    - **Order**: Authentication (task #17) → Authorization (task #18)
64. Apply auth middleware to Connect RPC handlers
    - **Pattern**: Apply to Connect RPC service registration
    - **Selective**: Some endpoints public (e.g., health check), most require auth
65. Update existing tfstate endpoints with auth middleware
    - **File**: cmd/gridapi/internal/server/tfstate.go
    - **Basic Auth extraction**: Middleware extracts token from HTTP Basic Auth password field (task #18)
    - **Actions**: authorization-design.md (tfstate:read, tfstate:write, tfstate:lock, tfstate:unlock)
    - **Enforcement**: Use enforcer with lock-aware bypass (task #18)
    - **Lock holder check**: Compare authenticated principal with LockInfo.Who for FR-061a
66. Docker Compose Keycloak service and PostgreSQL init script
    - **Config**: plan.md §326-380 (docker-compose.yml additions)
    - **Init script**: plan.md §382-398 (initdb/01-init-keycloak-db.sql)
    - **Manual setup**: plan.md §400-459 (Keycloak realm/client configuration)
67. Create action constants file (actions.go)
    - **Constants**: authorization-design.md (Actions Taxonomy section)
    - **Reference**: plan.md §288-303 (action enumeration)
    - **Location**: cmd/gridapi/internal/auth/actions.go

**Testing** (TDD order):
66. Auth repository unit tests (including group-role repository)
    - **Pattern**: Follow cmd/gridapi/internal/repository/*_test.go patterns
    - **Fixtures**: Real PostgreSQL database, migrations run before tests
67. JWT validation unit tests
    - **Mock JWKS**: Use httptest.Server to mock OIDC provider JWKS endpoint
    - **Test cases**: Valid token, expired token, invalid signature, wrong issuer
68. JWT claim extraction unit tests (flat and nested group claims)
    - **Flat array**: research.md §9 (lines 619-686, ["dev-team", "contractors"])
    - **Nested**: research.md §9 (lines 688-765, [{name: "dev-team"}] with mapstructure)
    - **Edge cases**: Empty groups, nil groups, invalid types
69. Middleware unit tests (httptest)
    - **Pattern**: Use httptest.NewRequest to test middleware in isolation
    - **Scenarios**: No token (401), invalid token (401), valid token (pass through)
    - **Lock-aware bypass**: Test that lock holder bypasses label scope for tfstate:write/unlock
70. Contract tests for auth RPCs (including group-role RPCs)
    - **Pattern**: Follow tests/contract/ patterns
    - **Coverage**: All RPCs from tasks #24-33 + 25a (proto contract verification)
    - **Revocation**: Test RevokeServiceAccount cascades to sessions (FR-070b)
71. Integration tests (Keycloak + real flow with group claims)
    - **Scenarios**: quickstart.md §1-5 (user stories as test cases)
    - **Setup**: Keycloak in docker-compose, real OIDC flow
    - **Group claims**: Test user with multiple groups gets union of permissions
    - **Basic Auth**: Test /tfstate endpoints extract token from HTTP Basic Auth password
72. CLI E2E tests (mocked token server + group management)
    - **Mock**: httptest.Server mocking /auth/login, /auth/callback endpoints
    - **Commands**: Test login, logout, status, role assign-group, list-groups
73. Terraform wrapper unit tests (pkg/sdk/terraform)
    - **Binary discovery**: Test precedence (--tf-bin → TF_BIN → auto-detect)
    - **STDIO pass-through**: Verify output streaming, exit codes preserved
    - **Token injection**: Verify TF_HTTP_PASSWORD set correctly, never logged
    - **401 retry**: Test single retry on 401, then fail
    - **Secret redaction**: Test --verbose masks tokens
74. Terraform wrapper E2E tests
    - **Mock HTTP backend**: httptest.Server mocking /tfstate/* with 401 scenarios
    - **Test commands**: gridctl tf plan, gridctl tf apply with auth
    - **CI mode**: Test non-interactive service account auth
75. Webapp auth flow tests (React Testing Library)
    - **Pattern**: research.md §14 (lines 1075-1090, mock useAuth hook)
    - **Existing tests**: Follow webapp/src/__tests__/dashboard_*.test.tsx patterns
    - **Coverage**: AuthGuard, ProtectedAction, login flow, 401 handling

### Ordering Strategy

- **TDD order**: Tests written before implementation (tasks 66-75 reference earlier tasks)
- **Dependency order**:
  - Database (1-3) → Config (4) → Foundation (5-8) → Core (9-16) → Middleware (17-19) → Endpoints (20-23) → RPC (24-33, 25a) → CLI Auth (34-41) → CLI TF Wrapper (42-49) → Webapp (50-60) → Integration (61-65) → Tests (66-75)
- **Parallel execution markers [P]**:
  - Middleware tasks 17-19 can run in parallel (independent files)
  - HTTP endpoint tasks 20-23 can run in parallel
  - RPC handler tasks 24-33, 25a can run in parallel (all independent)
  - CLI auth command tasks 36-38 can run in parallel (login/logout/status)
  - CLI group commands 39-41 can run in parallel (assign/remove/list)
  - CLI TF wrapper core tasks 42-47 can run in parallel (pkg/sdk/terraform/ files)
  - Webapp component tasks 50-57 can run in parallel

### Estimated Output

**Total tasks**: ~75 numbered, ordered tasks in tasks.md

**Breakdown**:
- Foundation: 8 tasks (includes casbin_rule migration, auth tables with group_roles, seed policies with bexpr, config with OIDC claim fields, model.conf, enforcer init with bexprMatch, bexpr helper, identifier prefix helpers)
- Auth Core: 8 tasks (includes JWT claim extraction with mapstructure, dynamic Casbin grouping, group-role repository, cascade revocation)
- Middleware: 3 tasks (includes lock-aware bypass, Basic Auth extraction - all parallel)
- HTTP Endpoints: 4 tasks (all parallel)
- RPC Handlers: 11 tasks (includes 3 group-role RPCs + RevokeServiceAccount with cascade - all parallel)
- CLI Auth: 8 tasks (includes 3 group management commands, 6 parallel)
- CLI TF Wrapper: 8 tasks (pkg/sdk/terraform + gridctl tf command, 6 parallel)
- Webapp: 11 tasks (8 parallel)
- Integration: 5 tasks (includes Docker Compose init script, action constants, tfstate Basic Auth middleware)
- Testing: 10 tasks (includes group-role, claim extraction, lock bypass, revocation cascade, TF wrapper unit + E2E tests)

**Simplified vs Original Approach**:
- Removed: scope intersection logic, permission definition JSONB deserialization
- Simplified: Single bexprMatch function handles all label filtering
- Reduced complexity: Union (OR) semantics built into Casbin (no custom logic)

**Group-to-Role Mapping Additions**:
- New table: group_roles (group_name → role_id mappings)
- Dynamic grouping: user→group (JWT claims) → role (database) resolution
- Type-only UI: Admins type group names (no IdP browsing integration)
- Claim extraction: Supports flat arrays and nested objects via mapstructure
- Configurable: groups_claim_field (default: "groups"), groups_claim_path (optional for nested extraction)

**Terraform HTTP Backend Auth Additions**:
- Server-side Basic Auth extraction: Password field contains bearer token (username ignored, GitHub API pattern)
- Client-side token injection: pkg/sdk/terraform wrapper sets TF_HTTP_PASSWORD environment variable
- Lock-aware authorization bypass: Lock holders retain tfstate:write/unlock even if labels change (FR-061a)
- gridctl tf wrapper: Process spawning, STDIO pass-through, 401 retry, secret redaction, CI/interactive detection
- No persistent tokens: All credentials passed via environment variables, never written to disk

**Service Account Revocation Additions**:
- Cascade revocation: RevokeServiceAccount invalidates all active sessions (FR-070b)
- Revocation state: sessions.revoked column checked on every request (FR-102a)
- Rotation behavior: Credential rotation does NOT invalidate existing tokens (FR-070a)

**IMPORTANT**: This phase is executed by the /speckit.tasks command, NOT by /speckit.plan

## Phase 3+: Future Implementation
*These phases are beyond the scope of the /speckit.plan command*

**Phase 3**: Task execution (/speckit.tasks command creates tasks.md)

**Phase 4**: Implementation following constitutional principles:
- Use existing Go workspace patterns (internal packages)
- Generate Connect RPC code via `buf generate`
- Write contract tests first (TDD)
- No new modules introduced
- Follow existing migration pattern for database changes

**Phase 5**: Validation
- Run `go test ./... -v` (all tests pass)
- Execute quickstart.md scenarios manually
- Verify Keycloak Docker Compose setup
- Test CLI login flow end-to-end
- Performance validation: Token validation <10ms, authz check <5ms

## Complexity Tracking
*Fill ONLY if Constitution Check has violations that must be justified*

No violations - table empty.

## Progress Tracking
*This checklist is updated during execution flow*

**Phase Status**:
- [x] Phase 0: Research complete (/speckit.plan command)
- [x] Phase 1: Design complete (/speckit.plan command)
- [x] Phase 2: Task planning complete (/speckit.plan command - describe approach only)
- [ ] Phase 3: Tasks generated (/speckit.tasks command)
- [ ] Phase 4: Implementation complete
- [ ] Phase 5: Validation passed

**Gate Status**:
- [x] Initial Constitution Check: PASS
- [x] Post-Design Constitution Check: PASS
- [x] All NEEDS CLARIFICATION resolved
- [x] Complexity deviations documented (none)

---
*Based on Constitution v2.0.0 - See `.specify/memory/constitution.md`*
