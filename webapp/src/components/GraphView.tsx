import { useEffect, useMemo, useCallback } from 'react';
import {
  ReactFlow,
  Background,
  Controls,
  MiniMap,
  useNodesState,
  useEdgesState,
  useReactFlow,
  Panel,
  ReactFlowProvider,
} from '@xyflow/react';
import type { NodeTypes, EdgeTypes } from '@xyflow/react';
import type { StateInfo, DependencyEdge } from '@tcons/grid';
import { LabelFilter, type ActiveLabelFilter } from './LabelFilter';
import { GridNode } from './graphflow/GridNode';
import { GridEdge } from './graphflow/GridEdge';
import { computeLayout, toReactFlowNodes, toReactFlowEdges, getStatusColor, type GridNodeData } from './graphflow/utils';

interface GraphViewProps {
  states: StateInfo[];
  edges: DependencyEdge[];
  onStateClick: (logicId: string) => void;
  activeFilters: ActiveLabelFilter[];
  onFilterChange: (expression: string, filters: ActiveLabelFilter[]) => void;
}

// Define custom node and edge types
const nodeTypes: NodeTypes = {
  gridNode: GridNode,
};

const edgeTypes: EdgeTypes = {
  gridEdge: GridEdge,
};

function GraphViewContent({
  states,
  edges,
  onStateClick,
  activeFilters,
  onFilterChange,
}: GraphViewProps) {
  // Compute layout and convert to React Flow format
  const { reactFlowNodes, reactFlowEdges } = useMemo(() => {
    const positionMap = computeLayout(states);
    return {
      reactFlowNodes: toReactFlowNodes(states, positionMap),
      reactFlowEdges: toReactFlowEdges(edges, positionMap),
    };
  }, [states, edges]);

  const [nodes, setNodes, onNodesChange] = useNodesState(reactFlowNodes);
  const [rfEdges, setEdges, onEdgesChange] = useEdgesState(reactFlowEdges);
  const { fitView } = useReactFlow();

  // Update nodes and edges when data changes
  useEffect(() => {
    setNodes(reactFlowNodes);
    setEdges(reactFlowEdges);
  }, [reactFlowNodes, reactFlowEdges, setNodes, setEdges]);

  // Fit view when nodes change
  useEffect(() => {
    if (nodes.length > 0) {
      // Small delay to ensure nodes are rendered
      setTimeout(() => {
        fitView({ padding: 0.2, duration: 200 });
      }, 10);
    }
  }, [nodes.length, fitView]);

  // Handle node clicks
  const onNodeClick = useCallback(
    (_event: React.MouseEvent, node: any) => {
      const state = states.find(s => s.guid === node.id);
      if (state) {
        onStateClick(state.logic_id);
      }
    },
    [states, onStateClick]
  );

  return (
    <div className="w-full h-full bg-gray-50">
      <div className="max-w-7xl mx-auto p-4 h-full flex flex-col gap-4">
        <LabelFilter
          onFilterChange={onFilterChange}
          initialFilters={activeFilters}
        />
        <div className="bg-white rounded-lg shadow flex-1 min-h-0">
          <ReactFlow
            nodes={nodes}
            edges={rfEdges}
            onNodesChange={onNodesChange}
            onEdgesChange={onEdgesChange}
            onNodeClick={onNodeClick}
            nodeTypes={nodeTypes}
            edgeTypes={edgeTypes}
            nodesDraggable={false}
            nodesConnectable={false}
            elementsSelectable={true}
            panOnScroll={true}
            zoomOnScroll={true}
            fitView
            minZoom={0.1}
            maxZoom={2}
          >
            <Background color="#e5e7eb" gap={16} />
            <Controls />
            <MiniMap
              nodeColor={(node) => {
                const nodeData = node.data as GridNodeData | undefined;
                const status = nodeData?.status;
                return getStatusColor(status);
              }}
              maskColor="rgba(0, 0, 0, 0.1)"
            />

            {/* Edge Status Legend */}
            <Panel position="bottom-right" className="bg-white rounded-lg shadow-lg p-3 space-y-1.5">
              <div className="text-xs font-semibold text-gray-700 mb-1">Edge Status</div>
              <div className="flex items-center gap-2 text-xs">
                <div className="w-3 h-1 bg-green-500 rounded"></div>
                <span className="text-gray-600">Clean</span>
              </div>
              <div className="flex items-center gap-2 text-xs">
                <div className="w-3 h-1 bg-amber-500 rounded"></div>
                <span className="text-gray-600">Dirty</span>
              </div>
              <div className="flex items-center gap-2 text-xs">
                <div className="w-3 h-1 bg-blue-500 rounded"></div>
                <span className="text-gray-600">Pending</span>
              </div>
            </Panel>
          </ReactFlow>
        </div>
      </div>
    </div>
  );
}

export function GraphView(props: GraphViewProps) {
  return (
    <ReactFlowProvider>
      <GraphViewContent {...props} />
    </ReactFlowProvider>
  );
}
