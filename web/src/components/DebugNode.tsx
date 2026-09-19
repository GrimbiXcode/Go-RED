import type { NodeProps } from '@xyflow/react';
import { NodeShell } from './NodeShell';
import { NodeHandles } from './NodeHandles';
import { NodeIcon } from './CategoryIcon';
import { useRuntimeStore, selectNodeStatus } from '../store/runtimeStore';
import type { CanvasNode } from './canvasTypes';

export function DebugNode({ id, data, selected }: NodeProps<CanvasNode>) {
  const { label, node, metadata, flowId } = data;
  const category = metadata?.category || 'output';
  const status = useRuntimeStore(selectNodeStatus(flowId, id)) ?? node.status;

  return (
    <NodeShell
      category={category}
      label={label}
      icon={<NodeIcon icon={metadata?.icon} category={category} className="w-4 h-4 shrink-0" />}
      selected={selected}
      status={status}
      title={metadata?.description || `Debug Node: ${metadata?.name || node.type}`}
    >
      <NodeHandles inputPorts={metadata?.inputs || []} outputPorts={metadata?.outputs || []} />
    </NodeShell>
  );
}

export default DebugNode;
