# Go-RED Internal Package Guidelines

This file contains guidelines specific to the **`internal/`** directory, which houses the core backend packages of Go-RED.

---

## Package Overview

The `internal/` directory contains all internal Go packages that power Go-RED:

```
internal/
├── engine/      # Flow execution engine - CORE COMPONENT
├── nodes/       # Built-in node implementations
├── registry/    # Node type registration and discovery
└── state/       # Flow persistence management
```

---

## Package-Specific Guidelines

### Cross-Package Dependencies

**Allowed Dependency Direction:**
```
registry/ ← nodes/ (nodes register themselves)
          ↓
engine/ ─────┬───── state/
             ↓
          cmd/go-red/
```

**Rules:**
- ✅ `engine/` can import `registry/` and `state/`
- ✅ `nodes/*` can import `registry/`
- ❌ `registry/` should NOT import `engine/` (circular dependency)
- ❌ `state/` should NOT import `engine/` or `registry/`
- ❌ `internal/` packages should NOT be imported by packages outside `internal/`

---

## Coding Standards

### Error Handling
```go
// Good - wrapped errors with context
func (e *FlowEngine) Deploy(flow *Flow) error {
    if err := flow.Validate(); err != nil {
        return errors.New("invalid flow: " + err.Error())
    }
    // ...
}

// Good - errors.Is for type checking
if errors.Is(err, ErrFlowNotFound) {
    return http.StatusNotFound
}
```

### Concurrency Patterns
```go
// Good - proper WaitGroup usage
func (e *FlowEngine) Start() error {
    for i := 0; i < e.config.WorkerPoolSize; i++ {
        e.wg.Add(1)
        go e.worker()
    }
    e.wg.Add(1)
    go e.processMessages()
    return nil
}

// Good - context propagation
ctx, cancel := context.WithTimeout(parentCtx, e.config.DefaultTimeout)
defer cancel()
```

### Logging
```go
// Good - use standard log package with consistent format
log.Println("Starting FlowEngine...")
log.Printf("Flow %s deployed successfully", flow.ID)
log.Printf("Node %s in flow %s failed: %v", nodeID, flowID, err)

// Avoid - raw print statements
// fmt.Println("Starting...") // ❌
```

---

## Package-Specific Instructions

### For `engine/` Package

**Core Responsibilities:**
- Flow lifecycle management (Create, Deploy, Undeploy, Delete)
- Message routing and processing
- Worker pool management
- Node execution coordination
- Message logging and debugging

**Key Types:**
- `FlowEngine` - Main orchestrator
- `ActiveFlow` - Deployed flow with execution context
- `Flow` - Flow definition and structure
- `Message` - Data passed between nodes
- `EngineConfig` - Configuration options

**Performance Considerations:**
- Message channels should have appropriate buffer sizes
- Worker pool size should be configurable
- Avoid memory leaks in long-running flows
- Message log should have size limits (circular buffer)

**Testing:**
- Test concurrent message processing
- Test flow deployment/undeployment under load
- Test error recovery scenarios
- Test message ordering guarantees

---

### For `nodes/` Package

**Core Responsibilities:**
- Built-in node implementations
- Node type definitions
- Node execution logic
- Node configuration validation

**Node Development Pattern:**
```go
package [nodetype]

import "github.com/GrimbiXcode/Go-RED/internal/registry"

type Node struct {
    // Node-specific configuration
}

func (n *Node) Execute(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error) {
    // Process input, return output
}

func init() {
    registry.RegisterNodeType("node-type", &registry.NodeMetadata{
        Name:        "Node Type Name",
        Description: "What this node does",
        // ...
    }, func(config map[string]interface{}) (registry.NodeExecutor, error) {
        return &Node{/* init from config */}, nil
    })
}
```

**Node Categories:**
- `input/` - Message sources (inject, websocket, HTTP)
- `output/` - Message sinks (debug, file, database)
- `function/` - Transformation nodes (function, template)
- `logic/` - Control flow (switch, condition)
- `network/` - Network operations (HTTP request, TCP)

**Testing:**
- Test each node type in isolation
- Test with various input types
- Test error handling
- Test configuration validation

---

### For `registry/` Package

**Core Responsibilities:**
- Node type registration
- Node metadata storage
- Node factory functions
- Node discovery and listing

