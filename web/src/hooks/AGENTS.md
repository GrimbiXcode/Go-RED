# Go-RED Hooks Guidelines

This file contains **hook-specific** guidelines for custom React hooks in `web/src/hooks/`.

---

## Overview

The `hooks/` directory contains all **custom React hooks** for the Go-RED Web UI. These hooks encapsulate reusable logic and provide a clean API for components to interact with the application state and external services.

```
hooks/
├── index.ts            # Re-exports all hooks
├── useFlows.ts         # Flow state management hook
├── useMessageLog.ts    # Message log management hook
└── useWebSocket.ts     # WebSocket communication hook
```

---

## Hook Design Principles

### 1. Single Responsibility
Each hook should have **one clear responsibility**:
- `useWebSocket` - Manages WebSocket connection
- `useFlows` - Manages flow state and operations
- `useMessageLog` - Manages message log state

### 2. Reusability
Hooks should be **reusable across multiple components**. Avoid hooks that are only used in one place.

### 3. Type Safety
All hooks should be **fully typed** with TypeScript interfaces for inputs, outputs, and return values.

### 4. Clean API
Hooks should provide a **simple, intuitive API** to components. Hide implementation details.

### 5. Error Handling
Hooks should handle errors gracefully and provide error state to components.

### 6. Performance
Hooks should be optimized to prevent unnecessary re-renders and computations.

---

## Hook Development Guidelines

### Hook Structure

Each hook should follow this structure:

```typescript
// hooks/useExample.ts

// 1. Imports
import { useState, useEffect, useCallback } from 'react';
import { SomeType } from '../types';

// 2. Types/Interfaces
interface UseExampleOptions {
    initialValue?: SomeType;
    onChange?: (value: SomeType) => void;
}

interface UseExampleResult {
    value: SomeType;
    setValue: (value: SomeType) => void;
    isLoading: boolean;
    error: Error | null;
    reset: () => void;
}

// 3. Default values
const DEFAULT_OPTIONS: UseExampleOptions = {
    initialValue: { /* default */ },
};

// 4. Hook implementation
export function useExample(options: UseExampleOptions = {}): UseExampleResult {
    // Merge options with defaults
    const { initialValue, onChange } = { ...DEFAULT_OPTIONS, ...options };
    
    // State
    const [value, setValue] = useState<SomeType>(initialValue);
    const [isLoading, setIsLoading] = useState(false);
    const [error, setError] = useState<Error | null>(null);
    
    // Effects
    useEffect(() => {
        // Side effects
        return () => {
            // Cleanup
        };
    }, [/* dependencies */]);
    
    // Memoized callbacks
    const handleSetValue = useCallback((newValue: SomeType) => {
        setValue(newValue);
        onChange?.(newValue);
    }, [onChange]);
    
    const reset = useCallback(() => {
        setValue(initialValue);
        setError(null);
    }, [initialValue]);
    
    // Return API
    return {
        value,
        setValue: handleSetValue,
        isLoading,
        error,
        reset,
    };
}

// 5. Export
// (Already exported above)
```

### Hook Best Practices

1. **Use descriptive names**: Hooks should clearly indicate what they do
2. **Prefix with `use`**: All hooks must start with `use`
3. **Document the API**: Add JSDoc comments explaining the hook's purpose, parameters, and return values
4. **Handle dependencies**: Use `useCallback` and `useMemo` appropriately
5. **Clean up resources**: Clean up subscriptions, timers, and event listeners
6. **Isolate side effects**: Keep side effects in `useEffect` hooks
7. **Provide clear error states**: Make it easy for components to handle errors
8. **Avoid prop drilling**: Hooks should access the data they need directly

---

## Application Hooks

### useWebSocket.ts

**Purpose**: Manage WebSocket connection and messaging with the backend.

**Responsibilities**:
- Establish and maintain WebSocket connection
- Handle connection state (connected, disconnected, error)
- Send messages to the server
- Receive and distribute messages from the server
- Handle reconnection logic

**API**:
```typescript
interface WebSocketMessage {
    type: string;
    payload?: any;
}

interface UseWebSocketResult {
    socket: WebSocket | null;
    isConnected: boolean;
    lastError: string | null;
    sendMessage: (message: WebSocketMessage) => void;
    reconnect: () => void;
}

function useWebSocket(url: string): UseWebSocketResult
```

