/**
 * Browser authentication helpers for Grid SDK
 *
 * This module provides browser-based authentication operations for connecting to gridapi.
 * These functions handle HTTP calls to /auth/* endpoints as per the constitutional exception
 * for OIDC authentication flows (see specs/007-webapp-auth/plan.md).
 *
 * @see specs/007-webapp-auth/contracts/README.md for endpoint specifications
 * @see specs/007-webapp-auth/data-model.md for data model
 */

import type {
  AuthConfig,
  LoginCredentials,
  LoginResponse,
  WhoamiResponse,
} from './types/auth.js';

/**
 * Base URL for API calls
 *
 * **CRITICAL**: Defaults to current origin for browser compatibility (enables Vite proxy and session cookies).
 * Can be overridden by consumers of this module using setApiBaseUrl().
 *
 * **Why `window.location.origin`**:
 * - Development: Webapp runs on localhost:5173, API on localhost:8080 (different origins)
 * - Vite proxy in webapp/vite.config.ts proxies API requests to localhost:8080
 * - Using window.location.origin ensures requests go through proxy (same-origin)
 * - httpOnly session cookies ONLY work with same-origin requests
 *
 * **DO NOT** hardcode to 'http://localhost:8080' - it will break authentication:
 * - Session cookies won't be sent (different origin)
 * - SSO callback will fail (cross-origin cookie issue)
 * - All authenticated requests will return 401
 *
 * **Production**: Webapp and API must be same-origin (reverse proxy or embedded static files)
 *
 * See:
 * - webapp/README.md: Deployment architecture and proxy configuration
 * - Beads issue grid-202d: SSO callback redirect fix
 * - specs/007-webapp-auth/: Authentication implementation details
 */
let API_BASE_URL =
  typeof window !== 'undefined' ? window.location.origin : 'http://localhost:8080';

/**
 * Configure the API base URL
 *
 * @param url The base URL for Grid API calls
 */
export function setApiBaseUrl(url: string): void {
  API_BASE_URL = url;
}

/**
 * Fetch authentication configuration from gridapi
 *
 * Makes a request to GET /auth/config to determine the authentication mode
 * (internal IdP, external IdP, or disabled).
 *
 * @returns Authentication configuration
 * @throws Error if the request fails
 *
 * @example
 * ```typescript
 * const config = await fetchAuthConfig();
 * if (config.mode === 'disabled') {
 *   // No authentication required
 * } else if (config.mode === 'internal-idp') {
 *   // Show username/password login form
 * } else {
 *   // Show SSO login button
 * }
 * ```
 */
export async function fetchAuthConfig(): Promise<AuthConfig> {
  try {
    const response = await fetch(`${API_BASE_URL}/auth/config`, {
      method: 'GET',
      headers: { 'Content-Type': 'application/json' },
    });

    if (!response.ok) {
      // Default to disabled mode on error
      return {
        mode: 'disabled',
        supportsDeviceFlow: false,
      };
    }

    const data = await response.json() as any; // API returns snake_case
    return {
      mode: data.mode,
      issuer: data.issuer,
      clientId: data.client_id, // Map snake_case to camelCase
      audience: data.audience,
      supportsDeviceFlow: data.supports_device_flow || false, // Map snake_case to camelCase
    };
  } catch (error) {
    // Default to disabled mode on network error
    return {
      mode: 'disabled',
      supportsDeviceFlow: false,
    };
  }
}

/**
 * Authenticate using internal IdP (username/password)
 *
 * Makes a POST request to /auth/login with the user's credentials.
 * The server will set an httpOnly session cookie on successful authentication.
 *
 * @param credentials User credentials (username and password)
 * @returns Login response with authenticated user and session expiration time
 * @throws Error if authentication fails
 *
 * @example
 * ```typescript
 * const response = await loginInternal({
 *   username: 'user@example.com',
 *   password: 'password123'
 * });
 * console.log(response.user.username); // Logged-in user
 * ```
 */
export async function loginInternal(
  credentials: LoginCredentials
): Promise<LoginResponse> {
  const response = await fetch(`${API_BASE_URL}/auth/login`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    credentials: 'include', // Include httpOnly cookies
    body: JSON.stringify({
      username: credentials.username,
      password: credentials.password,
    }),
  });

  if (!response.ok) {
    const errorText = await response.text();
    throw new Error(`Login failed: ${response.status} ${errorText}`);
  }

  const data = await response.json() as any; // API returns snake_case
  return {
    user: {
      id: data.user.id,
      username: data.user.username,
      email: data.user.email,
      authType: data.user.auth_type, // Map snake_case to camelCase
      roles: data.user.roles || [],
      groups: data.user.groups,
    },
    expiresAt: data.expires_at, // Map snake_case to camelCase
  };
}

