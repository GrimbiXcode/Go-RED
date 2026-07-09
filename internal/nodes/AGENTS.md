# Go-RED Nodes Package Guidelines

This file contains **node-specific** guidelines for developing and maintaining nodes in the `internal/nodes/` directory.

---

## Package Overview

The `nodes/` directory contains all **built-in node implementations** for Go-RED. Each subdirectory represents a node type:

```
internal/nodes/
├── debug/          # Debug output node - logs messages to console
│   └── node.go
├── function/       # JavaScript function node - executes JS code
│   └── node.go
└── inject/         # Message injection node - manual message trigger
    └── node.go
```

**Future Categories (planned):**
- `input/` - Message sources (websocket, HTTP, MQTT, etc.)
- `output/` - Message sinks (file, database, HTTP response, etc.)
- `logic/` - Control flow (switch, condition, router, etc.)
- `transform/` - Data transformation (JSON, template, encode, etc.)
- `network/` - Network operations (HTTP request, TCP, UDP, etc.)
- `storage/` - Persistence (Redis, MongoDB, PostgreSQL, etc.)
- `utility/` - Helper nodes (delay, rate-limit, batch, etc.)

---

## Node Architecture

### Node Type System

All nodes implement the **`NodeExecutor`** interface from `internal/registry`:

```go
type NodeExecutor interface {
    Execute(ctx interface{}, input map[string]interface{}) (map[string]interface{}, error)
    Validate() error
    GetConfig() map[string]interface{}
    SetConfig(config map[string]interface{}) error
}
```
(the real interface, `internal/registry/registry.go`, has four methods, not
one — `Validate`/`GetConfig`/`SetConfig` are required too)

### Node Registration Pattern

Every node must register itself in its `init()` function:

```go
func init() {
    reg := registry.GetGlobalRegistry()
    err := reg.RegisterFactory("node-type-id", func() registry.NodeExecutor {
        // Factory function - creates a new (unconfigured) node instance.
        // Config is applied afterwards via NodeExecutor.SetConfig.
        return &MyNode{}
    }, registry.NodeMetadata{
        ID:          "node-type-id",
        Type:        "node-type-id",
        Name:        "Human Readable Name",
        Description: "What this node does",
        Category:    "category",
        Icon:        "icon-name",
        Inputs:      []registry.Port{{ID: "input1", Name: "input1"}, {ID: "input2", Name: "input2"}},
        Outputs:     []registry.Port{{ID: "output1", Name: "output1"}, {ID: "output2", Name: "output2"}},
        ConfigSchema: registry.Schema{
            Properties: map[string]registry.Property{
                "propertyName": {
                    Type:        "string",
                    Default:     "defaultValue",
                    Description: "Property description",
                },
            },
            Required: []string{"propertyName"},
        },
    })
    if err != nil {
        panic(err)
    }
}
```
(see `internal/nodes/debug/node.go` for a real, working example — note that
`NodeFactory` takes no arguments and `ConfigSchema` is `registry.Schema`, not
a bare `map[string]registry.ConfigProperty`; `registry.ConfigProperty`
doesn't exist, the real type is `registry.Property`)

---

## Node Development Guide

### Step 1: Create Node Directory

```bash
mkdir -p internal/nodes/[node-name]
```

### Step 2: Implement the Node

**File: `internal/nodes/[node-name]/node.go`**

```go
package [nodename]

import (
    "context"
    "fmt"
    "log"

    "github.com/GrimbiXcode/Go-RED/internal/registry"
)

// Node struct holds node configuration
type Node struct {
    // Configuration fields - should match ConfigSchema
    ConfigField string `json:"configField"`
    // Internal state (if needed)
    initialized bool
}

// Execute implements the NodeExecutor interface
func (n *Node) Execute(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error) {
    // 1. Validate input
    if input == nil {
        return nil, fmt.Errorf("input cannot be nil")
    }

    // 2. Extract configuration from input or node state
    // input["config"] may contain runtime overrides

    // 3. Process the message
    output := make(map[string]interface{})
    
    // Copy input to output by default
    for k, v := range input {
        output[k] = v
    }
    
    // Add node-specific processing
    // ...

    // 4. Return output or error
    return output, nil
}

// init registers the node type
func init() {
    reg := registry.GetGlobalRegistry()
    err := reg.RegisterFactory("my-node", func() registry.NodeExecutor {
        return &Node{}
    }, registry.NodeMetadata{
        ID:          "my-node",
        Type:        "my-node",
        Name:        "My Node",
        Description: "A custom node that does something",
        Category:    "utility",
        Icon:        "cog",
        Inputs:      []registry.Port{{ID: "input", Name: "input"}},
        Outputs:     []registry.Port{{ID: "output", Name: "output"}},
        ConfigSchema: registry.Schema{
            Properties: map[string]registry.Property{
                "configField": {
                    Type:        "string",
                    Default:     "default",
                    Description: "Example configuration",
                },
            },
        },
    })
    if err != nil {
        panic(err)
    }
}
```
(`Node.SetConfig` — required by `NodeExecutor` — is where `config["configField"]`
gets extracted at runtime, not the factory function; there is no `Color` field
on `NodeMetadata`)

---

## Existing Nodes Documentation

### Debug Node (`debug/`)

**Purpose**: Output messages to the console for debugging

**Configuration**: None required

**Behavior**:
- Receives any message
- Logs the message payload to stdout
- Passes the message through unchanged

**Use Case**: Development, debugging flows, monitoring message flow

---

### Function Node (`function/`)

**Purpose**: Execute JavaScript code to transform messages

**Configuration**:
- `function`: string - JavaScript code to execute
- `output`: string - Optional output field name (default: "payload")

**Behavior**:
- Executes JavaScript code with message payload as context
- Uses Goja JavaScript runtime
- Returns the result as the message payload
- Can access `msg` object with input data

**Example Configuration**:
```json
{
  "function": "return { result: msg.payload * 2 };",
  "output": "result"
}
```

**Security**:
- JavaScript runs in a sandboxed environment (Goja)
- No access to filesystem or network by default
- Timeout protection via context

---

### Inject Node (`inject/`)

**Purpose**: Manually inject messages into a flow

**Configuration**:
- `payload`: map - The message payload to inject
- `topic`: string - Optional message topic
- `repeat`: number - Optional repeat interval in seconds

**Behavior**:
- When triggered (via API or button), sends configured payload
- Can repeat at configured interval
- Payload can be any JSON-serializable data

**Use Case**: Testing flows, periodic triggers, manual message injection

---

## Node Development Best Practices

### 1. Configuration

**Do:**
- Define a clear ConfigSchema
- Provide sensible defaults
- Validate configuration in the factory function
- Document all configuration options

**Don't:**
- Make required fields without defaults
- Accept arbitrary configuration without validation
- Store configuration in global state

### 2. Execution

**Do:**
- Return errors for recoverable failures
- Use context for timeout and cancellation
- Make nodes idempotent where possible
- Preserve message metadata (msg.ID, msg.Timestamp)
- Handle nil inputs gracefully

**Don't:**
- Panic on bad input (return error instead)
- Block indefinitely (respect context timeout)
- Modify the input map directly (create a copy if needed)
- Leak goroutines or resources

### 3. Error Handling

**Pattern:**
```go
func (n *Node) Execute(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error) {
    // Validate
    if input == nil {
        return nil, fmt.Errorf("input cannot be nil")
    }
    
    // Process with error handling
    result, err := n.process(input)
    if err != nil {
        // Optionally add error details to output
        output := make(map[string]interface{})
        output["error"] = err.Error()
        output["original"] = input
        return output, fmt.Errorf("processing failed: %w", err)
    }
    
    return result, nil
}
```

### 4. State Management

**For Stateless Nodes:**
- All configuration comes from constructor
- No internal state between Execute calls
- Thread-safe by design

**For Stateful Nodes:**
- Use mutexes to protect internal state
- Document thread-safety guarantees
- Consider using sync.Map for concurrent access
- Clean up state when node is no longer used

```go
type StatefulNode struct {
    mu      sync.Mutex
    counter int
}

func (n *StatefulNode) Execute(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error) {
    n.mu.Lock()
    defer n.mu.Unlock()
    
    n.counter++
    
    output := make(map[string]interface{})
    for k, v := range input {
        output[k] = v
    }
    output["count"] = n.counter
    
    return output, nil
}
```

---

## Node Categories & Conventions

### Category Definitions

| Category | Purpose | Example Nodes |
|----------|---------|--------------|
| `input` | Message sources | inject, websocket-in, http-in, mqtt-in |
| `output` | Message sinks | debug, file-out, http-out, mqtt-out |
| `function` | Transformation | function, template, json |
| `logic` | Control flow | switch, condition, router, join |
| `network` | Network ops | http-request, tcp, websocket |
| `storage` | Persistence | redis, mongodb, postgres, file |
| `utility` | Helpers | delay, rate-limit, batch, counter |
| `sensor` | Data acquisition | serial, gpio, ble |
| `dashboard` | UI elements | gauge, chart, text, button |

### Color Coding

`registry.NodeMetadata` has no `Color` field — category colors are a
frontend-only concern (see `categoryColors` in `web/src/components/
NodePalette.tsx` and `NodeComponent.tsx`), keyed by the same category string
used here. Keep the two in sync by hand when adding a new category; don't add
a `Color` field to the Go struct without also adding it to the generated
wire type via `internal/dto`/`cmd/gentypes`.

### Icon Naming

Use consistent icon names from a common icon set (e.g., Font Awesome, Material Icons):

- `arrow-right` - input nodes
- `arrow-left` - output nodes
- `code` - function nodes
- `project-diagram` - logic nodes
- `server` - network nodes
- `database` - storage nodes
- `clock` - timing nodes
- `exclamation-triangle` - error handling

---

## Testing Nodes

### Unit Tests

Test each node in isolation:

```go
package mynode_test

import (
    "context"
    "testing"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestNode_Execute(t *testing.T) {
    // Setup
    config := map[string]interface{}{
        "configField": "test-value",
    }
    
    // Create node through registry
    registry := registry.GetGlobalRegistry()
    factory, err := registry.GetFactory("my-node")
    require.NoError(t, err)
    
    node, err := factory(config)
    require.NoError(t, err)
    
    // Execute
    input := map[string]interface{}{
        "payload": "test input",
    }
    
    output, err := node.Execute(context.Background(), input)
    
    // Verify
    require.NoError(t, err)
    assert.NotNil(t, output)
    // Add specific assertions for your node
}

func TestNode_Configuration(t *testing.T) {
    tests := []struct {
        name     string
        config   map[string]interface{}
        wantErr  bool
        errMsg   string
    }{
        {
            name: "valid config",
            config: map[string]interface{}{
                "configField": "value",
            },
            wantErr: false,
        },
        {
            name: "invalid type",
            config: map[string]interface{}{
                "configField": 123, // Should be string
            },
            wantErr: true,
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            registry := registry.GetGlobalRegistry()
            factory, _ := registry.GetFactory("my-node")
            
            _, err := factory(tt.config)
            
            if tt.wantErr {
                require.Error(t, err)
                if tt.errMsg != "" {
                    assert.Contains(t, err.Error(), tt.errMsg)
                }
            } else {
                require.NoError(t, err)
            }
        })
    }
}
```

### Integration Tests

Test nodes in the context of a running flow:

```go
func TestNode_Integration(t *testing.T) {
    // Setup engine
    registry := registry.GetGlobalRegistry()
    engine := NewFlowEngine(DefaultEngineConfig(), registry)
    
    // Create flow with test node
    flow := NewFlow("test-flow", "Test Flow")
    flow.Nodes["input"] = &Node{Type: "inject", Config: map[string]interface{}{}}
    flow.Nodes["my-node"] = &Node{
        Type: "my-node",
        Config: map[string]interface{}{
            "configField": "test",
        },
    }
    flow.Nodes["output"] = &Node{Type: "debug", Config: map[string]interface{}{}}
    
    flow.Connections = []NodeConnection{
        {ID: "1", SourceNode: "input", TargetNode: "my-node"},
        {ID: "2", SourceNode: "my-node", TargetNode: "output"},
    }
    
    // Deploy and test
    require.NoError(t, engine.Deploy(flow))
    defer engine.Undeploy(flow.ID)
    
    // Inject message and verify output
    err := engine.InjectMessage(flow.ID, "input", map[string]interface{}{
        "test": "data",
    })
    require.NoError(t, err)
    
    // Check debug output or message log
    messages := engine.GetMessageLogForFlow(flow.ID)
    assert.Len(t, messages, 1)
    // Verify message was processed correctly
}
```

---

## Node Configuration Schema

The `ConfigSchema` in `NodeMetadata` defines how nodes are configured in the UI.
It is `registry.Schema`, not a bare map — required property *names* are listed
separately from the properties themselves, and there is no
`Placeholder`/`Options`/`Editor`/`EditorConfig` (those were never implemented):

```go
ConfigSchema: registry.Schema{
    Properties: map[string]registry.Property{
        "propertyName": {
            Type:        "string|number|boolean|array|object",
            Default:     interface{}, // Default value
            Description: string,      // Tooltip text
            Enum:        []string,    // For select/dropdown
            Min:         *float64,    // Minimum value (for numbers)
            Max:         *float64,    // Maximum value (for numbers)
            Pattern:     string,      // Regex pattern (for strings)
        },
    },
    Required: []string{"propertyName"}, // names of required properties
}
```

### Property Types

| Type | Description | Example |
|------|-------------|---------|
| `string` | Text input | `"hello"` |
| `number` | Numeric input | `42` |
| `boolean` | Checkbox | `true` |
| `array` | List of values | `[1, 2, 3]` |
| `object` | Key-value pairs | `{"key": "value"}` |

### Editor Types

| Editor | Usage | Config |
|--------|-------|--------|
| `text` | Single line text | - |
| `textarea` | Multi-line text | `rows: 5` |
| `number` | Numeric input | `min: 0, max: 100, step: 1` |
| `checkbox` | Boolean toggle | - |
| `select` | Dropdown | `options: ["a", "b", "c"]` |
| `code` | Code editor | `language: "javascript"` |
| `password` | Hidden input | - |
| `json` | JSON editor | - |

---

## Metadata Best Practices

### Good Metadata Examples

```go
&registry.NodeMetadata{
    ID:          "http-request",
    Type:        "http-request",
    Name:        "HTTP Request",
    Description: "Make HTTP requests to external services",
    Category:    "network",
    Icon:        "globe",

    // Clear port definitions (Port, not a bare string)
    Inputs:  []registry.Port{{ID: "input", Name: "input"}},
    Outputs: []registry.Port{{ID: "output", Name: "output"}, {ID: "error", Name: "error"}},

    // Comprehensive config schema
    ConfigSchema: registry.Schema{
        Properties: map[string]registry.Property{
            "method": {
                Type:        "string",
                Default:     "GET",
                Description: "HTTP method",
                Enum:        []string{"GET", "POST", "PUT", "DELETE", "PATCH"},
            },
            "url": {
                Type:        "string",
                Default:     "",
                Description: "Request URL",
            },
            "timeout": {
                Type:        "number",
                Default:     30,
                Description: "Request timeout in seconds",
                Min:         floatPtr(1),
                Max:         floatPtr(300),
            },
        },
        Required: []string{"method", "url"},
    },
}
```

### Bad Metadata Examples

```go
// ❌ Avoid - missing required fields
&registry.NodeMetadata{
    Name: "My Node",
    // Missing description, category, etc.
}

// ❌ Avoid - unclear config schema
&registry.NodeMetadata{
    ConfigSchema: registry.Schema{
        Properties: map[string]registry.Property{
            "x": {Type: "string"}, // No description or default
        },
    },
}

// ❌ Avoid - inconsistent naming
&registry.NodeMetadata{
    Name: "my-node", // Should be title case
    Category: "MISC", // Should be lowercase
}
```

---

## Node Validation

Validate node configuration in the factory function:

```go
func factory(config map[string]interface{}) (registry.NodeExecutor, error) {
    node := &MyNode{}
    
    // Type assertion with error
    if val, ok := config["requiredField"].(string); ok {
        if val == "" {
            return nil, fmt.Errorf("requiredField cannot be empty")
        }
        node.RequiredField = val
    } else {
        return nil, fmt.Errorf("requiredField must be a string")
    }
    
    // Optional field with default
    if val, ok := config["optionalField"].(int); ok {
        node.OptionalField = val
    } else {
        node.OptionalField = 42 // default
    }
    
    // Range validation
    if node.OptionalField < 1 || node.OptionalField > 100 {
        return nil, fmt.Errorf("optionalField must be between 1 and 100")
    }
    
    return node, nil
}
```

---

## Helper Utilities

### Common Node Utilities

```go
// CopyMap creates a deep copy of a map
func CopyMap(src map[string]interface{}) map[string]interface{} {
    dst := make(map[string]interface{}, len(src))
    for k, v := range src {
        dst[k] = v
    }
    return dst
}

// GetString extracts a string from input with default
func GetString(input map[string]interface{}, key string, defaultValue string) string {
    if val, ok := input[key].(string); ok {
        return val
    }
    return defaultValue
}

// GetInt extracts an int from input with default
func GetInt(input map[string]interface{}, key string, defaultValue int) int {
    if val, ok := input[key].(float64); ok {
        return int(val)
    }
    return defaultValue
}

// HasKey checks if a key exists in the map
func HasKey(input map[string]interface{}, key string) bool {
    _, ok := input[key]
    return ok
}

// MergeMaps merges two maps (src into dst)
func MergeMaps(dst, src map[string]interface{}) map[string]interface{} {
    result := CopyMap(dst)
    for k, v := range src {
        result[k] = v
    }
    return result
}
```

---

## Performance Considerations

### Fast Nodes
- Avoid expensive operations in Execute()
- Use efficient data structures
- Minimize allocations
- Return early for error cases

### Slow Nodes
- Use context timeout
- Provide progress updates (future enhancement)
- Consider chunking large operations
- Document performance characteristics

### Resource-Intensive Nodes
- Limit concurrent execution (use semaphores)
- Clean up resources after execution
- Provide configuration for resource limits
- Document resource requirements

---

## Documentation Standards

Each node should have:

1. **Code Documentation**: Comments in node.go explaining behavior
2. **Metadata**: Clear name, description, category
3. **Config Documentation**: Descriptions for all configuration options
4. **Examples**: Usage examples in documentation
5. **Limitations**: Document any known limitations

---

## Checklist for New Nodes

Before adding a new node:

- [ ] Node implements `NodeExecutor` interface
- [ ] Node registers itself in `init()`
- [ ] Node has complete `NodeMetadata`
- [ ] All configuration options are documented
- [ ] Node handles nil input gracefully
- [ ] Node respects context cancellation
- [ ] Node doesn't leak goroutines or resources
- [ ] Node has unit tests
- [ ] Node has integration tests (if applicable)
- [ ] Node works with the existing flow engine
- [ ] Node metadata matches UI expectations
- [ ] Node icon and color are appropriate

---

## Common Issues & Solutions

### Issue: Node doesn't appear in UI
**Solution**: Check that `init()` is being called and registration succeeded

### Issue: Configuration not being applied
**Solution**: Verify config extraction in factory function

### Issue: Node panics on execution
**Solution**: Add nil checks and proper error handling

### Issue: Node blocks flow execution
**Solution**: Use context timeout or move to goroutine with proper error handling

### Issue: Configuration validation errors
**Solution**: Check ConfigSchema type definitions match actual usage

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
