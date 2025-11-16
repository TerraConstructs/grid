# Feature Specification: WebApp User Login Flow with Role-Based Filtering

**Feature Branch**: `007-webapp-auth`
**Created**: 2025-11-04
**Status**: Draft
**Input**: User description: "Add webapp user login flow when gridapi requires authentication. It should allow users to log in and review their login status. The webapp is a read only dashboard and when authentication is required, should filter the view based on the authorization roles applicable to the user."

## User Scenarios & Testing *(mandatory)*

### User Story 1 - View States Without Authentication (Priority: P1)

When gridapi runs without authentication enabled, users access the dashboard without login and see all states.

**Why this priority**: This is the baseline behavior - the webapp must continue to work in non-authenticated mode for backward compatibility and development scenarios.

**Independent Test**: Can be fully tested by running gridapi without authentication configuration, navigating to the webapp, and verifying the dashboard loads immediately showing all states without any login UI.

**Acceptance Scenarios**:

1. **Given** gridapi is running without authentication enabled, **When** user navigates to the webapp, **Then** the dashboard displays immediately without login prompt
2. **Given** the dashboard is displayed, **When** user views the states list, **Then** all states in the system are visible
3. **Given** the dashboard is displayed, **When** user checks for authentication UI, **Then** no login button or user menu is visible

---

### User Story 2 - Authenticate Using Internal Identity Provider (Priority: P1)

When gridapi uses internal IdP mode, web users authenticate with username and password to access the dashboard with their assigned roles.

**Why this priority**: This is the primary web authentication flow for internal IdP mode - without this, authenticated web access is impossible when using Grid's built-in authentication.

**Independent Test**: Can be fully tested by running gridapi with internal IdP enabled, attempting to access the dashboard, entering valid username/password credentials, and verifying successful login with role assignment displayed.

**Acceptance Scenarios**:

1. **Given** gridapi requires authentication via internal IdP and user is not logged in, **When** user navigates to the webapp, **Then** user sees a login form with username and password fields
2. **Given** user is on the login page, **When** user enters valid username and password and submits, **Then** user is authenticated and sees the dashboard
3. **Given** user has an active session, **When** user navigates to the webapp, **Then** the dashboard displays immediately without login prompt
4. **Given** user is authenticated, **When** user views their authentication status, **Then** system displays username, email, assigned roles, and authentication type (Basic Auth - Internal IdP)
5. **Given** user enters invalid credentials, **When** user attempts to log in, **Then** system displays an error message without revealing whether username or password was incorrect

---

### User Story 3 - Authenticate Using External Identity Provider (Priority: P1)

When gridapi uses external IdP mode (SSO), web users authenticate through their organization's identity provider to access the dashboard with role assignments based on group memberships.

**Why this priority**: This is the primary web authentication flow for external IdP mode (SSO) - critical for organizations using enterprise identity providers like Keycloak, Azure Entra ID, or Okta.

**Independent Test**: Can be fully tested by running gridapi with external IdP enabled, attempting to access the dashboard, clicking SSO login, completing authentication at the external IdP, and verifying successful callback with group-based role assignment displayed.

**Acceptance Scenarios**:

1. **Given** gridapi requires authentication via external IdP and user is not logged in, **When** user navigates to the webapp, **Then** user sees a login form with SSO login option
2. **Given** user is on the login page, **When** user initiates SSO login, **Then** user is redirected to the external identity provider's login page
3. **Given** user completes authentication at the external IdP, **When** OAuth2 callback completes successfully, **Then** user returns to the dashboard authenticated
4. **Given** user has an active session, **When** user views their authentication status, **Then** system displays username, email, group memberships, derived roles, and authentication type (OIDC)
5. **Given** user is authenticated via external IdP, **When** JWT contains group membership claims, **Then** system maps groups to roles according to configured mappings

---

### User Story 4 - See Only Authorized States (Priority: P1)

When authentication is enabled, the dashboard automatically filters the states list to show only states the user is authorized to view based on their role's user scope (label-based filtering).

**Why this priority**: This is critical for security - users must not see states they don't have access to. Without this, role-based access control is meaningless.

**Independent Test**: Can be fully tested by creating states with different label combinations (e.g., env=dev, env=prod, product=foo), logging in as users with different role scopes, and verifying each user only sees states matching their role's user scope expression.

**Acceptance Scenarios**:

1. **Given** user with role having user scope `env=="dev"` is authenticated, **When** user views the dashboard, **Then** only states with env=dev label are displayed
2. **Given** user with role having user scope `env=="prod"` is authenticated, **When** user views the dashboard, **Then** only states with env=prod label are displayed
3. **Given** user with role having user scope `env=="dev" && product=="foo"` is authenticated, **When** user views the dashboard, **Then** only states matching both label conditions are displayed
4. **Given** user has no role assignments, **When** user views the dashboard, **Then** dashboard shows empty state list with message indicating no accessible states
5. **Given** the dashboard is displaying filtered states, **When** a state is created that the user cannot access, **Then** that state does not appear in the user's dashboard view

---

### User Story 5 - View Authentication Status and Role Information (Priority: P2)

Users can view their current authentication status, username, email, authentication type, and assigned roles through a user menu dropdown.

**Why this priority**: This provides transparency and helps users understand their identity and assigned roles, but the dashboard can function without it.

**Independent Test**: Can be fully tested by logging in, clicking the user menu in the header, and verifying that user information, authentication type, roles, and groups (if external IdP) are displayed correctly.

**Acceptance Scenarios**:

1. **Given** user is authenticated, **When** user clicks on their username in the header, **Then** a dropdown displays showing email, authentication type, roles, and session expiration
2. **Given** user authenticated via internal IdP, **When** user opens the authentication status dropdown, **Then** system displays username, email, roles, authentication type (Basic Auth), and session expiration time
3. **Given** user authenticated via external IdP, **When** user opens the authentication status dropdown, **Then** system displays username, email, group memberships, derived roles, authentication type (OIDC), and session expiration time
4. **Given** user is viewing authentication status, **When** user has multiple roles assigned, **Then** all roles are displayed with distinct visual styling

---

### User Story 6 - Log Out (Priority: P2)

Users can log out of the application, clearing their session and requiring re-authentication to access the dashboard again.

**Why this priority**: Logout is important for security and shared computer scenarios, but the application can function without it (sessions will eventually expire).

**Independent Test**: Can be fully tested by logging in, clicking the logout button in the user menu, and verifying the session is cleared and the user is returned to the login page.

**Acceptance Scenarios**:

1. **Given** user is authenticated, **When** user clicks "Sign Out" in the user menu dropdown, **Then** user's session is cleared and user is logged out
2. **Given** user has logged out, **When** user attempts to access the dashboard, **Then** user is shown the login page
3. **Given** user is logging out, **When** logout completes, **Then** user sees the login page ready for re-authentication

---

### Edge Cases

- What happens when the authentication configuration changes while the webapp is loaded? Webapp should detect authentication errors on next API call and redirect to appropriate login method.
- What happens when the user's session expires while they are viewing the dashboard? The next API request should return 401, triggering redirect to the login page.
- What happens when a user with no role assignments attempts to log in? User can authenticate but sees an empty dashboard with a message indicating no accessible states.
- **What happens when the user's role assignments change while they are logged in?** *(Out of Scope)* User continues with current session using their session-cached roles until they log out and re-authenticate, at which point new roles are applied. Real-time role synchronization is not implemented in this feature. This is acceptable because: (1) role assignments typically change infrequently, (2) administrators can request users to re-login for urgent changes, (3) session tokens expire regularly (forcing re-auth), and (4) implementing real-time sync would require polling or WebSocket infrastructure.
- What happens when gridapi switches from authenticated to non-authenticated mode? Webapp should detect lack of authentication errors and function without authentication UI on next reload.
- What happens when network connectivity is lost during login? User should see a user-friendly error message indicating connection failure (see FR-007 for error message requirements).
- What happens when a user has group memberships but no group-to-role mappings exist (external IdP mode)? User can authenticate but has no roles assigned, resulting in empty dashboard view.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST detect whether gridapi requires authentication and which authentication mode is configured by checking /health and /auth/config endpoints
- **FR-002**: System MUST display a login form when authentication is required, adapting the UI based on authentication mode (internal IdP shows username/password form, external IdP shows SSO button)
- **FR-003**: System MUST support automatic session restoration when user returns to the webapp with valid session
- **FR-004**: System MUST provide a user menu displaying the authenticated user's username, email, authentication type, assigned roles, and session expiration time
- **FR-005**: System MUST display group memberships for users authenticated via external IdP
- **FR-006**: System MUST filter the states list based on the user's role scope, showing only states matching the boolean expression defined in the user's assigned roles
- **FR-007**: System MUST handle authentication errors with clear, actionable messaging:
  - Error messages MUST be non-technical (no stack traces, database errors, file paths, or internal system details)
  - Messages MUST guide users toward resolution (e.g., "Check your credentials and try again" instead of "Authentication failed")
  - Errors MUST be categorized distinctly: invalid credentials, network/connection errors, session expiry, configuration issues, server errors
  - Sensitive context (usernames, email addresses, hashed values) MUST NOT appear in user-facing error UI
  - Errors MUST be logged to browser console for debugging purposes (without exposing sensitive data to user view)
