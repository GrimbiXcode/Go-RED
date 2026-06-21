# Go-RED Components Guidelines

This file contains **component-specific** guidelines for React components in `web/src/components/`.

---

## Overview

The `components/` directory contains all **React components** for the Go-RED Web UI. Components are organized by functionality and reusability:

```
components/
├── index.ts                 # Re-exports all components
├── App.tsx                  # Root application component
├── FlowEditor.tsx           # Main flow editor container
├── FlowProvider.tsx         # Zustand store provider
├── FlowCanvas.tsx           # ReactFlow canvas component
├── NodeComponent.tsx        # Individual node rendering
├── NodePalette.tsx          # Available nodes panel
├── Sidebar.tsx              # Left sidebar with flows and nodes
├── Toolbar.tsx              # Top toolbar with actions
├── MessageLogPanel.tsx      # Message logging display
├── WebSocketStatus.tsx      # WebSocket connection status
├── ExportModal.tsx          # Flow export modal dialog
├── ImportModal.tsx          # Flow import modal dialog
├── NodeConfigModal.tsx      # Node configuration modal
└── ToastNotification.tsx    # Toast notification system
```

---

## Component Hierarchy

```
App
├── FlowProvider (Zustand context)
│   └── FlowEditor (main container)
│       ├── Sidebar (left panel)
│       │   ├── FlowList (list of all flows)
│       │   └── NodePalette (draggable node types)
│       ├── Toolbar (top bar)
│       │   ├── SaveButton
│       │   ├── DeployButton
│       │   ├── UndeployButton
│       │   ├── ExportButton (opens ExportModal)
│       │   └── ImportButton (opens ImportModal)
│       ├── FlowCanvas (ReactFlow)
│       │   ├── Background
│       │   ├── Controls
│       │   ├── MiniMap
│       │   └── NodeComponent (per node)
│       └── MessageLogPanel (right panel)
│           └── MessageLogEntry (per message)
└── ToastProvider (toast context)
    └── ToastNotification (per toast)
```

---

## Component-Specific Guidelines

### FlowEditor.tsx

**Purpose**: Main container component that orchestrates the entire flow editor UI.

**Responsibilities**:
- Coordinate between all major components
- Manage WebSocket connection
- Handle global state via Zustand
- Provide context to child components

**Key Hooks Used**:
- `useFlows()` - Flow state management
- `useWebSocket()` - WebSocket connection
- `useMessageLog()` - Message log state

**Example Structure**:
```typescript
import { FlowList } from './FlowList';
import { NodePalette } from './NodePalette';
import { Toolbar } from './Toolbar';
import { FlowCanvas } from './FlowCanvas';
import { MessageLogPanel } from './MessageLogPanel';
import { WebSocketStatus } from './WebSocketStatus';

export function FlowEditor() {
    const { flows, activeFlowId, activeFlow, setActiveFlowId } = useFlows();
    const { isConnected } = useWebSocket('/ws');
    const { messages, clearMessages } = useMessageLog();
    
    if (!isConnected) {
        return (
            <div className="flex h-screen items-center justify-center">
                <WebSocketStatus />
            </div>
        );
    }
    
    return (
        <div className="flex h-screen w-full bg-gray-50">
            <Sidebar 
                flows={flows} 
                activeFlowId={activeFlowId}
                onSelectFlow={setActiveFlowId}
            />
            <div className="flex-1 flex flex-col">
                <Toolbar activeFlow={activeFlow} />
                <FlowCanvas flow={activeFlow} />
            </div>
            <MessageLogPanel 
                messages={messages}
                onClear={clearMessages}
            />
        </div>
    );
}
```

**Testing Focus**:
- Integration with child components
- WebSocket connection management
- State synchronization

---

### FlowProvider.tsx

**Purpose**: Provide Zustand store context to all child components.

**Responsibilities**:
- Initialize Zustand store
- Provide store to component tree
- Enable DevTools for debugging

