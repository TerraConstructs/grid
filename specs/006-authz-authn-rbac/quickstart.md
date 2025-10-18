# Quickstart: Auth N/Z & RBAC Validation

**Feature**: 006-authz-authn-rbac
**Purpose**: Manual validation scenarios for authentication and authorization
**Date**: 2025-10-11

## Prerequisites

1. **Database running**: PostgreSQL with auth schema migrated
2. **Deployment Mode Selected**: Choose Mode 1 (External IdP) or Mode 2 (Internal IdP) - see below
3. **API server running**: `./bin/gridapi serve` with authentication configured
4. **CLI built**: `./bin/gridctl` available

## Deployment Mode Selection

Grid supports **two mutually exclusive authentication modes**. Choose ONE mode for your deployment:

### Mode 1: External IdP Only (Recommended)
- **Use Case**: Enterprise SSO, existing identity provider (Keycloak, Azure Entra ID, Okta)
- **Grid Role**: Resource Server (validates tokens, never issues them)
- **Service Accounts**: Created as IdP clients (managed in IdP, not Grid)
- **Configuration**: `EXTERNAL_IDP_*` environment variables only

### Mode 2: Internal IdP Only (Air-Gapped)
- **Use Case**: Self-contained deployments, no external IdP available
- **Grid Role**: OIDC Provider (issues and validates its own tokens)
- **Service Accounts**: Created in Grid via admin API
- **Configuration**: `OIDC_ISSUER` environment variable only

**This quickstart uses Mode 1 (External IdP)** with Keycloak for demonstration. For Mode 2 instructions, see the note at the end of each scenario.

---

## Configuration Setup: Mode 1 (External IdP Only)

### Environment Variables (recommended for quickstart):
```bash
# External IdP Configuration (Mode 1)
export EXTERNAL_IDP_ISSUER="http://localhost:8443/realms/grid"
export EXTERNAL_IDP_CLIENT_ID="grid-api"
export EXTERNAL_IDP_CLIENT_SECRET="<your-keycloak-client-secret>"
export EXTERNAL_IDP_REDIRECT_URI="http://localhost:8080/auth/sso/callback"

# Optional: Customize JWT claim extraction (defaults shown)
export OIDC_GROUPS_CLAIM="groups"           # Claim field containing groups
export OIDC_GROUPS_PATH=""                  # JSONPath for nested groups (e.g., "name" for [{"name": "dev-team"}])
export OIDC_USER_ID_CLAIM="sub"             # Claim field for user ID
export OIDC_EMAIL_CLAIM="email"             # Claim field for email

# IMPORTANT: Do NOT set OIDC_ISSUER in Mode 1 (causes mode conflict error)
```

### Keycloak Setup (External IdP for Mode 1):
```bash
# Start Keycloak with helper scripts (FR-111)
make keycloak-up

# View logs
make keycloak-logs

# Reset environment if needed
./scripts/dev/keycloak-reset.sh

# NOTE: Local dev uses HTTP (not HTTPS) per FR-100 Local Development Exception
# Production deployments MUST use HTTPS/TLS

# Manual Keycloak configuration (FR-112):
# Navigate to http://localhost:8443 (note: port 8443, HTTP protocol for local dev) and:
# 1. Login with admin/admin
# 2. Create realm "grid"
# 3. Create client "grid-api" with:
#    - Client ID: grid-api
#    - Client Protocol: openid-connect
#    - Access Type: confidential
#    - Valid Redirect URIs: http://localhost:8080/auth/sso/callback
# 4. Copy Client Secret from Credentials tab
# 5. Configure Groups claim mapper (for Scenario 9)
# 6. Create service account clients for Scenario 3 (see Mode 1 instructions)
```

### Configuration Setup: Mode 2 (Internal IdP Only)

For Mode 2 deployments, use this configuration instead:

```bash
# Internal IdP Configuration (Mode 2)
export OIDC_ISSUER="http://localhost:8080"  # Grid's URL

# JWT claim extraction (same as Mode 1)
export OIDC_GROUPS_CLAIM="groups"
export OIDC_USER_ID_CLAIM="sub"
export OIDC_EMAIL_CLAIM="email"

# IMPORTANT: Do NOT set EXTERNAL_IDP_* variables in Mode 2 (causes mode conflict error)
```

**Mode 2 Note**: No external IdP needed. Grid manages users and service accounts internally.

---

## Scenario 1: SSO User Login (Web Flow)

