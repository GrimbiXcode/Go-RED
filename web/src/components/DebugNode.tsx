import { NodeProps } from 'reactflow';
import type { FlowNode } from '../types/flow';
import type { NodeMetadata } from '../types/node';
import { NodeShell } from './NodeShell';
import { NodeHandles } from './NodeHandles';
import { NodeIcon } from './CategoryIcon';

interface NodeData {
  label: string;
  node: FlowNode;
  metadata: NodeMetadata | null;
  flowId?: string;
}

interface DebugNodeProps extends NodeProps {
  data: NodeData;
}

export function DebugNode({ data, selected }: DebugNodeProps) {
  const { label, node, metadata } = data;
  const category = metadata?.category || 'output';

  return (
    <NodeShell
      category={category}
      label={label}
      icon={<NodeIcon icon={metadata?.icon} category={category} className="w-4 h-4 shrink-0" />}
      selected={selected}
      status={node.status}
      title={metadata?.description || `Debug Node: ${metadata?.name || node.type}`}
    >
      <NodeHandles inputPorts={metadata?.inputs || []} outputPorts={metadata?.outputs || []} />
    </NodeShell>
  );
}

export default DebugNode;
