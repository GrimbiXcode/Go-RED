package engine

import (
	"context"
	"errors"
	"log"
	"sync"
	"time"

	"github.com/GrimbiXcode/Go-RED/internal/registry"
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
	}
}

// SetStateManager sets the state manager for the engine.
func (e *FlowEngine) SetStateManager(sm StateManager) {
	e.stateManager = sm
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
	
	// Validate the flow
	if err := flow.Validate(); err != nil {
		return errors.New("invalid flow: " + err.Error())
	}
	
	// Check if flow is already deployed
	if _, exists := e.flows[flow.ID]; exists {
		return errors.New("flow " + flow.ID + " is already deployed")
	}
	
	// Create active flow
	activeFlow := &ActiveFlow{
		Flow:         flow,
		Status:       FlowStatusActive,
		msgChan:      make(chan Message, e.config.MessageBufferSize),
		nodeExecutors: make(map[string]registry.NodeExecutor),
	}
	
	// Create context for this flow
	activeFlow.ctx, activeFlow.cancel = context.WithCancel(e.ctx)
	
	// Initialize all nodes in the flow
	for nodeID, node := range flow.Nodes {
		executor, err := e.registry.InitializeNode(node.Type, node.Config)
		if err != nil {
			activeFlow.cancel()
			return errors.New("failed to initialize node " + nodeID + ": " + err.Error())
		}
		activeFlow.nodeExecutors[nodeID] = executor
	}
	
	// Start flow message processor
	e.wg.Add(1)
	go e.processFlowMessages(activeFlow)
	
	// Add to active flows
	e.flows[flow.ID] = activeFlow
	
	log.Printf("Flow %s deployed successfully", flow.ID)
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
func (e *FlowEngine) CreateFlow(id, name string) (*Flow, error) {
	flow := NewFlow(id, name)
	
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
	
	flows, err := e.stateManager.LoadAllFlows()
	if err != nil {
		return errors.New("failed to load flows: " + err.Error())
	}
	
	for _, flow := range flows {
		if err := e.Deploy(flow); err != nil {
			log.Printf("Failed to deploy flow %s: %v", flow.ID, err)
			// Continue with other flows
		}
	}
	
	return nil
}
