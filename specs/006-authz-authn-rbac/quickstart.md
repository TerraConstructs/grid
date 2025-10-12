# Quickstart: Auth N/Z & RBAC Validation

**Feature**: 006-authz-authn-rbac
**Purpose**: Manual validation scenarios for authentication and authorization
**Date**: 2025-10-11

## Prerequisites

1. **Database running**: PostgreSQL with auth schema migrated
2. **Keycloak running**: Docker Compose service started (or your OIDC provider)
3. **OIDC Configuration**: Environment variables or config file with IdP settings
4. **API server running**: `./bin/gridapi serve` with OIDC configured
5. **CLI built**: `./bin/gridctl` available

### OIDC Configuration Setup

Before starting the API server, configure your OIDC provider settings. You can use either environment variables or a config file.

**Environment Variables** (recommended for quickstart):
```bash
export OIDC_ISSUER="http://localhost:8180/realms/grid"
export OIDC_CLIENT_ID="grid-api"
export OIDC_CLIENT_SECRET="<your-client-secret>"
export OIDC_REDIRECT_URI="http://localhost:8080/auth/callback"

# Optional: Customize JWT claim extraction (defaults shown)
export OIDC_GROUPS_CLAIM="groups"           # Claim field containing groups
export OIDC_GROUPS_PATH=""                  # JSONPath for nested groups (e.g., "name" for [{"name": "dev-team"}])
export OIDC_USER_ID_CLAIM="sub"             # Claim field for user ID
export OIDC_EMAIL_CLAIM="email"             # Claim field for email
```

**Config File** (`config.yaml`):
```yaml
oidc:
  issuer: "http://localhost:8180/realms/grid"
  client_id: "grid-api"
  client_secret: "<your-client-secret>"
  redirect_uri: "http://localhost:8080/auth/callback"

  # Optional claim mappings (defaults shown)
  groups_claim_field: "groups"      # Flat array: ["dev-team", "admins"]
  groups_claim_path: ""             # For nested: [{"name": "dev-team"}] use "name"
  user_id_claim_field: "sub"
  email_claim_field: "email"
```

**Keycloak Setup** (if using provided docker-compose):
```bash
# Start Keycloak
docker compose up -d keycloak

# Create realm and client (automated via docker-compose, or manually via UI)
# Navigate to http://localhost:8180 and:
# 1. Create realm "grid"
# 2. Create client "grid-api" with:
#    - Client ID: grid-api
#    - Client Protocol: openid-connect
#    - Access Type: confidential
#    - Valid Redirect URIs: http://localhost:8080/auth/callback
# 3. Copy Client Secret from Credentials tab
```

---

## Scenario 1: SSO User Login (Web Flow)

**Goal**: Validate OIDC authentication flow for browser-based users

### Setup
```bash
# Configure OIDC (if not already set)
export OIDC_ISSUER="http://localhost:8180/realms/grid"
export OIDC_CLIENT_ID="grid-api"
export OIDC_CLIENT_SECRET="<your-keycloak-client-secret>"
export OIDC_REDIRECT_URI="http://localhost:8080/auth/callback"

# Start Keycloak (if not running)
docker compose up -d keycloak

# Create test user in Keycloak
docker compose exec keycloak /opt/keycloak/bin/kc.sh \
  add-user --realm grid --user alice@example.com --password test123

# Start API server (reads OIDC config from environment)
./bin/gridapi serve --server-addr :8080 --db-url "postgres://grid:gridpass@localhost:5432/grid?sslmode=disable"
```

### Steps
1. Open browser to `http://localhost:8080/auth/login`
2. Expect redirect to Keycloak login page
3. Enter credentials: `alice@example.com` / `test123`
4. Expect redirect back to `http://localhost:8080/auth/callback?code=...`
5. Expect session cookie set (`grid_session`)
6. Verify user created in database:
   ```sql
   SELECT * FROM users WHERE email = 'alice@example.com';
   ```

### Success Criteria
- ✅ User redirected to Keycloak
- ✅ Successful login returns to callback
- ✅ Session cookie set (HTTPOnly, Secure in prod)
- ✅ User record created in `users` table
- ✅ Session record created in `sessions` table

### Failure Scenarios
- ❌ Invalid credentials → Keycloak error page
- ❌ Canceled login → Error page with message
- ❌ Expired state → 400 Bad Request

---

## Scenario 2: CLI User Login (PKCE + Loopback Flow)

**Goal**: Validate PKCE + loopback authentication for CLI users