**Mode**: Mode 1 (External IdP Only)
**Goal**: Validate OIDC authentication flow for browser-based users

### Setup
```bash
# Configure External IdP (Mode 1)
export EXTERNAL_IDP_ISSUER="http://localhost:8443/realms/grid"
export EXTERNAL_IDP_CLIENT_ID="grid-api"
export EXTERNAL_IDP_CLIENT_SECRET="<your-keycloak-client-secret>"
export EXTERNAL_IDP_REDIRECT_URI="http://localhost:8080/auth/sso/callback"

# Start Keycloak (if not running)
make keycloak-up

# Create test user in Keycloak
docker compose exec keycloak /opt/keycloak/bin/kc.sh \
  add-user --realm grid --user alice@example.com --password test123

# Start API server (reads External IdP config from environment)
./bin/gridapi serve --server-addr :8080 --db-url "postgres://grid:gridpass@localhost:5432/grid?sslmode=disable"
```

### Steps
1. Open browser to `http://localhost:8080/auth/sso/login`
2. Expect redirect to Keycloak login page (external IdP)
3. Enter credentials: `alice@example.com` / `test123`
4. Expect redirect back to `http://localhost:8080/auth/sso/callback?code=...`
5. Expect session cookie set (`grid_session`)
6. Verify user created in database:
   ```sql
   SELECT * FROM users WHERE email = 'alice@example.com';
   ```

### Success Criteria
- ✅ User redirected to Keycloak (external IdP)
- ✅ Successful login returns to SSO callback
- ✅ Session cookie set (HTTPOnly, Secure in prod)
- ✅ User record created in `users` table (on first request, not at token issuance)
- ✅ Session record created in `sessions` table

### Failure Scenarios
- ❌ Invalid credentials → Keycloak error page
- ❌ Canceled login → Error page with message
- ❌ Expired state → 400 Bad Request

### Mode 2 Alternative
In Mode 2 (Internal IdP), Grid would host its own login form instead of redirecting to Keycloak. The flow would be:
1. Open `http://localhost:8080/auth/login` (Grid-hosted form)
2. Enter credentials managed in Grid's user table
3. Grid issues token and creates session at issuance time

---

## Scenario 2: CLI User Login (Device Code Flow)

**Mode**: Mode 1 (External IdP) or Mode 2 (Internal IdP) - flows differ
**Goal**: Validate OIDC device authorization for CLI users

### Mode 1: Device Flow via External IdP

In Mode 1, the CLI initiates device authorization flow with the external IdP (Keycloak). Grid acts as proxy/broker.

**Flow**: CLI → External IdP's `/device_authorization` → User approves at IdP → IdP issues token → Grid validates token

### Setup (Mode 1)
```bash
# Ensure API server running with External IdP configured
# (See Configuration Setup section for EXTERNAL_IDP_* variables)
./bin/gridapi serve --server-addr :8080 --db-url "postgres://grid:gridpass@localhost:5432/grid?sslmode=disable"
```

### Steps (Mode 1)
1. Run CLI login command:
   ```bash
   ./bin/gridctl login
   ```
2. Expect output similar to:
   ```
   Visit the following URL to authenticate:
     http://localhost:8443/realms/grid/device
   Enter the user code: ZXCV-1234

   Waiting for authorization (polling every 5s)...
   ```

3. Browser opens to Keycloak device verification page (external IdP)
4. Enter the user code and authenticate with credentials: `bob@example.com` / `test456`
5. After approval, the CLI poll detects success and displays:
   ```
   ✓ Authentication successful
   Token expires: 2025-10-12T11:30:00Z
   Token saved to ~/.grid/credentials.json
   ```
6. Verify credentials file:
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

### Success Criteria (Mode 1)
- ✅ CLI initiates device flow with external IdP
- ✅ User approves at IdP's device verification page
- ✅ IdP issues token with `iss` = IdP's URL
- ✅ Grid validates token on first API request
- ✅ Token saved to `~/.grid/credentials.json` with mode 0600
- ✅ Subsequent CLI commands automatically use saved token

### Mode 2: Device Flow via Grid

In Mode 2, Grid acts as the OIDC provider and hosts the device authorization endpoints.

**Flow**: CLI → Grid's `/device_authorization` → User approves at Grid's UI (`/auth/device/verify`) → Grid issues token

### Setup (Mode 2)
```bash
# Ensure API server running with Internal IdP configured
export OIDC_ISSUER="http://localhost:8080"
./bin/gridapi serve --server-addr :8080 --db-url "postgres://grid:gridpass@localhost:5432/grid?sslmode=disable"
```

