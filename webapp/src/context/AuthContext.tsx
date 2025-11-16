/**
 * Authentication context for managing user session state
 *
 * This context provides authentication state management for the webapp.
 * It handles session restoration, login, logout, and authentication configuration.
 *
 * @see specs/007-webapp-auth/data-model.md
 */

import {
  createContext,
  useContext,
  useReducer,
  ReactNode,
  Dispatch,
  useEffect,
} from 'react';

import type {
  AuthConfig,
  AuthState,
  LoginCredentials,
  LoginResponse,
  WhoamiResponse,
} from '../types/auth';
import {
  fetchAuthConfig,
  loginInternal,
  loginExternal,
  fetchWhoami,
  logout as logoutSDK,
} from '@tcons/grid';

/**
 * Available actions for the auth state machine
 */
export type AuthAction =
  | { type: 'AUTH_CONFIG_LOADED'; payload: AuthConfig }
  | { type: 'SESSION_RESTORE_START' }
  | { type: 'SESSION_RESTORE_SUCCESS'; payload: WhoamiResponse }
  | { type: 'SESSION_RESTORE_FAILED'; payload: string }
  | { type: 'LOGIN_START' }
  | { type: 'LOGIN_SUCCESS'; payload: LoginResponse }
  | { type: 'LOGIN_FAILED'; payload: string }
  | { type: 'LOGOUT' }
  | { type: 'SESSION_EXPIRED' };

/**
 * Auth context value including state and dispatch
 */
interface AuthContextValue {
  state: AuthState;
  dispatch: Dispatch<AuthAction>;
  login: (credentials: LoginCredentials) => Promise<void>;
  loginSSO: () => Promise<void>;
  logout: () => Promise<void>;
}

const AuthContext = createContext<AuthContextValue | null>(null);

/**
 * Initial auth state
 */
const initialState: AuthState = {
  user: null,
  session: null,
  config: null,
  loading: true,
  error: null,
};

/**
 * Auth reducer implementing the state machine from data-model.md
 */
function authReducer(state: AuthState, action: AuthAction): AuthState {
  switch (action.type) {
    case 'AUTH_CONFIG_LOADED':
      return {
        ...state,
        config: action.payload,
      };

    case 'SESSION_RESTORE_START':
      return {
        ...state,
        loading: true,
        error: null,
      };

    case 'SESSION_RESTORE_SUCCESS': {
      const { user, expiresAt } = action.payload;
      return {
        ...state,
        user,
        session: {
          user,
          expiresAt,
          isLoading: false,
          error: null,
        },
        loading: false,
        error: null,
      };
    }

    case 'SESSION_RESTORE_FAILED':
      return {
        ...state,
        user: null,
        session: null,
        loading: false,
        error: action.payload,
      };

    case 'LOGIN_START':
      return {
        ...state,
        loading: true,
        error: null,
      };

    case 'LOGIN_SUCCESS': {
      const { user, expiresAt } = action.payload;
      return {
        ...state,
        user,
        session: {
          user,
          expiresAt,
          isLoading: false,
          error: null,
        },
        loading: false,
        error: null,
      };
    }

    case 'LOGIN_FAILED':
      return {
        ...state,
        user: null,
        session: null,
        loading: false,
        error: action.payload,
      };

    case 'LOGOUT':
      return {
        ...state,
        user: null,
        session: null,
        loading: false,
      };

    case 'SESSION_EXPIRED':
      return {
        ...state,
        user: null,
        session: null,
        loading: false,
        error: 'Session expired. Please log in again.',
      };

    default:
      return state;
  }
}

/**
 * Props for AuthProvider component
 */
interface AuthProviderProps {
  children: ReactNode;
}

/**
 * Auth provider component
 *
 * Provides authentication state and dispatch to child components.
 * Place this at the root of your component tree.
 *
 * @example
 * ```tsx
 * <AuthProvider>
 *   <App />
 * </AuthProvider>
 * ```
 */
