// Package inject provides the Inject node implementation.
//
// An Inject node originates messages: manually (the editor's inject button
// routes through FlowEngine.InjectMessage, which calls Execute), once when
// the flow is deployed (InjectOnce), or repeatedly on a fixed interval
// (Interval > 0). The automatic variants run in Start, which the engine
// invokes for every registry.EmittingNode when the flow is deployed.
package inject

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/GrimbiXcode/Go-RED/internal/registry"
)

// InjectNode implements a node that injects messages into a flow.
type InjectNode struct {
	config InjectConfig

	// mu guards ticker, lastPayload and stopped: Start runs in its own
	// goroutine and races with Execute/Stop calls from other goroutines
	// otherwise.
	mu sync.Mutex

	// ticker is the interval ticker while Start is running with Interval > 0.
	ticker *time.Ticker

	// done stops Start (and the ticker) when the node is closed.
	done chan struct{}

	// stopped guards against closing done twice.
	stopped bool

	// lastPayload stores the last manually injected payload.
	lastPayload map[string]interface{}
}

// InjectConfig contains the configuration for an Inject node.
type InjectConfig struct {
	// Payload is the data to inject
	Payload map[string]interface{} `json:"payload"`

	// Interval is the time between automatic injections (in milliseconds)
	// 0 means manual injection only
	Interval int64 `json:"interval"`

	// Topic is an optional topic for the message
	Topic string `json:"topic"`

	// InjectOnce indicates whether to inject once when the flow is deployed
	InjectOnce bool `json:"injectOnce"`
}

// NewInjectNode creates a new InjectNode with default configuration.
func NewInjectNode() *InjectNode {
	return &InjectNode{
		config: InjectConfig{
			Payload:    map[string]interface{}{"payload": ""},
			Interval:   0,
			Topic:      "",
			InjectOnce: false,
		},
		done: make(chan struct{}),
	}
}

// Execute handles a manual injection: the payload given by the caller (or
// the configured payload when none is given) becomes the emitted message.
func (n *InjectNode) Execute(ctx interface{}, input map[string]interface{}) (map[string]interface{}, error) {
	if input != nil {
		n.mu.Lock()
		n.lastPayload = input
		n.mu.Unlock()
	}
	return n.injectPayload(), nil
}

// Start runs the automatic injections for the lifetime of the flow: one
// message right away when InjectOnce is set, and one message per Interval
// while Interval > 0. It blocks until ctx is cancelled (flow undeployed)
// or the node is stopped, as the registry.EmittingNode contract requires.
func (n *InjectNode) Start(ctx context.Context, emit func(payload map[string]interface{})) error {
	if n.config.InjectOnce {
		emit(n.configuredPayload())
	}

	if n.config.Interval <= 0 {
		select {
		case <-ctx.Done():
		case <-n.done:
		}
		return nil
	}

	ticker := time.NewTicker(time.Duration(n.config.Interval) * time.Millisecond)
	defer ticker.Stop()

	n.mu.Lock()
	n.ticker = ticker
	n.mu.Unlock()

	defer func() {
		n.mu.Lock()
		if n.ticker == ticker {
			n.ticker = nil
		}
		n.mu.Unlock()
	}()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-n.done:
			return nil
		case <-ticker.C:
			emit(n.configuredPayload())
		}
	}
}

// configuredPayload returns a fresh copy of the configured payload (plus
// topic), so downstream nodes mutating the message never touch the config.
func (n *InjectNode) configuredPayload() map[string]interface{} {
	n.mu.Lock()
	payload := clonePayload(n.config.Payload)
	n.mu.Unlock()
	return n.withTopic(payload)
}

// injectPayload returns the payload for a manual injection: the last
// payload handed to Execute, or the configured one.
func (n *InjectNode) injectPayload() map[string]interface{} {
	n.mu.Lock()
	source := n.config.Payload
	if n.lastPayload != nil {
		source = n.lastPayload
	}
	payload := clonePayload(source)
	n.mu.Unlock()
	return n.withTopic(payload)
}

func (n *InjectNode) withTopic(payload map[string]interface{}) map[string]interface{} {
	if n.config.Topic != "" {
		if payload == nil {
			payload = make(map[string]interface{})
		}
		payload["topic"] = n.config.Topic
	}
	return payload
}

