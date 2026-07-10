// Package delay provides the Delay node implementation.
//
// Two modes: "delay" (the default) holds every message for a fixed
// duration before sending it on; "rate" queues messages and releases them
// at a steady rate (RateLimit messages per RateIntervalMs), either queuing
// every message (FIFO) or, if DropIntermediate is set, keeping only the
// most recently queued one - matching Node-RED's rate-limit node's "queue"
// vs. "just the latest message" modes. Node-RED's "random delay" and
// "delay each message additionally" variants are not implemented (see
// docs/NODE_PALETTE_PLAN.md).
//
// "delay" mode blocks synchronously inside Execute for the configured
// duration; since the engine runs every node invocation in its own
// goroutine, this only delays that one message, not the rest of the flow.
// "rate" mode instead uses registry.EmittingNode: ExecuteMulti only
// enqueues (sending on no port), and Start runs a ticker that dequeues and
// emits at the configured rate for as long as the flow is deployed.
package delay

import (
    "context"
    "fmt"
    "sync"
    "time"

    "github.com/GrimbiXcode/Go-RED/internal/registry"
)

// Node holds a Delay node's configuration and, in "rate" mode, its pending
// message queue.
type Node struct {
    // Mode is "delay" or "rate".
    Mode string
    // DelayMs is the fixed hold duration in "delay" mode.
    DelayMs int64
    // RateLimit messages are released per RateIntervalMs in "rate" mode.
    RateLimit      int
    RateIntervalMs int64
    // DropIntermediate, in "rate" mode, keeps only the most recently
    // queued message instead of queuing every one.
    DropIntermediate bool

    mu    sync.Mutex
    queue []map[string]interface{}
}

// ExecuteMulti either blocks for DelayMs and sends on "output" ("delay"
// mode), or enqueues input for later release by Start and sends on no port
// ("rate" mode).
func (n *Node) ExecuteMulti(ctx interface{}, input map[string]interface{}) (map[string]map[string]interface{}, error) {
    if n.Mode == "rate" {
        n.enqueue(input)
        return map[string]map[string]interface{}{}, nil
    }
    return n.executeDelay(ctx, input)
}

func (n *Node) enqueue(input map[string]interface{}) {
    n.mu.Lock()
    defer n.mu.Unlock()
    if n.DropIntermediate {
        n.queue = []map[string]interface{}{cloneMap(input)}
        return
    }
    n.queue = append(n.queue, cloneMap(input))
}

func (n *Node) executeDelay(ctx interface{}, input map[string]interface{}) (map[string]map[string]interface{}, error) {
    delay := time.Duration(n.DelayMs) * time.Millisecond
    if c, ok := ctx.(context.Context); ok {
        select {
        case <-time.After(delay):
        case <-c.Done():
            return nil, fmt.Errorf("delay: %w", c.Err())
        }
    } else {
        time.Sleep(delay)
    }
    return map[string]map[string]interface{}{"output": cloneMap(input)}, nil
}

// Start releases queued messages at the configured rate for as long as ctx
// is not cancelled. In "delay" mode there is nothing to release; Start
// simply waits for ctx to be cancelled.
func (n *Node) Start(ctx context.Context, emit func(map[string]interface{})) error {
    if n.Mode != "rate" {
        <-ctx.Done()
        return nil
    }

    ticker := time.NewTicker(n.releaseInterval())
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            return nil
        case <-ticker.C:
            if next, ok := n.dequeue(); ok {
                emit(next)
            }
        }
    }
}

func (n *Node) dequeue() (map[string]interface{}, bool) {
    n.mu.Lock()
    defer n.mu.Unlock()
    if len(n.queue) == 0 {
        return nil, false
    }
    next := n.queue[0]
    n.queue = n.queue[1:]
    return next, true
}

// releaseInterval spaces releases evenly across RateIntervalMs rather than
// releasing RateLimit messages in a burst then waiting.
func (n *Node) releaseInterval() time.Duration {
    limit := n.RateLimit
    if limit < 1 {
        limit = 1
    }
    interval := time.Duration(n.RateIntervalMs) * time.Millisecond / time.Duration(limit)
    if interval <= 0 {
        interval = time.Millisecond
    }
    return interval
}

