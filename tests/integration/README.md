# Grid Integration Tests

This directory contains end-to-end integration tests for Grid's authentication and authorization system.

## Test Coverage

### Mode 1 (External IdP) Tests - `auth_mode1_test.go`

Tests Grid as a Resource Server validating tokens from an external OIDC provider (Keycloak).

#### Phase 1: Infrastructure Tests
- **TestMode1_KeycloakHealth**: Verifies Keycloak is running and Grid is properly configured for Mode 1
  - Checks Keycloak health endpoint
  - Verifies Keycloak OIDC discovery document
  - Confirms Grid does NOT expose OIDC discovery (Resource Server role)
  - Verifies Grid exposes SSO endpoints (RelyingParty role)

#### Phase 2: Core Authentication Tests
- **TestMode1_ExternalTokenValidation**: Verifies Grid validates tokens issued by Keycloak
  - Authenticates with Keycloak using client credentials
  - Parses JWT to verify claims (issuer, subject, etc.)
  - Uses Keycloak token to call Grid API
  - Confirms Grid validates token against Keycloak's JWKS

- **TestMode1_ServiceAccountAuth**: Tests service account authentication via Keycloak
  - Service accounts in Mode 1 are Keycloak clients (not Grid entities)
  - Tests OAuth2 client credentials flow
  - Verifies token issuer is Keycloak
  - Confirms Grid API accepts Keycloak-issued service account tokens

#### Phase 3: Advanced Tests
- **TestMode1_UserGroupAuthorization**: Tests group-based RBAC with IdP groups (Scenario 9)
  - Verifies groups claim extraction from JWT
  - Tests group-to-role mapping
  - Confirms Grid applies group-based authorization

- **TestMode1_SSO_WebFlow**: Tests browser-based SSO login (Scenario 1)
  - Verifies redirect to Keycloak authorization endpoint
  - Checks OIDC parameters (client_id, redirect_uri, state, etc.)
  - Confirms callback endpoint exists

- **TestMode1_DeviceFlow**: Tests CLI device authorization infrastructure (Scenario 2)
  - Checks Keycloak device authorization endpoint availability
  - Verifies infrastructure for CLI device flow

#### Phase 4: Security Tests
- **TestMode1_TokenExpiry**: Tests expired token rejection (Scenario 7)
  - Verifies JWT exp claim is present
  - Confirms valid tokens are accepted
  - Documents Grid's token expiry validation

- **TestMode1_InvalidTokenRejection**: Tests malformed/invalid token security
  - Empty tokens
  - Malformed tokens (not JWT format)
  - Invalid JWT structure
  - Invalid base64 encoding
  - Wrong issuer
  - All should return 401 Unauthorized

### Mode 2 (Internal IdP) Tests - `auth_mode2_test.go`

Tests Grid as an internal OIDC provider issuing and validating its own tokens.

- **TestMode2_SigningKeyGeneration**: Verifies JWT signing key auto-generation and persistence
- **TestMode2_ServiceAccountBootstrap**: Tests service account creation via bootstrap command
- **TestMode2_ServiceAccountAuthentication**: Tests OAuth2 client credentials flow
- **TestMode2_AuthenticatedAPICall**: Verifies authenticated API calls work
- **TestMode2_JWTRevocation**: Tests JWT revocation via jti denylist

## Prerequisites

### For Mode 1 Tests

1. **PostgreSQL**: Running and accessible
   ```bash
   make db-up
   ```

2. **Keycloak**: Running with Grid realm configured
   ```bash
   make keycloak-up
   ```

3. **Keycloak Configuration** (for full tests):
   - Access Keycloak at http://localhost:8443
   - Login with admin/admin
   - Create realm: `grid`
   - Create client: `grid-api`
     - Client Protocol: openid-connect
     - Access Type: confidential
     - Valid Redirect URIs: http://localhost:8080/auth/sso/callback
   - Copy Client Secret from Credentials tab

4. **Service Account Client** (for Phase 2 tests):
   - Create a client for testing (e.g., `ci-pipeline`)
   - Client Protocol: openid-connect
   - Access Type: confidential
   - Service Accounts Enabled: ON
   - Copy Client Secret

5. **Environment Variables** (for full tests):
   ```bash
   export MODE1_TEST_CLIENT_ID="ci-pipeline"
   export MODE1_TEST_CLIENT_SECRET="<keycloak-generated-secret>"
   ```

### For Mode 2 Tests

1. **PostgreSQL**: Running and accessible
   ```bash
   make db-up
   ```

2. **Environment Variables**:
   ```bash
   export OIDC_ISSUER="http://localhost:8080"
   export OIDC_CLIENT_ID="gridapi"
   export OIDC_SIGNING_KEY_PATH="tmp/keys/signing-key.pem"
   ```