### Steps (Mode 2)
1. Run CLI login command (same as Mode 1)
2. CLI contacts Grid's `/device_authorization` endpoint
3. Browser opens to Grid's device verification page: `http://localhost:8080/auth/device/verify?user_code=ZXCV-1234`
4. User authenticates with Grid-managed credentials (username/password form)
5. Grid issues token with `iss` = Grid's URL
6. Session persisted at issuance time (Mode 2 characteristic)

---

## Scenario 3: Service Account Authentication

**Mode**: Mode 1 vs Mode 2 - **COMPLETELY DIFFERENT** approaches
**Goal**: Validate client credentials flow for automated systems

### Mode 1: Service Accounts as IdP Clients

In Mode 1, service accounts are **NOT Grid entities**. They are created in the external IdP (Keycloak) as confidential clients.

### Setup (Mode 1)
```bash
# Create service account client in Keycloak (not Grid!)
# Via Keycloak Admin Console:
# 1. Navigate to http://localhost:8443
# 2. Go to Clients → Create Client
# 3. Client ID: ci-pipeline (choose a name)
# 4. Client Protocol: openid-connect
# 5. Access Type: confidential
# 6. Service Accounts Enabled: ON
# 7. Copy Client Secret from Credentials tab
```

### Steps (Mode 1)
1. Save credentials to environment:
   ```bash
   export SERVICE_ACCOUNT_CLIENT_ID="ci-pipeline"
   export SERVICE_ACCOUNT_CLIENT_SECRET="<keycloak-generated-secret>"
   ```
2. Authenticate using client credentials **against Keycloak** (not Grid):
   ```bash
   curl -X POST http://localhost:8443/realms/grid/protocol/openid-connect/token \
     -H "Content-Type: application/x-www-form-urlencoded" \
     -d "grant_type=client_credentials" \
     -d "client_id=$SERVICE_ACCOUNT_CLIENT_ID" \
     -d "client_secret=$SERVICE_ACCOUNT_CLIENT_SECRET"
   ```
3. Expect response from Keycloak:
   ```json
   {
     "access_token": "eyJhbGc...",
     "token_type": "Bearer",
     "expires_in": 300
   }
   ```
4. Use token to call Grid API (Grid validates token):
   ```bash
   curl -H "Authorization: Bearer eyJhbGc..." \
     http://localhost:8080/state.v1.StateService/ListStates
   ```

### Success Criteria (Mode 1)
- ✅ Service account created in Keycloak (not Grid database)
- ✅ Token issued by Keycloak with `iss` = Keycloak URL
- ✅ Grid validates token against Keycloak's JWKS
- ✅ Session created on first API request
- ✅ Token used successfully in API requests

---

### Mode 2: Service Accounts as Grid Entities

In Mode 2, service accounts are Grid-managed entities stored in the `service_accounts` table.

### Setup (Mode 2)
```bash
# Create service account via Grid admin CLI
./bin/gridctl admin create-service-account \
  --name ci-pipeline \
  --description "GitHub Actions CI/CD"

# Expected output:
# Service Account created:
#   Client ID: 550e8400-e29b-41d4-a716-446655440000
#   Client Secret: <secret> (save this, won't be shown again)
```

### Steps (Mode 2)
1. Save credentials to environment:
   ```bash
   export GRID_CLIENT_ID="550e8400-e29b-41d4-a716-446655440000"
   export GRID_CLIENT_SECRET="<secret>"
   ```
2. Authenticate using client credentials **against Grid**:
   ```bash
   curl -X POST http://localhost:8080/oauth/token \
     -H "Content-Type: application/x-www-form-urlencoded" \
     -d "grant_type=client_credentials" \
     -d "client_id=$GRID_CLIENT_ID" \
     -d "client_secret=$GRID_CLIENT_SECRET"
   ```
3. Expect response from Grid:
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

### Success Criteria (Mode 2)
- ✅ Service account created in Grid's `service_accounts` table
- ✅ Client secret bcrypt hashed in database
- ✅ Token issued by Grid with `iss` = Grid URL
- ✅ Session persisted at issuance time
- ✅ Token used successfully in API requests

---

**Note**: Scenarios 4-12 are **mode-agnostic** - they work identically in both Mode 1 and Mode 2. These scenarios focus on authorization (RBAC, label scopes, constraints) which operate on authenticated principals regardless of how tokens were issued.

---

