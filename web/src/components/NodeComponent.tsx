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

interface NodeComponentProps extends NodeProps {
  data: NodeData;
}

export function NodeComponent({ data, selected }: NodeComponentProps) {
  const { label, node, metadata } = data;
  const category = metadata?.category || 'custom';

  return (
    <NodeShell
      category={category}
      label={label}
      icon={<NodeIcon icon={metadata?.icon} category={category} className="w-4 h-4 shrink-0" />}
      selected={selected}
      status={node.status}
      title={metadata?.description || `Node: ${metadata?.name || node.type}`}
    >
      <NodeHandles inputPorts={metadata?.inputs || []} outputPorts={metadata?.outputs || []} />
    </NodeShell>
  );
}