**Example Implementation**:
```typescript
import { create } from 'zustand';
import { devtools } from 'zustand/middleware';
import { Flow, Node, Connection, FlowStatus, NodeType } from '../types';

interface FlowStore {
    // State
    flows: Flow[];
    activeFlowId: string | null;
    nodeTypes: NodeType[];
    isLoading: boolean;
    error: string | null;
    
    // Actions
    setFlows: (flows: Flow[]) => void;
    setActiveFlowId: (flowId: string | null) => void;
    setNodeTypes: (nodeTypes: NodeType[]) => void;
    // ... more actions
}

const useFlowStore = create<FlowStore>()(
    devtools((set) => ({
        flows: [],
        activeFlowId: null,
        nodeTypes: [],
        isLoading: false,
        error: null,
        
        setFlows: (flows) => set({ flows }),
        setActiveFlowId: (flowId) => set({ activeFlowId: flowId }),
        setNodeTypes: (nodeTypes) => set({ nodeTypes }),
        // ... more actions
    }), { name: 'FlowStore' })
);

// Selector hooks
export const useFlows = () => useFlowStore(state => state.flows);
export const useActiveFlowId = () => useFlowStore(state => state.activeFlowId);
export const useActiveFlow = () => {
    const { activeFlowId, flows } = useFlowStore();
    return flows.find(f => f.id === activeFlowId);
};
export const useNodeTypes = () => useFlowStore(state => state.nodeTypes);

// Provider component
export function FlowProvider({ children }: { children: React.ReactNode }) {
    return <>{children}</>;
    // The store is already global via create(), but we might add local providers here
}
```

**Testing Focus**:
- Store initialization
- Action execution
- Selector functions
- DevTools integration

---

### FlowCanvas.tsx

**Purpose**: Render the flow visualization and editing area using ReactFlow.

**Responsibilities**:
- Render nodes and connections
- Handle node drag-and-drop
- Manage connections between nodes
- Convert between domain model and ReactFlow model
- Handle user interactions (node selection, panning, zooming)

**Key Features**:
- Custom node types via `NodeComponent`
- Background grid
- Controls (zoom, pan)
- Mini-map for navigation

**Example Implementation**:
```typescript
import ReactFlow, {
    ReactFlowProvider,
    Background,
    Controls,
    MiniMap,
    useNodesState,
    useEdgesState,
    addEdge,
    Node as RFNode,
    Edge as RFEdge,
    OnConnect,
    OnEdgesChange,
    OnNodesChange,
} from 'reactflow';
import { Node as DomainNode, Connection as DomainConnection, Flow } from '../types';
import { NodeComponent } from './NodeComponent';
import { useCallback, useEffect } from 'react';

// Custom node types
const nodeTypes = {
    custom: NodeComponent,
};

// Color scheme
const colors = {
    background: '#f0f0f0',
    primary: '#3b82f6',
    secondary: '#10b981',
    error: '#ef4444',
};

function convertToReactFlowNodes(nodes: Record<string, DomainNode>): RFNode[] {
    return Object.values(nodes).map(node => ({
        id: node.id,
        type: 'custom',
        position: { x: node.x, y: node.y },
        data: { label: node.name || node.type, node },
        selected: false,
        dragging: false,
    }));
}

function convertToReactFlowEdges(connections: DomainConnection[]): RFEdge[] {
    return connections.map(conn => ({
        id: conn.id,
        source: conn.sourceNode,
        target: conn.targetNode,
        sourceHandle: conn.sourcePort,
        targetHandle: conn.targetPort,
        animated: false,
        style: { stroke: '#666', strokeWidth: 2 },
    }));
}

export function FlowCanvas({ flow }: { flow?: Flow }) {
    const [nodes, setNodes, onNodesChange] = useNodesState([]);
    const [edges, setEdges, onEdgesChange] = useEdgesState([]);
    
    // Convert flow to ReactFlow format
    useEffect(() => {
        if (flow) {
            setNodes(convertToReactFlowNodes(flow.nodes));
            setEdges(convertToReactFlowEdges(flow.connections));
        }
    }, [flow, setNodes, setEdges]);
    
    // Handle new connections
    const onConnect: OnConnect = useCallback((params) => {
        setEdges(prev => addEdge(params, prev));
    }, [setEdges]);
    
    // Handle node changes
    const onNodeChanges: OnNodesChange = useCallback((changes) => {
        onNodesChange(changes);
        
        // Update positions in flow state
        changes.forEach(change => {
            if (change.type === 'position' && change.position) {
                // Update flow store with new position
            }
        });
    }, [onNodesChange]);
    
    return (
        <div className="flex-1" data-testid="flow-canvas">
            <ReactFlow
                nodes={nodes}
                edges={edges}
                onNodesChange={onNodeChanges}
                onEdgesChange={onEdgesChange}
                onConnect={onConnect}
                nodeTypes={nodeTypes}
                defaultViewport={{ x: 0, y: 0, zoom: 1 }}
                fitView
                fitViewOptions={{ padding: 0.5, duration: 500 }}
                minZoom={0.1}
                maxZoom={4}
            >
                <Background color={colors.background} gap={16} />
                <Controls showInteractive={true} />
                <MiniMap nodeColor={(n) => (n as any).data?.node?.status?.state === 'error' ? colors.error : colors.primary} />
            </ReactFlow>
        </div>
    );
}

export function FlowCanvasWithProvider({ flow }: { flow?: Flow }) {
    return (
        <ReactFlowProvider>
            <FlowCanvas flow={flow} />
        </ReactFlowProvider>
    );
}
```

