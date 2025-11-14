# Quickstart: WebApp Authentication

**Feature**: 007-webapp-auth
**Date**: 2025-11-04
**Audience**: Developers implementing the webapp authentication feature

## Prerequisites

- Grid repository cloned locally
- Go 1.24+ installed
- Node.js 20+ installed
- pnpm installed
- PostgreSQL running (for gridapi)
- Keycloak or other OIDC provider (optional, for external IdP testing)

## Architecture Overview

```
┌─────────────────────────────────────────────────────────────────┐
│                           Browser                               │
│  ┌────────────────────────────────────────────────────────────┐ │
│  │                        Webapp                               │ │
│  │  ┌──────────────┐  ┌──────────────┐  ┌─────────────────┐  │ │
│  │  │ LoginPage    │  │ AuthStatus   │  │ Dashboard       │  │ │
│  │  │ Component    │  │ Component    │  │ (filtered)      │  │ │
│  │  └──────┬───────┘  └──────┬───────┘  └────────┬────────┘  │ │
│  │         │                  │                   │           │ │
│  │         └──────────────────┼───────────────────┘           │ │
│  │                            │                               │ │
│  │                    ┌───────▼────────┐                      │ │
│  │                    │  AuthContext   │                      │ │
│  │                    │  (React State) │                      │ │
│  │                    └───────┬────────┘                      │ │
│  │                            │                               │ │
│  │                    ┌───────▼────────┐                      │ │
│  │                    │   js/sdk       │                      │ │
│  │                    │   auth.ts      │                      │ │
│  │                    └───────┬────────┘                      │ │
│  └────────────────────────────┼───────────────────────────────┘ │
│                                │                                 │
│    HTTP (credentials: 'include')                                │
│                                │                                 │
└────────────────────────────────┼─────────────────────────────────┘
                                 │
                                 ▼
┌─────────────────────────────────────────────────────────────────┐
│                          Gridapi Server                         │
│  ┌────────────────────────────────────────────────────────────┐ │
│  │  /health, /auth/config, /auth/login, /auth/callback, etc.  │ │
│  └────────────────┬───────────────────────────────────────────┘ │
│                   │                                              │
│  ┌────────────────▼───────────────┐                             │
│  │  Authentication Middleware     │                             │
│  │  (JWT validation, Casbin)      │                             │
│  └────────────────┬───────────────┘                             │
│                   │                                              │
│  ┌────────────────▼───────────────┐                             │
│  │  Connect RPC Services          │                             │
│  │  (StateService, etc.)          │                             │
│  └────────────────────────────────┘                             │
└─────────────────────────────────────────────────────────────────┘
```

## Development Workflow

### Step 1: Review Existing Mockups

The designer has already created UI mockups. Review them first:

```bash
# Open existing mockup components
code webapp/src/components/LoginPage.tsx
code webapp/src/components/AuthStatus.tsx
code webapp/src/services/authMockService.ts
```

**Key observations**:
- `LoginPage`: Has form for username/password AND SSO mode selector
- `AuthStatus`: Dropdown showing user info, roles, groups, session expiry
- `authMockService`: Mock implementation with demo accounts

**Goal**: Replace mock service with real gridapi integration.

### Step 2: Setup Development Environment

```bash
# Start PostgreSQL (if not already running)
make db-up

# Initialize database with auth tables
./bin/gridapi db migrate

# Start gridapi with internal IdP
export DATABASE_URL="postgres://grid:gridpass@localhost:5432/grid?sslmode=disable"
export OIDC_ISSUER="http://localhost:8080"  # Internal IdP mode
export SERVER_ADDR="localhost:8080"
./bin/gridapi serve

# In another terminal, start webapp dev server
cd webapp
pnpm install
pnpm run dev
# Webapp runs on http://localhost:5173
```

**⚠️ Important - Vite Proxy Configuration**:

The webapp **requires** the Vite proxy configuration in `webapp/vite.config.ts` for development:

```typescript
server: {
  proxy: {
    '/auth': 'http://localhost:8080',
    '/api': 'http://localhost:8080',
    '/state.v1.StateService': 'http://localhost:8080',
    '/tfstate': 'http://localhost:8080',
  }
}
```

**Why**: httpOnly session cookies only work with same-origin requests. The proxy makes localhost:5173 (webapp) and localhost:8080 (API) appear same-origin to the browser.

**DO NOT REMOVE** this configuration. Without it:
- Authentication will fail (401 Unauthorized)
- Session cookies won't be sent with API requests
- SSO login will break

See `webapp/README.md` for deployment architecture details and Beads issue `grid-202d` for SSO callback fix tracking.

### Step 2.5: Bootstrap Internal IdP Users

**⚠️ REQUIRED for Internal IdP Mode**: Create initial admin user for testing.

```bash
# Create bootstrap admin account (Internal IdP only)
# NOTE: --role flag is REQUIRED (at least one role must be assigned)
./bin/gridapi users create \
  --email admin@grid.local \
  --username "Grid Admin" \
  --password "change-me-on-first-login" \
  --role platform-engineer

# Expected output:
# Assigning roles...
# ✓ Assigned role 'platform-engineer'
# User created successfully!
# ----------------------------------------
# User ID: <uuid>
# Email: admin@grid.local
# Username: Grid Admin
# Roles: platform-engineer
# ----------------------------------------
```

**Alternative: Use stdin for password (recommended for production)**:
```bash
# Avoid password in shell history
./bin/gridapi users create \
  --email admin@grid.local \
  --username "Grid Admin" \
  --password-stdin \
  --role platform-engineer

# Then type password when prompted (input hidden)

```bash
# Avoid password in shell history
./bin/gridapi users create \
  --email admin@grid.local \
  --username "Grid Admin" \
  --password-stdin \
  --role platform-engineer

# Then type password when prompted (input hidden)
# Password: [type here]
```

**Create additional test users**:

```bash
# Product engineer role user (all users are read-only in internal IdP mode - they are web users only)
# NOTE: --role flag is REQUIRED
./bin/gridapi users create \
  --email editor@grid.local \
  --username "Test Product Engineer" \
  --password "editor-test-pass-123" \
  --role product-engineer
```

**Note**: This step is only required for internal IdP mode. For external IdP (SSO), users are created automatically via JIT provisioning on first login.

### Step 3: ⚠️ Verify Backend Prerequisites

**CRITICAL**: Webapp authentication requires `/api/auth/whoami` endpoint which **DOES NOT EXIST YET** in gridapi.

```bash
# Check if whoami endpoint exists
curl -i http://localhost:8080/api/auth/whoami
# Expected: 401 Unauthorized (endpoint exists but no session)
# If 404 Not Found, you MUST implement backend changes first!
```

**If endpoint returns 404**, you must implement the backend changes documented in:
- `specs/007-webapp-auth/plan.md` - See "Complexity Tracking" section
- `specs/007-webapp-auth/contracts/README.md` - See `/api/auth/whoami` specification
- `specs/007-webapp-auth/research.md` - See "Backend Endpoint Required" section (lines 1004-1063)

**Required Backend Changes**:
1. Add `HandleWhoAmI()` in `cmd/gridapi/internal/server/auth_handlers.go`
2. Fix bug: Populate `AuthenticatedPrincipal.SessionID` in `cmd/gridapi/internal/middleware/authn.go`
3. Mount endpoint in `cmd/gridapi/internal/server/router.go`
4. Verify session repository methods exist

**Estimated implementation time**: 2-3 hours

---

### Step 4: Verify Other Auth Endpoints

```bash
# Check health endpoint
curl http://localhost:8080/health
# Expected: {"status":"ok","oidc_enabled":true}

# Check auth config
curl http://localhost:8080/auth/config
# Expected: {"mode":"internal-idp","issuer":"http://localhost:8080",...}

