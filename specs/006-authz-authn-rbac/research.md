# Research: Authentication, Authorization, and RBAC

**Feature**: 006-authz-authn-rbac
**Date**: 2025-10-11
**Status**: Complete

## Overview

This document consolidates research findings for implementing authentication and authorization in Grid using industry-standard libraries and protocols.

---

## 1. Casbin Integration with Chi Router

### Decision
Use **github.com/casbin/chi-authz** middleware with **github.com/casbin/casbin/v2** enforcer and **github.com/msales/casbin-bun-adapter** for PostgreSQL storage.

### Rationale
- Official Chi middleware maintained by Casbin project
- msales/casbin-bun-adapter accepts existing *bun.DB (shares connection pool, supports transactions)
- Zero custom middleware development required
- Proven integration pattern (used in production by multiple projects)
- Supports policy hot-reloading without server restart
- In-memory policy evaluation (<1ms latency)
- Union (OR) semantics by default - any role granting access is sufficient

### Implementation Pattern
```go
import (
    "github.com/casbin/casbin/v2"
    casbinbunadapter "github.com/msales/casbin-bun-adapter"
    chiauth "github.com/casbin/chi-authz"
    "github.com/hashicorp/go-bexpr"
)

// Initialize adapter with existing *bun.DB instance
adapter := casbinbunadapter.NewAdapter(bunDB)

// Create enforcer with model and adapter
enforcer, _ := casbin.NewEnforcer("model.conf", adapter)

// Register custom bexprMatch function for label filtering
enforcer.AddFunction("bexprMatch", func(args ...any) (any, error) {
    scopeExpr := args[0].(string)
    labels := args[1].(map[string]any)
    return evaluateBexpr(scopeExpr, labels), nil
})

// Load policies from database
_ = enforcer.LoadPolicy()

// Attach middleware
r.Use(chiauth.NewAuthorizer(enforcer, chiauth.WithSubjectFn(extractSubject)))

// Custom subject extraction from JWT claims
func extractSubject(r *http.Request) string {
    // Extract user ID from context (set by auth middleware)
    userID, ok := r.Context().Value("user_id").(string)
    if !ok {
        return ""
    }
    return userID
}
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

## 2. OIDC Provider Setup

### Decision
- **Development**: Keycloak 22+ in Docker Compose
- **Production**: Azure Entra ID (Microsoft identity platform)

### Rationale

**Keycloak**:
- Open-source, self-hosted (no cloud vendor lock-in for dev)
- Full OIDC support with discovery endpoint
- Supports all required flows (authorization code, client credentials, device code)
- Docker image: quay.io/keycloak/keycloak:22.0

**Azure Entra ID**:
- Production-grade SLA (99.99% uptime)
- Native integration with Microsoft 365 (if org uses it)
- Enterprise features: Conditional Access, MFA policies
- Well-documented OAuth2/OIDC endpoints

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
Use **PKCE + Loopback Redirect** (OAuth2 Authorization Code Flow with PKCE).

### Rationale
- Standard OAuth2 flow recommended by IETF RFC 8252
- No client secret required (public client)
- Secure against authorization code interception attacks
- Works on machines with browser access
- Azure Entra ID officially supports localhost redirect (https://localhost:port)

### Flow Steps
1. Generate PKCE code verifier (random 43-128 char string)
2. Derive code challenge: SHA256(verifier) → Base64URL
3. Start local HTTP server on random port (e.g., :5432)
4. Build authorization URL with challenge + redirect_uri=http://localhost:5432
5. Open browser (or print URL for user to copy)
6. User authenticates at IdP
7. IdP redirects to localhost:5432?code=...
8. Exchange code + verifier for tokens
9. Store access token locally (~/.grid/credentials.json)
10. Shutdown local server

### Implementation Libraries
- **golang.org/x/oauth2**: Standard OAuth2 client (code exchange, refresh)
- **github.com/nirasan/go-oauth-pkce-code-verifier**: PKCE generation helper
- **net/http + net.Listen**: Loopback server (stdlib, no deps)

### Port Handling
```go
// Find random free port
listener, err := net.Listen("tcp", "127.0.0.1:0")
if err != nil {
    return err
}
port := listener.Addr().(*net.TCPAddr).Port
redirectURI := fmt.Sprintf("http://127.0.0.1:%d/callback", port)
```

### Error Handling
- User cancels: Server timeout after 5 minutes, return error
- Code reused: OAuth2 error response, prompt re-login
- Port collision: Try next random port (loop 3 times)
- Firewall blocks localhost: Fallback to manual code entry (future enhancement)

### Alternatives Considered
- Device code flow: Rejected for primary flow (worse UX, requires typing code)
  - Note: Still supported for headless environments
- Embedded browser: Rejected (platform-specific, security concerns)
- Copy/paste token: Rejected (poor UX, token exposure risk)

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

## 6. JWT Validation

### Decision
Use **github.com/coreos/go-oidc/v3/oidc** for validation.

### Rationale
- Handles JWKS fetching and caching automatically
- Verifies all standard claims (iss, aud, exp, nbf, iat)
- Validates signature using keys from IdP's JWKS endpoint
- Thread-safe caching (no race conditions)
- Used by Kubernetes, Prometheus, and other CNCF projects

### Validation Process
```go
import "github.com/coreos/go-oidc/v3/oidc"

