import { useCallback, useState, type MouseEvent } from 'react';
import type { NodeProps } from '@xyflow/react';
import { useTranslation } from 'react-i18next';
import { Play } from 'lucide-react';
import { NodeShell } from './NodeShell';
import { NodeHandles } from './NodeHandles';
import { wsClient } from '../lib/wsClient';
import { useRuntimeStore, selectNodeStatus } from '../store/runtimeStore';
import { notify } from '../store/notificationStore';
import type { CanvasNode } from './canvasTypes';

/** Inject node: the icon well is the trigger button, like Node-RED's inject tab. */
export function InjectNode({ id, data, selected }: NodeProps<CanvasNode>) {
  const { t } = useTranslation();
  const { label, node, metadata, outputs, flowId } = data;
  const [isInjecting, setIsInjecting] = useState(false);
  const [lastInjectionTime, setLastInjectionTime] = useState<string | null>(null);

  const category = metadata?.category || 'input';
  const status = useRuntimeStore(selectNodeStatus(flowId, id));

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
        setTimeout(() => setIsInjecting(false), 400);
      }
    },
    [isInjecting, node.config, id, flowId]
  );

  const title = isInjecting ? t('canvas.injecting') : lastInjectionTime ? t('canvas.lastInject', { time: lastInjectionTime }) : t('canvas.inject');

  return (
    <NodeShell
      category={category}
      label={label}
      icon={<Play className={`w-4 h-4 shrink-0 ${isInjecting ? 'animate-pulse' : ''}`} strokeWidth={2.25} fill="currentColor" aria-hidden="true" />}
      selected={selected}
      disabled={node.disabled}
      status={status}
      color={metadata?.color}
      title={metadata?.description || `Inject Node: ${metadata?.name || node.type}`}
      wellAction={{ onClick: (event) => void handleInject(event), label: t('canvas.inject'), title, disabled: isInjecting || node.disabled }}
    >
      <NodeHandles inputPorts={metadata?.inputs || []} outputPorts={outputs} />
    </NodeShell>
  );
}

export default InjectNode;