# Test internal IdP login (use bootstrap user from Step 2.5)
curl -X POST http://localhost:8080/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin@grid.local","password":"change-me-on-first-login"}' \
  -c cookies.txt
# Expected: 200 OK with session cookie in cookies.txt

# Test whoami with valid session
curl http://localhost:8080/api/auth/whoami -b cookies.txt
# Expected: 200 OK with user info + session info + roles + groups

# Test protected endpoint with cookie
curl http://localhost:8080/state.v1.StateService/ListStates \
  -b cookies.txt \
  -H "Content-Type: application/json" \
  -d '{}'
# Expected: 200 OK with states list

# Test logout
curl -X POST http://localhost:8080/auth/logout -b cookies.txt
# Expected: 200 OK, session cleared
```

### Step 5: Implement js/sdk Auth Helpers

Create `js/sdk/auth.ts` with HTTP functions for auth endpoints:

```bash
cd js/sdk
touch auth.ts
```

**Implementation checklist**:
- [ ] `getAuthConfig()`: Fetch `/auth/config`
- [ ] `loginInternal(username, password)`: POST to `/auth/login` (internal IdP)
- [ ] `initiateExternalLogin(redirectUri?)`: Redirect to `/auth/login` (external IdP)
- [ ] `refreshSession()`: POST to `/auth/refresh`
- [ ] `logout()`: POST to `/auth/logout`
- [ ] All functions use `credentials: 'include'` for cookie handling
- [ ] TypeScript interfaces for request/response types
- [ ] Error handling (throw typed errors on HTTP errors)

**Example**:

```typescript
// js/sdk/auth.ts
export interface AuthConfig {
  mode: 'internal-idp' | 'external-idp';
  issuer?: string;
  // ... other fields
}

export async function getAuthConfig(): Promise<AuthConfig> {
  const response = await fetch('/auth/config', {
    method: 'GET',
    credentials: 'include',
  });

  if (!response.ok) {
    throw new Error(`Failed to fetch auth config: ${response.statusText}`);
  }

  return response.json();
}

// ... other functions
```

**Testing**:

```bash
# Unit tests
pnpm test auth.test.ts

# Manual testing from browser console
# (run webapp dev server, open http://localhost:5173)
import { getAuthConfig } from '../js/sdk/auth';
const config = await getAuthConfig();
console.log(config);
```

### Step 5.5: Implement Auth Service Layer

Create `webapp/src/services/authApi.ts` for testability and separation of concerns:

```bash
cd webapp/src
mkdir -p services
touch services/authApi.ts
```

**Implementation checklist**:
- [ ] `getAuthConfig()`: Fetch `/auth/config`
- [ ] `getSessionStatus()`: Fetch `/api/auth/whoami`
- [ ] `loginInternal(username, password)`: POST to `/auth/login`
- [ ] `initiateExternalLogin(redirectUri?)`: Redirect to `/auth/login`
- [ ] `logout()`: POST to `/auth/logout`
- [ ] All functions use `credentials: 'include'`
- [ ] TypeScript interfaces for User, AuthConfig types
- [ ] Error handling with typed errors

**Example Implementation**:

```typescript
// webapp/src/services/authApi.ts
import type { User, AuthConfig } from '../types/auth';

export async function getAuthConfig(): Promise<AuthConfig | null> {
  try {
    const response = await fetch('/auth/config', {
      method: 'GET',
      credentials: 'include',
    });

    if (!response.ok) {
      return null; // Auth not configured
    }

    return response.json();
  } catch (error) {
    console.error('Failed to fetch auth config:', error);
    return null;
  }
}

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

export function initiateExternalLogin(redirectUri?: string): void {
  const params = redirectUri ? `?redirect_uri=${encodeURIComponent(redirectUri)}` : '';
  window.location.href = `/auth/login${params}`;
}

export async function logout(): Promise<void> {
  await fetch('/auth/logout', {
    method: 'POST',
    credentials: 'include',
  });
}
```

**Testing**:

```bash
# Unit tests (mock fetch)
cd webapp
pnpm test src/services/__tests__/authApi.test.ts

