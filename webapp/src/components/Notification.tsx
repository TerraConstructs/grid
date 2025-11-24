/**
 * Toast notification component for Grid webapp
 *
 * Displays success/error messages with auto-dismiss functionality.
 * Uses role="alert" for accessibility and E2E test selectors.
 */

import { useEffect } from 'react';
import { CheckCircle, XCircle, X } from 'lucide-react';

export type NotificationType = 'success' | 'error';

export interface Notification {
  id: string;
  type: NotificationType;
  message: string;
}

interface NotificationToastProps {
  notification: Notification;
  onDismiss: (id: string) => void;
}

export function NotificationToast({ notification, onDismiss }: NotificationToastProps) {
  // Auto-dismiss after 5 seconds
  useEffect(() => {
    const timer = setTimeout(() => {
      onDismiss(notification.id);
    }, 5000);

    return () => clearTimeout(timer);
  }, [notification.id, onDismiss]);

  const isSuccess = notification.type === 'success';

  return (
    <div
      role="alert"
      data-testid={`notification-toast-${notification.id}`}
      className={`flex items-start gap-3 p-4 rounded-lg shadow-lg border ${
        isSuccess
          ? 'bg-green-50 border-green-200'
          : 'bg-red-50 border-red-200'
      } min-w-[300px] max-w-md animate-slide-in`}
    >
      {isSuccess ? (
        <CheckCircle className="w-5 h-5 text-green-600 flex-shrink-0 mt-0.5" />
      ) : (
        <XCircle className="w-5 h-5 text-red-600 flex-shrink-0 mt-0.5" />
      )}

      <div className="flex-1">
        <p
          className={`text-sm font-medium ${
            isSuccess ? 'text-green-900' : 'text-red-900'
          }`}
        >
          {notification.message}
        </p>
      </div>

      <button
        onClick={() => onDismiss(notification.id)}
        data-testid={`notification-dismiss-${notification.id}`}
        className={`flex-shrink-0 ${
          isSuccess
            ? 'text-green-600 hover:text-green-800'
            : 'text-red-600 hover:text-red-800'
        }`}
      >
        <X className="w-4 h-4" />
      </button>
    </div>
  );
}

interface NotificationContainerProps {
  notifications: Notification[];
  onDismiss: (id: string) => void;
}

export function NotificationContainer({
  notifications,
  onDismiss,
}: NotificationContainerProps) {
  if (notifications.length === 0) {
    return null;
  }

  return (
    <div className="fixed top-4 right-4 z-50 flex flex-col gap-2">
      {notifications.map((notification) => (
        <NotificationToast
          key={notification.id}
          notification={notification}
          onDismiss={onDismiss}
        />
      ))}
    </div>
  );
}
