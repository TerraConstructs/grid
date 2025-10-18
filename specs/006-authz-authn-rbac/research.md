# Research: Authentication, Authorization, and RBAC

**Feature**: 006-authz-authn-rbac
**Date**: 2025-10-11
**Status**: Complete

## Overview

This document consolidates research findings for implementing authentication and authorization in Grid using industry-standard libraries and protocols.

---

## 1. Casbin Integration with Chi Router

### Decision
Implement a bespoke Chi middleware around **github.com/casbin/casbin/v2** (backed by **github.com/msales/casbin-bun-adapter**) instead of using **github.com/casbin/chi-authz**.

### Rationale
- chi-authz depends on the legacy casbin v1 API; mixing it with casbin/v2 introduces type mismatches and prevents use of SyncedEnforcer.
- Custom middleware keeps the request signature aligned with Grid’s needs (`subject`, `objectType`, `action`, `labels`) without extra adapters.
- Direct control makes it easy to add lock-aware bypass logic (FR-061a) and structured logging.
- msales/casbin-bun-adapter continues to share the existing *bun.DB connection pool, so only the middleware layer changes.
- Still benefits from Casbin’s in-memory evaluation (<1ms) and union (OR) semantics.

### Implementation Pattern
```go
import (
    "net/http"

    "github.com/casbin/casbin/v2"
    casbinbunadapter "github.com/msales/casbin-bun-adapter"
)

func NewAuthorizer(enforcer casbin.IEnforcer, subjectFn func(*http.Request) string, labelsFn func(*http.Request) (map[string]any, error)) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            sub := subjectFn(r)
            if sub == "" {
                http.Error(w, "unauthenticated", http.StatusUnauthorized)
                return
            }

            labels, err := labelsFn(r)
            if err != nil {
                http.Error(w, "authorization lookup failed", http.StatusInternalServerError)
                return
            }

            objType, act := resolveRequest(r)
            ok, err := enforcer.Enforce(sub, objType, act, labels)
            if err != nil {
                http.Error(w, "authorization error", http.StatusInternalServerError)
                return
            }
            if !ok {
                http.Error(w, "forbidden", http.StatusForbidden)
                return
            }
            next.ServeHTTP(w, r)
        })
    }
}

// Adapter + enforcer initialisation
adapter, _ := casbinbunadapter.NewAdapter(bunDB)
enforcer, _ := casbin.NewSyncedEnforcer("model.conf", adapter)
enforcer.AddFunction("bexprMatch", bexprMatch)
_ = enforcer.LoadPolicy()
```

model.conf Example

The RBAC model definition uses go-bexpr for label scope evaluation:

```
# cmd/gridapi/casbin/model.conf

[request_definition]
r = sub, objType, act, labels

[policy_definition]
p = role, objType, act, scopeExpr, eft

[role_definition]
g = _, _

[policy_effect]
e = some(where (p.eft == allow)) && !some(where (p.eft == deny))

[matchers]
m = g(r.sub, p.role) && r.objType == p.objType && r.act == p.act && bexprMatch(p.scopeExpr, r.labels)
```

Explanation:
- **r = sub, objType, act, labels**: Request includes subject (user ID), object type (e.g., "state"), action (e.g., "read"), and resource labels (map[string]any)
- **p = role, objType, act, scopeExpr, eft**: Policy includes role name, object type, action, scope expression (go-bexpr string), and effect (allow/deny)
- **g(r.sub, p.role)**: User-to-role mapping (RBAC)
- **bexprMatch(p.scopeExpr, r.labels)**: Custom function that evaluates go-bexpr expression against resource labels
- Empty scopeExpr treated as "true" (no constraint)
- **Union semantics**: Multiple roles are ORed - any matching role grants access

