# Security Review: OIDC Client Implementation in gridapi

**Date**: 2025-12-28
**Reviewer**: Claude (Opus 4.5)
**Scope**: `cmd/gridapi/internal/auth/` - OIDC client security (key verification, signature check, expiration validation, replay attacks)

## Executive Summary

The OIDC client implementation in `cmd/gridapi` is **well-architected with proper security controls**. The implementation correctly handles key verification, signature validation, and expiration checks. I found **3 issues** (1 medium, 2 low severity) and documented **6 security best practices** that are correctly implemented.

**Severity Summary**:
- Critical: 0
- High: 1 (`op.WithAllowInsecure()` on OIDC provider)
- Medium: 2 (nbf clock skew, in-memory OAuth state storage)
- Low: 2 (insecure cookie handler, ParseUnverified)

---

## 1. Key Verification (JWKS)

### Internal IdP Mode (Grid issues tokens)

**Location**: `cmd/gridapi/internal/auth/oidc.go:57-132`

| Check | Status | Details |
|-------|--------|---------|
| Key size | ✅ Pass | RSA 2048-bit keys (line 100) |
| Key persistence | ✅ Pass | Keys saved to disk, reloaded on restart |
| Key ID (kid) | ✅ Pass | Unique UUID generated and persisted (line 106) |
| JWKS endpoints | ✅ Pass | Exposed at `/keys` and `/jwks` |

### External IdP Mode (Grid validates tokens from Keycloak/Entra/etc.)

**Location**: `cmd/gridapi/internal/auth/jwt.go:121-135`, `cmd/gridapi/internal/services/iam/jwt_auth.go:77-90`

| Check | Status | Details |
|-------|--------|---------|
| JWKS fetching | ✅ Pass | go-oidc-middleware auto-fetches from `.well-known/openid-configuration` |
| JWKS caching | ✅ Pass | Handled by library |
| Lazy loading | ✅ Pass | `WithLazyLoadJwks(true)` for internal provider to avoid race condition |

**Finding**: No issues found with key verification.

---

## 2. Signature Verification

### Algorithm Security

**Location**: `cmd/gridapi/internal/auth/oidc.go:229`

| Check | Status | Details |
|-------|--------|---------|
| Algorithm confusion | ✅ Pass | Only RS256 is used, hardcoded not from JWT header |
| "none" algorithm | ✅ Pass | Not present anywhere in codebase |
| Algorithm configuration | ✅ Pass | Not configurable by attacker |

```go
// oidc.go:227-229
signingKey: &rsaSigningKey{
    id:        keyID,
    algorithm: jose.RS256,  // Hardcoded RS256
    key:       privateKey,
},
```

### Signature Verification Flow

**Location**: `cmd/gridapi/internal/services/iam/jwt_auth.go:128-132`

| Check | Status | Details |
|-------|--------|---------|
| Verification library | ✅ Pass | go-jose/go-jose/v4 (v4.1.3) |
| Signature validation | ✅ Pass | `tokenHandler.ParseToken()` validates against JWKS |

**Finding**: No issues found with signature verification.

---

## 3. Expiration Validation & Replay Attack Prevention

### Multi-Layer Expiration Validation

