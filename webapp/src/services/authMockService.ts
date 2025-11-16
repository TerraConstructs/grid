export type AuthType = 'oidc' | 'basic' | null;
export type UserRole = 'admin' | 'editor' | 'viewer';

export interface User {
  id: string;
  username: string;
  email: string;
  authType: AuthType;
  roles: UserRole[];
  groups?: string[];
}

export interface Session {
  user: User;
  token: string;
  expiresAt: number;
}

// Mock OIDC users (would come from KeyCloak in production)
const mockOIDCUsers: Record<string, { password: string; groups: string[] }> = {
  'alice@example.com': {
    password: 'oidc-alice-123',
    groups: ['admin-group', 'platform-team'],
  },
  'bob@example.com': {
    password: 'oidc-bob-456',
    groups: ['editor-group', 'app-team'],
  },
  'charlie@example.com': {
    password: 'oidc-charlie-789',
    groups: ['viewer-group'],
  },
};

// Mock basic auth users (internal IdP)
const mockBasicAuthUsers: Record<string, { password: string; roles: UserRole[] }> = {
  'admin@internal': {
    password: 'admin-secure-pass-123',
    roles: ['admin'],
  },
  'editor@internal': {
    password: 'editor-secure-pass-456',
    roles: ['admin', 'editor'],
  },
  'viewer@internal': {
    password: 'viewer-secure-pass-789',
    roles: ['viewer'],
  },
};

const mapGroupsToRoles = (groups: string[]): UserRole[] => {
  const roles: UserRole[] = [];

  if (groups.some(g => g.includes('admin'))) {
    roles.push('admin');
  }
  if (groups.some(g => g.includes('editor'))) {
    roles.push('editor');
  }
  if (groups.some(g => g.includes('viewer'))) {
    roles.push('viewer');
  }

  if (roles.length === 0) {
    roles.push('viewer');
  }

  return roles;
};

const generateToken = (): string => {
  return 'token_' + Math.random().toString(36).substring(2, 15) + '_' + Date.now();
};

export const authService = {
  async loginWithOIDC(email: string, password: string): Promise<Session> {
    await new Promise(resolve => setTimeout(resolve, 800));

    const user = mockOIDCUsers[email];
    if (!user || user.password !== password) {
      throw new Error('Invalid OIDC credentials');
    }

    const roles = mapGroupsToRoles(user.groups);
    const token = generateToken();
    const expiresAt = Date.now() + 24 * 60 * 60 * 1000;

    return {
      user: {
        id: email,
        username: email.split('@')[0],
        email,
        authType: 'oidc',
        roles,
        groups: user.groups,
      },
      token,
      expiresAt,
    };
  },

  async loginWithBasicAuth(username: string, password: string): Promise<Session> {
    await new Promise(resolve => setTimeout(resolve, 800));

    const user = mockBasicAuthUsers[username];
    if (!user || user.password !== password) {
      throw new Error('Invalid basic auth credentials');
    }

    const token = generateToken();
    const expiresAt = Date.now() + 24 * 60 * 60 * 1000;

    return {
      user: {
        id: username,
        username,
        email: `${username}@internal.grid`,
        authType: 'basic',
        roles: user.roles,
      },
      token,
      expiresAt,
    };
  },

  getSessionFromCookie(): Session | null {
    const cookie = document.cookie
      .split(';')
      .find(c => c.trim().startsWith('grid_session='));

    if (!cookie) {
      return null;
    }

    try {
      const encoded = cookie.split('=')[1];
      const decoded = atob(encoded);
      const session = JSON.parse(decoded);

      if (session.expiresAt < Date.now()) {
        authService.clearSession();
        return null;
      }

      return session;
    } catch {
      return null;
    }
  },

  setSessionCookie(session: Session): void {
    const encoded = btoa(JSON.stringify(session));
    const maxAge = (session.expiresAt - Date.now()) / 1000;
    document.cookie = `grid_session=${encoded}; path=/; max-age=${Math.floor(maxAge)}; SameSite=Strict`;
  },

  clearSession(): void {
    document.cookie = 'grid_session=; path=/; max-age=0';
  },

  async logout(): Promise<void> {
    await new Promise(resolve => setTimeout(resolve, 300));
    authService.clearSession();
  },

  getOIDCDemoAccounts(): { email: string; password: string; groups: string[] }[] {
    return Object.entries(mockOIDCUsers).map(([email, data]) => ({
      email,
      password: data.password,
      groups: data.groups,
    }));
  },

  getBasicAuthDemoAccounts(): { username: string; password: string; roles: UserRole[] }[] {
    return Object.entries(mockBasicAuthUsers).map(([username, data]) => ({
      username,
      password: data.password,
      roles: data.roles,
    }));
  },
};
