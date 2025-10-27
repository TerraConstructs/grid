# Notes on Integration Tests

## Why does authenticateUserWithPassword (in tests/integration/auth_mode1_auth_helpers.go) require the ClientId and ClientSecret?

AuthenticateUserWithPassword requires both ClientId and ClientSecret:

### Why Password Grant Requires Client Credentials

Looking at `auth_mode1_auth_helpers.go`:74-119, the password grant flow requires both client credentials (client_id + client_secret) AND user credentials
(username + password). Here's why:

#### OAuth2/OIDC Password Grant Specification

The https://datatracker.ietf.org/doc/html/rfc6749#section-4.3 requires:

```console
POST /token HTTP/1.1
Content-Type: application/x-www-form-urlencoded

grant_type=password
&username=alice@example.com
&password=test123
&client_id=grid-api          ← Client authenticates the application
&client_secret=<secret>       ← Client secret proves it's the real app
```

#### Two-Level Authentication

1. Client Authentication (lines 85-86):
    - client_id + client_secret authenticate the application (Grid API)
    - Proves the token request is coming from a legitimate, registered OAuth2 client
    - Without this, any malicious app could request tokens for your users
2. User Authentication (lines 87-88):
    - username + password authenticate the user (alice@example.com)
    - Proves the actual end-user's identity

#### Why This Matters for Grid

From the critical comment on lines 92-93:

```golang
// CRITICAL: gridapi validates tokens with RequiredAudience=EXTERNAL_IDP_CLIENT_ID (see jwt.go:94, jwt.go:122)
// Without this, user tokens will be rejected with 401 even if signature/issuer are valid
```

The client_id also serves as the audience claim in the JWT (line 95):

```golang
data.Set("audience", clientID)
```

Grid's JWT validator requires tokens to have `aud=grid-api` to ensure the token was issued for Grid API specifically, not for some other application in the
same Keycloak realm.

#### Public vs Confidential Clients

There are two types of OAuth2 clients:

1. Confidential Clients (like Grid API):
    - Can securely store `client_secret`
    - Required to authenticate with both `client_id` + `client_secret`
    - Used for server-to-server or backend applications
2. Public Clients (like browser/mobile apps):
    - Cannot securely store secrets
    - Only use `client_id` (no `client_secret`)
    - Must use PKCE (Proof Key for Code Exchange) for security

Grid API is configured as a **confidential client** in Keycloak, hence the requirement for both credentials.

#### When Is client_secret Not Required?

The `client_secret` is optional in these scenarios:

- Public clients (browser SPAs, mobile apps) - but these should use Authorization Code + PKCE flow instead of password grant
- Client credentials flow when using client certificate authentication instead of shared secret

For password grant with confidential clients (like Grid's test scenario), both are required per OAuth2 spec.

### Security Note

The password grant is generally discouraged in modern OAuth2 because:

- Users must trust the client with their password
- No MFA/consent flow
- Less secure than authorization code flow

It's only used in the test suite for automated testing. Production users should use:

- SSO Web Flow (/auth/sso/login) - authorization code flow
- Device Flow - for CLI tools
- Service Accounts - client credentials flow

The test uses password grant for convenience (automated testing without browser interaction), but this flow should be disabled in production Keycloak
clients.

1. Groupings are correctly applied (with duplicate code commented out):
    - AuthN middleware extracts groups from JWT: [product-engineers]
    - AuthN middleware builds group→role map: map[product-engineers:[product-engineer]]
    - Casbin groupings are applied: user:454... → group:product-engineers → role:product-engineer
    - Effective roles calculated correctly: [product-engineer]
2. Groups persist to AuthZ interceptor:
    - Groups from context: [product-engineers] ✓
    - Casbin roles: [group:product-engineers] ✓