## Scenario 4: Role Assignment and Permission Check

**Mode**: Both (mode-agnostic)
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

7. Inspect another user's effective permissions (FR-048):
   ```bash
   ./bin/gridctl role inspect user:alice@example.com
   ```
   **Expect**: ✅ Shows Alice's roles, permissions, label scope, constraints
   ```
   Principal: user:alice@example.com
   Type: user

   Assigned Roles:
   - product-engineer

   Effective Permissions:
   - state:create, state:read, state:list, state:update-labels
   - tfstate:read, tfstate:write, tfstate:lock, tfstate:unlock
   - dependency:create, dependency:read, dependency:list, dependency:delete
   - policy:read

   Label Scope: env == "dev"
   Create Constraints: env must be one of [dev]
   Immutable Keys: [env]
   ```

### Success Criteria
- ✅ Admin can create states with any labels
- ✅ Admin sees all states (no filtering)
- ✅ Admin can modify immutable labels
- ✅ Admin can force unlock
- ✅ Admin can create/modify roles
- ✅ Admin can inspect any user's effective permissions (FR-048)

---

## Scenario 6: Service Account Data Plane Access

**Mode**: Both (setup differs, permissions work the same)
**Goal**: Validate service-account role has tfstate:* permissions only

### Setup
```bash
# NOTE: Service account creation differs by mode (see Scenario 3)
# - Mode 1: Create in Keycloak, get token from Keycloak
# - Mode 2: Create in Grid, get token from Grid

# Example using Mode 2 commands:
CLIENT_ID=$(./bin/gridctl admin create-service-account --name terraform-runner --json | jq -r .client_id)
./bin/gridctl admin assign-role --service-account $CLIENT_ID --role service-account

# Get access token (Mode 2)
TOKEN=$(curl -s -X POST http://localhost:8080/oauth/token \
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

5. Test service account revocation with cascade (FR-070b):
   ```bash
   # Revoke the service account
   ./bin/gridctl admin revoke-service-account --client-id $CLIENT_ID

   # Attempt to use the previously-issued token
   curl -H "Authorization: Bearer $TOKEN" \
     http://localhost:8080/tfstate/<guid>
   ```
   **Expect**: ❌ 401 Unauthorized (session invalidated even though token not yet expired)
   ```json
   {
     "error": "unauthorized",
     "message": "Service account has been revoked"
   }
   ```

### Success Criteria
- ✅ Service account can read/write/lock/unlock Terraform state
- ✅ Service account CANNOT create states (Control Plane)
- ✅ Service account CANNOT manage dependencies
- ✅ Service account CANNOT manage roles
- ✅ Revoking service account immediately invalidates all active sessions (FR-070b)

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

## Scenario 9: Group-Based Authorization (SSO Group Mapping)

**Goal**: Validate JWT groups claim extraction and group-to-role mapping (FR-104-109)

### Setup
```bash
# Configure Keycloak to include groups claim
# 1. Navigate to Keycloak Admin Console (http://localhost:8443)
# 2. Go to Clients → grid-api → Client Scopes → grid-api-dedicated
# 3. Add Mapper → By Configuration → Group Membership
#    - Name: groups
#    - Token Claim Name: groups
#    - Full group path: OFF
#    - Add to ID token: ON
#    - Add to access token: ON

# Create groups in Keycloak
# Navigate to Groups → Create Group:
#   - platform-engineers
#   - product-engineers

# Assign test users to groups
# Users → alice@example.com → Groups → Join Group → product-engineers
# Users → bob@example.com → Groups → Join Group → platform-engineers

