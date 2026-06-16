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
    })
}
```

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

## Best Practices

- Keep nodes stateless
- Use context for timeout/cancellation
- Validate configuration
- Handle errors gracefully
- Add descriptive metadata
