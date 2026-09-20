import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import {
  ReactFlow,
  Background,
  Controls,
  MiniMap,
  SelectionMode,
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
import {
  AlignCenterHorizontal,
  AlignCenterVertical,
  AlignEndHorizontal,
  AlignEndVertical,
  AlignHorizontalDistributeCenter,
  AlignStartHorizontal,
  AlignStartVertical,
  AlignVerticalDistributeCenter,
  ClipboardPaste,
  Copy,
  CopyPlus,
  LayoutGrid,
  Maximize,
  Plus,
  Power,
  Settings,
  SquareDashedMousePointer,
  Trash2,
  Unlink,
} from 'lucide-react';
import type { Flow, FlowNode, NodeConnection, NodeRegistry } from '../types/flow';
import type { NodeMetadata } from '../types/node';
import { useFlowStore } from '../store/flowStore';
import { useEditorStore } from '../store/editorStore';
import { useRuntimeStore } from '../store/runtimeStore';
import { alignSelection, autoLayoutFlow, copySelection, deleteSelection, distributeSelection, duplicateSelection, pasteClipboard, toggleSelectionDisabled } from '../store/editorActions';
import { hasClipboard } from '../lib/clipboard';
import { GRID } from '../lib/layout';
import { NodeComponent } from './NodeComponent';
import { InjectNode } from './InjectNode';
import { DebugNode } from './DebugNode';
import { ContextMenu, type ContextMenuItem } from './ContextMenu';
import { QuickAdd } from './QuickAdd';
import type { CanvasNode } from './canvasTypes';
import { outputPortsFor } from '../schema/ports';
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
      // xyflow hides a node until it has measured it; a store update must not
      // throw the measurement away, or the node blinks (and can stay hidden
      // when the re-measure races the next update).
      measured: existing.measured,
    };
  });
}

/** The wire under a screen point, if any (edges carry their id as data-id). */
export function edgeIdAt(clientX: number, clientY: number): string | null {
  if (typeof document === 'undefined' || typeof document.elementsFromPoint !== 'function') return null;
  for (const element of document.elementsFromPoint(clientX, clientY)) {
    const edge = element.closest('.react-flow__edge');
    if (edge) return edge.getAttribute('data-id');
  }
  return null;
}

function isEditableTarget(target: EventTarget | null): boolean {
  if (!(target instanceof HTMLElement)) return false;
  const tag = target.tagName;
  return tag === 'INPUT' || tag === 'TEXTAREA' || tag === 'SELECT' || target.isContentEditable;
}

interface MenuState {
  x: number;
  y: number;
  kind: 'node' | 'edge' | 'pane';
  id?: string;
  flowPosition?: { x: number; y: number };
}

export interface FlowCanvasProps {
  flow: Flow | null;
}

