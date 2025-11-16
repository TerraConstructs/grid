/**
 * Authentication types for the webapp
 *
 * These types represent client-side authentication state and data structures
 * derived from gridapi responses. All entities are as described in data-model.md.
 *
 * @see specs/007-webapp-auth/data-model.md
 */

/**
 * Represents an authenticated user in the webapp
 */
export interface User {
  /** User ID (sub claim from JWT) */
  id: string;

  /** Display name */
  username: string;

  /** User email */
  email: string;

  /** Authentication mode */
  authType: 'internal' | 'external';

  /** Assigned role names */
  roles: string[];

  /** Group memberships (external IdP only) */
  groups?: string[];
}

/**
 * Represents the user's authentication session
 */
export interface Session {
  /** Authenticated user information */
  user: User;

  /** Session expiration time (Unix timestamp in milliseconds) */
  expiresAt: number;

  /** Whether session check is in progress */
  isLoading: boolean;

  /** Human-readable error message if session check fails */
  error: string | null;
}

/**
 * Represents the gridapi authentication configuration
 */
export interface AuthConfig {
  /** Authentication mode */
  mode: 'internal-idp' | 'external-idp' | 'disabled';

  /** OIDC issuer URL (external IdP only) */
  issuer?: string;

  /** Public OAuth2 client ID (external IdP only) */
  clientId?: string;

  /** Expected aud claim (external IdP only) */
  audience?: string;

  /** Whether device flow is available (CLI use) */
  supportsDeviceFlow: boolean;
}

/**
 * Complete authentication state in React Context
 */
export interface AuthState {
  /** Current authenticated user (null if not authenticated) */
  user: User | null;

  /** Session metadata (null if not authenticated) */
  session: Session | null;

  /** Auth configuration from gridapi (null if not yet fetched) */
  config: AuthConfig | null;

  /** Whether auth check is in progress */
  loading: boolean;

  /** Human-readable error message (null if no error) */
  error: string | null;
}

/**
 * Credentials submitted via login form
 */
export interface LoginCredentials {
  /** Username or email */
  username: string;

  /** User password */
  password: string;
}

/**
 * Response from gridapi auth endpoints
 */
export interface LoginResponse {
  /** Authenticated user */
  user: User;

  /** Session expiration timestamp (Unix milliseconds) */
  expiresAt: number;
}

/**
 * Response from /api/auth/whoami endpoint
 */
export interface WhoamiResponse {
  /** Authenticated user */
  user: User;

  /** Session expiration timestamp (Unix milliseconds) */
  expiresAt: number;
}
