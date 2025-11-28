import type { StateInfo } from '@tcons/grid';
import { X, Database, ArrowRight, ArrowLeft, Lock, ExternalLink, Tag, Package } from 'lucide-react';
import { useState } from 'react';
import { LabelList } from './LabelList';
import { OutputCard } from './OutputCard';

interface DetailViewProps {
  state: StateInfo;
  onClose: () => void;
  onNavigate: (logicId: string) => void;
}

const getEdgeStatusColor = (status: string): string => {
  const colors: Record<string, string> = {
    clean: 'text-green-600 bg-green-50 border-green-200',
    'clean-invalid': 'text-red-600 bg-red-50 border-red-200',
    dirty: 'text-orange-600 bg-orange-50 border-orange-200',
    'dirty-invalid': 'text-red-600 bg-red-50 border-red-200',
    pending: 'text-blue-600 bg-blue-50 border-blue-200',
    'potentially-stale': 'text-yellow-600 bg-yellow-50 border-yellow-200',
    'schema-invalid': 'text-red-600 bg-red-50 border-red-200',
    'missing-output': 'text-red-700 bg-red-100 border-red-300',
    mock: 'text-purple-600 bg-purple-50 border-purple-200',
  };
  return colors[status] || 'text-gray-600 bg-gray-50 border-gray-200';
};