**Flow Type**: This scenario uses PKCE with loopback redirect (OAuth 2.0 for Native Apps - RFC 8252). The CLI starts a temporary HTTP server on a random port to receive the authorization code. This is more user-friendly than device-code flow for desktop environments where browser opening is supported.

**Note**: The HTTP contract also includes device-code endpoints (`/auth/device/code`, `/auth/device/token`) for environments where loopback is not available (e.g., remote SSH sessions). See plan.md:686-694 for implementation details.

### Setup
```bash
# Ensure API server running with OIDC configured
# (See Prerequisites section for OIDC environment variables)
./bin/gridapi serve --server-addr :8080 --db-url "postgres://grid:gridpass@localhost:5432/grid?sslmode=disable"
```

### Steps
1. Run CLI login command:
   ```bash
   ./bin/gridctl login
   ```
2. Expect output (note: port will be random):
   ```
   Starting local callback server on http://127.0.0.1:54321...
   Opening browser to authenticate...  (print out URL for user to handle this step manually)
   If browser doesn't open automatically, visit this URL:
     http://localhost:8080/auth/login?redirect_uri=http://127.0.0.1:54321/callback

   Waiting for authentication...
   ```

   **Note**: The callback port (`54321` in this example) is **randomly selected** by the CLI using `net.Listen(":0")`. Your actual port will differ.

3. Browser opens to Keycloak login page
4. Enter credentials: `bob@example.com` / `test456`
5. Browser redirected to `http://127.0.0.1:<random-port>/callback?code=...&state=...`
6. CLI receives authorization code and displays:
   ```
   ✓ Authentication successful
   Token expires: 2025-10-12T11:30:00Z
   Token saved to ~/.grid/credentials.json
   ```
7. Verify credentials file:
   ```bash
   cat ~/.grid/credentials.json
   ```
   Expected:
   ```json
   {
     "access_token": "eyJhbGc...",
     "token_type": "Bearer",
     "expires_at": "2025-10-11T23:59:59Z"
   }
   ```

### Success Criteria
- ✅ CLI starts local callback server on random free port (via `net.Listen(":0")`)
- ✅ Callback URL uses dynamic port in redirect_uri parameter
- ✅ Browser opens automatically to OIDC login flow
- ✅ PKCE code verifier and challenge generated
- ✅ Authorization code exchanged for token at callback endpoint
- ✅ Token saved to `~/.grid/credentials.json` with mode 0600
- ✅ Subsequent CLI commands automatically use saved token

### Failure Scenarios
- ❌ Port binding fails → Retry with different port (should never happen with :0)
- ❌ User cancels login → Timeout after 5 minutes, error message
- ❌ Authorization code expired → OAuth2 error, prompt to re-login
- ❌ PKCE validation fails → 400 Bad Request from server

---

## Scenario 3: Service Account Authentication

**Goal**: Validate client credentials flow for automated systems

### Setup
```bash
# Create service account via admin CLI
./bin/gridctl admin create-service-account \
  --name ci-pipeline \
  --description "GitHub Actions CI/CD"

# Expected output:
# Service Account created:
#   Client ID: 550e8400-e29b-41d4-a716-446655440000
#   Client Secret: <secret> (save this, won't be shown again)
```

### Steps
1. Save credentials to environment:
   ```bash
   export GRID_CLIENT_ID="550e8400-e29b-41d4-a716-446655440000"
   export GRID_CLIENT_SECRET="<secret>"
   ```
2. Authenticate using client credentials:
   ```bash
   curl -X POST http://localhost:8080/auth/token \
     -H "Content-Type: application/x-www-form-urlencoded" \
     -d "grant_type=client_credentials" \
     -d "client_id=$GRID_CLIENT_ID" \
     -d "client_secret=$GRID_CLIENT_SECRET"
   ```
3. Expect response:
   ```json
   {
     "access_token": "eyJhbGc...",
     "token_type": "Bearer",
     "expires_in": 43200
   }
   ```
4. Use token in subsequent requests:
   ```bash
   curl -H "Authorization: Bearer eyJhbGc..." \
     http://localhost:8080/state.v1.StateService/ListStates
   ```

### Success Criteria
- ✅ Service account created with UUIDv4 client_id
- ✅ Client secret bcrypt hashed in database
- ✅ Token exchange succeeds with valid credentials
- ✅ Token exchange fails with invalid credentials (401)
- ✅ Token used successfully in API requests

---

## Scenario 4: Role Assignment and Permission Check

**Goal**: Validate RBAC role assignment and authorization enforcement