**Testing Focus**:
- Node rendering
- Connection creation
- Position updates
- Zoom and pan functionality
- ReactFlow integration

---

### NodeComponent.tsx

**Purpose**: Render individual nodes in the flow canvas.

**Responsibilities**:
- Display node name and type
- Show node configuration
- Display node status
- Render input/output ports
- Handle node selection
- Show node icon

**Key Features**:
- Dynamic styling based on node type and status
- Custom content for different node types
- Port rendering for connections
- Selection state

**Example Implementation**:
```typescript
import { memo } from 'react';
import { Node as RFNode, useNodeId } from 'reactflow';
import { Node as DomainNode, NodeType, NodeStatus } from '../types';
import { useNodeType } from '../hooks/useNodeTypes';

interface NodeComponentProps {
    data: {
        label: string;
        node: DomainNode;
    };
    selected?: boolean;
}

// Status colors
const statusColors = {
    idle: 'bg-transparent',
    processing: 'bg-yellow-500',
    error: 'bg-red-500',
};

// Get icon component based on icon name
function getIcon(iconName: string) {
    const icons: Record<string, React.ReactNode> = {
        inject: '➕',
        debug: '🐛',
        function: '📝',
        default: '🔧',
    };
    return icons[iconName] || icons.default;
}

export const NodeComponent = memo(({ data, selected }: NodeComponentProps) => {
    const nodeId = useNodeId();
    const { node } = data;
    const nodeType = useNodeType(node.type);
    
    // Determine background color
    const bgColor = nodeType?.color || '#3b82f6';
    
    // Determine status color
    const statusColor = statusColors[node.status?.state || 'idle'];
    
    return (
        <div 
            className={`rounded shadow-md transition-all ${selected ? 'ring-2 ring-blue-500' : ''}`}
            style={{ backgroundColor: bgColor }}
        >
            {/* Header */}
            <div className="p-2">
                <div className="flex items-center gap-2">
                    <span className="text-white">{getIcon(nodeType?.icon || '')}</span>
                    <span className="text-white font-medium text-sm">{node.name || node.type}</span>
                </div>
            </div>
            
            {/* Content */}
            <div className="px-2 pb-1">
                <NodeContent node={node} />
            </div>
            
            {/* Status indicator */}
            <div className={`h-1 ${statusColor}`} />
            
            {/* Ports */}
            <NodePorts node={node} nodeType={nodeType} />
        </div>
    );
});

// Sub-components
function NodeContent({ node }: { node: DomainNode }) {
    if (!node) return null;
    
    switch (node.type) {
        case 'debug':
            return <DebugNodeContent node={node} />;
        case 'function':
            return <FunctionNodeContent node={node} />;
        case 'inject':
            return <InjectNodeContent node={node} />;
        default:
            return <DefaultNodeContent node={node} />;
    }
}

function DebugNodeContent({ node }: { node: DomainNode }) {
    return (
        <div className="text-xs text-white/80">
            Output: {node.config?.output || 'all'}
        </div>
    );
}

function FunctionNodeContent({ node }: { node: DomainNode }) {
    const func = node.config?.function;
    const preview = func ? func.substring(0, 30) + (func.length > 30 ? '...' : '') : 'No function';
    return (
        <div className="text-xs text-white/80 truncate" title={func || ''}>
            {preview}
        </div>
    );
}

function InjectNodeContent({ node }: { node: DomainNode }) {
    const repeat = node.config?.repeat;
    return (
        <div className="text-xs text-white/80">
            {repeat ? `Every ${repeat}s` : 'Manual'}
        </div>
    );
}

function DefaultNodeContent({ node }: { node: DomainNode }) {
    const configCount = Object.keys(node.config || {}).length;
    return (
        <div className="text-xs text-white/80">
            {configCount > 0 ? `${configCount} configs` : 'No config'}
        </div>
    );
}

function NodePorts({ node, nodeType }: { node: DomainNode; nodeType?: NodeType }) {
    const inputPorts = nodeType?.inputPorts || ['input'];
    const outputPorts = nodeType?.outputPorts || ['output'];
    
    return (
        <div className="flex justify-between px-1 py-1">
            {/* Input ports */}
            <div className="flex gap-1">
                {inputPorts.map(port => (
                    <div 
                        key={port} 
                        className="w-3 h-3 bg-white/50 rounded-full"
                        title={port}
                    />
                ))}
            </div>
            
            {/* Output ports */}
            <div className="flex gap-1">
                {outputPorts.map(port => (
                    <div 
                        key={port} 
                        className="w-3 h-3 bg-white/50 rounded-full"
                        title={port}
                    />
                ))}
            </div>
        </div>
    );
}
```

