# Go-RED Node Registry Guidelines

This file contains **registry-specific** guidelines for the node type registration system in `internal/registry/`.

---

## Package Overview

The `registry/` package provides a **central catalog** of all available node types in Go-RED. It enables:

1. **Node Discovery** - List all available node types
2. **Node Registration** - Add new node types (including plugins)
3. **Node Instantiation** - Create node instances from configuration
4. **Metadata Management** - Store and retrieve node type information

---

## Architecture

### Core Components

```
NodeRegistry (Central Catalog)
├── nodes: map[string]*Node          (nodeType -> Node)
├── factories: map[string]NodeFactory (nodeType -> Factory)
└── mu: sync.RWMutex (thread-safe access)

Node (Per-Node-Type, internal registry entry)
├── Type: string
├── Metadata: NodeMetadata (value, not pointer)
└── Factory: NodeFactory (creates instances)

NodeMetadata (Type Information — this is also the wire/API shape; see
internal/dto's note below, and internal/registry/registry.go for the source
of truth)
├── ID, Type, Name, Description, Category (all string)
├── Inputs, Outputs: []Port
├── ConfigSchema: Schema
├── Icon: string
└── Tags: []string

Port (Input/Output Port)
├── ID, Name, Description: string
└── Required: bool

Schema (Config Schema Wrapper)
├── Properties: map[string]Property
└── Required: []string (names of required properties)

Property (Configuration Property Definition)
├── Type, Description, Pattern: string
├── Default: interface{}
├── Enum: []string
├── Min, Max: *float64 (JSON keys are "min"/"max")
```

There is no `NodeTypeEntry`, `ConfigProperty`, `InputPorts`/`OutputPorts`, `Color`,
`Hidden`/`Deprecated`/`Replaces`/`Version` field, `Placeholder`, `Options`,
`Editor`/`EditorConfig` anywhere in the real `NodeMetadata`/`Property` structs —
an earlier version of this file described an aspirational shape that was never
implemented. Always check `internal/registry/registry.go` directly before
relying on a struct shape described here.

### Singleton Pattern

The package uses a **global registry singleton**:

```go
var globalRegistry *NodeRegistry
var once sync.Once

// GetGlobalRegistry returns the singleton registry
func GetGlobalRegistry() *NodeRegistry {
    once.Do(func() {
        globalRegistry = NewNodeRegistry()
    })
    return globalRegistry
}
```

**Usage:**
- All built-in nodes register to the global registry
- Plugins should register to the global registry
- The flow engine uses the global registry
- Tests can create isolated registries

### Optional Node Interfaces (`registry.go`)

Beyond `NodeExecutor`, three optional interfaces let a node opt into extra engine
capabilities (checked via type assertion, so implementing none of them - like
`inject`/`debug`/`function` - is unaffected):

- **`MultiOutputExecutor`** (`NodeExecutor` + `ExecuteMulti`) - route different payloads
  to specific output ports by ID (`map[string]map[string]interface{}`; a missing/`nil`
  port sends nothing on it). Used by `linkout` today; future `switch`/`catch`/`split`.
- **`Closeable`** (`Close() error`) - release resources on `Undeploy` or a failed
  `Deploy`. No current node needs it yet (future `mqtt in/out`, `tcp in`, `watch`, ...).
- **`EmittingNode`** (`NodeExecutor` + `Start(ctx, emit)`) - originate messages instead
  of only reacting to input; the engine runs `Start` in its own goroutine for the
  lifetime of the flow. Used by `catch`/`status`/`complete` today (they subscribe to
  `NodeRuntime` events and `emit` on a match, blocking on `<-ctx.Done()`).

### NodeRuntime, EventBus, ContextStore (`runtime.go`, `eventbus.go`, `context_store.go`)

These three types are the engine-facing services a node reaches via
`registry.RuntimeFromContext(ctx)` inside `Execute`/`ExecuteMulti`/`Start` (the engine
embeds a `*NodeRuntime` into the `context.Context` it passes in). They live in
`registry`, not `internal/engine`, specifically so node packages only ever import
`internal/registry` like every other node - never `internal/engine` - keeping the same
dependency direction the plugin system design already assumes.

