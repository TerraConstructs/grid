/**
 * Create State Page Component
 *
 * Form for creating new Terraform state resources.
 * Supports logic ID and label (key-value) inputs.
 */

import { useState } from 'react';
import { X, Plus, Trash2 } from 'lucide-react';

interface CreateStatePageProps {
  onClose: () => void;
  onSuccess: (message: string) => void;
  onError: (message: string) => void;
}

interface LabelEntry {
  key: string;
  value: string;
}

export function CreateStatePage({
  onClose,
  onSuccess,
  onError,
}: CreateStatePageProps) {
  const [logicId, setLogicId] = useState('');
  const [labels, setLabels] = useState<LabelEntry[]>([{ key: '', value: '' }]);
  const [submitting, setSubmitting] = useState(false);

  const handleAddLabel = () => {
    setLabels([...labels, { key: '', value: '' }]);
  };

  const handleRemoveLabel = (index: number) => {
    setLabels(labels.filter((_, i) => i !== index));
  };

  const handleLabelChange = (index: number, field: 'key' | 'value', value: string) => {
    const newLabels = [...labels];
    newLabels[index][field] = value;
    setLabels(newLabels);
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setSubmitting(true);

    try {
      // Generate UUIDv7 for guid
      const guid = crypto.randomUUID();

      // Convert labels array to map, filtering out empty entries
      const labelsMap: Record<string, string> = {};
      labels.forEach(({ key, value }) => {
        if (key.trim() && value.trim()) {
          labelsMap[key.trim()] = value.trim();
        }
      });

      const request = {
        guid,
        logicId,
        labels: labelsMap,
      };

      const response = await fetch('/state.v1.StateService/CreateState', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        credentials: 'include',
        body: JSON.stringify(request),
      });

      if (!response.ok) {
        const errorData = await response.json().catch(() => ({}));
        const errorMessage =
          errorData.message || errorData.error || `Failed to create state: ${response.statusText}`;

        // Check for permission denied
        if (response.status === 403 || errorMessage.toLowerCase().includes('permission')) {
          throw new Error(`Permission denied: ${errorMessage}`);
        }

        throw new Error(errorMessage);
      }

      const data = await response.json();
      onSuccess(`State "${data.logicId || logicId}" created successfully`);
      onClose();
    } catch (error) {
      const message = error instanceof Error ? error.message : 'Failed to create state';
      onError(message);
    } finally {
      setSubmitting(false);
    }
  };

  const canSubmit = logicId.trim().length > 0 && !submitting;

  return (
    <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50 p-4">
      <div className="bg-white rounded-lg shadow-xl max-w-2xl w-full max-h-[90vh] overflow-hidden flex flex-col">
        {/* Header */}
        <div className="flex items-center justify-between p-6 border-b border-gray-200">
          <h2 className="text-2xl font-bold text-gray-900">Create State</h2>
          <button
            onClick={onClose}
            className="text-gray-400 hover:text-gray-600 transition-colors"
          >
            <X className="w-6 h-6" />
          </button>
        </div>

        {/* Form */}
        <form onSubmit={handleSubmit} className="flex-1 overflow-y-auto p-6">
          <div className="space-y-6">
            {/* Logic ID */}
            <div>
              <label
                htmlFor="logic-id"
                className="block text-sm font-medium text-gray-700 mb-2"
              >
                Logic ID *
              </label>
              <input
                id="logic-id"
                type="text"
                value={logicId}
                onChange={(e) => setLogicId(e.target.value)}
                placeholder="my-terraform-state"
                className="w-full px-4 py-2 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-purple-600 focus:border-transparent"
                required
                disabled={submitting}
              />
              <p className="mt-1 text-sm text-gray-500">
                Human-readable identifier for this state
              </p>
            </div>

            {/* Labels */}
            <div>
              <div className="flex items-center justify-between mb-2">
                <label className="block text-sm font-medium text-gray-700">
                  Labels
                </label>
                <button
                  type="button"
                  onClick={handleAddLabel}
                  className="flex items-center gap-1 text-sm text-purple-600 hover:text-purple-700 font-medium"
                  disabled={submitting}
                >
                  <Plus className="w-4 h-4" />
                  Add Label
                </button>
              </div>

              <div className="space-y-2">
                {labels.map((label, index) => (
                  <div key={index} className="flex gap-2">
                    <input
                      type="text"
                      value={label.key}
                      onChange={(e) => handleLabelChange(index, 'key', e.target.value)}
                      placeholder="env"
                      aria-label={`Label key ${index + 1}`}
                      className="flex-1 px-3 py-2 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-purple-600 focus:border-transparent text-sm"
                      disabled={submitting}
                    />
                    <input
                      type="text"
                      value={label.value}
                      onChange={(e) => handleLabelChange(index, 'value', e.target.value)}
                      placeholder="dev"
                      aria-label={`Label value ${index + 1}`}
                      className="flex-1 px-3 py-2 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-purple-600 focus:border-transparent text-sm"
                      disabled={submitting}
                    />
                    {labels.length > 1 && (
                      <button
                        type="button"
                        onClick={() => handleRemoveLabel(index)}
                        className="px-2 text-red-600 hover:text-red-700 transition-colors"
                        disabled={submitting}
                      >
                        <Trash2 className="w-4 h-4" />
                      </button>
                    )}
                  </div>
                ))}
              </div>

              <p className="mt-1 text-sm text-gray-500">
                Key-value pairs for organizing and filtering states (e.g., env=dev)
              </p>
            </div>
          </div>
        </form>

        {/* Footer */}
        <div className="flex items-center justify-end gap-3 p-6 border-t border-gray-200">
          <button
            type="button"
            onClick={onClose}
            className="px-4 py-2 text-sm font-medium text-gray-700 bg-white border border-gray-300 rounded-lg hover:bg-gray-50 transition-colors"
            disabled={submitting}
          >
            Cancel
          </button>
          <button
            type="submit"
            onClick={handleSubmit}
            disabled={!canSubmit}
            className="px-4 py-2 text-sm font-medium text-white bg-gradient-to-r from-purple-600 to-purple-700 rounded-lg hover:from-purple-700 hover:to-purple-800 disabled:from-gray-400 disabled:to-gray-500 transition-all"
          >
            {submitting ? 'Creating...' : 'Create State'}
          </button>
        </div>
      </div>
    </div>
  );
}
