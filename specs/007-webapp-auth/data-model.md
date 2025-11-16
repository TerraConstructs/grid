# Data Model: WebApp User Login Flow with Role-Based Filtering

**Feature**: 007-webapp-auth
**Date**: 2025-11-04
**Status**: Draft

## Overview

This document defines the frontend data model for webapp authentication. All entities are **client-side representations** of data retrieved from gridapi. The authoritative models live in gridapi's backend (`cmd/gridapi/internal/db/models`). This document describes the TypeScript interfaces used in the webapp and js/sdk.

## Core Entities

### User

Represents an authenticated user in the webapp.

**Source**: Derived from gridapi session/JWT claims
**Lifecycle**: Created on successful authentication, cleared on logout or session expiry
**Storage**: React Context state (AuthContext)

**TypeScript Interface**:

```typescript
interface User {
  id: string;                    // User ID (sub claim from JWT)
  username: string;              // Display name
  email: string;                 // User email
  authType: 'internal' | 'external'; // Authentication mode
  roles: string[];               // Assigned role names
  groups?: string[];             // Group memberships (external IdP only)
}
```

**Attributes**:

- `id`: Unique identifier (JWT `sub` claim). Immutable.
- `username`: Human-readable display name. Used in UI (e.g., AuthStatus dropdown).
- `email`: User's email address. Used for identification and display.
- `authType`: Discriminates between internal IdP (username/password) and external IdP (SSO). Affects UI presentation (shows "Basic Auth" vs "OIDC (KeyCloak)").
- `roles`: List of role names assigned to user (e.g., ["admin", "editor"]). Derived from direct user-role mappings (internal IdP) or group-role mappings (external IdP).
- `groups`: Optional. Only present for external IdP users. JWT `groups` claim. Used to show group memberships in AuthStatus UI.

**Validation Rules**:

- `id`, `username`, `email`, `authType`, `roles` are required
- `roles` array may be empty (user with no role assignments)
- `groups` is optional and only meaningful for `authType === 'external'`

**Relationships**:

- User has many Roles (via `roles` array)
- User has many Groups (via `groups` array, external IdP only)

### Session

Represents the user's authentication session.

**Source**: Retrieved from `/api/auth/whoami` endpoint ⚠️ **REQUIRES BACKEND IMPLEMENTATION** (see `specs/007-webapp-auth/plan.md` Complexity Tracking)
**Lifecycle**: Created on login, restored on page load, destroyed on logout or expiry
**Storage**: httpOnly cookie (server-managed, not accessible to JavaScript), session metadata fetched via whoami endpoint and cached in AuthContext

**TypeScript Interface**:

```typescript
interface Session {
  user: User;               // Authenticated user information
  expiresAt: number;        // Unix timestamp (milliseconds)
  isLoading: boolean;       // Session check in progress
  error: string | null;     // Session error message
}
```

**Attributes**:

- `user`: Embedded User entity. Contains all user information.
- `expiresAt`: Session expiration time. Used to show countdown in AuthStatus UI. Sessions expire after 2 hours (gridapi default).
- `isLoading`: Boolean flag indicating session restoration or auth check in progress. Used to show loading spinner.
- `error`: Human-readable error message if session check fails (e.g., "Session expired. Please log in again."). `null` if no error.

**State Transitions**:

```
[Initial] -> (page load) -> [Loading]
[Loading] -> (session valid) -> [Authenticated]
[Loading] -> (session invalid/expired) -> [Unauthenticated]
[Authenticated] -> (401 response) -> [Unauthenticated]
[Authenticated] -> (logout) -> [Unauthenticated]
[Unauthenticated] -> (login success) -> [Authenticated]
```

**Validation Rules**:

- If `user` is present, session is authenticated
- If `user` is null and `error` is null, session is unauthenticated (not yet checked or logged out)
- If `user` is null and `error` is present, session check failed

### AuthConfig

Represents the gridapi authentication configuration.

**Source**: Retrieved from `/auth/config` endpoint
**Lifecycle**: Fetched once on app load, cached in AuthContext
**Storage**: React Context state

**TypeScript Interface**:

```typescript
interface AuthConfig {
  mode: 'internal-idp' | 'external-idp' | 'disabled'; // Auth mode
  issuer?: string;              // OIDC issuer URL (external IdP only)
  clientId?: string;            // Public client ID (external IdP only)
  audience?: string;            // Expected aud claim (external IdP only)
  supportsDeviceFlow: boolean;  // Whether device flow is available (CLI use)
}
```

**Attributes**:

- `mode`: Determines which login UI to show and which auth endpoints to use
  - `'internal-idp'`: Show username/password form, POST credentials to `/auth/login`
  - `'external-idp'`: Show SSO button, redirect to external IdP via `/auth/login`
  - `'disabled'`: No authentication required, don't show login UI
- `issuer`: OIDC issuer URL (e.g., `https://keycloak.example.com/realms/grid`). Only present for external IdP mode.
- `clientId`: Public OAuth2 client ID. Used by CLI for device flow. Not used by webapp.
- `audience`: Expected `aud` claim in JWTs. Used for validation. Not directly used by webapp (server-side validation).
- `supportsDeviceFlow`: Indicates if CLI device flow is available. Not used by webapp (webapp uses authorization code flow).

