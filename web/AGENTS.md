# Go-RED Web UI Guidelines

This file contains **frontend-specific** guidelines for the React Web UI in the `web/` directory.

---

## Package Overview

The `web/` directory contains the **React-based Web UI** for Go-RED:

```
web/
├── index.html                    # HTML entry point
├── package.json                  # Node.js dependencies and scripts
├── package-lock.json             # Dependency lock file
├── tsconfig.json                # TypeScript configuration
├── tsconfig.node.json           # TypeScript config for Node
├── vite.config.ts               # Vite bundler configuration
├── vitest.config.ts             # Vitest test configuration
├── tailwind.config.js           # Tailwind CSS configuration
├── public/                      # Static assets (favicon, etc.)
└── src/                         # TypeScript/React source code
    ├── index.tsx                # Application entry point
    ├── App.tsx                  # Main App component
    ├── components/             # React components
    │   ├── FlowEditor.tsx      # Main flow editor
    │   ├── FlowProvider.tsx    # Zustand state provider
    │   ├── FlowCanvas.tsx      # ReactFlow canvas
    │   ├── NodePalette.tsx     # Available nodes panel
    │   ├── NodeComponent.tsx   # Node rendering
    │   ├── Sidebar.tsx         # Left sidebar
    │   ├── Toolbar.tsx         # Top toolbar
    │   ├── MessageLogPanel.tsx # Message log
    │   ├── ToastNotification.tsx # Toast notifications
    │   ├── ExportModal.tsx     # Export flow modal
    │   └── ImportModal.tsx     # Import flow modal
    ├── hooks/                  # Custom React hooks
    │   ├── index.ts            # Hook exports
    │   ├── useFlows.ts         # Flow management
    │   ├── useMessageLog.ts    # Message log
    │   └── useWebSocket.ts     # WebSocket communication
    ├── types/                  # TypeScript type definitions
    │   ├── index.ts            # Type exports
    │   ├── api.ts              # API response types
    │   ├── flow.ts             # Flow types
    │   ├── node.ts             # Node types
    │   └── message.ts          # Message types
    ├── utils/                  # Utility functions
    │   └── api.ts              # API client functions
    └── styles/                 # CSS/styles
        └── tailwind.css         # Tailwind CSS
    └── test/                     # Frontend tests
        ├── setup.ts             # Test setup
        ├── flowCanvas.test.tsx  # Flow canvas tests
        ├── types.test.ts        # Type tests
        └── utils.test.tsx       # Utility tests
```

---

## Architecture

### Technology Stack

| Technology | Purpose | Version |
|------------|---------|---------|
| **React** | UI Framework | 18.2.0 |
| **TypeScript** | Type System | 5.3.3 |
| **Vite** | Bundler/Dev Server | 5.0.10 |
| **ReactFlow** | Flow Diagram Library | 11.10.0 |
| **Zustand** | State Management | 4.4.7 |
| **Tailwind CSS** | Styling | 3.4.0 |
| **Vitest** | Testing | 1.3.1 |
| **WebSocket** | Real-time communication | Native |

### Data Flow

```
Frontend Components
        ↓
Custom Hooks (useFlows, useWebSocket, useMessageLog)
        ↓
Zustand Store (FlowProvider)
        ↓
WebSocket Connection (useWebSocket)
        ↓
REST API Calls (utils/api.ts)
        ↓
Go-RED Backend (cmd/go-red/main.go)
```

### State Management

**Zustand** is used for global state management:

```typescript
// Store definition
interface FlowStore {
    flows: Flow[];
    activeFlowId: string | null;
    nodes: Record<string, Node>;
    connections: Connection[];
    // Actions
    setFlows: (flows: Flow[]) => void;
    setActiveFlowId: (flowId: string | null) => void;
    addFlow: (flow: Flow) => void;
    updateFlow: (flowId: string, updates: Partial<Flow>) => void;
    deleteFlow: (flowId: string) => void;
}

// Usage in components
const activeFlowId = useFlowStore(state => state.activeFlowId);
const setActiveFlowId = useFlowStore(state => state.setActiveFlowId);
```