# Map groups to roles via Grid admin
./bin/gridctl role assign-group product-engineers product-engineer
./bin/gridctl role assign-group platform-engineers platform-engineer
```

### Steps
1. Login as Alice (JWT will include groups: ["product-engineers"]):
   ```bash
   ./bin/gridctl login
   # (Complete device flow authentication)
   ```

2. Check effective permissions:
   ```bash
   ./bin/gridctl auth status
   ```
   **Expect**: Shows product-engineer role inherited via group membership
   ```
   User: alice@example.com
   Groups: [product-engineers]
   Roles: [product-engineer] (via group)

   Effective Permissions:
   - state:create, state:read, state:list, state:update-labels
   - tfstate:*, dependency:*, policy:read

   Label Scope: env == "dev"
   ```

3. Attempt to create state with env=dev (allowed by group role):
   ```bash
   ./bin/gridctl state create \
     --logic-id alice-dev-state \
     --label env=dev
   ```
   **Expect**: ✅ Success (inherited permission from product-engineers group)

4. List all group-role mappings:
   ```bash
   ./bin/gridctl role list-groups
   ```
   **Expect**: Shows mappings
   ```
   GROUP                   ROLE                ASSIGNED_AT
   product-engineers       product-engineer    2025-10-11T10:00:00Z
   platform-engineers      platform-engineer   2025-10-11T10:05:00Z
   ```

5. Add Alice to second group in Keycloak (platform-engineers)
6. Re-login and check permissions:
   ```bash
   ./bin/gridctl logout
   ./bin/gridctl login
   ./bin/gridctl auth status
   ```
   **Expect**: Shows BOTH roles (union semantics)
   ```
   Groups: [product-engineers, platform-engineers]
   Roles: [product-engineer, platform-engineer] (via groups)

   Effective Permissions: (union of both roles)
   - state:*, tfstate:*, dependency:*, policy:*, admin:*
   Label Scope: (no constraint - platform-engineer has wildcard)
   ```

7. Verify union (OR) semantics - create state with env=prod:
   ```bash
   ./bin/gridctl state create \
     --logic-id alice-prod-state \
     --label env=prod
   ```
   **Expect**: ✅ Success (platform-engineer role has no label scope restriction)

8. Remove group-role mapping:
   ```bash
   ./bin/gridctl role remove-group product-engineers product-engineer
   ```

9. Re-login as Alice and check permissions:
   ```bash
   ./bin/gridctl logout
   ./bin/gridctl login
   ./bin/gridctl auth status
   ```
   **Expect**: Only platform-engineer role remains (product-engineer removed)

### Success Criteria
- ✅ Groups extracted from JWT claims (configurable field)
- ✅ Group-to-role mappings applied transitively
- ✅ Multiple groups use union (OR) semantics (FR-106)
- ✅ Admin can manage group-role mappings via CLI
- ✅ Group membership changes take effect on next login
- ✅ Supports flat array groups: `["dev-team", "contractors"]`
- ✅ Supports nested groups with path extraction: `[{"name": "dev-team"}]`

---

## Scenario 10: Role Configuration Management (Export/Import)

**Goal**: Validate role export/import for configuration portability (FR-045)

### Setup
```bash
# Ensure you have admin permissions
./bin/gridctl login  # as admin user
```

### Steps
1. Export all roles to JSON file:
   ```bash
   ./bin/gridctl role export --output=roles-backup.json
   ```
   **Expect**: ✅ File created with all role definitions
   ```bash
   cat roles-backup.json
   ```
   Expected content:
   ```json
   [
     {
       "name": "product-engineer",
       "permissions": ["state:create", "state:read", "state:list", "state:update-labels", "tfstate:*", "dependency:*", "policy:read"],
       "label_scope": "env == \"dev\"",
       "create_constraints": {"env": ["dev"]},
       "immutable_keys": ["env"]
     },
     {
       "name": "platform-engineer",
       "permissions": ["*:*"],
       "label_scope": "",
       "create_constraints": {},
       "immutable_keys": []
     }
   ]
   ```

2. Export specific roles:
   ```bash
   ./bin/gridctl role export product-engineer platform-engineer \
     --output=selected-roles.json
   ```
   **Expect**: ✅ Only specified roles exported

3. Modify a role definition in the exported file:
   ```bash
   # Edit roles-backup.json - add new custom role
   cat >> roles-backup.json <<EOF
   ,{
     "name": "staging-engineer",
     "permissions": ["state:*", "tfstate:*", "dependency:*", "policy:read"],
     "label_scope": "env == \"staging\"",
     "create_constraints": {"env": ["staging"]},
     "immutable_keys": ["env"]
   }
   EOF
   ```

4. Import roles into another environment (idempotent):
   ```bash
   ./bin/gridctl role import --file=roles-backup.json
   ```
   **Expect**: ✅ Summary output
   ```
   Role import complete:
   - Imported: 1 (staging-engineer)
   - Skipped: 2 (product-engineer, platform-engineer - already exist)
   - Errors: 0
   ```

5. Import with overwrite flag:
   ```bash
   ./bin/gridctl role import --file=roles-backup.json --force
   ```
   **Expect**: ✅ All roles updated
   ```
   Role import complete:
   - Imported: 3 (all roles overwritten)
   - Skipped: 0
   - Errors: 0
   ```

6. Verify imported role works:
   ```bash
   # Assign staging-engineer role to user
   ./bin/gridctl admin assign-role \
     --user test@example.com \
     --role staging-engineer

   # Login as that user and test permissions
   # (should have env=staging scope)
   ```

### Success Criteria
- ✅ Export creates valid JSON with all role metadata
- ✅ Export can filter by specific role names
- ✅ Import is idempotent (same import can run multiple times)
- ✅ Import without --force skips existing roles
- ✅ Import with --force overwrites existing roles
- ✅ Import validates label_scope_expr as valid go-bexpr syntax
- ✅ Imported roles work immediately (no restart required)
- ✅ Import reports clear summary of results

---

## Scenario 11: Terraform Wrapper with Authentication

**Goal**: Validate gridctl tf wrapper injects tokens correctly (FR-097a-097l)

### Setup
```bash
# Login as user with appropriate permissions
./bin/gridctl login

