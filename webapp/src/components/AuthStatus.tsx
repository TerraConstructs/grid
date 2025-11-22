import { useState } from 'react';
import { User, LogOut, ChevronDown } from 'lucide-react';
import { logout } from '../../../js/sdk/src/auth';
import type { User as UserType } from '../types/auth';

interface AuthStatusProps {
  user: UserType;
  onLogout: () => void;
}

export function AuthStatus({ user, onLogout }: AuthStatusProps) {
  const [isOpen, setIsOpen] = useState(false);

  const handleLogout = async () => {
    await logout();
    onLogout();
  };

  const getAuthTypeLabel = (type: string) => {
    return type === 'external' ? 'OIDC' : 'Basic Auth';
  };

  const getRoleBadgeColor = (role: string) => {
    switch (role) {
      case 'admin':
        return 'bg-red-100 text-red-800';
      case 'editor':
        return 'bg-blue-100 text-blue-800';
      case 'viewer':
        return 'bg-green-100 text-green-800';
      default:
        return 'bg-gray-100 text-gray-800';
    }
  };

  return (
    <div className="relative">
      <button
        onClick={() => setIsOpen(!isOpen)}
        className="flex items-center gap-2 px-3 py-1.5 rounded-lg text-sm font-medium bg-gray-700 text-gray-300 hover:bg-gray-600 transition-colors"
      >
        <User className="w-4 h-4" />
        <span className="hidden sm:inline">{user.username}</span>
        <ChevronDown className="w-4 h-4" />
      </button>

      {isOpen && (
        <>
          <div
            className="fixed inset-0 z-10"
            onClick={() => setIsOpen(false)}
          />
          <div className="absolute top-full right-0 mt-2 w-80 bg-white rounded-lg shadow-xl border border-gray-200 z-20">
            <div className="p-4 border-b border-gray-200">
              <div className="flex items-center gap-3">
                <div className="w-10 h-10 bg-purple-100 rounded-lg flex items-center justify-center">
                  <User className="w-6 h-6 text-purple-600" />
                </div>
                <div className="flex-1">
                  <p className="font-semibold text-gray-900">{user.username}</p>
                  <p className="text-xs text-gray-500">{user.email}</p>
                </div>
              </div>
            </div>

            <div className="p-4 space-y-3">
              <div>
                <p className="text-xs font-medium text-gray-500 uppercase mb-1">
                  Authentication Type
                </p>
                <div className="inline-block px-3 py-1 bg-blue-100 text-blue-800 rounded text-xs font-medium">
                  {getAuthTypeLabel(user.authType)}
                </div>
              </div>

              <div>
                <p className="text-xs font-medium text-gray-500 uppercase mb-2">
                  Roles
                </p>
                <div className="flex flex-wrap gap-1.5">
                  {user.roles.length === 0 ? (
                    <span className="text-xs text-gray-500">No roles assigned</span>
                  ) : (
                    user.roles.map((role) => (
                      <span
                        key={role}
                        className={`px-2 py-1 rounded text-xs font-medium ${getRoleBadgeColor(
                          role
                        )}`}
                      >
                        {role}
                      </span>
                    ))
                  )}
                </div>
              </div>

              {user.groups && user.groups.length > 0 && (
                <div>
                  <p className="text-xs font-medium text-gray-500 uppercase mb-2">
                    Group Memberships
                  </p>
                  <div className="space-y-1">
                    {user.groups.map((group) => (
                      <div
                        key={group}
                        className="px-2 py-1 bg-gray-50 rounded text-xs text-gray-700"
                      >
                        {group}
                      </div>
                    ))}
                  </div>
                </div>
              )}

              {user.sessionExpiresAt && (
                <div>
                  <p className="text-xs font-medium text-gray-500 uppercase mb-1">
                    Session Expires
                  </p>
                  <p className="text-xs text-gray-700">
                    {new Date(user.sessionExpiresAt).toLocaleString()}
                  </p>
                </div>
              )}
            </div>

            <div className="p-4 border-t border-gray-200">
              <button
                onClick={handleLogout}
                className="w-full flex items-center justify-center gap-2 px-3 py-2 bg-red-50 hover:bg-red-100 text-red-700 rounded-lg text-sm font-medium transition-colors"
              >
                <LogOut className="w-4 h-4" />
                Log Out
              </button>
            </div>
          </div>
        </>
      )}
    </div>
  );
}