/**
 * Authenticate using external IdP (SSO)
 *
 * Initiates an OAuth2/OIDC authorization code flow by redirecting to the
 * external IdP (e.g., Keycloak). After user grants permission, the IdP
 * redirects back to /auth/callback which handles token exchange and
 * session creation.
 *
 * This function triggers a page redirect and does not return.
 *
 * The redirect_uri parameter is passed to ensure the server redirects back
 * to the webapp origin after successful authentication. This is critical for
 * development mode where webapp and API run on different ports.
 *
 * @throws Error if the login flow cannot be initiated
 *
 * @see Beads issue grid-202d - SSO callback redirect fix
 *
 * @example
 * ```typescript
 * // User clicks "Sign In with SSO" button
 * async function handleSSOClick() {
 *   try {
 *     await loginExternal();
 *     // This function will redirect the page, so no code runs after this
 *   } catch (error) {
 *     console.error('SSO login failed:', error);
 *   }
 * }
 * ```
 */
export async function loginExternal(): Promise<void> {
  // Redirect to /auth/sso/login which initiates the OAuth2 flow
  // Pass redirect_uri to tell the server where to redirect after authentication
  // In dev mode: localhost:5173 -> gridapi redirects back to localhost:5173 (not localhost:8080)
  // In prod: same-origin, so redirect_uri = current origin
  const redirectUri = encodeURIComponent(window.location.origin + '/');
  window.location.href = `${API_BASE_URL}/auth/sso/login?redirect_uri=${redirectUri}`;
}

/**
 * Restore user session from httpOnly cookie
 *
 * Makes a GET request to /api/auth/whoami to fetch the authenticated user
 * and session information from the server. The httpOnly session cookie is
 * automatically included by the browser.
 *
 * This is called on app load to restore the user session if one exists.
 *
 * @returns Session information with authenticated user and expiration time
 * @throws Error if the request fails or user is not authenticated
 *
 * @example
 * ```typescript
 * useEffect(() => {
 *   fetchWhoami()
 *     .then(response => {
 *       dispatch({ type: 'SESSION_RESTORE_SUCCESS', payload: response });
 *     })
 *     .catch(error => {
 *       dispatch({ type: 'SESSION_RESTORE_FAILED', payload: error.message });
 *     });
 * }, [dispatch]);
 * ```
 */
export async function fetchWhoami(): Promise<WhoamiResponse> {
  const response = await fetch(`${API_BASE_URL}/api/auth/whoami`, {
    method: 'GET',
    headers: { 'Content-Type': 'application/json' },
    credentials: 'include', // Include httpOnly cookies
  });

  if (!response.ok) {
    const errorText = await response.text();
    throw new Error(`Fetch whoami failed: ${response.status} ${errorText}`);
  }

  const data = await response.json() as any; // API returns snake_case
  return {
    user: {
      id: data.user.id,
      username: data.user.username,
      email: data.user.email,
      authType: data.user.auth_type, // Map snake_case to camelCase
      roles: data.user.roles || [],
      groups: data.user.groups,
    },
    session: {
      id: data.session.id,
      expiresAt: data.session.expires_at, // Map snake_case to camelCase
    },
  };
}

/**
 * Log out the current user
 *
 * Makes a POST request to /auth/logout to invalidate the user's session
 * on the server. The httpOnly session cookie is cleared by the server.
 *
 * @throws Error if the logout request fails
 *
 * @example
 * ```typescript
 * async function handleLogout() {
 *   try {
 *     await logout();
 *     dispatch({ type: 'LOGOUT' });
 *     navigate('/');
 *   } catch (error) {
 *     console.error('Logout failed:', error);
 *   }
 * }
 * ```
 */
export async function logout(): Promise<void> {
  const response = await fetch(`${API_BASE_URL}/auth/logout`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    credentials: 'include', // Include httpOnly cookies
  });

  if (!response.ok) {
    const errorText = await response.text();
    throw new Error(`Logout failed: ${response.status} ${errorText}`);
  }
}
