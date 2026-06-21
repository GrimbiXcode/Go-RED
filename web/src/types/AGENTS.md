# Go-RED Type Definitions Guidelines

This file contains **type-specific** guidelines for TypeScript type definitions in `web/src/types/`.

---

## Overview

The `types/` directory contains all **TypeScript type definitions** for the Go-RED Web UI. These types define the data structures used throughout the frontend and ensure type safety when communicating with the backend.

```
types/
├── index.ts            # Re-exports all types
├── api.ts              # API response/error types
├── flow.ts             # Flow-related types
├── node.ts             # Node-related types
└── message.ts          # Message-related types
```

---

## Type Organization Principles

### 1. Single Source of Truth
Each type should be defined **once** in the most appropriate file. Avoid duplicating types.

### 2. Group by Domain
Types should be grouped by their domain:
- **Flow types**: Everything related to flows (`flow.ts`)
- **Node types**: Everything related to nodes (`node.ts`)
- **Message types**: Everything related to messages (`message.ts`)
- **API types**: Response and request types for API calls (`api.ts`)

### 3. Type Hierarchy
Types should be organized from most general to most specific:
```typescript
// General → Specific
// 1. Base types (interfaces)
// 2. Union types
// 3. Extended types
// 4. Utility types
```

### 4. Naming Conventions
| Type Kind | Convention | Example |
|-----------|------------|---------|
| Interface | PascalCase | `Flow`, `Node`, `Message` |
| Type Alias | PascalCase | `FlowStatus`, `NodeType` |
| Enum | PascalCase | `FlowStatusEnum` (if needed) |
| Union Type | PascalCase | `FlowStatus` |
| Generic | PascalCase | `ApiResponse<T>` |

---

## Type Definition Guidelines

### Interface vs Type Alias

**Use `interface` for:**
- Object shapes
- Class implementations
- When you need to extend or implement

**Use `type` for:**
- Union types
- Tuple types
- Mapped types
- Conditional types
- When you need computed properties

```typescript
// Good - Interface for object shape
interface Flow {
    id: string;
    name: string;
    nodes: Record<string, Node>;
}

// Good - Type alias for union
type FlowStatus = 'draft' | 'running' | 'error' | 'deploying' | 'undeploying';

// Good - Type alias for complex type
type ApiResponse<T> = {
    data?: T;
    error?: string;
    status: number;
};
```

### Type Composition

**Extend interfaces for shared properties:**
```typescript
interface BaseNode {
    id: string;
    type: string;
    x: number;
    y: number;
}

interface Node extends BaseNode {
    name?: string;
    config: Record<string, any>;
    disabled: boolean;
    status?: NodeStatus;
}
```

**Use intersection types for combining:**
```typescript
type NodeWithPosition = Node & {
    x: number;
    y: number;
};
```

**Use union types for alternatives:**
```typescript
type NodeInput = string | number | boolean | Record<string, any> | any[];
```

---

## Domain-Specific Types

### Flow Types (`flow.ts`)