### Setup
```bash
# Assign product-engineer role to Alice
./bin/gridctl admin assign-role \
  --user alice@example.com \
  --role product-engineer
```

### Steps
1. Login as Alice (via web or CLI)
2. Attempt to create state with `env=dev`:
   ```bash
   ./bin/gridctl state create \
     --logic-id my-dev-state \
     --label env=dev \
     --label team=platform
   ```
   **Expect**: ✅ Success (allowed by role)

3. Attempt to create state with `env=prod`:
   ```bash
   ./bin/gridctl state create \
     --logic-id my-prod-state \
     --label env=prod
   ```
   **Expect**: ❌ 403 Forbidden
   ```json
   {
     "error": "forbidden",
     "message": "Create constraint violation: env must be one of [dev]"
   }
   ```

4. Attempt to list states:
   ```bash
   ./bin/gridctl state list
   ```
   **Expect**: Only states with `env=dev` returned (label scope filter)

5. Attempt to update `env` label on existing state:
   ```bash
   ./bin/gridctl state update-labels \
     --guid <state-guid> \
     --add env=staging
   ```
   **Expect**: ❌ 403 Forbidden
   ```json
   {
     "error": "forbidden",
     "message": "Cannot modify immutable label key: env"
   }
   ```

### Success Criteria
- ✅ Role assigned successfully
- ✅ Create with allowed labels succeeds
- ✅ Create with disallowed labels denied (403 + clear error)
- ✅ List operations filtered by label scope
- ✅ Immutable label modifications denied

---

## Scenario 5: Admin Operations

**Goal**: Validate platform-engineer role has full access

### Setup
```bash
# Assign platform-engineer role to admin user
./bin/gridctl admin assign-role \
  --user admin@example.com \
  --role platform-engineer
```

### Steps
1. Login as admin
2. Create state with any labels:
   ```bash
   ./bin/gridctl state create \
     --logic-id prod-state \
     --label env=prod \
     --label critical=true
   ```
   **Expect**: ✅ Success (no create constraints)

3. List all states (no filter):
   ```bash
   ./bin/gridctl state list
   ```
   **Expect**: All states returned (no label scope restriction)

4. Update any label including immutable ones:
   ```bash
   ./bin/gridctl state update-labels \
     --guid <state-guid> \
     --add env=staging
   ```
   **Expect**: ✅ Success (no immutable key restrictions)

5. Force unlock state locked by another user:
   ```bash
   ./bin/gridctl state unlock --guid <state-guid> --force
   ```
   **Expect**: ✅ Success (admin override)

6. Create custom role:
   ```bash
   ./bin/gridctl admin create-role \
     --name custom-role \
     --action state:read \
     --action state:list \
     --label-scope env=staging
   ```
   **Expect**: ✅ Success

### Success Criteria
- ✅ Admin can create states with any labels
- ✅ Admin sees all states (no filtering)
- ✅ Admin can modify immutable labels
- ✅ Admin can force unlock
- ✅ Admin can create/modify roles

---

## Scenario 6: Service Account Data Plane Access

**Goal**: Validate service-account role has tfstate:* permissions only

### Setup
```bash
# Create service account and assign role
CLIENT_ID=$(./bin/gridctl admin create-service-account --name terraform-runner --json | jq -r .client_id)
./bin/gridctl admin assign-role --service-account $CLIENT_ID --role service-account

# Get access token
TOKEN=$(curl -s -X POST http://localhost:8080/auth/token \
  -d "grant_type=client_credentials" \
  -d "client_id=$CLIENT_ID" \
  -d "client_secret=$CLIENT_SECRET" \
  | jq -r .access_token)
```

### Steps
1. Read Terraform state:
   ```bash
   curl -H "Authorization: Bearer $TOKEN" \
     http://localhost:8080/tfstate/<guid>
   ```
   **Expect**: ✅ 200 OK (state JSON returned)

2. Write Terraform state:
   ```bash
   curl -X POST -H "Authorization: Bearer $TOKEN" \
     -H "Content-Type: application/json" \
     -d '{"version": 4, "resources": []}' \
     http://localhost:8080/tfstate/<guid>
   ```
   **Expect**: ✅ 200 OK

3. Lock state:
   ```bash
   curl -X LOCK -H "Authorization: Bearer $TOKEN" \
     -d '{"ID": "lock-123", "Operation": "apply"}' \
     http://localhost:8080/tfstate/<guid>/lock
   ```
   **Expect**: ✅ 200 OK