**Implementation**:
```typescript
// hooks/useWebSocket.ts
import { useState, useEffect, useCallback, useRef } from 'react';

interface WebSocketMessage {
    type: string;
    payload?: unknown;
}

interface UseWebSocketResult {
    socket: WebSocket | null;
    isConnected: boolean;
    lastError: string | null;
    sendMessage: (message: WebSocketMessage) => void;
    reconnect: () => void;
}

// Message handlers type
interface MessageHandlers {
    [key: string]: (payload: unknown) => void;
}

// Default reconnection settings
const DEFAULT_RECONNECT_DELAY = 5000; // 5 seconds
const MAX_RECONNECT_DELAY = 30000; // 30 seconds

export function useWebSocket(url: string): UseWebSocketResult {
    const [socket, setSocket] = useState<WebSocket | null>(null);
    const [isConnected, setIsConnected] = useState(false);
    const [lastError, setLastError] = useState<string | null>(null);
    
    // Track reconnection delay for exponential backoff
    const reconnectDelayRef = useRef(DEFAULT_RECONNECT_DELAY);
    
    // Message handlers ref
    const messageHandlersRef = useRef<MessageHandlers>({});
    
    // Register message handler
    const registerHandler = useCallback((type: string, handler: (payload: unknown) => void) => {
        messageHandlersRef.current[type] = handler;
        
        // Return cleanup function
        return () => {
            delete messageHandlersRef.current[type];
        };
    }, []);
    
    // Unregister message handler
    const unregisterHandler = useCallback((type: string) => {
        delete messageHandlersRef.current[type];
    }, []);
    
    // Send message
    const sendMessage = useCallback((message: WebSocketMessage) => {
        if (socket?.readyState === WebSocket.OPEN) {
            try {
                socket.send(JSON.stringify(message));
            } catch (error) {
                setLastError(`Failed to send message: ${error}`);
            }
        } else {
            console.warn('WebSocket not connected, message not sent:', message);
            setLastError('WebSocket not connected');
        }
    }, [socket]);
    
    // Reconnect
    const reconnect = useCallback(() => {
        if (socket) {
            socket.close();
        }
    }, [socket]);
    
    // Handle connection
    useEffect(() => {
        let ws: WebSocket;
        let reconnectTimeout: NodeJS.Timeout;
        
        const connect = () => {
            try {
                ws = new WebSocket(url);
                
                ws.onopen = () => {
                    setIsConnected(true);
                    setLastError(null);
                    reconnectDelayRef.current = DEFAULT_RECONNECT_DELAY;
                };
                
                ws.onclose = () => {
                    setIsConnected(false);
                    
                    // Attempt reconnection with exponential backoff
                    if (reconnectDelayRef.current <= MAX_RECONNECT_DELAY) {
                        reconnectTimeout = setTimeout(connect, reconnectDelayRef.current);
                        reconnectDelayRef.current *= 2; // Double the delay
                    }
                };
                
                ws.onerror = (error) => {
                    setLastError(error.message);
                    setIsConnected(false);
                };
                
                ws.onmessage = (event) => {
                    try {
                        const message: WebSocketMessage = JSON.parse(event.data);
                        const handler = messageHandlersRef.current[message.type];
                        
                        if (handler) {
                            handler(message.payload);
                        } else {
                            console.warn('No handler for message type:', message.type);
                        }
                    } catch (error) {
                        console.error('Failed to parse WebSocket message:', error);
                        setLastError('Failed to parse message');
                    }
                };
                
                setSocket(ws);
            } catch (error) {
                setLastError(`Failed to connect: ${error}`);
                // Retry after delay
                reconnectTimeout = setTimeout(connect, reconnectDelayRef.current);
            }
        };
        
        // Initial connection
        connect();
        
        // Cleanup
        return () => {
            clearTimeout(reconnectTimeout);
            ws?.close();
        };
    }, [url]);
    
    return {
        socket,
        isConnected,
        lastError,
        sendMessage,
        reconnect,
        // Additional convenience methods
        registerHandler,
        unregisterHandler,
    };
}
```

