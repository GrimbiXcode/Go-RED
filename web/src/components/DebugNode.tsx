import { Handle, Position, NodeProps } from 'reactflow';
import DOMPurify from 'dompurify';
import type { FlowNode } from '../types/flow';
import type { NodeMetadata, Port } from '../types/node';

interface NodeData {
  label: string;
  node: FlowNode;
  metadata: NodeMetadata | null;
  flowId?: string;
}

interface DebugNodeProps extends NodeProps {
  data: NodeData;
}

const getHandleColor = (port: Port) => {
  if (port.required) return 'bg-red-500';
  return 'bg-blue-500';
};

const getStatusColor = () => {
  // Debug nodes have a distinct color
  return 'bg-orange-500';
};

// Helper function to format a small preview of the last debug message
function formatDebugPreview(config: Record<string, any>): string {
  if (config?.prefix) {
    return `[${config.prefix}]`;
  }
  if (config?.enabled !== undefined) {
    return config.enabled ? 'Enabled' : 'Disabled';
  }
  return '';
}

export function DebugNode({ data, selected }: DebugNodeProps) {
  const { label, node, metadata } = data;

  const icon = metadata?.icon || '🐛';
  const color = metadata?.color || 'bg-orange-500';

  const inputPorts = metadata?.inputs || [];
  const outputPorts = metadata?.outputs || [];

  return (
    <div
      className={`rounded-md border-2 ${selected ? 'border-blue-500' : 'border-gray-300'} bg-white shadow-sm`}
      title={metadata?.description || `Debug Node: ${metadata?.name || node.type}`}
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
        {/* Debug node specific info */}
        <div className="text-xs text-gray-600">
          {formatDebugPreview(node.config || {})}
        </div>

        {/* Configuration summary */}
        {node.config && Object.keys(node.config).length > 0 && (
          <div className="text-xs text-gray-600 mt-1">
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

export default DebugNode;