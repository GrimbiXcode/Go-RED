# Go-RED Flow Engine Guidelines

This file contains **engine-specific** guidelines that apply when working with the flow execution engine in `internal/engine/`.

---

## Package Overview

The `engine/` package is the **core** of Go-RED, responsible for:

1. **Flow Lifecycle Management**: Create, Deploy, Undeploy, Delete flows
2. **Message Processing**: Routing and execution of messages through node networks
3. **Concurrency Control**: Worker pools, goroutine management
4. **State Tracking**: Flow status, message logging, debugging

---

## Key Components

### Type Hierarchy

```
FlowEngine (Singleton-like)
├── flows: map[string]*ActiveFlow
├── registry: *NodeRegistry
├── msgChan: chan Message (main message queue)
├── wg: sync.WaitGroup (goroutine tracking)
├── ctx: context.Context (lifetime management)
└── messageLog: []Message (debugging)

ActiveFlow (Per-deployed-flow)
├── Flow: *Flow (definition)
├── Status: FlowStatus
├── msgChan: chan Message (flow-specific queue)
├── nodeExecutors: map[string]NodeExecutor
├── ctx: context.Context (flow lifetime)
└── wg: sync.WaitGroup

Flow (Definition)
├── ID, Name, Description
├── Nodes: map[string]*Node
├── Connections: []NodeConnection
├── Config: FlowConfig
├── Status: FlowStatus
├── CreatedAt, UpdatedAt: time.Time
└── Version: string

Message (Data unit)
├── ID: string
├── FlowID: string
├── Payload: map[string]interface{}
├── Path: []string (node traversal history)
├── Context: context.Context
├── Timestamp: time.Time
└── Metadata: map[string]interface{}
```

---

## Core Principles

### 1. Message-Driven Architecture
- **All processing happens via Message passing**
- **Nodes are stateless** - they receive a message, process it, emit new messages
- **Flows are directed graphs** - messages follow connections between nodes
- **Asynchronous by default** - non-blocking message processing

### 2. Concurrency Model
- **Worker Pool Pattern**: Fixed number of goroutines process messages
- **Fan-out**: One message can be sent to multiple nodes
- **Parallel Execution**: Multiple nodes in a flow can execute concurrently
- **Ordered Processing**: Messages from the same source maintain order

### 3. Error Handling
- **Continue on Error**: One node failure doesn't stop the entire flow
- **Retry Mechanism**: Configurable retry policy with backoff
- **Error Output**: Errors can be routed to error handling nodes
- **Logging**: All errors are logged with context

---

## Development Guidelines

### Adding New Flow Features

1. **Add to Flow type** (`flow.go`):
   - New fields for flow-level configuration
   - Update `NewFlow()` constructor
   - Update `Validate()` method

2. **Add to FlowConfig** (`flow.go`):
   - New configuration options
   - Set sensible defaults

3. **Update FlowEngine** (`engine.go`):
   - Modify `Deploy()` to handle new configuration
   - Update `processMessage()` for new message types
   - Add new public methods if needed

4. **Add Tests** (`*_test.go`):
   - Unit tests for new functionality
   - Integration tests for flow lifecycle
   - Performance tests if applicable

### Modifying Message Processing

**Before changing message processing logic:**

1. Understand the current flow:
   ```
   SubmitMessage() → msgChan → worker() → processMessage()
                                        ↓
                                findTargetNodes() → nodeExecutors
                                        ↓
                                Execute() → submitMessage() (recursive)
   ```

2. Consider thread safety:
   - Which mutexes protect which data?
   - Are you accessing shared state?
   - Could this cause deadlocks?

3. Consider performance:
   - Are you adding blocking operations?
   - Could this cause memory growth?
   - Are channels properly sized?

4. Update tests:
   - Existing tests may break
   - Add new test cases for changed behavior

### Adding Flow Statuses

1. **Define new status** in `flow.go`:
   ```go
   type FlowStatus string
   const (
       FlowStatusInactive   FlowStatus = "inactive"
       FlowStatusActive     FlowStatus = "active"
       FlowStatusError      FlowStatus = "error"
       FlowStatusDeploying  FlowStatus = "deploying"
       FlowStatusUndeploying FlowStatus = "undeploying"
       // Add new status here
       FlowStatusPaused     FlowStatus = "paused"
   )
   ```

2. **Update state transitions**:
   - When can a flow enter this state?
   - When does it exit?
   - What triggers the transition?

3. **Update frontend conversion** in `cmd/go-red/main.go`:
   ```go
   func convertFlowStatusAPI(status engine.FlowStatus) string {
       switch status {
       case engine.FlowStatusPaused:
           return "paused"
       // ...
       }
   }
   ```

---

## Message Processing Deep Dive

### Message Lifecycle

