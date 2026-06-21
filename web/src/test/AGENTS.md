# Go-RED Frontend Test Guidelines

This file contains **test-specific** guidelines for frontend tests in `web/src/test/`.

---

## Overview

The `test/` directory contains all **frontend tests** for the Go-RED Web UI. Tests use **Vitest** as the test runner and **@testing-library/react** for component testing.

```
test/
├── setup.ts             # Global test setup and mocks
├── flowCanvas.test.tsx  # FlowCanvas component tests
├── types.test.ts        # Type definition tests
└── utils.test.tsx       # Utility function tests
```

---

## Test Philosophy

### Testing Pyramid

```
          ┌─────────┐
          │   E2E   │   ← Few, slow, high-level
          └────┬────┘
           ┌───┴───┐
          │Integration│   ← Some, medium speed
           └───┬───┘
               │
          ┌────┴────┐
          │  Unit   │   ← Many, fast, low-level
          └─────────┘
```

### Test Priorities

1. **Unit Tests**: Test individual functions, components, and hooks in isolation
2. **Integration Tests**: Test interaction between components and hooks
3. **E2E Tests**: Test complete user flows (may be in a separate directory)

### Test Characteristics

- **Deterministic**: Tests should produce the same result every time
- **Isolated**: Tests should not depend on each other
- **Fast**: Tests should run quickly
- **Maintainable**: Tests should be easy to understand and update
- **Reliable**: Tests should not flake (intermittent failures)

---

## Test Setup

### Global Test Setup (`setup.ts`)

The `setup.ts` file configures the global test environment:

```typescript
// test/setup.ts
import { beforeAll, afterAll, afterEach, vi } from 'vitest';
import { cleanup } from '@testing-library/react';
import '@testing-library/jest-dom';

// Mock global objects
// ====================

// Mock WebSocket
global.WebSocket = class MockWebSocket {
    static OPEN = WebSocket.OPEN;
    static CONNECTING = WebSocket.CONNECTING;
    static CLOSED = WebSocket.CLOSED;
    
    onopen: (() => void) | null = null;
    onclose: (() => void) | null = null;
    onmessage: ((event: MessageEvent) => void) | null = null;
    onerror: ((error: Error) => void) | null = null;
    readyState = WebSocket.CONNECTING;
    
    // Store sent messages for verification
    sentMessages: string[] = [];
    
    constructor(url: string) {
        // Auto-connect in tests
        setTimeout(() => {
            this.readyState = WebSocket.OPEN;
            this.onopen?.();
        }, 0);
    }
    
    send(message: string) {
        this.sentMessages.push(message);
        if (this.onmessage) {
            this.onmessage({ data: message } as MessageEvent);
        }
    }
    
    close() {
        this.readyState = WebSocket.CLOSED;
        this.onclose?.();
    }
    
    // Helper for tests
    simulateMessage(data: string) {
        if (this.onmessage) {
            this.onmessage({ data } as MessageEvent);
        }
    }
    
    simulateError(message: string) {
        if (this.onerror) {
            this.onerror(new Error(message));
        }
    }
} as any;

// Mock fetch
global.fetch = vi.fn(() =>
    Promise.resolve({
        ok: true,
        status: 200,
        json: () => Promise.resolve({}),
        text: () => Promise.resolve(''),
    })
);

// Mock console methods to prevent noise
const originalConsoleError = console.error;
const originalConsoleWarn = console.warn;

beforeAll(() => {
    // Suppress console.error and console.warn in tests
    // unless explicitly wanted
    console.error = vi.fn((...args) => {
        if (process.env.CONSOLE_ERROR !== 'true') {
            return;
        }
        originalConsoleError(...args);
    });
    
    console.warn = vi.fn((...args) => {
        if (process.env.CONSOLE_WARN !== 'true') {
            return;
        }
        originalConsoleWarn(...args);
    });
});

afterAll(() => {
    vi.restoreAllMocks();
});

// Cleanup after each test
afterEach(() => {
    cleanup();
    vi.clearAllMocks();
});

// Enable @testing-library/jest-dom matchers
expect.extend({
    // ... custom matchers if needed
});
```

### Mock Utilities

Create reusable mock utilities:

```typescript
// test/mocks.ts
import { vi } from 'vitest';
import { Flow, Node, Message, FlowStatus, NodeType } from '../types';

// Mock Flow factory
export function createMockFlow(overrides: Partial<Flow> = {}): Flow {
    return {
        id: `flow-${Math.random().toString(36).substr(2, 9)}`,
        name: 'Mock Flow',
        description: 'A mock flow',
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

// Mock Node factory
export function createMockNode(overrides: Partial<Node> = {}): Node {
    return {
        id: `node-${Math.random().toString(36).substr(2, 9)}`,
        type: 'inject',
        name: 'Mock Node',
        x: 0,
        y: 0,
        config: {},
        disabled: false,
        ...overrides,
    };
}

// Mock Message factory
export function createMockMessage(overrides: Partial<Message> = {}): Message {
    return {
        id: `msg-${Math.random().toString(36).substr(2, 9)}`,
        flowId: 'flow-1',
        payload: { data: 'mock' },
        path: [],
        timestamp: new Date().toISOString(),
        ...overrides,
    };
}

// Mock NodeType factory
export function createMockNodeType(overrides: Partial<NodeType> = {}): NodeType {
    return {
        id: `node-type-${Math.random().toString(36).substr(2, 9)}`,
        name: 'Mock Node Type',
        description: 'A mock node type',
        category: 'utility',
        icon: 'cog',
        color: '#3b82f6',
        inputPorts: ['input'],
        outputPorts: ['output'],
        configSchema: {},
        ...overrides,
    };
}

// Mock flow with nodes
export function createMockFlowWithNodes(nodeCount: number = 3): Flow {
    const flow = createMockFlow();
    const nodes: Record<string, Node> = {};
    const connections: import('../types').NodeConnection[] = [];
    
    for (let i = 0; i < nodeCount; i++) {
        const nodeId = `node-${i}`;
        nodes[nodeId] = createMockNode({
            id: nodeId,
            x: i * 100,
            y: 0,
        });
        
        if (i > 0) {
            connections.push({
                id: `conn-${i}`,
                sourceNode: `node-${i - 1}`,
                targetNode: nodeId,
            });
        }
    }
    
    return {
        ...flow,
        nodes,
        connections,
    };
}

// Mock API responses
export const mockFlowListResponse = {
    flows: [
        createMockFlow({ id: 'flow-1', name: 'Flow 1' }),
        createMockFlow({ id: 'flow-2', name: 'Flow 2' }),
    ],
};

export const mockNodeTypeListResponse = {
    nodes: [
        createMockNodeType({ id: 'inject', name: 'Inject' }),
        createMockNodeType({ id: 'debug', name: 'Debug' }),
        createMockNodeType({ id: 'function', name: 'Function' }),
    ],
};
```

---

## Test File Structure

### Unit Test Structure

```typescript
// test/[name].test.tsx (or .ts for non-component tests)

// 1. Imports
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MyComponent } from '../components/MyComponent';
import { createMockFlow } from './mocks';

// 2. Mock setup (if needed)
vi.mock('../hooks/useWebSocket');
vi.mock('../utils/api');

// 3. Test suite
describe('MyComponent', () => {
    // 3a. Setup
    const mockProps = {
        flow: createMockFlow(),
        onChange: vi.fn(),
    };
    
    beforeEach(() => {
        vi.clearAllMocks();
    });
    
    afterEach(() => {
        // Cleanup
    });
    
    // 3b. Test cases
    
    describe('Rendering', () => {
        it('renders without crashing', () => {
            render(<MyComponent {...mockProps} />);
            expect(screen.getByTestId('my-component')).toBeInTheDocument();
        });
        
        it('displays the flow name', () => {
            render(<MyComponent {...mockProps} />);
            expect(screen.getByText(mockProps.flow.name)).toBeInTheDocument();
        });
    });
    
    describe('User Interaction', () => {
        it('calls onChange when user interacts', async () => {
            const user = userEvent.setup();
            render(<MyComponent {...mockProps} />);
            
            await user.click(screen.getByRole('button'));
            
            expect(mockProps.onChange).toHaveBeenCalled();
        });
    });
    
    describe('Edge Cases', () => {
        it('handles null flow gracefully', () => {
            render(<MyComponent flow={null} onChange={mockProps.onChange} />);
            expect(screen.getByText('No flow')).toBeInTheDocument();
        });
    });
});
```