- **`NodeRuntime`**: per-invocation handle with `FlowContext`/`GlobalContext`
  (`*ContextStore`), `ReportStatus`/`ReportError` (publish), `OnError`/`OnStatus`/
  `OnComplete` (subscribe - normally called once, inside `EmittingNode.Start`),
  `SubmitToNode(nodeID, payload)` (deliver directly to another node's output in the same
  flow, bypassing a drawn wire - used by `linkout` to reach a `linkin` node; **no
  cross-flow support yet**), and `GetNode(nodeID) (NodeExecutor, bool)` (Phase 6 - look
  up another node's *live executor instance* within the same flow, used to reach a
  shared config node like `mqtt-broker` by the ID stored in a consumer's own string
  config property; resolved lazily at `Execute`/`ExecuteMulti`/`Start` call time, not at
  `SetConfig` time, since Deploy's node-init loop iterates a Go map in unspecified order
  - see `internal/nodes/AGENTS.md`'s config-node section for the full ordering
  discussion, including why a config node must become "ready" inside its own `SetConfig`
  rather than an `EmittingNode.Start`).
- **`EventBus`**: per-flow pub/sub for `NodeErrorEvent`/`NodeStatusEvent`/
  `NodeCompleteEvent`. The engine auto-publishes an error event on every failed
  `Execute`/`ExecuteMulti` and a complete event on every successful one; status events
  are always node-initiated via `ReportStatus`.
- **`ContextStore`**: thread-safe key-value store (`flow.get`/`set`,
  `global.get`/`set`); one per flow plus one global one on the engine.

**Feedback-loop hazard:** a `complete` (or any future `NodeCompleteEvent` consumer)
whose own output feeds back into a node inside its `scope` will re-trigger itself
forever, because "node finished successfully" is the *same* event every node emits -
unlike error/status, which normal successful nodes never produce. Always scope such a
node to the specific node(s) being watched, not to "everything" (see
`internal/nodes/complete/node.go`'s doc comment and
`internal/engine/phase1_flow_control_test.go` for a reproduction that was caught while
writing that test).

---

## Development Guidelines

### Adding New Node Types

Nodes self-register in their `init()` function via `RegisterFactory` (see
`internal/nodes/debug/node.go` for a real example):

```go
func init() {
    reg := registry.GetGlobalRegistry()
    err := reg.RegisterFactory("unique-node-id", func() registry.NodeExecutor {
        return &MyNode{}
    }, registry.NodeMetadata{
        ID:       "unique-node-id",
        Type:     "unique-node-id",
        Name:     "My Node",
        Category: "function",
        Inputs:   []registry.Port{{ID: "in", Name: "in"}},
        Outputs:  []registry.Port{{ID: "out", Name: "out"}},
        ConfigSchema: registry.Schema{
            Properties: map[string]registry.Property{
                "example": {Type: "string", Description: "An example property"},
            },
        },
    })
    if err != nil {
        panic(err)
    }
}
```

### Registry Initialization

**In `main.go` or tests:**

```go
// Get the global registry (auto-initializes)
reg := registry.GetGlobalRegistry()

// Or create a new isolated registry
reg := registry.NewNodeRegistry()
```

### Node Registration

```go
// RegisterFactory registers a node factory with metadata (the usual entry
// point from a node package's init()). It builds a Node{Type, Metadata,
// Factory} and delegates to RegisterNode.
func (r *NodeRegistry) RegisterFactory(nodeType string, factory NodeFactory, metadata NodeMetadata) error

// RegisterNode registers a fully-constructed Node directly. Validates that
// Type is non-empty and rejects duplicate types.
func (r *NodeRegistry) RegisterNode(node *Node) error
```

