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

The real `FlowNode` type (generated from Go's `internal/dto.Node`, re-exported
in `flow.ts`) nests coordinates under `position: {x, y}` — it does **not**
have flat `x`/`y` fields, and `status`/`disabled` are always present (never
optional), matching what the backend always sends:

```typescript
export type { Node as FlowNode } from './generated';
// FlowNode shape: { id, type, name?, position: {x, y}, config, status, disabled }
```

Do not reintroduce a flat-`x`/`y` `Node`/`BaseNode` shape here — that was a
previous, aspirational version of this file that never matched the real
backend struct (`internal/engine.Node` uses flat `X`/`Y` internally, but the
wire format nests them under `position`, and `internal/dto` is what this
type is generated from).

**Use union types for alternatives:**
```typescript
type NodeInput = string | number | boolean | Record<string, any> | any[];
```

---

## Domain-Specific Types

### Flow Types (`flow.ts`)

**The illustrative snippet below is close to, but not identical to, reality —
`Flow`/`FlowConfig`/`RetryPolicy`/`FlowSummary`/`FlowStatus` are generated
from `internal/dto` into `generated.ts` and re-exported from `flow.ts`; treat
`web/src/types/generated.ts` as the actual ground truth, not this file.**
One concrete difference: `description` below is shown as optional
(`description?`), but the real wire type has it as a required `string`
(the Go backend always sends it, empty string if none) — this file's
example predates that correction.

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
    
    /** Description of the flow (always present, empty string if none) */
    description: string;
    
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

**This section previously described a fictional shape** (flat `x`/`y`
instead of a nested `position` object, a `NodeType`/`ConfigProperty` pair
with `inputPorts`/`outputPorts`/`color`/`placeholder`/`options`/`editor`
fields that never existed in the Go backend). The real types:

- `FlowNode` (`Node` re-exported from `generated.ts`, generated from
  `internal/dto.Node`): `{ id, type, name?, position: {x, y}, config,
  status, disabled }` — `status` and `disabled` are always present (the
  backend always sends them), not optional.
- `NodeMetadata` (re-exported from `generated.ts`, generated from
  `internal/registry.NodeMetadata`): `{ id, type, name, description,
  category, inputs: Port[], outputs: Port[], configSchema: Schema, icon,
  tags }`. No `color`, `hidden`, `deprecated`, or per-type `version` field.
- `Port` (re-exported from `generated.ts`): `{ id, name, description,
  required }` — no `type`/`schema` field (an earlier version of this file
  described those, but the backend `registry.Port` struct never had them).
- `PropertySchema` (a rename of the generated `Property` type, generated
  from `internal/registry.Property`): `{ type, description, default, enum,
  min?, max?, pattern }` — flat, not recursive. It has no `properties`,
  `items`, `oneOf`, `allOf`, `format`, or `examples` — Go has no
  equivalent for a recursive JSON-Schema-like structure. `required` lives
  on the wrapping `Schema` type as a list of property *names*, not as a
  per-property boolean.

`NodeCategory`, `PortType`, `SchemaType`, `NodeProperty`, `NodeTypeDefinition`,
and `NodePaletteItem` remain hand-written in `node.ts` — they are pure
frontend/UI concepts with no Go equivalent (in particular, `NodeMetadata`'s
real `category` field is an unconstrained `string`, not the `NodeCategory`
union — components that index by category must handle arbitrary strings,
see `NodePalette.tsx`/`NodeComponent.tsx` for the pattern).

### Message Types (`message.ts`)

**Core Message Type** (`Message`, re-exported from `generated.ts`, generated
from `internal/dto.Message`):
```typescript
export interface Message {
    id: string;
    flowId: string;
    payload: Record<string, any>;
    metadata: Record<string, string>; // always present, string values only —
                                       // Go cannot produce a richer typed
                                       // metadata object (no qos: number,
                                       // retain: boolean, etc.)
    path: string[];
    timestamp: string;
}
```

The real hand-written `MessageLogEntry` (`message.ts`, UI-only, no Go
equivalent) is `{ id, flowId, nodeId, message: Message, timestamp, level:
'debug'|'info'|'warn'|'error' }` — **not** the `{sourceNode?, success,
error?} extends Message` shape shown in earlier versions of this file,
which was never implemented. `MessageBatch`/`NodeMessage`/`FlowMessage` in
`message.ts` are similarly hand-written UI conveniences layered on top of
the wire `Message`, not generated.

### API Types (`api.ts`)

**There is no response envelope.** An earlier version of this file described
an `ApiResponse<T>` wrapper (`{data?, error?, message?, status}`) with
`FlowListResponse`/`FlowResponse`/`NodeTypeListResponse`/etc. wrapping every
resource in a `data` field — none of that was ever real. `utils/api.ts`'s
`apiRequest<T>()` returns `response.json()` directly, unwrapped; every REST
endpoint responds with the bare resource (a `Flow`, a `FlowSummary[]`, a
`NodeMetadata[]`, ...). Do not reintroduce an envelope type unless the Go
handlers in `cmd/go-red/main.go` actually start wrapping responses that way.

