import { useCallback, useMemo, useRef } from 'react';
import React from 'react';
import ReactFlow, {
  Background,
  Controls,
  Edge,
  Node,
  Connection,
  useNodesState,
  useEdgesState,
  useReactFlow,
  NodeTypes,
  EdgeTypes,
} from 'reactflow';
import 'reactflow/dist/style.css';
import '../styles/reactflow-overrides.css';
import type { Flow, FlowNode, NodeConnection, NodeRegistry } from '../types/flow';
import type { NodeMetadata } from '../types/node';
import { NodeComponent } from './NodeComponent';
import { InjectNode } from './InjectNode';
import { DebugNode } from './DebugNode';

interface FlowCanvasProps {
  flow: Flow | null;
  // NodeEditor passes the flat NodeMetadata[] from useFlows() state, not a
  // NodeRegistry lookup object — build the registry from it below instead
  // of indexing the array by node.type (which previously always returned
  // undefined and silently fell back to 'custom' styling for every node).
  availableNodeTypes?: NodeMetadata[];
  onNodeSelect: (node: FlowNode) => void;
  onNodeDeselect: () => void;
  onAddNode: (nodeType: string, position: { x: number; y: number }) => void;
  onRemoveNode: (nodeId: string) => void;
  onUpdateNode: (nodeId: string, updates: Partial<FlowNode>) => void;
  onAddConnection: (connection: Omit<NodeConnection, 'id'>) => void;
  onRemoveConnection: (connectionId: string) => void;
}

const nodeTypeComponents: NodeTypes = {
  default: NodeComponent,
  inject: InjectNode,
  debug: DebugNode,
};

const edgeTypes: EdgeTypes = {};

function flowNodeToReactFlowNode(flowNode: FlowNode, nodeTypes: NodeRegistry, flowId?: string): Node {
  // Ensure position is always defined with default values if missing
  const position = flowNode.position || { x: 0, y: 0 };
  
  // Get metadata for this node type
  const metadata = nodeTypes[flowNode.type] || null;
  
  return {
    id: flowNode.id,
    type: flowNode.type || 'default',
    position: position,
    data: {
      label: flowNode.name || flowNode.type,
      node: flowNode,
      metadata: metadata,
      flowId: flowId,
    },
  };
}

function connectionToEdge(connection: NodeConnection): Edge {
  return {
    id: connection.id,
    source: connection.sourceNode,
    target: connection.targetNode,
    sourceHandle: connection.sourcePort,
    targetHandle: connection.targetPort,
  };
}

