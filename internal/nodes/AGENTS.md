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
├── inject/         # Message injection node - manual message trigger
│   └── node.go
├── junction/       # Passthrough wire-routing point (no runtime effect)
│   └── node.go
├── comment/        # Editor annotation (no ports, no runtime effect)
│   └── node.go
├── catch/          # Fires on another node's error (registry.EmittingNode)
│   └── node.go
├── status/         # Fires on another node's status report (registry.EmittingNode)
│   └── node.go
├── complete/       # Fires on another node's successful completion (registry.EmittingNode)
│   └── node.go
├── linkin/         # "link in" - virtual entry point for linkout (same flow)
│   └── node.go
├── linkout/        # "link out" - delivers to a linkin node (registry.MultiOutputExecutor)
│   └── node.go
├── switchnode/      # "switch" - rule-based routing (registry.MultiOutputExecutor); named switchnode, "switch" is a Go keyword
│   └── node.go
├── change/         # Set/change/delete/move on msg/flow/global (typedvalue.PropertyRef)
│   └── node.go
├── rangenode/      # "range" - scale a numeric property; named rangenode, "range" is a Go keyword
│   └── node.go
├── rbe/            # Report-by-exception / deadband filter (registry.MultiOutputExecutor)
│   └── node.go
├── template/       # {{msg.path}} variable substitution into a message property
│   └── node.go
├── delay/          # Fixed delay or rate-limit (registry.EmittingNode + queue)
│   └── node.go
├── trigger/        # Send now, optional delayed second send (registry.Closeable timers)
│   └── node.go
├── execnode/       # "exec" - run an external command, argv-only, opt-in via GORED_ENABLE_EXEC; named execnode to avoid shadowing the imported os/exec package
│   └── node.go
├── jsonnode/       # "json" - JSON string <-> object (direction from payload type); named jsonnode to avoid shadowing the imported encoding/json package
│   └── node.go
├── yamlnode/       # "yaml" - YAML string <-> object; named yamlnode to avoid shadowing the imported yaml.v3 package
│   └── node.go
├── csvnode/        # "csv" - CSV string <-> array of rows; named csvnode to avoid shadowing the imported encoding/csv package
│   └── node.go
├── xmlnode/        # "xml" - XML string <-> object, own schema (see package doc); named xmlnode to avoid shadowing the imported encoding/xml package
│   └── node.go
├── htmlnode/       # "html" - CSS-selector-subset extraction; named htmlnode to avoid shadowing the imported golang.org/x/net/html package
│   └── node.go
├── split/          # Splits an array/object/string into N messages (registry.MultiOutputExecutor + NodeRuntime.SubmitToNode fan-out)
│   └── node.go
├── join/           # Reassembles a msg.parts group back into one message (stateful, registry.MultiOutputExecutor)
│   └── node.go
├── sortnode/       # "sort" - buffers a msg.parts group, re-emits sorted; named sortnode to avoid shadowing the imported stdlib sort package
│   └── node.go
├── batch/          # Groups messages by count or interval (registry.MultiOutputExecutor + registry.EmittingNode)
│   └── node.go
├── file/           # Writes/appends/deletes a file from msg.payload (registry.MultiOutputExecutor)
│   └── node.go
├── filein/         # "file in" - reads a file into msg.payload, whole or line-by-line; named filein ("link in" -> linkin is the precedent)
│   └── node.go
├── watch/          # Emits on filesystem change events (registry.EmittingNode, no input port, github.com/fsnotify/fsnotify)
│   └── node.go
├── tlsconfig/      # "tls-config" config node - reusable TLS cert/key/CA, Category "config"
│   └── node.go
├── httpproxy/      # "http proxy" config node - reusable outbound proxy settings, Category "config"
│   └── node.go
├── mqttbroker/     # "mqtt-broker" config node - shared paho.mqtt.golang client, Category "config"
│   └── node.go
├── mqttin/         # "mqtt in" - subscribes via a shared mqtt-broker (registry.EmittingNode, no input port)
│   └── node.go
├── mqttout/        # "mqtt out" - publishes via a shared mqtt-broker
│   └── node.go
├── httpin/         # "http in" - shared HTTP listener + router (registry.EmittingNode, no input port); also exports RegisterHandler for websocketlistener
│   ├── node.go
│   ├── server.go
│   └── response_handle.go
├── httpresponse/   # "http response" - completes the pending request an httpin node is holding open
│   └── node.go
├── httprequest/    # "http request" - outgoing HTTP call; see package doc for the security review
│   └── node.go
├── websocketlistener/ # "websocket-listener" config node - server-side WS endpoint on httpin's shared listener, Category "config"
│   └── node.go
├── websocketclient/   # "websocket-client" config node - outgoing WS connection, auto-reconnecting, Category "config"
│   └── node.go
├── websocketin/    # "websocket in" - emits on messages from a websocket-listener/-client (registry.EmittingNode, no input port)
│   └── node.go
├── websocketout/   # "websocket out" - sends via a websocket-listener/-client
│   └── node.go
├── tcpin/          # "tcp in" - server or client mode (registry.EmittingNode, no input port)
│   └── node.go
├── tcpout/         # "tcp out" - connects out and writes, one connection per message
│   └── node.go
├── tcprequest/     # "tcp request" - connects out, writes, half-closes, reads the reply
│   └── node.go
├── udpin/          # "udp in" - listens for datagrams (registry.EmittingNode, no input port)
│   └── node.go
└── udpout/         # "udp out" - sends a single datagram
    └── node.go
