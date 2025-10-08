import { GridApiAdapter, createGridTransport } from '@tcons/grid';

/**
 * Get the Grid API base URL from environment variables.
 * Defaults to http://localhost:8080 for local development.
 */
function getApiBaseUrl(): string {
  return import.meta.env.VITE_GRID_API_URL || 'http://localhost:8080';
}

/**
 * Create and export the Grid API transport.
 */
export const gridTransport = createGridTransport(getApiBaseUrl());

/**
 * Create and export the Grid API adapter instance.
 */
export const gridApi = new GridApiAdapter(gridTransport);
