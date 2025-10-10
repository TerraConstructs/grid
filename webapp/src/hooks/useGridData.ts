import { useState, useCallback } from 'react';
import { useGrid } from '../context/GridContext';
import type { StateInfo, DependencyEdge } from '@tcons/grid';

interface UseGridDataReturn {
  states: StateInfo[];
  edges: DependencyEdge[];
  loading: boolean;
  error: string | null;
  filter: string;
  loadData: (options?: { filter?: string }) => Promise<void>;
  getStateInfo: (logicId: string) => Promise<StateInfo | null>;
}

/**
 * Hook for loading and managing Grid data (states and edges).
 *
 * Provides manual refresh functionality (no background polling).
 * Preserves selected state across refreshes.
 */
export function useGridData(): UseGridDataReturn {
  const { api } = useGrid();
  const [states, setStates] = useState<StateInfo[]>([]);
  const [edges, setEdges] = useState<DependencyEdge[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [filter, setFilter] = useState<string>('');

  const loadData = useCallback(async (options?: { filter?: string }) => {
    setLoading(true);
    setError(null);

    const requestedFilter = options?.filter;
    const nextFilter = requestedFilter !== undefined ? requestedFilter : filter;
    const trimmedFilter = nextFilter.trim();
    const filterOption = trimmedFilter === '' ? undefined : trimmedFilter;

    if (requestedFilter !== undefined) {
      setFilter(nextFilter);
    }

    try {
      const [statesData, edgesData] = await Promise.all([
        api.listStates({
          includeLabels: true,
          ...(filterOption ? { filter: filterOption } : {}),
        }),
        api.getAllEdges(),
      ]);

      const stateGuids = new Set(statesData.map((state) => state.guid));
      const relevantEdges = edgesData.filter(
        (edge) => stateGuids.has(edge.from_guid) && stateGuids.has(edge.to_guid),
      );

      setStates(statesData);
      setEdges(relevantEdges);
    } catch (err) {
      const errorMessage = err instanceof Error
        ? err.message
        : 'Failed to load data from Grid API';

      setError(errorMessage);
      console.error('Failed to load Grid data:', err);
    } finally {
      setLoading(false);
    }
  }, [api, filter]);

  const getStateInfo = useCallback(async (logicId: string): Promise<StateInfo | null> => {
    try {
      return await api.getStateInfo(logicId);
    } catch (err) {
      console.error(`Failed to get state info for ${logicId}:`, err);
      return null;
    }
  }, [api]);

  return {
    states,
    edges,
    loading,
    error,
    filter,
    loadData,
    getStateInfo,
  };
}
