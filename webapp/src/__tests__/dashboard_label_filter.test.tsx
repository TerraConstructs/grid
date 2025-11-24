import { describe, it, expect, vi, afterEach } from 'vitest';
import { screen, fireEvent } from '@testing-library/react';
import { LabelFilter } from '../components/LabelFilter';
import type { ActiveLabelFilter } from '../components/LabelFilter';
import { renderWithGrid } from '../../test/gridTestUtils';
import type { GridApiAdapter } from '@tcons/grid';
import { __resetLabelPolicyCacheForTests } from '../hooks/useLabelPolicy';

afterEach(() => {
  __resetLabelPolicyCacheForTests();
});

describe('LabelFilter component', () => {
  it('builds bexpr filters using enum selections from policy', async () => {
    const onFilterChange = vi.fn((_, filters: ActiveLabelFilter[]) => filters);

    // Create mock API that returns a policy with allowed keys and values
    const policyJson = JSON.stringify({
      allowed_keys: {
        env: {},
        team: {},
      },
      allowed_values: {
        env: ['prod', 'staging', 'dev'],
        team: ['core', 'platform'],
      },
      max_keys: 32,
      max_value_len: 256,
    });

    const mockApi = {
      getLabelPolicy: vi.fn().mockResolvedValue({
        version: 1,
        policyJson,
        createdAt: new Date(),
        updatedAt: new Date(),
      }),
      listStates: vi.fn(),
      getAllEdges: vi.fn(),
      getStateInfo: vi.fn(),
    } as unknown as GridApiAdapter;

    renderWithGrid(<LabelFilter onFilterChange={onFilterChange} />, { api: mockApi });

    // Wait for policy to load and enum buttons to appear
    const prodButton = await screen.findByRole('button', { name: 'prod' });
    const stagingButton = await screen.findByRole('button', { name: 'staging' });
    const addFilter = await screen.findByRole('button', { name: /add filter/i });

    // Select multiple enum values to trigger IN expression
    fireEvent.click(prodButton);
    fireEvent.click(stagingButton);
    fireEvent.click(addFilter);

    const lastCall = onFilterChange.mock.calls.at(-1);
    expect(lastCall?.[0]).toMatch(/env/i);
    const filters = lastCall?.[1] as ActiveLabelFilter[];
    expect(filters).toHaveLength(1);
    expect(filters[0]).toMatchObject({
      key: 'env',
      values: ['prod', 'staging'],
    });
  });

  it('falls back to free-text inputs when no policy keys are defined', async () => {
    const onFilterChange = vi.fn();

    // Create mock API that returns null (no policy)
    const mockApi = {
      getLabelPolicy: vi.fn().mockResolvedValue(null),
      listStates: vi.fn(),
      getAllEdges: vi.fn(),
      getStateInfo: vi.fn(),
    } as unknown as GridApiAdapter;

    renderWithGrid(<LabelFilter onFilterChange={onFilterChange} />, { api: mockApi });

    // Wait for policy to load (or fail to load) and freeform inputs to appear
    const keyInput = await screen.findByPlaceholderText(/enter label key/i);
    const valueInput = await screen.findByPlaceholderText(/enter value/i);
    const addButton = await screen.findByRole('button', { name: /add filter/i });

    fireEvent.change(keyInput, { target: { value: 'team' } });
    fireEvent.change(valueInput, { target: { value: 'platform' } });
    fireEvent.click(addButton);

    const lastCall = onFilterChange.mock.calls.at(-1);
    expect(lastCall?.[0]).toBe('team == "platform"');
  });
});