**Core Flow Type:**
```typescript
// types/flow.ts
import { Node, NodeConnection } from './node';

/**
 * Represents a flow in Go-RED.
 * A flow is a collection of nodes connected together to process data.
 */
export interface Flow {
    /** Unique identifier for the flow */
    id: string;
    
    /** Human-readable name for the flow */
    name: string;
    
    /** Optional description of the flow */
    description?: string;
    
    /** Map of node ID to Node */
    nodes: Record<string, Node>;
    
    /** Array of connections between nodes */
    connections: NodeConnection[];
    
    /** Flow configuration */
    config: FlowConfig;
    
    /** Current status of the flow */
    status: FlowStatus;
    
    /** When the flow was created */
    createdAt: string; // ISO 8601 date string
    
    /** When the flow was last updated */
    updatedAt: string; // ISO 8601 date string
    
    /** Version identifier */
    version: string;
}

/**
 * Status of a flow.
 * Determines the current state of the flow execution.
 */
export type FlowStatus = 
    | 'draft'       // Flow is saved but not deployed
    | 'running'     // Flow is active and processing messages
    | 'error'       // Flow encountered an error
    | 'deploying'   // Flow is being deployed
    | 'undeploying'; // Flow is being undeployed

/**
 * Configuration for a flow.
 * Controls how the flow behaves during execution.
 */
export interface FlowConfig {
    /** Timeout for node execution in seconds */
    timeout: number;
    
    /** Maximum number of messages to process concurrently */
    maxConcurrency: number;
    
    /** Environment variables available to nodes */
    environment: Record<string, string>;
    
    /** Retry policy for failed message processing */
    retryPolicy: RetryPolicy;
}

/**
 * Retry policy configuration.
 * Determines how failed messages are retried.
 */
export interface RetryPolicy {
    /** Maximum number of retry attempts */
    maxRetries: number;
    
    /** Initial backoff time in seconds */
    backoff: number;
    
    /** Maximum backoff time in seconds */
    maxBackoff: number;
    
    /** Array of error types to retry on */
    retryOn: string[];
}

/**
 * Minimal flow data for creation.
 * Only required fields for creating a new flow.
 */
export interface FlowCreateData {
    name: string;
    description?: string;
}

/**
 * Flow summary for listing.
 * Contains only essential information for display in lists.
 */
export interface FlowSummary {
    id: string;
    name: string;
    description?: string;
    status: FlowStatus;
    nodeCount: number;
    createdAt: string;
    updatedAt: string;
}
```

### Node Types (`node.ts`)

**Core Node Type:**
```typescript
// types/node.ts
/**
 * Represents a node in a flow.
 * Nodes are the building blocks of flows that process data.
 */
export interface Node {
    /** Unique identifier for the node */
    id: string;
    
    /** Type of the node (e.g., 'inject', 'debug', 'function') */
    type: string;
    
    /** Optional human-readable name for the node */
    name?: string;
    
    /** X coordinate position on the canvas */
    x: number;
    
    /** Y coordinate position on the canvas */
    y: number;
    
    /** Node configuration */
    config: Record<string, any>;
    
    /** Whether the node is disabled */
    disabled: boolean;
    
    /** Runtime status of the node */
    status?: NodeStatus;
}

/**
 * Status of a node during execution.
 * Tracks the current state and processing statistics.
 */
export interface NodeStatus {
    /** Current state of the node */
    state: 'idle' | 'processing' | 'error';
    
    /** Status message */
    message: string;
    
    /** Timestamp of last status change */
    timestamp: string;
    
    /** Number of messages processed */
    processingCount: number;
    
    /** Number of errors encountered */
    errorCount: number;
}

/**
 * Represents a connection between two nodes.
 * Defines how data flows from one node to another.
 */
export interface NodeConnection {
    /** Unique identifier for the connection */
    id: string;
    
    /** Source node ID */
    sourceNode: string;
    
    /** Source port (optional) */
    sourcePort?: string;
    
    /** Target node ID */
    targetNode: string;
    
    /** Target port (optional) */
    targetPort?: string;
}

/**
 * Metadata for a node type.
 * Describes the properties and behavior of a node type.
 */
export interface NodeType {
    /** Unique identifier for the node type */
    id: string;
    
    /** Human-readable name for the node type */
    name: string;
    
    /** Description of what the node does */
    description: string;
    
    /** Category the node belongs to */
    category: string;
    
    /** Icon name for display */
    icon: string;
    
    /** Background color for the node */
    color: string;
    
    /** Names of input ports */
    inputPorts: string[];
    
    /** Names of output ports */
    outputPorts: string[];
    
    /** Configuration schema for the node */
    configSchema: Record<string, ConfigProperty>;
    
    /** Whether the node type is hidden from the UI */
    hidden?: boolean;
    
    /** Whether the node type is deprecated */
    deprecated?: boolean;
    
    /** Node type version */
    version?: string;
}

/**
 * Configuration property definition.
 * Describes a single configuration option for a node.
 */
export interface ConfigProperty {
    /** Type of the configuration value */
    type: 'string' | 'number' | 'boolean' | 'array' | 'object';
    
    /** Default value for the configuration */
    default: any;
    
    /** Whether the configuration is required */
    required: boolean;
    
    /** Description of the configuration */
    description: string;
    
    /** Placeholder text for input */
    placeholder?: string;
    
    /** Available options for select dropdowns */
    options?: string[];
    
    /** Minimum value for numbers */
    min?: number;
    
    /** Maximum value for numbers */
    max?: number;
    
    /** Regex pattern for string validation */
    pattern?: string;
    
    /** Type of editor to use for input */
    editor?: 'text' | 'textarea' | 'number' | 'checkbox' | 'select' | 'code' | 'password' | 'json';
    
    /** Additional configuration for the editor */
    editorConfig?: Record<string, any>;
}

/**
 * Minimal node data for creation.
 * Only required fields for creating a new node.
 */
export interface NodeCreateData {
    type: string;
    x?: number;
    y?: number;
    config?: Record<string, any>;
}
```

