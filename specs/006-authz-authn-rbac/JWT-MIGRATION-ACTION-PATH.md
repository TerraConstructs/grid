# JWT Migration Action Path

**Purpose**: Migrate from opaque tokens to JWT-based authentication architecture
**Status**: ✅ **COMPLETED** - JWT tokens now working with persistent signing keys
**Priority**: CRITICAL - Blocking all auth features

---

## ✅ MIGRATION COMPLETE

**Date Completed**: 2025-10-18

### What Was Accomplished

1. ✅ **Signing Key Persistence**: Keys now load from disk (`OIDC_SIGNING_KEY_PATH`)
2. ✅ **JWT Token Generation**: zitadel/oidc library creates JWTs automatically when `AccessTokenType() = JWT`
3. ✅ **Configuration Verified**: `AccessTokenType()` = JWT, `GetAudience()` = ["gridapi"]
4. ✅ **Server Startup**: Successfully starts without JWKS errors
5. ✅ **Token Verification**: JWT tokens have correct structure (header.payload.signature) and claims

### Key Discovery

The zitadel/oidc library **handles JWT creation internally** when `AccessTokenType() = JWT`:
- Library uses our RSA key from `provider.Crypto()`
- Library generates `jti` internally (as a nested JWT)
- Our `createJWT()` method is NOT dead code - it's part of the required `op.Storage` interface
- No manual JWT signing needed beyond providing the signing key

---

## Overview

The current implementation issues **opaque tokens** for service accounts in internal IdP mode, which are stored in-memory and lost on server restart. This document provides a step-by-step action path to migrate to **JWT tokens** with JTI-based revocation.

### What Changed

| Component | Current (Wrong) | Target (Correct) |
|-----------|----------------|------------------|
| **Token Type** | Opaque (random hex) | JWT (signed, self-contained) |
| **Storage** | In-memory map | No token storage (JTI in DB for revocation) |
| **Validation** | Session lookup on every request | JWT signature + JTI denylist check |
| **Revocation** | Delete from map | Insert JTI into `revoked_jti` table |

---

## Phase 1: Database Schema (T001A)

### Task: Create `revoked_jti` Table

**File**: `cmd/gridapi/internal/migrations/YYYYMMDDHHMMSS_revoked_jti.go`

#### Before You Start
1. **Read existing migration patterns**:
   - Check `cmd/gridapi/internal/migrations/20251013140500_auth_tables.go`
   - Understand how `bun.DB` migrations work
   - Note the `Up()` and `Down()` method structure

#### Implementation

```go
package migrations

import (
	"context"
	"database/sql"
	"github.com/uptrace/bun"
)

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		// Create revoked_jti table
		_, err := db.ExecContext(ctx, `
			CREATE TABLE IF NOT EXISTS revoked_jti (
				jti TEXT PRIMARY KEY,
				subject TEXT NOT NULL,
				exp TIMESTAMP NOT NULL,
				revoked_at TIMESTAMP NOT NULL DEFAULT NOW(),
				revoked_by TEXT
			)
		`)
		if err != nil {
			return err
		}

		// Create index on exp for cleanup queries
		_, err = db.ExecContext(ctx, `
			CREATE INDEX IF NOT EXISTS idx_revoked_jti_exp ON revoked_jti(exp)
		`)
		return err
	}, func(ctx context.Context, db *bun.DB) error {
		// Rollback
		_, err := db.ExecContext(ctx, `DROP TABLE IF EXISTS revoked_jti`)
		return err
	})
}
```

#### Verification
- Run migration: `./bin/gridapi db migrate --db-url="postgres://..."`
- Check table exists: `psql -c "\d revoked_jti"`
- Verify index: `psql -c "\di revoked_jti*"`

#### Model (Optional)
Create `cmd/gridapi/internal/db/models/revoked_jti.go`:

```go
package models

import "time"

type RevokedJTI struct {
	JTI        string    `bun:",pk"`
	Subject    string    `bun:",notnull"`
	Exp        time.Time `bun:",notnull"`
	RevokedAt  time.Time `bun:",notnull,default:current_timestamp"`
	RevokedBy  string
}
```

---

## Phase 2: JWT Token Issuance (T009A)

### Task: Modify `oidc.go` to Issue JWTs Instead of Opaque Tokens

**File**: `cmd/gridapi/internal/auth/oidc.go`

#### Before You Start

