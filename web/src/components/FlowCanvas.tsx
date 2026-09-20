import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import {
  ReactFlow,
  Background,
  Controls,
  MiniMap,
  useNodesState,
  useEdgesState,
  useReactFlow,
  type Connection,
  type Edge,
  type NodeTypes,
  type OnDelete,
  type OnNodeDrag,
  type OnSelectionChangeParams,
} from '@xyflow/react';
import '@xyflow/react/dist/style.css';
import '../styles/reactflow-overrides.css';
import { useTranslation } from 'react-i18next';
import type { Flow, FlowNode, NodeConnection, NodeRegistry } from '../types/flow';
import type { NodeMetadata } from '../types/node';
import { useFlowStore } from '../store/flowStore';
import { useEditorStore } from '../store/editorStore';
import { NodeComponent } from './NodeComponent';
import { InjectNode } from './InjectNode';
import { DebugNode } from './DebugNode';
import type { CanvasNode } from './canvasTypes';
import { outputPortsFor } from '../schema/ports';
import { useRuntimeStore } from '../store/runtimeStore';
import { getCategoryColor } from '../utils/nodeCategories';

const nodeTypeComponents: NodeTypes = {
  default: NodeComponent,
  inject: InjectNode,
  debug: DebugNode,
};

export function flowNodeToCanvasNode(flowNode: FlowNode, nodeTypes: NodeRegistry, flowId?: string): CanvasNode {
  const position = flowNode.position || { x: 0, y: 0 };
  const metadata: NodeMetadata | null = nodeTypes[flowNode.type] || null;
  const registered = flowNode.type in nodeTypeComponents;

  return {
    id: flowNode.id,
    type: registered ? flowNode.type : 'default',
    position,
    data: {
      label: flowNode.name || metadata?.name || flowNode.type,
      node: flowNode,
      metadata,
      outputs: outputPortsFor(metadata, flowNode.config),
      flowId,
    },
  };
}

export function connectionToEdge(connection: NodeConnection): Edge {
  return {
    id: connection.id,
    source: connection.sourceNode,
    target: connection.targetNode,
    sourceHandle: connection.sourcePort,
    targetHandle: connection.targetPort,
  };
}

/**
 * Merges nodes derived from the flow into the current canvas node list,
 * keeping the on-screen position of nodes that are being dragged and the
 * current selection, so a store update arriving mid-drag never yanks a
 * node back or drops the selection.
 */
export function mergeCanvasNodes(current: CanvasNode[], incoming: CanvasNode[]): CanvasNode[] {
  const byId = new Map(current.map((node) => [node.id, node]));
  return incoming.map((node) => {
    const existing = byId.get(node.id);
    if (!existing) return node;
    return {
      ...node,
      position: existing.dragging ? existing.position : node.position,
      selected: existing.selected,
      dragging: existing.dragging,
    };
  });
}

export interface FlowCanvasProps {
  flow: Flow | null;
}

