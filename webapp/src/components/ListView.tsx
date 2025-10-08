import { StateInfo, DependencyEdge } from '../services/mockApi';
import { Database, GitBranch, ArrowRight, Lock, CheckCircle2, AlertCircle, Clock } from 'lucide-react';

interface ListViewProps {
  states: StateInfo[];
  edges: DependencyEdge[];
  onStateClick: (logicId: string) => void;
  onEdgeClick: (edge: DependencyEdge) => void;
}

const getStatusIcon = (status?: string) => {
  switch (status) {
    case 'clean':
      return <CheckCircle2 className="w-4 h-4 text-green-500" />;
    case 'stale':
      return <AlertCircle className="w-4 h-4 text-orange-500" />;
    case 'potentially-stale':
      return <Clock className="w-4 h-4 text-orange-400" />;
    default:
      return <AlertCircle className="w-4 h-4 text-gray-400" />;
  }
};

const getEdgeStatusBadge = (status: string) => {
  const styles: Record<string, string> = {
    clean: 'bg-green-100 text-green-800',
    dirty: 'bg-orange-100 text-orange-800',
    pending: 'bg-blue-100 text-blue-800',
    'potentially-stale': 'bg-yellow-100 text-yellow-800',
    mock: 'bg-purple-100 text-purple-800',
    'missing-output': 'bg-red-100 text-red-800',
  };

  return (
    <span className={`px-2 py-1 rounded text-xs font-medium ${styles[status] || 'bg-gray-100 text-gray-800'}`}>
      {status}
    </span>
  );
};

export function ListView({ states, edges, onStateClick, onEdgeClick }: ListViewProps) {
  return (
    <div className="w-full h-full overflow-auto bg-gray-50">
      <div className="max-w-7xl mx-auto p-4 space-y-4">
        <section>
          <div className="flex items-center gap-2 mb-3">
            <Database className="w-5 h-5 text-purple-600" />
            <h2 className="text-xl font-bold text-gray-900">States</h2>
            <span className="text-sm text-gray-500">({states.length})</span>
          </div>

          <div className="bg-white rounded-lg shadow overflow-hidden">
            <table className="min-w-full divide-y divide-gray-200">
              <thead className="bg-gray-50">
                <tr>
                  <th className="px-4 py-2 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                    Status
                  </th>
                  <th className="px-4 py-2 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                    Logic ID
                  </th>
                  <th className="px-4 py-2 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                    Dependencies
                  </th>
                  <th className="px-4 py-2 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                    Outputs
                  </th>
                  <th className="px-4 py-2 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                    Size
                  </th>
                  <th className="px-4 py-2 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                    Updated
                  </th>
                </tr>
              </thead>
              <tbody className="bg-white divide-y divide-gray-200">
                {states.map((state) => (
                  <tr
                    key={state.guid}
                    onClick={() => onStateClick(state.logic_id)}
                    className="hover:bg-purple-50 cursor-pointer transition-colors"
                  >
                    <td className="px-4 py-2 whitespace-nowrap">
                      <div className="flex items-center gap-2">
                        {getStatusIcon(state.computed_status)}
                        {state.locked && <Lock className="w-4 h-4 text-red-500" />}
                      </div>
                    </td>
                    <td className="px-4 py-2 whitespace-nowrap">
                      <div className="text-sm font-medium text-gray-900">{state.logic_id}</div>
                      <div className="text-xs text-gray-500 font-mono">{state.guid.slice(0, 16)}...</div>
                    </td>
                    <td className="px-4 py-2 whitespace-nowrap">
                      <div className="text-sm text-gray-600">
                        {state.dependencies.length} in / {state.dependents.length} out
                      </div>
                    </td>
                    <td className="px-4 py-2 whitespace-nowrap">
                      <div className="text-sm text-gray-900">{state.outputs.length} outputs</div>
                    </td>
                    <td className="px-4 py-2 whitespace-nowrap">
                      <div className="text-sm text-gray-600">
                        {(state.size_bytes / 1024).toFixed(1)} KB
                      </div>
                    </td>
                    <td className="px-4 py-2 whitespace-nowrap text-sm text-gray-500">
                      {new Date(state.updated_at).toLocaleDateString()}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </section>

        <section>
          <div className="flex items-center gap-2 mb-3">
            <GitBranch className="w-5 h-5 text-purple-600" />
            <h2 className="text-xl font-bold text-gray-900">Dependency Edges</h2>
            <span className="text-sm text-gray-500">({edges.length})</span>
          </div>

          <div className="bg-white rounded-lg shadow overflow-hidden">
            <table className="min-w-full divide-y divide-gray-200">
              <thead className="bg-gray-50">
                <tr>
                  <th className="px-4 py-2 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                    Status
                  </th>
                  <th className="px-4 py-2 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                    From State
                  </th>
                  <th className="px-4 py-2 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                    Output
                  </th>
                  <th className="px-4 py-2 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                    To State
                  </th>
                  <th className="px-4 py-2 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                    Input Name
                  </th>
                  <th className="px-4 py-2 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                    Updated
                  </th>
                </tr>
              </thead>
              <tbody className="bg-white divide-y divide-gray-200">
                {edges.map((edge) => (
                  <tr
                    key={edge.id}
                    onClick={() => onEdgeClick(edge)}
                    className="hover:bg-purple-50 cursor-pointer transition-colors"
                  >
                    <td className="px-4 py-2 whitespace-nowrap">
                      {getEdgeStatusBadge(edge.status)}
                    </td>
                    <td className="px-4 py-2 whitespace-nowrap">
                      <div className="text-sm font-medium text-gray-900">{edge.from_logic_id}</div>
                    </td>
                    <td className="px-4 py-2 whitespace-nowrap">
                      <div className="flex items-center gap-2">
                        <code className="text-xs bg-gray-100 px-2 py-1 rounded text-purple-600">
                          {edge.from_output}
                        </code>
                        <ArrowRight className="w-4 h-4 text-gray-400" />
                      </div>
                    </td>
                    <td className="px-4 py-2 whitespace-nowrap">
                      <div className="text-sm font-medium text-gray-900">{edge.to_logic_id}</div>
                    </td>
                    <td className="px-4 py-2 whitespace-nowrap">
                      <code className="text-xs bg-gray-100 px-2 py-1 rounded text-gray-700">
                        {edge.to_input_name || '-'}
                      </code>
                    </td>
                    <td className="px-4 py-2 whitespace-nowrap text-sm text-gray-500">
                      {new Date(edge.updated_at).toLocaleDateString()}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </section>
      </div>
    </div>
  );
}