1. **Read the current implementation** (`cmd/gridapi/internal/auth/oidc.go`):
   - Line 206-228: `createJWT()` method **already creates JWTs** ✅
   - Line 230-242: `CreateAccessToken()` **already calls createJWT()** ✅
   - Line 244-286: `CreateAccessAndRefreshTokens()` **already calls createJWT()** ✅
   - Line 237, 278: Both methods call `persistSession(ctx, request, claims.Claims.ID, ...)` - storing **JTI** as token hash

2. **Key observations**:
   - JWT creation is **already implemented** using go-jose
   - JTI (`claims.Claims.ID`) is **already generated** (line 214: `uuid.NewString()`)
   - JWT is **already signed** with RSA key (lines 218-223)
   - Session persistence **already stores JTI** (lines 237, 278, 805)

#### What Actually Needs to Change

**NOTHING in oidc.go token creation!** 🎉

The code **already issues JWTs**. The problem is elsewhere:
- ❌ `jwt.go` line 158-162: Verifier bypasses JWT validation for internal IdP
- ❌ `authn.go` lines 49-76: Middleware tries to handle opaque tokens as fallback

#### Verification Steps

1. **Confirm JWT structure**:
   ```bash
   # After gridapi runs, check token from CLI
   TOKEN=$(jq -r '.access_token' ~/.grid/credentials.json)
   echo $TOKEN | awk -F. '{print NF}'  # Should output: 3 (header.payload.signature)
   ```

2. **Decode JWT claims**:
   ```bash
   # Decode payload (middle part)
   echo $TOKEN | awk -F. '{print $2}' | base64 -d 2>/dev/null | jq .
   # Should show: iss, sub, aud, exp, iat, jti
   ```

3. **Verify JTI in database**:
   ```sql
   SELECT token_hash FROM sessions ORDER BY created_at DESC LIMIT 1;
   -- Compare with: echo -n "<jti-from-token>" | sha256sum
   ```

#### Configuration Enhancement (Optional)

Update `config.go` to support configurable token TTL (for Terraform):

```go
// In cmd/gridapi/internal/config/config.go
type OIDCConfig struct {
	// ... existing fields ...
	TerraformTokenTTL time.Duration `env:"TERRAFORM_TOKEN_TTL" envDefault:"120m"`
}

// In oidc.go, update CreateAccessToken:
func (s *providerStorage) CreateAccessToken(ctx context.Context, request op.TokenRequest) (string, time.Time, error) {
	ttl := defaultAccessTokenTTL

	// Check if this is a service account (client credentials)
	if _, ok := request.(*clientCredentialsTokenRequest); ok {
		// Use longer TTL for Terraform (configured in T003)
		if s.terraformTokenTTL > 0 {
			ttl = s.terraformTokenTTL
		}
	}

	exp := time.Now().Add(ttl)
	token, claims, err := s.createJWT(request, exp)
	// ... rest unchanged
}
```

#### **IMPORTANT: Dead Code Note**

When `AccessTokenType() = JWT`, the zitadel/oidc library handles JWT creation **internally**:
- ❌ **Our custom `createJWT()` method (if present) is NOT used** - it's dead code
- ❌ **Manual go-jose signing setup** - NOT USED by the library
- ✅ **The library calls our RSA key** via `provider.Crypto()` (line 80)
- ✅ **The library calls our trait methods**: `GetSubject()`, `GetAudience()`, `GetScopes()` to populate claims
- ✅ **The library generates its own `jti`** internally
- ✅ **The library signs the JWT** with our RSA private key
- ✅ **The library exposes JWKS** via `/keys` endpoint with our public key

**What This Means**:
1. The signing key setup (T003A) is **STILL CRITICAL** - the library uses `providerStorage.signingKey`
2. Our custom `createJWT()` method can be **removed or ignored** - it will never be called
3. The persistent key loading issue is **STILL CRITICAL** - random keys invalidate tokens on restart

**Correct Implementation**:
- Set `AccessTokenType()` to return `op.AccessTokenTypeJWT` (not `op.AccessTokenTypeBearer`)
- Fix `GetAudience()` to return correct audience (e.g., `[]string{"gridapi"}`)
- Load persistent signing key from disk (T003A)
- Let the library handle the rest

---

## Phase 3: JWT Validation (T010A)

### Task: Enable Universal JWT Validation

