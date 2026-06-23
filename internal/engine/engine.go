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
}

// NewFlowEngine creates a new FlowEngine with the given configuration and registry.
func NewFlowEngine(config EngineConfig, registry *registry.NodeRegistry) *FlowEngine {
	ctx, cancel := context.WithCancel(context.Background())
	
	return &FlowEngine{
		flows:       make(map[string]*ActiveFlow),
		registry:    registry,
		msgChan:     make(chan Message, config.MessageBufferSize),
		ctx:         ctx,
		cancel:      cancel,
		config:      config,
		messageIDCounter: 0,
		messageLog:   make([]Message, 0),
		maxMessageLog: 1000, // Store last 1000 messages
	}
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
		
		// Create a new message context with timeout
		ctx, cancel := context.WithTimeout(msg.Context, e.config.DefaultTimeout)
		
		// Execute the node in a goroutine
		go func(nodeID string, exec registry.NodeExecutor, nodeCtx context.Context) {
			defer cancel()
			
			// Add node to path
			newMsg := msg.Clone()
			newMsg.AddToPath(nodeID)
			newMsg.Context = nodeCtx
			
			// Execute the node
			output, err := exec.Execute(nodeCtx, newMsg.Payload)
			if err != nil {
				log.Printf("Node %s in flow %s failed: %v", nodeID, msg.FlowID, err)
				// TODO: Handle error (retry, error output, etc.)
				return
			}
			
			// Create new message with output
			newMsg.Payload = output
			
			// Send to next nodes
			e.submitMessage(newMsg)
		}(nodeID, executor, ctx)
	}
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
	return e.findConnectedNodes(flow, lastNodeID)
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
func (e *FlowEngine) findConnectedNodes(flow *Flow, nodeID string) []string {
	var connectedNodes []string
	
	for _, conn := range flow.Connections {
		if conn.SourceNode == nodeID {
			// Check if the connection's source port matches
			// (For now, we ignore ports and connect all outputs)
			connectedNodes = append(connectedNodes, conn.TargetNode)
		}
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
	}
	
	// Update flow status to active
	flow.Status = FlowStatusActive
	
	// Create context for this flow
	activeFlow.ctx, activeFlow.cancel = context.WithCancel(e.ctx)
	
	// Initialize all nodes in the flow
	log.Printf("[ENGINE] Initializing %d nodes for flow %s", len(flow.Nodes), flow.ID)
	for nodeID, node := range flow.Nodes {
		log.Printf("[ENGINE] Initializing node %s of type %s", nodeID, node.Type)
		executor, err := e.registry.InitializeNode(node.Type, node.Config)
		if err != nil {
			activeFlow.cancel()
			log.Printf("[ENGINE] Failed to initialize node %s: %v", nodeID, err)
			return errors.New("failed to initialize node " + nodeID + ": " + err.Error())
		}
		activeFlow.nodeExecutors[nodeID] = executor
		log.Printf("[ENGINE] Node %s initialized successfully", nodeID)
	}
	
	// Start flow message processor
	e.wg.Add(1)
	go e.processFlowMessages(activeFlow)
	
	// Add to active flows
	e.flows[flow.ID] = activeFlow
	
	log.Printf("[ENGINE] Flow %s deployed successfully with %d initialized nodes", flow.ID, len(activeFlow.nodeExecutors))
	return nil
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
	
	// Wait for flow goroutines to complete
	activeFlow.wg.Wait()
	
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

// FlowSummary represents a summary of a flow for listings
type FlowSummary struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	Status      FlowStatus `json:"status"`
	NodeCount   int       `json:"nodeCount"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

// GetAllFlowsSummary returns a summary of all flows for listings
func (e *FlowEngine) GetAllFlowsSummary() []FlowSummary {
	e.mu.RLock()
	defer e.mu.RUnlock()
	
	summaries := make([]FlowSummary, 0, len(e.flows))
	for _, activeFlow := range e.flows {
		flow := activeFlow.Flow
		summaries = append(summaries, FlowSummary{
			ID:          flow.ID,
			Name:        flow.Name,
			Description: flow.Description,
			Status:      flow.Status,
			NodeCount:   len(flow.Nodes),
			CreatedAt:   flow.CreatedAt,
			UpdatedAt:   flow.UpdatedAt,
		})
	}
	return summaries
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
		Flow:   flow,
		Status: FlowStatusInactive,
		msgChan: make(chan Message, e.config.MessageBufferSize),
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