---

## Component Testing

### Testing with @testing-library/react

**Best Practices:**

1. **Use semantic queries**: Prefer `getByRole` over `getByTestId`
2. **Test user behavior**: Test what users can do, not implementation
3. **Use `waitFor` for async**: When waiting for state updates
4. **Use `userEvent`**: For realistic user interaction simulation
5. **Clean up**: Always clean up after tests

```typescript
// test/FlowCanvas.test.tsx
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { FlowCanvas } from '../components/FlowCanvas';
import { createMockFlowWithNodes } from './mocks';
import { FlowProvider } from '../components/FlowProvider';

describe('FlowCanvas', () => {
    const mockFlow = createMockFlowWithNodes(3);
    
    beforeEach(() => {
        // Mock ReactFlow if needed
        vi.mock('reactflow', async () => {
            const actual = await vi.importActual('reactflow');
            return {
                ...actual,
                // Mock specific components if needed
            };
        });
    });
    
    it('renders without crashing', () => {
        render(
            <FlowProvider>
                <FlowCanvas flow={mockFlow} />
            </FlowProvider>
        );
        
        expect(screen.getByTestId('flow-canvas')).toBeInTheDocument();
    });
    
    it('displays all nodes', () => {
        render(
            <FlowProvider>
                <FlowCanvas flow={mockFlow} />
            </FlowProvider>
        );
        
        Object.keys(mockFlow.nodes).forEach(nodeId => {
            // Assuming nodes are rendered with their ID
            expect(screen.getByText(nodeId)).toBeInTheDocument();
        });
    });
    
    it('handles node selection', async () => {
        const user = userEvent.setup();
        
        render(
            <FlowProvider>
                <FlowCanvas flow={mockFlow} />
            </FlowProvider>
        );
        
        // Find and click on a node
        const node = screen.getByText('node-0');
        await user.click(node);
        
        // Verify selection (implementation-specific)
        // expect(node).toHaveClass('selected');
    });
    
    it('updates when flow changes', async () => {
        const { rerender } = render(
            <FlowProvider>
                <FlowCanvas flow={mockFlow} />
            </FlowProvider>
        );
        
        const updatedFlow = createMockFlowWithNodes(5);
        
        rerender(
            <FlowProvider>
                <FlowCanvas flow={updatedFlow} />
            </FlowProvider>
        );
        
        // Verify new nodes are rendered
        await waitFor(() => {
            Object.keys(updatedFlow.nodes).forEach(nodeId => {
                expect(screen.getByText(nodeId)).toBeInTheDocument();
            });
        });
    });
});
```

### Testing ReactFlow Components

Testing components that use ReactFlow requires special handling:

```typescript
// test/FlowCanvas.test.tsx
import { render } from '@testing-library/react';
import { FlowCanvas } from '../components/FlowCanvas';
import { createMockFlow } from './mocks';

// Mock ReactFlow
vi.mock('reactflow', async () => {
    const actual = await vi.importActual('reactflow');
    
    return {
        ...actual,
        // Mock the default ReactFlow component
        default: vi.fn(({ children }) => (
            <div data-testid="react-flow">
                {children}
            </div>
        )),
        ReactFlowProvider: vi.fn(({ children }) => children),
        useNodesState: () => [
            [],
            vi.fn(),
            vi.fn()
        ],
        useEdgesState: () => [
            [],
            vi.fn(),
            vi.fn()
        ],
        addEdge: vi.fn(),
        // ... other exports
    };
});

describe('FlowCanvas with mocked ReactFlow', () => {
    const mockFlow = createMockFlow();
    
    it('renders the canvas wrapper', () => {
        render(<FlowCanvas flow={mockFlow} />);
        
        expect(screen.getByTestId('flow-canvas')).toBeInTheDocument();
    });
});
```

---

## Hook Testing

### Testing Custom Hooks

Use `renderHook` from `@testing-library/react`:

```typescript
// test/hooks.test.tsx
import { renderHook, act } from '@testing-library/react';
import { useFlows } from '../hooks/useFlows';
import { createMockFlow } from './mocks';
import * as api from '../utils/api';

vi.mock('../utils/api');
vi.mock('../hooks/useWebSocket');

describe('useFlows', () => {
    const mockFlows = [
        createMockFlow({ id: 'flow-1', name: 'Flow 1' }),
        createMockFlow({ id: 'flow-2', name: 'Flow 2' }),
    ];
    
    beforeEach(() => {
        vi.clearAllMocks();
        vi.mocked(api.fetchFlows).mockResolvedValue(mockFlows);
    });
    
    it('initializes with empty state', () => {
        const { result } = renderHook(() => useFlows());
        
        expect(result.current.flows).toEqual([]);
        expect(result.current.activeFlowId).toBeNull();
        expect(result.current.isLoading).toBe(true);
        expect(result.current.error).toBeNull();
    });
    
    it('loads flows on mount', async () => {
        const { result, waitForNextUpdate } = renderHook(() => useFlows());
        
        await waitForNextUpdate();
        
        expect(api.fetchFlows).toHaveBeenCalled();
        expect(result.current.flows).toEqual(mockFlows);
        expect(result.current.isLoading).toBe(false);
    });
    
    it('handles API errors', async () => {
        const mockError = new Error('Failed to fetch');
        vi.mocked(api.fetchFlows).mockRejectedValue(mockError);
        
        const { result, waitForNextUpdate } = renderHook(() => useFlows());
        
        await waitForNextUpdate();
        
        expect(result.current.error).toBe(mockError.message);
        expect(result.current.isLoading).toBe(false);
    });
    
    it('sets active flow', () => {
        const { result } = renderHook(() => useFlows());
        
        act(() => {
            result.current.setActiveFlowId('flow-1');
        });
        
        expect(result.current.activeFlowId).toBe('flow-1');
    });
    
    it('gets active flow', async () => {
        const { result, waitForNextUpdate } = renderHook(() => useFlows());
        
        await waitForNextUpdate();
        
        act(() => {
            result.current.setActiveFlowId('flow-1');
        });
        
        expect(result.current.activeFlow).toEqual(mockFlows[0]);
    });
    
    it('creates a new flow', async () => {
        const newFlow = createMockFlow({ id: 'flow-3', name: 'Flow 3' });
        vi.mocked(api.createFlow).mockResolvedValue(newFlow);
        
        const { result, waitForNextUpdate } = renderHook(() => useFlows());
        
        await waitForNextUpdate();
        
        await act(async () => {
            const created = await result.current.createFlow({ name: 'Flow 3' });
            expect(created).toEqual(newFlow);
        });
        
        expect(api.createFlow).toHaveBeenCalledWith({ name: 'Flow 3' });
    });
});
```

---

## WebSocket Testing

### Testing useWebSocket Hook

