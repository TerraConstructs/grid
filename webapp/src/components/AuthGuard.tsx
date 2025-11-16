/**
 * Route protection component that guards authenticated routes
 *
 * Redirects unauthenticated users to the login page while showing a loading spinner
 * during authentication state checking.
 *
 * @see specs/007-webapp-auth/spec.md FR-004
 */

import { ReactNode } from 'react';
import { useAuthState } from '../context/AuthContext';

interface AuthGuardProps {
  children: ReactNode;
}

/**
 * AuthGuard component for protecting authenticated routes
 *
 * @example
 * ```tsx
 * <AuthGuard>
 *   <Dashboard />
 * </AuthGuard>
 * ```
 */
export function AuthGuard({ children }: AuthGuardProps) {
  const authState = useAuthState();

  // Show loading spinner while checking authentication
  if (authState.loading) {
    return (
      <div className="flex items-center justify-center min-h-screen">
        <div className="animate-spin rounded-full h-12 w-12 border-b-2 border-blue-500" />
      </div>
    );
  }

  // Redirect to login if not authenticated
  if (!authState.user) {
    window.location.href = '/login';
    return null;
  }

  // Render protected content
  return <>{children}</>;
}
