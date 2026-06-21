# Go-RED Web UI Source Guidelines

This file contains **source-specific** guidelines for the TypeScript/React code in `web/src/`.

---

## Overview

This directory contains the **core TypeScript/React source code** for the Go-RED Web UI. It follows a structured approach with clear separation of concerns:

```
src/
├── index.tsx               # Application entry point - renders App
├── App.tsx                 # Root component - sets up providers and routing
├── components/            # Reusable React components
├── hooks/                 # Custom React hooks
├── types/                 # TypeScript type definitions
├── utils/                 # Utility functions
└── styles/                # CSS and styling
```

---

## Source Code Organization

### Entry Points

**index.tsx** - Application bootstrap:
- Renders the root `<App />` component
- Mounts to DOM element with id `root`
- Should be minimal - delegate to App.tsx

**App.tsx** - Application shell:
- Sets up Zustand providers
- Manages global state
- Handles WebSocket connection
- Coordinates between major components

### Component Hierarchy

```
App
├── FlowProvider (Zustand)
│   └── FlowEditor
│       ├── Sidebar
│       │   ├── FlowList
│       │   └── NodePalette
│       ├── Toolbar
│       ├── FlowCanvas (ReactFlow)
│       │   └── NodeComponent (per node)
│       └── MessageLogPanel
└── ToastProvider
    └── ToastNotification
```

---

## Component Development Guidelines

### Component File Structure

Each component should be in its own file with:

1. **Imports** - Organized by type (React, external, internal)
2. **Interfaces/Types** - Component props and local types
3. **Component Definition** - The main component
4. **Sub-components** - Child components (if small and only used here)
5. **Exports** - Named export for the component

```typescript
// components/ExampleComponent.tsx

// 1. Imports
import React, { useState, useEffect } from 'react';
import { SomeType } from '../types';
import { useCustomHook } from '../hooks';

// 2. Interfaces
interface ExampleComponentProps {
    data: SomeType;
    onChange: (value: any) => void;
    className?: string;
}

// 3. Component
function ExampleComponent({ data, onChange, className = '' }: ExampleComponentProps) {
    const [state, setState] = useState(data);
    
    useEffect(() => {
        setState(data);
    }, [data]);
    
    const handleChange = (e: React.ChangeEvent<HTMLInputElement>) => {
        setState(e.target.value);
        onChange(e.target.value);
    };
    
    return (
        <div className={`p-4 ${className}`}>
            <input 
                value={state} 
                onChange={handleChange}
                className="w-full p-2 border rounded"
            />
        </div>
    );
}

// 4. Export
export { ExampleComponent };
```

### Component Best Practices

1. **Props Validation**: Use TypeScript types for props
2. **Default Props**: Provide sensible defaults for optional props
3. **Memoization**: Use `React.memo()` for pure components
4. **Error Boundaries**: Wrap error-prone components
5. **Forward Refs**: Use `React.forwardRef()` when ref access is needed
6. **Display Names**: Set `displayName` for debugging

```typescript
// Good - Memoized component with display name
const MemoizedComponent = React.memo(function MyComponent({ value }: { value: string }) {
    return <div>{value}</div>;
});
MemoizedComponent.displayName = 'MemoizedComponent';

// Good - Forward ref
interface ButtonProps extends React.ButtonHTMLAttributes<HTMLButtonElement> {
    variant?: 'primary' | 'secondary';
}

const Button = React.forwardRef<HTMLButtonElement, ButtonProps>(
    ({ variant = 'primary', children, ...props }, ref) => {
        const className = clsx(
            'px-4 py-2 rounded',
            variant === 'primary' && 'bg-blue-500 text-white',
            variant === 'secondary' && 'bg-gray-200 text-gray-800',
            props.className
        );
        return (
            <button ref={ref} className={className} {...props}>
                {children}
            </button>
        );
    }
);
Button.displayName = 'Button';
```

### Component Testing

Each component should have corresponding tests:

```typescript
// components/MyComponent.test.tsx
import { render, screen, fireEvent } from '@testing-library/react';
import { MyComponent } from './MyComponent';

describe('MyComponent', () => {
    const mockOnChange = vi.fn();
    const defaultProps = {
        value: 'initial',
        onChange: mockOnChange,
    };
    
    beforeEach(() => {
        vi.clearAllMocks();
    });
    
    it('renders with initial value', () => {
        render(<MyComponent {...defaultProps} />);
        expect(screen.getByDisplayValue('initial')).toBeInTheDocument();
    });
    
    it('calls onChange when input changes', () => {
        render(<MyComponent {...defaultProps} />);
        fireEvent.change(screen.getByRole('textbox'), { target: { value: 'new' } });
        expect(mockOnChange).toHaveBeenCalledWith('new');
    });
    
    it('applies custom className', () => {
        render(<MyComponent {...defaultProps} className="custom-class" />);
        expect(screen.getByDisplayValue('initial')).toHaveClass('custom-class');
    });
});
```

---

## Hook Development Guidelines

### Custom Hooks Structure

Each hook should:
1. Start with `use` prefix
2. Be placed in the `hooks/` directory
3. Have a clear, single responsibility
4. Return consistent types
5. Document dependencies and return values

```typescript
// hooks/useCounter.ts
import { useState, useCallback } from 'react';

interface UseCounterResult {
    count: number;
    increment: () => void;
    decrement: () => void;
    reset: () => void;
}

export function useCounter(initialValue: number = 0): UseCounterResult {
    const [count, setCount] = useState(initialValue);
    
    const increment = useCallback(() => {
        setCount(prev => prev + 1);
    }, []);
    
    const decrement = useCallback(() => {
        setCount(prev => prev - 1);
    }, []);
    
    const reset = useCallback(() => {
        setCount(initialValue);
    }, [initialValue]);
    
    return { count, increment, decrement, reset };
}
```

### Hook Testing

```typescript
// hooks/useCounter.test.ts
import { renderHook, act } from '@testing-library/react';
import { useCounter } from './useCounter';

describe('useCounter', () => {
    it('initializes with default value', () => {
        const { result } = renderHook(() => useCounter());
        expect(result.current.count).toBe(0);
    });
    
    it('initializes with custom value', () => {
        const { result } = renderHook(() => useCounter(10));
        expect(result.current.count).toBe(10);
    });
    
    it('increments counter', () => {
        const { result } = renderHook(() => useCounter());
        act(() => result.current.increment());
        expect(result.current.count).toBe(1);
    });
    
    it('decrements counter', () => {
        const { result } = renderHook(() => useCounter(5));
        act(() => result.current.decrement());
        expect(result.current.count).toBe(4);
    });
    
    it('resets to initial value', () => {
        const { result } = renderHook(() => useCounter(10));
        act(() => {
            result.current.increment();
            result.current.reset();
        });
        expect(result.current.count).toBe(10);
    });
});
```

### Application-Specific Hooks

The application has these custom hooks:

| Hook | Purpose | Dependencies |
|------|---------|--------------|
| `useFlows` | Flow state management | useFlowStore, useWebSocket |
| `useWebSocket` | WebSocket connection | - |
| `useMessageLog` | Message log management | useWebSocket |

---

## Type Definition Guidelines

### Type Organization

Types should be organized in the `types/` directory:

```
types/
├── index.ts       # Re-exports all types
├── api.ts        # API response/error types
├── flow.ts       # Flow-related types
├── node.ts       # Node-related types
└── message.ts    # Message-related types
```

### Type Best Practices

1. **Use interfaces for shapes**: When defining object shapes
2. **Use types for unions**: When defining union types
3. **Use generics**: When types need to be reusable
4. **Keep types simple**: Break complex types into smaller ones
5. **Document types**: Use JSDoc comments for complex types

```typescript
// Good - Well-organized types

// Union types
export type FlowStatus = 'draft' | 'running' | 'error' | 'deploying' | 'undeploying';

// Interface for data shapes
export interface Flow {
    id: string;
    name: string;
    description?: string;
    nodes: Record<string, Node>;
    connections: NodeConnection[];
    config: FlowConfig;
    status: FlowStatus;
    createdAt: string;
    updatedAt: string;
    version: string;
}

// Generic type
export interface ApiResponse<T> {
    data?: T;
    error?: string;
    message?: string;
    status: number;
}

// Type with JSDoc
/**
 * Represents a node in a flow.
 * All nodes must have an ID and type.
 * Position (x, y) determines where the node appears in the canvas.
 */
export interface Node {
    id: string;
    type: string;
    name?: string;
    x: number;
    y: number;
    config: Record<string, any>;
    disabled: boolean;
    status?: NodeStatus;
}
```