```

See `docs/NODE_PALETTE_PLAN.md` for the full Node-RED core palette this is working
towards, including which nodes are implemented, deferred, and why. `catch`/`status`/
`complete`/`linkin`/`linkout` are the first nodes that use `registry.NodeRuntime`
(`registry.RuntimeFromContext(ctx)` inside `Execute`/`ExecuteMulti`/`Start`) for
flow/global context, error/status/complete event subscription, and same-flow message
delivery — see `internal/registry/runtime.go`, `eventbus.go`, `context_store.go`.
`switchnode`/`change` are the first nodes to use `typedvalue.PropertyRef`
(`internal/typedvalue/propertyref.go`) for reading/writing a msg path or a flow/global
context key generically.

`split`/`join`/`sortnode` are the first nodes to use `msg.parts` grouping metadata -
represented as a plain `"parts"` key in the same `map[string]interface{}` used for
`msg.payload`/`msg.topic`/etc., not a dedicated engine field (an earlier
`engine.Message.Parts` field from Phase 0 had no producer/consumer and was removed
here - see `docs/NODE_PALETTE_PLAN.md`, Phase 4). They're also the first nodes where one
input message produces multiple *output* messages on the *same* port: since
`registry.MultiOutputExecutor` only allows one message per port, `split`/`sortnode`
return the first message as `ExecuteMulti`'s normal result and dispatch the rest via
`NodeRuntime.SubmitToNode(rt.NodeID, ...)` - the same mechanism `linkout` uses to reach
a *different* node, here targeting the node's own ID to re-enter its own outgoing wires.

`file`/`filein`/`watch` (Phase 5, `docs/NODE_PALETTE_PLAN.md`) are the storage-category
nodes. `file`/`filein` resolve their target path from a `typedvalue.Value` (usually a
fixed string, optionally `msg`/`env`/flow/global-sourced) rather than reading the path
straight off the message, mirroring Node-RED's typed-input widget for this field.
`filein`'s `"lines"` format reuses the same same-port `SubmitToNode` fan-out as
`split`. `watch` is the first `registry.EmittingNode` with *no* input port at all -
its `Execute` only exists to satisfy `NodeExecutor` (embedded in `EmittingNode`) and
always errors if called, since the engine never calls it for a node with no wired
input; `Start` runs `github.com/fsnotify/fsnotify` until the flow's context is
cancelled, reporting transient watcher errors via `NodeRuntime.ReportError` (so a Catch
node can observe them) rather than returning from `Start` and ending emission for the
rest of the flow's life.

`tlsconfig`/`httpproxy`/`mqttbroker`/`websocketlistener`/`websocketclient` (Phase 6,
`docs/NODE_PALETTE_PLAN.md`) are Go-RED's first **config nodes** - reusable
configuration/connection objects with `Category: "config"` and no ports, referenced by
ID from a plain string config property on a consuming node (e.g. `mqttin.Node.Broker`),
not through any dedicated wire-format support (`registry.Property.Type: "string"`, same
as any other property - no `NodeMetadata`/`Property` struct change, so the Interface-
Verification protocol below doesn't trigger). A consumer resolves the live instance via
`registry.NodeRuntime.GetNode(configNodeID)` (new in Phase 6 - `runtime.go`'s `getNode`
field/`GetNode` method, wired from `engine.go`'s `newNodeRuntime` off
`activeFlow.nodeExecutors`) and type-asserts it to a small Go interface the two packages
share directly (e.g. `mqttbroker.Broker`, `tlsconfig.Provider`) - a direct Go import
between sibling node packages, not a registry-mediated contract, since only a handful of
node types need each one.

**Config-node readiness vs. `Start()` ordering.** `engine.go`'s `startEmittingNodes`
launches one goroutine per `registry.EmittingNode` with *no ordering guarantee between
them* - so a config node that only becomes "ready" inside its own `Start` cannot safely
be consumed from another `EmittingNode`'s `Start` (a real race, not just Go's unordered
map iteration during Deploy's init loop, which every node's `SetConfig` already
tolerates by resolving config-node references lazily rather than at `SetConfig` time).
The fix used throughout Phase 6: `mqttbroker`/`websocketlistener`/`websocketclient` do
their connection setup (constructing the client, dialing, or mounting an HTTP handler)
directly inside `SetConfig` - which runs synchronously inside Deploy's single-node-at-a-
time init loop, always complete before any `Start` goroutine exists - and implement only
`registry.Closeable`, never `registry.EmittingNode`, themselves. `httpin` sidesteps the
question entirely: route registration happens in its own `Start` (it has no config-node
dependency to race against), and `websocketlistener` mounts on `httpin`'s shared
listener via the exported `httpin.RegisterHandler`, also called from `SetConfig`.

`httpin`/`httpresponse` correlate a live `http.ResponseWriter` with the message that
carries it through the flow via a `*httpin.ResponseHandle` stored under the plain
message key `httpin.KeyResponseHandle` - an ordinary `interface{}` value that survives
every node's shallow `cloneMap` helper along the way (the pointer is just copied, same as
any other map value), the same "put a live Go value on the message" approach `watch`
established for filesystem-event metadata. All `httpin`-derived nodes share a single
process-wide `*http.Server` (env var `GORED_HTTP_NODE_PORT`, default 1880), deliberately
separate from `cmd/go-red`'s own `-port` editor/API server - node packages self-register
via `init()` with no access to that server's `mux`.

`httprequest` is the first node in this codebase with a dedicated "Security" section in
its package doc (mirroring `execnode`'s), since it lets a flow issue arbitrary outgoing
requests, potentially to an attacker-influenced URL (SSRF) - deliberately *not*
mitigated here (parity with Node-RED's own http request node; see the package doc for
why an application-level block-list is the wrong tool), but timeouts/response-size
caps/redirect caps *are* enforced by default.

Eight directories are named `<type>node` instead of their registered Node-RED type ID
because that ID isn't a valid Go identifier or would collide with an import of the same
name: `switchnode`/`rangenode` (`switch`/`range` are Go keywords), and
`execnode`/`jsonnode`/`yamlnode`/`csvnode`/`xmlnode`/`htmlnode` (each imports a
stdlib/third-party package - `os/exec`, `encoding/json`, `yaml.v3`, `encoding/csv`,
`encoding/xml`, `golang.org/x/net/html` respectively - whose own package identifier
matches the node's natural name). The registered `NodeMetadata.Type`/factory key is
still the real Node-RED ID (`"switch"`, `"range"`, `"exec"`, `"json"`, `"yaml"`,
`"csv"`, `"xml"`, `"html"`); only the Go package/directory name differs.

**Category strings (not directories - every node type lives directly under `internal/nodes/`, one directory per type, regardless of category):**
- `flow-control` - `junction`/`comment`/`catch`/`status`/`complete`/`link in`/`link out`/`split`/`join`/`sort`/`batch`
- `network` - `mqtt in`/`mqtt out`/`http in`/`http response`/`http request`/`websocket in`/`websocket out`/`tcp in`/`tcp out`/`tcp request`/`udp in`/`udp out` (Phase 6)
- `storage` - `file`/`file in`/`watch` (Phase 5)
- `config` - `tls-config`/`http proxy`/`mqtt-broker`/`websocket-listener`/`websocket-client` (Phase 6) - no ports, referenced by ID, see the config-node section above
- `function` - `function`/`change`/`range`/`rbe`/`template`/`delay`/`trigger`/`exec`/`json`/`yaml`/`csv`/`xml`/`html`

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
| `flow-control` | Structural/control, no normal message transform | junction, comment, catch, status, complete, link in, link out |
| `logic` | Control flow | switch, condition, router, join |
| `network` | Network ops | mqtt in/out, http in/response/request, websocket in/out, tcp in/out/request, udp in/out |
| `storage` | Persistence | file, file in, watch |
| `config` | Reusable config/connection, no ports | tls-config, http proxy, mqtt-broker, websocket-listener, websocket-client |
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
