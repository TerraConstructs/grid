import { screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, it, expect, vi } from 'vitest';
import App from '../App';
import { renderWithGrid } from '../../test/gridTestUtils';
import type { GridApiAdapter, StateInfo, DependencyEdge } from '@tcons/grid';

const dependencyEdge: DependencyEdge = {
  id: 42,
  from_guid: 'network-guid',
  from_logic_id: 'network/prod',
  from_output: 'vpc_id',
  to_guid: 'app-guid',
  to_logic_id: 'app/prod',
  to_input_name: 'network_vpc_id',
  status: 'clean',
  created_at: '2024-02-01T00:00:00.000Z',
  updated_at: '2024-02-01T00:30:00.000Z',
};

const networkState: StateInfo = {
  guid: 'network-guid',
  logic_id: 'network/prod',
  created_at: '2024-02-01T00:00:00.000Z',
  updated_at: '2024-02-01T00:10:00.000Z',
  dependency_logic_ids: [],
  backend_config: {
    address: 'https://grid/states/network-guid',
    lock_address: 'https://grid/states/network-guid/lock',
    unlock_address: 'https://grid/states/network-guid/unlock',
  },
  dependencies: [],
  dependents: [{ ...dependencyEdge }],
  outputs: [{ key: 'vpc_id', sensitive: false }],
  computed_status: 'clean',
  size_bytes: 4096,
};

const appState: StateInfo = {
  guid: 'app-guid',
  logic_id: 'app/prod',
  created_at: '2024-02-01T00:00:00.000Z',
  updated_at: '2024-02-01T00:20:00.000Z',
  dependency_logic_ids: ['network/prod'],
  backend_config: {
    address: 'https://grid/states/app-guid',
    lock_address: 'https://grid/states/app-guid/lock',
    unlock_address: 'https://grid/states/app-guid/unlock',
  },
  dependencies: [{ ...dependencyEdge }],
  dependents: [],
  outputs: [
    { key: 'service_url', sensitive: false },
    { key: 'db_password', sensitive: true },
  ],
  computed_status: 'clean',
  size_bytes: 2048,
};

const states = [networkState, appState];
const edges = [dependencyEdge];

describe('Dashboard list view', () => {
  it('shows state and edge tables and opens detail drawer from list row', async () => {
    const listStates = vi.fn().mockResolvedValue(states);
    const getAllEdges = vi.fn().mockResolvedValue(edges);
    const getStateInfo = vi.fn().mockImplementation(async (logicId: string) => {
      return states.find((state) => state.logic_id === logicId) ?? null;
    });

    const api = {
      listStates,
      getAllEdges,
      getStateInfo,
    } as unknown as GridApiAdapter;

    renderWithGrid(<App />, { api });

    await waitFor(() => expect(listStates).toHaveBeenCalledTimes(1));

    const listToggle = await screen.findByRole('button', { name: /List/i });
    await userEvent.click(listToggle);

    await waitFor(() => {
      expect(screen.getByText('States')).toBeInTheDocument();
      expect(screen.getByText('Dependency Edges')).toBeInTheDocument();
    });

    expect(screen.getAllByText('network/prod').length).toBeGreaterThan(0);
    expect(screen.getAllByText('app/prod').length).toBeGreaterThan(0);
    expect(screen.getByText('vpc_id')).toBeInTheDocument();
    expect(screen.getAllByText('clean').length).toBeGreaterThan(0);

    const appStateRows = screen.getAllByText('app/prod');
    await userEvent.click(appStateRows[0]);

    await waitFor(() => {
      expect(getStateInfo).toHaveBeenCalledWith('app/prod');
    });

    expect(await screen.findByText(/State JSON/i)).toBeInTheDocument();
  });
});
