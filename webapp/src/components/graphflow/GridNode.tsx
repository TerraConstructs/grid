import { memo, useState } from 'react';
import type { NodeProps } from '@xyflow/react';
import { Handle, Position } from '@xyflow/react';
import type { GridNodeData } from './utils';
import { getStatusColor } from './utils';

export const GridNode = memo(({ data, selected }: NodeProps) => {
  const [isHovered, setIsHovered] = useState(false);
  const nodeData = data as GridNodeData;
  const statusColor = getStatusColor(nodeData.status);

  return (
    <div
      className="relative group"
      onMouseEnter={() => setIsHovered(true)}
      onMouseLeave={() => setIsHovered(false)}
    >
      {/* Handles for connections */}
      <Handle type="target" position={Position.Top} className="opacity-0" />
      <Handle type="source" position={Position.Bottom} className="opacity-0" />

      {/* Node body */}
      <div
        className={`
          w-[200px] h-[80px] rounded-lg bg-white shadow-md
          transition-all duration-200
          ${selected || isHovered ? 'ring-2 ring-purple-500 shadow-lg' : 'ring-1 ring-gray-200'}
        `}
      >
        <div className="p-3 h-full flex flex-col justify-between">
          {/* Status indicator */}
          <div className="flex items-start justify-between">
            <div
              className="w-3 h-3 rounded-full"
              style={{ backgroundColor: statusColor }}
            />
          </div>

          {/* Label */}
          <div className="flex-1 flex items-center justify-center">
            <span className="text-sm font-semibold text-gray-800 text-center truncate px-2">
              {nodeData.label.length > 28 ? nodeData.label.slice(0, 25) + '...' : nodeData.label}
            </span>
          </div>

          {/* Status and output count */}
          <div className="flex flex-col gap-0.5">
            <span className="text-xs text-gray-600 text-center">
              {nodeData.computedStatus || 'unknown'}
            </span>
            <span className="text-xs text-gray-400 text-center">
              {nodeData.outputCount} outputs
            </span>
          </div>
        </div>
      </div>

      {/* Tooltip */}
      {isHovered && (
        <div className="absolute left-1/2 top-full z-50 mt-2 -translate-x-1/2 pointer-events-none">
          <div className="bg-gray-900 text-white text-xs rounded-md px-3 py-2 shadow-lg whitespace-nowrap">
            <div className="font-semibold">{nodeData.label}</div>
            <div className="text-gray-300 mt-1">Status: {nodeData.computedStatus || 'unknown'}</div>
            <div className="text-gray-300">GUID: {nodeData.guid.slice(0, 8)}...</div>
          </div>
        </div>
      )}
    </div>
  );
});

GridNode.displayName = 'GridNode';