**Best Practices:**
- Use **kebab-case** for node IDs (e.g., "http-request", "debug-output")
- Use **Title Case** for display names (e.g., "HTTP Request", "Debug Output")
- Register in `init()` to ensure registration happens before use
- Handle registration errors (though they're rare in practice)

---

## API Reference

### Public Methods

#### GetGlobalRegistry
```go
func GetGlobalRegistry() *NodeRegistry
```
Returns the singleton global registry.

#### NewNodeRegistry
```go
func NewNodeRegistry() *NodeRegistry
```
Creates a new isolated registry (useful for testing).

#### RegisterFactory / RegisterNode
```go
func (r *NodeRegistry) RegisterFactory(nodeType string, factory NodeFactory, metadata NodeMetadata) error
func (r *NodeRegistry) RegisterNode(node *Node) error
```
Registers a new node type. Returns error if the type is empty or already registered.

#### GetExecutor
```go
func (r *NodeRegistry) GetExecutor(nodeType string) (NodeExecutor, error)
```
Creates a new executor instance for a node type via its factory.

#### GetMetadata
```go
func (r *NodeRegistry) GetMetadata(nodeType string) (NodeMetadata, error)
```
Gets metadata for a node type. Used by the UI to display node information.

#### GetAllNodes
```go
func (r *NodeRegistry) GetAllNodes() []NodeMetadata
```
Returns metadata for all registered node types (value slice, not pointers).
Used to populate the node palette in the UI (`GET /api/nodes`).

#### GetNodesByCategory
```go
func (r *NodeRegistry) GetNodesByCategory(category string) []NodeMetadata
```
Returns node types filtered by category.

#### IsRegistered
```go
func (r *NodeRegistry) IsRegistered(nodeType string) bool
```
Checks if a node type is registered.

#### Unregister
```go
func (r *NodeRegistry) Unregister(nodeType string) error
```
Removes a node type from the registry. Primarily for testing.

#### InitializeNode
```go
func (r *NodeRegistry) InitializeNode(
    nodeType string,
    config map[string]interface{},
) (NodeExecutor, error)
```
Convenience method to create a node instance by type ID and configuration.

---

## Node Factory Pattern

A **NodeFactory** is a function that creates node instances:

```go
type NodeFactory func(config map[string]interface{}) (NodeExecutor, error)
```

**Example Factory:**

```go
func newDebugNodeFactory() (NodeExecutor, error) {
    return func(config map[string]interface{}) (NodeExecutor, error) {
        // Validate configuration
        if config == nil {
            config = map[string]interface{}{}
        }
        
        // Create and initialize node
        node := &DebugNode{
            // Extract configuration
        }
        
        // Validate node configuration
        if err := node.Validate(); err != nil {
            return nil, err
        }
        
        return node, nil
    }
}
```

**Factory Responsibilities:**
1. Validate input configuration
2. Create node instance
3. Initialize node with configuration
4. Validate node is properly configured
5. Return node or error

---

## Metadata System

### NodeMetadata Structure

This is the real, current shape (`internal/registry/registry.go`). It is
part of the canonical wire contract: `cmd/gentypes` generates the matching
TypeScript `NodeMetadata` interface into `web/src/types/generated.ts` — do
not hand-describe it elsewhere.

```go
type NodeMetadata struct {
    ID           string   `json:"id"`
    Type         string   `json:"type"`
    Name         string   `json:"name"`
    Description  string   `json:"description"`
    Category     string   `json:"category"`
    Inputs       []Port   `json:"inputs"`
    Outputs      []Port   `json:"outputs"`
    ConfigSchema Schema   `json:"configSchema"`
    Icon         string   `json:"icon"`
    Tags         []string `json:"tags"`
}

type Port struct {
    ID          string `json:"id"`
    Name        string `json:"name"`
    Description string `json:"description"`
    Required    bool   `json:"required"`
}

type Schema struct {
    Properties map[string]Property `json:"properties"`
    Required   []string            `json:"required"` // names of required properties
}

type Property struct {
    Type        string      `json:"type"`
    Description string      `json:"description"`
    Default     interface{} `json:"default"`
    Enum        []string    `json:"enum"`
    Min         *float64    `json:"min"`
    Max         *float64    `json:"max"`
    Pattern     string      `json:"pattern"`
}
```

There is no `Color`, `Hidden`, `Deprecated`, `Replaces`, per-property `Version`,
`Placeholder`, `Options`, `Editor`, or `EditorConfig` field — those never
existed in the real struct. Note also that `Required` lives at the `Schema`
level (a list of required property *names*), not as a per-`Property` boolean.

### Standard Categories

Use these standard categories for consistency:

| Category | Description | Example |
|----------|-------------|---------|
| `input` | Message sources | inject, websocket-in, http-in |
| `output` | Message sinks | debug, http-out, file |
| `function` | Data transformation | function, template, json |
| `logic` | Control flow | switch, condition, router |
| `network` | Network operations | http-request, websocket |
| `storage` | Persistence | redis, mongodb, postgres |
| `utility` | Helper nodes | delay, rate-limit, batch |
| `sensor` | Data acquisition | serial, gpio |
| `dashboard` | UI widgets | gauge, chart |

---

## Plugin System Integration

> **Status: not implemented.** No `.so`-plugin loader, `plugin.Open` call, or
> `Register(*NodeRegistry) error` convention exists anywhere in the current
> codebase (`config.PluginDir` in `cmd/go-red/main.go` is parsed as a flag but
> never used to load anything). The examples below describe a planned design
> (see `docs/IMPLEMENTATION_PLAN.md` Phase 2), not current, working code —
> and they also use registry method names (`RegisterNodeType`, `HasNodeType`)
> that don't exist even internally (the real methods are `RegisterFactory`/
> `RegisterNode` and `IsRegistered`, see the API Reference above). Treat this
> whole section as a design sketch, not a contract.

### Loading Plugins

