import type { NodeProps } from '@xyflow/react';
import { NodeShell } from './NodeShell';
import { NodeHandles } from './NodeHandles';
import { NodeIcon } from './CategoryIcon';
import { useRuntimeStore, selectNodeStatus } from '../store/runtimeStore';
import type { CanvasNode } from './canvasTypes';

export function NodeComponent({ id, data, selected }: NodeProps<CanvasNode>) {
  const { label, node, metadata, outputs, flowId } = data;
  const category = metadata?.category || 'custom';
  const status = useRuntimeStore(selectNodeStatus(flowId, id));

  return (
    <NodeShell
      category={category}
      label={label}
      icon={<NodeIcon icon={metadata?.icon} category={category} className="w-4 h-4 shrink-0" />}
      selected={selected}
      status={status}
      color={metadata?.color}
      disabled={node.disabled}
      title={metadata?.description || `Node: ${metadata?.name || node.type}`}
    >
      <NodeHandles inputPorts={metadata?.inputs || []} outputPorts={outputs} />
    </NodeShell>
  );
}
