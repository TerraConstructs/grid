# Security Review: OIDC Client Implementation in gridapi

**Date**: 2025-12-28
**Reviewer**: Claude (Opus 4.5)
**Scope**: `cmd/gridapi/internal/auth/` - OIDC client security (key verification, signature check, expiration validation, replay attacks)

## Executive Summary

The OIDC client implementation in `cmd/gridapi` is **well-architected with proper security controls**. The implementation correctly handles key verification, signature validation, and expiration checks. I found **3 issues** (1 medium, 2 low severity) and documented **6 security best practices** that are correctly implemented.

**Severity Summary**:
- Critical: 0
- High: 0
- Medium: 1 (nbf clock skew)
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

## Security Issues Found

### Issue #1: MEDIUM - No Clock Skew Tolerance for `nbf` Claim

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

### Issue #2: LOW - Insecure Cookie Handler in Development

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

### Issue #3: LOW - ParseUnverified Used for ID Token Group Extraction

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
| PKCE for OAuth flow | `rp.WithPKCE()` enabled | `relying_party.go:63` |
| ID token max age | 10-second max age for `iat` | `relying_party.go:62` |
| Token hashing for storage | SHA256 hashing | `session.go:50-55` |

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

The OIDC client implementation in gridapi is **secure and well-designed**. The architecture correctly separates concerns between the IAM service and authentication middleware, with proper layered validation of tokens. The use of JTI revocation provides replay attack prevention, and the hardcoded RS256 algorithm prevents algorithm confusion attacks.

The medium-severity issue with `nbf` clock skew should be addressed to prevent false rejections in distributed environments with clock drift.
