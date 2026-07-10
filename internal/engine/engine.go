package engine

import (
    "context"
    "errors"
    "log"
    "sync"
    "time"

    "github.com/GrimbiXcode/Go-RED/internal/registry"
    "github.com/google/uuid"
)

// eagerReadyTimeout bounds how long Deploy waits for a single
// registry.EagerlyReadyNode to call registry.SignalReady before giving up
// on it and proceeding anyway (see startEmittingNodes) - generous enough
// for the synchronous, non-blocking setup (an EventBus subscription) this
// is meant for, but bounded so a node that never signals (a bug, or a
// node that shouldn't have implemented the marker) can't hang Deploy.
const eagerReadyTimeout = 2 * time.Second

// EngineConfig contains configuration options for the FlowEngine.
type EngineConfig struct {
    // WorkerPoolSize is the number of worker goroutines to spawn.
    WorkerPoolSize int
    
    // MessageBufferSize is the size of the message channel buffer.
    MessageBufferSize int
    
    // DefaultTimeout is the default timeout for node execution.
    DefaultTimeout time.Duration
    
    // MaxRetries is the default maximum number of retries for failed messages.
    MaxRetries int
    
    // RetryBackoff is the default backoff duration between retries.
    RetryBackoff time.Duration
}

// DefaultEngineConfig returns a default EngineConfig.
func DefaultEngineConfig() EngineConfig {
    return EngineConfig{
        WorkerPoolSize:    100,
        MessageBufferSize: 1000,
        DefaultTimeout:    30 * time.Second,
        MaxRetries:        3,
        RetryBackoff:      1 * time.Second,
    }
}

// StateManager interface for persisting flows.
// This allows for different storage backends (file system, database, etc.)
type StateManager interface {
    SaveFlow(flow *Flow) error
    LoadFlow(flowID string) (*Flow, error)
    LoadAllFlows() ([]*Flow, error)
    DeleteFlow(flowID string) error
}

// FlowEngine is the core component that orchestrates flow execution.
// It manages active flows, processes messages, and coordinates node execution.
type FlowEngine struct {
    // flows contains all active flows.
    flows map[string]*ActiveFlow
    
    // registry contains all available node types.
    registry *registry.NodeRegistry
    
    // msgChan is the channel for incoming messages.
    msgChan chan Message
    
    // wg is used to wait for all goroutines to complete.
    wg sync.WaitGroup
    
    // ctx and cancel are used for graceful shutdown.
    ctx    context.Context
    cancel context.CancelFunc

    // stopped prevents Stop from being called multiple times.
    stopped bool
    stopMu  sync.Mutex

    // config contains the engine configuration.
    config EngineConfig
    
    // mu protects access to flows map.
    mu sync.RWMutex
    
    // stateManager is used for persisting flows.
    stateManager StateManager
    
    // messageIDCounter is used to generate unique message IDs.
    messageIDCounter uint64
    // messageIDMu protects messageIDCounter.
    messageIDMu sync.Mutex

    // messageLog stores recent messages for debugging and monitoring.
    // This is thread-safe and has a maximum size to prevent memory issues.
    messageLog    []Message
    messageLogMu  sync.RWMutex
    maxMessageLog int

    // globalStore is the "global" context (flow.get/set's global-scoped
    // counterpart), shared by every flow deployed on this engine.
    globalStore *registry.ContextStore
}

// ActiveFlow represents an active (deployed) flow.
type ActiveFlow struct {
    // flow is the flow definition.
    Flow *Flow
    
    // status is the current status of the flow.
    Status FlowStatus
    
    // msgChan is the channel for messages specific to this flow.
    msgChan chan Message
    
    // wg is used to wait for all flow goroutines to complete.
    wg sync.WaitGroup
    
    // ctx and cancel are used for flow-specific cancellation.
    ctx    context.Context
    cancel context.CancelFunc
    
    // nodeExecutors contains initialized node executors for this flow.
    nodeExecutors map[string]registry.NodeExecutor

    // ContextStore is this flow's private key-value context (flow.get/set).
    ContextStore *registry.ContextStore

    // EventBus carries error/status/complete events for this flow's nodes,
    // consumed by Catch/Status/Complete-style nodes
    // (docs/NODE_PALETTE_PLAN.md, Phase 1).
    EventBus *registry.EventBus
}

