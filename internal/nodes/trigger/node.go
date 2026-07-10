// Package trigger provides the Trigger node implementation.
//
// Trigger sends FirstPayload immediately, and - if SecondPayload is
// configured (its Type is non-empty) - schedules a second send of
// SecondPayload after DelayMs, delivered back to this node's own output via
// registry.NodeRuntime.SubmitToNode(rt.NodeID, ...) so it goes out the same
// wires as a normal Execute result would.
//
// Scope cut vs. Node-RED (see docs/NODE_PALETTE_PLAN.md): there is no
// "reset" support - an incoming message never cancels another message's
// already-scheduled second send. Every triggering message gets its own,
// independent timer. registry.Closeable stops any timers still pending on
// Undeploy (or a failed Deploy); they are not tied to the per-Execute
// context, which is cancelled the moment Execute returns and so cannot be
// used to schedule anything that must outlive it.
package trigger

import (
    "context"
    "fmt"
    "sync"
    "time"

    "github.com/GrimbiXcode/Go-RED/internal/registry"
    "github.com/GrimbiXcode/Go-RED/internal/typedvalue"
)

// Node holds a Trigger node's configuration and its currently pending
// second-send timers.
type Node struct {
    // FirstPayload is resolved and written to the "payload" field of the
    // message sent immediately. A zero Value (Type == "") passes the
    // incoming message through unchanged.
    FirstPayload typedvalue.Value
    // SecondPayload, if Type is non-empty, is resolved and sent as a
    // separate message's "payload" field after DelayMs.
    SecondPayload typedvalue.Value
    DelayMs       int64

    mu     sync.Mutex
    closed bool
    timers []*time.Timer
}

// Execute sends FirstPayload immediately and, if configured, schedules
// SecondPayload for DelayMs later.
func (n *Node) Execute(ctx interface{}, input map[string]interface{}) (map[string]interface{}, error) {
    valueResolver := resolver(ctx, input)

    first := cloneMap(input)
    if n.FirstPayload.Type != "" {
        val, err := n.FirstPayload.Resolve(valueResolver)
        if err != nil {
            return nil, fmt.Errorf("trigger: first payload: %w", err)
        }
        first["payload"] = val
    }

    if n.SecondPayload.Type != "" {
        if rt, ok := runtimeFrom(ctx); ok {
            val, err := n.SecondPayload.Resolve(valueResolver)
            if err != nil {
                return nil, fmt.Errorf("trigger: second payload: %w", err)
            }
            second := cloneMap(input)
            second["payload"] = val
            n.scheduleSecond(rt, second)
        }
    }

    return first, nil
}

// scheduleSecond arranges for payload to be delivered to this node's own
// output after DelayMs, unless Close has already run (flow undeployed).
//
// Fired timers are intentionally not pruned from n.timers before Close runs
// (an earlier version tried to remove a timer from within its own callback,
// which raced on time.AfterFunc's return value: the callback goroutine can
// start before the assignment capturing it completes, for a near-zero
// delay). n.timers accumulates one entry per second-send scheduled since
// deploy; Stop on an already-fired timer is a safe no-op. For a flow with
// very high trigger volume over a very long deployment this is unbounded
// growth, not a leak in any dangerous sense - acceptable for now.
func (n *Node) scheduleSecond(rt *registry.NodeRuntime, payload map[string]interface{}) {
    n.mu.Lock()
    defer n.mu.Unlock()
    if n.closed {
        return
    }

    timer := time.AfterFunc(time.Duration(n.DelayMs)*time.Millisecond, func() {
        rt.SubmitToNode(rt.NodeID, payload)
    })
    n.timers = append(n.timers, timer)
}

// Close stops every timer that hasn't fired yet, so Undeploy doesn't leave
// pending sends for a flow that no longer exists.
func (n *Node) Close() error {
    n.mu.Lock()
    defer n.mu.Unlock()
    n.closed = true
    for _, t := range n.timers {
        t.Stop()
    }
    n.timers = nil
    return nil
}

func (n *Node) Validate() error {
    if n.DelayMs < 0 {
        return fmt.Errorf("trigger: delayMs cannot be negative")
    }
    return nil
}