### Type Guards

Use type guards for runtime validation:

```typescript
// types/guards.ts
export function isFlowStatus(value: string): value is FlowStatus {
    return ['draft', 'running', 'error', 'deploying', 'undeploying'].includes(value);
}

export function isNode(object: any): object is Node {
    return (
        typeof object?.id === 'string' &&
        typeof object?.type === 'string' &&
        typeof object?.x === 'number' &&
        typeof object?.y === 'number'
    );
}
```

---

## Utility Function Guidelines

### Utility Organization

Utilities should be placed in the `utils/` directory and grouped by functionality:

```
utils/
├── index.ts       # Re-exports all utilities
├── api.ts        # API client utilities
├── flow.ts       # Flow-related utilities
├── node.ts       # Node-related utilities
└── validation.ts # Validation utilities
```

### Utility Best Practices

1. **Pure functions**: Avoid side effects
2. **Single responsibility**: One function, one job
3. **Type-safe**: Use TypeScript types
4. **Well-documented**: JSDoc comments for all exported functions
5. **Testable**: Easy to test in isolation

```typescript
// utils/flow.ts

/**
 * Creates a new flow with default values.
 * @param overrides - Partial flow data to override defaults
 * @returns A new Flow object with all required fields
 */
export function createDefaultFlow(overrides: Partial<Flow> = {}): Flow {
    return {
        id: generateId('flow'),
        name: 'New Flow',
        nodes: {},
        connections: [],
        config: {
            timeout: 30,
            maxConcurrency: 10,
            environment: {},
            retryPolicy: {
                maxRetries: 3,
                backoff: 1,
                maxBackoff: 30,
                retryOn: [],
            },
        },
        status: 'draft',
        createdAt: new Date().toISOString(),
        updatedAt: new Date().toISOString(),
        version: '1.0.0',
        ...overrides,
    };
}

/**
 * Validates a flow configuration.
 * @param flow - The flow to validate
 * @returns An array of validation errors, or empty array if valid
 */
export function validateFlow(flow: Partial<Flow>): string[] {
    const errors: string[] = [];
    
    if (!flow.id || typeof flow.id !== 'string') {
        errors.push('Flow must have a valid ID');
    }
    
    if (!flow.name || typeof flow.name !== 'string') {
        errors.push('Flow must have a valid name');
    }
    
    if (flow.nodes && typeof flow.nodes !== 'object') {
        errors.push('Flow nodes must be an object');
    }
    
    // Validate each node
    if (flow.nodes) {
        Object.values(flow.nodes).forEach((node, index) => {
            const nodeErrors = validateNode(node);
            nodeErrors.forEach(error => {
                errors.push(`Node ${index}: ${error}`);
            });
        });
    }
    
    return errors;
}

/**
 * Generates a unique ID with a prefix.
 * @param prefix - The prefix to use (e.g., 'flow', 'node')
 * @returns A unique ID with the format: {prefix}-{uuid}
 */
export function generateId(prefix: string): string {
    return `${prefix}-${crypto.randomUUID()}`;
}
```

---

## API Client Guidelines

### API Client Structure

All API calls should go through a centralized client in `utils/api.ts`:

```typescript
// utils/api.ts

// Base URL for API requests
const API_BASE = '/api';

// Standard headers for all requests
const defaultHeaders = {
    'Content-Type': 'application/json',
};

// Error handling helper
async function handleResponse<T>(response: Response): Promise<T> {
    if (!response.ok) {
        const errorData = await response.json().catch(() => ({}));
        throw new Error(
            errorData.error || 
            errorData.message ||
            `HTTP ${response.status}: ${response.statusText}`
        );
    }
    return response.json();
}

// CRUD operations for flows
export async function fetchFlows(): Promise<Flow[]> {
    const response = await fetch(`${API_BASE}/flows`);
    return handleResponse<Flow[]>(response);
}

export async function fetchFlow(flowId: string): Promise<Flow> {
    const response = await fetch(`${API_BASE}/flows/${flowId}`);
    return handleResponse<Flow>(response);
}

export async function createFlow(flow: Partial<Flow>): Promise<Flow> {
    const response = await fetch(`${API_BASE}/flows`, {
        method: 'POST',
        headers: defaultHeaders,
        body: JSON.stringify(flow),
    });
    return handleResponse<Flow>(response);
}

export async function updateFlow(flowId: string, flow: Partial<Flow>): Promise<Flow> {
    const response = await fetch(`${API_BASE}/flows/${flowId}`, {
        method: 'PUT',
        headers: defaultHeaders,
        body: JSON.stringify(flow),
    });
    return handleResponse<Flow>(response);
}

export async function deleteFlow(flowId: string): Promise<void> {
    const response = await fetch(`${API_BASE}/flows/${flowId}`, {
        method: 'DELETE',
    });
    await handleResponse<void>(response);
}

// Flow actions
export async function deployFlow(flowId: string): Promise<void> {
    const response = await fetch(`${API_BASE}/flows/${flowId}/deploy`, {
        method: 'POST',
    });
    await handleResponse<void>(response);
}

export async function undeployFlow(flowId: string): Promise<void> {
    const response = await fetch(`${API_BASE}/flows/${flowId}/undeploy`, {
        method: 'POST',
    });
    await handleResponse<void>(response);
}

// Export/Import
export async function exportFlow(flowId: string): Promise<Flow> {
    const response = await fetch(`${API_BASE}/flows/${flowId}/export`);
    return handleResponse<Flow>(response);
}

export async function importFlow(flow: Flow): Promise<Flow> {
    const response = await fetch(`${API_BASE}/flows/import`, {
        method: 'POST',
        headers: defaultHeaders,
        body: JSON.stringify(flow),
    });
    return handleResponse<Flow>(response);
}

// Node operations
export async function fetchNodeTypes(): Promise<NodeType[]> {
    const response = await fetch(`${API_BASE}/nodes`);
    return handleResponse<NodeType[]>(response);
}

export async function fetchNodeType(nodeType: string): Promise<NodeType> {
    const response = await fetch(`${API_BASE}/nodes/${nodeType}`);
    return handleResponse<NodeType>(response);
}

// Message operations
export async function fetchMessages(options?: { flowId?: string; limit?: number }): Promise<Message[]> {
    const params = new URLSearchParams();
    if (options?.flowId) params.append('flowId', options.flowId);
    if (options?.limit) params.append('limit', options.limit.toString());
    
    const response = await fetch(`${API_BASE}/messages?${params.toString()}`);
    return handleResponse<Message[]>(response);
}
```

---

## State Management Guidelines

### Zustand Store Patterns

The application uses **Zustand** for state management. Follow these patterns:

1. **Single store per domain**: One store per major domain (flows, nodes, messages)
2. **Actions as methods**: Include both synchronous and asynchronous actions
3. **Selectors**: Provide selector functions for derived state
4. **DevTools**: Enable Redux DevTools for debugging