## Running Tests

### Run All Integration Tests
```bash
make test-integration
```

### Run Mode 1 Tests Only
```bash
make test-integration-mode1
```

This will:
- Start PostgreSQL
- Start Keycloak
- Build gridapi
- Run tests matching `TestMode1*`

### Run Mode 2 Tests Only
```bash
make test-integration-mode2
```

This will:
- Start PostgreSQL
- Build gridapi
- Run tests matching `TestMode2*` with Mode 2 environment

### Run Specific Test
```bash
cd tests/integration
export MODE1_TEST_CLIENT_ID="ci-pipeline"
export MODE1_TEST_CLIENT_SECRET="<secret>"
go test -v -run TestMode1_KeycloakHealth
```

### Run Tests in Short Mode (Skips Integration Tests)
```bash
cd tests/integration
go test -short -v
```

## Test Organization

### Test Main (`main_test.go`)
- Sets up test environment
- Starts gridapi server in background
- Manages test lifecycle
- Cleans up resources after tests

### Helper Functions

#### Mode 1 Helpers
- `isMode1Configured(t)`: Checks if server is running in Mode 1
- `isKeycloakHealthy(t)`: Verifies Keycloak is accessible
- `getKeycloakDiscovery(t)`: Fetches OIDC discovery document from Keycloak
- `authenticateWithKeycloak(t, clientID, clientSecret)`: Performs client credentials flow

#### Mode 2 Helpers
- `isMode2Configured(t)`: Checks if server is running in Mode 2
- `setupMode2Environment(t)`: Configures Mode 2 environment variables
- `createServiceAccountBootstrap(t, name, role)`: Creates service account via CLI

## Test Scenarios Coverage

The integration tests cover the following scenarios from `quickstart.md`:

- ✅ **Scenario 1**: SSO User Login (Web Flow) - TestMode1_SSO_WebFlow
- ✅ **Scenario 2**: CLI User Login (Device Code Flow) - TestMode1_DeviceFlow
- ✅ **Scenario 3**: Service Account Authentication - TestMode1_ServiceAccountAuth
- ✅ **Scenario 7**: Token Expiry and Refresh - TestMode1_TokenExpiry
- ✅ **Scenario 9**: Group-Based Authorization - TestMode1_UserGroupAuthorization

## CI/CD Integration

### GitHub Actions
```yaml
- name: Run Mode 1 Integration Tests
  run: |
    make keycloak-up
    # Configure Keycloak realm and client via automation
    export MODE1_TEST_CLIENT_ID="ci-pipeline"
    export MODE1_TEST_CLIENT_SECRET="${{ secrets.KEYCLOAK_TEST_CLIENT_SECRET }}"
    make test-integration-mode1
```

## Troubleshooting

### Keycloak Not Starting
```bash
# Check Keycloak logs
make keycloak-logs

# Reset Keycloak environment
make keycloak-reset
```

### Tests Failing Due to Missing Configuration
- Ensure Keycloak realm `grid` is created
- Verify client `grid-api` exists with correct redirect URIs
- Check client secret is correctly set in environment variables

### Port Conflicts
- Keycloak runs on port 8443
- Grid API runs on port 8080 (in tests)
- PostgreSQL runs on port 5432
- Ensure these ports are available

### Database Connection Errors
```bash
# Check PostgreSQL is running
docker compose ps postgres

# Reset database if needed
make db-reset
```

## Test Maintenance

### Adding New Tests

1. **Create test function** in appropriate file (`auth_mode1_test.go` or `auth_mode2_test.go`)
2. **Follow naming convention**: `TestMode1_FeatureName` or `TestMode2_FeatureName`
3. **Add short mode skip** at the beginning:
   ```go
   if testing.Short() {
       t.Skip("Skipping Mode 1 integration test in short mode")
   }
   ```
4. **Document prerequisites** in test comments
5. **Update this README** with test coverage

### Test Dependencies

- `github.com/stretchr/testify/require`: Assertions
- `gopkg.in/square/go-jose.v2/jwt`: JWT parsing
- `github.com/google/uuid`: UUID generation (Mode 2)
- `github.com/lib/pq`: PostgreSQL driver (Mode 2)

## References

- **Feature Spec**: `specs/006-authz-authn-rbac/spec.md`
- **Quickstart Scenarios**: `specs/006-authz-authn-rbac/quickstart.md`
- **Implementation Plan**: `specs/006-authz-authn-rbac/plan.md`
- **Tasks**: `specs/006-authz-authn-rbac/tasks.md`
