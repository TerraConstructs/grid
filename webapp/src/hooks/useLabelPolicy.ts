import { useState, useEffect, useCallback } from 'react';
import type { GridApiAdapter } from '@tcons/grid';
import { useGrid } from '../context/GridContext';

/**
 * PolicyDefinition represents the parsed label validation policy.
 * Per FR-029: Defines validation rules for state labels.
 */
export interface PolicyDefinition {
  allowed_keys?: Record<string, unknown>;
  allowed_values?: Record<string, string[]>;
  reserved_prefixes?: string[];
  max_keys: number;
  max_value_len: number;
}

const DEFAULT_POLICY: PolicyDefinition = {
  max_keys: 32,
  max_value_len: 256,
};

interface CachedPolicyState {
  policy: PolicyDefinition;
  version: number;
}

let cachedPolicyState: CachedPolicyState | null = null;
let policyRequest: Promise<CachedPolicyState> | null = null;

// Test-only helper to clear module-level caches
export function __resetLabelPolicyCacheForTests() {
  cachedPolicyState = null;
  policyRequest = null;
}

async function fetchPolicy(api: GridApiAdapter): Promise<CachedPolicyState> {
  const response = await api.getLabelPolicy();
  if (!response) {
    return { policy: DEFAULT_POLICY, version: 0 };
  }

  const parsed: PolicyDefinition = JSON.parse(response.policyJson);
  return {
    policy: parsed,
    version: response.version,
  };
}

/**
 * LabelPolicy represents the versioned policy from the API.
 */
export interface LabelPolicy {
  version: number;
  policyJson: string;
  createdAt: Date;
  updatedAt: Date;
}

interface UseLabelPolicyReturn {
  policy: PolicyDefinition | null;
  version: number | null;
  loading: boolean;
  error: string | null;
  /**
   * Get allowed values for a specific label key (for dropdown enums).
   * Per FR-044: Extract enum values from policy for UI dropdowns.
   */
  getAllowedValues: (key: string) => string[] | null;
  /**
   * Get all allowed keys from the policy.
   */
  getAllowedKeys: () => string[];
  /**
   * Refresh the policy from the server.
   */
  refresh: () => Promise<void>;
}

/**
 * Hook for fetching and parsing the label validation policy.
 * Per T059: Fetches policy on mount, parses policyJson for allowed_values extraction.
 * Per FR-044: Enables dropdown enum extraction for filter UI.
 */
export function useLabelPolicy(): UseLabelPolicyReturn {
  const { api } = useGrid();
  const [policy, setPolicy] = useState<PolicyDefinition | null>(null);
  const [version, setVersion] = useState<number | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const loadPolicy = useCallback(async (options?: { force?: boolean }) => {
    setLoading(true);
    setError(null);

    const force = options?.force === true;
    let request: Promise<CachedPolicyState> | null = null;

    try {
      if (!force && cachedPolicyState) {
        setPolicy(cachedPolicyState.policy);
        setVersion(cachedPolicyState.version);
        return;
      }

      if (!force && policyRequest) {
        const cached = await policyRequest;
        setPolicy(cached.policy);
        setVersion(cached.version);
        return;
      }

      if (force) {
        cachedPolicyState = null;
      }

      request = fetchPolicy(api);
      policyRequest = request;
      const result = await request;
      cachedPolicyState = result;
      setPolicy(result.policy);
      setVersion(result.version);
    } catch (err) {
      const errorMessage = err instanceof Error
        ? err.message
        : 'Failed to load label policy';
      setError(errorMessage);
      console.error('Failed to load label policy:', err);
    } finally {
      if (request && policyRequest === request) {
        policyRequest = null;
      }
      setLoading(false);
    }
  }, [api]);

  // Load policy on mount
  useEffect(() => {
    loadPolicy();
  }, [loadPolicy]);

  const getAllowedValues = useCallback((key: string): string[] | null => {
    if (!policy?.allowed_values) {
      return null;
    }
    return policy.allowed_values[key] || null;
  }, [policy]);

  const getAllowedKeys = useCallback((): string[] => {
    if (!policy?.allowed_keys) {
      return [];
    }
    return Object.keys(policy.allowed_keys);
  }, [policy]);

  return {
    policy,
    version,
    loading,
    error,
    getAllowedValues,
    getAllowedKeys,
    refresh: () => loadPolicy({ force: true }),
  };
}
