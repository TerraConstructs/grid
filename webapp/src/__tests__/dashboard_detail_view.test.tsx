import { describe, it, expect, vi } from "vitest";
import { waitFor, screen } from '@testing-library/react';
import '@testing-library/jest-dom/vitest';
import userEvent from '@testing-library/user-event';
import App from '../App';
import { renderWithGrid } from '../../test/gridTestUtils';
import type { GridApiAdapter, StateInfo, DependencyEdge } from '@tcons/grid';

describe('Dashboard detail view', () => {
  it('displays state detail with dependencies, outputs, and supports navigation', async () => {
    const dependencyEdge: DependencyEdge = {
      id: 7,
      from_guid: 'network-guid',
      from_logic_id: 'network/prod',
      from_output: 'vpc_id',
      to_guid: 'app-guid',
      to_logic_id: 'app/prod',
      to_input_name: 'network_vpc_id',
      status: 'clean',
      created_at: '2024-02-01T00:00:00.000Z',
      updated_at: '2024-02-01T00:05:00.000Z',
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
      dependents: [{ ...dependencyEdge }],
      outputs: [{ key: 'vpc_id', sensitive: false }],
      computed_status: 'clean',
      size_bytes: 4096,
    };

    const appState: StateInfo = {
      guid: 'app-guid',
      logic_id: 'app/prod',
      created_at: '2024-02-01T00:00:00.000Z',
      updated_at: '2024-02-01T00:15:00.000Z',
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

    const listStates = vi.fn().mockResolvedValue(states);
    const getAllEdges = vi.fn().mockResolvedValue(edges);
    const getStateInfo = vi.fn().mockImplementation(async (logicId: string) => {
      return states.find((state) => state.logic_id === logicId) ?? null;
    });
    const getLabelPolicy = vi.fn().mockResolvedValue({
      version: 1,
      policyJson: JSON.stringify({
        allowed_keys: { env: {}, team: {} },
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
      getStateInfo,
      getLabelPolicy,
    } as unknown as GridApiAdapter;

    renderWithGrid(<App />, { api });

    const listButton = await screen.findByRole('button', { name: /^List$/ });
    await userEvent.click(listButton);

    const appRow = await screen.findAllByText('app/prod');
    await userEvent.click(appRow[0]);

    await waitFor(() => {
      expect(screen.getByRole('heading', { name: 'app/prod' })).toBeInTheDocument();
    });

    expect(screen.getByText('service_url')).toBeInTheDocument();
    expect(screen.getByText('db_password')).toBeInTheDocument();
    expect(screen.getAllByText('sensitive').length).toBeGreaterThan(0);

    await userEvent.click(screen.getByRole('button', { name: /Dependencies/ }));

    expect(screen.getByRole('button', { name: 'network/prod' })).toBeInTheDocument();

    await userEvent.click(screen.getByRole('button', { name: 'network/prod' }));

    await waitFor(() => {
      expect(getStateInfo).toHaveBeenCalledWith('network/prod');
      expect(screen.getByRole('heading', { name: 'network/prod' })).toBeInTheDocument();
    });
  });
});
