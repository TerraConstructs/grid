/**
 * Authentication types for the Grid SDK
 *
 * Shared authentication types used by both the SDK and webapp consumers.
 * These types are derived from gridapi authentication responses.
 *
 * @see specs/007-webapp-auth/data-model.md
 */

/**
 * Represents an authenticated user
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
 * Session information from whoami endpoint
 */
export interface Session {
  /** Session ID */
  id: string;

  /** Session expiration timestamp (Unix milliseconds) */
  expiresAt: number;
}

/**
 * Response from /api/auth/whoami endpoint
 */
export interface WhoamiResponse {
  /** Authenticated user */
  user: User;

  /** Session information */
  session: Session;
}

/**
 * Grid authentication configuration
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
