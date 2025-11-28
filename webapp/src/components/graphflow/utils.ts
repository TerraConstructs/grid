import type { Node, Edge } from '@xyflow/react';
import type { StateSummary, DependencyEdge } from '@tcons/grid';

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
  sourceSlot?: number;
  sourceSlotCount?: number;
  targetSlot?: number;
  targetSlotCount?: number;
}

/**
 * Compute layered layout for DAG nodes using dependency edges
 * Returns positions for each state based on dependency layers
 */
export function computeLayout(states: StateSummary[], edges: DependencyEdge[]): Map<string, { x: number; y: number; layer: number }> {
  if (!states || states.length === 0) {
    return new Map();
  }

  const layers: Map<number, StateSummary[]> = new Map();
  const visited = new Set<string>();
  const layerMap = new Map<string, number>();

  // Assign layers based on dependencies using edges
  const assignLayer = (state: StateSummary, layer: number) => {
    if (visited.has(state.guid)) return;
    visited.add(state.guid);

    const currentLayer = layerMap.get(state.guid) ?? -1;
    const newLayer = Math.max(currentLayer, layer);
    layerMap.set(state.guid, newLayer);

    // Find outgoing edges from this state
    const outgoingEdges = edges.filter(e => e.from_guid === state.guid);
    outgoingEdges.forEach((edge) => {
      const dependent = states.find(s => s.guid === edge.to_guid);
      if (dependent) {
        assignLayer(dependent, newLayer + 1);
      }
    });
  };

  // Start with root nodes (no dependencies)
  const roots = states.filter(s => (s.dependencies_count ?? 0) === 0);
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
 * Convert StateSummary array to React Flow nodes
 */
export function toReactFlowNodes(
  states: StateSummary[],
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
        outputCount: state.outputs_count ?? 0,
        computedStatus: state.computed_status,
      },
      type: 'gridNode',
    };
  });
}

/**
 * Convert DependencyEdge array to React Flow edges with slot positions
 */
export function toReactFlowEdges(
  edges: DependencyEdge[],
  positionMap: Map<string, { x: number; y: number; layer: number }>
): Edge<GridEdgeData>[] {
  // Group edges by source node
  const edgesBySource = new Map<string, DependencyEdge[]>();
  const edgesByTarget = new Map<string, DependencyEdge[]>();

  edges.forEach((edge) => {
    // Group by source
    if (!edgesBySource.has(edge.from_guid)) {
      edgesBySource.set(edge.from_guid, []);
    }
    edgesBySource.get(edge.from_guid)!.push(edge);

    // Group by target
    if (!edgesByTarget.has(edge.to_guid)) {
      edgesByTarget.set(edge.to_guid, []);
    }
    edgesByTarget.get(edge.to_guid)!.push(edge);
  });

  // Sort source groups by target X position
  edgesBySource.forEach((edgeGroup) => {
    edgeGroup.sort((a, b) => {
      const posA = positionMap.get(a.to_guid);
      const posB = positionMap.get(b.to_guid);
      return (posA?.x ?? 0) - (posB?.x ?? 0);
    });
  });

  // Sort target groups by source X position
  edgesByTarget.forEach((edgeGroup) => {
    edgeGroup.sort((a, b) => {
      const posA = positionMap.get(a.from_guid);
      const posB = positionMap.get(b.from_guid);
      return (posA?.x ?? 0) - (posB?.x ?? 0);
    });
  });

  return edges.map((edge) => {
    const sourceGroup = edgesBySource.get(edge.from_guid) ?? [];
    const targetGroup = edgesByTarget.get(edge.to_guid) ?? [];

    const sourceSlot = sourceGroup.findIndex((e) => e.id === edge.id);
    const targetSlot = targetGroup.findIndex((e) => e.id === edge.id);

    return {
      id: `${edge.id}`,
      source: edge.from_guid,
      target: edge.to_guid,
      markerEnd: {
        type: 'arrowclosed',
        color: getEdgeColor(edge.status),
      },
      data: {
        fromOutput: edge.from_output,
        status: edge.status,
        sourceSlot: sourceSlot >= 0 ? sourceSlot : 0,
        sourceSlotCount: sourceGroup.length,
        targetSlot: targetSlot >= 0 ? targetSlot : 0,
        targetSlotCount: targetGroup.length,
      },
      type: 'gridEdge',
    };
  });
}

/**
 * Get color for edge based on status
 */
export function getEdgeColor(status: string): string {
  switch (status) {
    case 'clean':
      return '#10b981'; // green-500
    case 'clean-invalid':
      return '#ef4444'; // red-500 - synchronized but invalid
    case 'dirty':
      return '#f59e0b'; // amber-500
    case 'dirty-invalid':
      return '#ef4444'; // red-500 - out of sync and invalid
    case 'pending':
      return '#3b82f6'; // blue-500
    case 'potentially-stale':
      return '#f59e0b'; // amber-500
    case 'schema-invalid':
      return '#ef4444'; // red-500 - output fails schema validation
    case 'missing-output':
      return '#dc2626'; // red-600 - producer output missing
    case 'mock':
      return '#6b7280'; // gray-500 - using mock value
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
