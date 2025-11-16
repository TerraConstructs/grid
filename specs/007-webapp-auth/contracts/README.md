# API Contracts: WebApp User Login Flow

**Feature**: 007-webapp-auth
**Date**: 2025-11-04

## Overview

This document describes the HTTP and Connect RPC contracts used by the webapp authentication feature. **All contracts are already implemented in gridapi** - no new endpoints are required. This documentation serves as reference for the webapp implementation.

## Authentication Discovery

### GET /health

Health check endpoint that indicates if authentication is enabled.

**Request**: None

**Response**: 200 OK

```json
{
  "status": "ok",
  "oidc_enabled": true
}
```

**Fields**:
- `status`: Always "ok" when server is healthy
- `oidc_enabled`: Boolean indicating if OIDC authentication is enabled

**Usage**: Webapp calls this on initial load to determine if authentication UI should be shown.

---

### GET /auth/config

Authentication configuration discovery endpoint.

**Request**: None (no authentication required)

**Response**: 200 OK

```json
{
  "mode": "external-idp",
  "issuer": "https://keycloak.example.com/realms/grid",
  "client_id": "grid-cli-public",
  "audience": "grid-api",
  "supports_device_flow": true
}
```

**Fields**:
- `mode`: String, one of `"internal-idp"` or `"external-idp"`
  - `"internal-idp"`: Grid's built-in IdP (username/password for web, service accounts for CLI)
  - `"external-idp"`: External OIDC provider (SSO for web, device flow for CLI)
- `issuer`: String (optional), OIDC issuer URL. Only present when `mode === "external-idp"`.
- `client_id`: String (optional), public OAuth2 client ID for CLI device flow. Not used by webapp.
- `audience`: String (optional), expected `aud` claim in access tokens. Used for server-side validation.
- `supports_device_flow`: Boolean, indicates if CLI device flow is available. Not used by webapp (webapp uses authorization code flow).

**Usage**: Webapp calls this on initial load to determine which login UI to render (username/password form vs SSO button).

**Implementation**: `cmd/gridapi/internal/server/auth_handlers.go:HandleAuthConfig()`

---

## Authentication Flow (Internal IdP Mode)

### POST /auth/login (Internal IdP)

Initiates authentication with internal IdP using username/password.

**Request**: POST application/json

```json
{
  "username": "admin@internal",
  "password": "admin-secure-pass-123"
}
```

**Response**: 200 OK

Sets httpOnly cookie: `grid_session`

```json
{
  "user": {
    "id": "user-uuid",
    "username": "admin",
    "email": "admin@internal.grid",
    "auth_type": "internal",
    "roles": ["admin", "editor"]
  },
  "expires_at": 1730793600000
}
```

**Error Responses**:

- 401 Unauthorized: Invalid credentials
  ```json
  {
    "error": "invalid_credentials",
    "error_description": "Invalid username or password"
  }
  ```

- 400 Bad Request: Missing fields
  ```json
  {
    "error": "invalid_request",
    "error_description": "username and password are required"
  }
  ```

**Side Effects**:
- Creates session in database
- Sets httpOnly session cookie with 2-hour expiry
- Cookie attributes: `HttpOnly`, `Secure` (HTTPS only), `SameSite=Lax`

**Usage**: LoginPage submits credentials when user selects internal IdP mode.

**Implementation**: `cmd/gridapi/internal/server/auth_handlers.go` (internal IdP login handler)

---

## Authentication Flow (External IdP Mode)

### GET /auth/login (External IdP)

Initiates OAuth2 authorization code flow with external IdP.

**Request**: GET with optional query parameters

Query Parameters:
- `redirect_uri` (optional): Where to redirect after successful authentication. Defaults to webapp root.

**Response**: 302 Found (Redirect)

Redirects to external IdP authorization endpoint with PKCE parameters.

Example redirect:
```
Location: https://keycloak.example.com/realms/grid/protocol/openid-connect/auth?
  response_type=code&
  client_id=grid-web-app&
  redirect_uri=https://grid.example.com/auth/callback&
  scope=openid profile email groups&
  state=cryptographic-random-state&
  code_challenge=base64url-sha256-hash&
  code_challenge_method=S256
```

**Side Effects**:
- Generates PKCE code verifier, stores in server-side session
- Generates cryptographic random state parameter, stores in httpOnly cookie
- State cookie attributes: `HttpOnly`, `Secure`, `SameSite=Lax`, short expiry (5 minutes)

**Usage**: Webapp redirects user to this endpoint when SSO button is clicked.

**Implementation**: `cmd/gridapi/internal/server/auth_handlers.go` (external IdP initiation handler)

---

### GET /auth/callback (External IdP)

Handles OAuth2 authorization code callback from external IdP.

**Request**: GET with query parameters (provided by IdP)

Query Parameters:
- `code`: Authorization code from IdP
- `state`: State parameter for CSRF protection

**Response**: 302 Found (Redirect to webapp)

Sets httpOnly cookie: `grid_session`

Redirects to: `${redirect_uri}` (from original `/auth/login` request)

**Error Responses**:

- 400 Bad Request: State mismatch (CSRF)
  ```json
  {
    "error": "invalid_state",
    "error_description": "State parameter mismatch. Possible CSRF attack."
  }
  ```

- 401 Unauthorized: Code exchange failed
  ```json
  {
    "error": "authentication_failed",
    "error_description": "Failed to exchange authorization code for tokens"
  }
  ```

**Side Effects**:
- Exchanges authorization code for tokens with IdP
- Validates ID token (signature, issuer, audience, expiry)
- Extracts user claims (sub, email, name, groups)
- Maps groups to roles via group-role mappings in database
- Creates session in database
- Sets httpOnly session cookie with 2-hour expiry
- Clears state cookie

**Usage**: User is redirected here by external IdP after successful login. Browser automatically calls this endpoint.

**Implementation**: `cmd/gridapi/internal/server/auth_handlers.go` (external IdP callback handler)

---

## Session Management

### GET /api/auth/whoami ⚠️ **REQUIRES IMPLEMENTATION**

Returns current authenticated user's identity and session information. This is the primary endpoint for webapp session restoration.

**Request**: GET (no body, authenticated via cookie)

Requires: Valid `grid_session` cookie

**Response**: 200 OK

```json
{
  "user": {
    "id": "user-uuid",
    "subject": "sub-claim-from-jwt",
    "username": "admin",
    "email": "admin@internal.grid",
    "auth_type": "internal",
    "roles": ["admin", "editor"],
    "groups": ["dev-team", "platform"]
  },
  "session": {
    "id": "session-uuid",
    "expires_at": 1730800800,
    "created_at": 1730793600
  }
}
```

**Error Responses**:

- 401 Unauthorized: Session invalid or expired
  ```json
  {
    "error": "unauthenticated",
    "error_description": "Session expired or invalid"
  }
  ```

**Usage**: Webapp calls this on page load to restore authentication context. Since session cookie is httpOnly (not readable by JavaScript), this is the only way for webapp to fetch user identity and session expiry.

**Implementation Status**: ⚠️ **DOES NOT EXIST YET**

**Required Backend Changes** (see `specs/007-webapp-auth/plan.md` Complexity Tracking):
1. Add `HandleWhoAmI()` function in `cmd/gridapi/internal/server/auth_handlers.go`
2. Fix bug: Populate `AuthenticatedPrincipal.SessionID` in `cmd/gridapi/internal/middleware/authn.go`
3. Mount endpoint: `r.Get("/api/auth/whoami", HandleWhoAmI(&opts.AuthnDeps))` in `cmd/gridapi/internal/server/router.go`
4. Verify repository methods: `session_repository.go` needs `GetByID()` and `GetByTokenHash()`

---

### POST /auth/refresh ⚠️ **NOTE: NOT FOR WEB SESSIONS**

This endpoint exists but is **NOT used for web session management**. It's intended for service account token refresh.

For webapp session restoration, use `/api/auth/whoami` instead (see above).

**Request**: POST (no body, authenticated via cookie)

Requires: Valid `grid_session` cookie

**Response**: 200 OK

Updates `grid_session` cookie expiry

```json
{
  "user": {
    "id": "user-uuid",
    "username": "admin",
    "email": "admin@internal.grid",
    "auth_type": "internal",
    "roles": ["admin", "editor"]
  },
  "expires_at": 1730800800000
}
```

**Error Responses**:

- 401 Unauthorized: Session invalid or expired
  ```json
  {
    "error": "unauthenticated",
    "error_description": "Session expired or invalid"
  }
  ```

**Side Effects**:
- Validates existing session cookie
- Extends session expiry in database (2 hours from now)
- Updates httpOnly cookie expiry

**Usage**: Webapp can call this periodically to keep session alive. Called on page load to restore session.

**Implementation**: `cmd/gridapi/internal/server/auth_handlers.go` (refresh handler)

---

### POST /auth/logout

Logs out the current user, destroying the session.

**Request**: POST (no body, authenticated via cookie)

Requires: Valid `grid_session` cookie

**Response**: 200 OK

Clears `grid_session` cookie

```json
{
  "message": "Logged out successfully"
}
```

**Side Effects**:
- Deletes session from database
- Adds JWT ID (JTI) to revoked tokens table
- Clears httpOnly session cookie (sets `max-age=0`)

**Usage**: Webapp calls this when user clicks "Sign Out" in AuthStatus menu.

**Implementation**: `cmd/gridapi/internal/server/auth_handlers.go` (logout handler)

---

## Protected Endpoints

All Connect RPC endpoints require authentication when OIDC is enabled. The webapp consumes these via `js/sdk` which uses `@connectrpc/connect-web`.

### Common Authentication Behavior

**Authentication**: Automatic via httpOnly cookies. Browser sends `grid_session` cookie with all requests.

**Authorization**: Server-side middleware checks:
1. Cookie present and valid
2. Session not expired
3. JWT not revoked (JTI check)
4. User has required permissions for the operation (Casbin + label-based scoping)

**Error Response**: 401 Unauthenticated

Connect RPC error format:

```json
{
  "code": "unauthenticated",
  "message": "Authentication required"
}
```

