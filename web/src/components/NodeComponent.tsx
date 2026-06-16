import React from 'react';
import { Handle, Position, NodeProps } from 'reactflow';
import type { FlowNode } from '../types/flow';

interface NodeData {
  label: string;
  node: FlowNode;
}

interface NodeComponentProps extends NodeProps {
  data: NodeData;
}

const nodeIcons: Record<string, string> = {
  input: '📥',
  output: '📤',
  function: '🔄',
  storage: '💾',
  network: '🌐',
  protocol: '🔌',
  parser: '📋',
  social: '💬',
  dashboard: '📊',
  custom: '⚙️',
};

const nodeColors: Record<string, string> = {
  input: 'bg-blue-500',
  output: 'bg-green-500',
  function: 'bg-purple-500',
  storage: 'bg-orange-500',
  network: 'bg-cyan-500',
  protocol: 'bg-indigo-500',
  parser: 'bg-pink-500',
  social: 'bg-rose-500',
  dashboard: 'bg-teal-500',
  custom: 'bg-gray-500',
};

export function NodeComponent({ data, selected }: NodeComponentProps) {
  const { label, node } = data;
  const getNodeMetadata = (): { category: string; name: string; description: string } => {
    const category = node.type.includes('input') ? 'input' :
                     node.type.includes('output') ? 'output' :
                     node.type.includes('function') ? 'function' :
                     node.type.includes('http') ? 'network' :
                     'custom';
    return {
      category,
      name: label,
      description: `Node for ${node.type}`,
    };
  };

  const metadata = getNodeMetadata();
  const icon = nodeIcons[metadata.category] || '⚙️';
  const color = nodeColors[metadata.category] || 'bg-gray-500';

  const getStatusColor = () => {
    if (!node.status) return 'bg-gray-200';
    switch (node.status.state) {
      case 'processing': return 'bg-yellow-400';
      case 'error': return 'bg-red-500';
      case 'completed': return 'bg-green-500';
      case 'idle': return 'bg-blue-200';
      default: return 'bg-gray-200';
    }
  };

  return (
    <div
      className={`rounded-md border-2 ${selected ? 'border-blue-500' : 'border-gray-300'} bg-white shadow-sm`}
    >
      <div className={`flex items-center justify-between p-2 rounded-t-md ${color} text-white`}>
        <div className="flex items-center gap-2">
          <span className="text-lg">{icon}</span>
          <span className="font-medium text-sm">{label}</span>
        </div>
        <div className={`w-3 h-3 rounded-full ${getStatusColor()}`} />
      </div>

      <div className="p-2">
        {node.config && Object.keys(node.config).length > 0 && (
          <div className="text-xs text-gray-600">
            {Object.entries(node.config).map(([key, value]) => (
              <div key={key} className="truncate">
                {key}: {JSON.stringify(value)}
              </div>
            ))}
          </div>
        )}
      </div>

      <div className="flex justify-center gap-1">
        <Handle
          type="target"
          position={Position.Left}
          id="input"
          className="w-3 h-3 bg-blue-500"
        />
      </div>

      <div className="flex justify-center gap-1">
        <Handle
          type="source"
          position={Position.Right}
          id="output"
          className="w-3 h-3 bg-green-500"
        />
      </div>
    </div>
  );
}