# Integration test with running gridapi
# (see research.md section 6.5 for full test examples)
```

**Benefits**:
- Testable without React (mock fetch, not Context)
- Reusable across components
- Cleaner AuthContext (delegates HTTP to service)

---

### Step 6: Implement AuthContext

Create `webapp/src/context/AuthContext.tsx` that **uses the auth service layer**:

```bash
cd webapp/src
mkdir -p context
touch context/AuthContext.tsx
```

**Implementation checklist**:
- [ ] `AuthState` interface (user, session, config, loading, error)
- [ ] `useReducer` for state management with actions:
  - `AUTH_CONFIG_LOADED`
  - `SESSION_RESTORE_START/SUCCESS/FAILED`
  - `LOGIN_START/SUCCESS/FAILED`
  - `LOGOUT`
  - `SESSION_EXPIRED`
- [ ] `AuthContext` creation with `createContext`
- [ ] `AuthProvider` component wrapping children
- [ ] `useAuth()` hook for consuming context
- [ ] `useEffect` on mount to:
  1. Fetch auth config via `getAuthConfig()`
  2. Attempt session restore via `refreshSession()`
- [ ] Provide functions: `login`, `logout`, `checkSession`

**Example** (using auth service layer):

```typescript
// webapp/src/context/AuthContext.tsx
import { useReducer, useEffect, useCallback, useMemo } from 'react';
import * as authApi from '../services/authApi';

interface AuthState {
  user: User | null;
  config: AuthConfig | null;
  loading: boolean;
  error: string | null;
}

type AuthAction =
  | { type: 'AUTH_CONFIG_LOADED'; payload: AuthConfig }
  | { type: 'SESSION_RESTORE_SUCCESS'; payload: { user: User; expiresAt: number } }
  | { type: 'LOGOUT' }
  // ... other actions

function authReducer(state: AuthState, action: AuthAction): AuthState {
  switch (action.type) {
    case 'AUTH_CONFIG_LOADED':
      return { ...state, config: action.payload };
    case 'SESSION_RESTORE_SUCCESS':
      return { ...state, user: action.payload.user, loading: false };
    case 'LOGOUT':
      return { ...state, user: null };
    // ... other cases
  }
}

export function AuthProvider({ children }: { children: React.ReactNode }) {
  const [state, dispatch] = useReducer(authReducer, {
    user: null,
    config: null,
    loading: true,
    error: null,
  });

  useEffect(() => {
    // Initialize on mount: load config and restore session
    const initAuth = async () => {
      const config = await authApi.getAuthConfig();
      dispatch({ type: 'AUTH_CONFIG_LOADED', payload: config });

      if (config?.mode !== 'disabled') {
        const sessionData = await authApi.getSessionStatus();
        if (sessionData) {
          dispatch({ type: 'SESSION_RESTORE_SUCCESS', payload: sessionData });
        }
      }
    };

    initAuth();
  }, []);

  const login = useCallback(async (credentials: LoginCredentials) => {
    const sessionData = await authApi.loginInternal(credentials.username, credentials.password);
    dispatch({ type: 'SESSION_RESTORE_SUCCESS', payload: sessionData });
  }, []);

  const logout = useCallback(async () => {
    await authApi.logout();
    dispatch({ type: 'LOGOUT' });
  }, []);

  const value = useMemo(
    () => ({ ...state, login, logout }),
    [state, login, logout]
  );

  return (
    <AuthContext.Provider value={value}>
      {children}
    </AuthContext.Provider>
  );
}
```

**Testing**:

```bash
# Component tests
pnpm test AuthContext.test.tsx

# Integration test: wrap test component with AuthProvider
import { AuthProvider } from './context/AuthContext';
import { render } from '@testing-library/react';

