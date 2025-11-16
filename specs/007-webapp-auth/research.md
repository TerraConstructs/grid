# Research: React WebApp Authentication with Connect RPC Backend

**Feature**: 007-webapp-auth
**Date**: 2025-11-04
**Status**: Complete

## Overview

This document provides comprehensive research findings for implementing authentication in React web applications that consume Connect RPC backends, with specific focus on Grid's dual authentication mode architecture (internal IdP and external IdP/SSO).

---

## Prior Work: Existing Auth Infrastructure in Gridapi

### Authentication Endpoints

Grid's backend already implements comprehensive authentication endpoints:

#### 1. `/health` - Authentication Discovery
Returns service health with authentication status:
```json
{
  "status": "healthy",
  "oidc_enabled": true
}
```

#### 2. `/auth/config` - Configuration Discovery
Returns authentication mode and configuration for SDK/webapp:
```json
{
  "mode": "external-idp",  // or "internal-idp"
  "issuer": "https://keycloak.example.com/realms/grid",
  "client_id": "grid-cli",  // for device flow
  "audience": "grid-api",
  "supports_device_flow": true
}
```

**Response Structure** (from `auth_handlers.go:124-131`):
- **Mode**: `"external-idp"` (SSO with Keycloak/Azure/Okta) or `"internal-idp"` (Grid's built-in IdP)
- **Issuer**: OIDC issuer URL for token validation
- **ClientID**: Public client ID for device flow (Mode 1 only, null for Mode 2)
- **Audience**: Expected `aud` claim in access tokens
- **SupportsDeviceFlow**: Boolean indicating CLI device flow support

#### 3. `/auth/login` - SSO Login Initiation (Internal IdP Only)
Initiates OAuth2 Authorization Code Flow with PKCE:
- Generates cryptographic `state` parameter (stored in httpOnly cookie)
- Redirects to OIDC provider's authorization endpoint
- Returns redirect URL for browser navigation

**Implementation** (from `auth_handlers.go:15-25`):
```go
func HandleSSOLogin(rp *auth.RelyingParty) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        state, err := auth.GenerateNonce()
        if err != nil {
            http.Error(w, "Failed to generate state", http.StatusInternalServerError)
            return
        }
        auth.SetStateCookie(w, state)
        http.Redirect(w, r, rp.AuthCodeURL(state), http.StatusFound)
    }
}
```

#### 4. `/auth/callback` - OAuth2 Callback Handler (Internal IdP Only)
Handles OAuth2 callback after successful authentication:
- Verifies `state` parameter against cookie (CSRF protection)
- Exchanges authorization code for tokens
- Validates ID token claims (issuer, audience, signature)
- Creates or retrieves user record via JIT provisioning
- Creates session record in database
- Sets httpOnly session cookie with access token
- Redirects to webapp root (`/`)

**Session Cookie Configuration** (from `auth_handlers.go:77-86`):
```go
cookie := &http.Cookie{
    Name:     auth.SessionCookieName,  // "grid_session"
    Value:    tokens.AccessToken,
    Path:     "/",
    Expires:  tokens.Expiry,
    HttpOnly: true,
    Secure:   r.URL.Scheme == "https",
    SameSite: http.SameSiteLaxMode,
}
```

#### 5. `/auth/logout` - Session Revocation
Revokes user session:
- Extracts principal from authenticated context
- Revokes session in database
- Clears session cookie (sets expiry to Unix epoch)
- Returns 200 OK

**Implementation** (from `auth_handlers.go:94-122`):
```go
func HandleLogout(deps *gridmiddleware.AuthnDependencies) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        principal, ok := auth.GetUserFromContext(r.Context())
        if !ok {
            http.Error(w, "No active session", http.StatusUnauthorized)
            return
        }

        if err := deps.Sessions.Revoke(r.Context(), principal.SessionID); err != nil {
            http.Error(w, "Failed to revoke session", http.StatusInternalServerError)
            return
        }

        // Clear the session cookie
        cookie := &http.Cookie{
            Name:     auth.SessionCookieName,
            Value:    "",
            Path:     "/",
            Expires:  time.Unix(0, 0),
            HttpOnly: true,
            Secure:   r.URL.Scheme == "https",
            SameSite: http.SameSiteLaxMode,
        }
        http.SetCookie(w, cookie)

        w.WriteHeader(http.StatusOK)
        _, _ = w.Write([]byte("Logged out"))
    }
}
```

### Server-Side Authentication Flow

The backend implements a sophisticated authentication middleware stack:

#### JWT Validation Middleware
- Extracts JWT from `Authorization: Bearer <token>` header OR httpOnly cookie
- Validates JWT signature, issuer, audience, expiration
- Extracts claims (`sub`, `jti`, `email`, `name`, `groups`)
- Stores validated claims in request context

#### JTI Revocation Check
- Extracts JTI (JWT ID) from claims
- Checks revoked_jtis table for revocation status
- Rejects request if token is revoked

#### Principal Resolution (from `authn.go:138-190`)
- Service accounts: subject starts with `sa:` → lookup by client_id
- Users: lookup by subject claim
- **JIT Provisioning (External IdP only)**: Automatically creates user record if not found
- Disabled account check: rejects if user/SA is disabled

#### Dynamic Casbin Grouping (from `authn.go:100-114`)
- Extracts groups from JWT claims (configurable claim path)
- Builds group→role mapping from database
- Applies dynamic role assignments to Casbin enforcer
- Stores principal and groups in request context

#### Authorization Interceptor
- Resolves object type and action from request path
- Extracts resource labels (for label-based scoping)
- Enforces Casbin policy: `Enforce(subject, objType, action, labels)`
- Returns 403 Forbidden if denied

### Authentication Modes

Grid supports two deployment modes:

#### Mode 1: External IdP (SSO)
- Organization uses existing identity provider (Keycloak, Azure Entra ID, Okta)
- Webapp redirects to external IdP for authentication
- Supports OAuth2 device flow for CLI
- Group-based role mapping (groups from JWT → roles in Grid)
- JIT user provisioning on first login

**Example Configuration**:
```yaml
oidc:
  external_idp:
    issuer: "https://keycloak.example.com/realms/grid"
    client_id: "grid-api"
    cli_client_id: "grid-cli"
  groups_claim_field: "groups"  # or "resource_access.grid-api.roles"
  email_claim_field: "email"
```

#### Mode 2: Internal IdP
- Grid acts as OIDC provider (simplified authentication)
- Users authenticate with username/password (basic auth)
- Service accounts only (no device flow)
- Direct role assignment (no group mapping)

**Example Configuration**:
```yaml
oidc:
  issuer: "https://grid.example.com"
  client_id: "grid-internal"
```

### Session Management

**Server-Side Sessions**:
- Session record stored in `sessions` table
- Contains: user_id, token_hash, expires_at, id_token, last_used_at
- Session cookie stores actual access token (JWT)
- Middleware validates token and checks session validity on each request

**Token Lifecycle**:
- Access tokens default to 120 minutes (2 hours)
- Session expires when token expires (no automatic refresh for web sessions)
- Logout revokes session and clears cookie

### Connect RPC Service Methods

The backend exposes Connect RPC methods for user/role management:

#### User & Service Account Management
- `CreateServiceAccount` - Generate service account credentials
- `ListServiceAccounts` - List all service accounts
- `RevokeServiceAccount` - Disable service account
- `RotateServiceAccount` - Rotate secret

#### Role & Permission Management
- `CreateRole` - Create new role with permissions
- `ListRoles` - List all roles
- `UpdateRole` - Modify role permissions
- `DeleteRole` - Remove role
- `AssignRole` - Assign role to user/service account
- `RemoveRole` - Remove role assignment
- `AssignGroupRole` - Assign role to group (external IdP)
- `RemoveGroupRole` - Remove group role assignment
- `ListGroupRoles` - List group role assignments
- `GetEffectivePermissions` - Get aggregated permissions for principal

#### Session Management
- `ListSessions` - List active sessions for user
- `RevokeSession` - Revoke specific session

### Authorization Model

Grid uses Casbin for RBAC with label-based scoping:

**Casbin Model** (from `research.md:74-92`):
```
[request_definition]
r = sub, objType, act, labels

[policy_definition]
p = role, objType, act, scopeExpr, eft

[role_definition]
g = _, _

[policy_effect]
e = some(where (p.eft == allow)) && !some(where (p.eft == deny))

[matchers]
m = g(r.sub, p.role) && r.objType == p.objType && r.act == p.act && bexprMatch(p.scopeExpr, r.labels)
```

**Key Concepts**:
- **User Scope**: Boolean expression (go-bexpr) that filters which resources a role can access
  - Example: `env == "dev"` (only dev environment)
  - Example: `env == "dev" && product == "foo"` (dev + specific product)
- **Create Constraints**: Label validation rules enforced during state creation
- **Immutable Keys**: Labels that cannot be modified after creation
- **Union Semantics**: Multiple roles use OR logic (any matching role grants access)

---

## 1. React Auth State Management

### Decision: Context API + useReducer Pattern

Use React Context API with `useReducer` for authentication state management, avoiding third-party state management libraries.

### Rationale

1. **Simplicity**: Context API is built into React, no additional dependencies
2. **Performance**: Using `useMemo` to memoize context values prevents unnecessary re-renders
3. **Predictability**: `useReducer` provides structured state updates with action types
4. **Sufficient for Auth**: Authentication state is relatively simple (user, loading, error) and doesn't need Redux's complexity
5. **Co-location**: Auth logic stays close to components that need it

### Pattern Structure

```typescript
// types.ts
export type AuthState = 
  | { status: 'initializing' }
  | { status: 'unauthenticated' }
  | { status: 'authenticated'; session: Session };

export interface Session {
  user: User;
  token: string;
  expiresAt: number;
}

export interface User {
  id: string;
  username: string;
  email: string;
  authType: 'oidc' | 'basic';
  roles: string[];
  groups?: string[];  // External IdP only
}

type AuthAction =
  | { type: 'INITIALIZE'; session: Session | null }
  | { type: 'LOGIN_SUCCESS'; session: Session }
  | { type: 'LOGOUT' }
  | { type: 'SESSION_EXPIRED' };

// AuthContext.tsx
const AuthContext = createContext<{
  state: AuthState;
  login: (credentials: LoginCredentials) => Promise<void>;
  logout: () => Promise<void>;
  refreshSession: () => Promise<void>;
} | null>(null);

function authReducer(state: AuthState, action: AuthAction): AuthState {
  switch (action.type) {
    case 'INITIALIZE':
      return action.session
        ? { status: 'authenticated', session: action.session }
        : { status: 'unauthenticated' };
    case 'LOGIN_SUCCESS':
      return { status: 'authenticated', session: action.session };
    case 'LOGOUT':
    case 'SESSION_EXPIRED':
      return { status: 'unauthenticated' };
    default:
      return state;
  }
}

export function AuthProvider({ children }: { children: ReactNode }) {
  const [state, dispatch] = useReducer(authReducer, { status: 'initializing' });

  // Initialize session on mount
  useEffect(() => {
    const initSession = async () => {
      const session = await authService.getSessionFromCookie();
      dispatch({ type: 'INITIALIZE', session });
    };
    initSession();
  }, []);

  const login = useCallback(async (credentials: LoginCredentials) => {
    const session = await authService.login(credentials);
    dispatch({ type: 'LOGIN_SUCCESS', session });
  }, []);

  const logout = useCallback(async () => {
    await authService.logout();
    dispatch({ type: 'LOGOUT' });
  }, []);

  const refreshSession = useCallback(async () => {
    try {
      const session = await authService.getSessionFromCookie();
      if (session) {
        dispatch({ type: 'INITIALIZE', session });
      } else {
        dispatch({ type: 'SESSION_EXPIRED' });
      }
    } catch {
      dispatch({ type: 'SESSION_EXPIRED' });
    }
  }, []);

  const value = useMemo(
    () => ({ state, login, logout, refreshSession }),
    [state, login, logout, refreshSession]
  );

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

export function useAuth() {
  const context = useContext(AuthContext);
  if (!context) {
    throw new Error('useAuth must be used within AuthProvider');
  }
  return context;
}
```

### Token Storage Strategy

**Decision**: HttpOnly Cookies (Server-Managed)

Grid's backend already implements httpOnly cookie-based sessions:
- Session cookie set by `/auth/callback` endpoint
- Cookie name: `grid_session`
- Attributes: `HttpOnly=true`, `SameSite=Lax`, `Secure` (when HTTPS)
- Contains JWT access token
- Automatically sent with all requests (no JS access needed)

**Advantages**:
- XSS protection: JavaScript cannot access token
- Automatic cookie sending: Browser includes cookie in all requests
- CSRF protection: `SameSite=Lax` prevents cross-site cookie sending
- Server-side revocation: Backend can invalidate sessions in database

**Disadvantages**:
- Cannot read token expiry from JavaScript (need to check session on load)
- CORS configuration required (credentials: 'include')
- Cannot manually add token to requests (rely on browser cookie sending)

### Session Restoration Pattern

```typescript
// On app initialization
export function AuthProvider({ children }: { children: ReactNode }) {
  const [state, dispatch] = useReducer(authReducer, { status: 'initializing' });

  useEffect(() => {
    const initSession = async () => {
      try {
        // Call a protected endpoint to verify session validity
        const response = await gridApi.getAuthStatus();
        
        if (response.authenticated) {
          const session: Session = {
            user: {
              id: response.user.id,
              username: response.user.username,
              email: response.user.email,
              authType: response.auth_type,
              roles: response.roles,
              groups: response.groups,
            },
            token: '',  // Token in httpOnly cookie (not accessible)
            expiresAt: response.expires_at,
          };
          dispatch({ type: 'INITIALIZE', session });
        } else {
          dispatch({ type: 'INITIALIZE', session: null });
        }
      } catch (error) {
        // Session invalid or expired
        dispatch({ type: 'INITIALIZE', session: null });
      }
    };

    initSession();
  }, []);

  // Show loading spinner during initialization
  if (state.status === 'initializing') {
    return <LoadingSpinner />;
  }

  // Rest of provider implementation...
}
```

**Key Points**:
1. Call protected endpoint on mount to verify session
2. If successful, extract user info and populate context
3. If failed (401), treat as unauthenticated
4. Show loading state during initialization to prevent flash of login page

### Alternatives Considered

#### Alternative 1: localStorage Token Storage
**Pros**: Simple, works offline, can read expiry
**Cons**: Vulnerable to XSS attacks, requires manual header management
**Rejected**: Grid uses httpOnly cookies for security

#### Alternative 2: Redux + Redux Toolkit
**Pros**: Powerful dev tools, time-travel debugging, standardized patterns
**Cons**: Additional bundle size (~10KB), unnecessary complexity for simple auth state
**Rejected**: Context API is sufficient for Grid's auth needs

#### Alternative 3: Zustand or Jotai
**Pros**: Lightweight, simple API, good performance
**Cons**: Additional dependency, less familiar to React developers
**Rejected**: Context API is standard and sufficient

---

## 1.5. Authentication Service Layer

### Decision: Separate Service Module for Auth Operations

Create `webapp/src/services/authApi.ts` to separate authentication HTTP operations from React state management, improving testability and code organization.

### Rationale

1. **Testability**: Mock service functions independently of React Context
2. **Reusability**: Share auth logic across components without prop drilling
3. **Separation of Concerns**: Keep AuthContext focused on state management, not HTTP details
4. **Consistency**: Follow existing Grid pattern (`webapp/src/services/gridApi.ts`)

### Implementation Pattern

```typescript
// webapp/src/services/authApi.ts
import type { User, AuthConfig } from '../types/auth';

/**
 * Fetch authentication configuration from gridapi
 * Determines which auth mode is enabled (internal, external, or disabled)
 */
export async function getAuthConfig(): Promise<AuthConfig | null> {
  try {
    const response = await fetch('/auth/config', {
      method: 'GET',
      credentials: 'include',
    });

    if (!response.ok) {
      // Auth not configured
      return null;
    }

    return response.json();
  } catch (error) {
    console.error('Failed to fetch auth config:', error);
    return null;
  }
}

/**
 * Check current session status and retrieve user info
 * Called on app initialization to restore session from httpOnly cookie
 */
export async function getSessionStatus(): Promise<{ user: User; expiresAt: number } | null> {
  try {
    const response = await fetch('/api/auth/whoami', {
      method: 'GET',
      credentials: 'include',
    });

    if (!response.ok) {
      return null;
    }

    const data = await response.json();
    return {
      user: {
        id: data.user.id,
        username: data.user.username,
        email: data.user.email,
        authType: data.user.auth_type,
        roles: data.user.roles || [],
        groups: data.user.groups || [],
      },
      expiresAt: data.session.expires_at,
    };
  } catch (error) {
    console.error('Failed to check session:', error);
    return null;
  }
}

/**
 * Login with internal IdP (username/password)
 * Only available when auth mode is 'internal-idp'
 */
export async function loginInternal(
  username: string,
  password: string
): Promise<{ user: User; expiresAt: number }> {
  const response = await fetch('/auth/login', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    credentials: 'include',
    body: JSON.stringify({ username, password }),
  });

  if (!response.ok) {
    const error = await response.text();
    throw new Error(error || 'Login failed');
  }

  return response.json();
}

/**
 * Initiate external IdP login (OAuth2/OIDC)
 * Redirects browser to external identity provider
 */
export function initiateExternalLogin(redirectUri?: string): void {
  const params = redirectUri ? `?redirect_uri=${encodeURIComponent(redirectUri)}` : '';
  window.location.href = `/auth/login${params}`;
}

/**
 * Logout current session
 * Revokes session on server and clears httpOnly cookie
 */
export async function logout(): Promise<void> {
  await fetch('/auth/logout', {
    method: 'POST',
    credentials: 'include',
  });
}
```

### Usage in AuthContext

```typescript
// webapp/src/context/AuthContext.tsx
import * as authApi from '../services/authApi';

export function AuthProvider({ children }: { children: ReactNode }) {
  const [state, dispatch] = useReducer(authReducer, { status: 'initializing' });

  // Initialize session on mount
  useEffect(() => {
    const initSession = async () => {
      const config = await authApi.getAuthConfig();
      dispatch({ type: 'CONFIG_LOADED', payload: config });

      if (config?.mode !== 'disabled') {
        const sessionData = await authApi.getSessionStatus();
        dispatch({ type: 'INITIALIZE', session: sessionData });
      } else {
        dispatch({ type: 'INITIALIZE', session: null });
      }
    };
    initSession();
  }, []);

  const login = useCallback(async (credentials: LoginCredentials) => {
    const sessionData = await authApi.loginInternal(credentials.username, credentials.password);
    dispatch({ type: 'LOGIN_SUCCESS', session: sessionData });
  }, []);

  const logout = useCallback(async () => {
    await authApi.logout();
    dispatch({ type: 'LOGOUT' });
  }, []);

  // ... rest of provider
}
```

### Testing Benefits

**Service layer tests** (pure functions, no React dependencies):
```typescript
// services/__tests__/authApi.test.ts
import { describe, it, expect, beforeEach, vi } from 'vitest';
import { getSessionStatus, loginInternal } from '../authApi';

describe('authApi', () => {
  beforeEach(() => {
    global.fetch = vi.fn();
  });

  it('returns null when session check fails', async () => {
    (global.fetch as ReturnType<typeof vi.fn>).mockResolvedValue({
      ok: false,
      status: 401,
    });

    const result = await getSessionStatus();
    expect(result).toBeNull();
  });

  it('parses user data from whoami response', async () => {
    (global.fetch as ReturnType<typeof vi.fn>).mockResolvedValue({
      ok: true,
      json: async () => ({
        user: { id: '1', username: 'test', email: 'test@example.com', auth_type: 'internal', roles: ['admin'] },
        session: { expires_at: 1234567890 },
      }),
    });

    const result = await getSessionStatus();
    expect(result?.user.username).toBe('test');
  });
});
```

**AuthContext tests** (mock service layer):
```typescript
// context/__tests__/AuthContext.test.tsx
import { describe, it, expect, vi } from 'vitest';
import { render } from '@testing-library/react';
import * as authApi from '../../services/authApi';

vi.mock('../../services/authApi');

describe('AuthProvider', () => {
  it('restores session on mount', async () => {
    vi.spyOn(authApi, 'getSessionStatus').mockResolvedValue({
      user: { id: '1', username: 'test', /* ... */ },
      expiresAt: Date.now() + 3600000,
    });

    const { findByText } = render(<AuthProvider><TestComponent /></AuthProvider>);
    expect(await findByText('Authenticated: test')).toBeInTheDocument();
  });
});
```

### Alternatives Considered

#### Alternative 1: Inline fetch calls in AuthContext
**Rejected**: Mixes HTTP concerns with state management, harder to test

#### Alternative 2: Custom hooks (useLogin, useLogout)
**Rejected**: Over-engineered for simple HTTP calls, adds unnecessary indirection

---

## 2. Connect RPC Authentication

### Decision: Transport-Level Interceptor with Credentials

Use Connect's interceptor API to handle authentication globally at the transport level, with credentials mode enabled for cookie-based auth.

### Implementation Pattern

```typescript
// authInterceptor.ts
import { Interceptor } from '@connectrpc/connect';

export function createAuthInterceptor(
  onAuthError: () => void
): Interceptor {
  return (next) => async (req) => {
    try {
      const response = await next(req);
      return response;
    } catch (error) {
      // Handle authentication errors
      if (error instanceof ConnectError) {
        if (error.code === Code.Unauthenticated) {
          // Session expired or invalid
          onAuthError();
        }
      }
      throw error;
    }
  };
}

// gridApi.ts
import { createConnectTransport } from '@connectrpc/connect-web';
import { createAuthInterceptor } from './authInterceptor';

export function createGridTransport(
  baseUrl: string,
  onAuthError: () => void
): Transport {
  return createConnectTransport({
    baseUrl,
    credentials: 'include',  // CRITICAL: Send cookies with requests
    interceptors: [
      createAuthInterceptor(onAuthError),
    ],
  });
}

// App.tsx
function App() {
  const { logout } = useAuth();
  
  const transport = useMemo(
    () => createGridTransport(
      import.meta.env.VITE_GRID_API_URL || 'http://localhost:8080',
      () => {
        // Handle auth errors globally
        logout();
      }
    ),
    [logout]
  );

  const gridApi = useMemo(() => new GridApiAdapter(transport), [transport]);

  return (
    <TransportProvider transport={transport}>
      <AuthProvider>
        <YourApp />
      </AuthProvider>
    </TransportProvider>
  );
}
```

### Handling 401 Responses

```typescript
export function createAuthInterceptor(
  onAuthError: () => void,
  retryLimit = 1
): Interceptor {
  return (next) => async (req) => {
    let retries = 0;

    const attemptRequest = async (): Promise<UnaryResponse> => {
      try {
        return await next(req);
      } catch (error) {
        if (error instanceof ConnectError) {
          if (error.code === Code.Unauthenticated && retries < retryLimit) {
            // Attempt session refresh
            retries++;
            try {
              // Option 1: Call /auth/refresh endpoint
              await fetch('/auth/refresh', {
                method: 'POST',
                credentials: 'include',
              });
              // Retry original request
              return await attemptRequest();
            } catch {
              // Refresh failed, trigger logout
              onAuthError();
              throw error;
            }
          } else if (error.code === Code.Unauthenticated) {
            // Out of retries or no refresh available
            onAuthError();
          }
        }
        throw error;
      }
    };

    return attemptRequest();
  };
}
```

**Note**: Grid's current implementation does not have `/auth/refresh` for web sessions. Sessions expire when the access token expires (120 minutes). The interceptor can be simplified to just call `onAuthError()` on 401, triggering logout.

### Retry Logic Considerations

**Decision**: No automatic retry for Grid webapp

**Rationale**:
1. Grid uses httpOnly cookie sessions with 2-hour expiry
2. No refresh token mechanism for web sessions (only for service accounts)
3. Retrying on 401 adds complexity without benefit
4. User should re-authenticate through login flow after session expiry

**Simplified Interceptor**:
```typescript
export function createAuthInterceptor(
  onAuthError: () => void
): Interceptor {
  return (next) => async (req) => {
    try {
      return await next(req);
    } catch (error) {
      if (error instanceof ConnectError && error.code === Code.Unauthenticated) {
        // Session expired or invalid - trigger logout
        onAuthError();
      }
      throw error;
    }
  };
}
```

### CORS Configuration

For httpOnly cookie-based auth to work, the backend must:
1. Set `Access-Control-Allow-Credentials: true`
2. Set `Access-Control-Allow-Origin` to specific origin (cannot use `*` with credentials)
3. Allow relevant headers in CORS preflight

Grid's backend should already handle this (Chi router with CORS middleware).

**Frontend Configuration**:
```typescript
const transport = createConnectTransport({
  baseUrl: 'https://gridapi.example.com',
  credentials: 'include',  // Send cookies with requests
});
```

### Alternatives Considered

#### Alternative 1: Request-Level Header Injection
**Pattern**: Add `Authorization: Bearer <token>` header to each request
```typescript
const authInterceptor: Interceptor = (next) => async (req) => {
  const token = localStorage.getItem('authToken');
  if (token) {
    req.header.set('Authorization', `Bearer ${token}`);
  }
  return await next(req);
};
```
**Rejected**: Grid uses httpOnly cookies for security; token not accessible to JavaScript

#### Alternative 2: Per-Request Credentials
**Pattern**: Pass credentials option on each RPC call
```typescript
await client.listStates({}, { credentials: 'include' });
```
**Rejected**: Repetitive and error-prone; transport-level configuration is cleaner

#### Alternative 3: Custom Fetch Implementation
**Pattern**: Override transport's fetch with custom implementation
**Rejected**: Connect's interceptor API is the standard approach

---

## 3. OIDC/OAuth2 Web Flows

### Decision: Server-Driven Authorization Code Flow with PKCE

Grid's backend already implements the full OAuth2 Authorization Code Flow with PKCE for external IdP mode. The webapp acts as a user-agent, initiating the flow via backend endpoints.

### Flow Diagram

```
┌─────────┐                ┌──────────┐                ┌───────────┐
│ Browser │                │  Gridapi │                │ External  │
│ (React) │                │ (Backend)│                │    IdP    │
└────┬────┘                └────┬─────┘                └─────┬─────┘
     │                          │                            │
     │  1. GET /auth/login      │                            │
     ├─────────────────────────>│                            │
     │                          │                            │
     │  2. Generate state       │                            │
     │     Set state cookie     │                            │
     │  3. 302 Redirect         │                            │
     │     Location: authz URL  │                            │
     │<─────────────────────────┤                            │
     │                          │                            │
     │  4. Follow redirect      │                            │
     │     (with state param)   │                            │
     ├────────────────────────────────────────────────────────>
     │                          │                            │
     │  5. User authenticates   │                            │
     │     at IdP login page    │                            │
     │                          │                            │
     │  6. 302 Redirect         │                            │
     │     Location: callback   │                            │
     │     ?code=xxx&state=yyy  │                            │
     │<────────────────────────────────────────────────────────
     │                          │                            │
     │  7. GET /auth/callback   │                            │
     │     ?code=xxx&state=yyy  │                            │
     ├─────────────────────────>│                            │
     │                          │                            │
     │  8. Verify state cookie  │                            │
     │  9. Exchange code        │                            │
     │     for tokens           │                            │
     │                          ├───────────────────────────>│
     │                          │    POST /token             │
     │                          │    (code, client_secret)   │
     │                          │<───────────────────────────┤
     │                          │    access_token, id_token  │
     │                          │                            │
     │ 10. Validate ID token    │                            │
     │ 11. Create/get user      │                            │
     │ 12. Create session       │                            │
     │ 13. Set session cookie   │                            │
     │ 14. 302 Redirect to /    │                            │
     │<─────────────────────────┤                            │
     │                          │                            │
     │ 15. Load webapp          │                            │
     │     (with session cookie)│                            │
     │                          │                            │
```

### State Parameter Validation

**Purpose**: Prevent CSRF attacks during OAuth2 callback

**Implementation** (handled by backend):
1. Generate cryptographically random `state` value (32 bytes, hex-encoded)
2. Store `state` in httpOnly cookie (`grid_oauth_state`)
3. Include `state` in authorization URL as query parameter
4. On callback, verify `state` query param matches cookie value
5. Clear `state` cookie after validation

**Backend Implementation** (from `auth_handlers.go`):
```go
// Generate state
state, err := auth.GenerateNonce()
auth.SetStateCookie(w, state)

// Verify state on callback
if err := auth.VerifyStateCookie(r, r.URL.Query().Get("state")); err != nil {
    http.Error(w, "Invalid state", http.StatusBadRequest)
    return
}
```

**Webapp Responsibility**: None - backend handles all state validation

### PKCE Implementation

**Purpose**: Prevent authorization code interception attacks

**Implementation** (handled by backend in internal IdP mode):
Grid's internal IdP mode (Mode 2) uses standard OAuth2 with client secret (not public client), so PKCE is not required. For external IdP mode (Mode 1), the external IdP handles PKCE if configured.

**Webapp Responsibility**: None - all PKCE logic handled by backend or external IdP

### Security Best Practices

#### 1. Use SameSite Cookies
Grid's backend sets `SameSite=Lax` on session cookies:
- Prevents CSRF attacks
- Allows cookies on top-level navigation (OAuth redirect)
- More secure than `SameSite=None`

#### 2. HttpOnly Cookies
Grid's session cookies are `HttpOnly=true`:
- Prevents XSS token theft
- JavaScript cannot access token
- Reduces attack surface

#### 3. Secure Flag
Grid sets `Secure=true` when HTTPS detected:
- Cookies only sent over HTTPS
- Prevents man-in-the-middle token theft

#### 4. State Parameter Validation
Grid validates state parameter on callback:
- Prevents CSRF attacks
- Ensures callback originated from legitimate authorization request

#### 5. Exact Redirect URI Matching
Grid's external IdP configuration must use exact redirect URI:
- No wildcard redirects
- Prevents open redirect vulnerabilities

#### 6. Token Expiry Enforcement
Grid enforces token expiry on every request:
- JWT `exp` claim validated
- Session record checked for expiry
- Expired sessions rejected with 401

### Handling Callbacks

**Webapp Implementation**:

```typescript
// CallbackPage.tsx
export function CallbackPage() {
  const navigate = useNavigate();
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    const handleCallback = async () => {
      try {
        // Backend handles all callback logic
        // Just wait for redirect to complete
        // If we're still on this page after 5 seconds, something went wrong
        const timeout = setTimeout(() => {
          setError('Authentication callback timed out. Please try again.');
        }, 5000);

        // Clean up timeout if component unmounts
        return () => clearTimeout(timeout);
      } catch (err) {
        setError(err instanceof Error ? err.message : 'Authentication failed');
      }
    };

    handleCallback();
  }, [navigate]);

  if (error) {
    return (
      <div className="error">
        <h2>Authentication Error</h2>
        <p>{error}</p>
        <button onClick={() => navigate('/login')}>Try Again</button>
      </div>
    );
  }

  return <LoadingSpinner message="Completing authentication..." />;
}
```

**Note**: In Grid's implementation, the backend automatically redirects to `/` after successful callback, so the webapp may never hit a dedicated callback page. The callback handling is entirely server-side.

### Detecting Auth Mode Changes

**Challenge**: Webapp must detect when gridapi switches authentication modes

**Solution**: Check `/auth/config` on app initialization

```typescript
export function useAuthConfig() {
  const [config, setConfig] = useState<AuthConfig | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    const fetchConfig = async () => {
      try {
        const response = await fetch('/auth/config');
        if (response.ok) {
          const data = await response.json();
          setConfig(data);
        } else {
          // Auth not configured
          setConfig(null);
        }
      } catch {
        // Auth not configured or network error
        setConfig(null);
      } finally {
        setLoading(false);
      }
    };

    fetchConfig();
  }, []);

  return { config, loading };
}

// App.tsx
function App() {
  const { config, loading } = useAuthConfig();

  if (loading) {
    return <LoadingSpinner />;
  }

  if (!config) {
    // Auth not required - show dashboard directly
    return <Dashboard />;
  }

  // Auth required - show login flow
  return (
    <AuthProvider config={config}>
      <Dashboard />
    </AuthProvider>
  );
}
```

### Alternatives Considered

#### Alternative 1: Client-Side Implicit Flow
**Pattern**: Use implicit flow with fragment-based token return
**Rejected**: Implicit flow is deprecated (OAuth 2.1), insecure, not supported by Grid

#### Alternative 2: Client-Side PKCE
**Pattern**: Generate code_verifier/code_challenge in JavaScript, handle token exchange
**Rejected**: Cannot securely store client secret in browser; Grid uses server-side flow

#### Alternative 3: Password Grant
**Pattern**: Send username/password directly to token endpoint
**Rejected**: Not supported for external IdP; only used for internal IdP basic auth

---

## 4. Session Management

### HttpOnly Cookie Handling from Frontend Perspective

**Key Concept**: Frontend cannot read or manipulate httpOnly cookies directly.

#### What the Frontend CAN Do:
1. **Initiate Login**: Navigate to `/auth/login` (triggers OAuth flow)
2. **Initiate Logout**: Call `/auth/logout` endpoint
3. **Rely on Cookies**: Browser automatically sends cookies with requests (when `credentials: 'include'`)
4. **Detect Auth State**: Call protected endpoints to verify session validity

#### What the Frontend CANNOT Do:
1. Read cookie value (httpOnly restriction)
2. Set or modify cookie (backend-only)
3. Check token expiry directly (need to call backend)
4. Manually refresh session (no refresh token in cookie)

### Session Restoration on Page Load

**Pattern**:
```typescript
// AuthProvider.tsx
export function AuthProvider({ children }: { children: ReactNode }) {
  const [state, dispatch] = useReducer(authReducer, { status: 'initializing' });

  useEffect(() => {
    const restoreSession = async () => {
      try {
        // Call a lightweight protected endpoint to verify session
        const response = await fetch('/api/auth/whoami', {
          credentials: 'include',  // Send session cookie
        });

        if (response.ok) {
          const userData = await response.json();
          const session: Session = {
            user: userData.user,
            token: '',  // Token in cookie, not accessible
            expiresAt: userData.expires_at,
          };
          dispatch({ type: 'INITIALIZE', session });
        } else {
          // Session invalid or expired
          dispatch({ type: 'INITIALIZE', session: null });
        }
      } catch {
        // Network error or session invalid
        dispatch({ type: 'INITIALIZE', session: null });
      }
    };

    restoreSession();
  }, []);

  // Show loading state during initialization
  if (state.status === 'initializing') {
    return (
      <div className="flex items-center justify-center min-h-screen">
        <LoadingSpinner />
      </div>
    );
  }

  return (
    <AuthContext.Provider value={{ state, login, logout }}>
      {children}
    </AuthContext.Provider>
  );
}
```

**⚠️ Backend Endpoint Required** (DOES NOT EXIST YET - must be implemented):

**Why Needed**:
- Grid currently has NO endpoint for webapp to restore session from httpOnly cookie
- `GetEffectivePermissions` RPC exists but insufficient (requires principal_id input, missing user identity/session/groups)
- Webapp needs user identity + session expiry + roles + groups for AuthContext state restoration

**Implementation Required**:
```go
// GET /api/auth/whoami
// Returns current user info if authenticated
// Location: cmd/gridapi/internal/server/auth_handlers.go
func HandleWhoAmI(w http.ResponseWriter, r *http.Request) {
    // 1. Extract principal from context (set by authn middleware)
    principal, ok := auth.GetUserFromContext(r.Context())
    if !ok {
        http.Error(w, "unauthenticated", http.StatusUnauthorized)
        return
    }

    // 2. Extract groups from context
    groups := auth.GetGroupsFromContext(r.Context())

    // 3. Look up session by principal.SessionID
    // NOTE: SessionID is currently NOT populated in authn.go - must fix first!
    session, err := deps.Sessions.GetByID(r.Context(), principal.SessionID)
    if err != nil {
        http.Error(w, "session not found", http.StatusUnauthorized)
        return
    }

    // 4. Build response
    response := map[string]interface{}{
        "user": map[string]interface{}{
            "id":        principal.InternalID,
            "subject":   principal.Subject,
            "username":  principal.Name,
            "email":     principal.Email,
            "auth_type": string(principal.Type),
            "roles":     principal.Roles,
            "groups":    groups,
        },
        "session": map[string]interface{}{
            "id":         session.ID,
            "expires_at": session.ExpiresAt.Unix(),
            "created_at": session.CreatedAt.Unix(),
        },
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(response)
}
```

**Additional Backend Changes Required**:
1. **Fix Bug**: Populate `AuthenticatedPrincipal.SessionID` in `cmd/gridapi/internal/middleware/authn.go` (currently declared but never set)
2. **Mount Endpoint**: Add `r.Get("/api/auth/whoami", HandleWhoAmI(&opts.AuthnDeps))` in `cmd/gridapi/internal/server/router.go`
3. **Verify Repository**: Ensure `session_repository.go` has `GetByID()` and `GetByTokenHash()` methods

**See**: `specs/007-webapp-auth/plan.md` Complexity Tracking section for detailed implementation requirements.

### Detecting Auth Mode Changes

**Challenge**: Gridapi may be reconfigured to change authentication mode while webapp is running

**Solution**: Handle 401 gracefully and re-check auth config

```typescript
export function createAuthInterceptor(
  onAuthError: () => void
): Interceptor {
  return (next) => async (req) => {
    try {
      return await next(req);
    } catch (error) {
      if (error instanceof ConnectError && error.code === Code.Unauthenticated) {
        // Could be session expiry OR auth mode change
        // Check if auth is still required
        try {
          const configResponse = await fetch('/auth/config');
          if (!configResponse.ok) {
            // Auth disabled - reload app
            window.location.reload();
            return;
          }
        } catch {
          // Network error or auth disabled
        }
        
        // Auth still required - trigger logout
        onAuthError();
      }
      throw error;
    }
  };
}
```

**Alternative Approach**: Periodic health check
```typescript
// App.tsx
useEffect(() => {
  // Check auth config every 5 minutes
  const interval = setInterval(async () => {
    try {
      const response = await fetch('/health');
      const health = await response.json();
      
      if (authRequired && !health.oidc_enabled) {
        // Auth was disabled - reload app
        console.log('Authentication disabled, reloading...');
        window.location.reload();
      } else if (!authRequired && health.oidc_enabled) {
        // Auth was enabled - reload app
        console.log('Authentication enabled, reloading...');
        window.location.reload();
      }
    } catch {
      // Ignore errors
    }
  }, 5 * 60 * 1000);

  return () => clearInterval(interval);
}, [authRequired]);
```

### Session Expiry Handling

**Pattern**: Detect 401 responses and redirect to login

```typescript
// AuthProvider.tsx
const handleAuthError = useCallback(() => {
  dispatch({ type: 'SESSION_EXPIRED' });
  // Optionally show notification
  toast.error('Your session has expired. Please log in again.');
}, []);

// In transport creation
const transport = createGridTransport(baseUrl, handleAuthError);
```

**User Experience**:
1. User is viewing dashboard
2. Session expires (2 hours)
3. User clicks refresh or navigates
4. API call returns 401
5. Interceptor catches error, calls `handleAuthError`
6. Auth state changes to `unauthenticated`
7. App redirects to login page (via conditional rendering)
8. User logs in again
9. Session restored

### Alternatives Considered

#### Alternative 1: Token Refresh Mechanism
**Pattern**: Use refresh tokens to extend session without re-login
**Rejected**: Grid's web sessions don't use refresh tokens (only service accounts do)

#### Alternative 2: localStorage Token Storage
**Pattern**: Store JWT in localStorage, manually add to requests
**Rejected**: Grid uses httpOnly cookies for security

#### Alternative 3: Silent iframe Renewal
**Pattern**: Use hidden iframe to renew tokens without user interaction
**Rejected**: Complex, requires IDP support, unnecessary with httpOnly cookies

---

## 5. Role-Based UI Filtering

### Decision: Fetch-and-Filter on Client Side

Fetch all states the user is authorized to see (server-side filtering) and optionally apply additional client-side filtering for UI purposes.

### Rationale

1. **Server-Side Authority**: Authorization decisions must be made server-side for security
2. **Grid's ListStates Behavior**: Currently returns all states user can access (based on role scope)
3. **Known Issue**: grid-f5947b22 tracks server-side label filtering for ListStates (future work)
4. **Client-Side Performance**: Small to medium datasets (100-1000 states) can be filtered efficiently in browser
5. **Consistency**: Same filtering logic used for graph and list views

### Implementation Pattern

#### Server-Side Filtering (Current State)

Grid's authorization middleware automatically filters resources based on role scope:

```go
// Authorization interceptor (simplified)
func AuthzInterceptor(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        principal := auth.GetUserFromContext(r.Context())
        
        // Get effective role scope (union of all roles)
        scopeExpr := getEffectiveScopeExpr(principal.Roles)
        
        // Store scope in context for repository layer
        ctx := context.WithValue(r.Context(), "scope_expr", scopeExpr)
        
        next.ServeHTTP(w, r.WithContext(ctx))
    })
}

// Repository layer
func (r *StateRepository) List(ctx context.Context) ([]State, error) {
    scopeExpr := ctx.Value("scope_expr").(string)
    
    // Apply scope filter to query
    query := r.db.NewSelect().Model(&State{})
    if scopeExpr != "" {
        // Filter by label scope using bexpr
        states, err := query.All()
        filtered := []State{}
        for _, state := range states {
            if evaluateBexpr(scopeExpr, state.Labels) {
                filtered = append(filtered, state)
            }
        }
        return filtered, nil
    }
    
    return query.All()
}
```

**Note**: This is the intended behavior for grid-f5947b22. Currently, ListStates returns all states without scope filtering.

#### Client-Side Filtering (Current Implementation)

```typescript
// useGridData.tsx
export function useGridData() {
  const [states, setStates] = useState<StateInfo[]>([]);
  const [filter, setFilter] = useState<string>('');  // bexpr filter
  
  const loadData = async ({ filter }: { filter?: string } = {}) => {
    // Fetch all authorized states from backend
    const response = await gridApi.listStates({
      filter,  // Optional client-side filter (not security boundary)
    });
    
    setStates(response.states);
  };
  
  // Client-side filtering for UI (not security boundary)
  const filteredStates = useMemo(() => {
    if (!filter) return states;
    
    return states.filter(state => {
      try {
        return evaluateBexpr(filter, state.labels);
      } catch {
        return false;
      }
    });
  }, [states, filter]);
  
  return {
    states: filteredStates,
    filter,
    loadData,
  };
}
```

### Security Implications

**Critical Principle**: Client-side filtering is NOT a security boundary.

1. **Server Authoritative**: Backend MUST enforce authorization on all read/write operations
2. **Defense in Depth**: Client-side filtering provides UX (hide unauthorized states) but is not trusted
3. **Audit Trail**: Backend logs all authorization decisions
4. **Immutable Enforcement**: Authorization logic lives in backend middleware, not client code

**Security Model**:
- ✅ Backend enforces role scope on ListStates, GetState, CreateState, UpdateState
- ✅ Backend validates labels against role constraints
- ✅ Backend returns 403 if user attempts unauthorized access
- ❌ Client UI hiding states is NOT security enforcement
- ❌ Client cannot bypass backend authorization

### Future Optimization: Server-Side Label Filtering

**Issue**: grid-f5947b22 - ListStates should filter by user scope server-side

**Current Behavior**:
```protobuf
message ListStatesRequest {
  string filter = 1;  // Optional bexpr filter (not security-enforced)
}
```

**Proposed Behavior**:
```go
func (s *StateService) ListStates(ctx context.Context, req *ListStatesRequest) (*ListStatesResponse, error) {
    principal := auth.GetUserFromContext(ctx)
    
    // Get effective scope from roles
    effectiveScopeExpr := getEffectiveScopeExpr(principal.Roles)
    
    // Combine role scope with optional user filter
    combinedFilter := effectiveScopeExpr
    if req.Filter != "" {
        combinedFilter = fmt.Sprintf("(%s) and (%s)", effectiveScopeExpr, req.Filter)
    }
    
    // Filter states in database query
    states, err := s.repo.ListStatesWithFilter(ctx, combinedFilter)
    if err != nil {
        return nil, err
    }
    
    return &ListStatesResponse{States: states}, nil
}
```

**Benefits**:
- Reduces data transfer (only send authorized states)
- Improves performance for large datasets
- Clearer security boundary (backend owns filtering)

**Migration Path**:
1. Backend implements server-side scope filtering (grid-f5947b22)
2. Webapp continues to work (no changes needed)
3. Optional: Remove client-side bexpr filtering (redundant)

### Alternatives Considered

#### Alternative 1: Full Client-Side Filtering
**Pattern**: Fetch ALL states, filter in browser based on user roles
**Rejected**: Leaks unauthorized state metadata to client (security issue)

#### Alternative 2: Separate Permission Check per State
**Pattern**: Call `CheckPermission(state_id)` for each state before displaying
**Rejected**: Inefficient (N+1 queries), high latency

#### Alternative 3: GraphQL with Field-Level Authorization
**Pattern**: Use GraphQL with @auth directives for fine-grained filtering
**Rejected**: Grid uses Connect RPC, not GraphQL; unnecessary complexity

---

## 5.5. Route Guards and Permission-Based UI Components

### Decision: Component-Based Route Protection

Use React components to guard routes and conditionally render UI elements based on authentication state and permissions.

### Route Guard Component (AuthGuard)

Protects entire routes, ensuring only authenticated users can access protected pages.

```typescript
// webapp/src/components/AuthGuard.tsx
import { ReactNode } from 'react';
import { Navigate, useLocation } from 'react-router-dom';
import { useAuth } from '../context/AuthContext';
import { LoadingSpinner } from './LoadingSpinner';

interface AuthGuardProps {
  children: ReactNode;
  fallbackPath?: string;
}

/**
 * Route guard component that ensures user is authenticated
 * Displays loading state during session check
 * Redirects to login page if unauthenticated
 */
export function AuthGuard({ children, fallbackPath = '/login' }: AuthGuardProps) {
  const { state } = useAuth();
  const location = useLocation();

  // Show loading spinner during initialization
  if (state.status === 'initializing') {
    return (
      <div className="flex items-center justify-center min-h-screen">
        <LoadingSpinner />
      </div>
    );
  }

  // Redirect to login if unauthenticated
  if (state.status === 'unauthenticated') {
    // Preserve intended destination for post-login redirect
    return <Navigate to={fallbackPath} state={{ from: location }} replace />;
  }

  // Render protected content
  return <>{children}</>;
}
```

**Usage in React Router**:

```typescript
// webapp/src/App.tsx
import { BrowserRouter, Routes, Route } from 'react-router-dom';
import { AuthGuard } from './components/AuthGuard';
import { LoginPage } from './components/LoginPage';
import { Dashboard } from './components/Dashboard';

export function App() {
  return (
    <BrowserRouter>
      <Routes>
        <Route path="/login" element={<LoginPage />} />

        {/* Protected routes */}
        <Route
          path="/"
          element={
            <AuthGuard>
              <Dashboard />
            </AuthGuard>
          }
        />

        <Route
          path="/states/:id"
          element={
            <AuthGuard>
              <StateDetailPage />
            </AuthGuard>
          }
        />
      </Routes>
    </BrowserRouter>
  );
}
```

### Permission-Based UI Component (ProtectedAction)

Conditionally renders UI elements based on user permissions or roles.

```typescript
// webapp/src/components/ProtectedAction.tsx
import { ReactNode } from 'react';
import { useAuth } from '../context/AuthContext';

interface ProtectedActionProps {
  requiredPermission: string;
  children: ReactNode;
  fallback?: ReactNode;
}

/**
 * Permission gate component for role-based UI rendering
 * Hides actions user is not authorized to perform
 *
 * Note: This is UI-only protection. Server MUST enforce authorization.
 */
export function ProtectedAction({
  requiredPermission,
  children,
  fallback = null,
}: ProtectedActionProps) {
  const { state } = useAuth();

  if (state.status !== 'authenticated') {
    return <>{fallback}</>;
  }

  const hasPermission = state.session.user.roles.includes(requiredPermission);

  if (!hasPermission) {
    return <>{fallback}</>;
  }

  return <>{children}</>;
}
```

**Usage in Dashboard**:

```typescript
// webapp/src/components/StateActions.tsx
import { ProtectedAction } from './ProtectedAction';

export function StateActions({ state }: { state: StateInfo }) {
  return (
    <div className="flex gap-2">
      {/* Read-only action - visible to all authenticated users */}
      <button onClick={() => viewState(state)}>View</button>

      {/* Write action - only visible to users with write permission */}
      <ProtectedAction requiredPermission="state:write">
        <button onClick={() => editState(state)}>Edit</button>
      </ProtectedAction>

      {/* Delete action - only visible to admins */}
      <ProtectedAction requiredPermission="state:delete">
        <button
          onClick={() => deleteState(state)}
          className="text-red-600"
        >
          Delete
        </button>
      </ProtectedAction>
    </div>
  );
}
```

### Dashboard Read-Only Scope

**Current Status**: Dashboard (webapp/) is **READ ONLY** in current feature scope (007-webapp-auth).

**Implications**:
- Auth protects existing read operations (`ListStates`, `GetState`, `ListDependencies`)
- No write operations (`CreateState`, `UpdateState`, `DeleteState`) exposed in UI
- `ProtectedAction` component prepared for future write operations, but not used yet
- Admin operations (role management, service account creation) remain **CLI-only**

**Read Operations Protected by Auth**:
- View state list (filtered by user's role scope)
- View state details
- View dependency graph
- View labels (filtered by scope)

**Out of Scope for Dashboard**:
- Creating new states (CLI via `gridctl state create`)
- Updating state labels (CLI via `gridctl state update-labels`)
- Deleting states (CLI, may be added to dashboard in future)
- Role management (CLI via `gridctl auth` commands)
- Service account management (CLI)

### Testing Route Guards

```typescript
// components/__tests__/AuthGuard.test.tsx
import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import { BrowserRouter } from 'react-router-dom';
import { AuthGuard } from '../AuthGuard';
import { useAuth } from '../../context/AuthContext';

vi.mock('../../context/AuthContext');

const mockUseAuth = useAuth as ReturnType<typeof vi.fn>;

describe('AuthGuard', () => {
  const TestComponent = () => <div>Protected Content</div>;

  it('shows loading spinner during initialization', () => {
    mockUseAuth.mockReturnValue({
      state: { status: 'initializing' },
    } as any);

    render(
      <BrowserRouter>
        <AuthGuard>
          <TestComponent />
        </AuthGuard>
      </BrowserRouter>
    );

    expect(screen.getByRole('status')).toBeInTheDocument();
  });

  it('redirects to login when unauthenticated', () => {
    mockUseAuth.mockReturnValue({
      state: { status: 'unauthenticated' },
    } as any);

    const { container } = render(
      <BrowserRouter>
        <AuthGuard>
          <TestComponent />
        </AuthGuard>
      </BrowserRouter>
    );

    // AuthGuard returns Navigate component, which causes navigation
    expect(container.textContent).toBe('');
  });

  it('renders protected content when authenticated', () => {
    mockUseAuth.mockReturnValue({
      state: {
        status: 'authenticated',
        session: {
          user: { id: '1', username: 'test', roles: ['admin'] },
          expiresAt: Date.now() + 3600000,
        },
      },
    } as any);

    render(
      <BrowserRouter>
        <AuthGuard>
          <TestComponent />
        </AuthGuard>
      </BrowserRouter>
    );

    expect(screen.getByText('Protected Content')).toBeInTheDocument();
  });
});
```

```typescript
// components/__tests__/ProtectedAction.test.tsx
import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import { ProtectedAction } from '../ProtectedAction';
import { useAuth } from '../../context/AuthContext';

vi.mock('../../context/AuthContext');

const mockUseAuth = useAuth as ReturnType<typeof vi.fn>;

describe('ProtectedAction', () => {
  it('hides content when unauthenticated', () => {
    mockUseAuth.mockReturnValue({
      state: { status: 'unauthenticated' },
    } as any);

    render(
      <ProtectedAction requiredPermission="state:write">
        <button>Delete</button>
      </ProtectedAction>
    );

    expect(screen.queryByText('Delete')).not.toBeInTheDocument();
  });

  it('hides content when user lacks permission', () => {
    mockUseAuth.mockReturnValue({
      state: {
        status: 'authenticated',
        session: {
          user: { id: '1', username: 'test', roles: ['viewer'] },
          expiresAt: Date.now() + 3600000,
        },
      },
    } as any);

    render(
      <ProtectedAction requiredPermission="state:delete">
        <button>Delete</button>
      </ProtectedAction>
    );

    expect(screen.queryByText('Delete')).not.toBeInTheDocument();
  });

  it('shows content when user has permission', () => {
    mockUseAuth.mockReturnValue({
      state: {
        status: 'authenticated',
        session: {
          user: { id: '1', username: 'test', roles: ['state:delete'] },
          expiresAt: Date.now() + 3600000,
        },
      },
    } as any);

    render(
      <ProtectedAction requiredPermission="state:delete">
        <button>Delete</button>
      </ProtectedAction>
    );

    expect(screen.getByText('Delete')).toBeInTheDocument();
  });

  it('shows fallback when user lacks permission', () => {
    mockUseAuth.mockReturnValue({
      state: {
        status: 'authenticated',
        session: {
          user: { id: '1', username: 'test', roles: ['viewer'] },
          expiresAt: Date.now() + 3600000,
        },
      },
    } as any);

    render(
      <ProtectedAction
        requiredPermission="state:delete"
        fallback={<span>Not authorized</span>}
      >
        <button>Delete</button>
      </ProtectedAction>
    );

    expect(screen.getByText('Not authorized')).toBeInTheDocument();
    expect(screen.queryByText('Delete')).not.toBeInTheDocument();
  });
});
```

### Security Reminder

**⚠️ IMPORTANT**: UI-level permission checks are for UX only, NOT security enforcement.

- Server MUST enforce authorization on all operations
- Hiding UI elements does NOT prevent API access
- Attackers can bypass client-side checks via browser dev tools
- Backend authorization is the ONLY security boundary

---

## 5.6. Context Integration and Provider Ordering

### Decision: AuthProvider Wraps GridProvider

The authentication context must wrap the Grid API context to ensure auth state is available when initializing the Connect transport.

### Provider Hierarchy

```typescript
// webapp/src/App.tsx
import { AuthProvider } from './context/AuthContext';
import { GridProvider } from './context/GridContext';
import { Dashboard } from './components/Dashboard';

export function App() {
  return (
    <AuthProvider>       {/* Outer: Provides auth state and logout callback */}
      <GridProvider>     {/* Inner: Needs auth context for interceptor */}
        <Dashboard />
      </GridProvider>
    </AuthProvider>
  );
}
```

### Rationale

**Why AuthProvider is outer**:
1. `GridProvider` creates Connect transport with auth interceptor
2. Auth interceptor needs `logout()` callback from `useAuth()`
3. `useAuth()` hook requires `AuthProvider` to be mounted first
4. React Context resolution happens bottom-up in component tree

**Dependency Chain**:
```
Dashboard
  └── needs GridContext (API client)
       └── GridProvider creates Transport
            └── Transport needs auth interceptor
                 └── Interceptor calls onAuthError callback
                      └── Callback uses logout() from AuthContext
                           └── AuthProvider MUST be mounted
```

### GridProvider Implementation with Auth Integration

```typescript
// webapp/src/context/GridContext.tsx
import { createContext, useContext, useMemo, ReactNode } from 'react';
import { createConnectTransport } from '@connectrpc/connect-web';
import { createAuthInterceptor } from '../services/authInterceptor';
import { useAuth } from './AuthContext';
import { GridApiAdapter } from '../services/gridApi';

interface GridContextValue {
  api: GridApiAdapter;
}

const GridContext = createContext<GridContextValue | null>(null);

export function GridProvider({ children }: { children: ReactNode }) {
  const { logout } = useAuth();  // ✅ Auth context available here

  const transport = useMemo(
    () =>
      createConnectTransport({
        baseUrl: import.meta.env.VITE_GRID_API_URL || 'http://localhost:8080',
        credentials: 'include',
        interceptors: [
          createAuthInterceptor(() => {
            // Handle auth errors by triggering logout
            logout();
          }),
        ],
      }),
    [logout]
  );

  const api = useMemo(() => new GridApiAdapter(transport), [transport]);

  return <GridContext.Provider value={{ api }}>{children}</GridContext.Provider>;
}

export function useGrid() {
  const context = useContext(GridContext);
  if (!context) {
    throw new Error('useGrid must be used within GridProvider');
  }
  return context;
}
```

### Incorrect Ordering (Anti-Pattern)

```typescript
// ❌ BAD: GridProvider outer, AuthProvider inner
export function App() {
  return (
    <GridProvider>      {/* Transport created first */}
      <AuthProvider>    {/* Auth context not available during transport creation */}
        <Dashboard />
      </AuthProvider>
    </GridProvider>
  );
}
```

**Problem**: When `GridProvider` renders, `useAuth()` is called but `AuthProvider` is not yet mounted, causing "useAuth must be used within AuthProvider" error.

### Alternative Pattern: Props-Based Injection

If provider ordering becomes complex, consider injecting logout callback via props:

```typescript
// Alternative (more explicit)
export function App() {
  return (
    <AuthProvider>
      {(authProps) => (
        <GridProvider onAuthError={authProps.logout}>
          <Dashboard />
        </GridProvider>
      )}
    </AuthProvider>
  );
}
```

**Not recommended for Grid**: Adds indirection without benefit; standard provider nesting is clearer.

### Testing Context Integration

```typescript
// test-utils.tsx
import { render } from '@testing-library/react';
import { AuthProvider } from '../context/AuthContext';
import { GridProvider } from '../context/GridContext';

/**
 * Test utility that wraps components with required providers
 */
export function renderWithProviders(ui: React.ReactElement) {
  return render(
    <AuthProvider>
      <GridProvider>
        {ui}
      </GridProvider>
    </AuthProvider>
  );
}

// Usage in tests
import { renderWithProviders } from './test-utils';

test('dashboard loads states', async () => {
  const { findByText } = renderWithProviders(<Dashboard />);
  expect(await findByText('States')).toBeInTheDocument();
});
```

---

## 6. Implementation Notes

### Connect RPC Interceptor Patterns

#### Basic Authentication Interceptor

```typescript
import { Interceptor, ConnectError, Code } from '@connectrpc/connect';

export function createAuthInterceptor(
  onAuthError: () => void
): Interceptor {
  return (next) => async (req) => {
    try {
      // Execute request (cookies sent automatically with credentials: 'include')
      const response = await next(req);
      return response;
    } catch (error) {
      // Handle authentication errors
      if (error instanceof ConnectError) {
        if (error.code === Code.Unauthenticated) {
          console.log('Authentication error detected, triggering logout');
          onAuthError();
        } else if (error.code === Code.PermissionDenied) {
          console.log('Authorization error detected');
          // Could show toast notification
        }
      }
      
      // Re-throw error for component to handle
      throw error;
    }
  };
}
```

#### Logging Interceptor (Development)

```typescript
export function createLoggingInterceptor(): Interceptor {
  return (next) => async (req) => {
    console.log(`[Connect RPC] ${req.method} ${req.url}`);
    const start = Date.now();
    
    try {
      const response = await next(req);
      const duration = Date.now() - start;
      console.log(`[Connect RPC] ${req.method} completed in ${duration}ms`);
      return response;
    } catch (error) {
      const duration = Date.now() - start;
      console.error(`[Connect RPC] ${req.method} failed in ${duration}ms`, error);
      throw error;
    }
  };
}
```

#### Combined Transport Configuration

```typescript
import { createConnectTransport } from '@connectrpc/connect-web';
import { Transport } from '@connectrpc/connect';

export function createGridTransport(
  baseUrl: string,
  onAuthError: () => void,
  isDevelopment: boolean = false
): Transport {
  const interceptors: Interceptor[] = [
    createAuthInterceptor(onAuthError),
  ];

  if (isDevelopment) {
    interceptors.push(createLoggingInterceptor());
  }

  return createConnectTransport({
    baseUrl,
    credentials: 'include',  // Send httpOnly cookies
    interceptors,
  });
}
```

### React Hooks Structure

#### useAuth Hook

```typescript
// hooks/useAuth.ts
export function useAuth() {
  const context = useContext(AuthContext);
  if (!context) {
    throw new Error('useAuth must be used within AuthProvider');
  }
  return context;
}

// Usage
function Dashboard() {
  const { state, logout } = useAuth();

  if (state.status === 'unauthenticated') {
    return <Navigate to="/login" />;
  }

  if (state.status === 'authenticated') {
    return (
      <div>
        <h1>Welcome, {state.session.user.username}</h1>
        <button onClick={logout}>Logout</button>
      </div>
    );
  }

  return null;
}
```

#### useAuthGuard Hook

```typescript
// hooks/useAuthGuard.ts
export function useAuthGuard() {
  const { state } = useAuth();
  const navigate = useNavigate();

  useEffect(() => {
    if (state.status === 'unauthenticated') {
      navigate('/login');
    }
  }, [state.status, navigate]);

  return state.status === 'authenticated';
}

// Usage
function ProtectedPage() {
  const isAuthenticated = useAuthGuard();

  if (!isAuthenticated) {
    return <LoadingSpinner />;
  }

  return <div>Protected content</div>;
}
```

#### usePermissions Hook (Future)

```typescript
// hooks/usePermissions.ts
export function usePermissions() {
  const { state } = useAuth();

  if (state.status !== 'authenticated') {
    return { hasPermission: () => false, roles: [] };
  }

  const hasPermission = useCallback(
    (requiredRole: string) => {
      return state.session.user.roles.includes(requiredRole);
    },
    [state.session.user.roles]
  );

  const canAccessState = useCallback(
    (state: StateInfo) => {
      // Check if state labels match user's role scope
      // This is redundant if backend filters correctly (grid-f5947b22)
      return true;  // Backend already filtered
    },
    [state.session.user.roles]
  );

  return {
    hasPermission,
    canAccessState,
    roles: state.session.user.roles,
  };
}

// Usage
function StateActions({ state }: { state: StateInfo }) {
  const { hasPermission } = usePermissions();

  return (
    <div>
      {hasPermission('state:write') && (
        <button onClick={() => updateState(state)}>Edit</button>
      )}
      {hasPermission('state:delete') && (
        <button onClick={() => deleteState(state)}>Delete</button>
      )}
    </div>
  );
}
```

### Session Restoration Flow

```
┌─────────────────────────────────────────────────────────────┐
│                     App Initialization                       │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
                  ┌───────────────────────┐
                  │  Check /auth/config   │
                  │  (auth required?)     │
                  └───────────┬───────────┘
                              │
                 ┌────────────┴────────────┐
                 │                         │
                 ▼                         ▼
        ┌─────────────────┐      ┌──────────────────┐
        │ Auth NOT        │      │ Auth REQUIRED    │
        │ required        │      │                  │
        └─────────────────┘      └──────────────────┘
                 │                         │
                 ▼                         ▼
        ┌─────────────────┐      ┌──────────────────┐
        │ Show Dashboard  │      │ Check session    │
        │ immediately     │      │ (call protected  │
        │                 │      │  endpoint)       │
        └─────────────────┘      └──────────────────┘
                                          │
                            ┌─────────────┴──────────────┐
                            │                            │
                            ▼                            ▼
                   ┌─────────────────┐        ┌──────────────────┐
                   │ Session Valid   │        │ Session Invalid  │
                   │ (200 OK)        │        │ (401)            │
                   └─────────────────┘        └──────────────────┘
                            │                            │
                            ▼                            ▼
                   ┌─────────────────┐        ┌──────────────────┐
                   │ Extract user    │        │ Show Login Page  │
                   │ info, populate  │        │                  │
                   │ AuthContext     │        └──────────────────┘
                   └─────────────────┘
                            │
                            ▼
                   ┌─────────────────┐
                   │ Show Dashboard  │
                   │ (authenticated) │
                   └─────────────────┘
```

### Error Handling Best Practices

#### Display User-Friendly Messages

```typescript
function ErrorBoundary({ error }: { error: unknown }) {
  if (error instanceof ConnectError) {
    switch (error.code) {
      case Code.Unauthenticated:
        return <LoginPrompt />;
      
      case Code.PermissionDenied:
        return (
          <Alert severity="error">
            You don't have permission to perform this action.
          </Alert>
        );
      
      case Code.NotFound:
        return (
          <Alert severity="warning">
            The requested resource was not found.
          </Alert>
        );
      
      case Code.InvalidArgument:
        return (
          <Alert severity="error">
            Invalid input. Please check your data and try again.
          </Alert>
        );
      
      default:
        return (
          <Alert severity="error">
            An unexpected error occurred. Please try again later.
          </Alert>
        );
    }
  }

  return (
    <Alert severity="error">
      An unexpected error occurred. Please try again later.
    </Alert>
  );
}
```

#### Don't Leak Sensitive Information

```typescript
// ❌ BAD: Exposes internal details
function ErrorDisplay({ error }: { error: Error }) {
  return <div>Error: {error.message}</div>;
}

// ✅ GOOD: Generic message for users, log details
function ErrorDisplay({ error }: { error: Error }) {
  console.error('Error details:', error);
  
  return (
    <div>
      Something went wrong. Please try again.
      {process.env.NODE_ENV === 'development' && (
        <pre>{error.message}</pre>
      )}
    </div>
  );
}
```

---

## 6.5. Testing Strategy

### Unit Testing Approach

**Goal**: Test authentication logic independently of React components and HTTP calls.

#### Testing AuthContext with Mocked Services

```typescript
// context/__tests__/AuthContext.test.tsx
import { describe, it, expect, beforeEach, vi } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import { act } from 'react-dom/test-utils';
import { AuthProvider, useAuth } from '../AuthContext';
import * as authApi from '../../services/authApi';

// Mock the entire auth service module
vi.mock('../../services/authApi');

const mockedAuthApi = authApi as ReturnType<typeof vi.mocked<typeof authApi>>;

// Test component that exposes auth state
function TestConsumer() {
  const { state, login, logout } = useAuth();

  return (
    <div>
      <div data-testid="status">{state.status}</div>
      {state.status === 'authenticated' && (
        <div data-testid="username">{state.session.user.username}</div>
      )}
      <button onClick={() => login({ username: 'test', password: 'test' })}>
        Login
      </button>
      <button onClick={logout}>Logout</button>
    </div>
  );
}

describe('AuthContext', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('initializes with loading state', () => {
    mockedAuthApi.getAuthConfig.mockResolvedValue({ mode: 'internal-idp' });
    mockedAuthApi.getSessionStatus.mockResolvedValue(null);

    render(
      <AuthProvider>
        <TestConsumer />
      </AuthProvider>
    );

    expect(screen.getByTestId('status')).toHaveTextContent('initializing');
  });

  it('restores session on mount if cookie is valid', async () => {
    const mockSession = {
      user: {
        id: '1',
        username: 'testuser',
        email: 'test@example.com',
        authType: 'internal' as const,
        roles: ['admin'],
        groups: [],
      },
      expiresAt: Date.now() + 3600000,
    };

    mockedAuthApi.getAuthConfig.mockResolvedValue({ mode: 'internal-idp' });
    mockedAuthApi.getSessionStatus.mockResolvedValue(mockSession);

    render(
      <AuthProvider>
        <TestConsumer />
      </AuthProvider>
    );

    await waitFor(() => {
      expect(screen.getByTestId('status')).toHaveTextContent('authenticated');
    });

    expect(screen.getByTestId('username')).toHaveTextContent('testuser');
  });

  it('transitions to unauthenticated when session check fails', async () => {
    mockedAuthApi.getAuthConfig.mockResolvedValue({ mode: 'internal-idp' });
    mockedAuthApi.getSessionStatus.mockResolvedValue(null);

    render(
      <AuthProvider>
        <TestConsumer />
      </AuthProvider>
    );

    await waitFor(() => {
      expect(screen.getByTestId('status')).toHaveTextContent('unauthenticated');
    });
  });

  it('handles login success', async () => {
    const mockSession = {
      user: {
        id: '1',
        username: 'newuser',
        email: 'new@example.com',
        authType: 'internal' as const,
        roles: ['editor'],
        groups: [],
      },
      expiresAt: Date.now() + 3600000,
    };

    mockedAuthApi.getAuthConfig.mockResolvedValue({ mode: 'internal-idp' });
    mockedAuthApi.getSessionStatus.mockResolvedValue(null);
    mockedAuthApi.loginInternal.mockResolvedValue(mockSession);

    const { getByText } = render(
      <AuthProvider>
        <TestConsumer />
      </AuthProvider>
    );

    await waitFor(() => {
      expect(screen.getByTestId('status')).toHaveTextContent('unauthenticated');
    });

    act(() => {
      getByText('Login').click();
    });

    await waitFor(() => {
      expect(screen.getByTestId('status')).toHaveTextContent('authenticated');
      expect(screen.getByTestId('username')).toHaveTextContent('newuser');
    });
  });

  it('handles logout', async () => {
    const mockSession = {
      user: {
        id: '1',
        username: 'testuser',
        email: 'test@example.com',
        authType: 'internal' as const,
        roles: ['admin'],
      },
      expiresAt: Date.now() + 3600000,
    };

    mockedAuthApi.getAuthConfig.mockResolvedValue({ mode: 'internal-idp' });
    mockedAuthApi.getSessionStatus.mockResolvedValue(mockSession);
    mockedAuthApi.logout.mockResolvedValue();

    const { getByText } = render(
      <AuthProvider>
        <TestConsumer />
      </AuthProvider>
    );

    await waitFor(() => {
      expect(screen.getByTestId('status')).toHaveTextContent('authenticated');
    });

    act(() => {
      getByText('Logout').click();
    });

    await waitFor(() => {
      expect(screen.getByTestId('status')).toHaveTextContent('unauthenticated');
    });
  });
});
```

#### Testing Auth Service Layer

```typescript
// services/__tests__/authApi.test.ts
import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import {
  getAuthConfig,
  getSessionStatus,
  loginInternal,
  logout,
} from '../authApi';

describe('authApi', () => {
  beforeEach(() => {
    global.fetch = vi.fn();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  describe('getAuthConfig', () => {
    it('returns auth config when available', async () => {
      const mockConfig = { mode: 'internal-idp', issuer: 'http://localhost:8080' };

      (global.fetch as ReturnType<typeof vi.fn>).mockResolvedValue({
        ok: true,
        json: async () => mockConfig,
      });

      const result = await getAuthConfig();

      expect(result).toEqual(mockConfig);
      expect(global.fetch).toHaveBeenCalledWith('/auth/config', {
        method: 'GET',
        credentials: 'include',
      });
    });

    it('returns null when auth is disabled', async () => {
      (global.fetch as ReturnType<typeof vi.fn>).mockResolvedValue({
        ok: false,
        status: 404,
      });

      const result = await getAuthConfig();

      expect(result).toBeNull();
    });
  });

  describe('getSessionStatus', () => {
    it('returns session data when authenticated', async () => {
      const mockResponse = {
        user: {
          id: '1',
          username: 'test',
          email: 'test@example.com',
          auth_type: 'internal',
          roles: ['admin'],
          groups: ['devops'],
        },
        session: {
          expires_at: 1234567890,
        },
      };

      (global.fetch as ReturnType<typeof vi.fn>).mockResolvedValue({
        ok: true,
        json: async () => mockResponse,
      });

      const result = await getSessionStatus();

      expect(result).toEqual({
        user: {
          id: '1',
          username: 'test',
          email: 'test@example.com',
          authType: 'internal',
          roles: ['admin'],
          groups: ['devops'],
        },
        expiresAt: 1234567890,
      });
    });

    it('returns null when unauthenticated', async () => {
      (global.fetch as ReturnType<typeof vi.fn>).mockResolvedValue({
        ok: false,
        status: 401,
      });

      const result = await getSessionStatus();

      expect(result).toBeNull();
    });
  });

  describe('loginInternal', () => {
    it('returns session on successful login', async () => {
      const mockResponse = {
        user: {
          id: '2',
          username: 'newuser',
          email: 'new@example.com',
          authType: 'internal',
          roles: ['editor'],
        },
        expiresAt: 9876543210,
      };

      (global.fetch as ReturnType<typeof vi.fn>).mockResolvedValue({
        ok: true,
        json: async () => mockResponse,
      });

      const result = await loginInternal('newuser', 'password123');

      expect(result).toEqual(mockResponse);
      expect(global.fetch).toHaveBeenCalledWith('/auth/login', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        credentials: 'include',
        body: JSON.stringify({ username: 'newuser', password: 'password123' }),
      });
    });

    it('throws error on failed login', async () => {
      (global.fetch as ReturnType<typeof vi.fn>).mockResolvedValue({
        ok: false,
        status: 401,
        text: async () => 'Invalid credentials',
      });

      await expect(loginInternal('baduser', 'badpass')).rejects.toThrow(
        'Invalid credentials'
      );
    });
  });

  describe('logout', () => {
    it('calls logout endpoint', async () => {
      (global.fetch as ReturnType<typeof vi.fn>).mockResolvedValue({
        ok: true,
      });

      await logout();

      expect(global.fetch).toHaveBeenCalledWith('/auth/logout', {
        method: 'POST',
        credentials: 'include',
      });
    });
  });
});
```

### Integration Testing

#### Testing with Real Backend (Optional)

```typescript
// tests/integration/webapp-auth.test.ts
import { test, expect } from '@playwright/test';

test.describe('WebApp Authentication Flow', () => {
  test('internal IdP login flow', async ({ page }) => {
    // Navigate to app
    await page.goto('http://localhost:5173');

    // Should redirect to login page
    await expect(page).toHaveURL(/.*\/login/);

    // Fill in credentials
    await page.fill('input[name="username"]', 'admin@internal');
    await page.fill('input[name="password"]', 'admin-secure-pass-123');

    // Submit login
    await page.click('button[type="submit"]');

    // Should redirect to dashboard
    await expect(page).toHaveURL('http://localhost:5173/');

    // Should see user info
    await expect(page.locator('[data-testid="auth-status"]')).toContainText('admin@internal');
  });

  test('session restoration on page reload', async ({ page, context }) => {
    // Login first
    await page.goto('http://localhost:5173/login');
    await page.fill('input[name="username"]', 'admin@internal');
    await page.fill('input[name="password"]', 'admin-secure-pass-123');
    await page.click('button[type="submit"]');

    // Wait for dashboard
    await expect(page).toHaveURL('http://localhost:5173/');

    // Reload page
    await page.reload();

    // Should still be authenticated (session restored from cookie)
    await expect(page.locator('[data-testid="auth-status"]')).toContainText('admin@internal');
  });

  test('logout clears session', async ({ page }) => {
    // Login
    await page.goto('http://localhost:5173/login');
    await page.fill('input[name="username"]', 'admin@internal');
    await page.fill('input[name="password"]', 'admin-secure-pass-123');
    await page.click('button[type="submit"]');

    // Click logout
    await page.click('[data-testid="logout-button"]');

    // Should redirect to login
    await expect(page).toHaveURL(/.*\/login/);

    // Session should be cleared (reload shows login page)
    await page.reload();
    await expect(page).toHaveURL(/.*\/login/);
  });
});
```

### Test Coverage Goals

- **AuthContext**: 100% (critical path)
- **Auth Service Layer**: 100% (pure functions, easy to test)
- **Route Guards**: 90%+ (test all state transitions)
- **Components (LoginPage, AuthStatus)**: 80%+ (integration with AuthContext)
- **E2E Flows**: Critical paths only (login, logout, session restoration)

---

## 7. Security Considerations

### XSS Prevention

#### HttpOnly Cookies
- ✅ Grid uses httpOnly cookies for session tokens
- ✅ JavaScript cannot access token via `document.cookie`
- ✅ Reduces attack surface for XSS token theft

#### Content Security Policy
**Recommendation**: Configure CSP headers in Grid's backend

```go
func SecurityHeadersMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Security-Policy", 
            "default-src 'self'; "+
            "script-src 'self' 'unsafe-inline'; "+  // React needs inline scripts
            "style-src 'self' 'unsafe-inline'; "+
            "img-src 'self' data: https:; "+
            "connect-src 'self'")
        
        w.Header().Set("X-Content-Type-Options", "nosniff")
        w.Header().Set("X-Frame-Options", "DENY")
        w.Header().Set("X-XSS-Protection", "1; mode=block")
        
        next.ServeHTTP(w, r)
    })
}
```

#### React XSS Protection
- ✅ React escapes JSX by default
- ❌ Avoid `dangerouslySetInnerHTML`
- ✅ Validate user input before rendering
- ✅ Use TypeScript to catch type errors

### CSRF Protection

#### SameSite Cookies
Grid uses `SameSite=Lax`:
- ✅ Prevents CSRF attacks from external sites
- ✅ Allows cookies on top-level navigation (OAuth redirect)
- ✅ More secure than `SameSite=None`

**Trade-off**: `SameSite=Lax` doesn't protect POST requests from same-site origins. For additional protection, Grid can:

1. **State Parameter Validation** (OAuth CSRF) - ✅ Already implemented
2. **Origin Header Validation** - ✅ Chi CORS middleware
3. **CSRF Tokens** (optional) - Not needed with httpOnly + SameSite + Origin checks

#### CORS Configuration
Grid's backend must:
```go
cors := cors.New(cors.Options{
    AllowedOrigins:   []string{"https://grid.example.com"},  // Specific origin
    AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE"},
    AllowedHeaders:   []string{"Content-Type", "Authorization"},
    AllowCredentials: true,  // Required for cookies
    MaxAge:           300,
})
```

**Critical**: Cannot use `AllowedOrigins: []string{"*"}` with `AllowCredentials: true`

### Auth Token Handling

#### Storage
- ✅ Tokens stored in httpOnly cookies (not localStorage)
- ✅ Cookies have `Secure` flag (HTTPS only)
- ✅ Cookies have `SameSite=Lax` (CSRF protection)
- ✅ Cookies have expiry matching token TTL

#### Transmission
- ✅ Cookies sent automatically by browser (no manual header management)
- ✅ HTTPS required in production (TLS encryption)
- ✅ Tokens never logged to console (sanitize logs)

#### Validation
- ✅ Backend validates JWT signature
- ✅ Backend validates JWT expiry
- ✅ Backend validates JWT issuer
- ✅ Backend validates JWT audience
- ✅ Backend checks JTI revocation

### CSRF Protection Details

#### What Grid Implements

1. **SameSite=Lax Cookies**
   - Blocks cross-site requests by default
   - Allows top-level navigation (OAuth redirects)
   - Sufficient for most CSRF scenarios

2. **State Parameter (OAuth CSRF)**
   - Cryptographically random state value
   - Stored in httpOnly cookie
   - Verified on callback
   - Prevents authorization code injection

3. **Origin Header Validation**
   - Chi CORS middleware checks Origin header
   - Rejects requests from unauthorized origins
   - Prevents cross-origin API calls

#### What Grid Does NOT Need

1. **CSRF Tokens for API Calls**
   - Not needed: Connect RPC uses POST for all operations (protected by SameSite)
   - Not needed: Origin header validated by CORS
   - Not needed: httpOnly cookies cannot be read by attacker scripts

2. **Double-Submit Cookie Pattern**
   - Not needed: SameSite cookies provide equivalent protection
   - Simpler: No need to generate/validate tokens on every request

### Common Vulnerabilities and Mitigations

#### 1. XSS → Token Theft
**Vulnerability**: Attacker injects script to steal token
**Mitigation**: HttpOnly cookies prevent JavaScript access

#### 2. CSRF → Unauthorized Actions
**Vulnerability**: Attacker tricks user into submitting request
**Mitigation**: SameSite=Lax cookies + Origin validation

#### 3. Session Fixation
**Vulnerability**: Attacker sets victim's session ID
**Mitigation**: Grid generates new session on login (not reused)

#### 4. Clickjacking
**Vulnerability**: Attacker embeds Grid in invisible iframe
**Mitigation**: X-Frame-Options: DENY header

#### 5. Authorization Bypass
**Vulnerability**: Client-side filtering bypassed
**Mitigation**: Backend enforces authorization on all operations

#### 6. Token Leakage in Logs
**Vulnerability**: Tokens logged to console/analytics
**Mitigation**: Sanitize logs, never log cookie values

#### 7. Open Redirect
**Vulnerability**: OAuth redirect_uri manipulated
**Mitigation**: Exact redirect URI matching (no wildcards)

---

## 8. Summary of Decisions

### Architecture Decisions

| Area | Decision | Rationale |
|------|----------|-----------|
| Auth State Management | React Context + useReducer | Simple, built-in, sufficient for auth needs |
| Token Storage | HttpOnly Cookies | XSS protection, automatic cookie sending |
| Connect RPC Auth | Transport-level interceptor | Global auth handling, automatic 401 handling |
| OAuth2 Flow | Server-driven Auth Code + PKCE | Backend handles all OAuth logic, secure |
| Session Restoration | Call protected endpoint on load | Verify session validity, populate user info |
| Role-Based Filtering | Server-side (future) + Client UI | Backend authority, client convenience |
| CSRF Protection | SameSite=Lax + State validation | Standard approach, secure |

### Implementation Priorities

#### Phase 1: Core Authentication (P0)
1. Implement AuthContext with Context API
2. Create login flow for both auth modes
3. Implement session restoration on page load
4. Add auth interceptor to Connect transport
5. Handle 401 responses (logout)

#### Phase 2: User Experience (P1)
1. Add AuthStatus dropdown component
2. Implement logout functionality
3. Add loading states during authentication
4. Display user roles and groups
5. Show session expiry information

#### Phase 3: Authorization (P1)
1. Implement client-side role checks (UI only)
2. Filter dashboard based on user roles
3. Hide unauthorized actions in UI
4. Display empty state when no access

#### Phase 4: Polish (P2)
1. Add error boundaries for auth errors
2. Implement auth mode detection
3. Add session expiry warnings
4. Improve loading transitions
5. Add development mode logging

---

## References

### Grid Documentation
- [Spec 006: Authentication, Authorization, and RBAC](/Users/vincentdesmet/tcons/grid/specs/006-authz-authn-rbac/spec.md)
- [Spec 007: WebApp Auth](/Users/vincentdesmet/tcons/grid/specs/007-webapp-auth/spec.md)
- [Auth Handlers Implementation](/Users/vincentdesmet/tcons/grid/cmd/gridapi/internal/server/auth_handlers.go)
- [Authentication Middleware](/Users/vincentdesmet/tcons/grid/cmd/gridapi/internal/middleware/authn.go)

### External Documentation
- [Connect RPC Interceptors](https://connectrpc.com/docs/web/interceptors/)
- [Connect RPC Headers](https://connectrpc.com/docs/web/headers-and-trailers/)
- [OAuth 2.0 for Browser-Based Apps (IETF Draft)](https://datatracker.ietf.org/doc/html/draft-ietf-oauth-browser-based-apps)
- [Authorization Code Flow with PKCE (Auth0)](https://auth0.com/docs/get-started/authentication-and-authorization-flow/authorization-code-flow-with-pkce)
- [React Context API Best Practices](https://react.dev/learn/passing-data-deeply-with-context)

### Security Resources
- [OWASP Top 10](https://owasp.org/www-project-top-ten/)
- [OWASP XSS Prevention Cheat Sheet](https://cheatsheetseries.owasp.org/cheatsheets/Cross_Site_Scripting_Prevention_Cheat_Sheet.html)
- [OWASP CSRF Prevention Cheat Sheet](https://cheatsheetseries.owasp.org/cheatsheets/Cross-Site_Request_Forgery_Prevention_Cheat_Sheet.html)

---

## Appendix: Code Examples

### Complete AuthContext Implementation

```typescript
// contexts/AuthContext.tsx
import {
  createContext,
  useContext,
  useReducer,
  useEffect,
  useMemo,
  useCallback,
  ReactNode,
} from 'react';

// Types
export interface User {
  id: string;
  username: string;
  email: string;
  authType: 'oidc' | 'basic';
  roles: string[];
  groups?: string[];
}

export interface Session {
  user: User;
  token: string;  // Empty for httpOnly cookie auth
  expiresAt: number;
}

export type AuthState =
  | { status: 'initializing' }
  | { status: 'unauthenticated' }
  | { status: 'authenticated'; session: Session };

type AuthAction =
  | { type: 'INITIALIZE'; session: Session | null }
  | { type: 'LOGIN_SUCCESS'; session: Session }
  | { type: 'LOGOUT' }
  | { type: 'SESSION_EXPIRED' };

interface AuthContextValue {
  state: AuthState;
  login: (mode: 'internal' | 'external') => void;
  logout: () => Promise<void>;
  refreshSession: () => Promise<void>;
}

// Context
const AuthContext = createContext<AuthContextValue | null>(null);

// Reducer
function authReducer(state: AuthState, action: AuthAction): AuthState {
  switch (action.type) {
    case 'INITIALIZE':
      return action.session
        ? { status: 'authenticated', session: action.session }
        : { status: 'unauthenticated' };
    
    case 'LOGIN_SUCCESS':
      return { status: 'authenticated', session: action.session };
    
    case 'LOGOUT':
    case 'SESSION_EXPIRED':
      return { status: 'unauthenticated' };
    
    default:
      return state;
  }
}

// Provider
export function AuthProvider({ children }: { children: ReactNode }) {
  const [state, dispatch] = useReducer(authReducer, { status: 'initializing' });

  // Initialize session on mount
  useEffect(() => {
    const initSession = async () => {
      try {
        const response = await fetch('/api/auth/whoami', {
          credentials: 'include',
        });

        if (response.ok) {
          const data = await response.json();
          const session: Session = {
            user: data.user,
            token: '',
            expiresAt: data.expires_at,
          };
          dispatch({ type: 'INITIALIZE', session });
        } else {
          dispatch({ type: 'INITIALIZE', session: null });
        }
      } catch {
        dispatch({ type: 'INITIALIZE', session: null });
      }
    };

    initSession();
  }, []);

  // Login handler
  const login = useCallback((mode: 'internal' | 'external') => {
    if (mode === 'internal') {
      // Navigate to internal IdP login
      window.location.href = '/auth/login';
    } else {
      // Navigate to external IdP login (OAuth)
      window.location.href = '/auth/login';
    }
  }, []);

  // Logout handler
  const logout = useCallback(async () => {
    try {
      await fetch('/auth/logout', {
        method: 'POST',
        credentials: 'include',
      });
    } finally {
      dispatch({ type: 'LOGOUT' });
    }
  }, []);

  // Refresh session handler
  const refreshSession = useCallback(async () => {
    try {
      const response = await fetch('/api/auth/whoami', {
        credentials: 'include',
      });

      if (response.ok) {
        const data = await response.json();
        const session: Session = {
          user: data.user,
          token: '',
          expiresAt: data.expires_at,
        };
        dispatch({ type: 'INITIALIZE', session });
      } else {
        dispatch({ type: 'SESSION_EXPIRED' });
      }
    } catch {
      dispatch({ type: 'SESSION_EXPIRED' });
    }
  }, []);

  // Memoize context value
  const value = useMemo(
    () => ({ state, login, logout, refreshSession }),
    [state, login, logout, refreshSession]
  );

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

// Hook
export function useAuth() {
  const context = useContext(AuthContext);
  if (!context) {
    throw new Error('useAuth must be used within AuthProvider');
  }
  return context;
}
```

### Complete Transport Configuration

```typescript
// services/gridTransport.ts
import { createConnectTransport } from '@connectrpc/connect-web';
import { Interceptor, ConnectError, Code } from '@connectrpc/connect';
import type { Transport } from '@connectrpc/connect';

function createAuthInterceptor(onAuthError: () => void): Interceptor {
  return (next) => async (req) => {
    try {
      return await next(req);
    } catch (error) {
      if (error instanceof ConnectError && error.code === Code.Unauthenticated) {
        console.log('Authentication error, triggering logout');
        onAuthError();
      }
      throw error;
    }
  };
}

function createLoggingInterceptor(): Interceptor {
  return (next) => async (req) => {
    console.log(`[RPC] ${req.method}`);
    const start = Date.now();
    
    try {
      const response = await next(req);
      console.log(`[RPC] ${req.method} completed in ${Date.now() - start}ms`);
      return response;
    } catch (error) {
      console.error(`[RPC] ${req.method} failed in ${Date.now() - start}ms`, error);
      throw error;
    }
  };
}

export function createGridTransport(
  baseUrl: string,
  onAuthError: () => void,
  options: { enableLogging?: boolean } = {}
): Transport {
  const interceptors: Interceptor[] = [
    createAuthInterceptor(onAuthError),
  ];

  if (options.enableLogging) {
    interceptors.push(createLoggingInterceptor());
  }

  return createConnectTransport({
    baseUrl,
    credentials: 'include',
    interceptors,
  });
}
```

---

**End of Research Document**
