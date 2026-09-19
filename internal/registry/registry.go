// Package registry manages all available node types and their factories.
package registry

import (
	"context"
	"errors"
	"log/slog"
	"sync"
)

// NodeExecutor is the base interface for all nodes.
// Every node in Go—RED must implement this interface.
type NodeExecutor interface {
	// Execute processes the input message and returns output.
	// The context can be used for timeout and cancellation.
	Execute(ctx interface{}, input map[string]interface{}) (map[string]interface{}, error)

	// Validate checks the node configuration.
	// Should return an error if the configuration is invalid.
	Validate() error

	// GetConfig returns the current configuration as a map.
	GetConfig() map[string]interface{}

	// SetConfig sets the configuration from a map.
	// Should validate the configuration and return an error if invalid.
	SetConfig(config map[string]interface{}) error
}

// The interfaces below are optional extensions to NodeExecutor. A node type
// implements one of them only if it needs the corresponding capability; the
// engine type-asserts for them at runtime, so existing NodeExecutor
// implementations (inject, debug, function) are unaffected by their
// existence. See docs/NODE_PALETTE_PLAN.md ("Architektur-Voraussetzungen,
// Phase 0") for the gap analysis that motivated them.

// MultiOutputExecutor is implemented by nodes that route different
// payloads to different output ports of the same node (e.g. a future
// Switch node picking one of several outputs, or a Catch/RBE node that may
// emit on no port at all for a given input). The engine calls ExecuteMulti
// instead of Execute when a node implements this interface.
type MultiOutputExecutor interface {
	NodeExecutor

	// ExecuteMulti behaves like Execute but returns one payload per output
	// port ID (matching registry.Port.ID from the node's NodeMetadata.Outputs).
	// A port that is absent from the result, or mapped to nil, means: do not
	// send a message on that port for this invocation.
	ExecuteMulti(ctx interface{}, input map[string]interface{}) (map[string]map[string]interface{}, error)
}

// Closeable is implemented by nodes that hold resources (open connections,
// timers, file handles, ...) which must be released when their flow is
// undeployed. The engine calls Close exactly once per node instance during
// Undeploy, after the flow's context has been cancelled and any Start
// goroutine (see EmittingNode) has returned.
type Closeable interface {
	Close() error
}

// EmittingNode is implemented by nodes that originate messages on their own
// instead of only reacting to an incoming one (e.g. a TCP listener, an MQTT
// subscription, a file watcher). The engine invokes Start once, in its own
// goroutine, when the node's flow is deployed. Start must block until ctx is
// cancelled (which happens when the flow is undeployed) and call emit for
// every message it wants to inject at this node's output.
type EmittingNode interface {
	NodeExecutor

	Start(ctx context.Context, emit func(payload map[string]interface{})) error
}

// EagerlyReadyNode is an optional marker an EmittingNode additionally
// implements to tell the engine: "my Start does synchronous setup - e.g.
// subscribing to this flow's EventBus - that other nodes' very first
// Execute call might race against, so wait for it before returning from
// Deploy." Without this, Deploy returns as soon as Start's goroutine has
// merely been launched, not run; a message injected immediately
// afterward (the common "deploy, then inject" sequence in tests and in
// any caller that doesn't add its own delay) can reach a node that
// publishes an event - e.g. NodeRuntime.ReportStatus, or the engine's own
// automatic error/complete events after every Execute - before a Catch/
// Status/Complete node's Start has actually called OnError/OnStatus/
// OnComplete to subscribe, silently dropping it (registry.EventBus has no
// replay/buffering for a subscriber that arrives late).
//
// Implementing this interface only makes sense for a node whose readiness
// depends on order relative to *other nodes in the same flow*, not on an
// external event source (a TCP listener, an MQTT subscription, a
// filesystem watch): those originate their own events independently of
// this flow's message processing, so there's nothing for them to race
// against in the same "Deploy then inject" sequence, and requiring the
// engine to wait for them would only add needless latency to every
// Deploy. Implemented by catch/status/complete; SignalReady/WithReady
// live in ready.go.
type EagerlyReadyNode interface {
	EmittingNode

	// EagerlyReady is a marker method with no meaningful behavior of its
	// own - implementing it opts a node into the engine's ready-wait (see
	// startEmittingNodes). The node's real obligation is to call
	// SignalReady(ctx) synchronously, as the first thing Start does,
	// once its setup is complete.
	EagerlyReady()
}