`FlowSummary`, `FlowCreateRequest`, and `FlowUpdateRequest` are generated
from `internal/dto` into `generated.ts` and re-exported from `api.ts` — do
not hand-describe them here (in particular, `FlowCreateRequest` has no
`nodes`/`connections`/`config` fields; the real Go handler only reads
`id`/`name`/`description` from it). `PaginatedResponse`, `Pagination`,
`DeployResponse`/`DeployRequest`, `UndeployRequest`, `HealthCheckResponse`,
`StatsResponse`, `MessageLogRequest`/`MessageLogResponse`, and
`FlowExportRequest`/`FlowImportRequest` remain hand-written in `api.ts`
(no corresponding Go DTO exists yet for most of them).

---

## Type Guards

> **Status: not implemented.** There is no `types/guards.ts` or
> `types/utilities.ts` file anywhere in `web/src/`. The functions below
> (`isFlowStatus`, `isFlow`, `isNode`, `isMessage`, `validateFlow`, and
> whatever utilities the "Type Utilities" section further down describes)
> are a design sketch that was never built — runtime WS message payloads are
> currently consumed as untyped `any` (see `useWebSocket.ts`) with no
> validation layer. Treat everything through "Type Testing" below as a
> proposal, not existing code, until it's actually implemented.

Type guards provide runtime type checking for TypeScript types:

```typescript
// types/guards.ts (proposed, does not exist yet)
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
 *   description: '',
 *   nodes: {
 *     'input': { id: 'input', type: 'inject', position: { x: 0, y: 0 }, config: {}, status: { state: 'idle' }, disabled: false },
 *     'output': { id: 'output', type: 'debug', position: { x: 100, y: 0 }, config: {}, status: { state: 'idle' }, disabled: false },
 *   },
 *   connections: [
 *     { id: 'conn-1', sourceNode: 'input', targetNode: 'output' },
 *   ],
 *   config: { timeout: 30, maxConcurrency: 10, environment: {}, retryPolicy: { maxRetries: 3, backoff: 1, maxBackoff: 30, retryOn: [] } },
 *   status: 'draft',
 *   createdAt: new Date().toISOString(),
 *   updatedAt: new Date().toISOString(),
 *   version: '1.0',
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

---

## Interface-Verifikation (PFLICHT bei Änderungen an Interfaces)

Trigger: Diese Schritte IMMER ausführen, bevor eine Änderung als fertig gilt, wenn eine der
folgenden Dateien/Verzeichnisse angefasst wurde:
- `internal/dto/**`
- `internal/registry/registry.go` (NodeMetadata/Port/Property/Schema)
- `cmd/go-red/websocket/hub.go` (WebSocketMessage/MessageType)
- irgendeine Datei unter `web/src/types/**`

Schritte (in dieser Reihenfolge, nach jeder Interface-Änderung):
1. `go build ./...` und `go vet ./...` — stellt sicher, dass die Go-Seite kompiliert.
2. `go generate ./internal/dto/...` — regeneriert `web/src/types/generated.ts` aus den
   aktuellen Go-DTOs.
3. `git diff --exit-code -- web/src/types/generated.ts` — falls dieser Befehl NICHT sauber
   durchläuft (also ein Diff zeigt), bedeutet das: die generierte Datei war vor der Änderung
   veraltet oder wurde von Hand editiert. Den Diff committen, NIEMALS `generated.ts` von Hand
   anpassen.
4. `cd web && npx tsc --noEmit` — deckt Call-Sites auf, die nach einer Schema-Änderung
   angepasst werden müssen (umbenannte/entfernte Felder etc.). Alle daraus resultierenden
   Fehler im selben Change beheben, nicht auf später verschieben.
5. `cd web && npm test` — stellt sicher, dass `types.test.ts` und alle anderen Tests weiterhin
   gegen die aktuelle Form bestehen.
6. Bei Änderungen, die REST- oder WebSocket-Payloads betreffen: kurzer manueller Smoke-Test
   (`go run cmd/go-red/main.go` + `npm run dev`, Flow erstellen/deployen/Message injizieren)
   um Laufzeitverhalten zu bestätigen, das ein Compiler nicht prüfen kann.

Nicht erlaubt: eine neue Wire-Form (Struct-Feld, Enum-Wert, WS-Message-Typ) einführen, ohne
dass sie durch `internal/dto` (bzw. `internal/registry`/`cmd/go-red/websocket` für deren
jeweilige Scan-Ziele) läuft und in `generated.ts` auftaucht. Handschriftliche TS-Interfaces,
die eine Backend-Form beschreiben, statt sie aus `generated.ts` zu re-exportieren, sind ein
Rückfall in den alten, driftanfälligen Zustand und müssen vermieden werden.