```
1. Creation
   ├── NewMessage() - from scratch
   ├── NewMessageWithContext() - with parent context
   └── Clone() - copy existing message

2. Submission
   ├── SubmitMessage() - public API
   └── submitMessage() - internal (adds ID, handles full channel)

3. Routing
   ├── worker() - receives from msgChan
   └── processMessage() - main processing logic

4. Target Selection
   ├── findTargetNodes() - determine which nodes receive the message
   ├── findRootNodes() - for new messages (empty path)
   └── findConnectedNodes() - for subsequent messages

5. Node Execution
   ├── executor.Execute(ctx, input) - node processes message
   └── output sent via submitMessage() (recursive)

6. Completion
   └── Message added to messageLog for debugging
```

### Message Path Tracking
- **Path**: Array of node IDs the message has traversed
- **Purpose**: Prevent infinite loops, debugging, flow visualization
- **Usage**: `msg.AddToPath(nodeID)` before node execution
- **Loop Prevention**: Check if node is already in path (future enhancement)

---

## Configuration

### EngineConfig

```go
type EngineConfig struct {
    WorkerPoolSize    int           // Number of message processing goroutines
    MessageBufferSize int           // Size of message channel buffers
    DefaultTimeout    time.Duration // Default timeout for node execution
    MaxRetries        int           // Maximum retry attempts for failed messages
    RetryBackoff      time.Duration // Backoff duration between retries
}
```

**Tuning Guidelines:**
- `WorkerPoolSize`: Start with CPU cores × 2, adjust based on workload
- `MessageBufferSize`: Should be large enough to handle bursts, but not unbounded
- `DefaultTimeout`: Most nodes should complete within this time
- `MaxRetries`: 3 is a good default for transient errors
- `RetryBackoff`: Exponential backoff recommended

---

## Testing Engine Code

### Test Categories

1. **Unit Tests** - Individual methods
2. **Flow Tests** - Complete flow execution
3. **Concurrency Tests** - Parallel message processing
4. **Performance Tests** - Throughput and latency
5. **Error Tests** - Error handling and recovery

### Test Helpers

```go
// Create a test engine
func setupTestEngine() *FlowEngine {
    registry := registry.NewNodeRegistry()
    // Register test nodes
    registry.RegisterNodeType("test-input", metadata, factory)
    
    engine := NewFlowEngine(EngineConfig{
        WorkerPoolSize:    10,
        MessageBufferSize: 100,
        DefaultTimeout:    time.Second,
    }, registry)
    
    engine.SetStateManager(&MockStateManager{})
    return engine
}

// Create a test flow
func createTestFlow() *Flow {
    flow := NewFlow("test-flow", "Test Flow")
    flow.Nodes["input"] = &Node{
        ID:   "input",
        Type: "test-input",
        X:    0, Y: 0,
    }
    flow.Nodes["output"] = &Node{
        ID:   "output",
        Type: "debug",
        X:    100, Y: 0,
    }
    flow.Connections = append(flow.Connections, NodeConnection{
        ID:          "conn-1",
        SourceNode:  "input",
        TargetNode:  "output",
    })
    return flow
}
```

### Important Test Cases

```go
// Test flow deployment and message processing
func TestFlowEngine_EndToEnd(t *testing.T) {
    engine := setupTestEngine()
    engine.Start()
    defer engine.Stop()
    
    flow := createTestFlow()
    require.NoError(t, engine.Deploy(flow))
    defer engine.Undeploy(flow.ID)
    
    // Inject message
    err := engine.InjectMessage(flow.ID, "input", map[string]interface{}{
        "payload": "test",
    })
    require.NoError(t, err)
    
    // Wait for processing
    time.Sleep(100 * time.Millisecond)
    
    // Verify message was processed
    messages := engine.GetMessageLogForFlow(flow.ID)
    assert.Len(t, messages, 1)
}

// Test concurrent message processing
func TestFlowEngine_ConcurrentMessages(t *testing.T) {
    engine := setupTestEngine()
    engine.Start()
    defer engine.Stop()
    
    flow := createTestFlow()
    require.NoError(t, engine.Deploy(flow))
    defer engine.Undeploy(flow.ID)
    
    // Send many messages concurrently
    var wg sync.WaitGroup
    for i := 0; i < 1000; i++ {
        wg.Add(1)
        go func(idx int) {
            defer wg.Done()
            engine.InjectMessage(flow.ID, "input", map[string]interface{}{
                "index": idx,
            })
        }(i)
    }
    wg.Wait()
    
    // All messages should be processed
    messages := engine.GetMessageLogForFlow(flow.ID)
    assert.Len(t, messages, 1000)
}
```

---

## Performance Optimization

### Hot Paths

1. **Message Submission**: `submitMessage()` is called for every message
2. **Target Finding**: `findTargetNodes()` is called for every message
3. **Node Execution**: `executor.Execute()` is the actual work
4. **Message Logging**: `AddMessageToLog()` adds overhead

