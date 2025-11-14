# Phase 3: Authenticator Pattern

**Priority**: P0
**Effort**: 6-8 hours
**Risk**: Medium
**Dependencies**: Phase 1, 2 complete

## Objectives

- Implement JWT and Session authenticators
- Create unified MultiAuth middleware
- Normalize both authentication paths to single Principal output

## Tasks

### Task 3.1: Implement JWTAuthenticator

**File**: `services/iam/jwt_auth.go`

**Logic**:
1. Extract "Authorization: Bearer <token>" header
2. Return (nil, nil) if not present
3. Verify JWT signature using existing `auth.Verifier`
4. Extract claims: sub, email, groups, jti
5. Check JTI revocation (call `revokedJTIs.IsRevoked()`)
6. Resolve user/service account (JIT provision if needed)
7. Call `iamService.ResolveRoles(userID, groups)` (uses immutable cache)
8. Construct Principal with all fields populated
9. Return Principal

**Dependencies**: Existing JWT verifier from `auth/jwt.go`

### Task 3.2: Implement SessionAuthenticator

**File**: `services/iam/session_auth.go`

**Logic**:
1. Extract "grid.session" cookie
2. Return (nil, nil) if not present
3. Hash cookie value (`auth.HashToken`)
4. Lookup session in DB (`sessions.GetByTokenHash`)
5. Validate: not revoked, not expired
6. Lookup user (`users.GetByID`)
7. Validate: not disabled
8. Extract groups from `session.id_token` (stored JWT)
9. Call `iamService.ResolveRoles(userID, groups)`
10. Construct Principal
11. Return Principal

### Task 3.3: Implement MultiAuth Middleware

**File**: `middleware/authn_multiauth.go`

**Logic**:
```go
func MultiAuthMiddleware(iamService iam.Service) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            authReq := iam.AuthRequest{
                Headers: r.Header,
                Cookies: r.Cookies(),
            }

            principal, err := iamService.AuthenticateRequest(ctx, authReq)
            if err != nil {
                http.Error(w, "Authentication failed", 401)
                return
            }

            if principal != nil {
                ctx = auth.SetUserContext(ctx, convertToLegacy(principal))
                ctx = auth.SetGroupsContext(ctx, principal.Groups)
            }

            next.ServeHTTP(w, r.WithContext(ctx))
        })
    }
}
```

**IAM Service Implementation**:
```go
func (s *iamService) AuthenticateRequest(ctx, req) (*Principal, error) {
    for _, authenticator := range s.authenticators {
        principal, err := authenticator.Authenticate(ctx, req)
        if err != nil {
            return nil, err // Authentication failed
        }
        if principal != nil {
            return principal, nil // Success!
        }
        // nil means "no credentials for this authenticator, try next"
    }
    return nil, nil // No valid credentials
}
```

### Task 3.4: Write Unit Tests

**Files**:
- `jwt_auth_test.go` - Test JWT validation, claim extraction, role resolution
- `session_auth_test.go` - Test session validation, user lookup, role resolution
- `authn_multiauth_test.go` - Test authenticator priority, fallback logic

## Deliverables

- [ ] JWTAuthenticator implemented and tested
- [ ] SessionAuthenticator implemented and tested
- [ ] MultiAuth middleware implemented and tested
- [ ] Both auth paths produce identical Principal struct
- [ ] Integration test showing both paths work

## Related Documents

- **Previous**: [phase-2-immutable-cache.md](phase-2-immutable-cache.md)
- **Next**: [phase-4-authorization-refactor.md](phase-4-authorization-refactor.md)
