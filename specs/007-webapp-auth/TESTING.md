# WebApp Authentication Testing Strategy

This document outlines the testing strategy for user authentication and authorization, distinguishing between backend integration tests and true end-to-end (E2E) user workflow tests.

## Integration Test Functionality Coverage (`tests/integration`)

The integration test suite is designed to verify lower-level mechanics of the authentication and authorization systems. It uses a combination of direct API calls with JWT Bearer tokens and mocked web session flows. It is not intended to test the full, browser-based user SSO experience.

| Feature / Functionality | How It's Verified | Test(s) | Notes |
| :--- | :--- | :--- | :--- |
| **Core Authentication** |
| Internal IdP (Mode 2) is configured | Checks for Mode 2 config via `/auth/config` | `isMode2Configured` | Confirms server is running as an internal OIDC provider. |
| External IdP (Mode 1) is configured | Checks for Mode 1 config via `/auth/config` | `isMode1Configured` | Confirms server is running as a resource server. |
| JWT Validation (Signature, Expiry, Issuer) | Sends valid/invalid JWTs from Keycloak/mock to a protected RPC endpoint. | `TestMode1_ExternalTokenValidation`, `TestMode1_InvalidTokenRejection` | Verifies the `JWTAuthenticator` correctly validates tokens against the IdP's public keys. |
| JIT User Provisioning | A JWT for a new external user is sent to an API endpoint; success implies user creation. | `TestMode1_SSO_UserAuth` | Tests the `iamService` logic for creating a user with an external `subject`. |
| **Role-Based Access Control (RBAC)** |
| Group-to-Role Mapping | A JWT with a `groups` claim is used to access an RPC endpoint; success/failure is checked. | `TestMode1_GroupRoleMapping`, `TestMode1_UserGroupAuthorization` | **This is the key JWT-based test.** It verifies the entire chain: `groups` claim -> `groupRoleCache` -> role -> permissions. |
| Role Permission Union | A JWT for a user in multiple groups is used to verify they get the combined permissions of all roles. | `TestMode1_GroupRoleMapping_UnionSemantics` | Confirms that permissions from multiple roles are additive. |
| **Service Account (SA) Flow** |
| Internal SA Creation & Auth | Uses `gridapi sa create` CLI and then client credentials flow against Grid's `/oauth/token`. | `TestMode2_ServiceAccountBootstrap`, `TestMode2_ServiceAccountAuthentication` | Verifies the entire lifecycle for SAs managed internally by Grid. |
| External SA Auth (Keycloak) | Uses client credentials flow against Keycloak to get a JWT, then uses it to call Grid API. | `TestMode1_ServiceAccountAuth` | Verifies that a Keycloak client can act as a service account for Grid. |
| **Web Session Flow** |
| Internal User Login | `POST /auth/login` with username/password; verifies a `grid.session` cookie is returned. | `TestMode2_WebAuth_LoginSuccess` | Tests the login handler for the internal IdP. |
| Session Cookie Attributes | Checks `HttpOnly`, `SameSite`, and `Expires` attributes on the session cookie. | `TestMode2_WebAuth_LoginCookieAttributes` | Verifies security best practices for the session cookie. |
| Session Invalidation (Logout) | Calls `POST /auth/logout` and verifies the session cookie is no longer valid. | `TestMode2_WebAuth_LogoutSuccess` | Confirms the logout handler correctly revokes the session. |
| **Session-to-RPC Contract** |
| **Internal User Session** | An **internal user** logs in via `POST /auth/login` to get a session cookie, which is then used to make successful Connect RPC calls. | `TestMode2_WebAuth_SessionWithConnectRPC` | Proves the `SessionAuthenticator` works for **internal users**. |
| **External User Session** | An **external user** SSO session authentication with Connect RPCs. | ❌ **NOT TESTED** | Requires E2E browser tests (see "Design: End-to-End (E2E) Test Suite" below). The previous brittle test that used database insertion was removed as it bypassed the OAuth2 callback flow. |

---

## Design: End-to-End (E2E) Test Suite

To address the gap in testing the full user-facing SSO workflow, a new, separate E2E test suite should be created. This suite will use a headless browser to simulate real user interactions.

