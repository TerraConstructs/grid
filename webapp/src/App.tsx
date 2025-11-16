import { useCallback, useEffect, useRef, useState } from 'react';
import type { StateInfo, DependencyEdge } from '@tcons/grid';
import { useGridData } from './hooks/useGridData';
import { GraphView } from './components/GraphView';
import { ListView } from './components/ListView';
import { DetailView } from './components/DetailView';
import { Network, List, Loader2, RefreshCw, AlertCircle } from 'lucide-react';
import type { ActiveLabelFilter } from './components/LabelFilter';
import { LoginPage } from './components/LoginPage';
import { AuthStatus } from './components/AuthStatus';
import { useAuth } from './context/AuthContext';

type View = 'graph' | 'list';

function AppContent() {
  const { state: authState, logout: authLogout } = useAuth();
  const [view, setView] = useState<View>('graph');
  const [selectedState, setSelectedState] = useState<StateInfo | null>(null);
  const {
    states,
    edges,
    loading,
    error,
    filter,
    loadData,
    getStateInfo,
  } = useGridData();
  const [activeFilters, setActiveFilters] = useState<ActiveLabelFilter[]>([]);
  const filterInitializedRef = useRef(false);

  const isAuthDisabled = authState.config?.mode === 'disabled';
  const canLoadGridData =
    !authState.loading &&
    authState.config !== null &&
    (isAuthDisabled || Boolean(authState.user));

  // Only load data once authentication is ready (or disabled mode)
  useEffect(() => {
    if (!canLoadGridData) {
      return;
    }
    loadData();
  }, [canLoadGridData, loadData]);

  const handleStateClick = useCallback(async (logicId: string) => {
    const state = await getStateInfo(logicId);
    if (state) {
      setSelectedState(state);
    }
  }, [getStateInfo]);

  const handleEdgeClick = useCallback((edge: DependencyEdge) => {
    console.log('Edge clicked:', edge);
  }, []);

  const handleNavigate = useCallback(async (logicId: string) => {
    const state = await getStateInfo(logicId);
    if (state) {
      setSelectedState(state);
    }
  }, [getStateInfo]);

  const handleLogout = useCallback(async () => {
    await authLogout();
    setSelectedState(null);
  }, [authLogout]);

  const handleRefresh = useCallback(async () => {
    const currentSelectedLogicId = selectedState?.logic_id;
    await loadData({ filter });

    // Preserve selected state after refresh
    if (currentSelectedLogicId) {
      const refreshedState = await getStateInfo(currentSelectedLogicId);
      if (refreshedState) {
        setSelectedState(refreshedState);
      } else {
        setSelectedState(null);
      }
    }
  }, [selectedState, filter, loadData, getStateInfo]);

  const handleFilterChange = useCallback((expression: string, filtersList: ActiveLabelFilter[]) => {
    setActiveFilters(filtersList);

    if (!filterInitializedRef.current) {
      filterInitializedRef.current = true;
      return;
    }

    if (expression === filter) {
      return;
    }

    void loadData({ filter: expression });
  }, [filter, loadData]);

  // Show loading spinner while checking authentication
  if (authState.loading) {
    return (
      <div className="min-h-screen bg-gray-900 flex items-center justify-center">
        <div className="flex items-center gap-3">
          <Loader2 className="w-8 h-8 text-purple-600 animate-spin" />
          <span className="text-white text-lg">Loading...</span>
        </div>
      </div>
    );
  }

  // Show login page if not authenticated and auth is required
  if (!authState.user && !isAuthDisabled) {
    return <LoginPage />;
  }

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

          <div className="hidden md:flex items-center gap-4 text-sm">
            <div className="text-gray-400">
              <span className="text-white font-semibold">{states.length}</span> states
            </div>
            <div className="text-gray-400">
              <span className="text-white font-semibold">{edges.length}</span> edges
            </div>
            <button
              onClick={handleRefresh}
              disabled={loading}
              className="flex items-center gap-2 px-3 py-1.5 rounded-lg text-sm font-medium bg-gray-700 text-gray-300 hover:bg-gray-600 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
              title="Refresh data"
            >
              <RefreshCw className={`w-4 h-4 ${loading ? 'animate-spin' : ''}`} />
              Refresh
            </button>
            {authState.user && (
              <div className="pl-2 border-l border-gray-600">
                <AuthStatus user={authState.user} onLogout={handleLogout} />
              </div>
            )}
          </div>
        </div>
      </header>

      {error && (
        <div className="bg-red-900/50 border-b border-red-700">
          <div className="max-w-7xl mx-auto px-4 py-3 flex items-center gap-3">
            <AlertCircle className="w-5 h-5 text-red-400" />
            <div className="flex-1">
              <p className="text-sm font-medium text-red-200">Failed to load data</p>
              <p className="text-xs text-red-300">{error}</p>
            </div>
            <button
              onClick={handleRefresh}
              className="text-sm text-red-200 hover:text-white font-medium"
            >
              Retry
            </button>
          </div>
        </div>
      )}

      <main className="flex-1 overflow-hidden">
        {view === 'graph' ? (
          <GraphView
            states={states}
            edges={edges}
            onStateClick={handleStateClick}
            activeFilters={activeFilters}
            onFilterChange={handleFilterChange}
          />
        ) : (
          <ListView
            states={states}
            edges={edges}
            onStateClick={handleStateClick}
            onEdgeClick={handleEdgeClick}
            activeFilters={activeFilters}
            onFilterChange={handleFilterChange}
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

function App() {
  return <AppContent />;
}

export default App;