// Execute exists only to satisfy registry.NodeExecutor (embedded in
// registry.MultiOutputExecutor) - the engine always calls ExecuteMulti for
// a node implementing it, never this.
func (n *Node) Execute(ctx interface{}, input map[string]interface{}) (map[string]interface{}, error) {
    outputs, err := n.ExecuteMulti(ctx, input)
    if err != nil {
        return nil, err
    }
    return outputs["output"], nil
}

func (n *Node) Validate() error {
    switch n.Mode {
    case "", "delay", "rate":
    default:
        return fmt.Errorf("delay: unsupported mode %q", n.Mode)
    }
    if n.DelayMs < 0 {
        return fmt.Errorf("delay: delayMs cannot be negative")
    }
    if n.RateLimit < 0 {
        return fmt.Errorf("delay: rateLimit cannot be negative")
    }
    if n.RateIntervalMs < 0 {
        return fmt.Errorf("delay: rateIntervalMs cannot be negative")
    }
    return nil
}

func (n *Node) GetConfig() map[string]interface{} {
    return map[string]interface{}{
        "mode":             n.Mode,
        "delayMs":          n.DelayMs,
        "rateLimit":        n.RateLimit,
        "rateIntervalMs":   n.RateIntervalMs,
        "dropIntermediate": n.DropIntermediate,
    }
}

func (n *Node) SetConfig(config map[string]interface{}) error {
    n.Mode = "delay"
    if m, ok := config["mode"].(string); ok && m != "" {
        n.Mode = m
    }
    n.DelayMs = 5000
    if d, ok := config["delayMs"].(float64); ok {
        n.DelayMs = int64(d)
    }
    n.RateLimit = 1
    if r, ok := config["rateLimit"].(float64); ok {
        n.RateLimit = int(r)
    }
    n.RateIntervalMs = 1000
    if ri, ok := config["rateIntervalMs"].(float64); ok {
        n.RateIntervalMs = int64(ri)
    }
    if di, ok := config["dropIntermediate"].(bool); ok {
        n.DropIntermediate = di
    }
    return n.Validate()
}

func cloneMap(src map[string]interface{}) map[string]interface{} {
    dst := make(map[string]interface{}, len(src))
    for k, v := range src {
        dst[k] = v
    }
    return dst
}

func init() {
    reg := registry.GetGlobalRegistry()
    err := reg.RegisterFactory("delay", func() registry.NodeExecutor {
        return &Node{Mode: "delay", DelayMs: 5000, RateLimit: 1, RateIntervalMs: 1000}
    }, registry.NodeMetadata{
        ID:          "delay",
        Type:        "delay",
        Name:        "Delay",
        Description: "Holds each message for a fixed delay, or releases queued messages at a steady rate",
        Category:    "function",
        Inputs: []registry.Port{
            {ID: "input", Name: "Input", Description: "Message to delay or rate-limit", Required: true},
        },
        Outputs: []registry.Port{
            {ID: "output", Name: "Output", Description: "Delayed or rate-limited message", Required: true},
        },
        ConfigSchema: registry.Schema{
            Properties: map[string]registry.Property{
                "mode":             {Type: "string", Description: "delay (fixed hold per message) or rate (steady release rate)", Default: "delay", Enum: []string{"delay", "rate"}},
                "delayMs":          {Type: "number", Description: "Delay in milliseconds (delay mode)", Default: float64(5000), Min: floatPtr(0)},
                "rateLimit":        {Type: "number", Description: "Messages released per rateIntervalMs (rate mode)", Default: float64(1), Min: floatPtr(1)},
                "rateIntervalMs":   {Type: "number", Description: "Interval in milliseconds rateLimit applies to (rate mode)", Default: float64(1000), Min: floatPtr(1)},
                "dropIntermediate": {Type: "boolean", Description: "rate mode: keep only the most recently queued message instead of queuing every one", Default: false},
            },
        },
        Icon: `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="#00ADD8"><path d="M12 2a10 10 0 1010 10A10 10 0 0012 2zm1 11h-6v-2h4V6h2z"/></svg>`,
        Tags: []string{"function", "delay", "rate-limit", "timing"},
    })
    if err != nil {
        panic(err)
    }
}

func floatPtr(f float64) *float64 { return &f }