- **FR-008**: System MUST support logout functionality that clears the user's session and returns them to the login page
- **FR-009**: System MUST display an empty state message when authenticated users have no roles or their role scope matches no states
  - Empty state MUST include descriptive message explaining why no states are visible
  - Message copy MUST be user-friendly (e.g., "You don't have access to any states yet" or "No states match your permissions")
  - Empty state MAY include icon/visual indicator to distinguish from error or loading state
  - Empty state MAY include helpful text about contacting administrator or requesting access (if configured)
- **FR-010**: System MUST continue to function in non-authenticated mode when gridapi does not require authentication

### Out of Scope

- Deep linking and return_to parameter handling (user is redirected to dashboard root after login, not to originally requested page)
- Label validation policy viewer (unrelated to authentication flow)
- Advanced profile page with detailed permission breakdowns (authentication status dropdown provides sufficient information)

### Key Entities *(include if feature involves data)*

- **User**: Represents an authenticated user with attributes: username (string), email (string), authentication type (internal or external), assigned roles (list of strings), group memberships (list of strings, external IdP only)
- **Session**: Represents the user's authentication session with attributes: user reference, token, expiration time
- **Role**: Represents an authorization role with attributes: role name (string), user scope expression (boolean expression for label-based filtering)
- **AuthConfig**: Represents the gridapi authentication configuration with attributes: mode (internal-idp or external-idp), issuer URL (for external IdP), whether device flow is supported

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Users can complete the login flow and access the dashboard with automated session restoration (no manual clicks once authenticated)
- **SC-002**: Authenticated users see only states matching their role's user scope expression, with 100% accuracy in filtering
- **SC-003**: Session restoration on page load completes within 2 seconds, providing a seamless experience for returning users
- **SC-004**: Users can view their authentication status, roles, and (if external IdP) group memberships in a clear, readable format
- **SC-005**: The webapp functions correctly in both authenticated and non-authenticated gridapi modes without requiring configuration changes
- **SC-006**: Authentication errors display user-friendly messages that help users resolve issues without exposing sensitive system details
- **SC-007**: Users can switch between internal IdP and external IdP authentication modes without webapp code changes (runtime detection)

### Previous work

### Epic: grid-dd2e - WebApp AuthN/AuthZ Integration

This feature is part of the broader authentication and authorization integration work documented in spec 006-authz-authn-rbac. The following related features and tasks have been completed or are in progress:

**Closed Features:**
- Authorization interceptor framework with label-based access control
- State output authorization (list, read, write operations)
- Dependency authorization with two-check model (source read + destination write)
- Integration tests for authorization flows
- Internal IdP implementation in gridapi (Mode 2)
- External IdP integration in gridapi (Mode 1)
- JWT-based authentication with service accounts
- Group-based role mapping for external IdP

**In Progress (grid-dd2e Epic):**
- **grid-6ae7** - JS/SDK Auth Helpers for Browser: Browser-compatible authentication helpers (phase 3.10)
- **grid-d00f** - Webapp React Authentication Integration: React components for authentication including AuthContext, hooks, guards, login/callback pages (phase 3.11)

**Existing Gridapi Endpoints:**
- `/health` - Health check with oidc_enabled flag
- `/auth/config` - Authentication configuration discovery (mode, issuer, client_id, supports_device_flow)
- `/auth/login` - Initiates OIDC login flow (internal IdP)
- `/auth/callback` - Handles OIDC callback (internal IdP)
- `/auth/refresh` - Refreshes access token
- `/auth/logout` - Clears session

**Known Issues:**
- **grid-f5947b22** - ListStates currently uses global state:list permission without label filtering (P3, future work - will be addressed separately to filter by user scope)
