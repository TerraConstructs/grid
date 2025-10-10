import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import { LabelFilter } from '../components/LabelFilter';
import type { ActiveLabelFilter } from '../components/LabelFilter';

const mockUseLabelPolicy = vi.fn();

vi.mock('../hooks/useLabelPolicy', () => ({
  useLabelPolicy: () => mockUseLabelPolicy(),
}));

describe('LabelFilter component', () => {
  beforeEach(() => {
    mockUseLabelPolicy.mockReset();
  });

  it('builds bexpr filters using enum selections from policy', () => {
    const keys = ['env', 'team'];
    const allowedValues: Record<string, string[]> = {
      env: ['prod', 'staging', 'dev'],
      team: ['core', 'platform'],
    };

    mockUseLabelPolicy.mockReturnValue({
      policy: null,
      version: 1,
      loading: false,
      error: null,
      getAllowedKeys: () => keys,
      getAllowedValues: (key: string) => allowedValues[key] ?? null,
      refresh: vi.fn(),
    });

    const onFilterChange = vi.fn((_, filters: ActiveLabelFilter[]) => filters);

    render(<LabelFilter onFilterChange={onFilterChange} />);

    const prodButton = screen.getByRole('button', { name: 'prod' });
    const stagingButton = screen.getByRole('button', { name: 'staging' });
    const addFilter = screen.getByRole('button', { name: /add filter/i });

    // Select multiple enum values to trigger IN expression
    fireEvent.click(prodButton);
    fireEvent.click(stagingButton);
    fireEvent.click(addFilter);

    const lastCall = onFilterChange.mock.calls.at(-1);
    expect(lastCall?.[0]).toBe('env in ["prod","staging"]');
    const filters = lastCall?.[1] as ActiveLabelFilter[];
    expect(filters).toHaveLength(1);
    expect(filters[0]).toMatchObject({
      key: 'env',
      values: ['prod', 'staging'],
    });
  });

  it('falls back to free-text inputs when no policy keys are defined', () => {
    mockUseLabelPolicy.mockReturnValue({
      policy: null,
      version: 0,
      loading: false,
      error: null,
      getAllowedKeys: () => [],
      getAllowedValues: () => null,
      refresh: vi.fn(),
    });

    const onFilterChange = vi.fn();

    render(<LabelFilter onFilterChange={onFilterChange} />);

    const keyInput = screen.getByPlaceholderText(/enter label key/i);
    const valueInput = screen.getByPlaceholderText(/enter value/i);
    const addButton = screen.getByRole('button', { name: /add filter/i });

    fireEvent.change(keyInput, { target: { value: 'team' } });
    fireEvent.change(valueInput, { target: { value: 'platform' } });
    fireEvent.click(addButton);

    const lastCall = onFilterChange.mock.calls.at(-1);
    expect(lastCall?.[0]).toBe('team == "platform"');
  });
});