// NodeFactory is a function that creates a new NodeExecutor instance.
type NodeFactory func() NodeExecutor

// Port defines an input or output port for a node.
type Port struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Required    bool   `json:"required"`
}

// Property defines a configuration property for a node.
type Property struct {
	Type        string      `json:"type"` // string, number, boolean, object, array
	Description string      `json:"description"`
	Default     interface{} `json:"default"`
	Enum        []string    `json:"enum"` // Possible values for dropdowns
	Min         *float64    `json:"min"`
	Max         *float64    `json:"max"`
	Pattern     string      `json:"pattern"` // Regex pattern for strings

	// Editor hints ("schema v2", see docs/PROTOCOL.md). None of them
	// changes what a value looks like on the wire; a property without a
	// Widget is rendered by Type: Enum → select, boolean → checkbox,
	// number → number, object/array → JSON, string → text.

	// Label is the field caption; Description is the help text under it.
	Label       string `json:"label,omitempty"`
	Placeholder string `json:"placeholder,omitempty"`
	// Group puts the field under a section heading; Order sorts fields.
	Group string `json:"group,omitempty"`
	Order int    `json:"order,omitempty"`
	// Widget picks the editor control (one of the Widget* constants).
	Widget string `json:"widget,omitempty"`
	// Language is the syntax of a WidgetCode field (javascript, json, mustache, text).
	Language string `json:"language,omitempty"`
	// Unit is the unit a WidgetDuration value is stored in (ms or s).
	Unit string `json:"unit,omitempty"`
	// Options are labelled choices for WidgetSelect; Enum is the unlabelled form.
	Options []Option `json:"options,omitempty"`
	// TypedInput configures WidgetTypedInput.
	TypedInput *TypedInputOptions `json:"typedInput,omitempty"`
	// Items is the schema of one element of a WidgetList array.
	Items *Schema `json:"items,omitempty"`
	// NodeTypes restricts WidgetNodeSelect to these node types (empty: any node).
	NodeTypes []string `json:"nodeTypes,omitempty"`
	// Multiple lets WidgetNodeSelect pick several nodes; the value is then an array.
	Multiple bool `json:"multiple,omitempty"`
	// VisibleWhen hides the field unless another property has one of the given values.
	VisibleWhen *Condition `json:"visibleWhen,omitempty"`
}

// Schema defines the configuration schema for a node.
type Schema struct {
	Properties map[string]Property `json:"properties"`
	Required   []string            `json:"required"`
}

// NodeMetadata contains metadata about a node type.
// This is used by the UI to display node information.
type NodeMetadata struct {
	ID           string   `json:"id"`
	Type         string   `json:"type"`
	Name         string   `json:"name"`
	Description  string   `json:"description"`
	Category     string   `json:"category"`     // input, output, function, flow-control, storage
	Inputs       []Port   `json:"inputs"`       // Input ports
	Outputs      []Port   `json:"outputs"`      // Output ports
	ConfigSchema Schema   `json:"configSchema"` // Configuration schema
	Icon         string   `json:"icon"`         // SVG icon for UI
	Tags         []string `json:"tags"`         // Search tags

	// OutputsFrom makes the number of output ports depend on the length
	// of an array-typed config property (a Switch node has one output per
	// rule). Outputs then only names the ports shown while that array is
	// empty. Port IDs are the element indexes ("0", "1", ...).
	OutputsFrom *OutputsFrom `json:"outputsFrom,omitempty"`
	// Color overrides the category color on the canvas (CSS color).
	Color string `json:"color,omitempty"`
	// Help is the node's documentation in Markdown, shown in the editor.
	Help string `json:"help,omitempty"`
	// DefaultName is the canvas label of a node that has no name; Name is
	// used when it is empty.
	DefaultName string `json:"defaultName,omitempty"`
}

