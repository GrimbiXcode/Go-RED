package registry

import (
    "sync"
    "time"
)

// NodeErrorEvent describes a node execution failure, published on a flow's
// EventBus so that Catch-style nodes (docs/NODE_PALETTE_PLAN.md, Phase 1)
// can react to errors from other nodes without a normal wire connection.
// Payload is the message payload the failing node was processing (not the
// full engine message - this package cannot depend on internal/engine
// without an import cycle, since engine already depends on registry for
// NodeExecutor and friends).
type NodeErrorEvent struct {
    FlowID    string
    NodeID    string
    NodeType  string
    Err       error
    Payload   map[string]interface{}
    Timestamp time.Time
}

// NodeStatusEvent describes a node-reported status change (e.g. "connected",
// "disconnected", a progress message), published on a flow's EventBus so
// that Status-style nodes can react to it. Status is free-form and defined
// by the reporting node; the engine does not interpret it.
type NodeStatusEvent struct {
    FlowID    string
    NodeID    string
    NodeType  string
    Status    string
    Detail    string
    Timestamp time.Time
}

// NodeCompleteEvent describes a node finishing a message successfully,
// published on a flow's EventBus so that Complete-style nodes can react to
// it. Payload is the message payload the node produced.
type NodeCompleteEvent struct {
    FlowID    string
    NodeID    string
    NodeType  string
    Payload   map[string]interface{}
    Timestamp time.Time
}

// EventBus is a simple, thread-safe pub/sub for flow-wide error, status, and
// completion events. Each active flow owns one. Handlers are called
// synchronously, in registration order, on the goroutine that published the
// event; handlers must not block for long or call back into the engine in a
// way that could deadlock (e.g. undeploying the same flow).
type EventBus struct {
    mu               sync.RWMutex
    errorHandlers    []func(NodeErrorEvent)
    statusHandlers   []func(NodeStatusEvent)
    completeHandlers []func(NodeCompleteEvent)
}

// NewEventBus creates an empty EventBus.
func NewEventBus() *EventBus {
    return &EventBus{}
}

// OnError registers a handler invoked for every published NodeErrorEvent.
func (b *EventBus) OnError(handler func(NodeErrorEvent)) {
    b.mu.Lock()
    defer b.mu.Unlock()
    b.errorHandlers = append(b.errorHandlers, handler)
}

// OnStatus registers a handler invoked for every published NodeStatusEvent.
func (b *EventBus) OnStatus(handler func(NodeStatusEvent)) {
    b.mu.Lock()
    defer b.mu.Unlock()
    b.statusHandlers = append(b.statusHandlers, handler)
}

// OnComplete registers a handler invoked for every published NodeCompleteEvent.
func (b *EventBus) OnComplete(handler func(NodeCompleteEvent)) {
    b.mu.Lock()
    defer b.mu.Unlock()
    b.completeHandlers = append(b.completeHandlers, handler)
}

// PublishError notifies all registered error handlers.
func (b *EventBus) PublishError(evt NodeErrorEvent) {
    b.mu.RLock()
    handlers := make([]func(NodeErrorEvent), len(b.errorHandlers))
    copy(handlers, b.errorHandlers)
    b.mu.RUnlock()

    for _, h := range handlers {
        h(evt)
    }
}

// PublishStatus notifies all registered status handlers.
func (b *EventBus) PublishStatus(evt NodeStatusEvent) {
    b.mu.RLock()
    handlers := make([]func(NodeStatusEvent), len(b.statusHandlers))
    copy(handlers, b.statusHandlers)
    b.mu.RUnlock()

    for _, h := range handlers {
        h(evt)
    }
}

// PublishComplete notifies all registered complete handlers.
func (b *EventBus) PublishComplete(evt NodeCompleteEvent) {
    b.mu.RLock()
    handlers := make([]func(NodeCompleteEvent), len(b.completeHandlers))
    copy(handlers, b.completeHandlers)
    b.mu.RUnlock()

    for _, h := range handlers {
        h(evt)
    }
}
