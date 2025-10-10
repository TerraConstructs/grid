import { Tag } from 'lucide-react';

export interface LabelListProps {
  labels?: Record<string, string | number | boolean>;
  className?: string;
}

/**
 * LabelList displays state labels as sorted key=value pairs.
 * Per FR-007: Labels are displayed in alphabetical order by key.
 * Per FR-043: Shows empty state when no labels present.
 */
export function LabelList({ labels, className = '' }: LabelListProps) {
  // Handle empty labels per FR-043
  if (!labels || Object.keys(labels).length === 0) {
    return (
      <div className={`flex flex-col items-center justify-center py-12 px-4 text-gray-400 ${className}`}>
        <Tag className="w-12 h-12 mb-3 opacity-50" />
        <p className="text-sm font-medium">No labels</p>
        <p className="text-xs mt-1">This state has no labels defined</p>
      </div>
    );
  }

  // Sort labels alphabetically by key per FR-007
  const sortedEntries = Object.entries(labels).sort(([keyA], [keyB]) =>
    keyA.localeCompare(keyB)
  );

  return (
    <div className={`${className}`}>
      <div className="grid grid-cols-1 gap-2">
        {sortedEntries.map(([key, value]) => (
          <div
            key={key}
            className="flex items-start gap-3 p-3 rounded-lg border border-gray-200 bg-gray-50 hover:bg-gray-100 transition-colors"
          >
            <Tag className="w-4 h-4 text-purple-600 mt-0.5 flex-shrink-0" />
            <div className="flex-1 min-w-0">
              <div className="flex items-baseline gap-2 flex-wrap">
                <span className="font-mono text-sm font-medium text-gray-900">
                  {key}
                </span>
                <span className="text-gray-400">=</span>
                <span className="font-mono text-sm text-gray-700 break-all">
                  {formatLabelValue(value)}
                </span>
              </div>
              <div className="text-xs text-gray-500 mt-1">
                {getValueType(value)}
              </div>
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}

/**
 * Format label value for display based on type
 */
function formatLabelValue(value: string | number | boolean): string {
  if (typeof value === 'boolean') {
    return value ? 'true' : 'false';
  }
  if (typeof value === 'number') {
    return value.toString();
  }
  return value;
}

/**
 * Get human-readable type for label value
 */
function getValueType(value: string | number | boolean): string {
  if (typeof value === 'boolean') {
    return 'boolean';
  }
  if (typeof value === 'number') {
    return 'number';
  }
  return 'string';
}
