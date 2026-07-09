import { useCallback, useState, type MouseEvent } from 'react';
import { NodeProps } from 'reactflow';
import type { FlowNode } from '../types/flow';
import type { NodeMetadata } from '../types/node';
import { useWebSocket } from '../hooks/useWebSocket';
import { NodeShell } from './NodeShell';
import { NodeHandles } from './NodeHandles';
import { NodeIcon } from './CategoryIcon';

interface NodeData {
  label: string;
  node: FlowNode;
  metadata: NodeMetadata | null;
  flowId?: string;
}

interface InjectNodeProps extends NodeProps {
  data: NodeData;
}

export function InjectNode({ data, selected }: InjectNodeProps) {
  const { label, node, metadata, flowId } = data;
  const { sendMessage } = useWebSocket();
  const [isInjecting, setIsInjecting] = useState(false);
  const [lastInjectionTime, setLastInjectionTime] = useState<string | null>(null);

  const category = metadata?.category || 'input';

  const handleInject = useCallback(
    async (event: MouseEvent) => {
      event.stopPropagation();
      if (isInjecting) return;

      setIsInjecting(true);
      try {
        const payload = node.config?.payload || { timestamp: new Date().toISOString(), source: 'manual-inject' };
        const effectiveFlowId = flowId || node.id.split('-')[0] || 'default-flow';

        await sendMessage('message:send', {
          flowId: effectiveFlowId,
          nodeId: node.id,
          payload,
        });

        setLastInjectionTime(new Date().toLocaleTimeString());
      } catch (error) {
        console.error('Failed to inject message:', error);
      } finally {
        setTimeout(() => setIsInjecting(false), 500); // Debounce rapid clicks
      }
    },
    [isInjecting, node.config, node.id, flowId, sendMessage]
  );

  return (
    <NodeShell
      category={category}
      label={label}
      icon={<NodeIcon icon={metadata?.icon} category={category} className="w-4 h-4 shrink-0" />}
      selected={selected}
      status={node.status}
      title={metadata?.description || `Inject Node: ${metadata?.name || node.type}`}
      action={
        <button
          onClick={handleInject}
          disabled={isInjecting}
          className={`shrink-0 w-5 h-5 flex items-center justify-center rounded-full text-[10px] leading-none ${
            isInjecting ? 'bg-white/30 cursor-not-allowed' : 'bg-white/25 hover:bg-white/40'
          }`}
          title={
            isInjecting
              ? 'Injecting...'
              : lastInjectionTime
                ? `Last: ${lastInjectionTime} — click to inject again`
                : 'Manually inject message'
          }
        >
          {isInjecting ? '●' : '▶'}
        </button>
      }
    >
      <NodeHandles inputPorts={metadata?.inputs || []} outputPorts={metadata?.outputs || []} />
    </NodeShell>
  );
}

export default InjectNode;