4. Attempt to create state (Control Plane):
   ```bash
   curl -H "Authorization: Bearer $TOKEN" \
     -d '{"guid": "<uuid>", "logic_id": "test"}' \
     http://localhost:8080/state.v1.StateService/CreateState
   ```
   **Expect**: ❌ 403 Forbidden
   ```json
   {
     "error": "forbidden",
     "message": "Missing required permission: state:create"
   }
   ```

### Success Criteria
- ✅ Service account can read/write/lock/unlock Terraform state
- ✅ Service account CANNOT create states (Control Plane)
- ✅ Service account CANNOT manage dependencies
- ✅ Service account CANNOT manage roles

---

## Scenario 7: Token Expiry and Refresh

**Goal**: Validate 12-hour token lifetime and session expiry

### Setup
```bash
# Login as user (creates session with 12-hour expiry)
./bin/gridctl login
```

### Steps
1. Check session expiry in database:
   ```sql
   SELECT id, user_id, expires_at, created_at
   FROM sessions
   WHERE user_id = (SELECT id FROM users WHERE email = 'test@example.com')
   ORDER BY created_at DESC
   LIMIT 1;
   ```
   **Expect**: `expires_at` = `created_at` + 12 hours

2. Make authenticated request immediately:
   ```bash
   ./bin/gridctl state list
   ```
   **Expect**: ✅ Success

3. Simulate token expiry (update database):
   ```sql
   UPDATE sessions
   SET expires_at = NOW() - INTERVAL '1 minute'
   WHERE token_hash = '<hash>';
   ```

4. Make authenticated request with expired token:
   ```bash
   ./bin/gridctl state list
   ```
   **Expect**: ❌ 401 Unauthorized
   ```json
   {
     "error": "unauthorized",
     "message": "Token expired, please re-authenticate"
   }
   ```

5. Re-login:
   ```bash
   ./bin/gridctl login
   ```
   **Expect**: ✅ New session created, old session cleaned up

### Success Criteria
- ✅ Session expires_at set to created_at + 12 hours
- ✅ Expired tokens rejected with 401
- ✅ Clear error message prompts re-authentication
- ✅ Re-login creates new session

---

## Scenario 8: Dependency Authorization (Both Ends)

**Goal**: Validate user must have access to both source and target states

### Setup
```bash
# Alice has product-engineer role (env=dev scope)
# Create two states: one dev, one prod
./bin/gridctl state create --logic-id dev-state --label env=dev  # As Alice
./bin/gridctl state create --logic-id prod-state --label env=prod  # As Admin
```

### Steps
1. Login as Alice
2. Attempt to create dependency dev → dev:
   ```bash
   ./bin/gridctl dependency add \
     --from dev-state \
     --output vpc_id \
     --to dev-state-consumer
   ```
   **Expect**: ✅ Success (both states in scope)

3. Attempt to create dependency dev → prod:
   ```bash
   ./bin/gridctl dependency add \
     --from dev-state \
     --output subnet_id \
     --to prod-state
   ```
   **Expect**: ❌ 403 Forbidden
   ```json
   {
     "error": "forbidden",
     "message": "You do not have permission to access state: prod-state"
   }
   ```

4. Attempt to create dependency prod → dev:
   ```bash
   ./bin/gridctl dependency add \
     --from prod-state \
     --output db_endpoint \
     --to dev-state
   ```
   **Expect**: ❌ 403 Forbidden (cannot access prod-state)

### Success Criteria
- ✅ Dependency creation succeeds when user has access to both states
- ✅ Dependency creation denied when user lacks access to source
- ✅ Dependency creation denied when user lacks access to target
- ✅ Error message indicates which state is inaccessible

---

## Validation Checklist

After completing all scenarios, verify:

- [ ] SSO login works (web flow)
- [ ] CLI login works (device code flow)
- [ ] Service account authentication works
- [ ] Role assignment works
- [ ] Create constraints enforced
- [ ] Immutable keys enforced
- [ ] Label scope filtering works
- [ ] Admin has full access
- [ ] Service account limited to Data Plane
- [ ] Token expiry enforced (12 hours)
- [ ] Dependency authorization checks both ends
- [ ] Clear error messages on denial
- [ ] Audit logs captured (check database)

---

## Cleanup

```bash
# Stop services
docker compose down

# Reset database (optional)
psql -d grid -c "DROP SCHEMA public CASCADE; CREATE SCHEMA public;"

# Clear CLI credentials
rm ~/.grid/credentials.json
```

---

**Status**: Quickstart scenarios defined. Ready for implementation and testing.
