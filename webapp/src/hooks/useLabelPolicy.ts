import { useState, useEffect, useCallback } from 'react';
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

  const loadPolicy = useCallback(async () => {
    setLoading(true);
    setError(null);

    try {
      const response = await api.getLabelPolicy();

      // Parse the JSON policy
      const parsed: PolicyDefinition = JSON.parse(response.policyJson);

      setPolicy(parsed);
      setVersion(response.version);
    } catch (err) {
      // Policy may not exist yet - this is not necessarily an error
      if (err instanceof Error && err.message.includes('not found')) {
        // Set default policy when none exists
        setPolicy({
          max_keys: 32,
          max_value_len: 256,
        });
        setVersion(0);
      } else {
        const errorMessage = err instanceof Error
          ? err.message
          : 'Failed to load label policy';
        setError(errorMessage);
        console.error('Failed to load label policy:', err);
      }
    } finally {
      setLoading(false);
    }
  }, [api]);

  // Load policy on mount
  useEffect(() => {
    loadPolicy();
  }, [loadPolicy]);

  const getAllowedValues = (key: string): string[] | null => {
    if (!policy?.allowed_values) {
      return null;
    }
    return policy.allowed_values[key] || null;
  };

  const getAllowedKeys = (): string[] => {
    if (!policy?.allowed_keys) {
      return [];
    }
    return Object.keys(policy.allowed_keys);
  };

  return {
    policy,
    version,
    loading,
    error,
    getAllowedValues,
    getAllowedKeys,
    refresh: loadPolicy,
  };
}