**Usage Example**:
```typescript
// In a component
import { useWebSocket } from '../hooks/useWebSocket';

function FlowEditor() {
    const { isConnected, sendMessage, registerHandler } = useWebSocket('/ws');
    
    // Register flow list handler
    useEffect(() => {
        const unregister = registerHandler('flow:list', (payload) => {
            console.log('Received flows:', payload);
        });
        
        return unregister;
    }, [registerHandler]);
    
    const handleRefresh = () => {
        sendMessage({ type: 'flow:list' });
    };
    
    return (
        <div>
            {isConnected ? 'Connected' : 'Disconnected'}
            <button onClick={handleRefresh}>Refresh</button>
        </div>
    );
}
```

**Testing**:
```typescript
// hooks/useWebSocket.test.ts
import { renderHook, act } from '@testing-library/react';
import { useWebSocket } from './useWebSocket';

describe('useWebSocket', () => {
    const mockUrl = 'ws://localhost:8080/ws';
    
    beforeEach(() => {
        // Reset WebSocket mock
        global.WebSocket = class MockWebSocket {
            onopen: (() => void) | null = null;
            onclose: (() => void) | null = null;
            onmessage: ((event: MessageEvent) => void) | null = null;
            onerror: ((error: Error) => void) | null = null;
            readyState = WebSocket.CONNECTING;
            
            constructor(url: string) {
                setTimeout(() => {
                    this.readyState = WebSocket.OPEN;
                    this.onopen?.();
                }, 0);
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
        
        // Verify message was sent (implementation depends on mock)
        expect(result.current.lastError).toBeNull();
    });
    
    it('handles message reception', async () => {
        const { result, waitForNextUpdate } = renderHook(() => useWebSocket(mockUrl));
        
        await waitForNextUpdate();
        
        const mockHandler = vi.fn();
        const mockMessage = { type: 'test', payload: { data: 'received' } };
        
        act(() => {
            result.current.registerHandler('test', mockHandler);
        });
        
        // Simulate receiving message
        act(() => {
            (result.current.socket as any).onmessage({ 
                data: JSON.stringify(mockMessage) 
            } as MessageEvent);
        });
        
        expect(mockHandler).toHaveBeenCalledWith(mockMessage.payload);
    });
});
```

---

### useFlows.ts

**Purpose**: Manage flow state and operations.

**Responsibilities**:
- Fetch and manage list of flows
- Track active flow
- Provide CRUD operations for flows
- Sync with WebSocket updates
- Handle loading and error states

**API**:
```typescript
interface UseFlowsResult {
    flows: Flow[];
    activeFlowId: string | null;
    activeFlow: Flow | undefined;
    isLoading: boolean;
    error: string | null;
    setActiveFlowId: (flowId: string | null) => void;
    refreshFlows: () => Promise<void>;
    createFlow: (flowData: Partial<Flow>) => Promise<Flow>;
    updateFlow: (flowId: string, updates: Partial<Flow>) => Promise<Flow>;
    deleteFlow: (flowId: string) => Promise<void>;
    deployFlow: (flowId: string) => Promise<void>;
    undeployFlow: (flowId: string) => Promise<void>;
}

function useFlows(): UseFlowsResult
```

