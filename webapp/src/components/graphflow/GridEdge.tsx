import { memo, useState } from 'react';
import type { EdgeProps } from '@xyflow/react';
import { BaseEdge, EdgeLabelRenderer, getSmoothStepPath } from '@xyflow/react';
import type { GridEdgeData } from './utils';
import { getEdgeColor } from './utils';

export const GridEdge = memo(({
  id,
  sourceX,
  sourceY,
  targetX,
  targetY,
  sourcePosition,
  targetPosition,
  data,
}: EdgeProps) => {
  const [isHovered, setIsHovered] = useState(false);
  const edgeData = data as GridEdgeData | undefined;

  const [edgePath, labelX, labelY] = getSmoothStepPath({
    sourceX,
    sourceY,
    sourcePosition,
    targetX,
    targetY,
    targetPosition,
  });

  const edgeColor = edgeData ? getEdgeColor(edgeData.status) : '#6b7280';

  return (
    <>
      <BaseEdge
        id={id}
        path={edgePath}
        style={{
          stroke: edgeColor,
          strokeWidth: isHovered ? 3 : 2,
          opacity: isHovered ? 1 : 0.7,
        }}
        markerEnd="url(#arrowhead)"
      />

      {/* Invisible wider path for easier hover detection */}
      <path
        d={edgePath}
        fill="none"
        stroke="transparent"
        strokeWidth={20}
        onMouseEnter={() => setIsHovered(true)}
        onMouseLeave={() => setIsHovered(false)}
        className="cursor-pointer"
      />

      {/* Tooltip */}
      {isHovered && edgeData && (
        <EdgeLabelRenderer>
          <div
            style={{
              position: 'absolute',
              transform: `translate(-50%, -50%) translate(${labelX}px, ${labelY}px)`,
              pointerEvents: 'none',
            }}
            className="z-50"
          >
            <div
              className="bg-white rounded-md shadow-lg px-3 py-2 border-2"
              style={{ borderColor: edgeColor }}
            >
              <div className="text-xs font-semibold text-gray-800">
                {edgeData.fromOutput}
              </div>
              <div className="text-xs text-gray-600 mt-0.5">
                {edgeData.status}
              </div>
            </div>
          </div>
        </EdgeLabelRenderer>
      )}
    </>
  );
});

GridEdge.displayName = 'GridEdge';