**Testing Focus**:
- Node rendering with different types
- Status display
- Port rendering
- Custom content for node types
- Selection state

---

### Sidebar.tsx

**Purpose**: Left sidebar containing flow list and node palette.

**Responsibilities**:
- Display list of all flows
- Show active flow
- Allow flow selection
- Display available node types
- Enable node dragging to canvas

**Example Implementation**:
```typescript
import { useState } from 'react';
import { FlowList } from './FlowList';
import { NodePalette } from './NodePalette';
import { Flow } from '../types';

export interface SidebarProps {
    flows: Flow[];
    activeFlowId: string | null;
    onSelectFlow: (flowId: string | null) => void;
}

type Tab = 'flows' | 'nodes';

export function Sidebar({ flows, activeFlowId, onSelectFlow }: SidebarProps) {
    const [activeTab, setActiveTab] = useState<Tab>('flows');
    
    return (
        <div className="w-64 bg-white border-r border-gray-200 flex flex-col h-full">
            {/* Tabs */}
            <div className="flex border-b border-gray-200">
                <button
                    className={`flex-1 p-3 text-sm font-medium ${activeTab === 'flows' ? 'bg-blue-50 text-blue-600' : 'text-gray-500 hover:bg-gray-50'}`}
                    onClick={() => setActiveTab('flows')}
                >
                    Flows
                </button>
                <button
                    className={`flex-1 p-3 text-sm font-medium ${activeTab === 'nodes' ? 'bg-blue-50 text-blue-600' : 'text-gray-500 hover:bg-gray-50'}`}
                    onClick={() => setActiveTab('nodes')}
                >
                    Nodes
                </button>
            </div>
            
            {/* Content */}
            <div className="flex-1 overflow-auto">
                {activeTab === 'flows' && (
                    <FlowList 
                        flows={flows} 
                        activeFlowId={activeFlowId}
                        onSelectFlow={onSelectFlow}
                    />
                )}
                {activeTab === 'nodes' && (
                    <NodePalette />
                )}
            </div>
        </div>
    );
}
```

**Testing Focus**:
- Tab switching
- Flow list display
- Node palette display
- Responsive behavior

---

### Toolbar.tsx

**Purpose**: Top toolbar with flow actions and information.

**Responsibilities**:
- Display active flow name
- Show flow status
- Provide flow actions (save, deploy, undeploy, export, import)
- Show WebSocket status
- Display error messages