### Message Types (`message.ts`)

**Core Message Type:**
```typescript
// types/message.ts
/**
 * Represents a message in the flow.
 * Messages are the data units that flow between nodes.
 */
export interface Message {
    /** Unique identifier for the message */
    id: string;
    
    /** ID of the flow the message belongs to */
    flowId: string;
    
    /** The data payload of the message */
    payload: Record<string, any>;
    
    /** Array of node IDs the message has traversed */
    path: string[];
    
    /** Timestamp when the message was created */
    timestamp: string;
    
    /** Optional metadata for the message */
    metadata?: Record<string, any>;
}

/**
 * Message with context.
 * Includes additional context information for processing.
 */
export interface MessageWithContext extends Message {
    /** Context for the message (e.g., deadline, cancellation) */
    context?: Record<string, any>;
}

/**
 * Message log entry.
 * Additional information for displaying messages in the log.
 */
export interface MessageLogEntry extends Message {
    /** Node that generated the message */
    sourceNode?: string;
    
    /** Whether the message was successfully processed */
    success: boolean;
    
    /** Error message if processing failed */
    error?: string;
}

/**
 * Filter options for fetching messages.
 */
export interface MessageFilter {
    /** Filter by flow ID */
    flowId?: string;
    
    /** Filter by node ID */
    nodeId?: string;
    
    /** Filter by timestamp range */
    startTime?: string;
    endTime?: string;
    
    /** Maximum number of messages to return */
    limit?: number;
}
```

### API Types (`api.ts`)

**API Response Types:**
```typescript
// types/api.ts
import { Flow, FlowSummary, NodeType, Message } from './index';

/**
 * Standard API response format.
 * Used for both success and error responses.
 */
export interface ApiResponse<T = unknown> {
    /** The data payload on success */
    data?: T;
    
    /** Error message on failure */
    error?: string;
    
    /** Human-readable message */
    message?: string;
    
    /** HTTP status code */
    status: number;
}

/**
 * Response for listing flows.
 */
export interface FlowListResponse extends ApiResponse {
    data: FlowSummary[];
}

/**
 * Response for getting a single flow.
 */
export interface FlowResponse extends ApiResponse {
    data: Flow;
}

/**
 * Response for flow creation.
 */
export interface FlowCreateResponse extends ApiResponse {
    data: Flow;
}

/**
 * Response for listing node types.
 */
export interface NodeTypeListResponse extends ApiResponse {
    data: NodeType[];
}

/**
 * Response for getting a single node type.
 */
export interface NodeTypeResponse extends ApiResponse {
    data: NodeType;
}

/**
 * Response for listing messages.
 */
export interface MessageListResponse extends ApiResponse {
    data: Message[];
}

/**
 * Request body for creating a flow.
 */
export interface CreateFlowRequest {
    name: string;
    description?: string;
    nodes?: Record<string, import('./node').NodeCreateData>;
    connections?: import('./node').NodeConnection[];
    config?: import('./flow').FlowConfig;
}

/**
 * Request body for updating a flow.
 */
export interface UpdateFlowRequest {
    name?: string;
    description?: string;
    nodes?: Record<string, import('./node').NodeCreateData>;
    connections?: import('./node').NodeConnection[];
    config?: Partial<import('./flow').FlowConfig>;
}
```