// NewFlowEngine creates a new FlowEngine with the given configuration and registry.
func NewFlowEngine(config EngineConfig, nodeRegistry *registry.NodeRegistry) *FlowEngine {
    ctx, cancel := context.WithCancel(context.Background())

    return &FlowEngine{
        flows:       make(map[string]*ActiveFlow),
        registry:    nodeRegistry,
        msgChan:     make(chan Message, config.MessageBufferSize),
        ctx:         ctx,
        cancel:      cancel,
        config:      config,
        messageIDCounter: 0,
        messageLog:   make([]Message, 0),
        maxMessageLog: 1000, // Store last 1000 messages
        globalStore:  registry.NewContextStore(),
    }
}

// GlobalContext returns the engine-wide context store shared by every flow
// (the global.get/set counterpart to each flow's own ContextStore).
func (e *FlowEngine) GlobalContext() *registry.ContextStore {
    return e.globalStore
}

// SetStateManager sets the state manager for the engine.
func (e *FlowEngine) SetStateManager(sm StateManager) {
    e.stateManager = sm
}

// GetStateManager returns the state manager for the engine.
func (e *FlowEngine) GetStateManager() StateManager {
    return e.stateManager
}

// Start starts the FlowEngine.
// This spawns the worker pool and starts processing messages.
func (e *FlowEngine) Start() error {
    log.Println("Starting FlowEngine...")
    
    // Start worker pool
    for i := 0; i < e.config.WorkerPoolSize; i++ {
        e.wg.Add(1)
        go e.worker()
    }
    
    // Start message processor
    e.wg.Add(1)
    go e.processMessages()
    
    log.Println("FlowEngine started")
    return nil
}

// Stop stops the FlowEngine and waits for all goroutines to complete.
func (e *FlowEngine) Stop() error {
    e.stopMu.Lock()
    defer e.stopMu.Unlock()
    
    if e.stopped {
        return nil
    }
    e.stopped = true
    
    log.Println("Stopping FlowEngine...")
    
    // Cancel the main context
    e.cancel()
    
    // Close the message channel
    close(e.msgChan)
    
    // Wait for all goroutines to complete
    e.wg.Wait()
    
    log.Println("FlowEngine stopped")
    return nil
}

// worker processes messages from the message channel.
func (e *FlowEngine) worker() {
    defer e.wg.Done()
    
    for msg := range e.msgChan {
        e.wg.Add(1)
        go func(m Message) {
            defer e.wg.Done()
            e.processMessage(m)
        }(msg)
    }
}

// processMessages processes messages from the main message channel.
func (e *FlowEngine) processMessages() {
    defer e.wg.Done()
    
    for {
        select {
        case msg := <-e.msgChan:
            e.wg.Add(1)
            go func(m Message) {
                defer e.wg.Done()
                e.processMessage(m)
            }(msg)
        case <-e.ctx.Done():
            return
        }
    }
}

