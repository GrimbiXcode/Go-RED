# Node Development Guide

## Overview

This guide explains how to develop custom nodes for Go—RED.

---

## Node Types

- **Input Nodes** - Receive data from external sources
- **Output Nodes** - Send data to external destinations
- **Function Nodes** - Transform or process data
- **Flow Control Nodes** - Control flow execution
- **Storage Nodes** - Store/retrieve data

---

## Node Interface

All nodes must implement the NodeExecutor interface:

```go
type NodeExecutor interface {
    Execute(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error)
    Validate() error
    GetConfig() map[string]interface{}
    SetConfig(config map[string]interface{}) error
}
```

---

## Developing Go Nodes

### Step 1: Create Node Structure

Create a new directory under internal/nodes/ or plugins/go/your-plugin/nodes/

### Step 2: Implement the Node

```go
package mynode

import "github.com/GrimbiXcode/Go-RED/internal/registry"

type MyNode struct {
    config MyNodeConfig
}

type MyNodeConfig struct {
    Greeting string `json:"greeting"`
    Count   int    `json:"count"`
}

func NewMyNode() *MyNode {
    return &MyNode{config: MyNodeConfig{Greeting: "Hello", Count: 1}}
}

func (n *MyNode) Execute(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error) {
    // Process input and return output
    return input, nil
}

func (n *MyNode) Validate() error {
    if n.config.Greeting == "" {
        return errors.New("greeting cannot be empty")
    }
    return nil
}

func (n *MyNode) GetConfig() map[string]interface{} {
    return map[string]interface{}{"greeting": n.config.Greeting, "count": n.config.Count}
}

func (n *MyNode) SetConfig(config map[string]interface{}) error {
    if greeting, ok := config["greeting"].(string); ok {
        n.config.Greeting = greeting
    }
    if count, ok := config["count"].(float64); ok {
        n.config.Count = int(count)
    }
    return n.Validate()
}
```

### Step 3: Register the Node

```go
func init() {
    reg := registry.GetGlobalRegistry()
    reg.RegisterFactory("my-node", func() registry.NodeExecutor {
        return NewMyNode()
    }, registry.NodeMetadata{
        ID: "my-node",
        Type: "my-node",
        Name: "My Custom Node",
        Description: "A custom node",
        Category: "function",
        Inputs: []registry.Port{{ID: "input", Name: "Input", Required: true}},
        Outputs: []registry.Port{{ID: "output", Name: "Output", Required: true}},
        ConfigSchema: registry.Schema{
            Properties: map[string]registry.Property{
                "url": {
                    Type: "string", Label: "URL", Order: 1, Widget: registry.WidgetText,
                    Placeholder: "https://example.org", Description: "Where to send the request",
                },
                "timeoutMs": {
                    Type: "number", Label: "Timeout", Order: 2, Widget: registry.WidgetDuration, Unit: "ms",
                    Default: float64(30000), Group: "Connection",
                },
                "mode": {
                    Type: "string", Label: "Mode", Order: 3, Widget: registry.WidgetSelect, Default: "simple",
                    Options: []registry.Option{{Value: "simple", Label: "Simple"}, {Value: "batch", Label: "Batch"}},
                },
                "batchSize": {
                    Type: "number", Label: "Batch size", Order: 4, Widget: registry.WidgetNumber,
                    VisibleWhen: &registry.Condition{Property: "mode", Values: []string{"batch"}},
                },
            },
            Required: []string{"url"},
        },
        Help: "**Sends requests** to a URL. Supports a batch mode.",
        Icon: "globe", // a Lucide icon name, see internal/nodes/AGENTS.md
    })
}
```

### Configuration schema

`ConfigSchema` is what the edit tray renders and validates (the full field
reference is in `docs/PROTOCOL.md`, section "Node schemas"). Rules of thumb:

- Give every property a `Label`, an `Order` and a `Widget` (constants in
  `internal/registry/schema.go`); `Description` is the help text under the
  field. Use `Group` to split long forms into sections.
- Use `WidgetTypedInput` with `registry.PropertyRefTypes` for a location
  (`{type, path}`, resolved with `typedvalue.PropertyRef`) and with
  `registry.ValueTypes` for a value (`{type, value}`, resolved with
  `typedvalue.Value`).
