import { waitFor, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, it, expect, vi } from "vitest";
import App from '../App';
import { renderWithGrid } from '../../test/gridTestUtils';
import type { GridApiAdapter, StateInfo, DependencyEdge } from '@tcons/grid';

describe('Dashboard edge status visualization', () => {
  it('shows edge status legend in graph view and badges in list view', async () => {
    const cleanEdge: DependencyEdge = {
      id: 1,
      from_guid: 'network-guid',
      from_logic_id: 'network/prod',
      from_output: 'vpc_id',
      to_guid: 'app-guid',
      to_logic_id: 'app/prod',
      status: 'clean',
      created_at: '2024-04-01T00:00:00.000Z',
      updated_at: '2024-04-01T00:05:00.000Z',
    };

    const dirtyEdge: DependencyEdge = {
      ...cleanEdge,
      id: 2,
      to_guid: 'analytics-guid',
      to_logic_id: 'analytics/prod',
      status: 'dirty',
    };

    const pendingEdge: DependencyEdge = {
      ...cleanEdge,
      id: 3,
      to_guid: 'billing-guid',
      to_logic_id: 'billing/prod',
      status: 'pending',
    };

    const networkState: StateInfo = {
      guid: 'network-guid',
      logic_id: 'network/prod',
      created_at: '2024-04-01T00:00:00.000Z',
      updated_at: '2024-04-01T00:05:00.000Z',
      dependency_logic_ids: [],
      backend_config: {
        address: 'https://grid/states/network-guid',
        lock_address: 'https://grid/states/network-guid/lock',
        unlock_address: 'https://grid/states/network-guid/unlock',
      },
      dependencies: [],
      dependents: [cleanEdge, dirtyEdge, pendingEdge],
      outputs: [{ key: 'vpc_id', sensitive: false }],
      computed_status: 'clean',
      size_bytes: 4096,
    };

    const appState: StateInfo = {
      guid: 'app-guid',
      logic_id: 'app/prod',
      created_at: '2024-04-01T00:00:00.000Z',
      updated_at: '2024-04-01T00:10:00.000Z',
      dependency_logic_ids: ['network/prod'],
      backend_config: {
        address: 'https://grid/states/app-guid',
        lock_address: 'https://grid/states/app-guid/lock',
        unlock_address: 'https://grid/states/app-guid/unlock',
      },
      dependencies: [cleanEdge],
      dependents: [],
      outputs: [{ key: 'service_url', sensitive: false }],
      computed_status: 'clean',
      size_bytes: 2048,
    };

    const analyticsState: StateInfo = {
      guid: 'analytics-guid',
      logic_id: 'analytics/prod',
      created_at: '2024-04-01T00:00:00.000Z',
      updated_at: '2024-04-01T00:10:00.000Z',
      dependency_logic_ids: ['network/prod'],
      backend_config: networkState.backend_config,
      dependencies: [dirtyEdge],
      dependents: [],
      outputs: [],
      computed_status: 'stale',
      size_bytes: 1024,
    };

    const billingState: StateInfo = {
      guid: 'billing-guid',
      logic_id: 'billing/prod',
      created_at: '2024-04-01T00:00:00.000Z',
      updated_at: '2024-04-01T00:10:00.000Z',
      dependency_logic_ids: ['network/prod'],
      backend_config: networkState.backend_config,
      dependencies: [pendingEdge],
      dependents: [],
      outputs: [],
      computed_status: 'potentially-stale',
      size_bytes: 1024,
    };

    const states = [networkState, appState, analyticsState, billingState];
    const edges = [cleanEdge, dirtyEdge, pendingEdge];

    const listStates = vi.fn().mockResolvedValue(states);
    const getAllEdges = vi.fn().mockResolvedValue(edges);
    const getStateInfo = vi.fn().mockResolvedValue(networkState);

    const api = {
      listStates,
      getAllEdges,
      getStateInfo,
    } as unknown as GridApiAdapter;

    renderWithGrid(<App />, { api });

    await waitFor(() => {
      expect(screen.getByText('Edge Status')).toBeInTheDocument();
      expect(screen.getByText('Clean')).toBeInTheDocument();
      expect(screen.getByText('Dirty')).toBeInTheDocument();
      expect(screen.getByText('Pending')).toBeInTheDocument();
    });

    const listButton = screen.getByRole('button', { name: /^List$/ });
    await userEvent.click(listButton);

    await waitFor(() => {
      expect(screen.getAllByText('clean').length).toBeGreaterThan(0);
      expect(screen.getAllByText('dirty').length).toBeGreaterThan(0);
      expect(screen.getAllByText('pending').length).toBeGreaterThan(0);
    });
  });
});
