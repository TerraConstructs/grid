import { GridApiAdapter, createGridTransport, setApiBaseUrl } from '@tcons/grid';

/**
 * Get the Grid API base URL.
 *
 * Uses window.location.origin to ensure requests go through Vite proxy in development.
 * This is CRITICAL for session cookies to work - they require same-origin requests.
 *
 * In development:
 * - Webapp runs on localhost:5174
 * - Using window.location.origin makes requests to localhost:5174
 * - Vite proxy forwards to localhost:8080
 * - Browser sees requests as same-origin, includes httpOnly cookies
 *
 * In production:
 * - Webapp and API deployed same-origin (reverse proxy or embedded)
 * - window.location.origin points to production URL
 * - Can override via VITE_GRID_API_URL if needed
 */
function getApiBaseUrl(): string {
  return import.meta.env.VITE_GRID_API_URL ||
         (typeof window !== 'undefined' ? window.location.origin : 'http://localhost:8080');
}

// Configure the API base URL for both Connect RPC and auth endpoints
const apiBaseUrl = getApiBaseUrl();
setApiBaseUrl(apiBaseUrl);

/**
 * Create and export the Grid API transport.
 */
export const gridTransport = createGridTransport(apiBaseUrl);

/**
 * Create and export the Grid API adapter instance.
 */
export const gridApi = new GridApiAdapter(gridTransport);