- Use `WidgetList` with `Items` for an ordered list of objects (rules), and
  `OutputsFrom` on the metadata when each element should own an output port.
- Use `WidgetNodeSelect` with `NodeTypes` to reference a config node of the
  same flow (`mqtt-broker`, `tls-config`, ...), `WidgetDuration` with `Unit`
  for times, `WidgetCode` with `Language` for code and templates.
- `VisibleWhen` hides a property until another one has a matching value.
- Add a `Help` text in Markdown; the tray and the Info sidebar show it.

`internal/nodes/schema_test.go` runs `NodeMetadata.Check` over every
registered node, so an unknown widget or a `VisibleWhen` pointing at a
missing property fails the build.

---

## Developing JavaScript Nodes

### Step 1: Create Node Directory

plugins/js/my-js-node/
├── node.js
└── manifest.json

### Step 2: Implement Node in JavaScript

```javascript
function process(input) {
    return { payload: input.payload.toUpperCase() };
}
```

### Step 3: Create Manifest

```json
{
  "id": "my-js-node",
  "type": "my-js-node",
  "name": "My JS Node",
  "description": "A JavaScript node",
  "category": "function",
  "inputs": [{"id": "input", "name": "Input", "required": true}],
  "outputs": [{"id": "output", "name": "Output", "required": true}],
  "configSchema": {
    "properties": {
      "transform": {"type": "string", "description": "Transformation", "default": "toUpperCase"}
    }
  }
}
```

---

## Shared helpers: `internal/nodes/base`

Do not copy config parsing and value conversion into a new node; the
`base` package has them, tested once:

```go
import "github.com/GrimbiXcode/Go-RED/internal/nodes/base"

func (n *Node) SetConfig(config map[string]interface{}) error {
    c := base.Config(config)
    n.Host = c.String("host", "localhost")
    n.Port = c.Int("port", 1883)            // float64, int, json.Number, "1883"
    n.Timeout = c.Duration("timeoutMs", 5*time.Second) // numbers are ms, strings via time.ParseDuration
    n.Retain = c.Bool("retain", false)
    n.Property = base.ParsePropertyRef(config["property"], typedvalue.PropertyRef{Type: typedvalue.TypeMsg, Path: "payload"})
    return n.Validate()
}
```

or, for a node with many fields, decode the whole map into a struct with
`json` tags: `base.Decode(config, &settings)`. `base.ToFloat`, `base.ToInt`,
`base.ToString`, `base.ToBytes` and `base.ToBool` convert message values the
way the built-in nodes do; `base.ParseValue`/`base.ValueToConfig` and
`base.ParsePropertyRef`/`base.PropertyRefToConfig` round-trip typed inputs;
`base.Context(ctx)` turns the `interface{}` the engine passes into a
`context.Context`, `base.Runtime(ctx)` returns the `NodeRuntime` the engine
attached (status, errors, debug, flow/global context) and
`base.Resolvers(ctx, msg)` builds the typed-value resolvers over the
message and both contexts.

## Context cancellation

The engine cancels a node's context when the flow is undeployed or
redeployed and when the per-message timeout expires, then waits for the
execution to return before it closes the node's resources. A node that
ignores its context holds that wait up (bounded by the timeout) and can
leak a goroutine, timer or connection. So:

- dial with a context (`(&net.Dialer{}).DialContext(ctx, ...)`,
  `websocket.Dialer.DialContext`, `http.NewRequestWithContext`);
- derive deadlines from it: `if d, ok := ctx.Deadline(); ok { conn.SetDeadline(d) }`,
  and close a blocking connection when it ends (`context.AfterFunc(ctx, func() { conn.Close() })`);
- never `time.Sleep`; select on a stopped timer and `ctx.Done()`;
- in `Start(ctx, emit)`, return when `ctx` is done and close listeners so
  accept and read loops return; drop emits after that;
- release queued work in `Close()` (delay, batch and join do);
- wrap the context error so callers can `errors.Is(err, context.Canceled)`.

Every network node has a `ctx_test.go` proving that a cancelled context
makes `Execute` return promptly and that `Start` returns after cancel; add
one for a new node.

## Best Practices

- Keep nodes stateless
- Use context for timeout/cancellation (see above)
- Validate configuration
- Handle errors gracefully
- Add descriptive metadata