**Example Implementation**:
```typescript
import { Flow, FlowStatus } from '../types';
import { SaveButton } from './SaveButton';
import { DeployButton } from './DeployButton';
import { UndeployButton } from './UndeployButton';
import { ExportModal } from './ExportModal';
import { ImportModal } from './ImportModal';
import { WebSocketStatus } from './WebSocketStatus';

export interface ToolbarProps {
    activeFlow?: Flow;
    isConnected: boolean;
}

const statusColors: Record<FlowStatus, string> = {
    draft: 'bg-gray-200 text-gray-700',
    running: 'bg-green-500 text-white',
    error: 'bg-red-500 text-white',
    deploying: 'bg-yellow-500 text-white',
    undeploying: 'bg-yellow-500 text-white',
};

export function Toolbar({ activeFlow, isConnected }: ToolbarProps) {
    const [showExportModal, setShowExportModal] = useState(false);
    const [showImportModal, setShowImportModal] = useState(false);
    
    return (
        <div className="flex items-center justify-between p-2 bg-white border-b border-gray-200">
            {/* Left side */}
            <div className="flex items-center gap-4">
                <WebSocketStatus isConnected={isConnected} />
                
                {activeFlow && (
                    <>
                        <span className="font-medium">{activeFlow.name}</span>
                        <span className={`px-2 py-1 rounded text-xs ${statusColors[activeFlow.status] || statusColors.draft}`}>
                            {activeFlow.status}
                        </span>
                    </>
                )}
            </div>
            
            {/* Right side */}
            <div className="flex items-center gap-2">
                <SaveButton flow={activeFlow} />
                <DeployButton flow={activeFlow} />
                <UndeployButton flow={activeFlow} />
                <button 
                    className="p-2 text-gray-500 hover:bg-gray-100 rounded"
                    onClick={() => setShowExportModal(true)}
                    title="Export flow"
                >
                    📤
                </button>
                <button 
                    className="p-2 text-gray-500 hover:bg-gray-100 rounded"
                    onClick={() => setShowImportModal(true)}
                    title="Import flow"
                >
                    📥
                </button>
            </div>
            
            {/* Modals */}
            <ExportModal 
                isOpen={showExportModal}
                onClose={() => setShowExportModal(false)}
                flow={activeFlow}
            />
            <ImportModal 
                isOpen={showImportModal}
                onClose={() => setShowImportModal(false)}
            />
        </div>
    );
}
```

**Testing Focus**:
- Action button functionality
- Status display
- Modal opening/closing
- Responsive layout

---

### MessageLogPanel.tsx

**Purpose**: Display the message log for debugging flow execution.

**Responsibilities**:
- Show recent messages
- Allow filtering messages
- Display message details
- Clear message log
- Auto-scroll to newest messages