```typescript
// components/FlowProvider.tsx
import { create } from 'zustand';
import { devtools } from 'zustand/middleware';

interface FlowStore {
    // State
    flows: Flow[];
    activeFlowId: string | null;
    isLoading: boolean;
    error: string | null;
    
    // Actions
    setFlows: (flows: Flow[]) => void;
    setActiveFlowId: (flowId: string | null) => void;
    addFlow: (flow: Flow) => void;
    updateFlow: (flowId: string, updates: Partial<Flow>) => void;
    deleteFlow: (flowId: string) => void;
    setLoading: (loading: boolean) => void;
    setError: (error: string | null) => void;
    
    // Selectors
    getActiveFlow: () => Flow | undefined;
    getFlowById: (flowId: string) => Flow | undefined;
    hasFlow: (flowId: string) => boolean;
    
    // Async actions
    fetchFlows: () => Promise<void>;
    createFlow: (flowData: Partial<Flow>) => Promise<Flow>;
    updateFlowAsync: (flowId: string, updates: Partial<Flow>) => Promise<Flow>;
    deleteFlowAsync: (flowId: string) => Promise<void>;
}

const initialState = {
    flows: [],
    activeFlowId: null,
    isLoading: false,
    error: null,
};

export const useFlowStore = create<FlowStore>()(
    devtools((set, get) => ({
        ...initialState,
        
        // Synchronous actions
        setFlows: (flows) => set({ flows }),
        setActiveFlowId: (flowId) => set({ activeFlowId: flowId }),
        addFlow: (flow) => set(state => ({ flows: [...state.flows, flow] })),
        updateFlow: (flowId, updates) => set(state => ({
            flows: state.flows.map(f => f.id === flowId ? { ...f, ...updates } : f)
        })),
        deleteFlow: (flowId) => set(state => ({
            flows: state.flows.filter(f => f.id !== flowId)
        })),
        setLoading: (loading) => set({ isLoading: loading }),
        setError: (error) => set({ error }),
        
        // Selectors
        getActiveFlow: () => {
            const { activeFlowId, flows } = get();
            return flows.find(f => f.id === activeFlowId);
        },
        getFlowById: (flowId) => {
            const { flows } = get();
            return flows.find(f => f.id === flowId);
        },
        hasFlow: (flowId) => {
            const { flows } = get();
            return flows.some(f => f.id === flowId);
        },
        
        // Async actions
        fetchFlows: async () => {
            set({ isLoading: true, error: null });
            try {
                const flows = await fetchFlows();
                set({ flows, isLoading: false });
            } catch (error) {
                set({ error: error.message, isLoading: false });
            }
        },
        
        createFlow: async (flowData) => {
            set({ isLoading: true });
            try {
                const newFlow = await createFlow(flowData);
                set(state => ({ 
                    flows: [...state.flows, newFlow],
                    isLoading: false 
                }));
                return newFlow;
            } catch (error) {
                set({ error: error.message, isLoading: false });
                throw error;
            }
        },
        
        updateFlowAsync: async (flowId, updates) => {
            set({ isLoading: true });
            try {
                const updatedFlow = await updateFlow(flowId, updates);
                set(state => ({
                    flows: state.flows.map(f => f.id === flowId ? updatedFlow : f),
                    isLoading: false
                }));
                return updatedFlow;
            } catch (error) {
                set({ error: error.message, isLoading: false });
                throw error;
            }
        },
        
        deleteFlowAsync: async (flowId) => {
            set({ isLoading: true });
            try {
                await deleteFlow(flowId);
                set(state => ({
                    flows: state.flows.filter(f => f.id !== flowId),
                    isLoading: false
                }));
            } catch (error) {
                set({ error: error.message, isLoading: false });
                throw error;
            }
        },
    }), { name: 'FlowStore' })
);

// Selector hooks for better performance
export const useFlows = () => useFlowStore(state => state.flows);
export const useActiveFlowId = () => useFlowStore(state => state.activeFlowId);
export const useActiveFlow = () => useFlowStore(state => state.getActiveFlow());
export const useFlowById = (flowId: string) => useFlowStore(state => state.getFlowById(flowId));
```

---

## ReactFlow Integration Guidelines

### ReactFlow Setup

The application uses **ReactFlow** for flow visualization. Key integration points:

```typescript
// components/FlowCanvas.tsx
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
    Connection,
    EdgeChange,
    NodeChange,
    OnConnect,
    OnEdgesChange,
    OnNodesChange,
} from 'reactflow';
import 'reactflow/dist/style.css';

// Custom node types
import { NodeComponent } from './NodeComponent';

// Register custom node types
const nodeTypes = {
    custom: NodeComponent,
};

// Default viewport settings
const defaultViewport = { x: 0, y: 0, zoom: 1 };

// Fit view options
const fitViewOptions = {
    padding: 0.5,
    duration: 500,
};

export function FlowCanvas({ flow }: { flow?: Flow }) {
    const [nodes, setNodes, onNodesChange] = useNodesState([]);
    const [edges, setEdges, onEdgesChange] = useEdgesState([]);
    
    // Convert domain model to ReactFlow model
    const convertToReactFlow = useCallback(() => {
        if (!flow) return { nodes: [], edges: [] };
        
        const rfNodes: RFNode[] = Object.values(flow.nodes).map(node => ({
            id: node.id,
            type: 'custom',
            position: { x: node.x, y: node.y },
            data: { label: node.name || node.type, node },
        }));
        
        const rfEdges: RFEdge[] = flow.connections.map(conn => ({
            id: conn.id,
            source: conn.sourceNode,
            target: conn.targetNode,
            sourceHandle: conn.sourcePort,
            targetHandle: conn.targetPort,
        }));
        
        return { nodes: rfNodes, edges: rfEdges };
    }, [flow]);
    
    // Update ReactFlow when flow changes
    useEffect(() => {
        const { nodes: newNodes, edges: newEdges } = convertToReactFlow();
        setNodes(newNodes);
        setEdges(newEdges);
    }, [flow, convertToReactFlow]);
    
    // Handle node changes (position, selection)
    const onNodeChanges = useCallback<OnNodesChange>((changes) => {
        onNodesChange(changes);
        
        // Update flow state with new positions
        changes.forEach(change => {
            if (change.type === 'position') {
                // Update flow node position
            }
        });
    }, [onNodesChange]);
    
    // Handle edge changes (add, remove, update)
    const onEdgeChanges = useCallback<OnEdgesChange>((changes) => {
        onEdgesChange(changes);
    }, [onEdgesChange]);
    
    // Handle new connections
    const onConnect = useCallback<OnConnect>((params) => {
        onEdgesChange(addEdge(params));
        
        // Create new connection in flow
        const newConnection: NodeConnection = {
            id: generateId('conn'),
            sourceNode: params.source,
            targetNode: params.target,
            sourcePort: params.sourceHandle,
            targetPort: params.targetHandle,
        };
        
        // Update flow state
    }, [onEdgesChange]);
    
    return (
        <div className="flex-1" data-testid="flow-canvas">
            <ReactFlow
                nodes={nodes}
                edges={edges}
                onNodesChange={onNodeChanges}
                onEdgesChange={onEdgeChanges}
                onConnect={onConnect}
                nodeTypes={nodeTypes}
                defaultViewport={defaultViewport}
                fitView
                fitViewOptions={fitViewOptions}
            >
                <Background color="#f0f0f0" gap={16} />
                <Controls />
                <MiniMap />
            </ReactFlow>
        </div>
    );
}

// Wrap in ReactFlowProvider for context
export function FlowCanvasWithProvider({ flow }: { flow?: Flow }) {
    return (
        <ReactFlowProvider>
            <FlowCanvas flow={flow} />
        </ReactFlowProvider>
    );
}
```

### Custom Node Components

Each node type can have a custom component for rendering:

```typescript
// components/NodeComponent.tsx
import { memo } from 'react';
import { Node as RFNode, useNodeId } from 'reactflow';
import { Node } from '../types/node';
import { useNodeType } from '../hooks/useNodeTypes';

interface NodeComponentProps {
    data: {
        label: string;
        node: Node;
    };
}

// Memoize to prevent unnecessary re-renders
export const NodeComponent = memo(({ data }: NodeComponentProps) => {
    const nodeId = useNodeId();
    const { node } = data;
    
    // Get node type metadata
    const nodeType = useNodeType(node.type);
    
    // Determine node color based on status
    const getStatusColor = () => {
        switch (node.status?.state) {
            case 'error': return 'bg-red-500';
            case 'processing': return 'bg-yellow-500';
            default: return nodeType?.color || 'bg-blue-500';
        }
    };
    
    // Handle node selection
    const onSelect = () => {
        // Select node in state
    };
    
    return (
        <div 
            className="rounded shadow-md cursor-pointer hover:shadow-lg transition-shadow"
            style={{ backgroundColor: nodeType?.color }}
            onClick={onSelect}
        >
            <div className="p-2">
                <div className="flex items-center gap-2">
                    {nodeType?.icon && (
                        <Icon name={nodeType.icon} className="text-white" />
                    )}
                    <span className="text-white font-medium text-sm">{node.name || node.type}</span>
                </div>
            </div>
            
            <div className="px-2 pb-2">
                {/* Node-specific content */}
                <NodeContent node={node} />
            </div>
            
            {/* Status indicator */}
            <div className={`h-1 ${getStatusColor()}`} />
            
            {/* Ports */}
            <NodePorts node={node} nodeType={nodeType} />
        </div>
    );
});

// Sub-components
function NodeContent({ node }: { node: Node }) {
    const nodeType = useNodeType(node.type);
    
    if (!nodeType) return null;
    
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

function DebugNodeContent({ node }: { node: Node }) {
    return (
        <div className="text-xs text-white/80">
            {node.config?.output || 'all'}
        </div>
    );
}

function FunctionNodeContent({ node }: { node: Node }) {
    return (
        <div className="text-xs text-white/80 truncate">
            {node.config?.function || ''}
        </div>
    );
}

function InjectNodeContent({ node }: { node: Node }) {
    return (
        <div className="text-xs text-white/80">
            {node.config?.repeat ? `Every ${node.config.repeat}s` : 'Manual'}
        </div>
    );
}

function DefaultNodeContent({ node }: { node: Node }) {
    return (
        <div className="text-xs text-white/80">
            {Object.keys(node.config || {}).length > 0 ? 
                `Config: ${Object.keys(node.config).length} props` :
                'No config'}
        </div>
    );
}

function NodePorts({ node, nodeType }: { node: Node; nodeType?: NodeType }) {
    const ports = [
        ...(nodeType?.inputPorts || ['input']),
        ...(nodeType?.outputPorts || ['output']),
    ];
    
    return (
        <div className="flex justify-between px-1 py-1">
            {nodeType?.inputPorts?.map(port => (
                <div key={port} className="w-3 h-3 bg-white/50 rounded-full" />
            ))}
            {nodeType?.outputPorts?.map(port => (
                <div key={port} className="w-3 h-3 bg-white/50 rounded-full" />
            ))}
        </div>
    );
}
```

---

## Testing Guidelines for Source

### Test File Organization

Each source file should have a corresponding test file:
- Components: `{ComponentName}.test.tsx`
- Hooks: `{HookName}.test.ts`
- Utilities: `{UtilityName}.test.ts`
- Types: `types.test.ts` (all types together)

### Test Setup

```typescript
// src/test/setup.ts
import { beforeAll, afterAll, vi, afterEach } from 'vitest';
import { cleanup } from '@testing-library/react';
import '@testing-library/jest-dom';

// Mock WebSocket
global.WebSocket = class MockWebSocket {
    onopen: (() => void) | null = null;
    onclose: (() => void) | null = null;
    onmessage: ((event: MessageEvent) => void) | null = null;
    onerror: ((error: Error) => void) | null = null;
    readyState = WebSocket.OPEN;
    
    constructor(url: string) {
        setTimeout(() => this.onopen?.(), 0);
    }
    
    send(message: string) {
        if (this.onmessage) {
            this.onmessage({ data: message } as MessageEvent);
        }
    }
    
    close() {
        this.readyState = WebSocket.CLOSED;
        this.onclose?.();
    }
} as any;

// Mock fetch
global.fetch = vi.fn(() =>
    Promise.resolve({
        ok: true,
        json: () => Promise.resolve({}),
    })
);

// Cleanup after each test
afterEach(() => {
    cleanup();
    vi.clearAllMocks();
});

// Cleanup after all tests
afterAll(() => {
    vi.restoreAllMocks();
});
```

---

## Checklist for Source Changes

Before committing changes to `web/src/`:

- [ ] All existing tests pass (`npm test`)
- [ ] TypeScript compilation succeeds (`npm run typecheck`)
- [ ] ESLint passes (`npm run lint`)
- [ ] No TypeScript errors in the IDE
- [ ] Components are properly typed
- [ ] Custom hooks follow the `use*` convention
- [ ] Utility functions are pure and well-documented
- [ ] Type definitions are organized and documented
- [ ] State management follows Zustand patterns
- [ ] ReactFlow integration works correctly
- [ ] Backend communication works correctly
- [ ] No console warnings in development mode
- [ ] Code follows existing patterns and conventions

---

*Last updated: 2026-06-21*
*Overrides: None (extends web/AGENTS.md and root AGENTS.md)*
