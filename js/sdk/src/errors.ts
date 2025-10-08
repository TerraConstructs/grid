import { ConnectError, Code } from '@connectrpc/connect';

/**
 * User-friendly error representation with actionable information.
 */
export interface UserFriendlyError {
  /** Short error title */
  title: string;

  /** Detailed user-facing message */
  message: string;

  /** Original Connect error code */
  code: Code;

  /** Whether the operation can be retried */
  canRetry: boolean;
}

/**
 * Normalize a Connect RPC error to a user-friendly format.
 *
 * Maps all 16 gRPC error codes to human-readable messages suitable
 * for display in UI toast notifications or error boundaries.
 *
 * @param error - The error to normalize (may or may not be a ConnectError)
 * @param context - Optional context string to prepend to the message
 * @returns User-friendly error object
 *
 * @example
 * ```typescript
 * try {
 *   await client.getStateInfo({ state: { case: 'logicId', value: 'missing' } });
 * } catch (error) {
 *   const friendly = normalizeConnectError(error, 'Loading state');
 *   console.error(friendly.title, friendly.message);
 *   // "Not Found: Loading state: The requested state could not be found."
 * }
 * ```
 */
export function normalizeConnectError(
  error: unknown,
  context?: string
): UserFriendlyError {
  if (!(error instanceof ConnectError)) {
    return {
      title: 'Unexpected Error',
      message: 'An unexpected error occurred. Please try again.',
      code: Code.Unknown,
      canRetry: true,
    };
  }

  const baseContext = context ? `${context}: ` : '';

  switch (error.code) {
    case Code.Canceled:
      return {
        title: 'Canceled',
        message: `${baseContext}The operation was canceled.`,
        code: error.code,
        canRetry: false,
      };

    case Code.Unknown:
      return {
        title: 'Unknown Error',
        message: `${baseContext}An unexpected error occurred.`,
        code: error.code,
        canRetry: true,
      };

    case Code.InvalidArgument:
      return {
        title: 'Invalid Input',
        message: `${baseContext}The provided input is invalid.`,
        code: error.code,
        canRetry: false,
      };

    case Code.DeadlineExceeded:
      return {
        title: 'Timeout',
        message: `${baseContext}The request took too long. Please try again.`,
        code: error.code,
        canRetry: true,
      };

    case Code.NotFound:
      return {
        title: 'Not Found',
        message: `${baseContext}The requested state could not be found.`,
        code: error.code,
        canRetry: false,
      };

    case Code.AlreadyExists:
      return {
        title: 'Already Exists',
        message: `${baseContext}A state with this identifier already exists.`,
        code: error.code,
        canRetry: false,
      };

    case Code.PermissionDenied:
      return {
        title: 'Permission Denied',
        message: `${baseContext}You don't have permission to perform this action.`,
        code: error.code,
        canRetry: false,
      };

    case Code.ResourceExhausted:
      return {
        title: 'Rate Limited',
        message: `${baseContext}Too many requests. Please wait and try again.`,
        code: error.code,
        canRetry: true,
      };

    case Code.FailedPrecondition:
      return {
        title: 'Failed Precondition',
        message: `${baseContext}The operation cannot be performed in the current state.`,
        code: error.code,
        canRetry: false,
      };

    case Code.Aborted:
      return {
        title: 'Conflict',
        message: `${baseContext}The operation was aborted due to a conflict. Please retry.`,
        code: error.code,
        canRetry: true,
      };

    case Code.OutOfRange:
      return {
        title: 'Out of Range',
        message: `${baseContext}The provided value is out of range.`,
        code: error.code,
        canRetry: false,
      };

    case Code.Unimplemented:
      return {
        title: 'Not Supported',
        message: `${baseContext}This feature is not currently supported.`,
        code: error.code,
        canRetry: false,
      };

    case Code.Internal:
      return {
        title: 'Server Error',
        message: `${baseContext}A server error occurred. Please try again later.`,
        code: error.code,
        canRetry: true,
      };

    case Code.Unavailable:
      return {
        title: 'Service Unavailable',
        message: `${baseContext}The service is temporarily unavailable. Please try again.`,
        code: error.code,
        canRetry: true,
      };

    case Code.DataLoss:
      return {
        title: 'Data Loss',
        message: `${baseContext}A server error occurred. Please contact support.`,
        code: error.code,
        canRetry: false,
      };

    case Code.Unauthenticated:
      return {
        title: 'Authentication Required',
        message: `${baseContext}Please sign in to continue.`,
        code: error.code,
        canRetry: false,
      };

    default:
      // Fallback for any unknown code
      return {
        title: 'Error',
        message: error.rawMessage || `${baseContext}An error occurred.`,
        code: error.code,
        canRetry: true,
      };
  }
}