// processMessage processes a single message.
func (e *FlowEngine) processMessage(msg Message) {
    // Add message to log for debugging
    e.AddMessageToLog(msg)

    log.Printf("[DEBUG] Processing message %s for flow %s, path: %v", msg.ID, msg.FlowID, msg.Path)

    e.mu.RLock()
    activeFlow, exists := e.flows[msg.FlowID]
    e.mu.RUnlock()
    
    if !exists {
        log.Printf("Flow %s not found, dropping message", msg.FlowID)
        return
    }
    
    // Find the target nodes for this message
    targetNodes := e.findTargetNodes(activeFlow.Flow, msg)

    // Process each target node
    for _, nodeID := range targetNodes {
        // Get the node executor
        executor, exists := activeFlow.nodeExecutors[nodeID]
        if !exists {
            log.Printf("Node %s not found in flow %s", nodeID, msg.FlowID)
            continue
        }

        var nodeType string
        if node, ok := activeFlow.Flow.Nodes[nodeID]; ok {
            nodeType = node.Type
        }

        // Create a new message context with timeout, and embed the
        // NodeRuntime (flow/global context store, error/status reporting)
        // the node can retrieve via registry.RuntimeFromContext.
        ctx, cancel := context.WithTimeout(msg.Context, e.config.DefaultTimeout)
        ctx = registry.WithRuntime(ctx, e.newNodeRuntime(activeFlow, nodeID, nodeType))

        // Execute the node in a goroutine
        go func(nodeID, nodeType string, exec registry.NodeExecutor, nodeCtx context.Context) {
            defer cancel()

            // Add node to path
            newMsg := msg.Clone()
            newMsg.AddToPath(nodeID)
            newMsg.Context = nodeCtx

            e.executeNode(activeFlow, nodeID, nodeType, exec, nodeCtx, newMsg)
        }(nodeID, nodeType, executor, ctx)
    }
}

// executeNode runs a single node against msg (whose Path already includes
// nodeID) and routes its output(s) onward. Nodes implementing
// registry.MultiOutputExecutor get explicit per-port routing (one submitted
// message per non-nil port); plain registry.NodeExecutor nodes keep the
// original behavior of a single implicit output followed by every outgoing
// connection regardless of port.
func (e *FlowEngine) executeNode(activeFlow *ActiveFlow, nodeID, nodeType string, exec registry.NodeExecutor, nodeCtx context.Context, msg Message) {
    if multi, ok := exec.(registry.MultiOutputExecutor); ok {
        outputs, err := multi.ExecuteMulti(nodeCtx, msg.Payload)
        if err != nil {
            e.handleNodeError(activeFlow, nodeID, nodeType, msg, err)
            return
        }

        sent := 0
        for port, payload := range outputs {
            if payload == nil {
                continue
            }
            portMsg := msg.Clone()
            portMsg.Payload = payload
            portMsg.OutputPort = port
            e.submitMessage(portMsg)
            e.handleNodeComplete(activeFlow, nodeID, nodeType, payload)
            sent++
        }
        if sent == 0 {
            // A MultiOutputExecutor that sent on no port (e.g. Link Out, or
            // a future RBE suppressing a duplicate) still finished
            // successfully - Complete-style nodes care about that too.
            e.handleNodeComplete(activeFlow, nodeID, nodeType, msg.Payload)
        }
        return
    }

    output, err := exec.Execute(nodeCtx, msg.Payload)
    if err != nil {
        e.handleNodeError(activeFlow, nodeID, nodeType, msg, err)
        return
    }

    msg.Payload = output
    msg.OutputPort = ""
    e.submitMessage(msg)
    e.handleNodeComplete(activeFlow, nodeID, nodeType, output)
}

// handleNodeComplete publishes a NodeCompleteEvent if the flow has an
// EventBus, so Complete-style nodes (docs/NODE_PALETTE_PLAN.md, Phase 1)
// can react to a node finishing successfully.
func (e *FlowEngine) handleNodeComplete(activeFlow *ActiveFlow, nodeID, nodeType string, payload map[string]interface{}) {
    if activeFlow.EventBus == nil {
        return
    }
    activeFlow.EventBus.PublishComplete(registry.NodeCompleteEvent{
        FlowID:    activeFlow.Flow.ID,
        NodeID:    nodeID,
        NodeType:  nodeType,
        Payload:   payload,
        Timestamp: time.Now().UTC(),
    })
}

// handleNodeError logs a node execution failure and, if the flow has an
// EventBus, publishes it so Catch-style nodes (docs/NODE_PALETTE_PLAN.md,
// Phase 1) can react to it.
func (e *FlowEngine) handleNodeError(activeFlow *ActiveFlow, nodeID, nodeType string, msg Message, err error) {
    log.Printf("Node %s in flow %s failed: %v", nodeID, msg.FlowID, err)

    if activeFlow.EventBus == nil {
        return
    }
    activeFlow.EventBus.PublishError(registry.NodeErrorEvent{
        FlowID:    msg.FlowID,
        NodeID:    nodeID,
        NodeType:  nodeType,
        Err:       err,
        Payload:   msg.Payload,
        Timestamp: time.Now().UTC(),
    })
}

