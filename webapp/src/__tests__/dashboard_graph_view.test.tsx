import { describe, it, expect, vi } from 'vitest';
import { screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import App from '../App';
import { renderWithGrid } from '../../test/gridTestUtils';
import type { GridApiAdapter, StateInfo, DependencyEdge } from '@tcons/grid';

describe('Dashboard graph view filtering', () => {
  it('fetches filtered states when label filters are applied', async () => {
    const dependencyEdge: DependencyEdge = {
      id: 7,
      from_guid: 'network-guid',
      from_logic_id: 'network/prod',
      from_output: 'vpc_id',
      to_guid: 'app-guid',
      to_logic_id: 'app/prod',
      status: 'clean',
      created_at: '2024-02-01T00:00:00.000Z',
      updated_at: '2024-02-01T00:30:00.000Z',
    };

    const networkState: StateInfo = {
      guid: 'network-guid',
      logic_id: 'network/prod',
      created_at: '2024-02-01T00:00:00.000Z',
      updated_at: '2024-02-01T00:05:00.000Z',
      dependency_logic_ids: [],
      backend_config: {
        address: 'https://grid/states/network-guid',
        lock_address: 'https://grid/states/network-guid/lock',
        unlock_address: 'https://grid/states/network-guid/unlock',
      },
      dependencies: [],
      dependents: [dependencyEdge],
      outputs: [{ key: 'vpc_id', sensitive: false }],
      computed_status: 'clean',
      size_bytes: 4096,
    };

    const appState: StateInfo = {
      guid: 'app-guid',
      logic_id: 'app/prod',
      created_at: '2024-02-01T00:00:00.000Z',
      updated_at: '2024-02-01T00:25:00.000Z',
      dependency_logic_ids: ['network/prod'],
      backend_config: networkState.backend_config,
      dependencies: [dependencyEdge],
      dependents: [],
      outputs: [{ key: 'service_url', sensitive: false }],
      computed_status: 'clean',
      size_bytes: 2048,
    };

    const states = [networkState, appState];
    const edges = [dependencyEdge];

    const listStates = vi.fn().mockImplementation(
      async (request?: { filter?: string }) => {
        if (request?.filter === 'env == "prod"') {
          return [appState];
        }
        return states;
      },
    );
    const getAllEdges = vi.fn().mockResolvedValue(edges);
    const getLabelPolicy = vi.fn().mockResolvedValue({
      version: 1,
      policyJson: JSON.stringify({
        allowed_keys: { env: {} },
        allowed_values: { env: ['prod', 'staging'] },
        max_keys: 32,
        max_value_len: 256,
      }),
      createdAt: new Date('2024-02-01T00:00:00.000Z'),
      updatedAt: new Date('2024-02-01T00:00:00.000Z'),
    });

    const api = {
      listStates,
      getAllEdges,
      getLabelPolicy,
      getStateInfo: vi.fn().mockResolvedValue(null),
    } as unknown as GridApiAdapter;

    renderWithGrid(<App />, { api });

    await waitFor(() => expect(listStates).toHaveBeenCalledTimes(1));

    const prodButton = await screen.findByRole('button', { name: 'prod' });
    await userEvent.click(prodButton);

    const addFilter = screen.getByRole('button', { name: /Add Filter/i });
    await userEvent.click(addFilter);

    await waitFor(() => expect(listStates).toHaveBeenCalledTimes(2));
    expect(listStates).toHaveBeenLastCalledWith(expect.objectContaining({
      includeLabels: true,
      filter: 'env == "prod"',
    }));

    await waitFor(() => {
      expect(screen.getByText('app/prod')).toBeInTheDocument();
      expect(screen.queryByText('network/prod')).not.toBeInTheDocument();
    });
  });
});
