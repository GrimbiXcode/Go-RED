
import { Handle, Position, NodeProps } from 'reactflow';
import type { FlowNode } from '../types/flow';
import type { NodeMetadata, Port } from '../types/node';
import DOMPurify from 'dompurify';

interface NodeData {
  label: string;
  node: FlowNode;
  metadata: NodeMetadata | null;
}

interface NodeComponentProps extends NodeProps {
  data: NodeData;
}

const categoryIcons: Record<string, string> = {
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

const categoryColors: Record<string, string> = {
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
  const { label, node, metadata } = data;
  
  const icon = metadata?.icon || categoryIcons[metadata?.category || 'custom'] || '⚙️';
  const color = metadata?.color || categoryColors[metadata?.category || 'custom'] || 'bg-gray-500';

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

  const getHandleColor = (port: Port) => {
    if (port.required) return 'bg-red-500';
    return 'bg-blue-500';
  };

  const inputPorts = metadata?.inputs || [];
  const outputPorts = metadata?.outputs || [];

  return (
    <div
      className={`rounded-md border-2 ${selected ? 'border-blue-500' : 'border-gray-300'} bg-white shadow-sm`}
      title={metadata?.description || `Node: ${metadata?.name || node.type}`}
    >
      <div className={`flex items-center justify-between p-2 rounded-t-md ${color} text-white`}>
        <div className="flex items-center gap-2">
          {icon.startsWith('<svg') ? (
            <span 
              className="text-lg"
              dangerouslySetInnerHTML={{ __html: DOMPurify.sanitize(icon) }}
            />
          ) : (
            <span className="text-lg">{icon}</span>
          )}
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

      {inputPorts.length > 0 && (
        <div className="flex flex-col items-start gap-1 px-1">
          {inputPorts.map((port) => (
            <div key={port.id} className="flex items-center gap-1">
              <Handle
                type="target"
                position={Position.Left}
                id={port.id}
                className={`w-3 h-3 ${getHandleColor(port)}`}
              />
              {port.name && (
                <span className="text-xs text-gray-600 bg-white px-1 rounded">{port.name}</span>
              )}
            </div>
          ))}
        </div>
      )}

      {outputPorts.length > 0 && (
        <div className="flex flex-col items-end gap-1 px-1">
          {outputPorts.map((port) => (
            <div key={port.id} className="flex items-center gap-1">
              {port.name && (
                <span className="text-xs text-gray-600 bg-white px-1 rounded">{port.name}</span>
              )}
              <Handle
                type="source"
                position={Position.Right}
                id={port.id}
                className={`w-3 h-3 ${getHandleColor(port)}`}
              />
            </div>
          ))}
        </div>
      )}

      {inputPorts.length === 0 && outputPorts.length === 0 && (
        <>
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
        </>
      )}
    </div>
  );
}