### Key Considerations
- Subject must be injected into request context by authentication middleware first
- Chi-authz middleware runs after authentication, before business logic
- Enforcer policies stored in database (casbin_rule table) via msales/casbin-bun-adapter
- Default deny: No policy match → 403 Forbidden
- Union (OR) semantics: Any role granting access is sufficient (Casbin's default)
- go-bexpr expressions cached per expression string (compile once, evaluate many times)

### Alternatives Considered
- Custom middleware: Rejected (reinventing wheel, maintenance burden)
- Open Policy Agent (OPA): Rejected (heavier, external service required)
- Oso (Polar): Rejected (less mature Go support than Casbin)

---

## 2. Deployment Mode Architecture

### Decision: Grid Supports Two Mutually Exclusive Authentication Modes

A deployment must choose **exactly ONE** mode. Hybrid mode is NOT supported.

#### Mode 1: External IdP Only (Recommended)
**Principle**: Grid is a **Resource Server** - validates tokens, never issues them.

**Configuration**:
- Environment: `EXTERNAL_IDP_ISSUER`, `EXTERNAL_IDP_CLIENT_ID`, `EXTERNAL_IDP_CLIENT_SECRET`, `EXTERNAL_IDP_REDIRECT_URI`
- `OIDC_ISSUER` must be empty

**Libraries**:
- `golang.org/x/oauth2` (authorization code flow)
- `github.com/coreos/go-oidc/v3` (ID token verification)

**Token Flow**:
- **Human SSO (Web/CLI)**: User authenticates at external IdP → IdP issues JWT (`iss` = IdP URL) → Grid validates via JWKS
- **Service Accounts**: Created as **IdP clients** (not Grid entities) → machine calls IdP's `/token` with client credentials → IdP issues JWT → Grid validates

**Session Creation**: On first API request after token validation (in authentication middleware)

**Implementation**:
- Custom handlers: `/auth/sso/login` (redirect to IdP), `/auth/sso/callback` (receive tokens)
- No Grid signing keys, no Grid OIDC provider endpoints
- Single verifier configured for `ExternalIdP.Issuer`

**External IdP Options**:
- **Development**: Keycloak 22+ in Docker Compose (open-source, self-hosted)
- **Production**: Azure Entra ID, Okta, Google Workspace, Auth0, or any OIDC-compliant provider

**Benefits**:
- Simplest deployment (no key management, no token issuance logic)
- Offloads all identity concerns to enterprise IdP
- Standard OAuth2/OIDC flows
- Service accounts managed in familiar IdP admin interface

**Tradeoffs**:
- Requires external IdP (cannot run fully air-gapped)
- Depends on IdP availability

#### Mode 2: Internal IdP Only (Air-Gapped)
**Principle**: Grid is a **self-contained IdP** - issues and validates its own tokens.

**Configuration**:
- Environment: `OIDC_ISSUER` set to Grid's URL (e.g., `https://grid.example.com`)
- `EXTERNAL_IDP_*` must be empty

**Library**: `github.com/zitadel/oidc/v3/pkg/op` (provider package)

**Token Flow**:
- **Service Accounts**: Created in Grid → machine calls Grid's `/token` with client credentials → Grid issues JWT (`iss` = Grid URL)
- **Human Users**: Grid manages credentials (username/password) → authentication via Grid's login forms → Grid issues tokens

**Session Creation**: At issuance time in `CreateAccessToken()` callback (oidc.go:194)

**Implementation**:
- Grid issues JWT tokens signed with its own RSA 2048 private keys
- Auto-mounted endpoints via `op.CreateRouter()`:
  - `/device_authorization` - CLI initiates device code flow (RFC 8628)
  - `/token` - Token endpoint (device_code, client_credentials, refresh_token grants)
  - `/keys` - Grid's JWKS for validating Grid-issued tokens
  - `/.well-known/openid-configuration` - Discovery for Grid-as-provider
- Custom handler: `/auth/device/verify` (user approval UI for device flow)
- Single verifier configured for `OIDC.Issuer`

**Benefits**:
- Full autonomy (no external dependencies)
- Works in air-gapped/offline environments
- Complete control over token lifecycle

**Tradeoffs**:
- Operational burden: key rotation, user credential management, login UI
- Grid must implement authentication flows (password reset, MFA, etc.)

### Token Validation Strategy

Authentication middleware uses **single-issuer validation** based on deployment mode:

**Mode Detection**:
```go
if cfg.OIDC.ExternalIdP != nil {
    // Mode 1: External IdP Only
    verifier, err = NewExternalIDPVerifier(cfg.OIDC.ExternalIdP)
} else if cfg.OIDC.Issuer != "" {
    // Mode 2: Internal IdP Only
    verifier, err = NewInternalIDPVerifier(cfg.OIDC.Issuer)
} else {
    // Auth disabled (development mode)
}
```

**Key Insight**: No dynamic issuer selection needed - each deployment validates tokens from **one trusted issuer** only.

### Session Persistence by Mode

**Mode 1 (External IdP)**:
- Session created **on first authenticated request** (in authentication middleware)
- Token hash extracted from validated JWT
- User record created/updated based on IdP claims (sub, email, name)
- Subsequent requests reuse existing session

**Mode 2 (Internal IdP)**:
- Session created **at token issuance** in `persistSession()` callback (oidc.go:784)
- Token hash stored immediately for revocation support
- User/service account lookup happens during issuance

Both strategies support immediate revocation via `sessions.revoked` column (FR-007/FR-102a)

### Discovery & JWKS

Both providers expose `/.well-known/openid-configuration`:
```json
{
  "issuer": "https://keycloak.example.com/realms/grid",
  "authorization_endpoint": "https://keycloak.example.com/realms/grid/protocol/openid-connect/auth",
  "token_endpoint": "https://keycloak.example.com/realms/grid/protocol/openid-connect/token",
  "jwks_uri": "https://keycloak.example.com/realms/grid/protocol/openid-connect/certs"
}
```

JWKS (JSON Web Key Set) provides public keys for JWT signature verification:
- Fetched at startup and cached
- Refreshed on key ID (kid) mismatch
- Standard caching headers respected (Cache-Control)

### Configuration
```yaml
# config.yaml
oidc:
  issuer: "https://keycloak.local:8443/realms/grid"  # Dev
  # issuer: "https://login.microsoftonline.com/{tenant_id}/v2.0"  # Prod
  client_id: "grid-api"
  client_secret: "${OIDC_CLIENT_SECRET}"  # From env var
  redirect_uri: "http://localhost:8080/auth/callback"
```

### Alternatives Considered
- Auth0: Rejected (SaaS cost, not needed for current scale)
- Okta: Rejected (same as Auth0)
- Self-rolled JWT: Rejected (security risk, no SSO)

---

## 3. CLI Authentication Flow

### Decision
Adopt the **OIDC Device Authorization Grant** (RFC 8628) as the primary CLI authentication mechanism, using zitadel/oidc helpers for both provider endpoints and client polling.

### Rationale
- Works uniformly across headless environments (SSH sessions, CI runners) and desktops
- Avoids local callback server/network gymnastics; only needs outbound HTTPS
- zitadel/oidc already provides `op.DeviceAuthorizationHandler` / `op.DeviceTokenHandler` and compatible client helpers, reducing custom code
- Aligns with FR-003a requirement to rely on maintained OIDC libraries instead of hand-rolled implementations
- Keeps CLI UX consistent with `gridctl tf` non-interactive usage (same credential store)

### Flow Steps
1. CLI invokes `/auth/device/code` via zitadel/oidc client to obtain `device_code`, `user_code`, verification URI, and polling interval
2. CLI displays the verification URI + user code (and attempts to open the browser when possible)
3. User visits the IdP-hosted verification page, enters the code, and completes authentication (Keycloak in dev, Azure Entra ID in production). The approval UI is delivered entirely by the identity provider, so no Grid-specific web surface is required (see Keycloak device grant docs, server_admin/index.html §“Auth 2.0 Device Authorization Grant”).
4. CLI polls `/auth/device/token` at the provider-specified interval
5. On approval, the CLI receives access/refresh tokens (or an error such as `slow_down`, `authorization_pending`, `access_denied`)
6. Tokens are persisted via the shared `CredentialStore` (`~/.grid/credentials.json` with mode 0600)

### Implementation Libraries
- **github.com/zitadel/oidc/v3/pkg/op**: Device authorization + token handlers mounted in `cmd/gridapi/internal/auth/oidc.go`
- **github.com/zitadel/oidc/v3/pkg/client**: Device client utilities for polling/token exchange
- **golang.org/x/oauth2**: Shared structures for token metadata (already in dependency tree)

### Error Handling
- Respect `slow_down` by backing off before next poll
- Abort after provider-reported expiry (`expires_in`) and prompt the user to restart login
- Surface `access_denied` and other error codes with actionable CLI messaging

### Alternatives Considered
- PKCE + loopback redirect (RFC 8252): Deferred for now; more ergonomic on desktops but adds callback server complexity. Documented as future enhancement below.
- Embedded browser: Rejected (platform-specific, security and maintenance concerns)
- Copy/paste bearer tokens: Rejected (poor UX, security exposure)

### Deferred Improvement: PKCE Loopback
The libraries in use (zitadel/oidc and go-oidc-middleware) already support authorization code + PKCE flows. Once the device grant is validated in production, we can layer a loopback-based PKCE experience on top for users who prefer seamless browser redirects. Tracking issue TBD; no changes are required in the current milestone.

- **Library hooks**: `pkg/client/rp/relying_party.go` exposes `WithPKCE` / `WithPKCEFromDiscovery` options plus `GenerateAndStoreCodeChallenge` helpers, making it straightforward to enable PKCE when instantiating a relying party. The CLI can reuse these helpers to spin up a loopback server while still persisting credentials through the shared `CredentialStore`.
- **Integration sketch**: create a secondary auth path that constructs a `rp.RelyingParty` with PKCE enabled, launch `AuthURLHandler` to open the browser (see `pkg/client/rp/cli/browser.go`), and exchange the code via zitadel/oidc’s existing token helpers. Device flow remains the default; PKCE becomes an opt-in enhancement for rich desktop users.

---

## 4. Service Account Authentication

### Decision
Use **OAuth2 Client Credentials Flow** (RFC 6749 Section 4.4).

### Rationale
- Standard flow for machine-to-machine authentication
- No user interaction required
- Tokens short-lived (refresh on expiry)
- Both Keycloak and Azure AD natively support it

### Flow
```
POST /token HTTP/1.1
Host: keycloak.example.com
Content-Type: application/x-www-form-urlencoded

grant_type=client_credentials
&client_id=grid-service-account
&client_secret=<secret>
&scope=openid profile

Response:
{
  "access_token": "eyJhbGc...",
  "token_type": "Bearer",
  "expires_in": 3600
}
```

### Implementation
```go
import "golang.org/x/oauth2/clientcredentials"

config := clientcredentials.Config{
    ClientID:     serviceAccount.ClientID,
    ClientSecret: serviceAccount.ClientSecret,
    TokenURL:     oidcConfig.TokenEndpoint,
    Scopes:       []string{"openid", "profile"},
}
token, err := config.Token(ctx)
```

### Service Account Creation
- Admin creates service account via Connect RPC: `CreateServiceAccount(name)`
- Returns client_id (UUIDv4) + client_secret (random 32-byte hex)
- Secret hashed (bcrypt, cost 10) before database storage
- Client registers with IdP (Keycloak API or Azure portal)

### Keycloak Setup
1. Create client: Type=confidential, Enable service accounts
2. Set client credentials: Standard flow enabled
3. Map roles: Service Accounts Roles tab

### Azure AD Setup
1. App registration: Create new app
2. Certificates & secrets: New client secret
3. API permissions: Grant application permissions (not delegated)
4. Enterprise applications: Assign roles

### Alternatives Considered
- API keys: Rejected (no standard, manual rotation)
- Mutual TLS: Rejected (complex cert management)
- JWT assertion: Rejected (requires custom signing logic)

---

## 5. Data Persistence, Scale, and Constraints

### Tables
- `roles`: Describes RBAC roles (`name`, `description`, `scope_expr`, `create_constraints`, `immutable_keys`) with embedded constraints as JSONB.
- `user_roles`: Direct user-to-role assignments (`user_id`, `role_id`) - fallback for users without group-based assignments.
- `group_roles`: Group-to-role mappings (`group_name`, `role_id`) - primary enterprise SSO pattern where IdP manages group membership.
- `service_accounts`: Stores machine principals (`client_id`, `secret_hash`, `display_name`, `rotated_at`) using bcrypt for secrets.
- `sessions`: Tracks active CLI/browser sessions (`session_id`, `user_id`, `token_hash`, `issued_at`, `expires_at`) to enable manual revocation.
- `casbin_rule`: Backing store for casbin-bun adapter (msales/casbin-bun-adapter) with columns (`ptype`, `v0`-`v5`) encoding Casbin policies. Label filtering uses go-bexpr expressions in v3 column (evaluated via custom bexprMatch function).

### Seed Data
- `service-account`: Programmatic access role; permissions restricted to the service account's label scope.
- `platform-engineer`: Full CRUD across state, dependency, policy, and auth resources.
- `product-engineer`: Read-only state/dependency access plus ability to queue Terraform runs within assigned labels.

### Constraints & Scale
- No pagination in this release; scope capped at 500 states and ~20 Casbin policies per plan.md.
- Expected identities: <100 humans, ~10 service accounts. Indexes on `sessions.token_hash` and `user_roles.identity_id` keep lookups under 5ms.
- Token lifetime fixed at 12 hours across roles; refresh handled exclusively by the IdP.
- No rate limiting (explicit YAGNI callout); monitoring will flag abuse scenarios.
- Label-scoped enforcement baked into Casbin attributes (e.g., `state:env=dev`), enabling in-memory evaluation without per-request DB hits.
- JSONB payloads (role scopes, service account scopes, per-assignment overrides) are read and evaluated inside the API server; SQLite’s JSON1 support is sufficient if we ever port because the repositories already handle deserialization.

---

## 6. Resource-Server Token Validation

### Decision
Use **github.com/XenitAB/go-oidc-middleware** to protect Chi routes, leveraging its embedded `github.com/coreos/go-oidc/v3/oidc` verifier.

### Rationale
- Drop-in Chi middleware: `verifier.VerifyToken()` extracts bearer tokens, validates them, and injects claims into request context.
- Handles discovery metadata and JWKS caching automatically, removing bespoke HTTP clients and retry logic.
- Provides hook points for custom logic (session revocation, group resolution) that we can wire immediately after validation.
- Keeps our code focused on revocation (FR-007) and Casbin policy enforcement (FR-011–FR-024), satisfying FR-006a.

### Evaluation: go-oidc-middleware vs hand-rolled go-oidc
| Dimension | go-oidc-middleware | Manual go-oidc wiring | Impact |
|-----------|--------------------|-----------------------|--------|
| Discovery & JWKS caching | Built-in, background refreshed | Must implement HTTP client, retries, caching | Reduces error-prone plumbing; mitigates FR-006 token validation risks |
| Token extraction | Handles `Authorization` header parsing, bearer vs token type errors | Custom parsing per handler | Ensures consistent 401 responses (FR-040) |
| Middleware ergonomics | Chi-style `VerifyToken()` + skipper; claims stored in context | Need to build middleware scaffolding | Faster adoption, less code to audit |
| Error handling | Returns typed errors mapped to HTTP 401/403, supports logging hooks | Custom error taxonomy | Aligns with FR-023 clear denial messaging |
| Extension points | Exposes `WithErrorHandler`, `WithAccessor`, `ClaimsFromContext` for downstream use | Need to design context propagation and hooks | Simplifies revocation + Casbin layering (FR-007, FR-038) |
| Maintenance | Upstream maintained; consumes latest go-oidc fixes | Grid must track upstream API changes | Lower ongoing cost |
| Footprint | Small wrapper, no additional transitive deps beyond go-oidc | Same base deps | No bloat introduced |

**Cons / Trade-offs**
- Middleware opinionates on context keys → when requirements change we conform to its types. Mitigation: wrap access in thin helper (`type ClaimsMap map[string]any`).
- Library lifecycle tied to XenitAB upstream; if abandoned we can drop-in our own middleware using identical underlying go-oidc verifier. Plan: pin version, monitor releases.
- Slightly less visibility into internals; however we can enable debug logging via provided options.

### Functional Requirement Alignment & Hook Usage
- **FR-006 / FR-006a**: Middleware guarantees signature/claim validation using discovery & JWKS; we keep enforcement code minimal.
- **FR-007 / FR-102a**: Immediately after `VerifyToken()`, insert `checkRevocation` middleware that reads custom `session_id` claim and consults `sessions` repository, returning 401/403 as required.
- **FR-011–FR-024 / FR-038–FR-039**: `applyCasbinGroups` middleware converts claims to principal/group sets and feeds Casbin before handlers execute.
- **FR-057–FR-061**: Shared middleware stack attaches to both Connect RPC and `/tfstate` routes, ensuring consistent 401/403 semantics.
- **FR-057–FR-059 (Terraform Basic Auth quirk)**: go-oidc-middleware expects a bearer token in the Authorization header; for Terraform HTTP backend we first run a shim that converts the Basic Auth password into a `Bearer <token>` header before the middleware executes, allowing tfstate requests to reuse the same verifier without library changes.
- **FR-098–FR-099**: Use middleware error hooks (`WithErrorHandler`) to emit structured audit logs on validation failures or authorization denials.

### Integration Pattern
```go
import (
    oidcmiddleware "github.com/XenitAB/go-oidc-middleware"
)

verifier := oidcmiddleware.NewVerifier(
    cfg.OIDC.Issuer,
    oidcmiddleware.WithClientID(cfg.OIDC.Audience),
    oidcmiddleware.WithSkipper(func(r *http.Request) bool {
        return strings.HasPrefix(r.URL.Path, "/health") || strings.HasPrefix(r.URL.Path, "/auth/")
    }),
)

r.Use(verifier.VerifyToken())
r.Use(checkRevocation(sessionsRepo))
r.Use(applyCasbinGroups(enforcer, ExtractGroups))
```

### Claims Access
- Middleware stores parsed claims in context: `claims, _ := oidcmiddleware.ClaimsFromContext(r.Context())`.
- `ExtractGroups` operates on the claims map to derive `[]string` for Casbin grouping using `github.com/mitchellh/mapstructure` to handle heterogeneous claim shapes (flat arrays, nested objects).
- Session metadata (e.g., session ID hash) should be added as custom claim so revocation middleware can look up `sessions` table rows directly.

### Functionality Overlap
- Both go-oidc-middleware and manual go-oidc wiring rely on the same verifier (`oidc.NewProvider(...).Verifier(...)`). Selecting the middleware avoids duplicating:
  - HTTP round-tripping for discovery documents
  - JWKS caching implementation
  - Request-scoped claim storage and context key management
  - Token-type error semantics and WWW-Authenticate headers
- If future requirements demand bespoke behavior, we can unwrap the embedded verifier (`middleware.Provider().Verifier(...)`) and extend selectively without re-implementing the full stack.

### Alternatives Considered
- Manual go-oidc wiring: More boilerplate (token extraction, error handling, context storage).
- dexidp/oidc: Archived.
- govalidator-based JWT parsing: Lacks discovery/JWKS support; security risk.

---

## 7. Provider-Side Implementation with `zitadel/oidc`

### Decision
Adopt `github.com/zitadel/oidc/v3` for hosting OpenID Provider endpoints (authorization, device, token, revocation, discovery) so Grid avoids reimplementing protocol-critical flows.

### Rationale
- Exposes `op.NewOpenIDProvider` that mounts handlers on Chi, fitting our routing layer.
- Includes RFC 8628 device authorization flow with sample CLI application—covers FR-003/FR-003a without bespoke polling.
- Manages discovery metadata (`/.well-known/openid-configuration`) and JWKS publishing automatically.
- Modular storage interfaces let us back sessions, device codes, and approvals with existing PostgreSQL repositories.

### Integration Checklist
1. Implement `op.Storage` adapters backed by our auth repositories (clients, users, sessions, device codes).
2. Configure signing keys (ed25519 or RSA). For dev we can reuse Keycloak-style static keys; production should load from secure key vault.
3. Wrap handlers to:
   - Persist Grid session rows on successful token issuance.
   - Attach HTTP-only cookies or return bearer tokens per spec FR-005.
   - Emit audit logs (FR-098/FR-099).
4. Expose device approval UI/API leveraging existing admin endpoints for service account approvals if required.

### Coordination with External IdPs
- When delegating to upstream IdPs (Keycloak/Azure AD), `zitadel/oidc` can operate in "federation" mode by implementing the respective upstream authentication in the Storage/Handler hooks, keeping Grid as the relying party while still benefiting from shared device/token endpoints.

### Risks / Mitigations
- **Storage Complexity**: Library expects certain tables (clients, users). Mitigate by mapping to our domain models and documenting schema mapping.
- **Session Duplication**: Ensure we do not double-store sessions (library + Grid). Consolidate into our `sessions` table via adapters.
- **Version Drift**: Pin module version and add Renovate/Dependabot rules.
- **Key Management**: Add a TODO to tasks to cover development key generation commands (ed25519/RSA), local config docs, and instructions for production deployments to load keys from secure vault/secrets manager.

---

---

## 8. Casbin Policy Management

### Decision
- **Server-side**: Casbin Go API (`enforcer.AddPolicy`, `enforcer.RemovePolicy`)
- **Admin UI**: JavaScript (@casbin/casbin) - defer to future iteration
- **Storage**: Database via **github.com/msales/casbin-bun-adapter**

### Rationale
- No external dashboard service needed (YAGNI)
- Admin operations via Connect RPC (CreateRole, AssignRole)
- Casbin Go API is synchronous and immediate (no lag)
- msales adapter accepts existing *bun.DB instance (shares connection pool, supports transactions)
- Database-backed policies enable runtime updates without server restart
- Single source of truth (no file sync issues)
- Follows Grid's existing Bun ORM patterns
- Union (OR) semantics built into Casbin - no custom logic needed

### Database Adapter: msales/casbin-bun-adapter

**Library**: `github.com/msales/casbin-bun-adapter`

**Key Advantages**:
- Constructor: `NewAdapter(db *bun.DB)` - inject existing Bun DB instance
- Supports FilteredAdapter interface for `LoadFilteredPolicy`
- Fewer dependencies (only casbin/casbin and uptrace/bun)
- Uses standard `casbin_rule` table name (Casbin convention)

**Table Structure**:

```golang
// Standard Casbin storage format
// https://casbin.org/docs/policy-storage#database-storage-format
type CasbinRule struct {
	bun.BaseModel `bun:"casbin_rule,alias:cr"`
	ID            int64  `bun:"id,pk,autoincrement"`
	PType         string `bun:"ptype,type:varchar(100),notnull"`  // Policy type: 'p' (policy), 'g' (grouping/role)
	V0            string `bun:"v0,type:varchar(100)"`  // Role name (for policies) or user ID (for groupings)
	V1            string `bun:"v1,type:varchar(100)"`  // Object type (e.g., "state", "tfstate")
	V2            string `bun:"v2,type:varchar(100)"`  // Action (e.g., "read", "write")
	V3            string `bun:"v3,type:varchar(100)"`  // Scope expression (go-bexpr string)
	V4            string `bun:"v4,type:varchar(100)"`  // Effect (allow/deny)
	V5            string `bun:"v5,type:varchar(100)"`  // Reserved
}
```

```sql
-- Recommended unique index for deduplication
CREATE UNIQUE INDEX idx_casbin_rule_unique
ON casbin_rule (ptype, v0, v1, v2, v3, v4, v5);
```

**Initialization Pattern**:
```go
import (
    casbinbunadapter "github.com/msales/casbin-bun-adapter"
    "github.com/casbin/casbin/v2"
    "github.com/uptrace/bun"
    "github.com/hashicorp/go-bexpr"
    "sync"
)

// Initialize enforcer with Bun adapter
func InitEnforcer(db *bun.DB, modelPath string) (*casbin.Enforcer, error) {
    // Create Bun adapter with existing *bun.DB instance
    adapter := casbinbunadapter.NewAdapter(db)

    // Load RBAC model from file (model.conf)
    enforcer, err := casbin.NewEnforcer(modelPath, adapter)
    if err != nil {
        return nil, fmt.Errorf("create casbin enforcer: %w", err)
    }

    // Register custom bexprMatch function for label filtering
    bexprCache := &sync.Map{}
    enforcer.AddFunction("bexprMatch", func(args ...any) (any, error) {
        scopeExpr := args[0].(string)
        labels := args[1].(map[string]any)

        // Empty expression means no constraint (allow all)
        if strings.TrimSpace(scopeExpr) == "" {
            return true, nil
        }

        // Check cache for compiled evaluator
        if cached, ok := bexprCache.Load(scopeExpr); ok {
            evaluator := cached.(*bexpr.Evaluator)
            matches, err := evaluator.Evaluate(labels)
            if err != nil {
                return false, nil
            }
            return matches, nil
        }

        // Compile and cache evaluator
        evaluator, err := bexpr.CreateEvaluator(scopeExpr)
        if err != nil {
            return false, fmt.Errorf("invalid bexpr: %w", err)
        }
        bexprCache.Store(scopeExpr, evaluator)

        matches, err := evaluator.Evaluate(labels)
        if err != nil {
            return false, nil
        }
        return matches, nil
    })

    // Load policies from database
    if err := enforcer.LoadPolicy(); err != nil {
        return nil, fmt.Errorf("load casbin policies: %w", err)
    }

    return enforcer, nil
}
```

**Seed Data Migration**:
```go
// Default policies seeded via database migration
func seedDefaultPolicies(ctx context.Context, db *bun.DB) error {
    // Using the model: p = role, objType, act, scopeExpr, eft
    type CasbinRule struct {
        PType string
        V0    string // role
        V1    string // objType
        V2    string // act
        V3    string // scopeExpr (go-bexpr)
        V4    string // eft
    }

    policies := []CasbinRule{
        // service-account role: Data plane access only (no scope constraint)
        {"p", "role:service-account", "state", "tfstate:read", "", "allow"},
        {"p", "role:service-account", "state", "tfstate:write", "", "allow"},
        {"p", "role:service-account", "state", "tfstate:lock", "", "allow"},
        {"p", "role:service-account", "state", "tfstate:unlock", "", "allow"},

        // platform-engineer role: Full access (wildcard, no constraint)
        {"p", "role:platform-engineer", "*", "*", "", "allow"},

        // product-engineer role: Label-scoped dev access
        {"p", "role:product-engineer", "state", "state:read", "env == \"dev\"", "allow"},
        {"p", "role:product-engineer", "state", "state:create", "env == \"dev\"", "allow"},
        {"p", "role:product-engineer", "state", "state:list", "env == \"dev\"", "allow"},
        {"p", "role:product-engineer", "state", "state:update-labels", "env == \"dev\"", "allow"},
        {"p", "role:product-engineer", "state", "tfstate:read", "env == \"dev\"", "allow"},
        {"p", "role:product-engineer", "state", "tfstate:write", "env == \"dev\"", "allow"},
        {"p", "role:product-engineer", "state", "tfstate:lock", "env == \"dev\"", "allow"},
        {"p", "role:product-engineer", "state", "tfstate:unlock", "env == \"dev\"", "allow"},
        {"p", "role:product-engineer", "state", "dependency:create", "env == \"dev\"", "allow"},
        {"p", "role:product-engineer", "state", "dependency:read", "env == \"dev\"", "allow"},
        {"p", "role:product-engineer", "state", "dependency:list", "env == \"dev\"", "allow"},
        {"p", "role:product-engineer", "state", "dependency:delete", "env == \"dev\"", "allow"},
        {"p", "role:product-engineer", "policy", "policy:read", "", "allow"},
    }

    for _, pol := range policies {
        _, err := db.NewInsert().
            Model(&pol).
            Table("casbin_rule").
            Exec(ctx)
        if err != nil {
            return err
        }
    }
    return nil
}
```

### Policy API Wrapper
```go
// Add role assignment
func (s *AuthService) AssignRole(ctx context.Context, userID, roleName string) error {
    ok, err := s.enforcer.AddRoleForUser(userID, roleName)
    if err != nil {
        return err
    }
    if !ok {
        return ErrRoleAlreadyAssigned
    }
    // Persist to database
    return s.enforcer.SavePolicy()
}

// Check permission
func (s *AuthService) Enforce(userID, resource, action string) (bool, error) {
    return s.enforcer.Enforce(userID, resource, action)
}
```

### File vs Database Storage

**File-based (CSV)**: REJECTED
- Requires parsing on startup
- Manual file updates during runtime
- No transaction support
- Git merge conflicts for policy changes

**Database-backed (Bun adapter)**: SELECTED
- Policies loaded from `casbin_rule` table (msales/casbin-bun-adapter)
- Runtime updates via Casbin API + SavePolicy()
- Transactional updates (rollback on error)
- Audit trail via database timestamps
- Hot reload: `enforcer.LoadPolicy()` without restart

### Label Scope Enforcement
- go-bexpr evaluates label expressions against state.Labels map
- Enforcement call: `enforcer.Enforce(userID, "state", "read", stateLabels)`
- No scope intersection logic needed - Casbin handles union (OR) of roles
- Empty scope expression = no constraint (matches all resources)
- Example expressions: `env == "dev"`, `env == "dev" and team == "platform"`, `region == "us-west" or region == "us-east"`

### Alternatives Considered
- casbin-dashboard: Rejected (heavyweight Java service, overkill)
- Custom admin UI: Deferred (build if Casbin API insufficient)
- Graph database (Neo4j): Rejected (Casbin handles policy graph internally)
- File-based CSV storage: Rejected (no runtime updates, sync issues)

---

## 9. JWT Claim Extraction for Nested Structures

### Decision
Use **github.com/mitchellh/mapstructure** (already in dependency tree via go-bexpr) for nested claim extraction, with optional JSONPath support for complex cases.

### Rationale
- **mapstructure**: Already imported via go-bexpr dependency (zero additional deps)
- **Handles both patterns**:
  - Flat arrays: `["dev-team", "contractors"]`
  - Nested objects: `[{"name": "dev-team", "type": "team"}]`
- **Simple API**: Works directly with `map[string]interface{}` from JWT claims
- **Proven**: Used by Vault, Consul, Terraform (HashiCorp ecosystem)

### Alternatives Considered

| Library | Pros | Cons | Decision |
|---------|------|------|----------|
| **github.com/mitchellh/mapstructure** | Already in deps (via go-bexpr), simple API, well-tested | Limited path expressions | ✅ **Use for simple nested extraction** |
| **github.com/tidwall/gjson** | Fast, JSONPath-like syntax, widely used | New dependency, overkill for simple cases | ⚠️ Consider for complex nested structures if needed |
| **github.com/PaesslerAG/jsonpath** | Full JSONPath spec, powerful queries | New dependency, slower, complex API | ❌ Reject (overkill) |
| **Manual type assertions** | Zero deps, full control | Brittle, hard to maintain, no path syntax | ❌ Reject (not scalable) |

### Implementation Pattern

**Config Structure** (`cmd/gridapi/internal/config/config.go`):
```go
type OIDCConfig struct {
    Issuer       string `yaml:"issuer" env:"OIDC_ISSUER"`
    ClientID     string `yaml:"client_id" env:"OIDC_CLIENT_ID"`
    ClientSecret string `yaml:"client_secret" env:"OIDC_CLIENT_SECRET"`
    RedirectURI  string `yaml:"redirect_uri" env:"OIDC_REDIRECT_URI"`

    // Claim field configuration
    GroupsClaimField  string `yaml:"groups_claim_field" env:"OIDC_GROUPS_CLAIM" default:"groups"`
    GroupsClaimPath   string `yaml:"groups_claim_path" env:"OIDC_GROUPS_PATH" default:""`  // Optional: JSONPath for nested
    UserIDClaimField  string `yaml:"user_id_claim_field" env:"OIDC_USER_ID_CLAIM" default:"sub"`
    EmailClaimField   string `yaml:"email_claim_field" env:"OIDC_EMAIL_CLAIM" default:"email"`
}
```

**Extraction Logic** (`cmd/gridapi/internal/auth/claims.go`):
```go
package auth

import (
    "fmt"
    "github.com/mitchellh/mapstructure"
)

// ExtractGroups handles both flat and nested group claims
func ExtractGroups(claims map[string]interface{}, claimField string, claimPath string) ([]string, error) {
    rawValue, ok := claims[claimField]
    if !ok {
        return nil, fmt.Errorf("claim field %s not found", claimField)
    }

    // Try flat string array first: ["dev-team", "contractors"]
    if groups, ok := rawValue.([]interface{}); ok {
        result := make([]string, 0, len(groups))
        for _, g := range groups {
            if str, ok := g.(string); ok {
                result = append(result, str)
            }
        }
        if len(result) > 0 {
            return result, nil
        }
    }

    // Try nested extraction if path provided: [{"name": "dev-team"}]
    if claimPath != "" {
        return extractNestedGroups(rawValue, claimPath)
    }

    return nil, fmt.Errorf("groups claim invalid format (expected []string or []object with path)")
}

// extractNestedGroups uses mapstructure to extract from nested objects
func extractNestedGroups(rawValue interface{}, path string) ([]string, error) {
    // For simple nested objects: [{"name": "dev-team"}] with path="name"
    if path == "name" || path == "value" || path == "id" {
        var objects []map[string]interface{}
        if err := mapstructure.Decode(rawValue, &objects); err != nil {
            return nil, err
        }

        result := make([]string, 0, len(objects))
        for _, obj := range objects {
            if val, ok := obj[path].(string); ok {
                result = append(result, val)
            }
        }
        return result, nil
    }

    // For complex paths (future): consider gjson if demand arises
    return nil, fmt.Errorf("complex nested paths not yet supported (path: %s)", path)
}
```

**Example Configurations**:

```yaml
# Keycloak (flat array)
oidc:
  groups_claim_field: "groups"
  groups_claim_path: ""  # Not needed

# Azure AD (nested objects)
oidc:
  groups_claim_field: "groups"
  groups_claim_path: "name"  # Extract from [{"name": "dev-team"}]

# Okta (custom claim)
oidc:
  groups_claim_field: "custom_groups"
  groups_claim_path: ""
```

### YAGNI Approach

**Start simple**:
- Default claim field: `groups`
- Support flat arrays out of the box
- Support simple nested extraction (one-level, single field)
- Document configuration examples for common IdPs

**Defer complex cases**:
- Multi-level nested paths (e.g., `groups[*].metadata.name`)
- JSONPath expressions (e.g., `$..groups[?(@.type=='team')].name`)
- Transform/filter functions (e.g., lowercase, regex match)

**If demand arises later**: Add `gjson` for complex path expressions without breaking existing configs.

### Testing Strategy

**Unit tests** (`cmd/gridapi/internal/auth/claims_test.go`):
```go
func TestExtractGroups_FlatArray(t *testing.T) {
    claims := map[string]interface{}{
        "groups": []interface{}{"dev-team", "contractors"},
    }
    groups, err := ExtractGroups(claims, "groups", "")
    assert.NoError(t, err)
    assert.Equal(t, []string{"dev-team", "contractors"}, groups)
}

func TestExtractGroups_NestedObjects(t *testing.T) {
    claims := map[string]interface{}{
        "groups": []interface{}{
            map[string]interface{}{"name": "dev-team", "type": "team"},
            map[string]interface{}{"name": "admins", "type": "role"},
        },
    }
    groups, err := ExtractGroups(claims, "groups", "name")
    assert.NoError(t, err)
    assert.Equal(t, []string{"dev-team", "admins"}, groups)
}
```

**Integration tests**: Test with real Keycloak/Azure AD JWT tokens

### Performance Considerations

- **Flat array extraction**: O(n) where n = number of groups (fast)
- **Nested extraction with mapstructure**: O(n) with small constant factor (acceptable)
- **Caching**: Not needed (extraction happens once per authentication, not per request)

---

## 10. Additional Libraries

### go-chi/chi/v5
- **Purpose**: HTTP router (existing dependency)
- **Usage**: Mount OIDC endpoints, apply middleware
- **Version**: v5.1.0+

### uptrace/bun
- **Purpose**: ORM for database (existing dependency)
- **Usage**: Auth tables (roles, users, sessions) and casbin_rule
- **Version**: v1.2.0+

### hashicorp/go-bexpr
- **Purpose**: Boolean expression evaluation for label filtering
- **Usage**: Evaluate scope expressions like `env == "dev" and team == "platform"` against state labels
- **Version**: Latest (github.com/hashicorp/go-bexpr)
- **Performance**: Compiled expressions cached per unique string
- **Bonus**: Brings mitchellh/mapstructure as transitive dependency (used for claim extraction)

### golang.org/x/crypto/bcrypt
- **Purpose**: Hash service account secrets
- **Usage**: bcrypt.GenerateFromPassword(secret, bcrypt.DefaultCost)
- **Version**: Latest (golang.org/x/crypto)

### google/uuid
- **Purpose**: Generate GUIDs for client_id, session tokens
- **Usage**: uuid.New().String()
- **Version**: v1.6.0+

### msales/casbin-bun-adapter
- **Purpose**: Casbin policy storage in PostgreSQL via Bun
- **Usage**: Adapter for Casbin enforcer
- **Version**: Latest (github.com/msales/casbin-bun-adapter)

### mitchellh/mapstructure
- **Purpose**: Decode nested structures from JWT claims (transitive via go-bexpr)
- **Usage**: Extract groups from nested claim objects like `[{"name": "dev-team"}]`
- **Version**: Latest (already in dep tree)
- **Note**: No additional `go get` needed

---

## 11. Performance Considerations

### Token Validation
- Target: <10ms per request
- JWKS cached (no network call)
- JWT parsed and verified (cryptographic operation)
- Estimated: 2-5ms on modern CPU

### Authorization Check
- Target: <5ms per request
- Casbin in-memory policy (hash map lookup)
- No database query per request
- Estimated: 0.5-2ms

### Session Lookup
- Target: <5ms per request
- Database query: SELECT * FROM sessions WHERE token_hash = ?
- Index on token_hash column
- Estimated: 1-3ms with proper indexing

### Overall Request Overhead
- Auth middleware: ~10ms (token validation)
- Authz middleware: ~5ms (policy check)
- Session lookup: ~5ms (DB query)
- **Total**: ~20ms added latency per authenticated request
- Acceptable for target <200ms p95 overall latency

---

## 12. Security Considerations

### Token Storage (CLI)
- Location: `~/.grid/credentials.json`
- Permissions: 0600 (read/write owner only)
- Format: JSON with access_token, refresh_token, expiry

### Secret Management
- Client secrets never in source code (env vars only)
- Service account secrets bcrypt hashed (cost 10)
- Database connection string from env (DATABASE_URL)

### HTTPS Required
- All OIDC flows require HTTPS (except localhost dev)
- Tokens transmitted over TLS only
- HTTP → HTTPS redirect enforced

### Token Expiry
- Access tokens: 12 hours (per spec clarifications)
- Refresh tokens: 30 days (IdP-configured)
- Session tokens: 12 hours (Grid-managed)

### CORS
- OIDC callback endpoints: No CORS (server-side only)
- Connect RPC endpoints: CORS configured for webapp origin

---

## 13. Open Questions Resolved

All questions resolved via user input and spec clarifications:

1. ✅ **Session duration**: 12 hours (uniform across roles)
2. ✅ **Multiple role scopes**: Intersection (AND) semantics
3. ✅ **Permission definition format**: JSON
4. ✅ **Export/import format**: JSON
5. ✅ **Role deletion**: Require unassignment first
6. ✅ **Information disclosure**: Do not reveal state existence
7. ✅ **IdP timeout**: 2-5 seconds
8. ✅ **Credential rotation**: Old tokens valid until expiry
9. ✅ **Concurrent sessions**: Unlimited
10. ✅ **Rate limiting**: None (YAGNI)

---

## 14. Webapp Authentication Patterns (React + Vite)

### Existing Patterns Analysis

**Current Implementation** (webapp/src/):
- **Context Pattern**: `GridContext.tsx` provides centralized state management via React Context API
  - Provider wraps App component, injects `api` and `transport` into component tree
  - Custom hook `useGrid()` provides type-safe access with error handling
- **Service Pattern**: `gridApi.ts` creates singleton instances
  - Uses environment variable `VITE_GRID_API_URL` for base URL configuration
  - Instantiates `GridApiAdapter` with Connect transport
- **Hook Pattern**: `useGridData.ts` encapsulates data fetching logic
  - Uses `useState` for state management (loading, error, data)
  - Uses `useCallback` for memoized functions
  - Manual refresh pattern (no background polling)

### Auth Integration Best Practices (2025)

**Context API Pattern** (RECOMMENDED):
- Create `AuthContext.tsx` following existing `GridContext.tsx` pattern
- Store: authentication state, user identity, permissions, auth functions
- Wrap in `App.tsx` alongside `GridProvider` (auth wraps grid for dependency order)

**Token Storage Security**:
- **NEVER use localStorage** - vulnerable to XSS attacks
- **Use httpOnly cookies for refresh tokens** (server sets, JS cannot read)
- **Short-lived access tokens in memory** (React state only, lost on refresh)
- **Token refresh on mount** - AuthContext effect fetches new token from cookie-based refresh endpoint

**Implementation Architecture**:

```typescript
// webapp/src/context/AuthContext.tsx
interface AuthState {
  user: UserInfo | null;
  loading: boolean;
  error: string | null;
}

interface AuthContextValue {
  ...AuthState;
  login: () => void;           // Redirect to /auth/login
  logout: () => Promise<void>; // Call /auth/logout, clear state
  refreshToken: () => Promise<void>;
}

// Pattern: Mirror GridContext structure
export function AuthProvider({ children }: { children: ReactNode }) {
  const [state, setState] = useState<AuthState>({ user: null, loading: true, error: null });

  useEffect(() => {
    // On mount: attempt token refresh from httpOnly cookie
    refreshToken().catch(() => setState(s => ({ ...s, loading: false })));
  }, []);

  // ... rest of implementation
}
```

**Service Layer**:

```typescript
// webapp/src/services/authApi.ts
// HTTP endpoints (NOT Connect RPC - OIDC flow exception)

export async function initLogin(redirectUri: string): Promise<void> {
  window.location.href = `/auth/login?redirect_uri=${encodeURIComponent(redirectUri)}`;
}

export async function handleCallback(code: string, state: string): Promise<UserInfo> {
  const response = await fetch('/auth/callback?code=${code}&state=${state}', {
    credentials: 'include', // Send httpOnly cookies
  });
  return response.json();
}

export async function logout(): Promise<void> {
  await fetch('/auth/logout', {
    method: 'POST',
    credentials: 'include',
  });
}

export async function refreshToken(): Promise<UserInfo> {
  const response = await fetch('/auth/refresh', {
    method: 'POST',
    credentials: 'include', // httpOnly cookie sent automatically
  });
  if (!response.ok) throw new Error('Refresh failed');
  return response.json();
}
```

**Route Protection**:

```typescript
// webapp/src/components/AuthGuard.tsx
export function AuthGuard({ children }: { children: ReactNode }) {
  const { user, loading } = useAuth();

  if (loading) return <LoadingSpinner />;
  if (!user) {
    // Redirect to login, preserve intended route
    window.location.href = `/login?return_to=${encodeURIComponent(window.location.pathname)}`;
    return null;
  }

  return <>{children}</>;
}

// Usage in App.tsx
<AuthGuard>
  <Dashboard />
</AuthGuard>
```

**Permission-Gated Actions**:

```typescript
// webapp/src/components/ProtectedAction.tsx
interface Props {
  requiredPermission: string;
  children: ReactNode;
  fallback?: ReactNode;
}

export function ProtectedAction({ requiredPermission, children, fallback }: Props) {
  const { hasPermission } = usePermissions();

  if (!hasPermission(requiredPermission)) {
    return fallback || null;
  }

  return <>{children}</>;
}

// Usage
<ProtectedAction requiredPermission="state:delete">
  <Button onClick={deleteState}>Delete State</Button>
</ProtectedAction>
```

**Error Handling in Existing gridApi.ts**:

```typescript
// Update webapp/src/services/gridApi.ts
// Intercept 401 responses, trigger re-auth

import { createPromiseClient, Interceptor } from '@connectrpc/connect';

const authInterceptor: Interceptor = (next) => async (req) => {
  try {
    return await next(req);
  } catch (err) {
    if (err.code === 'unauthenticated') {
      // Clear auth state, redirect to login
      window.location.href = `/login?return_to=${encodeURIComponent(window.location.pathname)}`;
    }
    throw err;
  }
};

export const gridTransport = createGridTransport(getApiBaseUrl(), {
  interceptors: [authInterceptor],
});
```

### Integration with Connect RPC

**Admin operations** (role management, service accounts) use Connect RPC:
- Generated TypeScript clients from proto files (js/sdk/gen/)
- Webapp imports `@tcons/grid` SDK for admin RPCs
- Connect transport automatically includes credentials for session cookie

**OIDC flow** uses direct HTTP fetch:
- Exception to "depend only on Connect RPC" rule (documented in Constitution Check)
- Browser redirect requirements incompatible with Connect RPC streaming

### Dashboard READ ONLY Scope

**Current Status**: Dashboard (webapp/) is **READ ONLY** in current feature scope.

**Auth Integration Tasks** (webapp tasks 42-52 in plan.md):
- **AuthContext, useAuth, AuthGuard**: Apply to all routes (no new write operations)
- **ProtectedAction component**: Prepared for future write operations, but not used in current dashboard
- **Admin operations**: Out of scope for dashboard (CLI-only: role management, service account creation)

**Functional Requirements Impact**:
- No need to update FR for dashboard write operations (already out of scope)
- Dashboard displays states/dependencies/labels (existing read operations)
- Auth protects existing read operations based on label scope (e.g., product-engineer sees only env=dev states)

### Testing Strategy

**React Testing Library** (webapp/src/__tests__/):
- Follow existing test patterns (dashboard_*.test.tsx files)
- Mock `useAuth` hook in tests:
  ```typescript
  jest.mock('../context/AuthContext', () => ({
    useAuth: () => ({ user: mockUser, loading: false, error: null }),
  }));
  ```
- Test AuthGuard rendering logic (loading, unauthenticated, authenticated states)
- Test ProtectedAction visibility based on permissions

**E2E Testing**:
- Use existing Playwright/Cypress setup if available
- Test OIDC flow end-to-end with Keycloak in docker-compose

---

## References

- [Casbin Documentation](https://casbin.org/docs/overview)
- [Casbin SyncedEnforcer](https://casbin.org/docs/en/synced-enforcer)
- [go-oidc Library](https://github.com/coreos/go-oidc)
- [RFC 8252: OAuth 2.0 for Native Apps](https://datatracker.ietf.org/doc/html/rfc8252)
- [RFC 7636: PKCE](https://datatracker.ietf.org/doc/html/rfc7636)
- [RFC 6749: OAuth 2.0](https://datatracker.ietf.org/doc/html/rfc6749)
- [Azure AD OIDC Documentation](https://learn.microsoft.com/en-us/azure/active-directory/develop/v2-protocols-oidc)
- [Keycloak Documentation](https://www.keycloak.org/docs/latest/server_admin/)

---

**Status**: All research complete. Ready for Phase 1 (Design & Contracts).