**Example Implementation**:
```typescript
import { useState, useRef, useEffect } from 'react';
import { Message } from '../types';

export interface MessageLogPanelProps {
    messages: Message[];
    onClear: () => void;
}

export function MessageLogPanel({ messages, onClear }: MessageLogPanelProps) {
    const [filter, setFilter] = useState('');
    const [autoScroll, setAutoScroll] = useState(true);
    const messagesEndRef = useRef<HTMLDivElement>(null);
    
    // Scroll to bottom when messages change
    useEffect(() => {
        if (autoScroll) {
            messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' });
        }
    }, [messages, autoScroll]);
    
    // Filter messages
    const filteredMessages = messages.filter(msg => {
        const searchText = filter.toLowerCase();
        return (
            msg.flowId.toLowerCase().includes(searchText) ||
            JSON.stringify(msg.payload).toLowerCase().includes(searchText) ||
            msg.id.toLowerCase().includes(searchText)
        );
    });
    
    // Format message timestamp
    const formatTimestamp = (timestamp: string) => {
        return new Date(timestamp).toLocaleTimeString();
    };
    
    // Format message preview
    const formatPreview = (payload: Record<string, any>) => {
        return JSON.stringify(payload).substring(0, 100);
    };
    
    return (
        <div className="w-80 bg-white border-l border-gray-200 flex flex-col">
            {/* Header */}
            <div className="p-2 border-b border-gray-200 flex items-center justify-between">
                <span className="font-medium">Message Log</span>
                <button 
                    className="text-xs text-gray-500 hover:text-gray-700"
                    onClick={onClear}
                >
                    Clear
                </button>
            </div>
            
            {/* Filter */}
            <div className="p-2">
                <input
                    type="text"
                    className="w-full p-2 border border-gray-200 rounded text-sm"
                    placeholder="Filter messages..."
                    value={filter}
                    onChange={(e) => setFilter(e.target.value)}
                />
            </div>
            
            {/* Messages */}
            <div className="flex-1 overflow-auto p-2">
                {filteredMessages.length === 0 ? (
                    <div className="text-center text-gray-500 text-sm p-4">
                        No messages
                    </div>
                ) : (
                    filteredMessages.map((msg, index) => (
                        <MessageLogEntry 
                            key={`${msg.id}-${index}`}
                            message={msg}
                            formatTimestamp={formatTimestamp}
                            formatPreview={formatPreview}
                        />
                    ))
                )}
                <div ref={messagesEndRef} />
            </div>
            
            {/* Auto-scroll toggle */}
            <div className="p-2 border-t border-gray-200">
                <label className="flex items-center gap-2 text-sm">
                    <input 
                        type="checkbox" 
                        checked={autoScroll}
                        onChange={(e) => setAutoScroll(e.target.checked)}
                    />
                    Auto-scroll
                </label>
            </div>
        </div>
    );
}

// Sub-component
function MessageLogEntry({ 
    message, 
    formatTimestamp,
    formatPreview 
}: {
    message: Message;
    formatTimestamp: (ts: string) => string;
    formatPreview: (payload: Record<string, any>) => string;
}) {
    const [expanded, setExpanded] = useState(false);
    
    return (
        <div 
            className={`p-2 mb-2 rounded border border-gray-100 ${expanded ? 'bg-gray-50' : ''}`}
            onClick={() => setExpanded(!expanded)}
        >
            <div className="flex items-center justify-between">
                <div>
                    <span className="text-xs text-gray-500">{formatTimestamp(message.timestamp)}</span>
                    <span className="ml-2 text-xs bg-blue-100 text-blue-700 px-1 rounded">
                        {message.flowId}
                    </span>
                </div>
                <span className="text-xs text-gray-400">
                    {message.path.join(' → ')}
                </span>
            </div>
            
            <div className={`mt-1 text-sm ${expanded ? 'block' : 'truncate'}`}>
                {formatPreview(message.payload)}
            </div>
            
            {expanded && (
                <div className="mt-2 p-2 bg-white rounded text-xs font-mono">
                    <pre>{JSON.stringify(message, null, 2)}</pre>
                </div>
            )}
        </div>
    );
}
```

**Testing Focus**:
- Message filtering
- Auto-scroll behavior
- Message expansion
- Clear functionality

---

### WebSocketStatus.tsx

**Purpose**: Display WebSocket connection status.

**Example Implementation**:
```typescript
import { useWebSocket } from '../hooks/useWebSocket';

export interface WebSocketStatusProps {
    isConnected?: boolean;
}

export function WebSocketStatus({ isConnected }: WebSocketStatusProps) {
    if (isConnected === undefined) {
        // Use hook if prop not provided
        const { isConnected: connected } = useWebSocket('/ws');
        isConnected = connected;
    }
    
    if (isConnected) {
        return (
            <span className="flex items-center gap-1 text-green-600 text-sm">
                <span className="w-2 h-2 bg-green-500 rounded-full animate-pulse" />
                Connected
            </span>
        );
    }
    
    return (
        <span className="flex items-center gap-1 text-red-600 text-sm">
            <span className="w-2 h-2 bg-red-500 rounded-full" />
            Disconnected
        </span>
    );
}
```

---

## Component Testing Guidelines

### Testing Library Best Practices

1. **Use descriptive test names**: Clearly state what is being tested
2. **Test user behavior**: Test what the user can do, not implementation details
3. **Use proper queries**: Prefer `getByRole` over `getByTestId` when possible
4. **Wait for async**: Use `waitFor` for async operations
5. **Clean up**: Clean up after each test