export function FlowCanvas({ flow }: FlowCanvasProps) {
  const { t } = useTranslation();
  const wrapper = useRef<HTMLDivElement>(null);
  const [nodes, setNodes, onNodesChange] = useNodesState<CanvasNode>([]);
  const [edges, setEdges, onEdgesChange] = useEdgesState<Edge>([]);
  const { screenToFlowPosition } = useReactFlow();

  const nodeTypes = useFlowStore((state) => state.nodeTypes);
  const addNode = useFlowStore((state) => state.addNode);
  const moveNodes = useFlowStore((state) => state.moveNodes);
  const removeNodes = useFlowStore((state) => state.removeNodes);
  const addConnection = useFlowStore((state) => state.addConnection);
  const removeConnections = useFlowStore((state) => state.removeConnections);
  const setSelection = useEditorStore((state) => state.setSelection);
  const openConfig = useEditorStore((state) => state.openConfig);

  const nodeRegistry: NodeRegistry = useMemo(
    () => Object.fromEntries(nodeTypes.map((nt) => [nt.type, nt])),
    [nodeTypes]
  );

  const flowNodes = useMemo(() => {
    if (!flow) return [];
    return Object.values(flow.nodes || {}).map((node) => flowNodeToCanvasNode(node, nodeRegistry, flow.id));
  }, [flow, nodeRegistry]);

  // Wires pulse briefly when flow:metrics reports that their source node
  // handled new messages.
  const metrics = useRuntimeStore((state) => (flow ? state.metrics[flow.id] : undefined));
  const lastCounts = useRef<Record<string, number>>({});
  const [pulsing, setPulsing] = useState<Record<string, number>>({});
  useEffect(() => {
    lastCounts.current = {};
    setPulsing({});
  }, [flow?.id]);
  useEffect(() => {
    if (!metrics) return;
    const changed: string[] = [];
    for (const [nodeId, counters] of Object.entries(metrics)) {
      const previous = lastCounts.current[nodeId];
      if (previous !== undefined && counters.messages > previous) changed.push(nodeId);
      lastCounts.current[nodeId] = counters.messages;
    }
    if (changed.length === 0) return;
    const stamp = Date.now();
    setPulsing((prev) => {
      const next = { ...prev };
      for (const id of changed) next[id] = stamp;
      return next;
    });
    setTimeout(() => {
      setPulsing((prev) => Object.fromEntries(Object.entries(prev).filter(([, at]) => at > stamp)));
    }, 700);
  }, [metrics]);

  const flowEdges = useMemo(
    () =>
      flow
        ? (flow.connections || []).map((connection) => ({
            ...connectionToEdge(connection),
            className: pulsing[connection.sourceNode] ? 'gr-pulse' : undefined,
          }))
        : [],
    [flow, pulsing]
  );

  useEffect(() => {
    setNodes((current) => mergeCanvasNodes(current, flowNodes));
    setEdges(flowEdges);
  }, [flowNodes, flowEdges, setNodes, setEdges]);

  const onSelectionChange = useCallback(
    ({ nodes: selected }: OnSelectionChangeParams) => {
      setSelection(selected.map((node) => node.id));
    },
    [setSelection]
  );

  const onNodeDoubleClick = useCallback(
    (_: React.MouseEvent, node: CanvasNode) => {
      openConfig(node.id);
    },
    [openConfig]
  );

  const onNodeDragStop: OnNodeDrag<CanvasNode> = useCallback(
    (_, node, draggedNodes) => {
      const moved = draggedNodes && draggedNodes.length > 0 ? draggedNodes : [node];
      moveNodes(moved.map((n) => ({ id: n.id, position: { x: n.position.x, y: n.position.y } })));
    },
    [moveNodes]
  );

  const onConnect = useCallback(
    (params: Connection) => {
      if (!params.source || !params.target) return;
      addConnection({
        sourceNode: params.source,
        sourcePort: params.sourceHandle || 'output',
        targetNode: params.target,
        targetPort: params.targetHandle || 'input',
      });
    },
    [addConnection]
  );

  const onDelete: OnDelete<CanvasNode, Edge> = useCallback(
    ({ nodes: deletedNodes, edges: deletedEdges }) => {
      if (deletedNodes.length > 0) removeNodes(deletedNodes.map((node) => node.id));
      if (deletedEdges.length > 0) removeConnections(deletedEdges.map((edge) => edge.id));
    },
    [removeNodes, removeConnections]
  );

  const onDrop = useCallback(
    (event: React.DragEvent<HTMLDivElement>) => {
      event.preventDefault();
      if (!wrapper.current || !flow) return;
      const data = event.dataTransfer.getData('application/reactflow');
      if (!data) return;
      let nodeType: string | undefined;
      try {
        nodeType = JSON.parse(data).nodeType;
      } catch {
        return;
      }
      if (!nodeType) return;
      const position = screenToFlowPosition({ x: event.clientX, y: event.clientY });
      addNode(nodeType, position);
    },
    [screenToFlowPosition, flow, addNode]
  );

  const onDragOver = useCallback((event: React.DragEvent<HTMLDivElement>) => {
    event.preventDefault();
    event.dataTransfer.dropEffect = 'move';
  }, []);

  if (!flow) {
    return (
      <div className="flex h-full w-full items-center justify-center bg-canvas">
        <div className="text-muted">{t('canvas.selectFlow')}</div>
      </div>
    );
  }

  const isEmpty = Object.keys(flow.nodes || {}).length === 0;

  return (
    <div className="relative h-full w-full bg-canvas" ref={wrapper} onDrop={onDrop} onDragOver={onDragOver} data-testid="flow-canvas">
      {isEmpty && (
        <div className="pointer-events-none absolute inset-0 z-[4] flex items-center justify-center" data-testid="canvas-empty">
          <div className="rounded-lg border border-dashed border-line-strong bg-panel/70 px-6 py-4 text-center text-sm text-muted backdrop-blur-sm">
            {t('canvas.emptyHint')}
          </div>
        </div>
      )}
      <ReactFlow<CanvasNode, Edge>
        nodes={nodes}
        edges={edges}
        onNodesChange={onNodesChange}
        onEdgesChange={onEdgesChange}
        onConnect={onConnect}
        onSelectionChange={onSelectionChange}
        onNodeDoubleClick={onNodeDoubleClick}
        onNodeDragStop={onNodeDragStop}
        onDelete={onDelete}
        deleteKeyCode={['Backspace', 'Delete']}
        nodeTypes={nodeTypeComponents}
        fitView
        fitViewOptions={{ padding: 0.5 }}
        minZoom={0.1}
        maxZoom={4}
        defaultEdgeOptions={{ animated: false }}
        proOptions={{ hideAttribution: true }}
      >
        <Background color="var(--grid-dot)" gap={20} size={1.5} />
        <Controls position="bottom-left" showInteractive={false} />
        <MiniMap
          position="bottom-right"
          pannable
          zoomable
          nodeColor={(node) => {
            const data = node.data as CanvasNode['data'] | undefined;
            return data?.metadata?.color || getCategoryColor(data?.metadata?.category || 'custom').fill;
          }}
          nodeStrokeWidth={0}
          maskColor="var(--selection)"
        />
      </ReactFlow>
    </div>
  );
}