```typescript
// test/useWebSocket.test.tsx
import { renderHook, act } from '@testing-library/react';
import { useWebSocket } from '../hooks/useWebSocket';

describe('useWebSocket', () => {
    const mockUrl = 'ws://localhost:8080/ws';
    
    beforeEach(() => {
        // Reset mock
        global.WebSocket = class MockWebSocket {
            onopen: (() => void) | null = null;
            onclose: (() => void) | null = null;
            onmessage: ((event: MessageEvent) => void) | null = null;
            onerror: ((error: Error) => void) | null = null;
            readyState = WebSocket.CONNECTING;
            sentMessages: string[] = [];
            
            constructor(url: string) {
                setTimeout(() => {
                    this.readyState = WebSocket.OPEN;
                    this.onopen?.();
                }, 0);
            }
            
            send(message: string) {
                this.sentMessages.push(message);
                if (this.onmessage) {
                    this.onmessage({ data: message } as MessageEvent);
                }
            }
            
            close() {
                this.readyState = WebSocket.CLOSED;
                this.onclose?.();
            }
            
            simulateMessage(data: string) {
                if (this.onmessage) {
                    this.onmessage({ data } as MessageEvent);
                }
            }
        } as any;
    });
    
    it('initializes with disconnected state', () => {
        const { result } = renderHook(() => useWebSocket(mockUrl));
        
        expect(result.current.isConnected).toBe(false);
        expect(result.current.socket).toBeNull();
        expect(result.current.lastError).toBeNull();
    });
    
    it('connects successfully', async () => {
        const { result, waitForNextUpdate } = renderHook(() => useWebSocket(mockUrl));
        
        await waitForNextUpdate();
        
        expect(result.current.isConnected).toBe(true);
        expect(result.current.socket).toBeInstanceOf(WebSocket);
    });
    
    it('sends messages', async () => {
        const { result, waitForNextUpdate } = renderHook(() => useWebSocket(mockUrl));
        
        await waitForNextUpdate();
        
        const mockMessage = { type: 'test', payload: { data: 'test' } };
        
        act(() => {
            result.current.sendMessage(mockMessage);
        });
        
        const socket = result.current.socket as any;
        expect(socket.sentMessages).toContainEqual(JSON.stringify(mockMessage));
        expect(result.current.lastError).toBeNull();
    });
    
    it('handles incoming messages', async () => {
        const { result, waitForNextUpdate } = renderHook(() => useWebSocket(mockUrl));
        
        await waitForNextUpdate();
        
        const mockHandler = vi.fn();
        const mockMessage = { type: 'test', payload: { data: 'received' } };
        
        act(() => {
            result.current.registerHandler('test', mockHandler);
        });
        
        act(() => {
            (result.current.socket as any).simulateMessage(JSON.stringify(mockMessage));
        });
        
        expect(mockHandler).toHaveBeenCalledWith(mockMessage.payload);
    });
    
    it('handles connection errors', async () => {
        global.WebSocket = class ErrorWebSocket {
            onerror: ((error: Error) => void) | null = null;
            
            constructor() {
                setTimeout(() => {
                    this.onerror?.(new Error('Connection failed'));
                }, 0);
            }
        } as any;
        
        const { result, waitForNextUpdate } = renderHook(() => useWebSocket(mockUrl));
        
        await waitForNextUpdate();
        
        expect(result.current.lastError).toBe('Connection failed');
        expect(result.current.isConnected).toBe(false);
    });
    
    it('reconnects on connection loss', async () => {
        let connectCount = 0;
        
        global.WebSocket = class ReconnectWebSocket {
            onopen: (() => void) | null = null;
            onclose: (() => void) | null = null;
            readyState = WebSocket.CONNECTING;
            
            constructor() {
                connectCount++;
                setTimeout(() => {
                    this.readyState = WebSocket.OPEN;
                    this.onopen?.();
                }, 0);
            }
            
            close() {
                this.readyState = WebSocket.CLOSED;
                this.onclose?.();
            }
        } as any;
        
        const { result, waitForNextUpdate } = renderHook(() => useWebSocket(mockUrl));
        
        await waitForNextUpdate();
        expect(connectCount).toBe(1);
        
        // Simulate disconnection
        act(() => {
            (result.current.socket as any).close();
        });
        
        // Wait for reconnection
        await waitForNextUpdate();
        expect(connectCount).toBe(2);
    });
});
```

---

## Type Testing

Test type definitions and type guards:

```typescript
// test/types.test.ts
import { describe, it, expect } from 'vitest';
import { 
    Flow, 
    FlowStatus, 
    Node,
    Message,
    NodeType,
    isFlowStatus,
    isFlow,
    isNode,
    isMessage,
    validateFlow,
    FlowConfig,
} from '../types';

describe('Type Definitions', () => {
    describe('FlowStatus', () => {
        it('has all valid values', () => {
            const validStatuses: FlowStatus[] = ['draft', 'running', 'error', 'deploying', 'undeploying'];
            validStatuses.forEach(status => {
                expect(status).toBeTypeOf('string');
            });
        });
        
        it('isFlowStatus validates correctly', () => {
            expect(isFlowStatus('draft')).toBe(true);
            expect(isFlowStatus('running')).toBe(true);
            expect(isFlowStatus('error')).toBe(true);
            expect(isFlowStatus('deploying')).toBe(true);
            expect(isFlowStatus('undeploying')).toBe(true);
            expect(isFlowStatus('invalid')).toBe(false);
            expect(isFlowStatus('')).toBe(false);
            expect(isFlowStatus('DRAFT')).toBe(false); // Case-sensitive
        });
    });
    
    describe('Flow', () => {
        it('has all required properties', () => {
            const flow: Flow = {
                id: 'flow-1',
                name: 'Test Flow',
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
            };
            
            expect(flow).toHaveProperty('id');
            expect(flow).toHaveProperty('name');
            expect(flow).toHaveProperty('nodes');
            expect(flow).toHaveProperty('connections');
            expect(flow).toHaveProperty('config');
            expect(flow).toHaveProperty('status');
            expect(flow).toHaveProperty('createdAt');
            expect(flow).toHaveProperty('updatedAt');
            expect(flow).toHaveProperty('version');
        });
        
        it('isFlow validates correctly', () => {
            const validFlow: Flow = {
                id: 'flow-1',
                name: 'Test Flow',
                nodes: {},
                connections: [],
                config: {
                    timeout: 30,
                    maxConcurrency: 10,
                    environment: {},
                    retryPolicy: { maxRetries: 3, backoff: 1, maxBackoff: 30, retryOn: [] },
                },
                status: 'draft',
                createdAt: new Date().toISOString(),
                updatedAt: new Date().toISOString(),
                version: '1.0.0',
            };
            
            expect(isFlow(validFlow)).toBe(true);
            expect(isFlow({})).toBe(false);
            expect(isFlow(null)).toBe(false);
            expect(isFlow(undefined)).toBe(false);
            expect(isFlow('not a flow')).toBe(false);
        });
    });
    
    describe('Node', () => {
        it('has all required properties', () => {
            const node: Node = {
                id: 'node-1',
                type: 'inject',
                x: 0,
                y: 0,
                config: {},
                disabled: false,
            };
            
            expect(node).toHaveProperty('id');
            expect(node).toHaveProperty('type');
            expect(node).toHaveProperty('x');
            expect(node).toHaveProperty('y');
            expect(node).toHaveProperty('config');
            expect(node).toHaveProperty('disabled');
        });
        
        it('isNode validates correctly', () => {
            const validNode: Node = {
                id: 'node-1',
                type: 'inject',
                x: 0,
                y: 0,
                config: {},
                disabled: false,
            };
            
            expect(isNode(validNode)).toBe(true);
            expect(isNode({})).toBe(false);
            expect(isNode(null)).toBe(false);
            expect(isNode({ id: 'node-1' })).toBe(false); // Missing required properties
        });
    });
    
    describe('Message', () => {
        it('has all required properties', () => {
            const message: Message = {
                id: 'msg-1',
                flowId: 'flow-1',
                payload: { data: 'test' },
                path: ['node-1', 'node-2'],
                timestamp: new Date().toISOString(),
            };
            
            expect(message).toHaveProperty('id');
            expect(message).toHaveProperty('flowId');
            expect(message).toHaveProperty('payload');
            expect(message).toHaveProperty('path');
            expect(message).toHaveProperty('timestamp');
        });
        
        it('isMessage validates correctly', () => {
            const validMessage: Message = {
                id: 'msg-1',
                flowId: 'flow-1',
                payload: {},
                path: [],
                timestamp: new Date().toISOString(),
            };
            
            expect(isMessage(validMessage)).toBe(true);
            expect(isMessage({})).toBe(false);
            expect(isMessage(null)).toBe(false);
        });
    });
    
    describe('validateFlow', () => {
        it('returns empty array for valid flow', () => {
            const validFlow: Flow = {
                id: 'flow-1',
                name: 'Test Flow',
                nodes: {},
                connections: [],
                config: {
                    timeout: 30,
                    maxConcurrency: 10,
                    environment: {},
                    retryPolicy: { maxRetries: 3, backoff: 1, maxBackoff: 30, retryOn: [] },
                },
                status: 'draft',
                createdAt: new Date().toISOString(),
                updatedAt: new Date().toISOString(),
                version: '1.0.0',
            };
            
            expect(validateFlow(validFlow)).toEqual([]);
        });
        
        it('returns errors for missing required fields', () => {
            const invalidFlow = {
                id: '',
                name: 'Test Flow',
                nodes: {},
                connections: [],
                config: {
                    timeout: 30,
                    maxConcurrency: 10,
                    environment: {},
                    retryPolicy: { maxRetries: 3, backoff: 1, maxBackoff: 30, retryOn: [] },
                },
                status: 'draft',
                createdAt: new Date().toISOString(),
                updatedAt: new Date().toISOString(),
                version: '1.0.0',
            };
            
            const errors = validateFlow(invalidFlow);
            expect(errors).toContain('Flow ID is required');
        });
        
        it('returns errors for invalid node', () => {
            const flow = {
                id: 'flow-1',
                name: 'Test Flow',
                nodes: {
                    'node-1': { id: 'wrong-id', type: 'inject', x: 0, y: 0, config: {}, disabled: false },
                },
                connections: [],
                config: {
                    timeout: 30,
                    maxConcurrency: 10,
                    environment: {},
                    retryPolicy: { maxRetries: 3, backoff: 1, maxBackoff: 30, retryOn: [] },
                },
                status: 'draft',
                createdAt: new Date().toISOString(),
                updatedAt: new Date().toISOString(),
                version: '1.0.0',
            };
            
            const errors = validateFlow(flow);
            expect(errors).toContain('Node ID mismatch: node-1');
        });
    });
});
```