---

## Development Guidelines

### Code Style

1. **TypeScript**: Use strict mode, explicit types
2. **React**: Use functional components with hooks
3. **Naming**: Use PascalCase for components, camelCase for variables/functions
4. **Formatting**: Use Prettier with project config
5. **Linting**: Use ESLint with project config

### Directory Structure Rules

- **components/**: Reusable React components
  - Each component in its own file
  - Use index.ts for exports
  - Props should be typed with interfaces
- **hooks/**: Custom React hooks
  - Start with `use` prefix
  - Type all return values
- **types/**: TypeScript type definitions
  - Group related types together
  - Export all types from index.ts
- **utils/**: Utility functions
  - Pure functions (no side effects)
  - Well-documented with JSDoc

### File Naming Conventions

| Type | Convention | Example |
|------|------------|---------|
| Components | PascalCase | `FlowEditor.tsx` |
| Hooks | use* prefix | `useWebSocket.ts` |
| Types | PascalCase or *Types | `flow.ts`, `FlowTypes.ts` |
| Utils | kebab-case or camelCase | `api.ts`, `flowUtils.ts` |
| Tests | *.test.tsx/ts | `flowCanvas.test.tsx` |

---

## Build & Development

### Development Server

```bash
# Start Vite development server
npm run dev

# Access at http://localhost:5173
```

### Production Build

```bash
# Build for production
npm run build

# Build output goes to web/dist/
```

### Testing

```bash
# Run all tests
npm test

# Run with coverage
npm run test:coverage

# Run type check
npm run typecheck

# Run linting
npm run lint
```

### Available Scripts

| Script | Description |
|--------|-------------|
| `npm run dev` | Start development server |
| `npm run build` | Build for production |
| `npm run preview` | Preview production build |
| `npm run lint` | Run ESLint |
| `npm run typecheck` | TypeScript type checking |
| `npm run test` | Run Vitest tests |
| `npm run test:coverage` | Run tests with coverage |

---

## Backend Communication

### REST API

All REST API calls go through `utils/api.ts`:

```typescript
// utils/api.ts
const API_BASE = '/api';

export async function fetchFlows(): Promise<Flow[]> {
    const response = await fetch(`${API_BASE}/flows`);
    if (!response.ok) {
        throw new Error('Failed to fetch flows');
    }
    return response.json();
}

export async function createFlow(flow: Partial<Flow>): Promise<Flow> {
    const response = await fetch(`${API_BASE}/flows`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(flow),
    });
    if (!response.ok) {
        throw new Error('Failed to create flow');
    }
    return response.json();
}
```

### WebSocket Communication

Real-time communication via WebSocket (handled in `hooks/useWebSocket.ts`):

```typescript
// hooks/useWebSocket.ts
interface WebSocketMessage {
    type: string;
    payload?: any;
}

export function useWebSocket(url: string) {
    const [socket, setSocket] = useState<WebSocket | null>(null);
    const [isConnected, setIsConnected] = useState(false);
    
    useEffect(() => {
        const ws = new WebSocket(url);
        
        ws.onopen = () => setIsConnected(true);
        ws.onclose = () => setIsConnected(false);
        ws.onmessage = (event) => {
            const message: WebSocketMessage = JSON.parse(event.data);
            // Handle message based on type
        };
        
        setSocket(ws);
        return () => ws.close();
    }, [url]);
    
    const sendMessage = useCallback((message: WebSocketMessage) => {
        if (socket?.readyState === WebSocket.OPEN) {
            socket.send(JSON.stringify(message));
        }
    }, [socket]);
    
    return { socket, isConnected, sendMessage };
}
```

### WebSocket Message Types

| Type | Direction | Description | Payload |
|------|-----------|-------------|---------|
| `flow:list` | Backend → Frontend | List of all flows | `{flows: Flow[]}` |
| `flow:created` | Backend → Frontend | New flow created | `{flow: Flow}` |
| `flow:updated` | Backend → Frontend | Flow updated | `{flow: Flow}` |
| `flow:deleted` | Backend → Frontend | Flow deleted | `{flowId: string}` |
| `flow:deployed` | Backend → Frontend | Flow deployed | `{flowId: string}` |
| `flow:undeployed` | Backend → Frontend | Flow undeployed | `{flowId: string}` |
| `flow:status` | Backend → Frontend | Flow status changed | `{flowId: string, status: FlowStatus}` |
| `node:list` | Backend → Frontend | List of node types | `{nodes: NodeType[]}` |
| `message:new` | Backend → Frontend | New message in log | `{flowId: string, message: Message}` |
| `error` | Backend → Frontend | Error occurred | `{error: string, details?: any}` |

---

## Component Guidelines

### FlowEditor (Main Component)

The `FlowEditor` component is the **root component** that orchestrates everything:

```typescript
// components/FlowEditor.tsx
export function FlowEditor() {
    const { flows, activeFlowId, setActiveFlowId } = useFlowStore();
    const { isConnected, sendMessage } = useWebSocket('/ws');
    const { messages } = useMessageLog();
    
    if (!isConnected) {
        return <WebSocketStatus />;
    }
    
    return (
        <div className="flex h-screen">
            <Sidebar flows={flows} activeFlowId={activeFlowId} />
            <div className="flex-1 flex flex-col">
                <Toolbar />
                <FlowProvider>
                    <FlowCanvas />
                </FlowProvider>
            </div>
            <MessageLogPanel messages={messages} />
        </div>
    );
}
```

### Component Structure

Each component should:

1. **Have typed props**:
```typescript
interface FlowCanvasProps {
    flow?: Flow;
    onNodeAdd: (node: Node) => void;
    onNodeSelect: (nodeId: string) => void;
}
```

2. **Use proper hooks**:
```typescript
import { useState, useEffect, useCallback } from 'react';
```

3. **Handle errors gracefully**:
```typescript
const [error, setError] = useState<string | null>(null);

if (error) {
    return <ErrorDisplay message={error} onRetry={() => setError(null)} />;
}
```

4. **Be accessible**:
```typescript
<button
    onClick={handleClick}
    aria-label="Add node"
    className="..."
>
    <PlusIcon />
</button>
```

### ReactFlow Integration

The project uses **ReactFlow** for flow visualization and editing:

```typescript
// components/FlowCanvas.tsx
import ReactFlow, {
    Background,
    Controls,
    useNodesState,
    useEdgesState,
    addEdge,
    Node as RFNode,
    Edge as RFEdge,
} from 'reactflow';

// Convert domain nodes to ReactFlow nodes
function convertToReactFlowNodes(nodes: Record<string, Node>): RFNode[] {
    return Object.values(nodes).map(node => ({
        id: node.id,
        type: 'custom',
        position: { x: node.position.x, y: node.position.y },
        data: { label: node.name, node },
    }));
}

// Convert domain connections to ReactFlow edges
function convertToReactFlowEdges(connections: Connection[]): RFEdge[] {
    return connections.map(conn => ({
        id: conn.id,
        source: conn.sourceNode,
        target: conn.targetNode,
        sourceHandle: conn.sourcePort,
        targetHandle: conn.targetPort,
    }));
}

export function FlowCanvas({ flow }: FlowCanvasProps) {
    const [nodes, , onNodesChange] = useNodesState([]);
    const [edges, , onEdgesChange] = useEdgesState([]);
    
    // Convert flow to ReactFlow format
    useEffect(() => {
        if (flow) {
            onNodesChange(convertToReactFlowNodes(flow.nodes));
            onEdgesChange(convertToReactFlowEdges(flow.connections));
        }
    }, [flow]);
    
    return (
        <ReactFlow
            nodes={nodes}
            edges={edges}
            onNodesChange={onNodesChange}
            onEdgesChange={onEdgesChange}
            onConnect={params => onEdgesChange(addEdge(params))}
            fitView
        >
            <Background />
            <Controls />
            <MiniMap />
        </ReactFlow>
    );
}
```

---

## State Management with Zustand

### Store Definition

```typescript
// components/FlowProvider.tsx
import { create } from 'zustand';

interface FlowState {
    flows: Flow[];
    activeFlowId: string | null;
    nodes: Record<string, Node>;
    connections: Connection[];
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
    
    // Async actions
    fetchFlows: () => Promise<void>;
    createFlow: (flowData: Partial<Flow>) => Promise<Flow>;
}

export const useFlowStore = create<FlowState>((set, get) => ({
    flows: [],
    activeFlowId: null,
    nodes: {},
    connections: [],
    isLoading: false,
    error: null,
    
    setFlows: (flows) => set({ flows }),
    setActiveFlowId: (flowId) => set({ activeFlowId: flowId }),
    // ... other setters ...
    
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
            const newFlow = await api.createFlow(flowData);
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
}));
```

### Custom Hooks

#### useFlows

Manages flow state and operations:

```typescript
// hooks/useFlows.ts
export function useFlows() {
    const store = useFlowStore();
    
    // Fetch flows on mount
    useEffect(() => {
        store.fetchFlows();
    }, []);
    
    // Subscribe to WebSocket updates
    const { sendMessage } = useWebSocket('/ws');
    
    // Request flow list
    const refreshFlows = useCallback(() => {
        sendMessage({ type: 'flow:list' });
    }, [sendMessage]);
    
    // Create flow
    const createFlow = useCallback(async (data: Partial<Flow>) => {
        const flow = await store.createFlow(data);
        sendMessage({ type: 'flow:list' }); // Refresh list
        return flow;
    }, [store, sendMessage]);
    
    return {
        flows: store.flows,
        activeFlowId: store.activeFlowId,
        activeFlow: store.flows.find(f => f.id === store.activeFlowId),
        isLoading: store.isLoading,
        error: store.error,
        setActiveFlowId: store.setActiveFlowId,
        refreshFlows,
        createFlow,
        // ... other operations
    };
}
```

#### useWebSocket

Manages WebSocket connection and messaging:

```typescript
// hooks/useWebSocket.ts
export function useWebSocket(url: string) {
    const [socket, setSocket] = useState<WebSocket | null>(null);
    const [isConnected, setIsConnected] = useState(false);
    const [lastError, setLastError] = useState<string | null>(null);
    
    useEffect(() => {
        let ws: WebSocket;
        
        const connect = () => {
            ws = new WebSocket(url);
            
            ws.onopen = () => {
                setIsConnected(true);
                setLastError(null);
            };
            
            ws.onclose = () => {
                setIsConnected(false);
                // Auto-reconnect after delay
                setTimeout(connect, 5000);
            };
            
            ws.onerror = (error) => {
                setLastError(error.message);
            };
            
            ws.onmessage = (event) => {
                try {
                    const message: WebSocketMessage = JSON.parse(event.data);
                    // Handle different message types
                    switch (message.type) {
                        case 'flow:list':
                            // Update flow store
                            break;
                        case 'flow:status':
                            // Update flow status
                            break;
                        case 'message:new':
                            // Add to message log
                            break;
                        case 'error':
                            setLastError(message.payload?.error);
                            break;
                    }
                } catch (error) {
                    console.error('Failed to parse WebSocket message:', error);
                }
            };
            
            setSocket(ws);
        };
        
        connect();
        
        return () => {
            ws?.close();
        };
    }, [url]);
    
    const sendMessage = useCallback((message: WebSocketMessage) => {
        if (socket?.readyState === WebSocket.OPEN) {
            socket.send(JSON.stringify(message));
        } else {
            console.warn('WebSocket not connected, message not sent:', message);
        }
    }, [socket]);
    
    return { socket, isConnected, lastError, sendMessage };
}
```

#### useMessageLog

Manages message log state:

```typescript
// hooks/useMessageLog.ts
export function useMessageLog() {
    const [messages, setMessages] = useState<Message[]>([]);
    const [filter, setFilter] = useState<string>('');
    
    const { sendMessage } = useWebSocket('/ws');
    
    // Request message log
    const fetchMessages = useCallback((flowId?: string, limit?: number) => {
        const params = new URLSearchParams();
        if (flowId) params.append('flowId', flowId);
        if (limit) params.append('limit', limit.toString());
        
        sendMessage({
            type: 'message:get',
            payload: { flowId, limit }
        });
    }, [sendMessage]);
    
    // Handle incoming messages
    const handleNewMessage = useCallback((message: Message) => {
        setMessages(prev => [message, ...prev].slice(0, 1000));
    }, []);
    
    // Filter messages
    const filteredMessages = useMemo(() => {
        if (!filter) return messages;
        return messages.filter(msg => 
            JSON.stringify(msg).toLowerCase().includes(filter.toLowerCase())
        );
    }, [messages, filter]);
    
    return {
        messages,
        filteredMessages,
        filter,
        setFilter,
        fetchMessages,
        clearMessages: () => setMessages([]),
    };
}
```

---

## Type Definitions

### Flow Types

```typescript
// types/flow.ts
export interface Flow {
    id: string;
    name: string;
    description?: string;
    nodes: Record<string, Node>;
    connections: Connection[];
    config: FlowConfig;
    status: FlowStatus;
    createdAt: string;
    updatedAt: string;
    version: string;
}

export type FlowStatus = 
    | 'draft' 
    | 'running' 
    | 'error' 
    | 'deploying' 
    | 'undeploying';

export interface FlowConfig {
    timeout: number; // seconds
    maxConcurrency: number;
    environment: Record<string, string>;
    retryPolicy: RetryPolicy;
}

export interface RetryPolicy {
    maxRetries: number;
    backoff: number; // seconds
    maxBackoff: number; // seconds
    retryOn: string[];
}
```

### Node Types

```typescript
// types/node.ts
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

export interface NodeStatus {
    state: 'idle' | 'processing' | 'error';
    message: string;
    timestamp: string;
    processingCount: number;
    errorCount: number;
}

export interface NodeConnection {
    id: string;
    sourceNode: string;
    sourcePort?: string;
    targetNode: string;
    targetPort?: string;
}

// Node type metadata
export interface NodeType {
    id: string;
    name: string;
    description: string;
    category: string;
    icon: string;
    color: string;
    inputPorts: string[];
    outputPorts: string[];
    configSchema: Record<string, ConfigProperty>;
    hidden?: boolean;
    deprecated?: boolean;
}

export interface ConfigProperty {
    type: 'string' | 'number' | 'boolean' | 'array' | 'object';
    default: any;
    required: boolean;
    description: string;
    placeholder?: string;
    options?: string[];
    min?: number;
    max?: number;
    pattern?: string;
    editor?: 'text' | 'textarea' | 'number' | 'checkbox' | 'select' | 'code' | 'password';
    editorConfig?: Record<string, any>;
}
```

### Message Types

```typescript
// types/message.ts
export interface Message {
    id: string;
    flowId: string;
    payload: Record<string, any>;
    path: string[]; // Node IDs the message has traversed
    timestamp: string;
    metadata?: Record<string, any>;
}
```

### API Types

```typescript
// types/api.ts
export interface ApiResponse<T> {
    data?: T;
    error?: string;
    message?: string;
    status: number;
}

export interface FlowListResponse {
    flows: Flow[];
}

export interface FlowResponse {
    flow: Flow;
}

export interface NodeListResponse {
    nodes: NodeType[];
}

export interface MessageLogResponse {
    messages: Message[];
}
```

---

## Testing Guidelines

### Test Structure

Tests are organized in the `web/src/test/` directory:

```
test/
├── setup.ts             # Test setup (vi.setConfig)
├── flowCanvas.test.tsx  # Flow canvas component tests
├── types.test.ts        # Type validation tests
├── utils.test.tsx       # Utility function tests
└── nodePalette.test.tsx # Node palette tests (future)
```

### Test Setup

```typescript
// test/setup.ts
import { beforeAll, afterAll, vi } from 'vitest';

// Mock WebSocket
class MockWebSocket {
    onopen: (() => void) | null = null;
    onclose: (() => void) | null = null;
    onmessage: ((event: MessageEvent) => void) | null = null;
    onerror: ((error: Error) => void) | null = null;
    readyState = WebSocket.OPEN;
    
    send(message: string) {
        if (this.onmessage) {
            this.onmessage({ data: message } as MessageEvent);
        }
    }
    
    close() {
        if (this.onclose) this.onclose();
    }
}

beforeAll(() => {
    global.WebSocket = MockWebSocket as any;
    global.fetch = vi.fn();
});

afterAll(() => {
    vi.restoreAllMocks();
});
```

### Component Tests

```typescript
// test/flowCanvas.test.tsx
import { render, screen, fireEvent } from '@testing-library/react';
import { FlowCanvas } from '../components/FlowCanvas';
import { FlowProvider } from '../components/FlowProvider';

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
        render(
            <FlowProvider>
                <FlowCanvas flow={mockFlow} />
            </FlowProvider>
        );
        
        expect(screen.getByTestId('flow-canvas')).toBeInTheDocument();
    });
    
    it('displays nodes', () => {
        render(
            <FlowProvider>
                <FlowCanvas flow={mockFlow} />
            </FlowProvider>
        );
        
        expect(screen.getByText('node-1')).toBeInTheDocument();
    });
});
```

### Hook Tests

```typescript
// test/hooks.test.tsx
import { renderHook, act } from '@testing-library/react';
import { useFlowStore } from '../components/FlowProvider';

describe('useFlowStore', () => {
    it('initializes with empty state', () => {
        const { result } = renderHook(() => useFlowStore());
        
        expect(result.current.flows).toEqual([]);
        expect(result.current.activeFlowId).toBeNull();
        expect(result.current.isLoading).toBe(false);
    });
    
    it('adds flows correctly', () => {
        const { result } = renderHook(() => useFlowStore());
        
        const mockFlow: Flow = {
            id: 'test-1',
            name: 'Test',
            // ... other required fields
        };
        
        act(() => {
            result.current.setFlows([mockFlow]);
        });
        
        expect(result.current.flows).toHaveLength(1);
        expect(result.current.flows[0].id).toBe('test-1');
    });
});
```

### Type Tests

```typescript
// test/types.test.ts
import { describe, it, expect } from 'vitest';
import { Flow, Node, FlowStatus } from '../types';

describe('Type Definitions', () => {
    it('FlowStatus has correct values', () => {
        const validStatuses: FlowStatus[] = ['draft', 'running', 'error', 'deploying', 'undeploying'];
        
        validStatuses.forEach(status => {
            expect(status).toBeTypeOf('string');
        });
    });
    
    it('Flow has all required fields', () => {
        const flow: Flow = {
            id: 'test',
            name: 'Test',
            nodes: {},
            connections: [],
            config: { timeout: 30, maxConcurrency: 10, environment: {}, retryPolicy: { maxRetries: 3, backoff: 1, maxBackoff: 30, retryOn: [] } },
            status: 'draft',
            createdAt: new Date().toISOString(),
            updatedAt: new Date().toISOString(),
            version: '1.0.0',
        };
        
        expect(flow).toHaveProperty('id');
        expect(flow).toHaveProperty('name');
        expect(flow).toHaveProperty('nodes');
        expect(flow).toHaveProperty('connections');
        expect(flow).toHaveProperty('config');
    });
});
```

---

## Styling Guidelines

### Tailwind CSS

The project uses **Tailwind CSS** for styling:

```typescript
// tailwind.config.js
module.exports = {
    content: [
        "./index.html",
        "./src/**/*.{js,ts,jsx,tsx}",
    ],
    theme: {
        extend: {
            colors: {
                primary: '#3b82f6',
                secondary: '#10b981',
            },
        },
    },
    plugins: [],
};
```

### Styling Conventions

1. **Use utility classes**: Prefer Tailwind utility classes over custom CSS
2. **Component classes**: Use meaningful class names for complex styles
3. **Responsive design**: Use responsive prefixes (sm:, md:, lg:, xl:)
4. **Dark mode**: Consider adding dark mode support

```tsx
// Good - Tailwind classes
<div className="flex flex-col bg-gray-50 p-4 rounded-lg shadow-sm">
    <button className="px-4 py-2 bg-blue-500 text-white rounded hover:bg-blue-600">
        Add Node
    </button>
