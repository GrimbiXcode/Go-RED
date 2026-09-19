import { useCallback, useState, type MouseEvent } from 'react';
import type { NodeProps } from '@xyflow/react';
import { useTranslation } from 'react-i18next';
import { NodeShell } from './NodeShell';
import { NodeHandles } from './NodeHandles';
import { NodeIcon } from './CategoryIcon';
import { wsClient } from '../lib/wsClient';
import { useRuntimeStore, selectNodeStatus } from '../store/runtimeStore';
import { notify } from '../store/notificationStore';
import type { CanvasNode } from './canvasTypes';

export function InjectNode({ id, data, selected }: NodeProps<CanvasNode>) {
  const { t } = useTranslation();
  const { label, node, metadata, flowId } = data;
  const [isInjecting, setIsInjecting] = useState(false);
  const [lastInjectionTime, setLastInjectionTime] = useState<string | null>(null);

  const category = metadata?.category || 'input';
  const status = useRuntimeStore(selectNodeStatus(flowId, id)) ?? node.status;

  const handleInject = useCallback(
    async (event: MouseEvent) => {
      event.stopPropagation();
      if (isInjecting || !flowId) return;

      setIsInjecting(true);
      try {
        const payload = node.config?.payload || { timestamp: new Date().toISOString(), source: 'manual-inject' };
        await wsClient.send('message:send', { flowId, nodeId: id, payload });
        setLastInjectionTime(new Date().toLocaleTimeString());
      } catch (error) {
        notify('error', error instanceof Error ? error.message : String(error));
      } finally {
        setTimeout(() => setIsInjecting(false), 500);
      }
    },
    [isInjecting, node.config, id, flowId]
  );

  return (
    <NodeShell
      category={category}
      label={label}
      icon={<NodeIcon icon={metadata?.icon} category={category} className="w-4 h-4 shrink-0" />}
      selected={selected}
      status={status}
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
              ? t('canvas.injecting')
              : lastInjectionTime
                ? t('canvas.lastInject', { time: lastInjectionTime })
                : t('canvas.inject')
          }
          aria-label={t('canvas.inject')}
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
