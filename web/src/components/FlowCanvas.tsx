import { useCallback, useMemo, useRef } from 'react';
import ReactFlow, {
  ReactFlowProvider,
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
import type { Flow, FlowNode, NodeConnection, NodeRegistry } from '../types/flow';
import { NodeComponent } from './NodeComponent';
import { InjectNode } from './InjectNode';
import { DebugNode } from './DebugNode';

interface FlowCanvasProps {
  flow: Flow | null;
  availableNodeTypes?: NodeRegistry;
  onNodeSelect: (node: FlowNode) => void;
  onNodeDeselect: () => void;
  onAddNode: (nodeType: string, position: { x: number; y: number }) => void;
  onRemoveNode: (nodeId: string) => void;
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
  onAddConnection,
  onRemoveConnection,
}: FlowCanvasProps) {
  const reactFlowWrapper = useRef<HTMLDivElement>(null);
  const [nodes, setNodes, onNodesChange] = useNodesState([]);
  const [edges, setEdges, onEdgesChange] = useEdgesState([]);
  const { screenToFlowPosition } = useReactFlow();

  const nodeRegistry = availableNodeTypes || {};

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
        setNodes((nds) =>
          nds.map((n) => (n.id === node.id ? { ...n, position: node.position } : n))
        );
      }
    },
    [flow, setNodes]
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
      const position = screenToFlowPosition({
        x: event.clientX,
        y: event.clientY,
      });
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
      <ReactFlowProvider>
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
          fitView
          fitViewOptions={{ padding: 0.5 }}
          minZoom={0.1}
          maxZoom={4}
          defaultEdgeOptions={{ animated: true }}
        >
          <Background color="#f0f0f0" gap={16} />
          <Controls />
        </ReactFlow>
      </ReactFlowProvider>
    </div>
  );
}