```typescript
// Example component test
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { FlowList } from './FlowList';
import { Flow } from '../types';

describe('FlowList', () => {
    const mockFlows: Flow[] = [
        { id: 'flow-1', name: 'Flow 1', status: 'draft' } as Flow,
        { id: 'flow-2', name: 'Flow 2', status: 'running' } as Flow,
    ];
    
    const mockOnSelect = vi.fn();
    
    it('renders all flows', () => {
        render(
            <FlowList 
                flows={mockFlows} 
                activeFlowId={null}
                onSelectFlow={mockOnSelect}
            />
        );
        
        expect(screen.getByText('Flow 1')).toBeInTheDocument();
        expect(screen.getByText('Flow 2')).toBeInTheDocument();
    });
    
    it('shows active flow as selected', () => {
        render(
            <FlowList 
                flows={mockFlows} 
                activeFlowId="flow-1"
                onSelectFlow={mockOnSelect}
            />
        );
        
        expect(screen.getByText('Flow 1')).toHaveClass('bg-blue-50');
        expect(screen.getByText('Flow 2')).not.toHaveClass('bg-blue-50');
    });
    
    it('calls onSelect when flow is clicked', async () => {
        render(
            <FlowList 
                flows={mockFlows} 
                activeFlowId={null}
                onSelectFlow={mockOnSelect}
            />
        );
        
        fireEvent.click(screen.getByText('Flow 1'));
        
        await waitFor(() => {
            expect(mockOnSelect).toHaveBeenCalledWith('flow-1');
        });
    });
});
```

### ReactFlow Component Testing

```typescript
// Example ReactFlow component test
import { render } from '@testing-library/react';
import { FlowCanvas } from './FlowCanvas';
import { Flow } from '../types';

describe('FlowCanvas', () => {
    const mockFlow: Flow = {
        id: 'test-flow',
        name: 'Test Flow',
        nodes: {
            'node-1': { id: 'node-1', type: 'inject', x: 0, y: 0, config: {} },
        },
        connections: [],
        config: { timeout: 30, maxConcurrency: 10, environment: {}, retryPolicy: { maxRetries: 3, backoff: 1, maxBackoff: 30, retryOn: [] } },
        status: 'draft',
        createdAt: new Date().toISOString(),
        updatedAt: new Date().toISOString(),
        version: '1.0.0',
    };
    
    it('renders without crashing', () => {
        render(<FlowCanvas flow={mockFlow} />);
        
        expect(screen.getByTestId('flow-canvas')).toBeInTheDocument();
    });
    
    it('renders nodes', () => {
        render(<FlowCanvas flow={mockFlow} />);
        
        // Check if node is rendered (implementation depends on NodeComponent)
        expect(screen.getByText('node-1')).toBeInTheDocument();
    });
});
```

---

## Accessibility Guidelines

### General Accessibility Rules

1. **Use semantic HTML**: Prefer `<button>` over `<div>` for clickable elements
2. **Add ARIA labels**: For icons and non-text elements
3. **Keyboard navigation**: Ensure all interactive elements are keyboard-accessible
4. **Focus management**: Manage focus for modals and overlays
5. **Color contrast**: Ensure sufficient contrast for readability

### Accessibility Checklist for Components

- [ ] All interactive elements have keyboard handlers (onKeyDown, onKeyUp)
- [ ] Images and icons have alt text or ARIA labels
- [ ] Form inputs have associated labels
- [ ] Buttons have descriptive text or ARIA labels
- [ ] Color is not the only way to convey information
- [ ] Focus indicators are visible
- [ ] Modals trap focus and return it on close
- [ ] Skip links are provided for long pages
- [ ] ARIA live regions for dynamic content

```typescript
// Good - Accessible button
<button
    onClick={handleClick}
    onKeyDown={(e) => e.key === 'Enter' && handleClick()}
    aria-label="Add new node"
    className="p-2 rounded hover:bg-gray-100"
>
    <PlusIcon />
</button>

// Good - Accessible modal
function Modal({ isOpen, onClose, children }: ModalProps) {
    const modalRef = useRef<HTMLDivElement>(null);
    const previousActiveElement = useRef<HTMLElement | null>(null);
    
    useEffect(() => {
        if (isOpen) {
            previousActiveElement.current = document.activeElement as HTMLElement;
            modalRef.current?.focus();
            document.addEventListener('keydown', handleKeyDown);
        }
        
        return () => {
            document.removeEventListener('keydown', handleKeyDown);
            if (previousActiveElement.current) {
                previousActiveElement.current.focus();
            }
        };
    }, [isOpen]);
    
    const handleKeyDown = (e: KeyboardEvent) => {
        if (e.key === 'Escape') onClose();
        if (e.key === 'Tab') {
            // Trap focus within modal
        }
    };
    
    if (!isOpen) return null;
    
    return createPortal(
        <div 
            className="fixed inset-0 bg-black/50 flex items-center justify-center z-50"
            onClick={onClose}
        >
            <div 
                ref={modalRef}
                className="bg-white rounded p-4 max-w-md w-full"
                onClick={(e) => e.stopPropagation()}
                role="dialog"
                aria-modal="true"
                aria-labelledby="modal-title"
                tabIndex={-1}
            >
                {children}
            </div>
        </div>,
        document.body
    );
}
```