Plugins are Go shared libraries (`.so` files) that register node types:

```go
// In main.go or plugin loader
func loadPlugin(path string) error {
    plug, err := plugin.Open(path)
    if err != nil {
        return err
    }
    
    // Look for Register function
    register, err := plug.Lookup("Register")
    if err != nil {
        return err
    }
    
    // Call register with the global registry
    if registerFunc, ok := register.(func(*NodeRegistry) error); ok {
        return registerFunc(registry.GetGlobalRegistry())
    }
    
    return fmt.Errorf("plugin does not have valid Register function")
}
```

### Plugin Structure

A plugin should expose a `Register` function:

```go
// In plugin code
package main

import (
    "github.com/GrimbiXcode/Go-RED/internal/registry"
)

// MyCustomNode implements NodeExecutor
type MyCustomNode struct {}

func (n *MyCustomNode) Execute(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error) {
    // Implementation
}

// Register registers plugin nodes
func Register(reg *registry.NodeRegistry) error {
    return reg.RegisterNodeType(
        "my-custom-node",
        &registry.NodeMetadata{
            Name:        "My Custom Node",
            Description: "Does something custom",
            Category:    "utility",
        },
        func(config map[string]interface{}) (registry.NodeExecutor, error) {
            return &MyCustomNode{}, nil
        },
    )
}

// Required for plugin build
var _ = Register
```

### Plugin Discovery

The application can discover plugins in a directory:

```go
func loadPluginsFromDirectory(dir string) error {
    entries, err := os.ReadDir(dir)
    if err != nil {
        return err
    }
    
    for _, entry := range entries {
        if entry.IsDir() {
            continue
        }
        
        path := filepath.Join(dir, entry.Name())
        if filepath.Ext(path) == ".so" {
            if err := loadPlugin(path); err != nil {
                log.Printf("Failed to load plugin %s: %v", path, err)
            }
        }
    }
    
    return nil
}
```

---

## Testing the Registry

### Unit Tests

```go
func TestNodeRegistry_Register(t *testing.T) {
    reg := registry.NewNodeRegistry()

    factory := func() registry.NodeExecutor { return &MockNode{} }

    // Test successful registration
    err := reg.RegisterFactory("test-node", factory, registry.NodeMetadata{
        Type: "test-node",
        Name: "Test Node",
    })
    assert.NoError(t, err)

    // Test duplicate registration
    err = reg.RegisterFactory("test-node", factory, registry.NodeMetadata{Type: "test-node"})
    assert.Error(t, err)

    // Test retrieval
    metadata, err := reg.GetMetadata("test-node")
    assert.NoError(t, err)
    assert.Equal(t, "Test Node", metadata.Name)
}

func TestNodeRegistry_GetAllNodes(t *testing.T) {
    reg := registry.NewNodeRegistry()

    // Register multiple nodes
    reg.RegisterFactory("node1", func() registry.NodeExecutor { return &MockNode{} }, registry.NodeMetadata{Type: "node1", Name: "Node 1"})
    reg.RegisterFactory("node2", func() registry.NodeExecutor { return &MockNode{} }, registry.NodeMetadata{Type: "node2", Name: "Node 2"})

    nodes := reg.GetAllNodes()
    assert.Len(t, nodes, 2)
}
```

### Integration Tests

```go
func TestNodeRegistry_WithEngine(t *testing.T) {
    // Create isolated registry
    reg := registry.NewNodeRegistry()

    // Register test node
    reg.RegisterFactory("test-node", func() registry.NodeExecutor {
        return &MockNode{Output: "test"}
    }, registry.NodeMetadata{Type: "test-node"})

    // Create engine with this registry
    engine := NewFlowEngine(DefaultEngineConfig(), reg)

    // Verify node is available
    assert.True(t, reg.IsRegistered("test-node"))

    // Create flow with test node
    flow := NewFlow("test-flow", "Test")
    flow.Nodes["n1"] = &Node{Type: "test-node", Config: map[string]interface{}{}}

    // Should deploy successfully
    err := engine.Deploy(flow)
    assert.NoError(t, err)
    defer engine.Undeploy(flow.ID)
}
```

### Mock Nodes for Testing

```go
type MockNode struct {
    Output interface{}
    Error  error
}

func (n *MockNode) Execute(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error) {
    if n.Error != nil {
        return nil, n.Error
    }
    return map[string]interface{}{"output": n.Output}, nil
}

// Helper to create a mock node factory. NodeFactory takes no arguments
// (config is applied afterwards via NodeExecutor.SetConfig, see
// NodeRegistry.InitializeNode) — it is not `func(config) (NodeExecutor, error)`.
func mockNodeFactory(output interface{}, err error) registry.NodeFactory {
    return func() registry.NodeExecutor {
        return &MockNode{Output: output, Error: err}
    }
}
```