# Create test Terraform configuration
mkdir -p /tmp/tf-test
cd /tmp/tf-test

cat > main.tf <<'EOF'
terraform {
  required_version = ">= 1.0"
}

resource "null_resource" "test" {
  triggers = {
    timestamp = timestamp()
  }
}
EOF

# Initialize state via gridctl
./bin/gridctl state create \
  --logic-id tf-wrapper-test \
  --label env=dev

# Create .grid context file
./bin/gridctl state init
# This creates .grid file with backend endpoint configuration
```

### Steps
1. Initialize Terraform via wrapper:
   ```bash
   ./bin/gridctl tf -- init
   ```
   **Expect**: ✅ Success - Terraform initializes using HTTP backend
   ```
   Initializing the backend...

   Successfully configured the backend "http"!
   ```

2. Run plan with automatic token injection:
   ```bash
   ./bin/gridctl tf -- plan
   ```
   **Expect**: ✅ Plan output, no auth errors
   ```
   Terraform will perform the following actions:

     # null_resource.test will be created
     + resource "null_resource" "test" {
         + id       = (known after apply)
         + triggers = {
             + "timestamp" = (known after apply)
           }
       }
   ```

3. Verify token not leaked in logs (FR-097g):
   ```bash
   ./bin/gridctl tf --verbose -- plan 2>&1 | grep -E "(token|password|secret)" -i
   ```
   **Expect**: No token values visible (should see "[REDACTED]" if tokens mentioned)

4. Test binary selection (FR-097b):
   ```bash
   # Test --tf-bin flag
   ./bin/gridctl tf --tf-bin=tofu -- version

   # Test TF_BIN environment variable
   export TF_BIN=tofu
   ./bin/gridctl tf -- version

   # Test auto-detect (should find terraform or tofu)
   unset TF_BIN
   ./bin/gridctl tf -- version
   ```
   **Expect**: ✅ Correct binary selected each time

5. Test 401 retry logic (FR-097e):
   ```bash
   # Expire token in database to simulate mid-run expiry
   psql -d grid -c "UPDATE sessions SET expires_at = NOW() - INTERVAL '1 minute' WHERE token_hash = '<hash>';"

   # Run terraform command - should auto-retry once
   ./bin/gridctl tf -- plan
   ```
   **Expect**: ✅ Either succeeds (token refreshed) or fails with clear auth message after 1 retry

6. Test CI mode detection (FR-097j):
   ```bash
   # Simulate CI environment
   export CI=true
   export GITHUB_ACTIONS=true
   unset DISPLAY

   # Should use service account credentials if available
   ./bin/gridctl tf -- plan
   ```
   **Expect**: Uses saved credentials, fails fast if missing with clear error

7. Verify token never persisted to disk (FR-097k):
   ```bash
   # Check no bearer tokens in terraform files
   grep -r "Bearer" .terraform/ backend.tf .grid 2>/dev/null
   ```
   **Expect**: No matches (token only passed via TF_HTTP_PASSWORD env var)

8. Test exit code preservation (FR-097h):
   ```bash
   # Introduce syntax error in Terraform config
   echo "invalid syntax" >> main.tf

   # Run terraform validate - should fail
   ./bin/gridctl tf -- validate
   echo "Exit code: $?"
   ```
   **Expect**: ❌ Non-zero exit code matching Terraform's exit code

9. Test backend init hint (FR-097l):
   ```bash
   # Remove .grid context file
   rm .grid

   # Run terraform command
   ./bin/gridctl tf -- plan
   ```
   **Expect**: ⚠️ Hint printed but execution continues
   ```
   Hint: Grid backend not detected. Run 'gridctl state init' to configure.
   ```

### Success Criteria
- ✅ Token automatically injected via TF_HTTP_PASSWORD (FR-097c)
- ✅ Terraform commands work without manual auth
- ✅ Tokens never logged or persisted to disk (FR-097g, FR-097k)
- ✅ Binary selection follows precedence (--tf-bin → TF_BIN → auto-detect) (FR-097b)
- ✅ STDIO pass-through preserves exact output (FR-097f)
- ✅ Exit codes preserved from subprocess (FR-097h)
- ✅ 401 retry logic works (single retry, then fail) (FR-097e)
- ✅ CI mode uses service account credentials (FR-097j)
- ✅ Helpful hint when .grid context missing (FR-097l)
- ✅ Secret redaction in verbose mode (FR-097g)

---

## Scenario 12: Lock-Aware Authorization Bypass

**Goal**: Validate lock holders retain tfstate:write/unlock permissions even if labels change (FR-061a)

### Setup
```bash
# Create state as product-engineer (env=dev)
./bin/gridctl login  # as alice@example.com (product-engineer role, env=dev scope)
STATE_GUID=$(./bin/gridctl state create \
  --logic-id lock-test-state \
  --label env=dev \
  --json | jq -r .guid)