---

## Utility Function Testing

Test utility functions for correctness:

```typescript
// test/utils.test.ts
import { describe, it, expect } from 'vitest';
import { 
    createDefaultFlow,
    validateFlow,
    generateId,
    convertToReactFlowNodes,
    convertToReactFlowEdges,
} from '../utils';
import { Flow, Node, NodeConnection } from '../types';
import { createMockFlow, createMockNode } from './mocks';

describe('Utility Functions', () => {
    describe('createDefaultFlow', () => {
        it('creates a flow with default values', () => {
            const flow = createDefaultFlow();
            
            expect(flow).toHaveProperty('id');
            expect(flow).toHaveProperty('name', 'New Flow');
            expect(flow).toHaveProperty('nodes', {});
            expect(flow).toHaveProperty('connections', []);
            expect(flow).toHaveProperty('config');
            expect(flow).toHaveProperty('status', 'draft');
        });
        
        it('applies overrides', () => {
            const flow = createDefaultFlow({
                name: 'Custom Flow',
                id: 'custom-id',
            });
            
            expect(flow.name).toBe('Custom Flow');
            expect(flow.id).toBe('custom-id');
        });
    });
    
    describe('generateId', () => {
        it('generates unique IDs', () => {
            const id1 = generateId('test');
            const id2 = generateId('test');
            
            expect(id1).not.toBe(id2);
            expect(id1.startsWith('test-')).toBe(true);
            expect(id2.startsWith('test-')).toBe(true);
        });
        
        it('generates IDs with different prefixes', () => {
            const flowId = generateId('flow');
            const nodeId = generateId('node');
            
            expect(flowId.startsWith('flow-')).toBe(true);
            expect(nodeId.startsWith('node-')).toBe(true);
        });
    });
    
    describe('convertToReactFlowNodes', () => {
        it('converts domain nodes to ReactFlow nodes', () => {
            const nodes: Record<string, Node> = {
                'node-1': createMockNode({ id: 'node-1', x: 100, y: 200 }),
                'node-2': createMockNode({ id: 'node-2', x: 300, y: 400 }),
            };
            
            const rfNodes = convertToReactFlowNodes(nodes);
            
            expect(rfNodes).toHaveLength(2);
            expect(rfNodes[0].id).toBe('node-1');
            expect(rfNodes[0].position.x).toBe(100);
            expect(rfNodes[0].position.y).toBe(200);
            expect(rfNodes[1].id).toBe('node-2');
        });
        
        it('returns empty array for empty input', () => {
            const rfNodes = convertToReactFlowNodes({});
            expect(rfNodes).toEqual([]);
        });
    });
    
    describe('convertToReactFlowEdges', () => {
        it('converts domain connections to ReactFlow edges', () => {
            const connections: NodeConnection[] = [
                { id: 'conn-1', sourceNode: 'node-1', targetNode: 'node-2' },
                { id: 'conn-2', sourceNode: 'node-2', targetNode: 'node-3' },
            ];
            
            const rfEdges = convertToReactFlowEdges(connections);
            
            expect(rfEdges).toHaveLength(2);
            expect(rfEdges[0].id).toBe('conn-1');
            expect(rfEdges[0].source).toBe('node-1');
            expect(rfEdges[0].target).toBe('node-2');
        });
        
        it('returns empty array for empty input', () => {
            const rfEdges = convertToReactFlowEdges([]);
            expect(rfEdges).toEqual([]);
        });
    });
});
```

---

## Test Best Practices

### 1. Test Descriptions

- **Be specific**: Describe exactly what is being tested
- **Use 