// findTargetNodes finds all nodes that should receive the message.
// This is based on the connections in the flow.
func (e *FlowEngine) findTargetNodes(flow *Flow, msg Message) []string {
    // If this is a new message (empty path), find nodes with no incoming connections
    if len(msg.Path) == 0 {
        return e.findRootNodes(flow)
    }

    // Otherwise, find nodes connected to the last node in the path
    lastNodeID := msg.Path[len(msg.Path)-1]
    return e.findConnectedNodes(flow, lastNodeID, msg.OutputPort)
}

// findRootNodes finds nodes that have no incoming connections (root nodes).
func (e *FlowEngine) findRootNodes(flow *Flow) []string {
    var rootNodes []string
    
    // Create a set of all target nodes
    targetNodes := make(map[string]bool)
    for _, conn := range flow.Connections {
        targetNodes[conn.TargetNode] = true
    }
    
    // Find nodes that are not targets
    for nodeID := range flow.Nodes {
        if !targetNodes[nodeID] {
            rootNodes = append(rootNodes, nodeID)
        }
    }
    
    return rootNodes
}

// findConnectedNodes finds nodes connected to the given node's outputs.
// When sourcePort is non-empty (a MultiOutputExecutor node reporting which
// port it sent on), only connections whose SourcePort matches it - or whose
// SourcePort is itself unset, for flows/tests that never set one - are
// followed. When sourcePort is empty (the default, single-output executor
// case), every outgoing connection is followed regardless of port, exactly
// like before per-port routing existed.
func (e *FlowEngine) findConnectedNodes(flow *Flow, nodeID string, sourcePort string) []string {
    var connectedNodes []string

    for _, conn := range flow.Connections {
        if conn.SourceNode != nodeID {
            continue
        }
        if sourcePort != "" && conn.SourcePort != "" && conn.SourcePort != sourcePort {
            continue
        }
        connectedNodes = append(connectedNodes, conn.TargetNode)
    }

    return connectedNodes
}

// submitMessage submits a message to the message channel.
func (e *FlowEngine) submitMessage(msg Message) {
    e.messageIDMu.Lock()
    e.messageIDCounter++
    msg.ID = "msg-" + string(rune(e.messageIDCounter))
    e.messageIDMu.Unlock()
    
    select {
    case e.msgChan <- msg:
        // Message submitted successfully
    default:
        // Message channel is full, drop the message
        log.Printf("Message channel full, dropping message %s", msg.ID)
    }
}

// SubmitMessage submits a message to the engine for processing.
// This is the public method for submitting messages.
func (e *FlowEngine) SubmitMessage(msg Message) {
    e.submitMessage(msg)
}

