import { useEffect, useMemo, useRef, useState } from 'react';
import type { StateInfo, DependencyEdge } from '@tcons/grid';

interface GraphViewProps {
  states: StateInfo[];
  edges: DependencyEdge[];
  onStateClick: (logicId: string) => void;
}

interface NodePosition {
  x: number;
  y: number;
  state: StateInfo;
  layer: number;
}

interface DirectedEdgeGroup {
  up: DependencyEdge[];
  down: DependencyEdge[];
}

const getEdgeColor = (status: string): string => {
  switch (status) {
    case 'clean':
      return '#10b981';
    case 'dirty':
      return '#f59e0b';
    case 'pending':
      return '#3b82f6';
    case 'potentially-stale':
      return '#f59e0b';
    default:
      return '#6b7280';
  }
};

const getStatusColor = (status?: string): string => {
  switch (status) {
    case 'clean':
      return '#10b981';
    case 'stale':
      return '#f59e0b';
    case 'potentially-stale':
      return '#f59e0b';
    default:
      return '#6b7280';
  }
};

export function GraphView({ states, edges, onStateClick }: GraphViewProps) {
  const svgRef = useRef<SVGSVGElement>(null);
  const [dimensions, setDimensions] = useState({ width: 1200, height: 800 });
  const [hoveredNode, setHoveredNode] = useState<string | null>(null);
  const [hoveredEdge, setHoveredEdge] = useState<number | null>(null);

  useEffect(() => {
    const updateDimensions = () => {
      if (svgRef.current) {
        const rect = svgRef.current.parentElement?.getBoundingClientRect();
        if (rect) {
          setDimensions({ width: rect.width, height: rect.height });
        }
      }
    };

    updateDimensions();
    window.addEventListener('resize', updateDimensions);
    return () => window.removeEventListener('resize', updateDimensions);
  }, []);

  const computeLayout = (): NodePosition[] => {
    const layers: Map<number, StateInfo[]> = new Map();
    const visited = new Set<string>();
    const layerMap = new Map<string, number>();

    const assignLayer = (state: StateInfo, layer: number) => {
      if (visited.has(state.guid)) return;
      visited.add(state.guid);

      const currentLayer = layerMap.get(state.guid) ?? -1;
      const newLayer = Math.max(currentLayer, layer);
      layerMap.set(state.guid, newLayer);

      state.dependents.forEach(edge => {
        const dependent = states.find(s => s.guid === edge.to_guid);
        if (dependent) {
          assignLayer(dependent, newLayer + 1);
        }
      });
    };

    const roots = states.filter(s => s.dependencies.length === 0);
    roots.forEach(root => assignLayer(root, 0));

    states.forEach(state => {
      if (!layerMap.has(state.guid)) {
        assignLayer(state, 0);
      }
    });

    states.forEach(state => {
      const layer = layerMap.get(state.guid) ?? 0;
      if (!layers.has(layer)) {
        layers.set(layer, []);
      }
      layers.get(layer)!.push(state);
    });

    const positions: NodePosition[] = [];
    const layerCount = Math.max(...Array.from(layers.keys())) + 1;
    const nodeWidth = 200;
    const nodeHeight = 80;
    const horizontalSpacing = 100;
    const verticalSpacing = 120;

    Array.from(layers.entries()).forEach(([layer, layerStates]) => {
      const layerWidth = layerStates.length * (nodeWidth + horizontalSpacing) - horizontalSpacing;
      const startX = (dimensions.width - layerWidth) / 2;
      const y = 100 + layer * (nodeHeight + verticalSpacing);

      layerStates.forEach((state, index) => {
        const x = startX + index * (nodeWidth + horizontalSpacing);
        positions.push({ x, y, state, layer });
      });
    });

    return positions;
  };

  const positions = computeLayout();
  const positionMap = new Map(positions.map(p => [p.state.guid, p]));

  const parallelGroups = useMemo(() => {
    const groups = new Map<string, DependencyEdge[]>();
    edges.forEach(edge => {
      const key = `${edge.from_guid}|${edge.to_guid}`;
      const list = groups.get(key);
      if (list) {
        list.push(edge);
      } else {
        groups.set(key, [edge]);
      }
    });
    return groups;
  }, [edges]);

  const outgoingGroups = useMemo(() => {
    const groups = new Map<string, DirectedEdgeGroup>();

    edges.forEach(edge => {
      const fromPosition = positionMap.get(edge.from_guid);
      const toPosition = positionMap.get(edge.to_guid);
      if (!fromPosition || !toPosition) {
        return;
      }

      let entry = groups.get(edge.from_guid);
      if (!entry) {
        entry = { up: [], down: [] };
        groups.set(edge.from_guid, entry);
      }

      if (toPosition.y >= fromPosition.y) {
        entry.down.push(edge);
      } else {
        entry.up.push(edge);
      }
    });

    const sortByTargetX = (a: DependencyEdge, b: DependencyEdge) => {
      const targetA = positionMap.get(a.to_guid);
      const targetB = positionMap.get(b.to_guid);
      return (targetA?.x ?? 0) - (targetB?.x ?? 0);
    };

    groups.forEach(entry => {
      entry.down.sort(sortByTargetX);
      entry.up.sort(sortByTargetX);
    });

    return groups;
  }, [edges, positions]);

  const getEdgePathData = (edge: DependencyEdge) => {
    const from = positionMap.get(edge.from_guid);
    const to = positionMap.get(edge.to_guid);
    if (!from || !to) return null;

    const nodeWidth = 200;
    const nodeHeight = 80;
    const fromCenterX = from.x + nodeWidth / 2;
    const toCenterX = to.x + nodeWidth / 2;
    const isDownward = to.y >= from.y;
    const fromAnchorY = isDownward ? from.y + nodeHeight : from.y;
    const toAnchorY = isDownward ? to.y : to.y + nodeHeight;

    const parallelKey = `${edge.from_guid}|${edge.to_guid}`;
    const parallelGroup = parallelGroups.get(parallelKey) ?? [edge];
    const parallelIndex = parallelGroup.findIndex(e => e.id === edge.id);
    const parallelSlots = parallelGroup.length || 1;
    const parallelStep = 14;
    const parallelOffset = (parallelIndex - (parallelSlots - 1) / 2) * parallelStep;

    const directedGroup = outgoingGroups.get(edge.from_guid);
    const outgoingGroup = directedGroup
      ? (isDownward ? directedGroup.down : directedGroup.up)
      : [];

    const outgoingIndex = outgoingGroup.findIndex(e => e.id === edge.id);
    const slotCount = outgoingGroup.length;
    const slotPosition = outgoingIndex >= 0 ? outgoingIndex : 0;
    const horizontalMargin = 28;
    const availableWidth = Math.max(nodeWidth - horizontalMargin * 2, 0);
    const slotSpacing = slotCount > 1 ? availableWidth / (slotCount - 1) : 0;
    const fromAnchorX = slotCount > 1
      ? from.x + horizontalMargin + slotPosition * slotSpacing
      : fromCenterX;

    const elbowOffset = 24;
    const direction = isDownward ? 1 : -1;
    const elbowY = fromAnchorY + direction * elbowOffset;
    const toX = toCenterX + parallelOffset;
    const tooltipX = fromAnchorX + (toX - fromAnchorX) / 2;
    const tooltipY = (elbowY + toAnchorY) / 2;
    const path = [
      `M ${fromAnchorX} ${fromAnchorY}`,
      `L ${fromAnchorX} ${elbowY}`,
      `L ${toX} ${elbowY}`,
      `L ${toX} ${toAnchorY}`,
    ].join(' ');

    return { path, tooltipX, tooltipY };
  };

  const renderEdge = (edge: DependencyEdge) => {
    const data = getEdgePathData(edge);
    if (!data) return null;

    const isHovered = hoveredEdge === edge.id;
    const color = getEdgeColor(edge.status);
    const strokeWidth = isHovered ? 3 : 2;

    return (
      <path
        key={edge.id}
        d={data.path}
        stroke={color}
        strokeWidth={strokeWidth}
        fill="none"
        markerEnd="url(#arrowhead)"
        className="transition-all cursor-pointer"
        onMouseEnter={() => setHoveredEdge(edge.id)}
        onMouseLeave={() => setHoveredEdge(null)}
        opacity={isHovered ? 1 : 0.7}
        style={{ color }}
      />
    );
  };

  const renderTooltip = (edge: DependencyEdge) => {
    if (hoveredEdge !== edge.id) return null;

    const data = getEdgePathData(edge);
    if (!data) return null;

    const color = getEdgeColor(edge.status);

    return (
      <g key={edge.id} style={{ pointerEvents: 'none' }}>
        <rect
          x={data.tooltipX - 70}
          y={data.tooltipY - 25}
          width={140}
          height={50}
          fill="white"
          stroke={color}
          strokeWidth={2}
          rx={4}
          filter="url(#tooltip-shadow)"
        />
        <text
          x={data.tooltipX}
          y={data.tooltipY - 8}
          textAnchor="middle"
          className="text-xs font-semibold"
          fill="#1f2937"
        >
          {edge.from_output}
        </text>
        <text
          x={data.tooltipX}
          y={data.tooltipY + 8}
          textAnchor="middle"
          className="text-xs"
          fill="#6b7280"
        >
          {edge.status}
        </text>
      </g>
    );
  };

  const renderNode = (position: NodePosition) => {
    const { x, y, state } = position;
    const isHovered = hoveredNode === state.guid;
    const statusColor = getStatusColor(state.computed_status);

    return (
      <g
        key={state.guid}
        transform={`translate(${x}, ${y})`}
        onClick={() => onStateClick(state.logic_id)}
        onMouseEnter={() => setHoveredNode(state.guid)}
        onMouseLeave={() => setHoveredNode(null)}
        className="cursor-pointer transition-all"
      >
        <rect
          width={200}
          height={80}
          rx={8}
          fill="white"
          stroke={isHovered ? '#8B5CF6' : '#e5e7eb'}
          strokeWidth={isHovered ? 3 : 2}
          className="transition-all"
          filter={isHovered ? 'url(#shadow)' : undefined}
        />
        <circle
          cx={16}
          cy={16}
          r={6}
          fill={statusColor}
        />
        <text
          x={100}
          y={30}
          textAnchor="middle"
          className="text-sm font-semibold"
          fill="#1f2937"
        >
          {state.logic_id}
        </text>
        <text
          x={100}
          y={50}
          textAnchor="middle"
          className="text-xs"
          fill="#6b7280"
        >
          {state.computed_status || 'unknown'}
        </text>
        <text
          x={100}
          y={65}
          textAnchor="middle"
          className="text-xs"
          fill="#9ca3af"
        >
          {state.outputs.length} outputs
        </text>
      </g>
    );
  };

  return (
    <div className="w-full h-full bg-gray-50 overflow-auto">
      <svg
        ref={svgRef}
        width={dimensions.width}
        height={Math.max(dimensions.height, 800)}
        className="min-h-full"
      >
        <defs>
          <marker
            id="arrowhead"
            markerWidth="10"
            markerHeight="10"
            refX="9"
            refY="3"
            orient="auto"
          >
            <polygon points="0 0, 10 3, 0 6" fill="currentColor" />
          </marker>
          <filter id="shadow" x="-50%" y="-50%" width="200%" height="200%">
            <feGaussianBlur in="SourceAlpha" stdDeviation="3" />
            <feOffset dx="0" dy="2" result="offsetblur" />
            <feComponentTransfer>
              <feFuncA type="linear" slope="0.3" />
            </feComponentTransfer>
            <feMerge>
              <feMergeNode />
              <feMergeNode in="SourceGraphic" />
            </feMerge>
          </filter>
          <filter id="tooltip-shadow" x="-50%" y="-50%" width="200%" height="200%">
            <feGaussianBlur in="SourceAlpha" stdDeviation="4" />
            <feOffset dx="0" dy="3" result="offsetblur" />
            <feComponentTransfer>
              <feFuncA type="linear" slope="0.5" />
            </feComponentTransfer>
            <feMerge>
              <feMergeNode />
              <feMergeNode in="SourceGraphic" />
            </feMerge>
          </filter>
        </defs>

        {edges.map(renderEdge)}
        {positions.map(renderNode)}
        {edges.map(renderTooltip)}
      </svg>

      <div className="fixed bottom-4 right-4 bg-white rounded-lg shadow-lg p-3 space-y-1.5 z-10">
        <div className="text-xs font-semibold text-gray-700 mb-1">Edge Status</div>
        <div className="flex items-center gap-2 text-xs">
          <div className="w-3 h-1 bg-green-500 rounded"></div>
          <span className="text-gray-600">Clean</span>
        </div>
        <div className="flex items-center gap-2 text-xs">
          <div className="w-3 h-1 bg-orange-500 rounded"></div>
          <span className="text-gray-600">Dirty</span>
        </div>
        <div className="flex items-center gap-2 text-xs">
          <div className="w-3 h-1 bg-blue-500 rounded"></div>
          <span className="text-gray-600">Pending</span>
        </div>
      </div>
    </div>
  );
}