---

## Type Guards

Type guards provide runtime type checking for TypeScript types:

```typescript
// types/guards.ts
import { Flow, FlowStatus, Node, Message } from './index';

/**
 * Type guard for FlowStatus.
 * Checks if a string is a valid flow status.
 */
export function isFlowStatus(value: string): value is FlowStatus {
    return ['draft', 'running', 'error', 'deploying', 'undeploying'].includes(value);
}

/**
 * Type guard for Flow.
 * Checks if an object has the required flow properties.
 */
export function isFlow(object: unknown): object is Flow {
    if (!object || typeof object !== 'object') {
        return false;
    }
    
    const flow = object as Record<string, unknown>;
    
    return (
        typeof flow.id === 'string' &&
        typeof flow.name === 'string' &&
        typeof flow.nodes === 'object' &&
        Array.isArray(flow.connections) &&
        typeof flow.config === 'object' &&
        typeof flow.status === 'string' &&
        typeof flow.createdAt === 'string' &&
        typeof flow.updatedAt === 'string' &&
        typeof flow.version === 'string'
    );
}

/**
 * Type guard for Node.
 * Checks if an object has the required node properties.
 */
export function isNode(object: unknown): object is Node {
    if (!object || typeof object !== 'object') {
        return false;
    }
    
    const node = object as Record<string, unknown>;
    
    return (
        typeof node.id === 'string' &&
        typeof node.type === 'string' &&
        typeof node.x === 'number' &&
        typeof node.y === 'number' &&
        typeof node.config === 'object' &&
        typeof node.disabled === 'boolean'
    );
}

/**
 * Type guard for Message.
 * Checks if an object has the required message properties.
 */
export function isMessage(object: unknown): object is Message {
    if (!object || typeof object !== 'object') {
        return false;
    }
    
    const message = object as Record<string, unknown>;
    
    return (
        typeof message.id === 'string' &&
        typeof message.flowId === 'string' &&
        typeof message.payload === 'object' &&
        Array.isArray(message.path) &&
        typeof message.timestamp === 'string'
    );
}

/**
 * Validates a flow object.
 * Returns an array of validation errors, or empty array if valid.
 */
export function validateFlow(flow: unknown): string[] {
    const errors: string[] = [];
    
    if (!isFlow(flow)) {
        errors.push('Invalid flow object');
        return errors;
    }
    
    // Check required fields
    if (!flow.id.trim()) {
        errors.push('Flow ID is required');
    }
    
    if (!flow.name.trim()) {
        errors.push('Flow name is required');
    }
    
    // Check nodes
    if (typeof flow.nodes !== 'object' || flow.nodes === null) {
        errors.push('Flow nodes must be an object');
    } else {
        Object.entries(flow.nodes).forEach(([id, node]) => {
            if (!isNode(node)) {
                errors.push(`Invalid node: ${id}`);
            }
            if (node.id !== id) {
                errors.push(`Node ID mismatch: ${id}`);
            }
        });
    }
    
    // Check connections
    if (!Array.isArray(flow.connections)) {
        errors.push('Flow connections must be an array');
    } else {
        flow.connections.forEach((conn, index) => {
            if (typeof conn.id !== 'string') {
                errors.push(`Connection ${index} has invalid ID`);
            }
            if (typeof conn.sourceNode !== 'string') {
                errors.push(`Connection ${index} has invalid source node`);
            }
            if (typeof conn.targetNode !== 'string') {
                errors.push(`Connection ${index} has invalid target node`);
            }
        });
    }
    
    // Check status
    if (!isFlowStatus(flow.status)) {
        errors.push(`Invalid flow status: ${flow.status}`);
    }
    
    return errors;
}
```