// Deploy deploys a flow, making it active and ready to process messages.
func (e *FlowEngine) Deploy(flow *Flow) error {
    e.mu.Lock()
    defer e.mu.Unlock()
    
    log.Printf("[ENGINE] Deploying flow %s with %d nodes and %d connections", flow.ID, len(flow.Nodes), len(flow.Connections))
    
    // Validate the flow
    if err := flow.Validate(); err != nil {
        log.Printf("[ENGINE] Flow %s validation failed: %v", flow.ID, err)
        return errors.New("invalid flow: " + err.Error())
    }
    log.Printf("[ENGINE] Flow %s validation passed", flow.ID)
    
    // Check if flow is already deployed
    if _, exists := e.flows[flow.ID]; exists {
        log.Printf("[ENGINE] Flow %s is already deployed", flow.ID)
        return errors.New("flow " + flow.ID + " is already deployed")
    }
    
    // Create active flow
    activeFlow := &ActiveFlow{
        Flow:         flow,
        Status:       FlowStatusActive,
        msgChan:      make(chan Message, e.config.MessageBufferSize),
        nodeExecutors: make(map[string]registry.NodeExecutor),
        ContextStore: registry.NewContextStore(),
        EventBus:     registry.NewEventBus(),
    }
    
    // Update flow status to active
    flow.Status = FlowStatusActive
    
    // Create context for this flow
    activeFlow.ctx, activeFlow.cancel = context.WithCancel(e.ctx)
    
    // Initialize all nodes in the flow
    log.Printf("[DEBUG] [ENGINE] Initializing %d nodes for flow %s", len(flow.Nodes), flow.ID)
    log.Printf("[ENGINE] Initializing %d nodes for flow %s", len(flow.Nodes), flow.ID)
    for nodeID, node := range flow.Nodes {
        log.Printf("[DEBUG] [ENGINE] Initializing node %s of type %s with config: %v", nodeID, node.Type, node.Config)
        log.Printf("[ENGINE] Initializing node %s of type %s", nodeID, node.Type)
        executor, err := e.registry.InitializeNode(node.Type, node.Config)
        if err != nil {
            // Release any resources already-initialized nodes in this flow
            // acquired (e.g. an earlier node opened a connection) before
            // bailing out, then cancel the flow context.
            closeNodeExecutors(activeFlow.nodeExecutors)
            activeFlow.cancel()
            log.Printf("[ENGINE] Failed to initialize node %s: %v", nodeID, err)
            return errors.New("failed to initialize node " + nodeID + ": " + err.Error())
        }
        activeFlow.nodeExecutors[nodeID] = executor
        log.Printf("[DEBUG] [ENGINE] Node %s initialized successfully", nodeID)
        log.Printf("[ENGINE] Node %s initialized successfully", nodeID)
    }

    // Start flow message processor
    e.wg.Add(1)
    go e.processFlowMessages(activeFlow)

    // Start nodes that originate their own messages (EmittingNode), e.g. a
    // TCP listener or file watcher, rather than only reacting to input.
    e.startEmittingNodes(activeFlow)

    // Add to active flows
    e.flows[flow.ID] = activeFlow

    log.Printf("[ENGINE] Flow %s deployed successfully with %d initialized nodes", flow.ID, len(activeFlow.nodeExecutors))
    return nil
}

// startEmittingNodes launches a goroutine per node implementing
// registry.EmittingNode, tracked by activeFlow.wg so Undeploy can wait for
// them to observe context cancellation and return before node resources are
// closed.
func (e *FlowEngine) startEmittingNodes(activeFlow *ActiveFlow) {
    // readyWG holds Deploy open until every registry.EagerlyReadyNode in
    // this flow has actually run the synchronous setup part of its Start
    // (typically an EventBus subscription) - see EagerlyReadyNode's doc
    // comment for why: without this, a message injected right after
    // Deploy returns could race ahead of a Catch/Status/Complete node's
    // subscription and have its event silently dropped. Nodes that don't
    // implement the marker aren't waited on at all, so they add no
    // latency to Deploy.
    var readyWG sync.WaitGroup

    for nodeID, executor := range activeFlow.nodeExecutors {
        emitter, ok := executor.(registry.EmittingNode)
        if !ok {
            continue
        }

        var nodeType string
        if node, ok := activeFlow.Flow.Nodes[nodeID]; ok {
            nodeType = node.Type
        }

        nodeCtx := registry.WithRuntime(activeFlow.ctx, e.newNodeRuntime(activeFlow, nodeID, nodeType))

        if _, ok := executor.(registry.EagerlyReadyNode); ok {
            readyCh := make(chan struct{})
            var once sync.Once
            nodeCtx = registry.WithReady(nodeCtx, func() { once.Do(func() { close(readyCh) }) })

            readyWG.Add(1)
            go func(nodeID string) {
                defer readyWG.Done()
                select {
                case <-readyCh:
                case <-time.After(eagerReadyTimeout):
                    log.Printf("[ENGINE] Node %s did not call SignalReady within %s - proceeding without waiting further", nodeID, eagerReadyTimeout)
                }
            }(nodeID)
        }

        activeFlow.wg.Add(1)
        go func(nodeID string, emitter registry.EmittingNode, nodeCtx context.Context) {
            defer activeFlow.wg.Done()

            emit := func(payload map[string]interface{}) {
                msg := NewMessageWithContext(nodeCtx, payload, activeFlow.Flow.ID)
                msg.AddToPath(nodeID)
                e.submitMessage(msg)
            }

            if err := emitter.Start(nodeCtx, emit); err != nil {
                log.Printf("[ENGINE] Node %s Start() returned error: %v", nodeID, err)
            }
        }(nodeID, emitter, nodeCtx)
    }

    readyWG.Wait()
}