func (n *Node) GetConfig() map[string]interface{} {
    return map[string]interface{}{
        "firstPayload":  valueToConfig(n.FirstPayload),
        "secondPayload": valueToConfig(n.SecondPayload),
        "delayMs":       n.DelayMs,
    }
}

func (n *Node) SetConfig(config map[string]interface{}) error {
    n.FirstPayload = parseValue(config["firstPayload"])
    if n.FirstPayload.Type == "" {
        n.FirstPayload = typedvalue.Value{Type: typedvalue.TypeMsg, Value: "payload"}
    }
    n.SecondPayload = parseValue(config["secondPayload"])

    n.DelayMs = 250
    if d, ok := config["delayMs"].(float64); ok {
        n.DelayMs = int64(d)
    }
    return n.Validate()
}

func runtimeFrom(ctx interface{}) (*registry.NodeRuntime, bool) {
    c, ok := ctx.(context.Context)
    if !ok {
        return nil, false
    }
    return registry.RuntimeFromContext(c)
}

// resolver builds the typedvalue.Resolver for this invocation, nil-safe
// against registry.NodeRuntime's FlowContext/GlobalContext being nil
// *registry.ContextStore pointers (see switchnode.resolvers for why this
// can't be a naive field assignment).
func resolver(ctx interface{}, input map[string]interface{}) typedvalue.Resolver {
    r := typedvalue.Resolver{Message: input}
    rt, ok := runtimeFrom(ctx)
    if !ok {
        return r
    }
    if rt.FlowContext != nil {
        r.FlowContext = rt.FlowContext
    }
    if rt.GlobalContext != nil {
        r.GlobalContext = rt.GlobalContext
    }
    return r
}

func cloneMap(src map[string]interface{}) map[string]interface{} {
    dst := make(map[string]interface{}, len(src))
    for k, v := range src {
        dst[k] = v
    }
    return dst
}

func parseValue(raw interface{}) typedvalue.Value {
    m, ok := raw.(map[string]interface{})
    if !ok {
        return typedvalue.Value{}
    }
    t, _ := m["type"].(string)
    v, _ := m["value"].(string)
    return typedvalue.Value{Type: typedvalue.Type(t), Value: v}
}

func valueToConfig(v typedvalue.Value) map[string]interface{} {
    return map[string]interface{}{"type": string(v.Type), "value": v.Value}
}

func init() {
    reg := registry.GetGlobalRegistry()
    err := reg.RegisterFactory("trigger", func() registry.NodeExecutor {
        return &Node{
            FirstPayload: typedvalue.Value{Type: typedvalue.TypeMsg, Value: "payload"},
            DelayMs:      250,
        }
    }, registry.NodeMetadata{
        ID:          "trigger",
        Type:        "trigger",
        Name:        "Trigger",
        Description: "Sends a message immediately, then optionally a second message after a delay",
        Category:    "function",
        Inputs: []registry.Port{
            {ID: "input", Name: "Input", Description: "Message that starts the trigger", Required: true},
        },
        Outputs: []registry.Port{
            {ID: "output", Name: "Output", Description: "Immediate message, and the delayed second message if configured", Required: true},
        },
        ConfigSchema: registry.Schema{
            Properties: map[string]registry.Property{
                "firstPayload":  {Type: "object", Description: `Payload to send immediately, e.g. {"type":"msg","value":"payload"} to pass through unchanged`, Default: map[string]interface{}{"type": "msg", "value": "payload"}},
                "secondPayload": {Type: "object", Description: `Payload to send after delayMs, e.g. {"type":"bool","value":"false"}; empty type means no second send`, Default: map[string]interface{}{}},
                "delayMs":       {Type: "number", Description: "Delay in milliseconds before the second send", Default: float64(250), Min: floatPtr(0)},
            },
        },
        Icon: `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="#00ADD8"><path d="M12 2a10 10 0 1010 10A10 10 0 0012 2zm1 15h-2v-2h2zm0-4h-2V7h2z"/></svg>`,
        Tags: []string{"function", "trigger", "timing"},
    })
    if err != nil {
        panic(err)
    }
}

func floatPtr(f float64) *float64 { return &f }