**File**: `cmd/gridapi/internal/auth/jwt.go`

#### Before You Start

1. **Read current implementation** (`cmd/gridapi/internal/auth/jwt.go`):
   - Lines 86-171: `NewVerifier()` function
   - Lines 122-124: **Problem area** - `WithLazyLoadJwks(true)` for internal provider
   - Lines 158-162: JWT parsing **already works** via `tokenHandler.ParseToken()`
   - Line 166: **Already extracts JTI** and hashes it: `TokenHashFromContext()`

2. **What's working**:
   - JWT validation logic exists (go-oidc-middleware)
   - JTI extraction exists (line 166)
   - Claims storage in context exists (line 164)

3. **What's broken**:
   - No explicit check that JTI extraction succeeded
   - Hash is computed but JTI might be missing/nil

#### Changes Required

**Minimal fix** - Add JTI presence validation:

```go
// In NewVerifier(), after line 162 (token parsing)

claims, err := tokenHandler.ParseToken(r.Context(), trimmedToken)
if err != nil {
	vOpts.errorResponder(w, r, fmt.Errorf("invalid token: %w", err))
	return
}

// ADD THIS: Validate JTI claim exists
jti, ok := claims["jti"].(string)
if !ok || jti == "" {
	vOpts.errorResponder(w, r, fmt.Errorf("token missing jti claim"))
	return
}

ctx := context.WithValue(r.Context(), defaultClaimsContextKey, claims)
ctx = context.WithValue(ctx, defaultTokenStringContextKey, trimmedToken)
ctx = context.WithValue(ctx, defaultTokenHashContextKey, HashToken(jti)) // Use extracted JTI

next.ServeHTTP(w, r.WithContext(ctx))
```

#### Verification

1. **Test with valid JWT**:
   ```bash
   TOKEN=$(jq -r '.access_token' ~/.grid/credentials.json)
   curl -H "Authorization: Bearer $TOKEN" http://localhost:8080/state.v1.StateService/ListStates
   # Should succeed
   ```

2. **Test with JWT missing JTI**:
   ```bash
   # Create JWT without JTI using jwt.io, try to use it
   # Should fail with "token missing jti claim"
   ```

3. **Test with malformed token**:
   ```bash
   curl -H "Authorization: Bearer invalid-token" http://localhost:8080/state.v1.StateService/ListStates
   # Should fail with "invalid token"
   ```

---

## Phase 4: Revocation Check (T017A)

### Task: Add JTI Denylist Check in AuthN Middleware

**File**: `cmd/gridapi/internal/middleware/authn.go`

#### Before You Start

1. **Read current implementation** (`cmd/gridapi/internal/middleware/authn.go`):
   - Lines 29-42: Middleware setup with dependencies
   - Lines 44-74: **Complex dual-path logic** - handles both JWT claims AND raw tokens
   - Lines 49-51: Checks for JWT claims from context
   - Lines 54-68: **Fallback logic** - tries to extract token manually if no claims
   - Lines 76-141: Session lookup and validation

2. **Current flow** (WRONG):
   ```
   Request → Try get claims → If no claims, try get token string →
   Hash token → Lookup session by hash → Validate session
   ```

3. **Target flow** (CORRECT):
   ```
   Request → JWT validated (jwt.go) → Claims in context (guaranteed) →
   Extract JTI → Check revoked_jti table → If not revoked, continue
   ```

#### Changes Required

**Complete refactor** of lines 44-141:

```go
func NewAuthnMiddleware(cfg *config.Config, deps AuthnDependencies, verifierOpts ...auth.VerifierOption) (func(http.Handler) http.Handler, error) {
	// ... validation unchanged ...

	verifier, err := auth.NewVerifier(cfg, verifierOpts...)
	if err != nil {
		return nil, fmt.Errorf("initialise oidc verifier: %w", err)
	}

	return func(next http.Handler) http.Handler {
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			// STEP 1: Get validated claims (set by jwt.go middleware)
			claims, hasClaims := auth.ClaimsFromContext(ctx)
			if !hasClaims {
				// No claims = public route (skipped by verifier)
				next.ServeHTTP(w, r)
				return
			}

			// STEP 2: Extract JTI from claims
			jti, ok := claims["jti"].(string)
			if !ok || jti == "" {
				// Should never happen (jwt.go validates this)
				http.Error(w, "invalid token: missing jti", http.StatusUnauthorized)
				return
			}

			// STEP 3: Check JTI revocation (new query)
			isRevoked, err := isJTIRevoked(ctx, deps, jti)
			if err != nil {
				// Log error but don't expose details
				http.Error(w, "authentication error", http.StatusInternalServerError)
				return
			}
			if isRevoked {
				http.Error(w, "token has been revoked", http.StatusUnauthorized)
				return
			}

			// STEP 4: Extract subject and check identity disabled
			subject, _ := claims["sub"].(string)
			if subject == "" {
				http.Error(w, "invalid token: missing subject", http.StatusUnauthorized)
				return
			}

			// Check if identity is disabled
			identityDisabled, err := checkIdentityDisabled(ctx, deps, subject)
			if err != nil {
				http.Error(w, "authentication error", http.StatusInternalServerError)
				return
			}
			if identityDisabled {
				http.Error(w, "account disabled", http.StatusUnauthorized)
				return
			}

			// STEP 5: Extract groups and apply dynamic Casbin grouping
			groups := extractGroupsFromClaims(claims, cfg)
			if err := applyDynamicGroupings(ctx, deps.Enforcer, deps.GroupRoles, subject, groups); err != nil {
				http.Error(w, "authorization setup failed", http.StatusInternalServerError)
				return
			}

			// STEP 6: Store principal in context for authz middleware
			ctx = auth.SetPrincipalContext(ctx, subject)
			ctx = auth.SetGroupsContext(ctx, groups)

			next.ServeHTTP(w, r.WithContext(ctx))
		})

		// Chain verifier → handler
		return verifier(handler)
	}, nil
}

// Helper: Check if JTI is in revoked_jti table
func isJTIRevoked(ctx context.Context, deps AuthnDependencies, jti string) (bool, error) {
	// Query revoked_jti table
	var count int
	err := deps.Sessions.DB().NewSelect().
		Table("revoked_jti").
		Where("jti = ?", jti).
		Scan(ctx, &count)

	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// Helper: Check if user/SA is disabled
func checkIdentityDisabled(ctx context.Context, deps AuthnDependencies, subject string) (bool, error) {
	// Check if service account
	if strings.HasPrefix(subject, "sa:") {
		clientID := strings.TrimPrefix(subject, "sa:")
		sa, err := deps.ServiceAccounts.GetByClientID(ctx, clientID)
		if err != nil {
			return false, err
		}
		return sa.Disabled, nil
	}

	// Check if user
	user, err := deps.Users.GetBySubject(ctx, subject)
	if err != nil {
		return false, err
	}
	return user.DisabledAt != nil, nil
}

// Helper: Extract groups from claims
func extractGroupsFromClaims(claims map[string]interface{}, cfg *config.Config) []string {
	// Use existing auth.ExtractGroups() from claims.go
	groups, _ := auth.ExtractGroups(claims, cfg.OIDC.GroupsClaimField, cfg.OIDC.GroupsClaimPath)
	return groups
}

// Helper: Apply dynamic groupings
func applyDynamicGroupings(ctx context.Context, enforcer casbin.IEnforcer, groupRoles repository.GroupRoleRepository, userID string, groups []string) error {
	// Use existing auth.ApplyDynamicGroupings() from grouping.go
	groupRoleMap, err := groupRoles.GetRolesForGroups(ctx, groups)
	if err != nil {
		return err
	}
	return auth.ApplyDynamicGroupings(enforcer, userID, groups, groupRoleMap)
}
```

#### What Was Removed

1. **Lines 54-68**: Manual token extraction fallback (NOT NEEDED)
2. **Lines 76-141**: Session lookup by token hash (NOT NEEDED for JWT validation)
3. **Lines 81-130**: External IdP user creation on first request (MOVE to separate endpoint)

#### What's New

1. **JTI revocation check**: Query `revoked_jti` table (required)
2. **Identity disabled check**: Check `users.disabled_at` or `service_accounts.disabled` (required)
3. **Simplified flow**: No session lookup, no opaque token handling

#### Verification

1. **Test valid JWT**:
   ```bash
   TOKEN=$(jq -r '.access_token' ~/.grid/credentials.json)
   curl -H "Authorization: Bearer $TOKEN" http://localhost:8080/state.v1.StateService/ListStates
   # Should succeed
   ```

