package engine

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"sort"
	"strconv"
	"sync"
	"sync/atomic"
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

// Sentinel errors returned by the engine's flow lifecycle methods. Callers
// (REST/WebSocket handlers) use errors.Is on these to pick a status code
// and a public message, instead of parsing error strings.
var (
	// ErrFlowNotFound means no flow with that ID is known to the engine.
	ErrFlowNotFound = errors.New("flow not found")
	// ErrFlowExists means a flow with that ID already exists.
	ErrFlowExists = errors.New("flow already exists")
	// ErrFlowNotDeployed means the flow is known but not currently running.
	ErrFlowNotDeployed = errors.New("flow is not deployed")
	// ErrInvalidFlowID means the ID does not match the allowed pattern.
	ErrInvalidFlowID = errors.New("invalid flow ID")
	// ErrInvalidFlow means the flow definition failed validation.
	ErrInvalidFlow = errors.New("invalid flow")
	// ErrNodeInit means a node could not be initialized from its config.
	ErrNodeInit = errors.New("failed to initialize node")
	// ErrPersist means the state manager could not save or delete a flow.
	// Unlike the other errors it may wrap file system details, so handlers
	// must not echo it to clients verbatim.
	ErrPersist = errors.New("failed to persist flow")
)

// flowIDPattern is the only shape a flow ID may have. IDs end up in file
// names (internal/state), URLs and log lines, so anything outside this
// alphabet (path separators, dots, whitespace, control characters) is
// rejected before it reaches any of those.
var flowIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,127}$`)

// ValidateFlowID reports whether id is a well-formed flow ID.
func ValidateFlowID(id string) error {
	if !flowIDPattern.MatchString(id) {
		return fmt.Errorf("%w: %q", ErrInvalidFlowID, id)
	}
	return nil
}

// EngineConfig contains configuration options for the FlowEngine.
type EngineConfig struct {
	// WorkerPoolSize is the number of worker goroutines that route messages
	// between nodes.
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
//
// It keeps two separate views of every flow:
//
//   - flows holds the editable definition of every flow the engine knows
//     about, deployed or not. This is what the editor reads and mutates
//     (always through engine methods, under mu) and what gets persisted.
//   - active holds the runtime of every currently deployed flow. Each
//     ActiveFlow works on a snapshot taken at deploy time, so editing a
//     definition never affects the running flow until it is redeployed.
type FlowEngine struct {
	// flows contains every known flow definition, keyed by flow ID.
	flows map[string]*Flow

	// active contains the runtime state of every deployed flow.
	active map[string]*ActiveFlow

	// registry contains all available node types.
	registry *registry.NodeRegistry

	// msgChan is the channel for messages routed between nodes.
	msgChan chan Message

	// wg tracks worker and per-flow processor goroutines.
	wg sync.WaitGroup

	// ctx and cancel are used for graceful shutdown.
	ctx    context.Context
	cancel context.CancelFunc

	// stopped prevents Stop from being called multiple times.
	stopped bool
	stopMu  sync.Mutex

	// config contains the engine configuration.
	config EngineConfig

	// mu protects flows and active.
	mu sync.RWMutex

	// stateManager is used for persisting flows.
	stateManager StateManager

	// messageIDCounter is used to generate unique message IDs.
	messageIDCounter atomic.Uint64

	// messageLog stores recent messages for debugging and monitoring.
	messageLog    []Message
	messageLogMu  sync.RWMutex
	maxMessageLog int

	// globalStore is the "global" context (flow.get/set's global-scoped
	// counterpart), shared by every flow deployed on this engine.
	globalStore *registry.ContextStore
}

// ActiveFlow is the runtime of a deployed flow.
type ActiveFlow struct {
	// Flow is the deploy-time snapshot of the flow definition this runtime
	// executes. It is never mutated after Deploy.
	Flow *Flow

	// Status is the runtime status of the flow.
	Status FlowStatus

	// msgChan is the channel for messages injected into this flow.
	msgChan chan Message

	// wg tracks EmittingNode.Start goroutines of this flow.
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
	if config.WorkerPoolSize <= 0 {
		config.WorkerPoolSize = DefaultEngineConfig().WorkerPoolSize
	}
	if config.MessageBufferSize <= 0 {
		config.MessageBufferSize = DefaultEngineConfig().MessageBufferSize
	}
	if config.DefaultTimeout <= 0 {
		config.DefaultTimeout = DefaultEngineConfig().DefaultTimeout
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &FlowEngine{
		flows:         make(map[string]*Flow),
		active:        make(map[string]*ActiveFlow),
		registry:      nodeRegistry,
		msgChan:       make(chan Message, config.MessageBufferSize),
		ctx:           ctx,
		cancel:        cancel,
		config:        config,
		messageLog:    make([]Message, 0),
		maxMessageLog: 1000, // Store last 1000 messages
		globalStore:   registry.NewContextStore(),
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

// Start starts the FlowEngine's worker pool.
func (e *FlowEngine) Start() error {
	for i := 0; i < e.config.WorkerPoolSize; i++ {
		e.wg.Add(1)
		go e.worker()
	}
	slog.Info("flow engine started", "workers", e.config.WorkerPoolSize)
	return nil
}

// Stop stops every deployed flow, then the worker pool, and waits for all
// engine goroutines to finish. Flow definitions keep their persisted status
// so that flows which were running are deployed again on the next start.
func (e *FlowEngine) Stop() error {
	e.stopMu.Lock()
	defer e.stopMu.Unlock()

	if e.stopped {
		return nil
	}
	e.stopped = true

	e.cancel()

	e.mu.Lock()
	for _, af := range e.active {
		e.stopActiveLocked(af)
	}
	e.mu.Unlock()

	e.wg.Wait()

	slog.Info("flow engine stopped")
	return nil
}

// worker routes messages from the engine channel until the engine stops.
func (e *FlowEngine) worker() {
	defer e.wg.Done()

	for {
		select {
		case msg := <-e.msgChan:
			e.processMessage(msg)
		case <-e.ctx.Done():
			return
		}
	}
}

// processMessage routes a single message to the nodes connected to the last
// node in its path and executes each of them in its own goroutine.
func (e *FlowEngine) processMessage(msg Message) {
	e.AddMessageToLog(msg)

	e.mu.RLock()
	activeFlow, exists := e.active[msg.FlowID]
	e.mu.RUnlock()

	if !exists {
		slog.Debug("dropping message for flow that is not deployed", "flow", msg.FlowID, "message", msg.ID)
		return
	}

	targetNodes := e.findTargetNodes(activeFlow.Flow, msg)

	for _, nodeID := range targetNodes {
		executor, exists := activeFlow.nodeExecutors[nodeID]
		if !exists {
			slog.Warn("message routed to unknown node", "flow", msg.FlowID, "node", nodeID)
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

		go func(nodeID, nodeType string, exec registry.NodeExecutor, nodeCtx context.Context) {
			defer cancel()

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
	slog.Warn("node execution failed", "flow", msg.FlowID, "node", nodeID, "type", nodeType, "err", err)

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

	targetNodes := make(map[string]bool)
	for _, conn := range flow.Connections {
		targetNodes[conn.TargetNode] = true
	}

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

// submitMessage hands a message to the worker pool. It never blocks and is
// safe to call at any time, including after Stop (the message is dropped).
func (e *FlowEngine) submitMessage(msg Message) {
	msg.ID = "msg-" + strconv.FormatUint(e.messageIDCounter.Add(1), 10)

	if e.ctx.Err() != nil {
		return
	}

	select {
	case e.msgChan <- msg:
	default:
		slog.Warn("message channel full, dropping message", "flow", msg.FlowID, "message", msg.ID)
	}
}

// SubmitMessage submits a message to the engine for processing.
// This is the public method for submitting messages.
func (e *FlowEngine) SubmitMessage(msg Message) {
	e.submitMessage(msg)
}

// Deploy registers flow as the definition for flow.ID (replacing any
// existing definition with that ID) and deploys it. If the flow is already
// running, the old runtime is stopped first and the new definition takes
// over, so Deploy is also the redeploy operation. The engine takes
// ownership of flow; callers must not mutate it afterwards.
func (e *FlowEngine) Deploy(flow *Flow) error {
	if flow == nil {
		return errors.New("flow is nil")
	}
	if err := ValidateFlowID(flow.ID); err != nil {
		return err
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	if err := e.deployLocked(flow); err != nil {
		return err
	}
	e.flows[flow.ID] = flow
	return nil
}

// DeployFlow deploys (or redeploys) the known flow with the given ID from
// its current definition.
func (e *FlowEngine) DeployFlow(flowID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	def, ok := e.flows[flowID]
	if !ok {
		return fmt.Errorf("%w: %s", ErrFlowNotFound, flowID)
	}
	return e.deployLocked(def)
}

// deployLocked validates def, stops a running instance of it if there is
// one, and starts a new runtime from a snapshot of def. Must be called with
// e.mu held. On failure def's status is set to FlowStatusError and the flow
// is left undeployed; on success it is FlowStatusActive. Either way the new
// status is persisted.
func (e *FlowEngine) deployLocked(def *Flow) error {
	if err := def.Validate(); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidFlow, err)
	}

	if old, running := e.active[def.ID]; running {
		slog.Info("redeploying flow", "flow", def.ID)
		e.stopActiveLocked(old)
	}

	snapshot := def.Clone()
	activeFlow := &ActiveFlow{
		Flow:          snapshot,
		Status:        FlowStatusActive,
		msgChan:       make(chan Message, e.config.MessageBufferSize),
		nodeExecutors: make(map[string]registry.NodeExecutor),
		ContextStore:  registry.NewContextStore(),
		EventBus:      registry.NewEventBus(),
	}
	activeFlow.ctx, activeFlow.cancel = context.WithCancel(e.ctx)

	for nodeID, node := range snapshot.Nodes {
		executor, err := e.registry.InitializeNode(node.Type, node.Config)
		if err != nil {
			// Release any resources already-initialized nodes in this flow
			// acquired (e.g. an earlier node opened a connection) before
			// bailing out, then cancel the flow context.
			closeNodeExecutors(activeFlow.nodeExecutors)
			activeFlow.cancel()
			def.Status = FlowStatusError
			e.persistLocked(def)
			slog.Warn("flow deploy failed", "flow", def.ID, "node", nodeID, "type", node.Type, "err", err)
			return fmt.Errorf("%w %s: %v", ErrNodeInit, nodeID, err)
		}
		activeFlow.nodeExecutors[nodeID] = executor
	}

	e.wg.Add(1)
	go e.processFlowMessages(activeFlow)

	// Start nodes that originate their own messages (EmittingNode), e.g. a
	// TCP listener or file watcher, rather than only reacting to input.
	e.startEmittingNodes(activeFlow)

	e.active[def.ID] = activeFlow
	def.Status = FlowStatusActive
	def.DeployedAt = time.Now().UTC()
	snapshot.Status = FlowStatusActive
	snapshot.DeployedAt = def.DeployedAt
	e.persistLocked(def)

	slog.Info("flow deployed", "flow", def.ID, "nodes", len(activeFlow.nodeExecutors), "connections", len(snapshot.Connections))
	return nil
}

// stopActiveLocked cancels a runtime, waits for its emitting nodes, releases
// node resources and removes it from e.active. Must be called with e.mu
// held. It does not touch the flow definition's status.
func (e *FlowEngine) stopActiveLocked(activeFlow *ActiveFlow) {
	activeFlow.cancel()

	// Wait for EmittingNode.Start goroutines to observe cancellation and
	// return before releasing node resources under them.
	activeFlow.wg.Wait()

	closeNodeExecutors(activeFlow.nodeExecutors)

	delete(e.active, activeFlow.Flow.ID)
}

// persistLocked saves def through the state manager, if one is configured.
// Must be called with e.mu held so nothing mutates def while it is
// serialized. Failures are logged and returned.
func (e *FlowEngine) persistLocked(def *Flow) error {
	if e.stateManager == nil {
		return nil
	}
	if err := e.stateManager.SaveFlow(def); err != nil {
		slog.Error("failed to persist flow", "flow", def.ID, "err", err)
		return fmt.Errorf("%w: %v", ErrPersist, err)
	}
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
					slog.Warn("node did not signal ready in time, proceeding", "flow", activeFlow.Flow.ID, "node", nodeID, "timeout", eagerReadyTimeout)
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
				slog.Warn("emitting node stopped with error", "flow", activeFlow.Flow.ID, "node", nodeID, "err", err)
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
		slog.Warn("SubmitToNode target not found", "flow", activeFlow.Flow.ID, "node", targetNodeID)
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
			slog.Warn("node Close returned error", "node", nodeID, "err", err)
		}
	}
}

// processFlowMessages processes messages injected into a specific flow.
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

// Undeploy stops a running flow. The definition stays known to the engine
// with status FlowStatusInactive. Undeploying a known flow that is not
// running is a no-op; an unknown flow returns ErrFlowNotFound.
func (e *FlowEngine) Undeploy(flowID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	def, known := e.flows[flowID]
	activeFlow, running := e.active[flowID]
	if !known && !running {
		return fmt.Errorf("%w: %s", ErrFlowNotFound, flowID)
	}

	if running {
		e.stopActiveLocked(activeFlow)
		slog.Info("flow undeployed", "flow", flowID)
	}
	if known {
		def.Status = FlowStatusInactive
		e.persistLocked(def)
	}
	return nil
}

// IsDeployed reports whether the flow is currently running.
func (e *FlowEngine) IsDeployed(flowID string) bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	_, running := e.active[flowID]
	return running
}

// GetFlow returns a copy of a known flow's definition. Mutating the copy has
// no effect on the engine; use UpdateFlow for that.
func (e *FlowEngine) GetFlow(flowID string) (*Flow, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	def, exists := e.flows[flowID]
	if !exists {
		return nil, fmt.Errorf("%w: %s", ErrFlowNotFound, flowID)
	}
	return def.Clone(), nil
}

// GetAllFlows returns copies of every known flow definition, deployed or
// not, in a stable order (creation time, then ID).
func (e *FlowEngine) GetAllFlows() []*Flow {
	e.mu.RLock()
	defer e.mu.RUnlock()

	flows := make([]*Flow, 0, len(e.flows))
	for _, def := range e.flows {
		flows = append(flows, def.Clone())
	}
	sort.Slice(flows, func(i, j int) bool {
		if !flows[i].CreatedAt.Equal(flows[j].CreatedAt) {
			return flows[i].CreatedAt.Before(flows[j].CreatedAt)
		}
		return flows[i].ID < flows[j].ID
	})
	return flows
}

// GetFlowStatus returns the status of a known flow.
func (e *FlowEngine) GetFlowStatus(flowID string) (FlowStatus, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	def, exists := e.flows[flowID]
	if !exists {
		return FlowStatusInactive, fmt.Errorf("%w: %s", ErrFlowNotFound, flowID)
	}
	return def.Status, nil
}

// GetFlowContext returns a running flow's private key-value context store
// (flow.get/set) and its EventBus (error/status events for Catch/Status
// nodes, docs/NODE_PALETTE_PLAN.md Phase 1).
func (e *FlowEngine) GetFlowContext(flowID string) (*registry.ContextStore, *registry.EventBus, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	activeFlow, exists := e.active[flowID]
	if !exists {
		return nil, nil, fmt.Errorf("%w: %s", ErrFlowNotDeployed, flowID)
	}
	return activeFlow.ContextStore, activeFlow.EventBus, nil
}

// InjectMessage injects a message into a running flow at a specific node.
func (e *FlowEngine) InjectMessage(flowID, nodeID string, payload map[string]interface{}) error {
	e.mu.RLock()
	activeFlow, exists := e.active[flowID]
	e.mu.RUnlock()

	if !exists {
		return fmt.Errorf("%w: %s", ErrFlowNotDeployed, flowID)
	}

	if _, exists := activeFlow.Flow.Nodes[nodeID]; !exists {
		return fmt.Errorf("node %s not found in flow %s", nodeID, flowID)
	}

	msg := NewMessageWithContext(activeFlow.ctx, payload, flowID)
	msg.AddToPath(nodeID)

	select {
	case activeFlow.msgChan <- msg:
		return nil
	default:
		return errors.New("flow message channel full")
	}
}

// CreateFlow creates and persists a new, undeployed flow. If id is empty a
// UUID-based ID is generated. Returns a copy of the new definition.
func (e *FlowEngine) CreateFlow(id, name, description string) (*Flow, error) {
	if id == "" {
		id = "flow-" + uuid.New().String()
	}
	if err := ValidateFlowID(id); err != nil {
		return nil, err
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	if _, exists := e.flows[id]; exists {
		return nil, fmt.Errorf("%w: %s", ErrFlowExists, id)
	}

	flow := NewFlow(id, name)
	flow.Description = description

	if err := e.persistLocked(flow); err != nil {
		return nil, err
	}
	e.flows[id] = flow

	slog.Info("flow created", "flow", id, "name", name)
	return flow.Clone(), nil
}

// AddFlow registers an existing, fully populated definition (e.g. an
// imported flow) as a new undeployed flow and persists it. The engine takes
// ownership of flow; callers must not mutate it afterwards.
func (e *FlowEngine) AddFlow(flow *Flow) error {
	if flow == nil {
		return errors.New("flow is nil")
	}
	if err := ValidateFlowID(flow.ID); err != nil {
		return err
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	if _, exists := e.flows[flow.ID]; exists {
		return fmt.Errorf("%w: %s", ErrFlowExists, flow.ID)
	}

	flow.Status = FlowStatusInactive
	if err := e.persistLocked(flow); err != nil {
		return err
	}
	e.flows[flow.ID] = flow
	return nil
}

// UpdateFlow applies mutate to the definition of a known flow under the
// engine lock, bumps UpdatedAt, persists the result and returns a copy of
// it. If mutate returns an error nothing is persisted. A running instance of
// the flow is not affected until it is redeployed.
func (e *FlowEngine) UpdateFlow(flowID string, mutate func(*Flow) error) (*Flow, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	def, exists := e.flows[flowID]
	if !exists {
		return nil, fmt.Errorf("%w: %s", ErrFlowNotFound, flowID)
	}

	if err := mutate(def); err != nil {
		return nil, err
	}
	def.ID = flowID
	def.UpdatedAt = time.Now().UTC()

	if err := e.persistLocked(def); err != nil {
		return nil, err
	}
	return def.Clone(), nil
}

// DeleteFlow stops the flow if it is running and removes its definition
// from the engine and from persistent storage.
func (e *FlowEngine) DeleteFlow(flowID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	_, known := e.flows[flowID]
	activeFlow, running := e.active[flowID]
	if !known && !running {
		return fmt.Errorf("%w: %s", ErrFlowNotFound, flowID)
	}

	if running {
		e.stopActiveLocked(activeFlow)
	}
	delete(e.flows, flowID)

	if e.stateManager != nil {
		if err := e.stateManager.DeleteFlow(flowID); err != nil && !errors.Is(err, ErrFlowNotFound) {
			slog.Error("failed to delete persisted flow", "flow", flowID, "err", err)
			return fmt.Errorf("%w: %v", ErrPersist, err)
		}
	}

	slog.Info("flow deleted", "flow", flowID)
	return nil
}

// LoadAllFlows loads every persisted flow definition into the engine and
// deploys those that were running when they were last saved. A flow that
// fails to deploy is kept as a known flow with status FlowStatusError.
func (e *FlowEngine) LoadAllFlows() error {
	if e.stateManager == nil {
		return errors.New("no state manager configured")
	}

	flows, err := e.stateManager.LoadAllFlows()
	if err != nil {
		return fmt.Errorf("failed to load flows: %w", err)
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	deployed := 0
	for _, flow := range flows {
		if err := ValidateFlowID(flow.ID); err != nil {
			slog.Warn("skipping persisted flow with invalid ID", "flow", flow.ID, "err", err)
			continue
		}
		wasRunning := flow.Status == FlowStatusActive
		flow.Status = FlowStatusInactive
		e.flows[flow.ID] = flow

		if !wasRunning {
			continue
		}
		if err := e.deployLocked(flow); err != nil {
			slog.Error("failed to deploy persisted flow", "flow", flow.ID, "err", err)
			flow.Status = FlowStatusError
			e.persistLocked(flow)
			continue
		}
		deployed++
	}

	slog.Info("flows loaded", "known", len(e.flows), "deployed", deployed)
	return nil
}

// AddMessageToLog adds a message to the message log for debugging.
// Messages are stored in a circular buffer with maxMessageLog size.
func (e *FlowEngine) AddMessageToLog(msg Message) {
	e.messageLogMu.Lock()
	defer e.messageLogMu.Unlock()

	e.messageLog = append(e.messageLog, msg)

	if len(e.messageLog) > e.maxMessageLog {
		e.messageLog = e.messageLog[len(e.messageLog)-e.maxMessageLog:]
	}
}

// GetMessageLog returns all messages in the log.
// The returned slice is a copy to prevent external modification.
func (e *FlowEngine) GetMessageLog() []Message {
	e.messageLogMu.RLock()
	defer e.messageLogMu.RUnlock()

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
	if len(e.messageLog) > e.maxMessageLog {
		e.messageLog = e.messageLog[len(e.messageLog)-e.maxMessageLog:]
	}
}