---

## Type Utilities

Utility types for common patterns:

```typescript
// types/utilities.ts
/**
 * Makes all properties optional.
 */
export type Partial<T> = {
    [P in keyof T]?: T[P];
};

/**
 * Makes all properties required.
 */
export type Required<T> = {
    [P in keyof T]-?: T[P];
};

/**
 * Makes specific properties optional.
 */
export type PartialBy<T, K extends keyof T> = Omit<T, K> & Partial<Pick<T, K>>;

/**
 * Makes specific properties required.
 */
export type RequiredBy<T, K extends keyof T> = Omit<T, K> & Required<Pick<T, K>>;

/**
 * Creates a type with all properties as readonly.
 */
export type Readonly<T> = {
    readonly [P in keyof T]: T[P];
};

/**
 * Creates a deep readonly type.
 */
export type DeepReadonly<T> = {
    readonly [P in keyof T]: DeepReadonly<T[P]>;
};

/**
 * Creates a type with all string properties as optional.
 */
export type StringKeysOptional<T> = {
    [P in keyof T as P extends string ? P : never]?: T[P];
};

/**
 * Creates a type with all properties as nullable.
 */
export type Nullable<T> = {
    [P in keyof T]: T[P] | null;
};

/**
 * Creates a type with all properties as undefined-able.
 */
export type Undefinable<T> = {
    [P in keyof T]: T[P] | undefined;
};

/**
 * Creates a type that is either T or null or undefined.
 */
export type Maybe<T> = T | null | undefined;

/**
 * Creates a tuple type from an array.
 */
export type Tuple<T extends any[], N extends number> = N extends N 
    ? number extends N 
        ? T 
        : _TupleOf<T, N, []>
    : never;

type _TupleOf<T extends any[], N extends number, R extends any[]> = 
    R['length'] extends N 
        ? R 
        : _TupleOf<T, N, [T[number], ...R]>;

/**
 * Creates a type that is the keys of T that are of type U.
 */
export type KeysOfType<T, U> = {
    [P in keyof T]: T[P] extends U ? P : never;
}[keyof T];

/**
 * Creates a type that is the keys of T that are functions.
 */
export type FunctionKeys<T> = KeysOfType<T, Function>;

/**
 * Creates a type that is the return type of all functions in T.
 */
export type FunctionReturnTypes<T> = {
    [K in FunctionKeys<T>]: ReturnType<T[K]>
};
```

---

## Type Testing

Test type definitions to ensure they work as expected:

```typescript
// types/types.test.ts
import { describe, it, expect } from 'vitest';
import { 
    Flow, 
    FlowStatus, 
    Node, 
    NodeType,
    Message,
    isFlowStatus,
    isFlow,
    validateFlow,
} from './index';

describe('Type Definitions', () => {
    describe('FlowStatus', () => {
        it('has correct values', () => {
            const validStatuses: FlowStatus[] = ['draft', 'running', 'error', 'deploying', 'undeploying'];
            validStatuses.forEach(status => {
                expect(status).toBeTypeOf('string');
            });
        });
        
        it('isFlowStatus correctly validates', () => {
            expect(isFlowStatus('draft')).toBe(true);
            expect(isFlowStatus('running')).toBe(true);
            expect(isFlowStatus('invalid')).toBe(false);
            expect(isFlowStatus('')).toBe(false);
        });
    });
    
    describe('Flow', () => {
        it('has all required properties', () => {
            const flow: Flow = {
                id: 'test-id',
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
        
        it('isFlow correctly validates', () => {
            const validFlow: Flow = {
                id: 'test',
                name: 'Test',
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
    });
    
    describe('Message', () => {
        it('has all required properties', () => {
            const message: Message = {
                id: 'msg-1',
                flowId: 'flow-1',
                payload: { data: 'test' },
                path: ['node-1'],
                timestamp: new Date().toISOString(),
            };
            
            expect(message).toHaveProperty('id');
            expect(message).toHaveProperty('flowId');
            expect(message).toHaveProperty('payload');
            expect(message).toHaveProperty('path');
            expect(message).toHaveProperty('timestamp');
        });
    });
    
    describe('validateFlow', () => {
        it('returns empty array for valid flow', () => {
            const validFlow: Flow = {
                id: 'test',
                name: 'Test',
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
        
        it('returns errors for invalid flow', () => {
            const errors = validateFlow({});
            expect(errors).toContain('Invalid flow object');
        });
        
        it('validates required fields', () => {
            const flow = {
                id: '',
                name: 'Test',
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
            
            const errors = validateFlow(flow);
            expect(errors).toContain('Flow ID is required');
        });
    });
});
```