render(
  <AuthProvider>
    <YourComponent />
  </AuthProvider>
);
```

### Step 7: Update App.tsx

Wrap existing app with `AuthProvider`:

```bash
code webapp/src/App.tsx
```

**Changes**:

```diff
+ import { AuthProvider } from './context/AuthContext';
+ import { LoginPage } from './components/LoginPage';
+ import { useAuth } from './context/AuthContext';

  function App() {
+   const { user, config, loading } = useAuth();
+
+   if (loading) {
+     return <LoadingSpinner />;
+   }
+
+   if (config?.mode !== 'disabled' && !user) {
+     return <LoginPage />;
+   }

    return (
      <div className="app">
        {/* existing dashboard */}
      </div>
    );
  }

+ export default function Root() {
+   return (
+     <AuthProvider>
+       <App />
+     </AuthProvider>
+   );
+ }
```

### Step 8: Update LoginPage Component

Replace mock service with real auth helpers:

```bash
code webapp/src/components/LoginPage.tsx
```

**Changes**:

```diff
- import { authService } from '../services/authMockService';
+ import { useAuth } from '../context/AuthContext';
+ import { loginInternal, initiateExternalLogin } from '../../js/sdk/auth';

  export function LoginPage() {
-   const [authType, setAuthType] = useState<'oidc' | 'basic'>('oidc');
+   const { config, login } = useAuth();
+   const authType = config?.mode === 'external-idp' ? 'oidc' : 'basic';

    const handleLogin = async (e: React.FormEvent) => {
      e.preventDefault();
      try {
-       const session = await authService.loginWithOIDC(email, password);
-       onLoginSuccess(session);
+       if (authType === 'oidc') {
+         initiateExternalLogin();  // Redirects to IdP
+       } else {
+         const response = await loginInternal(email, password);
+         login(response);
+       }
      } catch (err) {
        setError(err.message);
      }
    };

    // ... rest of component
  }
```

### Step 9: Update AuthStatus Component

Replace mock session with real auth context:

```bash
code webapp/src/components/AuthStatus.tsx
```

**Changes**:

```diff
- import { Session, authService } from '../services/authMockService';
+ import { useAuth } from '../context/AuthContext';

- interface AuthStatusProps {
-   session: Session;
-   onLogout: () => void;
- }