2. **Test revoked JWT**:
   ```sql
   -- Insert JTI into revoked_jti
   INSERT INTO revoked_jti (jti, subject, exp)
   VALUES ('<jti-from-token>', 'sa:client-id', NOW() + INTERVAL '1 hour');
   ```
   ```bash
   curl -H "Authorization: Bearer $TOKEN" http://localhost:8080/state.v1.StateService/ListStates
   # Should fail with "token has been revoked"
   ```

3. **Test disabled service account**:
   ```sql
   UPDATE service_accounts SET disabled = true WHERE client_id = '...';
   ```
   ```bash
   curl -H "Authorization: Bearer $TOKEN" http://localhost:8080/state.v1.StateService/ListStates
   # Should fail with "account disabled"
   ```

---

## Phase 5: Cleanup Session Table (Optional)

### Task: Update Session Schema to Store JTI Instead of Token Hash

**Current**: Sessions store `token_hash` (SHA256 of full JWT)
**Target**: Sessions store `jti` directly (cleaner, more explicit)

#### Migration

```sql
-- Option 1: Keep token_hash column but store JTI hash
-- No schema change needed, just update oidc.go line 805:
--   tokenHash := auth.HashBearerToken(accessToken)  // OLD
--   tokenHash := accessToken  // NEW (already stores JTI per line 237, 278)

-- Option 2: Rename column for clarity
ALTER TABLE sessions RENAME COLUMN token_hash TO jti;
```

#### Code Update

In `oidc.go`, line 805:
```go
// OLD:
tokenHash := HashBearerToken(accessToken)

// NEW:
jti := accessToken  // accessToken parameter is already the JTI (from lines 237, 278)
```

---

## Integration Testing

### Test Scenarios (from quickstart.md)

1. **Scenario 3 (Mode 2)**: Service account authentication
   ```bash
   # Bootstrap SA
   ./bin/gridapi sa create --name test-sa

   # Get token
   TOKEN=$(curl -X POST http://localhost:8080/oauth/token \
     -d "grant_type=client_credentials" \
     -d "client_id=<id>" \
     -d "client_secret=<secret>" | jq -r .access_token)

   # Verify JWT structure
   echo $TOKEN | awk -F. '{print NF}'  # Should be 3

   # Decode and check claims
   echo $TOKEN | awk -F. '{print $2}' | base64 -d | jq .
   # Should have: iss, sub, aud, exp, iat, jti
   ```

2. **Scenario 6**: Service account revocation
   ```bash
   # Use token
   curl -H "Authorization: Bearer $TOKEN" http://localhost:8080/tfstate/<guid>
   # Should succeed

   # Revoke SA
   ./bin/gridctl sa revoke --client-id <id>

   # Try again
   curl -H "Authorization: Bearer $TOKEN" http://localhost:8080/tfstate/<guid>
   # Should fail with 401
   ```

3. **JTI Revocation**:
   ```bash
   # Extract JTI
   JTI=$(echo $TOKEN | awk -F. '{print $2}' | base64 -d | jq -r .jti)

   # Insert into denylist
   psql -c "INSERT INTO revoked_jti (jti, subject, exp) VALUES ('$JTI', 'sa:test', NOW() + INTERVAL '1 hour')"

   # Try to use token
   curl -H "Authorization: Bearer $TOKEN" http://localhost:8080/tfstate/<guid>
   # Should fail with "token has been revoked"
   ```

---

## Rollback Plan

If migration fails:

1. **Revert authn.go**: Restore dual-path logic (git revert)
2. **Revert jwt.go**: Restore bypass for internal IdP (git revert)
3. **Drop revoked_jti table**: Run down migration
4. **Restart server**: Opaque tokens work in-memory again

---

## Summary of Changes

| File | Lines | Change | Complexity |
|------|-------|--------|-----------|
| `migrations/*` | New file | Add `revoked_jti` table | Low |
| `oidc.go` | None | **Already issues JWTs** ✅ | None |
| `jwt.go` | 163-167 | Add JTI validation | Low |
| `authn.go` | 44-141 | **Complete refactor** (remove 98 lines, add ~80) | High |
| `config.go` | +2 lines | Add `TerraformTokenTTL` (optional) | Low |

**Total Impact**: ~100 lines changed, 3 files touched (4 with config)

**Testing**: 3 integration tests from quickstart.md scenarios

**Risk**: Medium (authn.go refactor is significant, but isolated to one file)
