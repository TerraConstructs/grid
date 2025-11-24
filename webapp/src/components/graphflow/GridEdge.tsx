import { memo, useState } from 'react';
import type { EdgeProps } from '@xyflow/react';
import { BaseEdge, EdgeLabelRenderer, getSmoothStepPath } from '@xyflow/react';
import type { GridEdgeData } from './utils';
import { getEdgeColor } from './utils';

const NODE_WIDTH = 200;
const HORIZONTAL_MARGIN = 28;

export const GridEdge = memo(({
  id,
  sourceX,
  sourceY,
  targetX,
  targetY,
  sourcePosition,
  targetPosition,
  data,
  markerEnd,
}: EdgeProps) => {
  const [isHovered, setIsHovered] = useState(false);
  const edgeData = data as GridEdgeData | undefined;

  // Calculate custom source and target X positions based on slot positions
  let customSourceX = sourceX;
  let customTargetX = targetX;

  if (edgeData) {
    // Calculate source anchor X based on slot position
    if (edgeData.sourceSlotCount && edgeData.sourceSlotCount > 1) {
      const availableWidth = Math.max(NODE_WIDTH - HORIZONTAL_MARGIN * 2, 0);
      const slotSpacing = availableWidth / (edgeData.sourceSlotCount - 1);
      const slotPosition = edgeData.sourceSlot ?? 0;
      customSourceX = sourceX - NODE_WIDTH / 2 + HORIZONTAL_MARGIN + slotPosition * slotSpacing;
    }

    // Calculate target anchor X based on slot position
    if (edgeData.targetSlotCount && edgeData.targetSlotCount > 1) {
      const availableWidth = Math.max(NODE_WIDTH - HORIZONTAL_MARGIN * 2, 0);
      const slotSpacing = availableWidth / (edgeData.targetSlotCount - 1);
      const slotPosition = edgeData.targetSlot ?? 0;
      customTargetX = targetX - NODE_WIDTH / 2 + HORIZONTAL_MARGIN + slotPosition * slotSpacing;
    }
  }

  const [edgePath, labelX, labelY] = getSmoothStepPath({
    sourceX: customSourceX,
    sourceY,
    sourcePosition,
    targetX: customTargetX,
    targetY,
    targetPosition,
  });

  const edgeColor = edgeData ? getEdgeColor(edgeData.status) : '#6b7280';

  return (
    <>
      <BaseEdge
        id={id}
        path={edgePath}
        markerEnd={markerEnd}
        style={{
          stroke: edgeColor,
          strokeWidth: isHovered ? 3 : 2,
          opacity: isHovered ? 1 : 0.7,
        }}
        data-testid={`graph-edge-${id}`}
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
        data-testid={`graph-edge-hover-${id}`}
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