**Key Types:**
- `NodeRegistry` - Central registry of all node types
- `NodeMetadata` - Type information (name, description, category)
- `NodeExecutor` - Interface for node execution
- `NodeFactory` - Function to create node instances

**Global Registry:**
- Use `registry.GetGlobalRegistry()` to access the singleton
- Nodes register themselves in `init()` functions
- Registration should be idempotent

---

### For `state/` Package

**Core Responsibilities:**
- Flow persistence (save, load, delete)
- State manager abstraction
- Support for multiple backends (file, database)

**Interface Design:**
```go
type StateManager interface {
    SaveFlow(flow *engine.Flow) error
    LoadFlow(flowID string) (*engine.Flow, error)
    LoadAllFlows() ([]*engine.Flow, error)
    DeleteFlow(flowID string) error
}
```

**Current Implementation:**
- `FileStateManager` - JSON files in `data/flows/`
- Future: PostgreSQL, MongoDB, etc.

**Testing:**
- Test persistence round-trip (save then load)
- Test concurrent access
- Test error handling for missing files
- Test file system permissions

---

## Performance Guidelines

### Memory Management
- Use `sync.RWMutex` for read-heavy, write-occasional access
- Use `sync.Pool` for frequently allocated, short-lived objects
- Limit message log size to prevent unbounded memory growth
- Clean up resources when flows are undeployed

### Concurrency
- Use `context.Context` for cancellation and timeouts
- Always use `defer wg.Done()` in goroutines
- Avoid goroutine leaks - ensure all goroutines can exit
- Use buffered channels when appropriate to prevent blocking

### Error Recovery
- Worker goroutines should not panic
- Use `recover()` in top-level goroutines if needed
- Log panics but continue processing
- Implement retry logic with backoff for transient errors

---

## Testing Strategy

### Unit Tests
- Test individual functions in isolation
- Use table-driven tests for similar test cases
- Mock dependencies using interfaces

### Integration Tests
- Test interaction between packages (engine + registry + state)
- Test complete flow lifecycle
- Test message routing through multiple nodes

### Performance Tests
- Benchmark message processing throughput
- Test memory usage under load
- Test concurrent flow deployment

**Example Test Structure:**
```go
func TestFlowEngine_Deploy(t *testing.T) {
    // Setup
    registry := registry.NewNodeRegistry()
    engine := NewFlowEngine(DefaultEngineConfig(), registry)
    
    // Create test flow
    flow := NewFlow("test-flow", "Test Flow")
    flow.Nodes["node1"] = &Node{Type: "inject", Config: map[string]interface{}{}}
    
    // Execute
    err := engine.Deploy(flow)
    
    // Verify
    assert.NoError(t, err)
    assert.Len(t, engine.GetAllFlows(), 1)
    
    // Cleanup
    engine.Undeploy(flow.ID)
}
```

---

## Code Review Checklist

When reviewing code in `internal/`:

- [ ] No circular dependencies between packages
- [ ] Proper error handling with context
- [ ] Goroutines are properly managed (WaitGroup, context)
- [ ] Mutex usage follows Go conventions
- [ ] Channel operations won't deadlock
- [ ] Memory usage is bounded
- [ ] Tests cover edge cases
- [ ] API contracts are maintained
- [ ] Logging is informative but not excessive

---

## Common Pitfalls to Avoid

1. **Deadlocks**: Always acquire mutexes in the same order
2. **Race Conditions**: Use mutexes or channels, not both for the same data
3. **Goroutine Leaks**: Ensure all goroutines have an exit path
4. **Memory Leaks**: Unregister listeners, close channels when done
5. **Blocked Channels**: Use select with default for non-blocking sends
6. **Unbounded Buffers**: Always limit channel and slice sizes

---

## Debugging Tips

### Logging
- Use `log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)` for timestamps
- Include relevant IDs (flowID, nodeID, messageID) in log messages
- Log at appropriate levels (debug, info, warn, error)

### Tracing
- Use `context.Context` to propagate request IDs
- Add tracing headers to WebSocket messages for debugging
- Consider using OpenTelemetry for production tracing

### Testing
- Use `-race` flag for Go tests to detect race conditions
- Use `go test -coverprofile=coverage.out` for coverage analysis
- Use `pprof` for performance profiling

---

*Last updated: 2026-06-21*
*Overrides: None (extends root AGENTS.md)*