</div>

// Good - Responsive
<div className="w-full md:w-1/2 lg:w-1/3">
    <NodePalette className="hidden md:block" />
</div>

// Good - Conditional classes
<div className={`${isActive ? 'bg-blue-500' : 'bg-gray-200'} p-2 rounded`}>
    {label}
</div>
```

### Color Scheme

| Purpose | Color | Tailwind Class |
|---------|-------|----------------|
| Primary | Blue | bg-blue-500, text-blue-500 |
| Success | Green | bg-green-500, text-green-500 |
| Warning | Yellow | bg-yellow-500, text-yellow-500 |
| Error | Red | bg-red-500, text-red-500 |
| Background | Light Gray | bg-gray-50 |
| Surface | White | bg-white |
| Text Primary | Gray 900 | text-gray-900 |
| Text Secondary | Gray 600 | text-gray-600 |
| Border | Gray 300 | border-gray-300 |

---

## Node UI Components

Each node type should have a corresponding React component for rendering in the canvas:

```tsx
// components/NodeComponent.tsx
import { memo } from 'react';
import { Node as RFNode, useNodeId } from 'reactflow';
import { Node } from '../types/node';

interface NodeComponentProps {
    data: {
        label: string;
        node: Node;
    };
}

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
    
    return (
        <div className="rounded shadow-md" style={{ backgroundColor: nodeType?.color }}>
            <div className="p-2">
                <div className="flex items-center gap-2">
                    {nodeType?.icon && (
                        <span className="text-white">{getIcon(nodeType.icon)}</span>
                    )}
                    <span className="text-white font-medium">{node.name || node.type}</span>
                </div>
            </div>
            <div className="px-2 pb-2">
                {/* Node-specific content */}
                {node.type === 'debug' && (
                    <DebugNodeContent node={node} />
                )}
                {node.type === 'function' && (
                    <FunctionNodeContent node={node} />
                )}
            </div>
            <div className={`h-1 ${getStatusColor()}`} />
        </div>
    );
});

// Custom node content components
function DebugNodeContent({ node }: { node: Node }) {
    return <div className="text-xs text-white/80">Output: {node.config?.output || 'all'}</div>;
}

function FunctionNodeContent({ node }: { node: Node }) {
    return <div className="text-xs text-white/80">Func: {node.config?.function || ''}</div>;
}
```

---

## Checklist for Frontend Changes

Before committing changes to the frontend:

- [ ] All existing tests pass (`npm test`)
- [ ] TypeScript compilation succeeds (`npm run typecheck`)
- [ ] ESLint passes (`npm run lint`)
- [ ] No console errors in development mode
- [ ] Responsive design works on different screen sizes
- [ ] WebSocket communication works correctly
- [ ] REST API calls work correctly
- [ ] State management is correct
- [ ] Error handling is implemented
- [ ] Loading states are shown appropriately
- [ ] Accessibility is maintained
- [ ] Styling is consistent with existing code
- [ ] Backend integration still works

---

*Last updated: 2026-06-21*
*Overrides: None (extends root AGENTS.md)*
