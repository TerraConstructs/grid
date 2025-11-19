import type { Node, Edge } from '@xyflow/react';
import type { StateInfo, DependencyEdge } from '@tcons/grid';

export interface GridNodeData extends Record<string, unknown> {
  label: string;
  status: string | undefined;
  guid: string;
  outputCount: number;
  computedStatus: string | undefined;
}

export interface GridEdgeData extends Record<string, unknown> {
  fromOutput: string;
  status: string;
}

/**
 * Compute layered layout for DAG nodes
 * Returns positions for each state based on dependency layers
 */
export function computeLayout(states: StateInfo[]): Map<string, { x: number; y: number; layer: number }> {
  const layers: Map<number, StateInfo[]> = new Map();
  const visited = new Set<string>();
  const layerMap = new Map<string, number>();

  // Assign layers based on dependencies
  const assignLayer = (state: StateInfo, layer: number) => {
    if (visited.has(state.guid)) return;
    visited.add(state.guid);

    const currentLayer = layerMap.get(state.guid) ?? -1;
    const newLayer = Math.max(currentLayer, layer);
    layerMap.set(state.guid, newLayer);

    state.dependents.forEach((edge: DependencyEdge) => {
      const dependent = states.find(s => s.guid === edge.to_guid);
      if (dependent) {
        assignLayer(dependent, newLayer + 1);
      }
    });
  };

  // Start with root nodes (no dependencies)
  const roots = states.filter(s => s.dependencies.length === 0);
  roots.forEach(root => assignLayer(root, 0));

  // Assign any remaining nodes
  states.forEach(state => {
    if (!layerMap.has(state.guid)) {
      assignLayer(state, 0);
    }
  });

  // Group states by layer
  states.forEach(state => {
    const layer = layerMap.get(state.guid) ?? 0;
    if (!layers.has(layer)) {
      layers.set(layer, []);
    }
    layers.get(layer)!.push(state);
  });

  // Calculate positions
  const positionMap = new Map<string, { x: number; y: number; layer: number }>();
  const nodeWidth = 220;
  const nodeHeight = 100;
  const horizontalSpacing = 100;
  const verticalSpacing = 150;

  Array.from(layers.entries()).forEach(([layer, layerStates]) => {
    const layerWidth = layerStates.length * (nodeWidth + horizontalSpacing) - horizontalSpacing;
    const startX = -layerWidth / 2;
    const y = layer * (nodeHeight + verticalSpacing);

    layerStates.forEach((state, index) => {
      const x = startX + index * (nodeWidth + horizontalSpacing);
      positionMap.set(state.guid, { x, y, layer });
    });
  });

  return positionMap;
}

/**
 * Convert StateInfo array to React Flow nodes
 */
export function toReactFlowNodes(
  states: StateInfo[],
  positionMap: Map<string, { x: number; y: number; layer: number }>
): Node<GridNodeData>[] {
  return states.map((state) => {
    const position = positionMap.get(state.guid) ?? { x: 0, y: 0, layer: 0 };

    return {
      id: state.guid,
      position: { x: position.x, y: position.y },
      data: {
        label: state.logic_id,
        status: state.computed_status,
        guid: state.guid,
        outputCount: state.outputs.length,
        computedStatus: state.computed_status,
      },
      type: 'gridNode',
    };
  });
}

/**
 * Convert DependencyEdge array to React Flow edges
 */
export function toReactFlowEdges(edges: DependencyEdge[]): Edge<GridEdgeData>[] {
  return edges.map((edge) => ({
    id: `${edge.id}`,
    source: edge.from_guid,
    target: edge.to_guid,
    data: {
      fromOutput: edge.from_output,
      status: edge.status,
    },
    type: 'gridEdge',
  }));
}

/**
 * Get color for edge based on status
 */
export function getEdgeColor(status: string): string {
  switch (status) {
    case 'clean':
      return '#10b981'; // green-500
    case 'dirty':
      return '#f59e0b'; // amber-500
    case 'pending':
      return '#3b82f6'; // blue-500
    case 'potentially-stale':
      return '#f59e0b'; // amber-500
    default:
      return '#6b7280'; // gray-500
  }
}

/**
 * Get color for node status indicator
 */
export function getStatusColor(status?: string): string {
  switch (status) {
    case 'clean':
      return '#10b981'; // green-500
    case 'stale':
      return '#f59e0b'; // amber-500
    case 'potentially-stale':
      return '#f59e0b'; // amber-500
    default:
      return '#6b7280'; // gray-500
  }
}