func clonePayload(m map[string]interface{}) map[string]interface{} {
	if m == nil {
		return nil
	}
	clone := make(map[string]interface{}, len(m))
	for k, v := range m {
		clone[k] = v
	}
	return clone
}

// Ticker returns the currently running interval ticker, if any, for
// tests/callers that need to observe whether Start is ticking.
func (n *InjectNode) Ticker() *time.Ticker {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.ticker
}

func (n *InjectNode) Validate() error {
	if n.config.Interval < 0 {
		return errors.New("interval cannot be negative")
	}
	return nil
}

func (n *InjectNode) GetConfig() map[string]interface{} {
	return map[string]interface{}{
		"payload":    n.config.Payload,
		"interval":   n.config.Interval,
		"topic":      n.config.Topic,
		"injectOnce": n.config.InjectOnce,
	}
}

func (n *InjectNode) SetConfig(config map[string]interface{}) error {
	if payload, ok := config["payload"].(map[string]interface{}); ok {
		n.config.Payload = payload
	}
	if interval, ok := config["interval"].(float64); ok {
		n.config.Interval = int64(interval)
	}
	if topic, ok := config["topic"].(string); ok {
		n.config.Topic = topic
	}
	if injectOnce, ok := config["injectOnce"].(bool); ok {
		n.config.InjectOnce = injectOnce
	}
	return n.Validate()
}

// Stop ends a running Start loop and releases the ticker. Idempotent.
func (n *InjectNode) Stop() {
	n.mu.Lock()
	if n.stopped {
		n.mu.Unlock()
		return
	}
	n.stopped = true
	close(n.done)
	ticker := n.ticker
	n.ticker = nil
	n.mu.Unlock()

	if ticker != nil {
		ticker.Stop()
	}
}

// Close implements registry.Closeable so the engine stops the interval
// when the flow is undeployed.
func (n *InjectNode) Close() error {
	n.Stop()
	return nil
}

var (
	_ registry.EmittingNode = (*InjectNode)(nil)
	_ registry.Closeable    = (*InjectNode)(nil)
)

func init() {
	reg := registry.GetGlobalRegistry()
	err := reg.RegisterFactory("inject", func() registry.NodeExecutor {
		return NewInjectNode()
	}, registry.NodeMetadata{
		ID:          "inject",
		Type:        "inject",
		Name:        "Inject",
		Description: "Injects a message into a flow",
		Category:    "input",
		Inputs: []registry.Port{
			{ID: "input", Name: "Input", Description: "Trigger input (optional)", Required: false},
		},
		Outputs: []registry.Port{
			{ID: "output", Name: "Output", Description: "Injected message", Required: true},
		},
		ConfigSchema: registry.Schema{
			Properties: map[string]registry.Property{
				"payload": {
					Type:        "object",
					Description: "The data to inject",
					Default:     map[string]interface{}{"payload": ""},
					Label:       "Message",
					Order:       1,
					Widget:      "json",
				},
				"topic": {
					Type:        "string",
					Description: "Optional topic for the message",
					Default:     "",
					Label:       "Topic",
					Order:       2,
					Widget:      "text",
				},
				"interval": {
					Type:        "number",
					Description: "Time between injections in ms (0 = manual)",
					Default:     0,
					Min:         floatPtr(0),
					Label:       "Repeat every",
					Order:       3,
					Widget:      "duration",
					Unit:        "ms",
				},
				"injectOnce": {
					Type:        "boolean",
					Description: "Inject once when the flow is deployed",
					Default:     false,
					Label:       "Inject once after deploy",
					Order:       4,
					Widget:      "boolean",
				},
			},
		},
		Help: "**Starts a flow by hand or on a timer.** Click the button on the node to inject the message now; set *Repeat every* to inject on an interval (0 = never). The message is given as JSON, e.g. `{\"payload\": \"tick\"}`.",
		Icon: "circle-play",
		Tags: []string{"input", "inject", "trigger"},
	})
	if err != nil {
		panic(err)
	}
}

func floatPtr(f float64) *float64 {
	return &f
}