- export function AuthStatus({ session, onLogout }: AuthStatusProps) {
+ export function AuthStatus() {
+   const { user, logout } = useAuth();
+
+   if (!user) return null;

    const handleLogout = async () => {
-     await authService.logout();
-     onLogout();
+     await logout();
    };

    return (
      <div>
-       <span>{session.user.username}</span>
+       <span>{user.username}</span>
        {/* ... rest of component */}
      </div>
    );
  }
```

### Step 10: Add Connect RPC Auth Interceptor

Update `webapp/src/services/gridApi.ts` to add auth interceptor:

```bash
code webapp/src/services/gridApi.ts
```

**Changes**:

```diff
  import { createConnectTransport } from '@connectrpc/connect-web';
+ import { Code, ConnectError } from '@connectrpc/connect';

  export const transport = createConnectTransport({
    baseUrl: 'http://localhost:8080',
+   credentials: 'include',  // Send cookies
+   interceptors: [
+     (next) => async (req) => {
+       try {
+         return await next(req);
+       } catch (err) {
+         if (err instanceof ConnectError && err.code === Code.Unauthenticated) {
+           // Trigger logout on 401
+           window.dispatchEvent(new CustomEvent('auth:logout'));
+         }
+         throw err;
+       }
+     },
+   ],
  });
```

### Step 11: Implement Role-Based Filtering

Update dashboard to filter states based on user roles:

```bash
code webapp/src/hooks/useGridData.ts
```

**Changes**:

```diff
+ import { useAuth } from '../context/AuthContext';

  export function useGridData() {
+   const { user } = useAuth();
    const [states, setStates] = useState<StateInfo[]>([]);

    const loadData = async () => {
      const response = await client.listStates({});
-     setStates(response.states);
+
+     // TODO: Server-side filtering (issue grid-f5947b22)
+     // For now, client-side filtering based on role scope
+     const filtered = filterStatesByUserScope(response.states, user?.roles);
+     setStates(filtered);
    };

+   function filterStatesByUserScope(states: StateInfo[], roles?: string[]): StateInfo[] {
+     if (!roles || roles.length === 0) return [];
+
+     // Simplified: Show all states if user has any role
+     // Real implementation: Evaluate role scope expressions against state labels
+     return states;
+   }

    // ... rest of hook
  }
```

### Step 12: Testing

**Unit Tests**:

```bash
# Test auth helpers
cd js/sdk
pnpm test auth.test.ts

# Test AuthContext
cd webapp
pnpm test src/context/__tests__/AuthContext.test.tsx

# Test LoginPage
pnpm test src/components/__tests__/LoginPage.test.tsx

# Test AuthStatus
pnpm test src/components/__tests__/AuthStatus.test.tsx
```

**Integration Tests**:

```bash
# Start gridapi with test database
export DATABASE_URL="postgres://grid:gridpass@localhost:5432/grid_test?sslmode=disable"
./bin/gridapi serve &

# Run webapp integration tests
cd tests/integration
go test -v -run TestWebappAuth
```

**Manual Testing**:

1. **Without Authentication** (oidc_enabled: false):
   ```bash
   # Stop gridapi
   # Start without OIDC
   unset OIDC_ISSUER
   ./bin/gridapi serve

   # Open webapp - should show dashboard immediately
   open http://localhost:5173
   ```

2. **With Internal IdP**:
   ```bash
   # Start with internal IdP
   export OIDC_ISSUER="http://localhost:8080"
   ./bin/gridapi serve

   # Create bootstrap user first (if not already done in Step 2.5)
   ./bin/gridapi users create \
     --email admin@grid.local \
     --username "Grid Admin" \
     --password "change-me-on-first-login" \
     --role platform-engineer

   # Open webapp - should show login page
   # Try logging in with admin@grid.local / change-me-on-first-login
   open http://localhost:5173
   ```

3. **With External IdP** (requires Keycloak):
   ```bash
   # Start with external IdP config
   export OIDC_EXTERNAL_IDP_ISSUER="http://localhost:9090/realms/grid"
   export OIDC_EXTERNAL_IDP_CLIENT_ID="grid-web-app"
   export OIDC_EXTERNAL_IDP_CLIENT_SECRET="secret"
   ./bin/gridapi serve

   # Open webapp - should show SSO login button
   # Click SSO - should redirect to Keycloak
   ```

## Common Issues

### Issue: 401 on all requests

**Cause**: Cookies not being sent
**Fix**: Ensure `credentials: 'include'` in all fetch calls and Connect transport

### Issue: CORS errors

**Cause**: Gridapi not configured for webapp origin
**Fix**: Add CORS middleware in gridapi router (already implemented)

### Issue: State parameter mismatch

**Cause**: State cookie not being preserved across redirects
**Fix**: Ensure cookies have correct `SameSite` and `Secure` attributes

### Issue: Session not restoring on page reload

**Cause**: `refreshSession()` not called on mount
**Fix**: Check `useEffect` in AuthContext

### Issue: States not filtered by role

**Cause**: Server-side filtering not yet implemented (issue grid-f5947b22)
**Fix**: Implement client-side filtering as temporary solution

## Next Steps

After completing implementation:

1. Run full test suite: `pnpm test && go test ./tests/integration/...`
2. Create PR with implementation
3. Deploy to staging environment
4. Verify with real OIDC provider (Keycloak, Azure AD, etc.)
5. Update documentation with deployment notes

## References

- **Spec**: `specs/007-webapp-auth/spec.md`
- **Research**: `specs/007-webapp-auth/research.md`
- **Data Model**: `specs/007-webapp-auth/data-model.md`
- **Contracts**: `specs/007-webapp-auth/contracts/README.md`
- **Constitution**: `.specify/memory/constitution.md`
- **Designer Mockups**: `webapp/src/components/LoginPage.tsx`, `webapp/src/components/AuthStatus.tsx`
