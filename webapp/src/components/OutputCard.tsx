import { FileJson, CheckCircle2, AlertTriangle, XCircle, ChevronDown, ChevronUp } from 'lucide-react';
import { useState, useMemo } from 'react';
import type { OutputKey } from '@tcons/grid';

interface OutputCardProps {
  output: OutputKey;
}

export function OutputCard({ output }: OutputCardProps) {
  const [isSchemaExpanded, setIsSchemaExpanded] = useState(false);

  const hasSchema = !!output.schema_json;
  const validationStatus = output.validation_status;

  // Determine card styling based on validation status
  const getBorderColor = () => {
    if (!hasSchema) return 'border-gray-200';

    switch (validationStatus) {
      case 'valid':
        return 'border-green-200 bg-green-50/30';
      case 'invalid':
        return 'border-orange-200 bg-orange-50/30';
      case 'error':
        return 'border-red-200 bg-red-50/30';
      default:
        return 'border-gray-200';
    }
  };

  const getValidationIcon = () => {
    switch (validationStatus) {
      case 'valid':
        return <CheckCircle2 className="w-4 h-4 text-green-600" />;
      case 'invalid':
        return <AlertTriangle className="w-4 h-4 text-orange-600" />;
      case 'error':
        return <XCircle className="w-4 h-4 text-red-600" />;
      default:
        return null;
    }
  };

  const getValidationMessage = () => {
    if (!hasSchema) return null;

    switch (validationStatus) {
      case 'valid':
        return (
          <div className="flex items-center gap-2 text-sm text-green-700">
            <CheckCircle2 className="w-4 h-4" />
            <span>Schema validation passed</span>
          </div>
        );
      case 'invalid':
        return (
          <div className="space-y-1">
            <div className="flex items-center gap-2 text-sm text-orange-700 font-medium">
              <AlertTriangle className="w-4 h-4" />
              <span>Schema validation failed</span>
            </div>
            {output.validation_error && (
              <div className="ml-6 text-xs text-orange-600 bg-white rounded px-2 py-1 font-mono">
                {output.validation_error}
              </div>
            )}
          </div>
        );
      case 'error':
        return (
          <div className="space-y-1">
            <div className="flex items-center gap-2 text-sm text-red-700 font-medium">
              <XCircle className="w-4 h-4" />
              <span>Validation error</span>
            </div>
            {output.validation_error && (
              <div className="ml-6 text-xs text-red-600 bg-white rounded px-2 py-1 font-mono">
                {output.validation_error}
              </div>
            )}
          </div>
        );
      default:
        return null;
    }
  };

  const formatRelativeTime = (isoDate: string) => {
    const date = new Date(isoDate);
    const now = new Date();
    const diffMs = now.getTime() - date.getTime();
    const diffMins = Math.floor(diffMs / 60000);

    if (diffMins < 1) return 'just now';
    if (diffMins < 60) return `${diffMins} minute${diffMins > 1 ? 's' : ''} ago`;

    const diffHours = Math.floor(diffMins / 60);
    if (diffHours < 24) return `${diffHours} hour${diffHours > 1 ? 's' : ''} ago`;

    const diffDays = Math.floor(diffHours / 24);
    return `${diffDays} day${diffDays > 1 ? 's' : ''} ago`;
  };

  const parseSchemaPreview = (schemaJson: string): string => {
    try {
      const schema = JSON.parse(schemaJson);

      // Build human-readable preview
      const parts: string[] = [];

      if (schema.type) {
        parts.push(`type: ${schema.type}`);
      }

      if (schema.pattern) {
        parts.push(`pattern: ${schema.pattern}`);
      }

      if (schema.minLength !== undefined || schema.maxLength !== undefined) {
        parts.push(`length: ${schema.minLength || 0}-${schema.maxLength || '∞'}`);
      }

      if (schema.enum) {
        parts.push(`enum: [${schema.enum.slice(0, 3).join(', ')}${schema.enum.length > 3 ? '...' : ''}]`);
      }

      if (schema.items?.type) {
        parts.push(`items: ${schema.items.type}`);
      }

      return parts.join(', ') || 'See JSON Schema below';
    } catch {
      return 'Invalid JSON Schema';
    }
  };

  const parsedSchema = useMemo(() => {
    if (!output.schema_json) return null;
    try {
      return JSON.parse(output.schema_json);
    } catch {
      return null;
    }
  }, [output.schema_json]);

  return (
    <div
      className={`border rounded-lg transition-colors ${getBorderColor()}`}
      role="region"
      aria-label={`Output ${output.key}${output.sensitive ? ' (sensitive)' : ''}${
        validationStatus === 'invalid' ? ' - Schema validation failed' : ''
      }`}
    >
      <div className="p-3 space-y-2">
        {/* Header */}
        <div className="flex items-start justify-between">
          <div className="flex items-center gap-2">
            <code className="text-base font-semibold text-purple-700">
              {output.key}
            </code>
            {output.sensitive && (
              <span className="inline-flex items-center text-xs bg-red-100 text-red-700 px-2 py-0.5 rounded font-medium">
                sensitive
              </span>
            )}
          </div>
          {getValidationIcon()}
        </div>

        {/* Schema Preview */}
        {hasSchema && (
          <div className="space-y-2">
            <div className="flex items-center gap-2 text-xs text-gray-600">
              <FileJson className="w-3.5 h-3.5 text-purple-500" />
              <span className="font-mono">{parseSchemaPreview(output.schema_json!)}</span>
            </div>

            {/* Validation Status */}
            {getValidationMessage()}

            {/* Validated Timestamp */}
            {output.validated_at && (
              <div className="text-xs text-gray-500">
                Validated {formatRelativeTime(output.validated_at)}
              </div>
            )}

            {/* Expandable Schema Viewer */}
            <button
              onClick={() => setIsSchemaExpanded(!isSchemaExpanded)}
              className="flex items-center gap-1.5 text-sm text-purple-600 hover:text-purple-700 font-medium transition-colors"
            >
              {isSchemaExpanded ? (
                <>
                  <ChevronUp className="w-4 h-4" />
                  Hide Schema
                </>
              ) : (
                <>
                  <ChevronDown className="w-4 h-4" />
                  View Schema
                </>
              )}
            </button>

            {/* Expanded Schema JSON */}
            {isSchemaExpanded && parsedSchema && (
              <div className="mt-2 bg-gray-900 rounded-md p-3 overflow-auto max-h-60">
                <pre className="text-xs text-green-400 font-mono">
                  {JSON.stringify(parsedSchema, null, 2)}
                </pre>
              </div>
            )}
          </div>
        )}

        {/* No Schema Message */}
        {!hasSchema && (
          <div className="text-sm text-gray-500 italic">
            No schema defined
          </div>
        )}
      </div>
    </div>
  );
}
