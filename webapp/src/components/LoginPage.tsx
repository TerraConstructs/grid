/**
 * Login page for Grid application
 *
 * Supports both internal IdP (username/password) and external IdP (SSO) authentication modes.
 * Automatically detects the configured authentication mode and shows appropriate UI.
 *
 * @see specs/007-webapp-auth/spec.md FR-002
 */

import { useState } from 'react';
import { Lock, LogIn, AlertCircle, Eye, EyeOff } from 'lucide-react';
import { useAuthState, useAuth } from '../context/AuthContext';
import type { LoginCredentials } from '../types/auth';

export function LoginPage() {
  const authState = useAuthState();
  const { state, dispatch } = useAuth();
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [showPassword, setShowPassword] = useState(false);
  const [loading, setLoading] = useState(false);

  // Wait for config to be loaded
  if (authState.config === null) {
    return (
      <div className="min-h-screen bg-gray-900 flex items-center justify-center">
        <div className="flex items-center gap-3">
          <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-purple-600" />
          <span className="text-white">Loading...</span>
        </div>
      </div>
    );
  }

  const handleInternalIdPLogin = async (e: React.FormEvent) => {
    e.preventDefault();
    setLoading(true);

    try {
      const credentials: LoginCredentials = { username, password };
      // Call login through dispatch which is handled in AuthContext
      dispatch({ type: 'LOGIN_START' });

      // Import and call directly since we can't use async dispatch
      const { loginInternal } = await import('@tcons/grid');
      const response = await loginInternal(credentials);
      dispatch({ type: 'LOGIN_SUCCESS', payload: response });
    } catch (error) {
      const errorMessage = error instanceof Error ? error.message : 'Login failed';
      dispatch({ type: 'LOGIN_FAILED', payload: errorMessage });
    } finally {
      setLoading(false);
    }
  };

  const handleSSO = async () => {
    try {
      const { loginExternal } = await import('@tcons/grid');
      await loginExternal();
      // loginExternal redirects, so this won't complete
    } catch (error) {
      console.error('SSO login failed:', error);
    }
  };

  const isInternalIdP = authState.config.mode === 'internal-idp';
  const isExternalIdP = authState.config.mode === 'external-idp';

  return (
    <div className="min-h-screen bg-gradient-to-br from-gray-900 via-gray-800 to-gray-900 flex items-center justify-center p-4">
      <div className="w-full max-w-md">
        <div className="bg-white rounded-xl shadow-2xl overflow-hidden">
          <div className="bg-gradient-to-r from-purple-600 to-purple-700 px-6 py-8">
            <div className="flex items-center justify-center gap-3 mb-2">
              <div className="w-10 h-10 bg-white rounded-lg flex items-center justify-center">
                <Lock className="w-6 h-6 text-purple-600" />
              </div>
              <h1 className="text-2xl font-bold text-white">Grid</h1>
            </div>
            <p className="text-purple-100 text-sm text-center">Terraform State Management</p>
          </div>

          <div className="p-6">
            {isInternalIdP && (
              <form onSubmit={handleInternalIdPLogin} className="space-y-4">
                <div>
                  <label className="block text-sm font-medium text-gray-700 mb-1">
                    Username
                  </label>
                  <input
                    type="text"
                    value={username}
                    onChange={(e) => setUsername(e.target.value)}
                    placeholder="admin"
                    className="w-full px-4 py-2 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-purple-600 focus:border-transparent"
                    disabled={loading}
                    autoComplete="username"
                  />
                </div>

                <div>
                  <label className="block text-sm font-medium text-gray-700 mb-1">
                    Password
                  </label>
                  <div className="relative">
                    <input
                      type={showPassword ? 'text' : 'password'}
                      value={password}
                      onChange={(e) => setPassword(e.target.value)}
                      placeholder="Enter password"
                      className="w-full px-4 py-2 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-purple-600 focus:border-transparent pr-10"
                      disabled={loading}
                      autoComplete="current-password"
                    />
                    <button
                      type="button"
                      onClick={() => setShowPassword(!showPassword)}
                      className="absolute right-3 top-2.5 text-gray-400 hover:text-gray-600"
                    >
                      {showPassword ? (
                        <EyeOff className="w-4 h-4" />
                      ) : (
                        <Eye className="w-4 h-4" />
                      )}
                    </button>
                  </div>
                </div>

                {state.error && (
                  <div className="flex items-center gap-2 px-3 py-2 bg-red-50 border border-red-200 rounded-lg">
                    <AlertCircle className="w-4 h-4 text-red-600" />
                    <p className="text-sm text-red-700">{state.error}</p>
                  </div>
                )}

                <button
                  type="submit"
                  disabled={loading || !username || !password}
                  className="w-full py-2 px-4 bg-gradient-to-r from-purple-600 to-purple-700 text-white rounded-lg font-medium hover:from-purple-700 hover:to-purple-800 disabled:from-gray-400 disabled:to-gray-500 transition-all flex items-center justify-center gap-2"
                >
                  <LogIn className="w-4 h-4" />
                  {loading ? 'Signing in...' : 'Sign In'}
                </button>
              </form>
            )}

            {isExternalIdP && (
              <div className="space-y-4">
                <p className="text-sm text-gray-600 text-center">
                  Sign in with your organization account
                </p>

                {state.error && (
                  <div className="flex items-center gap-2 px-3 py-2 bg-red-50 border border-red-200 rounded-lg">
                    <AlertCircle className="w-4 h-4 text-red-600" />
                    <p className="text-sm text-red-700">{state.error}</p>
                  </div>
                )}

                <button
                  onClick={handleSSO}
                  disabled={loading}
                  className="w-full py-2 px-4 bg-gradient-to-r from-purple-600 to-purple-700 text-white rounded-lg font-medium hover:from-purple-700 hover:to-purple-800 disabled:from-gray-400 disabled:to-gray-500 transition-all flex items-center justify-center gap-2"
                >
                  <LogIn className="w-4 h-4" />
                  {loading ? 'Redirecting...' : 'Sign In with SSO'}
                </button>
              </div>
            )}
          </div>
        </div>

        <div className="mt-8 text-center text-sm text-gray-400">
          <p>Grid - Terraform State Management</p>
        </div>
      </div>
    </div>
  );
}
