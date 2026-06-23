import { useCallback, useState } from 'react';
import { Handle, Position, NodeProps } from 'reactflow';
import DOMPurify from 'dompurify';
import type { FlowNode } from '../types/flow';
import type { NodeMetadata, Port } from '../types/node';
import { useWebSocket } from '../hooks/useWebSocket';

interface NodeData {
  label: string;
  node: FlowNode;
  metadata: NodeMetadata | null;
  flowId?: string;
}

interface InjectNodeProps extends NodeProps {
  data: NodeData;
}

const getHandleColor = (port: Port) => {
  if (port.required) return 'bg-red-500';
  return 'bg-blue-500';
};

const getStatusColor = () => {
  // For now, inject nodes have a simple status indicator
  return 'bg-green-500';
};

export function InjectNode({ data, selected }: InjectNodeProps) {
  const { label, node, metadata, flowId } = data;
  const { sendMessage } = useWebSocket();
  const [isInjecting, setIsInjecting] = useState(false);
  const [lastInjectionTime, setLastInjectionTime] = useState<string | null>(null);

  const icon = metadata?.icon || '📥';
  const color = metadata?.color || 'bg-blue-500';

  const inputPorts = metadata?.inputs || [];
  const outputPorts = metadata?.outputs || [];

  const handleInject = useCallback(async () => {
    if (isInjecting) return;

    setIsInjecting(true);
    try {
      // Create a payload based on the node's config
      const payload = node.config?.payload || { timestamp: new Date().toISOString(), source: 'manual-inject' };
      
      // If we don't have a flowId from props, try to extract it from node.id as fallback
      const effectiveFlowId = flowId || node.id.split('-')[0] || 'default-flow';
      
      // Use the standard message:send type to inject messages
      await sendMessage('message:send', {
        flowId: effectiveFlowId,
        nodeId: node.id,
        payload: payload,
      });

      // Update last injection time
      setLastInjectionTime(new Date().toLocaleTimeString());
      
    } catch (error) {
      console.error('Failed to inject message:', error);
    } finally {
      setTimeout(() => setIsInjecting(false), 500); // Debounce to prevent rapid clicks
    }
  }, [isInjecting, node.config, node.id, flowId, sendMessage]);

  return (
    <div
      className={`rounded-md border-2 ${selected ? 'border-blue-500' : 'border-gray-300'} bg-white shadow-sm`}
      title={metadata?.description || `Inject Node: ${metadata?.name || node.type}`}
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
        {/* Inject button - prominent and accessible */}
        <button
          onClick={handleInject}
          disabled={isInjecting}
          className={`w-full flex items-center justify-center gap-2 py-2 px-4 rounded-md text-sm font-medium transition-colors ${
            isInjecting 
              ? 'bg-blue-400 text-white cursor-not-allowed' 
              : 'bg-blue-600 hover:bg-blue-700 text-white cursor-pointer'
          }`}
          title={isInjecting ? 'Injecting...' : 'Manually inject message'}
        >
          {isInjecting ? (
            <>
              <span className="animate-pulse">●</span>
              Injecting...
            </>
          ) : (
            <>
              <span>▶</span>
              Inject
            </>
          )}
        </button>

        {/* Last injection time */}
        {lastInjectionTime && (
          <div className="text-xs text-gray-500 mt-1 text-center">
            Last: {lastInjectionTime}
          </div>
        )}

        {/* Configuration summary */}
        {node.config && Object.keys(node.config).length > 0 && (
          <div className="text-xs text-gray-600 mt-2">
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

export default InjectNode;