### Optimization Techniques

1. **Channel Buffering**: Appropriate buffer sizes prevent blocking
2. **Worker Pool**: Right-size the pool for your workload
3. **Batching**: Consider batching small messages (future enhancement)
4. **Caching**: Cache flow structures that don't change often
5. **Circular Buffer**: Use circular buffer for message log (current implementation)

### Benchmarking

```go
func BenchmarkMessageProcessing(b *testing.B) {
    engine := setupTestEngine()
    engine.Start()
    defer engine.Stop()
    
    flow := createTestFlow()
    engine.Deploy(flow)
    defer engine.Undeploy(flow.ID)
    
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        engine.InjectMessage(flow.ID, "input", map[string]interface{}{
            "data": i,
        })
    }
}

func BenchmarkConcurrentMessageProcessing(b *testing.B) {
    engine := setupTestEngine()
    engine.config.WorkerPoolSize = 100
    engine.Start()
    defer engine.Stop()
    
    flow := createTestFlow()
    engine.Deploy(flow)
    defer engine.Undeploy(flow.ID)
    
    b.ResetTimer()
    b.RunParallel(func(pb *testing.PB) {
        for i := 0; pb.Next(); i++ {
            engine.InjectMessage(flow.ID, "input", map[string]interface{}{
                "data": i,
            })
        }
    })
}
```

---

## Debugging Engine Issues

### Common Problems

#### 1. Messages Not Processing
- **Check**: Is the engine started? (`engine.Start()` called)
- **Check**: Are workers running? (log should show "FlowEngine started")
- **Check**: Is the flow deployed? (`engine.GetFlow(flowID)`)
- **Check**: Are node executors initialized? (in `ActiveFlow.nodeExecutors`)

#### 2. Goroutine Leaks
- **Check**: Are all `wg.Add(1)` matched with `wg.Done()`?
- **Check**: Are contexts being cancelled properly?
- **Check**: Use `runtime.NumGoroutine()` to track goroutine count
- **Tool**: `go test -race` to detect race conditions

#### 3. Deadlocks
- **Check**: Mutex acquisition order
- **Check**: Are you holding a mutex while sending to a channel?
- **Tool**: Use `go vet` and manual code review
- **Symptom**: Test hangs indefinitely

#### 4. Message Loss
- **Check**: Channel buffer sizes (`MessageBufferSize`)
- **Check**: Are channels full? (log shows "Message channel full")
- **Check**: Is `submitMessage()` dropping messages?
- **Fix**: Increase buffer size or add backpressure

### Debug Logging

Enable verbose logging:
```go
// In main.go or test setup
log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile | log.Lmicroseconds)
log.SetOutput(os.Stdout)

// Or add debug logging to specific functions
func (e *FlowEngine) processMessage(msg Message) {
    log.Printf("DEBUG: Processing message %s for flow %s, path: %v", 
        msg.ID, msg.FlowID, msg.Path)
    // ... rest of function
}
```

---

## WebSocket Integration

The engine communicates with the frontend via WebSocket through the hub in `cmd/go-red/websocket/`.

### Message Flow to Frontend
```
FlowEngine events → Hub.Broadcast() → All connected clients
```

### Key Integration Points
- Flow CRUD operations trigger broadcasts
- Flow status changes are pushed to clients
- Message logging can be streamed (future enhancement)

### Frontend-Backend Contract
- Follow existing patterns in `cmd/go-red/main.go`
- REST API endpoints use `/api/*` prefix
- WebSocket endpoint is `/ws`
- Message formats must match TypeScript types in `web/src/types/`

---

## Future Enhancements

### Planned Features
- Message prioritization (QOS levels)
- Flow versioning and rollback
- Hot reloading of node types
- Dynamic worker pool resizing
- Message persistence for recovery
- Distributed flow execution

### Architecture Decisions
- **Single Engine**: Current design uses one engine instance
- **Per-Flow Isolation**: Each flow has its own goroutines and channels
- **In-Memory State**: Flows are kept in memory when active
- **File Persistence**: Default state manager uses JSON files

---

## Checklist for Engine Changes

Before committing changes to the engine:

- [ ] All existing tests pass (`go test ./internal/engine/...`)
- [ ] No race conditions detected (`go test -race ./internal/engine/...`)
- [ ] Memory usage is bounded (no unbounded growth)
- [ ] Goroutines are properly cleaned up
- [ ] Mutex usage is correct (same order, no nested locking)
- [ ] Channel operations won't deadlock
- [ ] Error handling is comprehensive
- [ ] Logging provides useful debugging info
- [ ] Frontend integration still works (manual test)
- [ ] Performance hasn't degraded (benchmark if changed hot paths)

---

*Last updated: 2026-06-21*
*Overrides: None (extends internal/AGENTS.md and root AGENTS.md)*
