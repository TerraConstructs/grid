import { describe, it, expect } from 'vitest';
import {
  buildEqualityFilter,
  buildInFilter,
  combineFilters,
} from './bexpr.js';

describe('bexpr utilities', () => {
  it('builds equality filter with string value', () => {
    expect(buildEqualityFilter('env', 'prod')).toBe('env == "prod"');
  });

  it('builds equality filter with numeric and boolean values', () => {
    expect(buildEqualityFilter('cost', 42)).toBe('cost == 42');
    expect(buildEqualityFilter('active', true)).toBe('active == true');
  });

  it('builds IN filter with string values', () => {
    expect(buildInFilter('env', ['staging', 'prod'])).toBe(
      '((env == "staging") or (env == "prod"))'
    );
  });

  it('builds IN filter with single value', () => {
    expect(buildInFilter('env', ['prod'])).toBe('env == "prod"');
  });

  it('combines filters with AND and OR', () => {
    const filters = [
      buildEqualityFilter('env', 'prod'),
      buildEqualityFilter('active', true),
    ];
    expect(combineFilters(filters)).toBe('(env == "prod") and (active == true)');
    expect(combineFilters(filters, 'OR')).toBe('(env == "prod") or (active == true)');
  });

  it('omits empty filters when combining', () => {
    expect(combineFilters(['', buildEqualityFilter('env', 'prod')])).toBe(
      'env == "prod"'
    );
  });

  it('correctly handles precedence when combining IN filter with AND', () => {
    const filters = [
      buildInFilter('env', ['prd', 'stg']),
      buildEqualityFilter('team', 'TIES'),
    ];
    // Should be: ((env == "prd") or (env == "stg")) and (team == "TIES")
    // NOT: (env == "prd") or (env == "stg") and team == "TIES" (wrong precedence)
    expect(combineFilters(filters, 'AND')).toBe(
      '((env == "prd") or (env == "stg")) and (team == "TIES")'
    );
  });
});
