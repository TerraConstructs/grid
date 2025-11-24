import { useEffect, useMemo, useRef, useState } from 'react';
import { PlusCircle, X, Filter as FilterIcon } from 'lucide-react';
import {
  buildEqualityFilter,
  buildInFilter,
  combineFilters,
  type LabelScalar,
} from '@tcons/grid';
import { useLabelPolicy } from '../hooks/useLabelPolicy';

export interface ActiveLabelFilter {
  id: string;
  key: string;
  values: LabelScalar[];
}

interface LabelFilterProps {
  onFilterChange: (expression: string, filters: ActiveLabelFilter[]) => void;
  initialFilters?: ActiveLabelFilter[];
}

const parseScalar = (raw: string): LabelScalar => {
  const trimmed = raw.trim();
  if (trimmed === '') {
    return '';
  }
  if (trimmed === 'true') {
    return true;
  }
  if (trimmed === 'false') {
    return false;
  }

  const numeric = Number(trimmed);
  if (!Number.isNaN(numeric) && trimmed === numeric.toString()) {
    return numeric;
  }

  return trimmed;
};

const generateId = (): string => {
  if (typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function') {
    return crypto.randomUUID();
  }
  return `filter-${Math.random().toString(36).slice(2, 10)}`;
};

const buildExpression = (filters: ActiveLabelFilter[]): string => {
  if (filters.length === 0) {
    return '';
  }

  const expressions = filters
    .map((filter) => {
      if (filter.values.length === 0) {
        return '';
      }
      if (filter.values.length === 1) {
        return buildEqualityFilter(filter.key, filter.values[0]);
      }
      return buildInFilter(filter.key, filter.values);
    })
    .filter(Boolean);

  return combineFilters(expressions, 'AND');
};