### 1. Guiding Principles

*   **True Black-Box:** The test suite will interact with the system *only* through the WebApp UI. There will be **no direct API calls, no database connections, and no reliance on internal implementation details.**
*   **User-Centric Scenarios:** Tests will be written from the perspective of a user trying to accomplish a goal (e.g., "log in and create a resource").
*   **Full System Testing:** The suite will run against a complete environment (`docker-compose up`) that includes the WebApp, GridAPI, Keycloak, and the database, treating them as a single unit.

### 2. Proposed Tooling

*   **Test Framework:** Go's standard `testing` package. This keeps the entire project within the Go ecosystem.
*   **Browser Automation:** **Playwright for Go**. It provides a high-level, modern API for controlling browsers that is more robust and readable than lower-level tools. It simplifies actions like finding elements, clicking, and waiting for navigation.
*   **Test Environment:** The existing `docker-compose.yml` will be used to orchestrate the full environment. The tests will be configured to point to the WebApp's URL.

### 3. Test Suite Structure

A new directory `tests/e2e` would be created with the following components:

*   `e2e/main_test.go`: Contains the `TestMain` function to manage the test lifecycle, including starting the Playwright browser instance once before all tests and closing it after.
*   `e2e/helpers_test.go`: Contains reusable helper functions that encapsulate common user actions.
    *   `login(t, page, username, password)`: Navigates to the login page, fills credentials, and handles the Keycloak redirect flow.
    *   `logout(t, page)`: Clicks the logout button and verifies the user is returned to the login screen.
    *   `createState(t, page, labels)`: Navigates to the state creation UI and fills out the form.
*   `e2e/auth_flow_test.go`: Contains test cases specifically for authentication and authorization workflows.

### 4. E2E Test Cases to Implement

This suite would replace the need for `TestMode1_WebAuth_SessionWithConnectRPC` and provide much higher confidence.

1.  **`TestE2E_SuccessfulLoginAndSessionPersistence`**
    *   **Objective:** Verify a user can log in via Keycloak and their session persists across page reloads.
    *   **Steps:**
        1.  Navigate to the WebApp URL.
        2.  Click the "Login" button.
        3.  Assert the browser is redirected to the Keycloak login page.
        4.  Enter credentials for a pre-defined test user (e.g., `alice@example.com`).
        5.  Assert the browser is redirected back to the WebApp dashboard.
        6.  Verify a welcome message like "Welcome, Alice" is visible.
        7.  Reload the page and assert the user is still logged in.

2.  **`TestE2E_GroupBasedPermission_Success`**
    *   **Objective:** Verify the entire chain from Keycloak group -> Grid role -> UI permission works.
    *   **Setup:** A test user (`product.engineer@example.com`) is a member of the `product-engineers` Keycloak group, which is mapped in Grid to the `product-engineer` role (which can create states with `env: dev`).
    *   **Steps:**
        1.  Log in as `product.engineer@example.com`.
        2.  Navigate to the "Create State" page in the UI.
        3.  Fill out the form to create a state with the label `env: dev`.
        4.  Submit the form.
        5.  **Assert that a "Success" notification appears and the new state is visible in the state list.**

3.  **`TestE2E_GroupBasedPermission_Forbidden`**
    *   **Objective:** Verify that a user is blocked by UI controls and API rules from performing actions they are not authorized for.
    *   **Setup:** Same as the success test.
    *   **Steps:**
        1.  Log in as `product.engineer@example.com`.
        2.  Navigate to the "Create State" page.
        3.  Fill out the form to create a state with the label `env: prod`.
        4.  Submit the form.
        5.  **Assert that a "Permission Denied" or "Forbidden" error message is displayed in the UI.**

4.  **`TestE2E_FirstTimeLogin_JIT_Provisioning`**
    *   **Objective:** Verify a user who has never logged in before can successfully authenticate.
    *   **Setup:** A user (`new.user@example.com`) exists in Keycloak but not in the Grid database.
    *   **Steps:**
        1.  Log in as `new.user@example.com`.
        2.  **Assert that the login succeeds and the user is taken to the WebApp dashboard.** This implicitly proves that the JIT provisioning in `HandleSSOCallback` was successful.