**Validation Rules**:

- If `mode === 'external-idp'`, `issuer` must be present
- If `mode === 'internal-idp'` or `mode === 'disabled'`, OIDC fields are unused

**Usage**:

- Fetched on app initialization to determine login UI
- Used by LoginPage to render appropriate form (username/password vs SSO)
- Used by AuthContext to conditionally enable auth checks

### AuthState (Context State)

Represents the complete authentication state in React Context.

**Source**: Managed by AuthContext reducer
**Lifecycle**: Initialized on app load, updated throughout session
**Storage**: React Context (in-memory)

**TypeScript Interface**:

```typescript
interface AuthState {
  user: User | null;           // Current authenticated user
  session: Session | null;     // Session metadata
  config: AuthConfig | null;   // Auth configuration
  loading: boolean;            // Auth check in progress
  error: string | null;        // Auth error message
}
```

**Attributes**:

- `user`: Current authenticated user. `null` if not authenticated.
- `session`: Session information including expiration. `null` if not authenticated.
- `config`: Auth configuration from gridapi. `null` if not yet fetched.
- `loading`: True during session restoration, login, or logout operations.
- `error`: Human-readable error message for auth failures. `null` if no error.

**State Machine**:

```
Initial State: { user: null, session: null, config: null, loading: true, error: null }

Actions:
- AUTH_CONFIG_LOADED: config = payload
- SESSION_RESTORE_START: loading = true
- SESSION_RESTORE_SUCCESS: user = payload.user, session = payload, loading = false
- SESSION_RESTORE_FAILED: user = null, loading = false, error = payload
- LOGIN_START: loading = true, error = null
- LOGIN_SUCCESS: user = payload.user, session = payload, loading = false
- LOGIN_FAILED: loading = false, error = payload
- LOGOUT: user = null, session = null, loading = false
- SESSION_EXPIRED: user = null, session = null, error = "Session expired"
```

## Entity Relationships

```
AuthState
  ├── user: User (0..1)
  ├── session: Session (0..1)
  │   └── user: User
  └── config: AuthConfig (0..1)

User
  ├── roles: string[] (0..*)
  └── groups: string[] (0..*) [external IdP only]

Session
  ├── user: User
  ├── expiresAt: number
  ├── isLoading: boolean
  └── error: string | null
```

**Relationship Notes**:
- **Session vs AuthState**: Session is server-side (database), AuthState is client-side (React Context)
- **User Embedding**: Session embeds User for convenience (denormalized)
- **Config Independence**: AuthConfig loaded once, cached separately from user session

## Derived Entities

### LoginCredentials

Credentials submitted via login form.

**TypeScript Interface**:

```typescript
interface LoginCredentials {
  username: string;  // Username or email
  password: string;  // User password
}
```

**Usage**: Internal to LoginPage component, submitted to auth service.

**Validation Rules**:

- Both fields required
- No length constraints on frontend (validated by gridapi)

### LoginResponse

Response from gridapi auth endpoints.

**TypeScript Interface**:

```typescript
interface LoginResponse {
  user: User;          // Authenticated user
  expiresAt: number;   // Session expiration timestamp
}
```

**Source**: Returned by `/auth/login` and `/auth/callback` endpoints
**Usage**: Converted to Session object in AuthContext

## Entity Relationships

```
AuthState
  ├── user: User (0..1)
  ├── session: Session (0..1)
  │   └── user: User
  └── config: AuthConfig (0..1)

User
  ├── roles: string[] (0..*)
  └── groups: string[] (0..*) [external IdP only]

Session
  ├── user: User
  ├── expiresAt: number
  ├── isLoading: boolean
  └── error: string | null
```

## Validation Summary

### User Validation

- `id`: Non-empty string
- `username`: Non-empty string
- `email`: Non-empty string, valid email format
- `authType`: One of ['internal', 'external']
- `roles`: Array of strings (may be empty)
- `groups`: Optional array of strings (only for external authType)

### Session Validation

- If `user` is present, session is authenticated
- `expiresAt` must be future timestamp
- `error` is null when session is valid

### AuthConfig Validation

- `mode`: One of ['internal-idp', 'external-idp', 'disabled']
- If `mode === 'external-idp'`: `issuer` must be valid URL
- `supportsDeviceFlow`: Boolean

### AuthState Validation

- At most one of `user` or `error` should be set
- `loading` is true during transitions, false when stable
- `config` should be loaded before attempting login

## Implementation Notes

### State Management

All entities except AuthConfig are ephemeral (cleared on logout). AuthConfig is cached for the session to avoid repeated `/auth/config` calls.

### Security Considerations

- **User object never contains passwords or tokens** - these are httpOnly cookie only
- **Session expiry is client-side hint only** - server enforces actual expiration
- **Role/group memberships are display-only** - server enforces authorization
- **AuthConfig may be cached** - but webapp should handle runtime config changes (detect on next API call failure)

### TypeScript Implementation

All interfaces will be defined in:
- `webapp/src/types/auth.ts` - Core auth types
- `js/sdk/auth.ts` - SDK-level types (LoginResponse, etc.)

Types are shared between webapp and js/sdk via package exports.
