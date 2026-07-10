// Package batch provides the Batch node implementation.
//
// Batch groups incoming messages into one combined message whose payload
// is an array of the buffered payloads, in one of two modes:
//   - "count" (the default): flush once Count messages have arrived.
//   - "interval": flush whatever is buffered every IntervalMs, via
//     registry.EmittingNode (the same ticker-and-flush shape
//     internal/nodes/delay's rate mode uses).
//
// Node-RED's overlap option (repeating some messages across consecutive
// batches) is not implemented (see docs/NODE_PALETTE_PLAN.md, Phase 4).
package batch

import (
    "context"
    "fmt"
    "sync"
    "time"

    "github.com/GrimbiXcode/Go-RED/internal/registry"
)

// Node holds a Batch node's configuration and buffer.
type Node struct {
    // Mode is "count" or "interval".
    Mode string
    // Count is the buffer size that triggers a flush in "count" mode.
    Count int
    // IntervalMs is the flush period in "interval" mode.
    IntervalMs int64

    mu     sync.Mutex
    buffer []map[string]interface{}
}

// ExecuteMulti buffers input. In "count" mode it sends a combined message
// on "output" once Count messages have accumulated (and on no port
// otherwise); in "interval" mode it always sends on no port - Start flushes
// the buffer on its own schedule instead.
func (n *Node) ExecuteMulti(ctx interface{}, input map[string]interface{}) (map[string]map[string]interface{}, error) {
    if n.Mode == "interval" {
        n.mu.Lock()
        n.buffer = append(n.buffer, cloneMap(input))
        n.mu.Unlock()
        return map[string]map[string]interface{}{}, nil
    }

    n.mu.Lock()
    n.buffer = append(n.buffer, cloneMap(input))
    var flushed []map[string]interface{}
    if len(n.buffer) >= n.Count {
        flushed = n.buffer
        n.buffer = nil
    }
    n.mu.Unlock()

    if flushed == nil {
        return map[string]map[string]interface{}{}, nil
    }
    return map[string]map[string]interface{}{"output": assemble(flushed)}, nil
}

// Start flushes whatever is buffered every IntervalMs, in "interval" mode
// only. In "count" mode there's nothing to do on a schedule; Start simply
// waits for ctx to be cancelled.
func (n *Node) Start(ctx context.Context, emit func(map[string]interface{})) error {
    if n.Mode != "interval" {
        <-ctx.Done()
        return nil
    }

    ticker := time.NewTicker(time.Duration(n.IntervalMs) * time.Millisecond)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            return nil
        case <-ticker.C:
            n.mu.Lock()
            flushed := n.buffer
            n.buffer = nil
            n.mu.Unlock()
            if len(flushed) > 0 {
                emit(assemble(flushed))
            }
        }
    }
}

func assemble(msgs []map[string]interface{}) map[string]interface{} {
    payloads := make([]interface{}, len(msgs))
    for i, m := range msgs {
        payloads[i] = m["payload"]
    }
    // The combined message otherwise takes its non-payload fields (topic,
    // etc.) from the first buffered message, same as Join does for its
    // reassembled message.
    out := cloneMap(msgs[0])
    out["payload"] = payloads
    return out
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
    case "", "count", "interval":
    default:
        return fmt.Errorf("batch: unsupported mode %q", n.Mode)
    }
    if n.Count < 0 {
        return fmt.Errorf("batch: count cannot be negative")
    }
    if n.IntervalMs < 0 {
        return fmt.Errorf("batch: intervalMs cannot be negative")
    }
    return nil
}

func (n *Node) GetConfig() map[string]interface{} {
    return map[string]interface{}{
        "mode":       n.Mode,
        "count":      n.Count,
        "intervalMs": n.IntervalMs,
    }
}

func (n *Node) SetConfig(config map[string]interface{}) error {
    n.Mode = "count"
    if m, ok := config["mode"].(string); ok && m != "" {
        n.Mode = m
    }
    n.Count = 10
    if c, ok := config["count"].(float64); ok {
        n.Count = int(c)
    }
    n.IntervalMs = 1000
    if i, ok := config["intervalMs"].(float64); ok {
        n.IntervalMs = int64(i)
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
    err := reg.RegisterFactory("batch", func() registry.NodeExecutor {
        return &Node{Mode: "count", Count: 10, IntervalMs: 1000}
    }, registry.NodeMetadata{
        ID:          "batch",
        Type:        "batch",
        Name:        "Batch",
        Description: "Groups messages into one combined message, by count or on a fixed interval",
        Category:    "flow-control",
        Inputs: []registry.Port{
            {ID: "input", Name: "Input", Description: "Message to buffer", Required: true},
        },
        Outputs: []registry.Port{
            {ID: "output", Name: "Output", Description: "Combined message: payload is an array of the buffered payloads", Required: true},
        },
        ConfigSchema: registry.Schema{
            Properties: map[string]registry.Property{
                "mode":       {Type: "string", Description: "count (flush at N messages) or interval (flush every intervalMs)", Default: "count", Enum: []string{"count", "interval"}},
                "count":      {Type: "number", Description: "Messages per batch (count mode)", Default: float64(10), Min: floatPtr(1)},
                "intervalMs": {Type: "number", Description: "Flush period in milliseconds (interval mode)", Default: float64(1000), Min: floatPtr(1)},
            },
        },
        Icon: `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="#00ADD8"><path d="M3 3h6v6H3zm8 0h6v6h-6zm8 0h2v6h-2zM3 11h6v6H3zm8 0h6v6h-6zm8 0h2v6h-2z"/></svg>`,
        Tags: []string{"flow-control", "batch", "sequence"},
    })
    if err != nil {
        panic(err)
    }
}

func floatPtr(f float64) *float64 { return &f }