// newNodeRuntime builds the registry.NodeRuntime a node reaches via
// registry.RuntimeFromContext(ctx) from within Execute/ExecuteMulti/Start:
// flow/global context, error/status/complete event reporting and
// subscription, and same-flow message delivery for Link nodes.
func (e *FlowEngine) newNodeRuntime(activeFlow *ActiveFlow, nodeID, nodeType string) *registry.NodeRuntime {
    return registry.NewNodeRuntime(
        activeFlow.Flow.ID,
        nodeID,
        nodeType,
        activeFlow.ContextStore,
        e.globalStore,
        activeFlow.EventBus,
        func(targetNodeID string, payload map[string]interface{}) {
            e.submitToFlowNode(activeFlow, targetNodeID, payload)
        },
        func(targetNodeID string) (registry.NodeExecutor, bool) {
            executor, ok := activeFlow.nodeExecutors[targetNodeID]
            return executor, ok
        },
    )
}

// submitToFlowNode delivers payload directly to targetNodeID's output within
// activeFlow, as if targetNodeID had just produced it - used by Link nodes
// (docs/NODE_PALETTE_PLAN.md, Phase 1) to jump to a Link In node elsewhere in
// the same flow without a drawn wire. Cross-flow linking is not supported
// yet (see the plan doc).
func (e *FlowEngine) submitToFlowNode(activeFlow *ActiveFlow, targetNodeID string, payload map[string]interface{}) {
    if _, exists := activeFlow.Flow.Nodes[targetNodeID]; !exists {
        log.Printf("[ENGINE] SubmitToNode: node %s not found in flow %s", targetNodeID, activeFlow.Flow.ID)
        return
    }
    msg := NewMessageWithContext(activeFlow.ctx, payload, activeFlow.Flow.ID)
    msg.AddToPath(targetNodeID)
    e.submitMessage(msg)
}

// closeNodeExecutors calls Close on every executor implementing
// registry.Closeable, best-effort (errors are logged, not returned, since a
// single misbehaving node's cleanup failure shouldn't block undeploying the
// rest of the flow).
func closeNodeExecutors(nodeExecutors map[string]registry.NodeExecutor) {
    for nodeID, executor := range nodeExecutors {
        closeable, ok := executor.(registry.Closeable)
        if !ok {
            continue
        }
        if err := closeable.Close(); err != nil {
            log.Printf("[ENGINE] Node %s Close() returned error: %v", nodeID, err)
        }
    }
}

// processFlowMessages processes messages for a specific flow.
func (e *FlowEngine) processFlowMessages(activeFlow *ActiveFlow) {
    defer e.wg.Done()
    defer activeFlow.cancel()
    
    for {
        select {
        case msg := <-activeFlow.msgChan:
            e.processMessage(msg)
        case <-activeFlow.ctx.Done():
            return
        }
    }
}

// Undeploy undeploys a flow, stopping all message processing.
func (e *FlowEngine) Undeploy(flowID string) error {
    e.mu.Lock()
    defer e.mu.Unlock()
    
    activeFlow, exists := e.flows[flowID]
    if !exists {
        return errors.New("flow " + flowID + " not found")
    }
    
    // Cancel the flow context
    activeFlow.cancel()

    // Wait for EmittingNode.Start goroutines to observe cancellation and
    // return before releasing node resources under them.
    activeFlow.wg.Wait()

    // Release resources held by nodes implementing registry.Closeable.
    closeNodeExecutors(activeFlow.nodeExecutors)

    // Remove from active flows
    delete(e.flows, flowID)

    log.Printf("Flow %s undeployed", flowID)
    return nil
}