export function FlowCanvas({
  flow,
  availableNodeTypes,
  onNodeSelect,
  onNodeDeselect,
  onAddNode,
  onRemoveNode,
  onUpdateNode,
  onAddConnection,
  onRemoveConnection,
}: FlowCanvasProps) {
  const reactFlowWrapper = useRef<HTMLDivElement>(null);
  const [nodes, setNodes, onNodesChange] = useNodesState([]);
  const [edges, setEdges, onEdgesChange] = useEdgesState([]);
  const { screenToFlowPosition, fitView } = useReactFlow();
  const fittedFlowId = useRef<string | null>(null);

  const nodeRegistry: NodeRegistry = useMemo(
    () => Object.fromEntries((availableNodeTypes || []).map((nt) => [nt.type, nt])),
    [availableNodeTypes]
  );

  const flowNodes = useMemo(() => {
    if (!flow) return [];
    const nodes = Object.values(flow.nodes);
    console.log('[FlowCanvas] flowNodes - Converting nodes:', nodes.map(n => ({id: n.id, position: n.position})));
    return nodes.map(node => flowNodeToReactFlowNode(node, nodeRegistry, flow.id));
  }, [flow, nodeRegistry]);

  const flowEdges = useMemo(() => {
    if (!flow) return [];
    const connections = flow.connections || [];
    console.log('[FlowCanvas] flowEdges - Converting connections:', connections.length);
    return connections.map(connectionToEdge);
  }, [flow]);

  React.useEffect(() => {
    setNodes(flowNodes);
    setEdges(flowEdges);
  }, [flowNodes, flowEdges, setNodes, setEdges]);

  // Fit the view once per opened flow (e.g. switching tabs or loading a
  // freshly created flow) rather than relying on ReactFlow's `fitView`
  // prop, which re-fires the first time any node gets measured — including
  // when the first node is added to an already-open, previously empty
  // flow, which unexpectedly changes the zoom the user had set.
  React.useEffect(() => {
    if (!flow || fittedFlowId.current === flow.id) return;
    fittedFlowId.current = flow.id;
    const raf = requestAnimationFrame(() => {
      fitView({ padding: 0.5 });
    });
    return () => cancelAnimationFrame(raf);
  }, [flow, fitView]);

  const onNodeClick = useCallback(
    (_: React.MouseEvent, node: Node) => {
      const flowNode = (node.data as { node: FlowNode }).node;
      onNodeSelect(flowNode);
    },
    [onNodeSelect]
  );

  const onCanvasClick = useCallback(() => {
    onNodeDeselect();
  }, [onNodeDeselect]);

  const onNodeDragStop = useCallback(
    (_: React.MouseEvent, node: Node) => {
      if (flow) {
        onUpdateNode(node.id, { position: node.position });
      }
    },
    [flow, onUpdateNode]
  );

  const onConnect = useCallback(
    (params: Connection) => {
      if (!flow) return;
      const connection: Omit<NodeConnection, 'id'> = {
        sourceNode: params.source!,
        sourcePort: params.sourceHandle || 'output',
        targetNode: params.target!,
        targetPort: params.targetHandle || 'input',
      };
      onAddConnection(connection);
    },
    [flow, onAddConnection]
  );

  const onNodesDelete = useCallback(
    (deletedNodes: Node[]) => {
      deletedNodes.forEach((node) => {
        onRemoveNode(node.id);
      });
    },
    [onRemoveNode]
  );

  const onEdgesDelete = useCallback(
    (deletedEdges: Edge[]) => {
      deletedEdges.forEach((edge) => {
        onRemoveConnection(edge.id);
      });
    },
    [onRemoveConnection]
  );

  const onDrop = useCallback(
    (event: React.DragEvent<HTMLDivElement>) => {
      event.preventDefault();
      if (!reactFlowWrapper.current || !flow) return;
      const data = event.dataTransfer.getData('application/reactflow');
      if (!data) return;
      const { nodeType } = JSON.parse(data);
      // A node's `position` is its top-left corner in ReactFlow, but the
      // cursor is where the user is looking when they drop it. Without this
      // offset the node's top-left corner lands under the cursor instead of
      // its center, so the node appears shifted down-and-right of the drop
      // point. Half of the default node box (see --gr-node-min-width /
      // --gr-node-height in tailwind.css) keeps the drop point visually
      // centered on the node regardless of zoom.
      const rawPosition = screenToFlowPosition({
        x: event.clientX,
        y: event.clientY,
      });
      const position = {
        x: rawPosition.x - 50,
        y: rawPosition.y - 15,
      };
      onAddNode(nodeType, position);
    },
    [screenToFlowPosition, flow, onAddNode]
  );

  const onDragOver = useCallback((event: React.DragEvent<HTMLDivElement>) => {
    event.preventDefault();
    event.dataTransfer.dropEffect = 'move';
  }, []);

  if (!flow) {
    return (
      <div className="flex h-full w-full items-center justify-center bg-gray-100">
        <div className="text-gray-500">
          {flow === null ? 'Select a flow to edit' : 'No flow selected'}
        </div>
      </div>
    );
  }

  return (
    <div
      className="h-full w-full"
      ref={reactFlowWrapper}
      onDrop={onDrop}
      onDragOver={onDragOver}
    >
      <ReactFlow
        nodes={nodes}
        edges={edges}
        onNodesChange={onNodesChange}
        onEdgesChange={onEdgesChange}
        onConnect={onConnect}
        onNodeClick={onNodeClick}
        onPaneClick={onCanvasClick}
        onNodeDragStop={onNodeDragStop}
        onNodesDelete={onNodesDelete}
        onEdgesDelete={onEdgesDelete}
        nodeTypes={nodeTypeComponents}
        edgeTypes={edgeTypes}
        minZoom={0.1}
        maxZoom={4}
        defaultEdgeOptions={{ animated: false }}
      >
        <Background color="var(--gr-blue-200)" gap={20} size={1.5} />
        <Controls position="bottom-left" />
      </ReactFlow>
    </div>
  );
}