// Package inject provides the Inject node implementation.
// The Inject node is used to inject messages into a flow at specific intervals or manually.
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

    // mu guards ticker, lastPayload, and stopped - startIntervalInjection
    // runs in its own goroutine (spawned from Execute) and races with
    // Stop()/Execute() calls from other goroutines otherwise.
    mu sync.Mutex

    // ticker is used for interval-based injection
    ticker *time.Ticker

    // done is used to stop the ticker
    done chan struct{}

    // stopped guards against closing done twice (Stop is not idempotent
    // otherwise: close of a closed channel panics).
    stopped bool

    // lastPayload stores the last injected payload
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
    
    // InjectOnce indicates whether to inject only once at startup
    InjectOnce bool `json:"injectOnce"`
}

// NewInjectNode creates a new InjectNode with default configuration.
func NewInjectNode() *InjectNode {
    return &InjectNode{
        config: InjectConfig{
            Payload:    map[string]interface{}{"payload": ""},
            Interval:  0,
            Topic:     "",
            InjectOnce: false,
        },
        done: make(chan struct{}),
    }
}

// Execute processes the input message and returns output.
func (n *InjectNode) Execute(ctx interface{}, input map[string]interface{}) (map[string]interface{}, error) {
    if input != nil {
        n.mu.Lock()
        n.lastPayload = input
        n.mu.Unlock()
    }

    if n.config.Interval > 0 {
        go n.startIntervalInjection(ctx.(context.Context))
    }

    if n.config.InjectOnce {
        return n.injectPayload(ctx.(context.Context))
    }

    return n.injectPayload(ctx.(context.Context))
}

// startIntervalInjection runs in its own goroutine (spawned by Execute).
// It keeps the *time.Ticker in a local variable for the select loop -
// never re-reading n.ticker after the initial assignment - so a
// concurrent Stop() clearing n.ticker can't race a nil dereference here.
// n.ticker itself is still kept in sync (mutex-guarded) purely so
// external callers/tests can observe whether a ticker is currently
// running.
func (n *InjectNode) startIntervalInjection(ctx context.Context) {
    interval := time.Duration(n.config.Interval) * time.Millisecond
    ticker := time.NewTicker(interval)
    defer ticker.Stop()

    n.mu.Lock()
    n.ticker = ticker
    n.mu.Unlock()

    for {
        select {
        case <-ctx.Done():
            return
        case <-n.done:
            return
        case <-ticker.C:
            n.mu.Lock()
            n.lastPayload = n.config.Payload
            n.mu.Unlock()
        }
    }
}

func (n *InjectNode) injectPayload(ctx context.Context) (map[string]interface{}, error) {
    n.mu.Lock()
    payload := n.config.Payload
    if n.lastPayload != nil {
        payload = n.lastPayload
    }
    n.mu.Unlock()

    if n.config.Topic != "" {
        if payload == nil {
            payload = make(map[string]interface{})
        }
        payload["topic"] = n.config.Topic
    }

    return payload, nil
}

// Ticker returns the currently running ticker, if any, for tests/callers
// that need to observe interval-injection state without racing
// startIntervalInjection's own goroutine.
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
        "interval":  n.config.Interval,
        "topic":     n.config.Topic,
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

// Stop signals startIntervalInjection's goroutine (if any) to exit and
// releases the ticker. Idempotent - a second call is a no-op rather than
// panicking on a double-close of done.
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
                "payload":    {Type: "object", Description: "The data to inject", Default: map[string]interface{}{"payload": ""}},
                "interval":  {Type: "number", Description: "Time between injections in ms (0 = manual)", Default: 0, Min: floatPtr(0)},
                "topic":     {Type: "string", Description: "Optional topic for the message", Default: ""},
                "injectOnce": {Type: "boolean", Description: "Inject only once at startup", Default: false},
            },
        },
        Icon: `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="#4CAF50"><path d="M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm-2 15l-5-5 1.41-1.41L10 14.17l7.59-7.59L19 8l-9 9z"/></svg>`,
        Tags: []string{"input", "inject", "trigger"},
    })
    if err != nil {
        panic(err)
    }
}

func floatPtr(f float64) *float64 {
    return &f
}