// GetFlow returns a flow by ID.
func (e *FlowEngine) GetFlow(flowID string) (*Flow, error) {
    e.mu.RLock()
    defer e.mu.RUnlock()
    
    activeFlow, exists := e.flows[flowID]
    if !exists {
        return nil, errors.New("flow " + flowID + " not found")
    }
    
    return activeFlow.Flow, nil
}

// GetAllFlows returns all active flows.
func (e *FlowEngine) GetAllFlows() []*Flow {
    e.mu.RLock()
    defer e.mu.RUnlock()
    
    flows := make([]*Flow, 0, len(e.flows))
    for _, activeFlow := range e.flows {
        flows = append(flows, activeFlow.Flow)
    }
    return flows
}

// GetFlowStatus returns the status of a flow.
func (e *FlowEngine) GetFlowStatus(flowID string) (FlowStatus, error) {
    e.mu.RLock()
    defer e.mu.RUnlock()
    
    activeFlow, exists := e.flows[flowID]
    if !exists {
        return FlowStatusInactive, errors.New("flow " + flowID + " not found")
    }
    
    return activeFlow.Status, nil
}

// GetFlowContext returns a flow's private key-value context store
// (flow.get/set) and its EventBus (error/status events for Catch/Status
// nodes, docs/NODE_PALETTE_PLAN.md Phase 1).
func (e *FlowEngine) GetFlowContext(flowID string) (*registry.ContextStore, *registry.EventBus, error) {
    e.mu.RLock()
    defer e.mu.RUnlock()

    activeFlow, exists := e.flows[flowID]
    if !exists {
        return nil, nil, errors.New("flow " + flowID + " not found")
    }

    return activeFlow.ContextStore, activeFlow.EventBus, nil
}

// InjectMessage injects a message into a flow at a specific node.
func (e *FlowEngine) InjectMessage(flowID, nodeID string, payload map[string]interface{}) error {
    e.mu.RLock()
    activeFlow, exists := e.flows[flowID]
    e.mu.RUnlock()
    
    if !exists {
        return errors.New("flow " + flowID + " not found")
    }
    
    // Check if node exists
    if _, exists := activeFlow.Flow.Nodes[nodeID]; !exists {
        return errors.New("node " + nodeID + " not found in flow " + flowID)
    }
    
    // Create message
    msg := NewMessageWithContext(activeFlow.ctx, payload, flowID)
    msg.AddToPath(nodeID)
    
    // Submit to flow's message channel
    select {
    case activeFlow.msgChan <- msg:
        return nil
    default:
        return errors.New("flow message channel full")
    }
}

// CreateFlow creates a new flow with the given ID and name.
// If id is empty, a UUID will be generated.
func (e *FlowEngine) CreateFlow(id, name string) (*Flow, error) {
    if id == "" {
        id = "flow-" + uuid.New().String()
    }
    flow := NewFlow(id, name)
    
    // Add flow to the engine's flow map
    e.mu.Lock()
    e.flows[id] = &ActiveFlow{
        Flow:         flow,
        Status:       FlowStatusInactive,
        msgChan:      make(chan Message, e.config.MessageBufferSize),
        ContextStore: registry.NewContextStore(),
        EventBus:     registry.NewEventBus(),
    }
    e.mu.Unlock()
    
    // If state manager is set, save the flow
    if e.stateManager != nil {
        if err := e.stateManager.SaveFlow(flow); err != nil {
            return nil, errors.New("failed to save flow: " + err.Error())
        }
    }
    
    return flow, nil
}