// Node represents a registered node type.
type Node struct {
	Type     string
	Metadata NodeMetadata
	Factory  NodeFactory
}

// NodeRegistry manages all registered node types.
type NodeRegistry struct {
	nodes     map[string]*Node       // nodeType -> Node
	factories map[string]NodeFactory // nodeType -> Factory
	mu        sync.RWMutex
}

// Global registry instance
var globalRegistry *NodeRegistry
var once sync.Once

// GetGlobalRegistry returns the global NodeRegistry instance.
func GetGlobalRegistry() *NodeRegistry {
	once.Do(func() {
		globalRegistry = NewNodeRegistry()
	})
	return globalRegistry
}

// NewNodeRegistry creates a new NodeRegistry instance.
func NewNodeRegistry() *NodeRegistry {
	return &NodeRegistry{
		nodes:     make(map[string]*Node),
		factories: make(map[string]NodeFactory),
	}
}

// RegisterNode registers a new node type.
func (r *NodeRegistry) RegisterNode(node *Node) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if node.Type == "" {
		return errors.New("node type cannot be empty")
	}

	if _, exists := r.nodes[node.Type]; exists {
		return errors.New("node type already registered: " + node.Type)
	}

	r.nodes[node.Type] = node
	r.factories[node.Type] = node.Factory

	slog.Debug("node type registered", "type", node.Type, "name", node.Metadata.Name)
	return nil
}

// RegisterFactory registers a node factory with metadata.
// This is a convenience method that creates a Node and registers it.
func (r *NodeRegistry) RegisterFactory(nodeType string, factory NodeFactory, metadata NodeMetadata) error {
	node := &Node{
		Type:     nodeType,
		Metadata: metadata,
		Factory:  factory,
	}
	return r.RegisterNode(node)
}

// GetExecutor returns a new NodeExecutor instance for the given node type.
func (r *NodeRegistry) GetExecutor(nodeType string) (NodeExecutor, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	factory, exists := r.factories[nodeType]
	if !exists {
		return nil, errors.New("node type not found: " + nodeType)
	}

	return factory(), nil
}

// GetMetadata returns the metadata for a node type.
func (r *NodeRegistry) GetMetadata(nodeType string) (NodeMetadata, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	node, exists := r.nodes[nodeType]
	if !exists {
		return NodeMetadata{}, errors.New("node type not found: " + nodeType)
	}

	return node.Metadata, nil
}

// GetAllNodes returns metadata for all registered nodes.
func (r *NodeRegistry) GetAllNodes() []NodeMetadata {
	r.mu.RLock()
	defer r.mu.RUnlock()

	nodes := make([]NodeMetadata, 0, len(r.nodes))
	for _, node := range r.nodes {
		nodes = append(nodes, node.Metadata)
	}
	return nodes
}

// GetNodesByCategory returns nodes filtered by category.
func (r *NodeRegistry) GetNodesByCategory(category string) []NodeMetadata {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var nodes []NodeMetadata
	for _, node := range r.nodes {
		if node.Metadata.Category == category {
			nodes = append(nodes, node.Metadata)
		}
	}
	return nodes
}

// InitializeNode initializes a node with its configuration.
func (r *NodeRegistry) InitializeNode(nodeType string, config map[string]interface{}) (NodeExecutor, error) {
	executor, err := r.GetExecutor(nodeType)
	if err != nil {
		return nil, err
	}

	if err := executor.SetConfig(config); err != nil {
		return nil, err
	}

	if err := executor.Validate(); err != nil {
		return nil, err
	}

	return executor, nil
}

// IsRegistered checks if a node type is registered.
func (r *NodeRegistry) IsRegistered(nodeType string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	_, exists := r.nodes[nodeType]
	return exists
}

// Unregister removes a node type from the registry.
func (r *NodeRegistry) Unregister(nodeType string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.nodes[nodeType]; !exists {
		return errors.New("node type not found: " + nodeType)
	}

	delete(r.nodes, nodeType)
	delete(r.factories, nodeType)

	slog.Debug("node type unregistered", "type", nodeType)
	return nil
}