**Implementation**:
```typescript
// hooks/useFlows.ts
import { useState, useEffect, useCallback } from 'react';
import { Flow } from '../types';
import { useWebSocket } from './useWebSocket';
import { 
    fetchFlows, 
    createFlow as createFlowApi, 
    updateFlow as updateFlowApi,
    deleteFlow as deleteFlowApi,
    deployFlow as deployFlowApi,
    undeployFlow as undeployFlowApi
} from '../utils/api';

interface UseFlowsResult {
    flows: Flow[];
    activeFlowId: string | null;
    activeFlow: Flow | undefined;
    isLoading: boolean;
    error: string | null;
    setActiveFlowId: (flowId: string | null) => void;
    refreshFlows: () => Promise<void>;
    createFlow: (flowData: Partial<Flow>) => Promise<Flow>;
    updateFlow: (flowId: string, updates: Partial<Flow>) => Promise<Flow>;
    deleteFlow: (flowId: string) => Promise<void>;
    deployFlow: (flowId: string) => Promise<void>;
    undeployFlow: (flowId: string) => Promise<void>;
}

export function useFlows(): UseFlowsResult {
    const [flows, setFlows] = useState<Flow[]>([]);
    const [activeFlowId, setActiveFlowId] = useState<string | null>(null);
    const [isLoading, setIsLoading] = useState(false);
    const [error, setError] = useState<string | null>(null);
    
    const { sendMessage, registerHandler } = useWebSocket('/ws');
    
    // Derived state
    const activeFlow = flows.find(f => f.id === activeFlowId);
    
    // Initial load
    useEffect(() => {
        refreshFlows();
    }, []);
    
    // Register WebSocket handlers
    useEffect(() => {
        const unregisterFlowList = registerHandler('flow:list', (payload: { flows: Flow[] }) => {
            setFlows(payload.flows);
            setIsLoading(false);
        });
        
        const unregisterFlowCreated = registerHandler('flow:created', (payload: { flow: Flow }) => {
            setFlows(prev => [...prev, payload.flow]);
        });
        
        const unregisterFlowUpdated = registerHandler('flow:updated', (payload: { flow: Flow }) => {
            setFlows(prev => prev.map(f => f.id === payload.flow.id ? payload.flow : f));
        });
        
        const unregisterFlowDeleted = registerHandler('flow:deleted', (payload: { flowId: string }) => {
            setFlows(prev => prev.filter(f => f.id !== payload.flowId));
            if (activeFlowId === payload.flowId) {
                setActiveFlowId(null);
            }
        });
        
        const unregisterFlowStatus = registerHandler('flow:status', (payload: { flowId: string; status: string }) => {
            setFlows(prev => prev.map(f => 
                f.id === payload.flowId ? { ...f, status: payload.status as Flow['status'] } : f
            ));
        });
        
        return () => {
            unregisterFlowList();
            unregisterFlowCreated();
            unregisterFlowUpdated();
            unregisterFlowDeleted();
            unregisterFlowStatus();
        };
    }, [registerHandler, activeFlowId]);
    
    // Fetch flows from API
    const refreshFlows = useCallback(async () => {
        setIsLoading(true);
        setError(null);
        
        try {
            const data = await fetchFlows();
            setFlows(data);
            
            // Also request via WebSocket
            sendMessage({ type: 'flow:list' });
        } catch (err) {
            setError(err.message);
            setIsLoading(false);
        }
    }, [sendMessage]);
    
    // Create flow
    const createFlow = useCallback(async (flowData: Partial<Flow>): Promise<Flow> => {
        setIsLoading(true);
        setError(null);
        
        try {
            const newFlow = await createFlowApi(flowData);
            return newFlow;
        } catch (err) {
            setError(err.message);
            setIsLoading(false);
            throw err;
        }
    }, []);
    
    // Update flow
    const updateFlow = useCallback(async (flowId: string, updates: Partial<Flow>): Promise<Flow> => {
        setIsLoading(true);
        setError(null);
        
        try {
            const updatedFlow = await updateFlowApi(flowId, updates);
            setFlows(prev => prev.map(f => f.id === flowId ? updatedFlow : f));
            return updatedFlow;
        } catch (err) {
            setError(err.message);
            setIsLoading(false);
            throw err;
        }
    }, []);
    
    // Delete flow
    const deleteFlow = useCallback(async (flowId: string): Promise<void> => {
        setIsLoading(true);
        setError(null);
        
        try {
            await deleteFlowApi(flowId);
            setFlows(prev => prev.filter(f => f.id !== flowId));
            if (activeFlowId === flowId) {
                setActiveFlowId(null);
            }
        } catch (err) {
            setError(err.message);
            setIsLoading(false);
            throw err;
        }
    }, [activeFlowId]);
    
    // Deploy flow
    const deployFlow = useCallback(async (flowId: string): Promise<void> => {
        setIsLoading(true);
        setError(null);
        
        try {
            await deployFlowApi(flowId);
            // Status will be updated via WebSocket
        } catch (err) {
            setError(err.message);
            setIsLoading(false);
            throw err;
        }
    }, []);
    
    // Undeploy flow
    const undeployFlow = useCallback(async (flowId: string): Promise<void> => {
        setIsLoading(true);
        setError(null);
        
        try {
            await undeployFlowApi(flowId);
            // Status will be updated via WebSocket
        } catch (err) {
            setError(err.message);
            setIsLoading(false);
            throw err;
        }
    }, []);
    
    return {
        flows,
        activeFlowId,
        activeFlow,
        isLoading,
        error,
        setActiveFlowId,
        refreshFlows,
        createFlow,
        updateFlow,
        deleteFlow,
        deployFlow,
        undeployFlow,
    };
}
```