---

## Error Handling

There are no exported sentinel error values (no `ErrNodeTypeNotFound` /
`ErrNodeTypeExists` / etc.) — the real registry constructs plain errors
inline with `errors.New(...)` and descriptive messages, e.g.:

```go
func (r *NodeRegistry) GetMetadata(nodeType string) (NodeMetadata, error) {
    r.mu.RLock()
    defer r.mu.RUnlock()

    node, exists := r.nodes[nodeType]
    if !exists {
        return NodeMetadata{}, errors.New("node type not found: " + nodeType)
    }

    return node.Metadata, nil
}
```

If callers need to distinguish "not found" programmatically rather than by
string, that would be a new addition — check `internal/registry/registry.go`
before assuming a sentinel error exists.

---

## Performance Considerations

### Thread Safety
- All public methods use `sync.RWMutex` for thread-safe access
- Read operations use `RLock()` (multiple concurrent readers)
- Write operations use `Lock()` (exclusive access)
- Registry is safe for concurrent use

### Memory Usage
- Registry stores metadata and factory functions (small footprint)
- Node instances are created on-demand, not stored in registry
- Each node type entry is ~1-2KB
- Even with 1000+ node types, memory overhead is minimal

### Lookup Performance
- All lookups are O(1) map operations
- No iteration required for single lookups
- `GetAllNodes()` is O(n) but returns a slice copy

---

## Debugging

### Common Issues

#### Node Type Not Found
**Symptoms**: Node doesn't appear in UI, deployment fails

**Checks:**
1. Is `init()` being called?
2. Is registration happening before engine starts?
3. Are there duplicate IDs?
4. Is the registry the global registry?

**Solution:**
```go
// Verify registration
reg := registry.GetGlobalRegistry()
if !reg.IsRegistered("my-node") {
    log.Println("Node not registered!")
}

// List all nodes (GetAllNodes returns []NodeMetadata directly, not entries)
for _, metadata := range reg.GetAllNodes() {
    log.Printf("Registered: %s", metadata.Name)
}
```

#### Configuration Validation Errors
**Symptoms**: Node creation fails, error messages about config

**Checks:**
1. Does ConfigSchema match actual config usage?
2. Are required fields present?
3. Are types correct (string vs int vs float64)?

**Solution:**
```go
// Test configuration
config := map[string]interface{}{
    "field": "value",
}
node, err := reg.InitializeNode("my-node", config)
if err != nil {
    log.Printf("Config error: %v", err)
}
```

---

## Frontend Integration

The registry provides data for the frontend node palette:

### API Endpoint

**Route**: `GET /api/nodes`

**Handler** (in `cmd/go-red/main.go`):
```go
func handleGetNodes(w http.ResponseWriter, r *http.Request, reg *registry.NodeRegistry) {
    nodes := reg.GetAllNodes()
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(nodes)
}
```

### TypeScript Types

The frontend `NodeMetadata`/`Port`/`Schema`/`Property` (re-exported as
`PropertySchema`) types in `web/src/types/node.ts` are generated from the Go
structs above via `go generate ./internal/dto/...` into
`web/src/types/generated.ts` — do not hand-describe them again here or in
`node.ts`. See the "Interface-Verifikation" section at the end of this file
for the required regeneration/verification steps.

---

## Future Enhancements

### Planned Features
- **Node versioning**: Multiple versions of the same node type
- **Node deprecation warnings**: Notify users of deprecated nodes
- **Node dependencies**: Specify which nodes require other nodes
- **Node groups**: Group related nodes together
- **Custom categories**: Allow plugins to define custom categories
- **Node search**: Full-text search on node metadata

### Architecture Improvements
- **Registry events**: Notify when nodes are registered/unregistered
- **Lazy loading**: Load plugin nodes on-demand
- **Caching**: Cache frequently accessed node metadata
- **Validation hooks**: Run validation when nodes are registered

---

## Checklist for Registry Changes

Before committing changes to the registry:

- [ ] All existing tests pass (`go test ./internal/registry/...`)
- [ ] No race conditions (`go test -race ./internal/registry/...`)
- [ ] New functionality has tests
- [ ] Thread safety is maintained
- [ ] Frontend can consume new data
- [ ] Documentation is updated
- [ ] Backward compatibility is preserved

---

*Last updated: 2026-06-21*
*Overrides: None (extends internal/AGENTS.md and root AGENTS.md)*

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