# Get token for API calls
TOKEN=$(cat ~/.grid/credentials.json | jq -r .access_token)
```

### Steps
1. Lock the state as Alice:
   ```bash
   curl -X LOCK -H "Authorization: Bearer $TOKEN" \
     -H "Content-Type: application/json" \
     -d '{
       "ID": "alice-lock-123",
       "Operation": "apply",
       "Info": "Alice is running apply",
       "Who": "user:alice@example.com",
       "Version": "1.5.0",
       "Created": "2025-10-11T10:00:00Z",
       "Path": "/tmp/tf-test"
     }' \
     http://localhost:8080/tfstate/$STATE_GUID/lock
   ```
   **Expect**: ✅ 200 OK (lock acquired)

2. Admin changes state labels to env=prod (out of Alice's scope):
   ```bash
   # As admin
   ./bin/gridctl state update-labels \
     --guid $STATE_GUID \
     --add env=prod \
     --remove env
   ```
   **Expect**: ✅ Success (admin can modify immutable keys)

3. Alice attempts to write state (still holding lock):
   ```bash
   curl -X POST -H "Authorization: Bearer $TOKEN" \
     -H "Content-Type: application/json" \
     -d '{"version": 4, "terraform_version": "1.5.0", "resources": []}' \
     http://localhost:8080/tfstate/$STATE_GUID
   ```
   **Expect**: ✅ 200 OK (lock-aware bypass allows write despite label change)

4. Alice attempts to read state metadata (Control Plane):
   ```bash
   curl -H "Authorization: Bearer $TOKEN" \
     http://localhost:8080/state.v1.StateService/GetState \
     -d "{\"guid\": \"$STATE_GUID\"}"
   ```
   **Expect**: ❌ 404 Not Found (state now outside scope, read not protected by lock)

5. Alice unlocks the state:
   ```bash
   curl -X UNLOCK -H "Authorization: Bearer $TOKEN" \
     -H "Content-Type: application/json" \
     -d "{\"ID\": \"alice-lock-123\"}" \
     http://localhost:8080/tfstate/$STATE_GUID/unlock
   ```
   **Expect**: ✅ 200 OK (lock-aware bypass allows unlock)

6. Alice attempts to write state again (after unlock):
   ```bash
   curl -X POST -H "Authorization: Bearer $TOKEN" \
     -H "Content-Type: application/json" \
     -d '{"version": 4, "resources": []}' \
     http://localhost:8080/tfstate/$STATE_GUID
   ```
   **Expect**: ❌ 403 Forbidden (no longer has access - lock released)
   ```json
   {
     "error": "forbidden",
     "message": "You do not have permission to access this state"
   }
   ```

### Success Criteria
- ✅ Lock holder retains tfstate:write permission during lock
- ✅ Lock holder retains tfstate:unlock permission during lock
- ✅ Label changes during lock don't evict lock holder
- ✅ Other operations (metadata reads) still respect label scope
- ✅ After unlock, normal authorization rules apply
- ✅ Lock holder identified by principal ID in LockInfo.Who

---

## Validation Checklist

After completing all scenarios, verify:

### Mode-Specific Authentication
**Mode 1 (External IdP Only)**:
- [ ] Config validation rejects both OIDC_ISSUER and EXTERNAL_IDP_* set simultaneously
- [ ] SSO login redirects to external IdP (Keycloak) - Scenario 1
- [ ] CLI device flow uses external IdP - Scenario 2, Mode 1
- [ ] Service accounts created in IdP, tokens from IdP - Scenario 3, Mode 1
- [ ] Sessions created on first request (not at issuance) - Scenarios 1-3, Mode 1

**Mode 2 (Internal IdP Only)**:
- [ ] Config validation rejects both modes enabled
- [ ] Grid hosts device verification UI at /auth/device/verify - Scenario 2, Mode 2
- [ ] Service accounts created in Grid, tokens from Grid - Scenario 3, Mode 2
- [ ] Sessions persisted at issuance time - Scenario 3, Mode 2
- [ ] Grid's JWKS endpoint serves signing keys - Mode 2

### Core Authorization (Mode-Agnostic)
- [ ] Role assignment works - Scenario 4
- [ ] Create constraints enforced - Scenario 4
- [ ] Immutable keys enforced - Scenario 4
- [ ] Label scope filtering works - Scenario 4
- [ ] Admin has full access - Scenario 5
- [ ] Admin can inspect any user's permissions (FR-048) - Scenario 5
- [ ] Service account limited to Data Plane - Scenario 6
- [ ] Service account revocation cascades to sessions (FR-070b) - Scenario 6
- [ ] Token expiry enforced (12 hours) - Scenario 7
- [ ] Dependency authorization checks both ends - Scenario 8

### Group-Based Authorization (NEW)
- [ ] JWT groups claim extracted from token - Scenario 9
- [ ] Group-to-role mappings work - Scenario 9
- [ ] Multiple groups use union (OR) semantics - Scenario 9
- [ ] Group membership changes take effect on re-login - Scenario 9
- [ ] Admin can manage group-role mappings - Scenario 9

### Role Configuration Management (NEW)
- [ ] Role export to JSON works - Scenario 10
- [ ] Role import is idempotent - Scenario 10
- [ ] Role import with --force overwrites existing - Scenario 10
- [ ] Imported roles work immediately - Scenario 10

### Terraform Wrapper (NEW)
- [ ] gridctl tf injects tokens automatically - Scenario 11
- [ ] Tokens never leaked in logs or files - Scenario 11
- [ ] Binary selection (--tf-bin, TF_BIN, auto-detect) - Scenario 11
- [ ] 401 retry logic works - Scenario 11
- [ ] CI mode detection works - Scenario 11
- [ ] Exit codes preserved - Scenario 11

### Lock-Aware Authorization (NEW)
- [ ] Lock holders retain tfstate:write during lock - Scenario 12
- [ ] Lock holders retain tfstate:unlock during lock - Scenario 12
- [ ] Label changes during lock don't evict holder - Scenario 12
- [ ] After unlock, normal authz rules apply - Scenario 12

### General
- [ ] Clear error messages on denial
- [ ] Audit logs captured (check database)
- [ ] Developer tooling (make targets) works - Prerequisites

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

**Status**: Comprehensive quickstart with 12 scenarios covering all functional requirements. Updated for mode-based authentication architecture. Ready for implementation and testing.

**Coverage Summary**:
- ✅ **Mode-Based Authentication** (Scenarios 1-3): Mode 1 (External IdP Only) and Mode 2 (Internal IdP Only) with distinct service account approaches
- ✅ **Core Authorization** (Scenarios 4-8, mode-agnostic): RBAC, label scopes, constraints, token lifecycle, dependency authz
- ✅ **Group-Based Authorization** (Scenario 9): JWT claims, group-to-role mappings, union semantics (FR-104-109)
- ✅ **Role Configuration Management** (Scenario 10): Export/import for portability (FR-045)
- ✅ **Terraform Wrapper** (Scenario 11): Token injection, secret redaction, CI mode (FR-097a-097l)
- ✅ **Lock-Aware Authorization** (Scenario 12): Lock holder bypass for label changes (FR-061a)
- ✅ **Developer Tooling**: Make targets for Keycloak and OIDC key management (FR-110-112)

**Architecture Updates**:
- Mutually exclusive deployment modes (Mode 1 OR Mode 2, not both)
- Mode 1 service accounts are IdP clients (created in Keycloak, not Grid)
- Mode 2 service accounts are Grid entities (created via Grid admin API)
- Single-issuer token validation per deployment (no dynamic issuer selection)