// Initialize verifier (startup)
provider, err := oidc.NewProvider(ctx, issuerURL)
verifier := provider.Verifier(&oidc.Config{
    ClientID: clientID,
})

// Validate token (per request)
idToken, err := verifier.Verify(ctx, rawIDToken)
if err != nil {
    // Invalid signature, expired, wrong issuer, etc.
    return err
}

// Extract claims
var claims struct {
    Sub   string `json:"sub"`
    Email string `json:"email"`
    Roles []string `json:"roles"`
}
idToken.Claims(&claims)
```

### JWKS Caching
- go-oidc fetches JWKS on first use
- Cached in memory (no external store needed)
- Refresh on kid (key ID) not found (handles key rotation)
- Respects Cache-Control headers from IdP

### Key Rotation Handling
1. IdP publishes new key to JWKS (both old and new keys present)
2. New tokens signed with new key
3. go-oidc detects unknown kid → refetches JWKS
4. Old tokens still valid (old key still in JWKS)
5. IdP removes old key after grace period (e.g., 24 hours)

### No Custom Signing
- Grid NEVER signs JWTs
- IdP is sole issuer (Grid is consumer only)
- Tokens verified against IdP's public keys (JWKS)
- No private keys stored in Grid

### Alternatives Considered
- golang-jwt/jwt: Rejected (manual JWKS handling)
- jose: Rejected (lower-level, more code)
- Custom validation: Rejected (security risk)

---

## 7. Casbin Policy Management

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

## 8. JWT Claim Extraction for Nested Structures

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

## 9. Additional Libraries

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

## 10. Performance Considerations

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

## 11. Security Considerations

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

## 12. Open Questions Resolved

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

## 13. Webapp Authentication Patterns (React + Vite)

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
- [Chi-authz Middleware](https://github.com/casbin/chi-authz)
- [go-oidc Library](https://github.com/coreos/go-oidc)
- [RFC 8252: OAuth 2.0 for Native Apps](https://datatracker.ietf.org/doc/html/rfc8252)
- [RFC 7636: PKCE](https://datatracker.ietf.org/doc/html/rfc7636)
- [RFC 6749: OAuth 2.0](https://datatracker.ietf.org/doc/html/rfc6749)
- [Azure AD OIDC Documentation](https://learn.microsoft.com/en-us/azure/active-directory/develop/v2-protocols-oidc)
- [Keycloak Documentation](https://www.keycloak.org/docs/latest/server_admin/)

---

**Status**: All research complete. Ready for Phase 1 (Design & Contracts).