---

## Type Documentation

Every type should have JSDoc documentation explaining:
1. **What it represents**: The purpose of the type
2. **How it's used**: Where and how the type is used
3. **Important fields**: Explanation of key properties
4. **Examples**: Example values when helpful

```typescript
/**
 * Represents a flow in Go-RED.
 * 
 * A flow is the primary entity in Go-RED, consisting of nodes connected
 * together to form a data processing pipeline. Messages enter a flow
 * through input nodes and are processed by subsequent nodes according
 * to the defined connections.
 * 
 * @example
 * ```typescript
 * const myFlow: Flow = {
 *   id: 'flow-1',
 *   name: 'My Flow',
 *   nodes: {
 *     'input': { id: 'input', type: 'inject', x: 0, y: 0, config: {} },
 *     'output': { id: 'output', type: 'debug', x: 100, y: 0, config: {} },
 *   },
 *   connections: [
 *     { id: 'conn-1', sourceNode: 'input', targetNode: 'output' },
 *   ],
 *   config: { timeout: 30, maxConcurrency: 10, environment: {}, retryPolicy: { maxRetries: 3, backoff: 1, maxBackoff: 30, retryOn: [] } },
 *   status: 'draft',
 *   createdAt: new Date().toISOString(),
 *   updatedAt: new Date().toISOString(),
 *   version: '1.0.0',
 * };
 * ```
 */
export interface Flow {
    // ... properties
}
```

---

## Type Versioning

When making **breaking changes** to types:

1. **Create new types** with version suffixes if possible:
```typescript
// Old type (deprecated)
export interface FlowV1 { /* ... */ }

// New type
export interface FlowV2 { /* ... */ }

// Current type (alias to latest)
export type Flow = FlowV2;
```

2. **Provide migration helpers**:
```typescript
// Migration function
export function migrateFlowV1toV2(flow: FlowV1): FlowV2 {
    return {
        ...flow,
        // Add new fields with defaults
        version: '2.0.0',
        // Migrate old fields
        newField: migrateOldField(flow.oldField),
    };
}
```

3. **Document breaking changes** in a `MIGRATIONS.md` file

---

## Checklist for Type Development

Before finalizing type definitions:

- [ ] Type has a clear, single purpose
- [ ] Type is in the correct file (grouped by domain)
- [ ] Type is properly named (PascalCase)
- [ ] Type has JSDoc documentation
- [ ] Type has all required properties
- [ ] Optional properties are marked as such
- [ ] Type uses appropriate TypeScript features (interface, type, enum, etc.)
- [ ] Type is exported from `index.ts`
- [ ] Type is imported where needed (not duplicated)
- [ ] Type has corresponding type guards (if needed)
- [ ] Type has tests (if complex)
- [ ] Type follows existing patterns in the codebase
- [ ] Type is compatible with backend API responses
- [ ] Breaking changes are documented and handled

---

*Last updated: 2026-06-21*
*Overrides: None (extends web/src/AGENTS.md, web/AGENTS.md, and root AGENTS.md)*