**Testing**:
```typescript
// hooks/useFlows.test.ts
import { renderHook, act, waitFor } from '@testing-library/react';
import { useFlows } from './useFlows';
import * as api from '../utils/api';
import { Flow } from '../types';

vi.mock('../utils/api');
vi.mock('./useWebSocket');

describe('useFlows', () => {
    const mockFlows: Flow[] = [
        { id: 'flow-1', name: 'Flow 1', status: 'draft' } as Flow,
        { id: 'flow-2', name: 'Flow 2', status: 'running' } as Flow,
    ];
    
    beforeEach(() => {
        vi.clearAllMocks();
        vi.mocked(api.fetchFlows).mockResolvedValue(mockFlows);
    });
    
    it('initializes with empty state', () => {
        const { result } = renderHook(() => useFlows());
        
        expect(result.current.flows).toEqual([]);
        expect(result.current.activeFlowId).toBeNull();
        expect(result.current.activeFlow).toBeUndefined();
        expect(result.current.isLoading).toBe(true);
        expect(result.current.error).toBeNull();
    });
    
    it('loads flows on mount', async () => {
        const { result } = renderHook(() => useFlows());
        
        await waitFor(() => {
            expect(result.current.isLoading).toBe(false);
            expect(result.current.flows).toEqual(mockFlows);
        });
    });
    
    it('sets active flow', () => {
        const { result } = renderHook(() => useFlows());
        
        act(() => {
            result.current.setActiveFlowId('flow-1');
        });
        
        expect(result.current.activeFlowId).toBe('flow-1');
    });
    
    it('gets active flow by id', async () => {
        const { result } = renderHook(() => useFlows());
        
        await waitFor(() => {
            expect(result.current.flows).toEqual(mockFlows);
        });
        
        act(() => {
            result.current.setActiveFlowId('flow-1');
        });
        
        expect(result.current.activeFlow).toEqual(mockFlows[0]);
    });
    
    it('creates a new flow', async () => {
        const newFlow: Flow = { id: 'flow-3', name: 'Flow 3', status: 'draft' } as Flow;
        vi.mocked(api.createFlow).mockResolvedValue(newFlow);
        
        const { result } = renderHook(() => useFlows());
        
        await act(async () => {
            const created = await result.current.createFlow({ name: 'Flow 3' });
            expect(created).toEqual(newFlow);
        });
    });
    
    it('updates a flow', async () => {
        const updatedFlow: Flow = { ...mockFlows[0], name: 'Updated Flow 1' } as Flow;
        vi.mocked(api.updateFlow).mockResolvedValue(updatedFlow);
        
        const { result } = renderHook(() => useFlows());
        
        await waitFor(() => {
            expect(result.current.flows).toEqual(mockFlows);
        });
        
        await act(async () => {
            const updated = await result.current.updateFlow('flow-1', { name: 'Updated Flow 1' });
            expect(updated).toEqual(updatedFlow);
            expect(result.current.flows[0].name).toBe('Updated Flow 1');
        });
    });
    
    it('deletes a flow', async () => {
        vi.mocked(api.deleteFlow).mockResolvedValue(undefined);
        
        const { result } = renderHook(() => useFlows());
        
        await waitFor(() => {
            expect(result.current.flows).toEqual(mockFlows);
        });
        
        await act(async () => {
            await result.current.deleteFlow('flow-1');
            expect(result.current.flows).toHaveLength(1);
            expect(result.current.flows).not.toContainEqual(mockFlows[0]);
        });
    });
});
```