| Check | Location | Status | Details |
|-------|----------|--------|---------|
| `exp` claim (auto) | `jwt_auth.go:129` | ✅ Pass | `tokenHandler.ParseToken()` validates automatically |
| `exp` claim (manual) | `validation.go:21-36` | ✅ Pass | `time.Now().Unix() > expTime` |
| `iat` claim | `validation.go:38-54` | ✅ Pass | 5-minute clock skew tolerance |
| `nbf` claim | `validation.go:56-71` | ⚠️ Issue | No clock skew tolerance (see issue #1) |
| Session expiration | `session.go:105-109` | ✅ Pass | Proper expiration check |

### Token Lifetimes

**Location**: `cmd/gridapi/internal/auth/oidc.go:36-42`

| Token Type | TTL | Status |
|------------|-----|--------|
| Access Token | 120 minutes | ✅ Appropriate |
| Refresh Token | 24 hours | ✅ Appropriate |
| ID Token | 15 minutes | ✅ Appropriate |

### JTI (JWT ID) Revocation for Replay Attack Prevention

**Location**: `cmd/gridapi/internal/services/iam/jwt_auth.go:134-161`

| Check | Status | Details |
|-------|--------|---------|
| JTI required | ✅ Pass | Validated at lines 135-138 |
| JTI revocation check | ✅ Pass | Database lookup at lines 154-161 |
| Revocation persistence | ✅ Pass | Stored in `revoked_jti` table with expiration for cleanup |

```go
// jwt_auth.go:154-161
isRevoked, err := a.revokedJTIs.IsRevoked(ctx, jti)
if err != nil {
    return nil, fmt.Errorf("check revocation status: %w", err)
}
if isRevoked {
    return nil, fmt.Errorf("token has been revoked")
}
```

**Finding**: Replay attack prevention via JTI revocation is properly implemented.

---

## 4. Internal IdP Provider Security (Mode 2)

When Grid operates in internal IdP mode, it acts as a full OIDC provider using the zitadel/oidc library. The provider exposes standard OIDC/OAuth2 endpoints that are mounted at the root path.

### OIDC Provider Endpoints

**Location**: `cmd/gridapi/internal/auth/oidc.go:160-171`, `cmd/gridapi/internal/server/router.go:107-116`

The provider is created with `op.CreateRouter(provider)` and mounted at `/`:

| Endpoint | Purpose | Authentication |
|----------|---------|----------------|
| `/.well-known/openid-configuration` | OIDC discovery | Public |
| `/.well-known/jwks.json`, `/keys`, `/jwks` | JWKS endpoint | Public |
| `/authorize` | Authorization endpoint | User session |
| `/oauth/token` | Token endpoint | Client credentials (bcrypt) |
| `/oauth/introspect` | Token introspection | Client credentials |
| `/userinfo` | UserInfo endpoint | Bearer token |
| `/device_authorization` | Device flow | Client ID |
| `/revoke` | Token revocation | Client credentials |

### Token Endpoint Security

**Location**: `cmd/gridapi/internal/auth/oidc.go:463-472`

| Check | Status | Details |
|-------|--------|---------|
| Client authentication | ✅ Pass | `AuthorizeClientIDSecret()` uses bcrypt comparison |
| Disabled account check | ✅ Pass | `ClientCredentialsTokenRequest()` checks `sa.Disabled` |
| PKCE support | ✅ Pass | `CodeMethodS256: true` in config (line 152) |

```go
// oidc.go:468
if err := bcrypt.CompareHashAndPassword([]byte(sa.ClientSecretHash), []byte(clientSecret)); err != nil {
    return fmt.Errorf("invalid client secret")
}
```

### Introspection Endpoint Security

**Location**: `cmd/gridapi/internal/auth/oidc.go:493-515`

| Check | Status | Details |
|-------|--------|---------|
| Session revocation check | ✅ Pass | Returns `Active: false` for revoked sessions |
| Expiration check | ✅ Pass | `resp.Active = time.Now().Before(session.ExpiresAt)` |
| Unknown token handling | ✅ Pass | Returns `Active: false` for not-found tokens |

### UserInfo Endpoint Security

**Location**: `cmd/gridapi/internal/auth/oidc.go:482-491`

| Check | Status | Details |
|-------|--------|---------|
| Session revocation check | ✅ Pass | Returns error for revoked sessions |
| Token validation | ✅ Pass | Looks up session by token hash |

### Device Authorization Flow

**Location**: `cmd/gridapi/internal/auth/oidc.go:529-564`

| Check | Status | Details |
|-------|--------|---------|
| Client validation | ✅ Pass | Validates client exists via `GetByClientID()` |
| Duplicate user code | ✅ Pass | Returns `ErrDuplicateUserCode` if exists |
| Client ID binding | ✅ Pass | `GetDeviceAuthorizatonState()` validates clientID matches |

---

## Security Issues Found

### Issue #1: HIGH - `op.WithAllowInsecure()` Enables HTTP for OIDC Provider

**Location**: `cmd/gridapi/internal/auth/oidc.go:160`

**Code**:
```go
provider, err := op.NewProvider(opConfig, storage, op.StaticIssuer(cfg.Issuer), op.WithAllowInsecure())
```

**Issue**: The OIDC provider is created with `op.WithAllowInsecure()`, which allows the provider to operate over HTTP instead of requiring HTTPS. This means:
- Authorization codes transmitted in the clear
- ID tokens and access tokens visible to network observers
- Client secrets potentially exposed during token requests

**Impact**: Man-in-the-middle attacks can intercept OAuth flows and steal tokens/credentials.

**Recommendation**: Make `WithAllowInsecure()` conditional on configuration:
```go
opts := []op.Option{op.StaticIssuer(cfg.Issuer)}
if cfg.DevMode || cfg.AllowInsecure {
    opts = append(opts, op.WithAllowInsecure())
}
provider, err := op.NewProvider(opConfig, storage, opts...)
```

**Note**: If HTTPS is enforced at the network layer (load balancer/reverse proxy), this issue is mitigated. However, the code should not assume this.

---

### Issue #2: MEDIUM - In-Memory OAuth State Storage

**Location**: `cmd/gridapi/internal/auth/oidc.go:186-191`

**Code**:
```go
type providerStorage struct {
    // ...
    authRequests  map[string]*authRequest
    authCodes     map[string]string
    refreshTokens map[string]*refreshToken
    deviceCodes   map[string]deviceAuthorizationEntry
    userCodes     map[string]string
    // ...
}
```

**Issue**: OAuth state (authorization requests, authorization codes, refresh tokens, device codes) is stored in-memory maps. This means:
1. **Server restart**: All pending OAuth flows are invalidated
2. **Multi-instance deployment**: OAuth state is not shared between instances
3. **Refresh token loss**: Valid refresh tokens are lost on restart, forcing re-authentication

**Impact**:
- Users experience authentication failures after server restarts
- Cannot scale horizontally without sticky sessions
- Refresh tokens issued before restart become invalid

**Recommendation**:
- Move OAuth state to database (similar to how sessions are already persisted)
- Alternatively, document that single-instance deployment is required for internal IdP mode
- Note: JWT access tokens remain valid after restart (signature-based validation)

---

### Issue #3: MEDIUM - No Clock Skew Tolerance for `nbf` Claim

**Location**: `cmd/gridapi/internal/auth/validation.go:68-70`

**Code**:
```go
if time.Now().Unix() < nbfTime {
    return fmt.Errorf("token not yet valid")
}
```

**Issue**: The `nbf` (not before) validation has no clock skew tolerance, while `iat` has 5-minute tolerance. This could cause legitimate tokens to be rejected when server/client clocks are slightly out of sync.

**Impact**: Authentication failures for valid tokens in distributed environments with clock drift.

**Recommendation**:
```go
// Add 5-minute clock skew tolerance consistent with iat validation
if time.Now().Unix() < nbfTime-300 {
    return fmt.Errorf("token not yet valid")
}
```

### Issue #4: LOW - Insecure Cookie Handler in Development

**Location**: `cmd/gridapi/internal/auth/relying_party.go:43`

**Code**:
```go
cookieHandler := httphelper.NewCookieHandler(hashKey, cryptoKey, httphelper.WithUnsecure())
```

**Issue**: `WithUnsecure()` is always enabled, which allows cookies over HTTP. This should be conditional based on environment/config.

**Impact**: Session cookies sent over unencrypted HTTP in production if HTTPS not enforced at network layer.

**Recommendation**:
```go
var opts []httphelper.CookieHandlerOpt
if !cfg.DevMode { // or check for HTTPS
    opts = append(opts, httphelper.WithSecure())
}
cookieHandler := httphelper.NewCookieHandler(hashKey, cryptoKey, opts...)
```

### Issue #5: LOW - ParseUnverified Used for ID Token Group Extraction

**Location**: `cmd/gridapi/internal/auth/jwt.go:283-312`

**Code**:
```go
// Parse without verification (token is already validated and stored in DB)
_, _, err := new(jwt.Parser).ParseUnverified(idToken, claims)
```

**Issue**: `ParseUnverified` is used to extract groups from stored ID tokens. While the comment justifies this (tokens are already validated and stored in DB), if the database were compromised or tokens modified, this could lead to privilege escalation through fake group claims.

**Impact**: Potential privilege escalation if database is compromised.

**Recommendation**: Consider storing groups separately at session creation time, rather than re-extracting from the stored ID token. This reduces the attack surface and eliminates the need for `ParseUnverified`.

---

## Security Best Practices Correctly Implemented

| Practice | Implementation | Location |
|----------|----------------|----------|
| RS256 only (no algorithm confusion) | Hardcoded `jose.RS256` | `oidc.go:229` |
| JTI required for all tokens | Validates JTI presence | `jwt.go:177-182`, `jwt_auth.go:135-138` |
| JTI revocation checking | Database-backed revocation list | `jwt_auth.go:154-161` |
| PKCE for OAuth flow (S256) | `CodeMethodS256: true` | `oidc.go:152`, `relying_party.go:63` |
| ID token max age | 10-second max age for `iat` | `relying_party.go:62` |
| Token hashing for storage | SHA256 hashing | `session.go:50-55` |
| Client secret validation | bcrypt comparison | `oidc.go:468` |
| Disabled account check | `sa.Disabled` check | `oidc.go:581-583` |
| Introspection revocation check | Returns `Active: false` for revoked | `oidc.go:502-505` |
| UserInfo revocation check | Returns error for revoked sessions | `oidc.go:487-489` |

---

## Authentication Flow Verification

The authentication flow correctly routes through the IAM service:

1. `TerraformBasicAuthShim` (`serve.go:238`) - converts Basic Auth to Bearer
2. `MultiAuthMiddleware` (`serve.go:242-243`) → `iamService.AuthenticateRequest()`
3. `JWTAuthenticator.Authenticate()` performs:
   - Token extraction from Authorization header
   - Signature verification via `tokenHandler.ParseToken()` (go-oidc-middleware)
   - **JTI revocation check** ✅
   - Identity resolution (user or service account)
   - Role resolution via immutable cache

The old `jwt.go` `NewVerifier` middleware is NOT in the primary authentication path. The `JWTAuthenticator` in the IAM service handles all JWT authentication with proper revocation checking.

---

## Dependencies Review

| Dependency | Version | Status | Notes |
|------------|---------|--------|-------|
| `go-jose/go-jose/v4` | v4.1.3 | ✅ Current | Released Oct 2024, no known CVEs |
| `xenitab/go-oidc-middleware` | v0.0.44 | ✅ Current | Actively maintained |
| `zitadel/oidc/v3` | v3.45.0 | ✅ Current | Well-maintained OIDC library |
| `golang-jwt/jwt/v5` | v5.3.0 | ✅ Current | No known CVEs |

---

## Files Reviewed

- `cmd/gridapi/internal/auth/jwt.go` - JWT verification middleware
- `cmd/gridapi/internal/auth/oidc.go` - Internal OIDC provider
- `cmd/gridapi/internal/auth/validation.go` - Claims validation
- `cmd/gridapi/internal/auth/relying_party.go` - External IdP integration
- `cmd/gridapi/internal/auth/session.go` - Session management
- `cmd/gridapi/internal/auth/claims.go` - Claims extraction helpers
- `cmd/gridapi/internal/services/iam/jwt_auth.go` - JWT authenticator
- `cmd/gridapi/internal/services/iam/service_impl.go` - IAM service
- `cmd/gridapi/internal/middleware/authn_multiauth.go` - Authentication middleware
- `cmd/gridapi/cmd/serve.go` - Server startup and middleware wiring

---

## Conclusion

The OIDC client implementation in gridapi is **well-designed with solid security fundamentals**. The architecture correctly separates concerns between the IAM service and authentication middleware, with proper layered validation of tokens. The use of JTI revocation provides replay attack prevention, and the hardcoded RS256 algorithm prevents algorithm confusion attacks.

**Key Findings for Internal IdP Mode (Mode 2)**:

The internal OIDC provider endpoints (token, introspect, userinfo, device auth) are properly secured with:
- Client credential validation using bcrypt
- Session revocation checking on protected endpoints
- PKCE support (S256 challenge method)
- Disabled account enforcement

**Priority Remediation**:

1. **HIGH**: Remove or conditionally enable `op.WithAllowInsecure()` - OAuth flows over HTTP expose tokens to interception
2. **MEDIUM**: Address in-memory OAuth state storage for production deployments requiring high availability
3. **MEDIUM**: Add clock skew tolerance for `nbf` claim validation

The high-severity issue should be addressed before production deployment in environments where HTTPS is not enforced at the network layer.
