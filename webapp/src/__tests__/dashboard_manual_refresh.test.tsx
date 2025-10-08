import { waitFor, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, it, expect, vi } from 'vitest';
import App from '../App';
import { renderWithGrid } from '../../test/gridTestUtils';
import type { GridApiAdapter, StateInfo, DependencyEdge } from '@tcons/grid';

describe('Dashboard manual refresh', () => {
  it('preserves selected state after refresh and reflects updated data', async () => {
    const dependencyEdge: DependencyEdge = {
      id: 99,
      from_guid: 'network-guid',
      from_logic_id: 'network/prod',
      from_output: 'vpc_id',
      to_guid: 'app-guid',
      to_logic_id: 'app/prod',
      status: 'clean',
      created_at: '2024-03-01T00:00:00.000Z',
      updated_at: '2024-03-01T00:05:00.000Z',
    };

    const initialAppState: StateInfo = {
      guid: 'app-guid',
      logic_id: 'app/prod',
      created_at: '2024-03-01T00:00:00.000Z',
      updated_at: '2024-03-01T00:20:00.000Z',
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
      size_bytes: 2048,
    };

    const updatedAppState: StateInfo = {
      ...initialAppState,
      updated_at: '2024-03-01T01:00:00.000Z',
      outputs: [
        { key: 'service_url', sensitive: false },
        { key: 'db_password', sensitive: true },
      ],
    };

    const networkState: StateInfo = {
      guid: 'network-guid',
      logic_id: 'network/prod',
      created_at: '2024-03-01T00:00:00.000Z',
      updated_at: '2024-03-01T00:05:00.000Z',
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

    const initialStates = [networkState, initialAppState];
    const updatedStates = [networkState, updatedAppState];

    const initialEdges = [dependencyEdge];
    const updatedEdges = [
      { ...dependencyEdge, status: 'dirty', updated_at: '2024-03-01T01:00:00.000Z' },
    ];

    const listStates = vi
      .fn()
      .mockResolvedValueOnce(initialStates)
      .mockResolvedValueOnce(updatedStates);

    const getAllEdges = vi
      .fn()
      .mockResolvedValueOnce(initialEdges)
      .mockResolvedValueOnce(updatedEdges);

    const getStateInfo = vi
      .fn()
      .mockResolvedValueOnce(initialAppState)
      .mockResolvedValueOnce(updatedAppState);

    const api = {
      listStates,
      getAllEdges,
      getStateInfo,
    } as unknown as GridApiAdapter;

    renderWithGrid(<App />, { api });

    const listButton = await screen.findByRole('button', { name: /^List$/ });
    await userEvent.click(listButton);

    const appRow = await screen.findAllByText('app/prod');
    await userEvent.click(appRow[0]);

    await waitFor(() => {
      expect(getStateInfo).toHaveBeenCalledWith('app/prod');
      expect(screen.getByRole('heading', { name: 'app/prod' })).toBeInTheDocument();
      expect(screen.getByText('service_url')).toBeInTheDocument();
      expect(screen.queryByText('db_password')).not.toBeInTheDocument();
    });

    const refreshButton = screen.getByRole('button', { name: /Refresh/i });
    await userEvent.click(refreshButton);

    await waitFor(() => {
      expect(listStates).toHaveBeenCalledTimes(2);
      expect(getAllEdges).toHaveBeenCalledTimes(2);
      expect(getStateInfo).toHaveBeenCalledTimes(2);
    });

    await waitFor(() => {
      expect(screen.getByRole('heading', { name: 'app/prod' })).toBeInTheDocument();
      expect(screen.getByText('db_password')).toBeInTheDocument();
    });
  });
});
