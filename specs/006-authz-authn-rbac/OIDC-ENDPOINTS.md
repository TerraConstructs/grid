# OIDC Endpoints Reference

This document provides the canonical list of OIDC endpoints exposed by Grid's Internal IdP (Mode 2).

## Discovery Document

**Endpoint**: `GET /.well-known/openid-configuration`

Returns the OpenID Connect discovery document with all available endpoints.

## Authentication Endpoints (Auto-Mounted by zitadel/oidc)

These endpoints are automatically mounted by the `op.CreateRouter()` function from zitadel/oidc library:

### Token Endpoint
- **Path**: `/oauth/token` (NOT `/token`)
- **Method**: POST
- **Content-Type**: `application/x-www-form-urlencoded`
- **Grant Types**:
  - `client_credentials` - Service account authentication
  - `authorization_code` - Web SSO flow
  - `refresh_token` - Token refresh
  - `urn:ietf:params:oauth:grant-type:device_code` - Device code flow (CLI)

**Example (Client Credentials)**:
```bash
curl -X POST http://localhost:8080/oauth/token \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -d "grant_type=client_credentials" \
  -d "client_id=<client-id>" \
  -d "client_secret=<client-secret>"
```

### Device Authorization Endpoint
- **Path**: `/device_authorization` (NOT `/auth/device/code`)
- **Method**: POST
- **Purpose**: Initiate device code flow for CLI authentication

### Authorization Endpoint
- **Path**: `/authorize`
- **Method**: GET
- **Purpose**: Web-based authorization code flow

### Revocation Endpoint
- **Path**: `/revoke`
- **Method**: POST
- **Purpose**: Revoke refresh tokens

### Introspection Endpoint
- **Path**: `/oauth/introspect`
- **Method**: POST
- **Purpose**: Token introspection

### UserInfo Endpoint
- **Path**: `/userinfo`
- **Method**: GET
- **Purpose**: Retrieve user information claims

### JWKS Endpoint
- **Path**: `/keys` (NOT `/jwks` or `/.well-known/jwks.json`)
- **Method**: GET
- **Purpose**: JSON Web Key Set for JWT signature verification

### End Session Endpoint
- **Path**: `/end_session`
- **Method**: GET/POST
- **Purpose**: Logout/session termination

## Custom Endpoints (Grid-Specific)

These are NOT auto-mounted by zitadel/oidc. They are custom handlers in `cmd/gridapi/internal/server/auth_handlers.go`:

### SSO Login (External IdP Mode Only)
- **Path**: `/auth/sso/login`
- **Purpose**: Redirect to external IdP for SSO

### SSO Callback (External IdP Mode Only)
- **Path**: `/auth/sso/callback`
- **Purpose**: Handle OAuth callback from external IdP

### Logout
- **Path**: `/auth/logout`
- **Purpose**: Unified logout handler (both modes)

## Common Mistakes to Avoid

❌ **INCORRECT**: `/token` → ✅ **CORRECT**: `/oauth/token`
❌ **INCORRECT**: `/auth/device/code` → ✅ **CORRECT**: `/device_authorization`
❌ **INCORRECT**: `/auth/device/token` → ✅ **CORRECT**: `/oauth/token` (with device_code grant)
❌ **INCORRECT**: `/.well-known/jwks.json` → ✅ **CORRECT**: `/keys`

## Verification

Test the discovery document:
```bash
curl -s http://localhost:8080/.well-known/openid-configuration | jq '.token_endpoint'
# Should output: "http://localhost:8080/oauth/token"
```
