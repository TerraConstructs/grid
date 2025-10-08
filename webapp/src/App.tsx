import { useEffect, useState } from 'react';
import { mockApi, StateInfo, DependencyEdge } from './services/mockApi';
import { GraphView } from './components/GraphView';
import { ListView } from './components/ListView';
import { DetailView } from './components/DetailView';
import { Network, List, Loader2 } from 'lucide-react';

type View = 'graph' | 'list';

function App() {
  const [view, setView] = useState<View>('graph');
  const [states, setStates] = useState<StateInfo[]>([]);
  const [edges, setEdges] = useState<DependencyEdge[]>([]);
  const [selectedState, setSelectedState] = useState<StateInfo | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    loadData();
  }, []);

  const loadData = async () => {
    setLoading(true);
    try {
      const [statesData, edgesData] = await Promise.all([
        mockApi.listStates(),
        mockApi.getAllEdges(),
      ]);
      setStates(statesData);
      setEdges(edgesData);
    } catch (error) {
      console.error('Failed to load data:', error);
    } finally {
      setLoading(false);
    }
  };

  const handleStateClick = async (logicId: string) => {
    const state = await mockApi.getStateInfo(logicId);
    if (state) {
      setSelectedState(state);
    }
  };

  const handleEdgeClick = (edge: DependencyEdge) => {
    console.log('Edge clicked:', edge);
  };

  const handleNavigate = async (logicId: string) => {
    const state = await mockApi.getStateInfo(logicId);
    if (state) {
      setSelectedState(state);
    }
  };

  if (loading) {
    return (
      <div className="min-h-screen bg-gray-900 flex items-center justify-center">
        <div className="flex items-center gap-3">
          <Loader2 className="w-8 h-8 text-purple-600 animate-spin" />
          <span className="text-white text-lg">Loading Grid...</span>
        </div>
      </div>
    );
  }

  return (
    <div className="min-h-screen bg-gray-900 flex flex-col">
      <header className="bg-gray-800 border-b border-gray-700 shadow-lg">
        <div className="max-w-7xl mx-auto px-4 py-2 flex items-center justify-between">
          <div className="flex items-center gap-2">
            <div className="w-8 h-8 bg-gradient-to-br from-purple-600 to-purple-700 rounded-lg flex items-center justify-center">
              <Network className="w-5 h-5 text-white" />
            </div>
            <div>
              <h1 className="text-xl font-bold text-white">Grid</h1>
              <p className="text-xs text-gray-400">Terraform State Management</p>
            </div>
          </div>

          <div className="flex items-center gap-2">
            <button
              onClick={() => setView('graph')}
              className={`flex items-center gap-2 px-3 py-1.5 rounded-lg text-sm font-medium transition-colors ${
                view === 'graph'
                  ? 'bg-purple-600 text-white'
                  : 'bg-gray-700 text-gray-300 hover:bg-gray-600'
              }`}
            >
              <Network className="w-4 h-4" />
              Graph
            </button>
            <button
              onClick={() => setView('list')}
              className={`flex items-center gap-2 px-3 py-1.5 rounded-lg text-sm font-medium transition-colors ${
                view === 'list'
                  ? 'bg-purple-600 text-white'
                  : 'bg-gray-700 text-gray-300 hover:bg-gray-600'
              }`}
            >
              <List className="w-4 h-4" />
              List
            </button>
          </div>

          <div className="flex items-center gap-4 text-sm">
            <div className="text-gray-400">
              <span className="text-white font-semibold">{states.length}</span> states
            </div>
            <div className="text-gray-400">
              <span className="text-white font-semibold">{edges.length}</span> edges
            </div>
          </div>
        </div>
      </header>

      <main className="flex-1 overflow-hidden">
        {view === 'graph' ? (
          <GraphView states={states} edges={edges} onStateClick={handleStateClick} />
        ) : (
          <ListView
            states={states}
            edges={edges}
            onStateClick={handleStateClick}
            onEdgeClick={handleEdgeClick}
          />
        )}
      </main>

      {selectedState && (
        <DetailView
          state={selectedState}
          onClose={() => setSelectedState(null)}
          onNavigate={handleNavigate}
        />
      )}
    </div>
  );
}

export default App;
