import type { Node } from '@xyflow/react';
import type { FlowNode } from '../types/flow';
import type { NodeMetadata, Port } from '../types/node';

/** Data attached to every node rendered on the canvas. */
export interface CanvasNodeData extends Record<string, unknown> {
  label: string;
  node: FlowNode;
  metadata: NodeMetadata | null;
  /** Output ports as currently configured (see schema/ports). */
  outputs: Port[];
  flowId?: string;
}

export type CanvasNode = Node<CanvasNodeData>;