**Webapp Handling**:
- Connect RPC interceptor detects `Code.Unauthenticated` (401)
- Triggers logout and redirect to login page
- Preserves return URL (optional, marked as out-of-scope for this feature)

### Example: ListStates RPC

**Proto Definition** (reference only, already exists):

```protobuf
service StateService {
  rpc ListStates(ListStatesRequest) returns (ListStatesResponse);
}

message ListStatesRequest {
  // Empty for now - future: add label filters
}

message ListStatesResponse {
  repeated StateInfo states = 1;
}
```

**Connect RPC Usage** (js/sdk):

```typescript
import { createConnectTransport } from '@connectrpc/connect-web';
import { StateService } from './gen/state/v1/state_connect';

const transport = createConnectTransport({
  baseUrl: 'https://grid.example.com',
  credentials: 'include', // Send cookies
});

const client = createPromiseClient(StateService, transport);

// Automatic authentication via cookie
const response = await client.listStates({});
```

**Server-side Filtering** (future enhancement):

Currently, `ListStates` returns all states and relies on client-side filtering based on role scope. This is a known limitation tracked in issue `grid-f5947b22`. Future enhancement will add server-side label filtering based on user's role scope expression.

---

## Authentication State Diagram

```
┌─────────────────────────────────────────────────────────────────┐
│                        Webapp Initialization                    │
└────────────────┬────────────────────────────────────────────────┘
                 │
                 ▼
         ┌───────────────┐
         │ GET /health   │ ───→ oidc_enabled: false ───→ Skip Auth
         └───────┬───────┘
                 │
                 │ oidc_enabled: true
                 ▼
         ┌───────────────────┐
         │ GET /auth/config  │
         └────────┬──────────┘
                  │
           ┌──────┴──────┐
           │             │
    mode: internal    mode: external
           │             │
           ▼             ▼
  ┌────────────────┐  ┌─────────────────────┐
  │ LoginPage:     │  │ LoginPage:          │
  │ Username/Pass  │  │ SSO Button          │
  └────┬───────────┘  └──────┬──────────────┘
       │                     │
       │ POST /auth/login    │ GET /auth/login (redirect)
       ▼                     ▼
  ┌────────────────┐  ┌──────────────────────────────┐
  │ Session Cookie │  │ External IdP (Keycloak, etc) │
  │ Set            │  └────────┬─────────────────────┘
  └────┬───────────┘           │
       │                       │ User authenticates
       │                       ▼
       │              ┌──────────────────────┐
       │              │ GET /auth/callback   │
       │              │ (code exchange)      │
       │              └────────┬─────────────┘
       │                       │
       │                       ▼
       │              ┌────────────────┐
       │              │ Session Cookie │
       │              │ Set            │
       │              └────────┬───────┘
       └──────────────┬────────┘
                      │
                      ▼
              ┌───────────────┐
              │ Dashboard      │
              │ (authenticated)│
              └───────┬────────┘
                      │
                      ├─→ Connect RPC calls (with cookie)
                      │
                      ├─→ 401 Response ───→ Logout + Login Page
                      │
                      └─→ POST /auth/logout ───→ Login Page
```

---

## Security Considerations

### CSRF Protection

- **State parameter**: Generated by server, stored in httpOnly cookie, validated on callback
- **SameSite cookies**: Set to `Lax` to prevent CSRF
- **Origin validation**: Server validates `Origin` and `Referer` headers

### XSS Protection

- **HttpOnly cookies**: JavaScript cannot read session tokens
- **CSP headers**: Content Security Policy prevents inline scripts
- **React auto-escaping**: All user input is automatically escaped in JSX

### Token Security

- **No tokens in localStorage**: All tokens stored in httpOnly cookies
- **Secure transmission**: Cookies marked `Secure` (HTTPS only)
- **Short expiry**: Sessions expire after 2 hours
- **JTI revocation**: Logout adds JWT ID to revoked tokens list

### PKCE (External IdP)

- **Code challenge**: SHA-256 hash of random code verifier
- **Code verifier**: Stored server-side, never sent to browser
- **Prevents authorization code interception attacks**

---

## Implementation References

All endpoints are already implemented in gridapi:

- **Health**: `cmd/gridapi/cmd/serve.go:healthHandler()`
- **Auth Config**: `cmd/gridapi/internal/server/auth_handlers.go:HandleAuthConfig()`
- **Internal IdP Login**: `cmd/gridapi/internal/server/auth_handlers.go` (POST handler)
- **External IdP Login**: `cmd/gridapi/internal/server/auth_handlers.go` (GET redirect handler)
- **External IdP Callback**: `cmd/gridapi/internal/server/auth_handlers.go` (callback handler)
- **Refresh**: `cmd/gridapi/internal/server/auth_handlers.go:HandleRefresh()`
- **Logout**: `cmd/gridapi/internal/server/auth_handlers.go:HandleLogout()`
- **Connect RPC Auth**: `cmd/gridapi/internal/middleware/authn_interceptor.go`

Refer to `specs/007-webapp-auth/research.md` for detailed implementation patterns.
