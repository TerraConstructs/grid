import { useState } from 'react';
import { authService, Session } from '../services/authMockService';
import { Lock, LogIn, AlertCircle, Eye, EyeOff, ChevronDown } from 'lucide-react';

interface LoginPageProps {
  onLoginSuccess: (session: Session) => void;
}

export function LoginPage({ onLoginSuccess }: LoginPageProps) {
  const [authType, setAuthType] = useState<'oidc' | 'basic'>('oidc');
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [showPassword, setShowPassword] = useState(false);
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(false);
  const [showDemoAccounts, setShowDemoAccounts] = useState(false);

  const handleLogin = async (e: React.FormEvent) => {
    e.preventDefault();
    setError('');
    setLoading(true);

    try {
      let session: Session;

      if (authType === 'oidc') {
        session = await authService.loginWithOIDC(email, password);
      } else {
        session = await authService.loginWithBasicAuth(email, password);
      }

      authService.setSessionCookie(session);
      onLoginSuccess(session);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Login failed');
    } finally {
      setLoading(false);
    }
  };

  const handleDemoLogin = async (username: string, password: string) => {
    setError('');
    setLoading(true);

    try {
      let session: Session;

      if (authType === 'oidc') {
        session = await authService.loginWithOIDC(username, password);
      } else {
        session = await authService.loginWithBasicAuth(username, password);
      }

      authService.setSessionCookie(session);
      onLoginSuccess(session);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Login failed');
    } finally {
      setLoading(false);
    }
  };

  const oidcAccounts = authService.getOIDCDemoAccounts();
  const basicAccounts = authService.getBasicAuthDemoAccounts();
  const currentAccounts = authType === 'oidc' ? oidcAccounts : basicAccounts;

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
            <div className="mb-6">
              <label className="block text-sm font-medium text-gray-700 mb-2">
                Authentication Type
              </label>
              <div className="relative">
                <select
                  value={authType}
                  onChange={(e) => {
                    setAuthType(e.target.value as 'oidc' | 'basic');
                    setEmail('');
                    setPassword('');
                    setError('');
                  }}
                  className="w-full px-4 py-2 border border-gray-300 rounded-lg appearance-none bg-white focus:outline-none focus:ring-2 focus:ring-purple-600 focus:border-transparent"
                >
                  <option value="oidc">OIDC (KeyCloak)</option>
                  <option value="basic">Basic Auth (Internal IdP)</option>
                </select>
                <ChevronDown className="w-4 h-4 text-gray-400 absolute right-3 top-3 pointer-events-none" />
              </div>
            </div>

            <form onSubmit={handleLogin} className="space-y-4">
              <div>
                <label className="block text-sm font-medium text-gray-700 mb-1">
                  {authType === 'oidc' ? 'Email' : 'Username'}
                </label>
                <input
                  type={authType === 'oidc' ? 'email' : 'text'}
                  value={email}
                  onChange={(e) => setEmail(e.target.value)}
                  placeholder={authType === 'oidc' ? 'user@example.com' : 'admin@internal'}
                  className="w-full px-4 py-2 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-purple-600 focus:border-transparent"
                  disabled={loading}
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

              {error && (
                <div className="flex items-center gap-2 px-3 py-2 bg-red-50 border border-red-200 rounded-lg">
                  <AlertCircle className="w-4 h-4 text-red-600" />
                  <p className="text-sm text-red-700">{error}</p>
                </div>
              )}

              <button
                type="submit"
                disabled={loading || !email || !password}
                className="w-full py-2 px-4 bg-gradient-to-r from-purple-600 to-purple-700 text-white rounded-lg font-medium hover:from-purple-700 hover:to-purple-800 disabled:from-gray-400 disabled:to-gray-500 transition-all flex items-center justify-center gap-2"
              >
                <LogIn className="w-4 h-4" />
                {loading ? 'Signing in...' : 'Sign In'}
              </button>
            </form>

            <div className="mt-6 pt-6 border-t border-gray-200">
              <button
                onClick={() => setShowDemoAccounts(!showDemoAccounts)}
                className="w-full text-center text-sm text-purple-600 hover:text-purple-700 font-medium"
              >
                {showDemoAccounts ? 'Hide' : 'Show'} Demo Accounts
              </button>

              {showDemoAccounts && (
                <div className="mt-4 space-y-2">
                  {currentAccounts.map((account, idx) => (
                    <button
                      key={idx}
                      onClick={() => {
                        if (authType === 'oidc') {
                          handleDemoLogin(
                            (account as any).email,
                            account.password
                          );
                        } else {
                          handleDemoLogin(
                            (account as any).username,
                            account.password
                          );
                        }
                      }}
                      disabled={loading}
                      className="w-full text-left px-3 py-2 bg-gray-50 hover:bg-gray-100 rounded-lg text-xs border border-gray-200 transition-colors disabled:opacity-50"
                    >
                      <div className="font-medium text-gray-900">
                        {authType === 'oidc'
                          ? (account as any).email
                          : (account as any).username}
                      </div>
                      <div className="text-gray-600 text-xs mt-0.5">
                        {authType === 'oidc'
                          ? `Groups: ${(account as any).groups.join(', ')}`
                          : `Roles: ${account.roles.join(', ')}`}
                      </div>
                    </button>
                  ))}
                </div>
              )}
            </div>
          </div>
        </div>

        <div className="mt-8 text-center text-sm text-gray-400">
          <p>Demo environment with mock authentication</p>
          <p className="mt-1">All data is simulated</p>
        </div>
      </div>
    </div>
  );
}