export function FlowCanvas({ flow }: FlowCanvasProps) {
  const { t } = useTranslation();
  const wrapper = useRef<HTMLDivElement>(null);
  const [nodes, setNodes, onNodesChange] = useNodesState<CanvasNode>([]);
  const [edges, setEdges, onEdgesChange] = useEdgesState<Edge>([]);
  const { screenToFlowPosition, zoomIn, zoomOut, fitView } = useReactFlow();
  const [menu, setMenu] = useState<MenuState | null>(null);
  const [quickAdd, setQuickAdd] = useState<{ x: number; y: number } | null>(null);

  const nodeTypes = useFlowStore((state) => state.nodeTypes);
  const addNode = useFlowStore((state) => state.addNode);
  const moveNodes = useFlowStore((state) => state.moveNodes);
  const nudgeNodes = useFlowStore((state) => state.nudgeNodes);
  const removeNodes = useFlowStore((state) => state.removeNodes);
  const addConnection = useFlowStore((state) => state.addConnection);
  const removeConnections = useFlowStore((state) => state.removeConnections);
  const insertNodeOnEdge = useFlowStore((state) => state.insertNodeOnEdge);
  const spliceNodeIntoEdge = useFlowStore((state) => state.spliceNodeIntoEdge);
  const selectedNodeIds = useEditorStore((state) => state.selectedNodeIds);
  const setSelection = useEditorStore((state) => state.setSelection);
  const openConfig = useEditorStore((state) => state.openConfig);
  const pendingSelection = useEditorStore((state) => state.pendingSelection);
  const clearPendingSelection = useEditorStore((state) => state.clearPendingSelection);

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
    setMenu(null);
    setQuickAdd(null);
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

  // A paste or duplicate asks for its new nodes to be selected.
  useEffect(() => {
    if (!pendingSelection) return;
    const wanted = new Set(pendingSelection);
    setNodes((current) => current.map((node) => ({ ...node, selected: wanted.has(node.id) })));
    clearPendingSelection();
  }, [pendingSelection, setNodes, clearPendingSelection]);

  const selectAll = useCallback(() => {
    setNodes((current) => current.map((node) => ({ ...node, selected: true })));
  }, [setNodes]);

  const selectOnly = useCallback(
    (id: string) => {
      if (selectedNodeIds.includes(id)) return;
      setNodes((current) => current.map((node) => ({ ...node, selected: node.id === id })));
      setSelection([id]);
    },
    [selectedNodeIds, setNodes, setSelection]
  );

  // Canvas-local keys: arrows nudge the selection, +/- zoom, Ctrl+A selects all.
  useEffect(() => {
    const onKeyDown = (event: KeyboardEvent) => {
      if (isEditableTarget(event.target) || !flow) return;
      const mod = event.ctrlKey || event.metaKey;
      if (mod && !event.shiftKey && event.key.toLowerCase() === 'a') {
        event.preventDefault();
        selectAll();
        return;
      }
      if (mod) return;
      if (event.key === '+' || event.key === '=') {
        event.preventDefault();
        void zoomIn();
        return;
      }
      if (event.key === '-') {
        event.preventDefault();
        void zoomOut();
        return;
      }
      const step = event.shiftKey ? GRID : 1;
      const delta: Record<string, [number, number]> = { ArrowLeft: [-step, 0], ArrowRight: [step, 0], ArrowUp: [0, -step], ArrowDown: [0, step] };
      const move = delta[event.key];
      if (move && selectedNodeIds.length > 0) {
        event.preventDefault();
        nudgeNodes(selectedNodeIds, move[0], move[1]);
      }
    };
    window.addEventListener('keydown', onKeyDown);
    return () => window.removeEventListener('keydown', onKeyDown);
  }, [flow, selectedNodeIds, nudgeNodes, selectAll, zoomIn, zoomOut]);

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
    (event, node, draggedNodes) => {
      const moved = draggedNodes && draggedNodes.length > 0 ? draggedNodes : [node];
      moveNodes(moved.map((n) => ({ id: n.id, position: { x: n.position.x, y: n.position.y } })));
      // An unconnected node dropped on a wire is spliced into it.
      if (moved.length === 1 && flow && 'clientX' in event && !flow.connections.some((c) => c.sourceNode === node.id || c.targetNode === node.id)) {
        const edgeId = edgeIdAt(event.clientX, event.clientY);
        if (edgeId) spliceNodeIntoEdge(node.id, edgeId);
      }
    },
    [moveNodes, flow, spliceNodeIntoEdge]
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
      const edgeId = edgeIdAt(event.clientX, event.clientY);
      if (edgeId && insertNodeOnEdge(nodeType, edgeId, position)) return;
      addNode(nodeType, position);
    },
    [screenToFlowPosition, flow, addNode, insertNodeOnEdge]
  );

  const onDragOver = useCallback((event: React.DragEvent<HTMLDivElement>) => {
    event.preventDefault();
    event.dataTransfer.dropEffect = 'move';
  }, []);

  const onNodeContextMenu = useCallback(
    (event: React.MouseEvent, node: CanvasNode) => {
      event.preventDefault();
      selectOnly(node.id);
      setMenu({ x: event.clientX, y: event.clientY, kind: 'node', id: node.id });
    },
    [selectOnly]
  );

  const onSelectionContextMenu = useCallback((event: React.MouseEvent) => {
    event.preventDefault();
    setMenu({ x: event.clientX, y: event.clientY, kind: 'node' });
  }, []);

  const onEdgeContextMenu = useCallback((event: React.MouseEvent, edge: Edge) => {
    event.preventDefault();
    setMenu({ x: event.clientX, y: event.clientY, kind: 'edge', id: edge.id });
  }, []);

  const onPaneContextMenu = useCallback(
    (event: React.MouseEvent | MouseEvent) => {
      event.preventDefault();
      setMenu({ x: event.clientX, y: event.clientY, kind: 'pane', flowPosition: screenToFlowPosition({ x: event.clientX, y: event.clientY }) });
    },
    [screenToFlowPosition]
  );

  const onWrapperDoubleClick = useCallback((event: React.MouseEvent<HTMLDivElement>) => {
    const target = event.target as HTMLElement;
    if (!target.closest('.react-flow__pane') || target.closest('.react-flow__node, .react-flow__edge, .react-flow__controls, .react-flow__minimap')) return;
    setQuickAdd({ x: event.clientX, y: event.clientY });
  }, []);

  const menuItems = useMemo((): ContextMenuItem[] => {
    if (!menu || !flow) return [];
    if (menu.kind === 'edge') {
      return [{ id: 'delete-wire', label: t('context.deleteWire'), icon: <Unlink />, danger: true, onSelect: () => removeConnections([menu.id!]) }];
    }
    if (menu.kind === 'pane') {
      const at = { x: menu.x, y: menu.y };
      return [
        { id: 'add', label: t('context.addNode'), icon: <Plus />, onSelect: () => setQuickAdd(at) },
        { id: 'paste', label: t('context.paste'), icon: <ClipboardPaste />, shortcut: 'Ctrl+V', disabled: !hasClipboard(), onSelect: () => pasteClipboard(menu.flowPosition) },
        { id: 'sep1', label: '', separator: true },
        { id: 'select-all', label: t('context.selectAll'), icon: <SquareDashedMousePointer />, shortcut: 'Ctrl+A', onSelect: selectAll },
        { id: 'auto-layout', label: t('context.autoLayout'), icon: <LayoutGrid />, onSelect: () => autoLayoutFlow(nodeTypes) },
        { id: 'fit', label: t('context.fitView'), icon: <Maximize />, onSelect: () => void fitView({ padding: 0.3 }) },
      ];
    }
    const ids = selectedNodeIds.length > 0 ? selectedNodeIds : menu.id ? [menu.id] : [];
    const allDisabled = ids.length > 0 && ids.every((id) => flow.nodes[id]?.disabled);
    const items: ContextMenuItem[] = [];
    if (ids.length === 1) items.push({ id: 'configure', label: t('context.configure'), icon: <Settings />, onSelect: () => openConfig(ids[0]) });
    items.push(
      { id: 'duplicate', label: t('context.duplicate'), icon: <CopyPlus />, shortcut: 'Ctrl+D', onSelect: () => duplicateSelection() },
      { id: 'copy', label: t('context.copy'), icon: <Copy />, shortcut: 'Ctrl+C', onSelect: () => copySelection() },
      { id: 'toggle', label: allDisabled ? t('context.enable') : t('context.disable'), icon: <Power />, onSelect: toggleSelectionDisabled }
    );
    if (ids.length >= 2) {
      items.push(
        { id: 'sep-align', label: '', separator: true },
        { id: 'align-left', label: t('context.alignLeft'), icon: <AlignStartVertical />, onSelect: () => alignSelection('left') },
        { id: 'align-center', label: t('context.alignCenter'), icon: <AlignCenterVertical />, onSelect: () => alignSelection('centerX') },
        { id: 'align-right', label: t('context.alignRight'), icon: <AlignEndVertical />, onSelect: () => alignSelection('right') },
        { id: 'align-top', label: t('context.alignTop'), icon: <AlignStartHorizontal />, onSelect: () => alignSelection('top') },
        { id: 'align-middle', label: t('context.alignMiddle'), icon: <AlignCenterHorizontal />, onSelect: () => alignSelection('centerY') },
        { id: 'align-bottom', label: t('context.alignBottom'), icon: <AlignEndHorizontal />, onSelect: () => alignSelection('bottom') }
      );
    }
    if (ids.length >= 3) {
      items.push(
        { id: 'distribute-h', label: t('context.distributeH'), icon: <AlignHorizontalDistributeCenter />, onSelect: () => distributeSelection('horizontal') },
        { id: 'distribute-v', label: t('context.distributeV'), icon: <AlignVerticalDistributeCenter />, onSelect: () => distributeSelection('vertical') }
      );
    }
    items.push({ id: 'sep-delete', label: '', separator: true }, { id: 'delete', label: t('context.delete'), icon: <Trash2 />, shortcut: 'Del', danger: true, onSelect: deleteSelection });
    return items;
  }, [menu, flow, selectedNodeIds, t, removeConnections, selectAll, nodeTypes, fitView, openConfig]);

  if (!flow) {
    return (
      <div className="flex h-full w-full items-center justify-center bg-canvas">
        <div className="text-muted">{t('canvas.selectFlow')}</div>
      </div>
    );
  }

  const isEmpty = Object.keys(flow.nodes || {}).length === 0;

  return (
    <div
      className="relative h-full w-full bg-canvas"
      ref={wrapper}
      onDrop={onDrop}
      onDragOver={onDragOver}
      onDoubleClick={onWrapperDoubleClick}
      data-testid="flow-canvas"
    >
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
        onNodeContextMenu={onNodeContextMenu}
        onSelectionContextMenu={onSelectionContextMenu}
        onEdgeContextMenu={onEdgeContextMenu}
        onPaneContextMenu={onPaneContextMenu}
        onDelete={onDelete}
        deleteKeyCode={['Backspace', 'Delete']}
        nodeTypes={nodeTypeComponents}
        fitView
        fitViewOptions={{ padding: 0.5 }}
        minZoom={0.1}
        maxZoom={4}
        snapToGrid
        snapGrid={[GRID, GRID]}
        selectionOnDrag
        selectionMode={SelectionMode.Partial}
        panOnDrag={[1, 2]}
        panActivationKeyCode="Space"
        zoomOnDoubleClick={false}
        disableKeyboardA11y
        defaultEdgeOptions={{ animated: false }}
        proOptions={{ hideAttribution: true }}
      >
        <Background color="var(--grid-dot)" gap={GRID} size={1.5} />
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

      {menu && <ContextMenu x={menu.x} y={menu.y} items={menuItems} onClose={() => setMenu(null)} />}
      {quickAdd && (
        <QuickAdd
          x={quickAdd.x}
          y={quickAdd.y}
          nodeTypes={nodeTypes}
          onPick={(type) => {
            addNode(type, screenToFlowPosition(quickAdd));
            setQuickAdd(null);
          }}
          onClose={() => setQuickAdd(null)}
        />
      )}
    </div>
  );
}