export function DetailView({ state, onClose, onNavigate }: DetailViewProps) {
  const [activeTab, setActiveTab] = useState<'overview' | 'outputs' | 'dependencies' | 'dependents' | 'labels'>('overview');

  return (
    <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50 p-4">
      <div className="bg-white rounded-lg shadow-2xl max-w-5xl w-full max-h-[90vh] overflow-hidden flex flex-col">
        <div className="bg-gradient-to-r from-purple-600 to-purple-700 px-4 py-3 flex items-center justify-between">
          <div className="flex items-center gap-2">
            <Database className="w-5 h-5 text-white" />
            <div>
              <h2 className="text-lg font-bold text-white">{state.logic_id}</h2>
              <p className="text-purple-100 text-xs font-mono">{state.guid}</p>
            </div>
          </div>
          <button
            onClick={onClose}
            className="text-white hover:bg-purple-800 rounded-lg p-1.5 transition-colors"
          >
            <X className="w-5 h-5" />
          </button>
        </div>

        <div className="border-b border-gray-200 bg-gray-50">
          <nav className="flex gap-1 px-4">
            {[
              { id: 'overview', label: 'Overview', icon: Database },
              { id: 'outputs', label: `Outputs (${state.outputs.length})`, icon: Package },
              { id: 'labels', label: `Labels (${Object.keys(state.labels || {}).length})`, icon: Tag },
              { id: 'dependencies', label: `Dependencies (${state.dependencies.length})`, icon: ArrowLeft },
              { id: 'dependents', label: `Dependents (${state.dependents.length})`, icon: ArrowRight },
            ].map((tab) => (
              <button
                key={tab.id}
                onClick={() => setActiveTab(tab.id as any)}
                className={`flex items-center gap-1.5 px-3 py-2 text-sm font-medium border-b-2 transition-colors ${
                  activeTab === tab.id
                    ? 'border-purple-600 text-purple-600'
                    : 'border-transparent text-gray-500 hover:text-gray-700 hover:border-gray-300'
                }`}
              >
                <tab.icon className="w-4 h-4" />
                {tab.label}
              </button>
            ))}
          </nav>
        </div>

        <div className="flex-1 overflow-auto p-4">
          {activeTab === 'overview' && (
            <div className="space-y-4">
              <div className="grid grid-cols-2 gap-4">
                <div>
                  <h3 className="text-xs font-medium text-gray-500 mb-1.5">Status</h3>
                  <div className="flex items-center gap-2">
                    <span className="inline-flex items-center px-3 py-1 rounded-full text-sm font-medium bg-purple-100 text-purple-800">
                      {state.computed_status || 'unknown'}
                    </span>
                    {state.locked && (
                      <span className="inline-flex items-center gap-1 px-3 py-1 rounded-full text-sm font-medium bg-red-100 text-red-800">
                        <Lock className="w-3 h-3" />
                        Locked
                      </span>
                    )}
                  </div>
                </div>

                <div>
                  <h3 className="text-sm font-medium text-gray-500 mb-2">Size</h3>
                  <p className="text-lg font-semibold text-gray-900">
                    {((state.size_bytes ?? 0) / 1024).toFixed(1)} KB
                  </p>
                </div>

                <div>
                  <h3 className="text-sm font-medium text-gray-500 mb-2">Created</h3>
                  <p className="text-sm text-gray-900">
                    {new Date(state.created_at).toLocaleString()}
                  </p>
                </div>

                <div>
                  <h3 className="text-sm font-medium text-gray-500 mb-2">Last Updated</h3>
                  <p className="text-sm text-gray-900">
                    {new Date(state.updated_at).toLocaleString()}
                  </p>
                </div>
              </div>

              <div>
                <h3 className="text-xs font-medium text-gray-500 mb-2">Backend Configuration</h3>
                <div className="bg-gray-50 rounded-lg p-3 space-y-2">
                  <div className="flex items-start gap-2">
                    <ExternalLink className="w-4 h-4 text-gray-400 mt-0.5" />
                    <div className="flex-1">
                      <p className="text-xs text-gray-500">Address</p>
                      <p className="text-sm font-mono text-gray-900 break-all">{state.backend_config.address}</p>
                    </div>
                  </div>
                  <div className="flex items-start gap-2">
                    <Lock className="w-4 h-4 text-gray-400 mt-0.5" />
                    <div className="flex-1">
                      <p className="text-xs text-gray-500">Lock Address</p>
                      <p className="text-sm font-mono text-gray-900 break-all">{state.backend_config.lock_address}</p>
                    </div>
                  </div>
                </div>
              </div>
            </div>
          )}

          {activeTab === 'outputs' && (
            <div className="space-y-3">
              {state.outputs.length === 0 ? (
                <p className="text-gray-500 text-center py-8">No outputs defined</p>
              ) : (
                state.outputs.map((output) => (
                  <OutputCard key={output.key} output={output} />
                ))
              )}
            </div>
          )}

          {activeTab === 'dependencies' && (
            <div className="space-y-2">
              {state.dependencies.length === 0 ? (
                <p className="text-gray-500 text-center py-8">No incoming dependencies</p>
              ) : (
                state.dependencies.map((edge) => (
                  <div
                    key={edge.id}
                    className={`border rounded-lg p-3 ${getEdgeStatusColor(edge.status)}`}
                  >
                    <div className="flex items-start justify-between mb-2">
                      <button
                        onClick={() => onNavigate(edge.from_logic_id)}
                        className="text-sm font-semibold hover:underline"
                      >
                        {edge.from_logic_id}
                      </button>
                      <span className="text-xs font-medium px-2 py-1 rounded bg-white bg-opacity-50">
                        {edge.status}
                      </span>
                    </div>
                    <div className="flex items-center gap-2 text-sm">
                      <code className="bg-white bg-opacity-50 px-2 py-1 rounded">
                        {edge.from_output}
                      </code>
                      <ArrowRight className="w-4 h-4" />
                      <code className="bg-white bg-opacity-50 px-2 py-1 rounded">
                        {edge.to_input_name || 'default'}
                      </code>
                    </div>
                    {edge.last_out_at && (
                      <p className="text-xs mt-2 opacity-75">
                        Last updated: {new Date(edge.last_out_at).toLocaleString()}
                      </p>
                    )}
                  </div>
                ))
              )}
            </div>
          )}

          {activeTab === 'dependents' && (
            <div className="space-y-2">
              {state.dependents.length === 0 ? (
                <p className="text-gray-500 text-center py-8">No outgoing dependencies</p>
              ) : (
                state.dependents.map((edge) => (
                  <div
                    key={edge.id}
                    className={`border rounded-lg p-3 ${getEdgeStatusColor(edge.status)}`}
                  >
                    <div className="flex items-start justify-between mb-2">
                      <button
                        onClick={() => onNavigate(edge.to_logic_id)}
                        className="text-sm font-semibold hover:underline"
                      >
                        {edge.to_logic_id}
                      </button>
                      <span className="text-xs font-medium px-2 py-1 rounded bg-white bg-opacity-50">
                        {edge.status}
                      </span>
                    </div>
                    <div className="flex items-center gap-2 text-sm">
                      <code className="bg-white bg-opacity-50 px-2 py-1 rounded">
                        {edge.from_output}
                      </code>
                      <ArrowRight className="w-4 h-4" />
                      <code className="bg-white bg-opacity-50 px-2 py-1 rounded">
                        {edge.to_input_name || 'default'}
                      </code>
                    </div>
                    {edge.last_in_at && (
                      <p className="text-xs mt-2 opacity-75">
                        Last consumed: {new Date(edge.last_in_at).toLocaleString()}
                      </p>
                    )}
                  </div>
                ))
              )}
            </div>
          )}

          {activeTab === 'labels' && (
            <div>
              <LabelList labels={state.labels} />
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
