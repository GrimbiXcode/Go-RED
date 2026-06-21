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
├── nodeTypes: map[string]*NodeTypeEntry
├── mu: sync.RWMutex (thread-safe access)
└── builtIn: bool (indicates built-in registry)

NodeTypeEntry (Per-Node-Type)
├── Metadata: *NodeMetadata
└── Factory: NodeFactory (creates instances)

NodeMetadata (Type Information)
├── Name, Description, Category
├── Icon, Color
├── InputPorts, OutputPorts
└── ConfigSchema: map[string]ConfigProperty

ConfigProperty (Configuration Definition)
├── Type, Default, Required
├── Description, Placeholder
├── Options, Min, Max, Pattern
└── Editor, EditorConfig
```

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

---

## Development Guidelines

### Adding New Node Types

Nodes self-register in their `init()` function:

```go
func init() {
    registry.RegisterNodeType(
        "unique-node-id",
        &registry.NodeMetadata{...},
        factoryFunction,
    )
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
// Register a node type
func RegisterNodeType(
    id string,
    metadata *NodeMetadata,
    factory NodeFactory,
) error {
    // Validates ID is not empty
    // Checks for duplicates
    // Stores in registry
}
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

#### RegisterNodeType
```go
func (r *NodeRegistry) RegisterNodeType(
    id string,
    metadata *NodeMetadata,
    factory NodeFactory,
) error
```
Registers a new node type. Returns error if ID is empty or duplicate.

#### GetNodeType
```go
func (r *NodeRegistry) GetNodeType(id string) (*NodeTypeEntry, error)
```
Gets a registered node type by ID. Returns `ErrNodeTypeNotFound` if not found.

#### GetFactory
```go
func (r *NodeRegistry) GetFactory(id string) (NodeFactory, error)
```
Gets the factory function for a node type. Used to create instances.

#### GetMetadata
```go
func (r *NodeRegistry) GetMetadata(id string) (*NodeMetadata, error)
```
Gets metadata for a node type. Used by the UI to display node information.

#### GetAllNodes
```go
func (r *NodeRegistry) GetAllNodes() []*NodeTypeEntry
```
Returns all registered node types. Used to populate the node palette in the UI.

#### GetNodesByCategory
```go
func (r *NodeRegistry) GetNodesByCategory(category string) []*NodeTypeEntry
```
Returns node types filtered by category.

#### HasNodeType
```go
func (r *NodeRegistry) HasNodeType(id string) bool
```
Checks if a node type is registered.

#### UnregisterNodeType
```go
func (r *NodeRegistry) UnregisterNodeType(id string) error
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

```go
type NodeMetadata struct {
    // Identification
    Name        string `json:"name"`
    Description string `json:"description"`
    
    // Categorization
    Category string `json:"category"`
    Tags     []string `json:"tags,omitempty"`
    
    // Visual
    Icon  string `json:"icon"`
    Color string `json:"color"`
    
    // Ports
    InputPorts  []string `json:"inputPorts"`
    OutputPorts []string `json:"outputPorts"`
    
    // Configuration
    ConfigSchema map[string]ConfigProperty `json:"configSchema"`
    
    // Advanced
    Hidden      bool   `json:"hidden,omitempty"`
    Deprecated  bool   `json:"deprecated,omitempty"`
    Replaces    string `json:"replaces,omitempty"`
    Version     string `json:"version,omitempty"`
}
```

### ConfigProperty Structure

```go
type ConfigProperty struct {
    Type        string      `json:"type"`
    Default     interface{} `json:"default"`
    Required    bool        `json:"required"`
    Description string      `json:"description"`
    Placeholder  string      `json:"placeholder,omitempty"`
    Options      []string    `json:"options,omitempty"`
    Min          *float64    `json:"min,omitempty"`
    Max          *float64    `json:"max,omitempty"`
    Pattern      string      `json:"pattern,omitempty"`
    Editor       string      `json:"editor,omitempty"`
    EditorConfig interface{} `json:"editorConfig,omitempty"`
}
```

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
    
    // Test successful registration
    err := reg.RegisterNodeType(
        "test-node",
        &registry.NodeMetadata{
            Name: "Test Node",
        },
        func(config map[string]interface{}) (registry.NodeExecutor, error) {
            return &MockNode{}, nil
        },
    )
    assert.NoError(t, err)
    
    // Test duplicate registration
    err = reg.RegisterNodeType("test-node", &registry.NodeMetadata{}, nil)
    assert.Error(t, err)
    assert.Equal(t, registry.ErrNodeTypeExists, err)
    
    // Test retrieval
    entry, err := reg.GetNodeType("test-node")
    assert.NoError(t, err)
    assert.Equal(t, "Test Node", entry.Metadata.Name)
}

func TestNodeRegistry_GetAllNodes(t *testing.T) {
    reg := registry.NewNodeRegistry()
    
    // Register multiple nodes
    reg.RegisterNodeType("node1", &registry.NodeMetadata{Name: "Node 1"}, nil)
    reg.RegisterNodeType("node2", &registry.NodeMetadata{Name: "Node 2"}, nil)
    
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
    reg.RegisterNodeType("test-node", &registry.NodeMetadata{}, 
        func(config map[string]interface{}) (registry.NodeExecutor, error) {
            return &MockNode{Output: "test"}, nil
        },
    )
    
    // Create engine with this registry
    engine := NewFlowEngine(DefaultEngineConfig(), reg)
    
    // Verify node is available
    assert.True(t, reg.HasNodeType("test-node"))
    
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

// Helper to create mock node factory
func mockNodeFactory(output interface{}, err error) registry.NodeFactory {
    return func(config map[string]interface{}) (registry.NodeExecutor, error) {
        return &MockNode{Output: output, Error: err}, nil
    }
}
```

---

## Error Handling

### Standard Errors

```go
var (
    ErrNodeTypeNotFound = errors.New("node type not found")
    ErrNodeTypeExists   = errors.New("node type already exists")
    ErrInvalidNodeType  = errors.New("invalid node type")
    ErrEmptyNodeID      = errors.New("node ID cannot be empty")
)
```

**Usage:**
```go
func (r *NodeRegistry) GetNodeType(id string) (*NodeTypeEntry, error) {
    if id == "" {
        return nil, ErrEmptyNodeID
    }
    
    r.mu.RLock()
    defer r.mu.RUnlock()
    
    entry, ok := r.nodeTypes[id]
    if !ok {
        return nil, ErrNodeTypeNotFound
    }
    
    return entry, nil
}
```

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
if !reg.HasNodeType("my-node") {
    log.Println("Node not registered!")
}

// List all nodes
for _, entry := range reg.GetAllNodes() {
    log.Printf("Registered: %s", entry.Metadata.Name)
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
factory, _ := reg.GetFactory("my-node")
node, err := factory(config)
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

Frontend types should match registry metadata:

```typescript
// web/src/types/node.ts
interface NodeMetadata {
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

interface ConfigProperty {
    type: string;
    default: any;
    required: boolean;
    description: string;
    placeholder?: string;
    options?: string[];
    min?: number;
    max?: number;
    pattern?: string;
    editor?: string;
    editorConfig?: Record<string, any>;
}
```

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