---

### useMessageLog.ts

**Purpose**: Manage message log state and display.

**Responsibilities**:
- Fetch and display messages
- Filter messages
- Auto-scroll to new messages
- Clear message log
- Handle real-time message updates via WebSocket

**API**:
```typescript
interface UseMessageLogResult {
    messages: Message[];
    filteredMessages: Message[];
    filter: string;
    setFilter: (filter: string) => void;
    isAutoScroll: boolean;
    setAutoScroll: (autoScroll: boolean) => void;
    fetchMessages: (flowId?: string, limit?: number) => void;
    clearMessages: () => void;
}

function useMessageLog(): UseMessageLogResult
```

**Implementation**:
```typescript
// hooks/useMessageLog.ts
import { useState, useEffect, useCallback, useRef } from 'react';
import { Message } from '../types';
import { useWebSocket } from './useWebSocket';
import { fetchMessages as fetchMessagesApi } from '../utils/api';

interface UseMessageLogResult {
    messages: Message[];
    filteredMessages: Message[];
    filter: string;
    setFilter: (filter: string) => void;
    isAutoScroll: boolean;
    setAutoScroll: (autoScroll: boolean) => void;
    fetchMessages: (flowId?: string, limit?: number) => void;
    clearMessages: () => void;
}

const MAX_MESSAGES = 1000;

export function useMessageLog(): UseMessageLogResult {
    const [messages, setMessages] = useState<Message[]>([]);
    const [filter, setFilter] = useState('');
    const [isAutoScroll, setAutoScroll] = useState(true);
    
    const { sendMessage, registerHandler } = useWebSocket('/ws');
    const messagesEndRef = useRef<HTMLDivElement>(null);
    
    // Filter messages
    const filteredMessages = useCallback(() => {
        if (!filter) return messages;
        
        const searchText = filter.toLowerCase();
        return messages.filter(msg => 
            msg.flowId.toLowerCase().includes(searchText) ||
            msg.id.toLowerCase().includes(searchText) ||
            JSON.stringify(msg.payload).toLowerCase().includes(searchText) ||
            msg.path.some(p => p.toLowerCase().includes(searchText))
        );
    }, [messages, filter])();
    
    // Register WebSocket handler for new messages
    useEffect(() => {
        const unregister = registerHandler('message:new', (payload: { message: Message }) => {
            setMessages(prev => [payload.message, ...prev].slice(0, MAX_MESSAGES));
        });
        
        return unregister;
    }, [registerHandler]);
    
    // Auto-scroll to bottom when messages change
    useEffect(() => {
        if (isAutoScroll && messagesEndRef.current) {
            messagesEndRef.current.scrollIntoView({ behavior: 'smooth' });
        }
    }, [messages, isAutoScroll]);
    
    // Fetch messages from API
    const fetchMessages = useCallback((flowId?: string, limit?: number) => {
        // This would typically be called via WebSocket
        // But provide API fallback
        fetchMessagesApi({ flowId, limit }).then(setMessages).catch(console.error);
    }, []);
    
    // Clear messages
    const clearMessages = useCallback(() => {
        setMessages([]);
    }, []);
    
    // Format timestamp for display
    const formatTimestamp = useCallback((timestamp: string) => {
        return new Date(timestamp).toLocaleTimeString();
    }, []);
    
    return {
        messages,
        filteredMessages,
        filter,
        setFilter,
        isAutoScroll,
        setAutoScroll,
        fetchMessages,
        clearMessages,
        // Additional helpers for components
        formatTimestamp,
        messagesEndRef,
    };
}
```

---

## Hook Testing Guidelines

### Test Structure

Each hook should have a corresponding test file with:
1. **Basic functionality tests**: Test the primary use cases
2. **Edge case tests**: Test error conditions and edge cases
3. **Integration tests**: Test how the hook integrates with other parts of the app

### Testing Utilities