export function LabelFilter({ onFilterChange, initialFilters }: LabelFilterProps) {
  const onFilterChangeRef = useRef(onFilterChange);
  const { getAllowedKeys, getAllowedValues, loading } = useLabelPolicy();
  const [activeFilters, setActiveFilters] = useState<ActiveLabelFilter[]>(initialFilters ?? []);
  const [selectedKey, setSelectedKey] = useState<string>('');
  const [customKey, setCustomKey] = useState<string>('');
  const [freeformValue, setFreeformValue] = useState<string>('');
  const [enumSelections, setEnumSelections] = useState<string[]>([]);

  useEffect(() => {
    onFilterChangeRef.current = onFilterChange;
  }, [onFilterChange]);

  useEffect(() => {
    if (initialFilters) {
      setActiveFilters(initialFilters);
    }
  }, [initialFilters]);

  const allowedKeys = useMemo(() => getAllowedKeys(), [getAllowedKeys]);

  useEffect(() => {
    if (allowedKeys.length > 0 && !allowedKeys.includes(selectedKey)) {
      setSelectedKey(allowedKeys[0]);
    }
  }, [allowedKeys, selectedKey]);

  const effectiveKey = allowedKeys.length > 0 ? selectedKey : customKey.trim();
  const allowedValues = effectiveKey ? getAllowedValues(effectiveKey) : null;

  useEffect(() => {
    const expression = buildExpression(activeFilters);
    onFilterChangeRef.current(expression, activeFilters);
  }, [activeFilters]);

  const handleEnumSelection = (value: string) => {
    setEnumSelections((current) => {
      if (current.includes(value)) {
        return current.filter((item) => item !== value);
      }
      return [...current, value];
    });
  };

  const resetInputs = () => {
    if (allowedKeys.length === 0) {
      setCustomKey('');
    }
    setFreeformValue('');
    setEnumSelections([]);
  };

  const addFilter = () => {
    if (!effectiveKey) {
      return;
    }

    const values: LabelScalar[] = [];

    if (allowedValues && allowedValues.length > 0) {
      if (enumSelections.length === 0) {
        return;
      }
      enumSelections.forEach((value) => {
        values.push(parseScalar(value));
      });
    } else {
      const trimmedValue = freeformValue.trim();
      if (trimmedValue === '') {
        return;
      }
      values.push(parseScalar(trimmedValue));
    }

    if (values.length === 0) {
      return;
    }

    setActiveFilters((current) => [
      ...current,
      {
        id: generateId(),
        key: effectiveKey,
        values,
      },
    ]);

    resetInputs();
  };

  const removeFilter = (id: string) => {
    setActiveFilters((current) => current.filter((filter) => filter.id !== id));
  };

  const clearAll = () => {
    setActiveFilters([]);
    resetInputs();
  };

  return (
    <div className="bg-white border border-gray-200 rounded-lg shadow-sm p-4 space-y-4">
      <div className="flex items-start gap-4 flex-wrap">
        <div className="flex flex-col gap-2">
          <label className="text-xs font-semibold text-gray-500 uppercase tracking-wide">
            Label Key
          </label>
          {allowedKeys.length > 0 ? (
            <select
              value={selectedKey}
              onChange={(event) => setSelectedKey(event.target.value)}
              disabled={loading || allowedKeys.length === 0}
              className="min-w-[160px] rounded-md border border-gray-300 bg-white px-3 py-2 text-sm text-gray-800 shadow-sm focus:border-purple-500 focus:outline-none focus:ring-2 focus:ring-purple-500/50 disabled:opacity-60"
            >
              {allowedKeys.map((key) => (
                <option key={key} value={key}>
                  {key}
                </option>
              ))}
            </select>
          ) : (
            <input
              type="text"
              value={customKey}
              onChange={(event) => setCustomKey(event.target.value)}
              placeholder="Enter label key"
              disabled={loading}
              className="min-w-[160px] rounded-md border border-gray-300 bg-white px-3 py-2 text-sm text-gray-800 shadow-sm focus:border-purple-500 focus:outline-none focus:ring-2 focus:ring-purple-500/50 disabled:opacity-60"
            />
          )}
        </div>

        <div className="flex-1 flex flex-col gap-2 min-w-[220px]">
          <label className="text-xs font-semibold text-gray-500 uppercase tracking-wide">
            Value
          </label>
          {allowedValues && allowedValues.length > 0 ? (
            <div className="flex flex-wrap gap-2">
              {allowedValues.map((value) => (
                <button
                  key={value}
                  type="button"
                  onClick={() => handleEnumSelection(value)}
                  className={`px-3 py-1.5 text-sm rounded-md border ${
                    enumSelections.includes(value)
                      ? 'border-purple-500 bg-purple-50 text-purple-600'
                      : 'border-gray-300 bg-white text-gray-700 hover:border-purple-400 hover:text-purple-600'
                  } transition-colors`}
                >
                  {value}
                </button>
              ))}
              {enumSelections.length > 0 && (
                <button
                  type="button"
                  onClick={() => setEnumSelections([])}
                  className="text-xs text-gray-500 underline"
                >
                  Clear
                </button>
              )}
            </div>
          ) : (
            <input
              type="text"
              value={freeformValue}
              onChange={(event) => setFreeformValue(event.target.value)}
              placeholder="Enter value (string, number, or boolean)"
              disabled={loading || !effectiveKey}
              className="rounded-md border border-gray-300 bg-white px-3 py-2 text-sm text-gray-800 shadow-sm focus:border-purple-500 focus:outline-none focus:ring-2 focus:ring-purple-500/50 disabled:opacity-60"
            />
          )}
        </div>

        <div className="flex items-end gap-2">
          <button
            type="button"
            onClick={addFilter}
            disabled={loading || !effectiveKey}
            className="inline-flex items-center gap-2 rounded-md bg-purple-600 px-4 py-2 text-sm font-medium text-white shadow-sm hover:bg-purple-700 focus:outline-none focus:ring-2 focus:ring-purple-500 focus:ring-offset-2 disabled:opacity-50 disabled:cursor-not-allowed"
          >
            <PlusCircle className="w-4 h-4" />
            Add Filter
          </button>
          {activeFilters.length > 0 && (
            <button
              type="button"
              onClick={clearAll}
              className="inline-flex items-center gap-2 rounded-md border border-gray-300 px-3 py-2 text-sm font-medium text-gray-700 hover:bg-gray-50"
            >
              <FilterIcon className="w-4 h-4" />
              Clear All
            </button>
          )}
        </div>
      </div>

      {activeFilters.length > 0 ? (
        <div className="flex flex-wrap gap-2">
          {activeFilters.map((filter) => {
            const label =
              filter.values.length === 1
                ? `${filter.key} = ${String(filter.values[0])}`
                : `${filter.key} in [${filter.values.map((value) => String(value)).join(', ')}]`;

            return (
              <span
                key={filter.id}
                className="inline-flex items-center gap-2 rounded-full bg-purple-50 px-3 py-1 text-sm text-purple-700 border border-purple-200"
              >
                {label}
                <button
                  type="button"
                  onClick={() => removeFilter(filter.id)}
                  className="text-purple-500 hover:text-purple-700"
                  aria-label={`Remove filter ${label}`}
                >
                  <X className="w-4 h-4" />
                </button>
              </span>
            );
          })}
        </div>
      ) : (
        <p className="text-sm text-gray-500 flex items-center gap-2">
          <FilterIcon className="w-4 h-4 text-gray-400" />
          No filters applied. Add filters to refine states by labels.
        </p>
      )}
    </div>
  );
}