// DeleteFlow deletes a flow.
func (e *FlowEngine) DeleteFlow(flowID string) error {
    // First undeploy if active
    e.Undeploy(flowID)
    
    // If state manager is set, delete the flow
    if e.stateManager != nil {
        if err := e.stateManager.DeleteFlow(flowID); err != nil {
            return errors.New("failed to delete flow: " + err.Error())
        }
    }
    
    return nil
}

// LoadFlow loads a flow from the state manager and deploys it.
func (e *FlowEngine) LoadFlow(flowID string) error {
    if e.stateManager == nil {
        return errors.New("no state manager configured")
    }
    
    flow, err := e.stateManager.LoadFlow(flowID)
    if err != nil {
        return errors.New("failed to load flow: " + err.Error())
    }
    
    return e.Deploy(flow)
}

// LoadAllFlows loads all flows from the state manager and deploys them.
func (e *FlowEngine) LoadAllFlows() error {
    if e.stateManager == nil {
        return errors.New("no state manager configured")
    }
    
    log.Printf("[ENGINE] Loading all flows from state manager")
    
    flows, err := e.stateManager.LoadAllFlows()
    if err != nil {
        log.Printf("[ENGINE] Failed to load flows: %v", err)
        return errors.New("failed to load flows: " + err.Error())
    }
    
    log.Printf("[ENGINE] Found %d flows to load", len(flows))
    
    for _, flow := range flows {
        log.Printf("[ENGINE] Loading flow %s with %d nodes and %d connections", flow.ID, len(flow.Nodes), len(flow.Connections))
        // Try to deploy the flow, but if it fails (e.g., validation), log and skip
        if err := e.Deploy(flow); err != nil {
            log.Printf("[ENGINE] Failed to deploy flow %s: %v", flow.ID, err)
            // Don't add to e.flows - let it be loaded on demand via REST API
            // Continue with other flows
            continue
        }
        log.Printf("[ENGINE] Flow %s loaded and deployed successfully", flow.ID)
    }
    
    log.Printf("[ENGINE] Loaded %d flows", len(flows))
    return nil
}

// AddMessageToLog adds a message to the message log for debugging.
// Messages are stored in a circular buffer with maxMessageLog size.
func (e *FlowEngine) AddMessageToLog(msg Message) {
    e.messageLogMu.Lock()
    defer e.messageLogMu.Unlock()

    // Append the message
    e.messageLog = append(e.messageLog, msg)
    
    // Trim if we exceed max size
    if len(e.messageLog) > e.maxMessageLog {
        e.messageLog = e.messageLog[len(e.messageLog)-e.maxMessageLog:]
    }
}

// GetMessageLog returns all messages in the log.
// The returned slice is a copy to prevent external modification.
func (e *FlowEngine) GetMessageLog() []Message {
    e.messageLogMu.RLock()
    defer e.messageLogMu.RUnlock()

    // Create a copy of the slice
    messages := make([]Message, len(e.messageLog))
    copy(messages, e.messageLog)
    return messages
}

// GetMessageLogForFlow returns messages for a specific flow.
func (e *FlowEngine) GetMessageLogForFlow(flowID string) []Message {
    e.messageLogMu.RLock()
    defer e.messageLogMu.RUnlock()

    var flowMessages []Message
    for _, msg := range e.messageLog {
        if msg.FlowID == flowID {
            flowMessages = append(flowMessages, msg)
        }
    }
    return flowMessages
}

// ClearMessageLog clears all messages from the log.
func (e *FlowEngine) ClearMessageLog() {
    e.messageLogMu.Lock()
    defer e.messageLogMu.Unlock()
    e.messageLog = make([]Message, 0)
}

// SetMaxMessageLog sets the maximum number of messages to keep in the log.
func (e *FlowEngine) SetMaxMessageLog(max int) {
    e.messageLogMu.Lock()
    defer e.messageLogMu.Unlock()
    e.maxMessageLog = max
    // Trim existing log if necessary
    if len(e.messageLog) > e.maxMessageLog {
        e.messageLog = e.messageLog[len(e.messageLog)-e.maxMessageLog:]
    }
}
