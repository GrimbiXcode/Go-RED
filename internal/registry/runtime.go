package registry

import (
    "context"
    "time"
)

// NodeRuntime gives a running node access to engine-level services that
// don't fit the input->output NodeExecutor.Execute contract: flow/global
// key-value context, error/status/complete event reporting and
// subscription (for Catch/Status/Complete nodes, docs/NODE_PALETTE_PLAN.md
// Phase 1), and submitting a message directly to another node in the same
// flow (for Link nodes). A node obtains it via RuntimeFromContext(ctx)
// inside Execute, ExecuteMulti, or Start - the engine embeds it into the
// context.Context it passes to all three.
type NodeRuntime struct {
    FlowID        string
    NodeID        string
    NodeType      string
    FlowContext   *ContextStore
    GlobalContext *ContextStore

    events  *EventBus
    submit  func(nodeID string, payload map[string]interface{})
    getNode func(nodeID string) (NodeExecutor, bool)
}

// NewNodeRuntime constructs a NodeRuntime. events, submit, and getNode may
// be nil (a runtime built for a context where no engine is backing it, e.g.
// a unit test); every method on NodeRuntime tolerates that.
func NewNodeRuntime(flowID, nodeID, nodeType string, flowContext, globalContext *ContextStore, events *EventBus, submit func(nodeID string, payload map[string]interface{}), getNode func(nodeID string) (NodeExecutor, bool)) *NodeRuntime {
    return &NodeRuntime{
        FlowID:        flowID,
        NodeID:        nodeID,
        NodeType:      nodeType,
        FlowContext:   flowContext,
        GlobalContext: globalContext,
        events:        events,
        submit:        submit,
        getNode:       getNode,
    }
}

// ReportStatus publishes a NodeStatusEvent on behalf of this node. Safe to
// call even if no Status node is listening.
func (r *NodeRuntime) ReportStatus(status, detail string) {
    if r == nil || r.events == nil {
        return
    }
    r.events.PublishStatus(NodeStatusEvent{
        FlowID:    r.FlowID,
        NodeID:    r.NodeID,
        NodeType:  r.NodeType,
        Status:    status,
        Detail:    detail,
        Timestamp: time.Now().UTC(),
    })
}

// ReportError publishes a NodeErrorEvent on behalf of this node, for
// non-fatal errors a node wants to surface to Catch nodes without failing
// the message it is currently processing (Execute/ExecuteMulti errors are
// already published automatically by the engine).
func (r *NodeRuntime) ReportError(err error) {
    if r == nil || r.events == nil || err == nil {
        return
    }
    r.events.PublishError(NodeErrorEvent{
        FlowID:    r.FlowID,
        NodeID:    r.NodeID,
        NodeType:  r.NodeType,
        Err:       err,
        Timestamp: time.Now().UTC(),
    })
}

// OnError subscribes handler to every NodeErrorEvent published on this
// node's flow, for the lifetime of the EventBus (typically the flow's
// deployment). Used by Catch-style nodes, normally from within
// EmittingNode.Start. A no-op if there is no backing EventBus.
func (r *NodeRuntime) OnError(handler func(NodeErrorEvent)) {
    if r == nil || r.events == nil {
        return
    }
    r.events.OnError(handler)
}

// OnStatus subscribes handler to every NodeStatusEvent published on this
// node's flow. Used by Status-style nodes. A no-op if there is no backing
// EventBus.
func (r *NodeRuntime) OnStatus(handler func(NodeStatusEvent)) {
    if r == nil || r.events == nil {
        return
    }
    r.events.OnStatus(handler)
}

// OnComplete subscribes handler to every NodeCompleteEvent published on
// this node's flow. Used by Complete-style nodes. A no-op if there is no
// backing EventBus.
func (r *NodeRuntime) OnComplete(handler func(NodeCompleteEvent)) {
    if r == nil || r.events == nil {
        return
    }
    r.events.OnComplete(handler)
}

// SubmitToNode delivers payload directly to nodeID's output, as if nodeID
// had just produced it, bypassing the normal input->output wire of nodeID
// itself (used by Link nodes to jump to a Link In node elsewhere in the
// same flow without a drawn wire). A no-op if there is no backing engine or
// nodeID does not exist in this flow.
func (r *NodeRuntime) SubmitToNode(nodeID string, payload map[string]interface{}) {
    if r == nil || r.submit == nil {
        return
    }
    r.submit(nodeID, payload)
}

// GetNode returns the live NodeExecutor instance for nodeID within this
// node's flow - used to reach a shared config node (e.g. an mqtt-broker's
// connection, a tls-config's certificate) referenced by ID from a node's own
// configuration (see docs/NODE_PALETTE_PLAN.md, Phase 6's config-node
// concept). Resolved lazily, at Execute/ExecuteMulti/Start call time rather
// than at SetConfig time, since Deploy initializes a flow's nodes by
// iterating a Go map (unordered) - a config node is not guaranteed to exist
// yet when a node referencing it is constructed, but every node exists by
// the time any Execute/Start call happens. A no-op returning (nil, false) if
// there is no backing engine or nodeID does not exist in this flow.
func (r *NodeRuntime) GetNode(nodeID string) (NodeExecutor, bool) {
    if r == nil || r.getNode == nil {
        return nil, false
    }
    return r.getNode(nodeID)
}

type runtimeContextKey struct{}

// WithRuntime returns a copy of ctx carrying rt, retrievable via
// RuntimeFromContext. Called by the engine when constructing the context it
// passes into a node's Execute/ExecuteMulti/Start.
func WithRuntime(ctx context.Context, rt *NodeRuntime) context.Context {
    return context.WithValue(ctx, runtimeContextKey{}, rt)
}

// RuntimeFromContext extracts the NodeRuntime embedded by the engine into a
// node's execution context, if any. Nodes running outside the engine (e.g.
// direct unit tests calling Execute with a plain context.Background()) will
// get ok == false and should treat runtime services as unavailable.
func RuntimeFromContext(ctx context.Context) (*NodeRuntime, bool) {
    rt, ok := ctx.Value(runtimeContextKey{}).(*NodeRuntime)
    return rt, ok
}