---

## Performance Optimization

### Component Optimization Techniques

1. **React.memo()**: Prevent unnecessary re-renders
2. **useMemo()**: Memoize expensive calculations
3. **useCallback()**: Memoize event handlers
4. **Virtualization**: Use for long lists (react-window)
5. **Lazy loading**: Load components only when needed
6. **Code splitting**: Split large components

```typescript
// Good - Memoized component
interface ExpensiveComponentProps {
    data: ComplexData;
}

const ExpensiveComponent = React.memo(({ data }: ExpensiveComponentProps) => {
    // Expensive rendering
    return <div>{/* ... */}</div>;
});

// Good - Memoized calculations
export function MyComponent({ items }: { items: Item[] }) {
    // Memoize filtered items
    const filteredItems = useMemo(() => {
        return items.filter(item => item.isActive);
    }, [items]);
    
    // Memoize sorted items
    const sortedItems = useMemo(() => {
        return [...filteredItems].sort((a, b) => a.name.localeCompare(b.name));
    }, [filteredItems]);
    
    // Memoize event handlers
    const handleClick = useCallback((id: string) => {
        // Handle click
    }, []);
    
    return (
        <ul>
            {sortedItems.map(item => (
                <li key={item.id} onClick={() => handleClick(item.id)}>
                    {item.name}
                </li>
            ))}
        </ul>
    );
}
```

### ReactFlow Performance Tips

1. **Memoize node/edge data**: Prevent unnecessary re-renders
2. **Use simple node components**: Keep NodeComponent lightweight
3. **Limit visible nodes**: Implement virtualization for large flows
4. **Debounce updates**: For rapid position changes
5. **Avoid inline functions**: In node/edge data

```typescript
// Good - Optimized FlowCanvas
export function FlowCanvas({ flow }: { flow?: Flow }) {
    // Memoize conversion functions
    const convertToReactFlowNodes = useCallback(() => {
        if (!flow) return [];
        return Object.values(flow.nodes).map(node => ({
            id: node.id,
            type: 'custom',
            position: { x: node.x, y: node.y },
            data: { label: node.name || node.type, node },
        }));
    }, [flow]);
    
    const convertToReactFlowEdges = useCallback(() => {
        if (!flow) return [];
        return flow.connections.map(conn => ({
            id: conn.id,
            source: conn.sourceNode,
            target: conn.targetNode,
        }));
    }, [flow]);
    
    // Memoize nodes and edges
    const nodes = useMemo(() => convertToReactFlowNodes(), [convertToReactFlowNodes]);
    const edges = useMemo(() => convertToReactFlowEdges(), [convertToReactFlowEdges]);
    
    return (
        <ReactFlow
            nodes={nodes}
            edges={edges}
            // ... other props
        >
            {/* ... */}
        </ReactFlow>
    );
}
```

---

## Checklist for Component Development

Before finalizing a component:

- [ ] Component has a clear, single responsibility
- [ ] Props are properly typed
- [ ] Component has appropriate default props
- [ ] Component handles all edge cases
- [ ] Component has proper error handling
- [ ] Component is accessible
- [ ] Component has tests
- [ ] Component follows existing patterns
- [ ] Component uses consistent styling
- [ ] Component is responsive
- [ ] Component has proper documentation
- [ ] Component uses appropriate hooks
- [ ] Component doesn't cause unnecessary re-renders
- [ ] Component works with the state management
- [ ] Component integrates correctly with parent components

---

*Last updated: 2026-06-21*
*Overrides: None (extends web/src/AGENTS.md, web/AGENTS.md, and root AGENTS.md)*