export function AuthProvider({ children }: AuthProviderProps) {
  const [state, dispatch] = useReducer(authReducer, initialState);

  // Load auth config on mount
  useEffect(() => {
    const loadAuthConfig = async () => {
      try {
        const config = await fetchAuthConfig();
        dispatch({ type: 'AUTH_CONFIG_LOADED', payload: config });
      } catch (error) {
        console.error('Failed to load auth config:', error);
        // Default to disabled mode on error
        dispatch({
          type: 'AUTH_CONFIG_LOADED',
          payload: { mode: 'disabled', supportsDeviceFlow: false },
        });
      }
    };

    loadAuthConfig();
  }, []);

  // Restore session from cookie on mount (if authenticated)
  useEffect(() => {
    if (state.config === null) {
      // Wait for config to be loaded first
      return;
    }

    if (state.config.mode === 'disabled') {
      // No authentication required, mark as loaded
      dispatch({ type: 'SESSION_RESTORE_SUCCESS', payload: { user: null as any, expiresAt: 0 } });
      return;
    }

    const restoreSession = async () => {
      dispatch({ type: 'SESSION_RESTORE_START' });
      try {
        const response = await fetchWhoami();
        dispatch({ type: 'SESSION_RESTORE_SUCCESS', payload: response });
      } catch (error) {
        // Not authenticated, that's OK
        dispatch({
          type: 'SESSION_RESTORE_FAILED',
          payload: 'Not authenticated',
        });
      }
    };

    restoreSession();
  }, [state.config]);

  // Helper function to handle login
  const login = async (credentials: LoginCredentials) => {
    if (state.config?.mode !== 'internal-idp') {
      throw new Error('Internal IdP login not available');
    }

    dispatch({ type: 'LOGIN_START' });
    try {
      const response = await loginInternal(credentials);
      dispatch({ type: 'LOGIN_SUCCESS', payload: response });
    } catch (error) {
      const errorMessage = error instanceof Error ? error.message : 'Login failed';
      dispatch({ type: 'LOGIN_FAILED', payload: errorMessage });
      throw error;
    }
  };

  // Helper function to handle SSO login
  const loginSSO = async () => {
    if (state.config?.mode !== 'external-idp') {
      throw new Error('SSO login not available');
    }

    dispatch({ type: 'LOGIN_START' });
    try {
      await loginExternal();
      // loginExternal redirects, so this code won't execute
    } catch (error) {
      const errorMessage = error instanceof Error ? error.message : 'SSO login failed';
      dispatch({ type: 'LOGIN_FAILED', payload: errorMessage });
      throw error;
    }
  };

  // Helper function to handle logout
  const logout = async () => {
    try {
      await logoutSDK();
    } catch (error) {
      console.error('Logout request failed:', error);
    } finally {
      dispatch({ type: 'LOGOUT' });
    }
  };

  const value: AuthContextValue = { state, dispatch, login, loginSSO, logout };

  return (
    <AuthContext.Provider value={value}>
      {children}
    </AuthContext.Provider>
  );
}

/**
 * Hook to access auth context
 *
 * @throws Error if used outside AuthProvider
 *
 * @example
 * ```tsx
 * const { state, dispatch } = useAuth();
 * ```
 */
export function useAuth(): AuthContextValue {
  const context = useContext(AuthContext);
  if (!context) {
    throw new Error('useAuth must be used within an AuthProvider');
  }
  return context;
}

/**
 * Hook to access just the auth state
 *
 * @throws Error if used outside AuthProvider
 */
export function useAuthState(): AuthState {
  const { state } = useAuth();
  return state;
}

/**
 * Hook to access just the dispatch function
 *
 * @throws Error if used outside AuthProvider
 */
export function useAuthDispatch(): Dispatch<AuthAction> {
  const { dispatch } = useAuth();
  return dispatch;
}