```typescript
// hooks/testUtils.ts
import { renderHook } from '@testing-library/react';

// Helper to wait for a specific number of updates
export async function waitForUpdates(hookResult: any, count: number) {
    for (let i = 0; i < count; i++) {
        await hookResult.waitForNextUpdate();
    }
}

// Helper to test WebSocket hooks
export function createMockWebSocket() {
    class MockWebSocket {
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
        
        simulateMessage(message: string) {
            if (this.onmessage) {
                this.onmessage({ data: message } as MessageEvent);
            }
        }
        
        simulateError(message: string) {
            if (this.onerror) {
                this.onerror(new Error(message));
            }
        }
    }
    
    global.WebSocket = MockWebSocket as any;
    return MockWebSocket;
}
```

---

## Hook Documentation Guidelines

### JSDoc Comments

Every hook should have comprehensive JSDoc documentation:

```typescript
/**
 * Manages WebSocket connection and messaging.
 * 
 * This hook establishes and maintains a WebSocket connection to the specified URL.
 * It handles automatic reconnection with exponential backoff and provides methods
 * for sending and receiving messages.
 * 
 * @param url - The WebSocket URL to connect to
 * @returns Object with WebSocket state and methods
 * 
 * @example
 * ```typescript
 * function MyComponent() {
 *   const { isConnected, sendMessage, registerHandler } = useWebSocket('/ws');
 *   
 *   useEffect(() => {
 *     const unregister = registerHandler('message', (payload) => {
 *       console.log('Received:', payload);
 *     });
 *     
 *     return unregister;
 *   }, [registerHandler]);
 *   
 *   const send = () => sendMessage({ type: 'ping' });
 *   
 *   return <button onClick={send}>Send</button>;
 * }
 * ```
 */
function useWebSocket(url: string): UseWebSocketResult {
    // Implementation
}
```

---

## Performance Optimization for Hooks

### 1. Memoize Dependencies

Use `useCallback` and `useMemo` to prevent unnecessary re-renders:

```typescript
// Good - Memoized callbacks
const handleClick = useCallback((id: string) => {
    setState(prev => ({ ...prev, selectedId: id }));
}, []); // No dependencies = stable reference

// Good - Memoized derived state
const filteredItems = useMemo(() => {
    return items.filter(item => item.isActive);
}, [items]); // Only recompute when items change
```

### 2. Optimize Effect Dependencies

Keep effect dependency arrays minimal:

```typescript
// Bad - Too many dependencies
useEffect(() => {
    // Effect that only needs 'id'
}, [id, other1, other2, other3]);

// Good - Only include what's needed
useEffect(() => {
    // Effect that only needs 'id'
}, [id]);

// Good - Extract stable values
const userId = useCallback(() => user.id, [user.id]);
useEffect(() => {
    // Effect that needs user.id
}, [userId]);
```

### 3. Batch State Updates

Batch multiple state updates to prevent multiple re-renders:

```typescript
// Bad - Multiple state updates
const [a, setA] = useState();
const [b, setB] = useState();

useEffect(() => {
    setA(newA);
    setB(newB);
}, []);

// Good - Batch updates
useEffect(() => {
    setState(prev => ({ ...prev, a: newA, b: newB }));
}, []);
```

### 4. Use Ref for Mutable State

Use `useRef` for values that don't trigger re-renders:

```typescript
// Good - Use ref for mutable state that doesn't affect rendering
const reconnectCountRef = useRef(0);

useEffect(() => {
    reconnectCountRef.current++;
    // This won't trigger a re-render
}, [/* dependencies */]);
```

---

## Checklist for Hook Development

Before finalizing a custom hook:

- [ ] Hook starts with `use` prefix
- [ ] Hook has a clear, single responsibility
- [ ] Hook is placed in the `hooks/` directory
- [ ] Hook has proper TypeScript types for inputs and outputs
- [ ] Hook has JSDoc documentation
- [ ] Hook handles errors gracefully
- [ ] Hook cleans up all resources (subscriptions, timers, etc.)
- [ ] Hook is optimized for performance (memoization, minimal dependencies)
- [ ] Hook has unit tests
- [ ] Hook works correctly with other hooks
- [ ] Hook follows existing patterns in the codebase
- [ ] Hook is reusable across multiple components

---

*Last updated: 2026-06-21*
*Overrides: None (extends web/src/AGENTS.md, web/AGENTS.md, and root AGENTS.md)*
