import { describe, it, expect, vi } from 'vitest';
import { screen, waitFor } from '@testing-library/react';
import App from '../App';
import { renderWithGrid } from '../../test/gridTestUtils';
import type { GridApiAdapter, StateInfo, DependencyEdge } from '@tcons/grid';

const dependencyEdge: DependencyEdge = {
  id: 1,
  from_guid: 'network-guid',
  from_logic_id: 'network/prod',
  from_output: 'vpc_id',
  to_guid: 'app-guid',
  to_logic_id: 'app/prod',
  status: 'clean',
  created_at: '2024-01-01T00:00:00.000Z',
  updated_at: '2024-01-01T00:10:00.000Z',
};

const networkState: StateInfo = {
  guid: 'network-guid',
  logic_id: 'network/prod',
  created_at: '2024-01-01T00:00:00.000Z',
  updated_at: '2024-01-01T00:05:00.000Z',
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
  size_bytes: 2048,
};

const appState: StateInfo = {
  guid: 'app-guid',
  logic_id: 'app/prod',
  created_at: '2024-01-01T00:00:00.000Z',
  updated_at: '2024-01-01T00:15:00.000Z',
  dependency_logic_ids: ['network/prod'],
  backend_config: {
    address: 'https://grid/states/app-guid',
    lock_address: 'https://grid/states/app-guid/lock',
    unlock_address: 'https://grid/states/app-guid/unlock',
  },
  dependencies: [{ ...dependencyEdge }],
  dependents: [],
  outputs: [{ key: 'service_url', sensitive: false }],
  computed_status: 'clean',
  size_bytes: 1024,
};

const states = [networkState, appState];
const edges = [dependencyEdge];

describe('Dashboard initial load', () => {
  it('renders loader and resolves with live state + edge counts', async () => {
    const createDeferred = <T,>() => {
      let resolve!: (value: T) => void;
      const promise = new Promise<T>((res) => {
        resolve = res;
      });
      return { promise, resolve };
    };

    const listStatesDeferred = createDeferred<StateInfo[]>();
    const listStates = vi.fn().mockReturnValue(listStatesDeferred.promise);

    const getAllEdgesDeferred = createDeferred<DependencyEdge[]>();
    const getAllEdges = vi.fn().mockReturnValue(getAllEdgesDeferred.promise);
    const getStateInfo = vi.fn();

    const api = {
      listStates,
      getAllEdges,
      getStateInfo,
    } as unknown as GridApiAdapter;

    renderWithGrid(<App />, { api });

    expect(screen.getByText(/Loading Grid/i)).toBeInTheDocument();

    expect(listStates).toHaveBeenCalledTimes(1);
    expect(getAllEdges).toHaveBeenCalledTimes(1);

    listStatesDeferred.resolve(states);
    getAllEdgesDeferred.resolve(edges);

    await waitFor(() => {
      const header = screen.getByRole('banner');
      expect(header).toHaveTextContent('2 states');
      expect(header).toHaveTextContent('1 edges');
      expect(screen.queryByText(/Loading Grid/i)).not.toBeInTheDocument();
    });

    expect(screen.getByText('network/prod')).toBeInTheDocument();
    expect(screen.getByText('app/prod')).toBeInTheDocument();
  });
});
