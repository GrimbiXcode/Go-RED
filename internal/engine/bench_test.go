package engine

import (
	"fmt"
	"runtime"
	"sync/atomic"
	"testing"
	"time"

	_ "github.com/GrimbiXcode/Go-RED/internal/nodes/debug"
	_ "github.com/GrimbiXcode/Go-RED/internal/nodes/function"
	_ "github.com/GrimbiXcode/Go-RED/internal/nodes/join"
	_ "github.com/GrimbiXcode/Go-RED/internal/nodes/split"
	_ "github.com/GrimbiXcode/Go-RED/internal/nodes/switchnode"
	"github.com/GrimbiXcode/Go-RED/internal/registry"
)

// The benchmarks run real node implementations through a real engine and
// measure end-to-end throughput: messages are injected at a source node and
// counted at a sink node at the far end of the flow. docs/PERFORMANCE.md
// records the numbers; run them with
//
//	go test ./internal/engine -run '^$' -bench . -benchtime 20000x

// benchSink counts arrivals and signals once the expected number is in.
type benchSink struct {
	count  atomic.Int64
	target atomic.Int64
	done   chan struct{}
}

func (n *benchSink) Execute(ctx interface{}, input map[string]interface{}) (map[string]interface{}, error) {
	if n.count.Add(1) == n.target.Load() {
		select {
		case n.done <- struct{}{}:
		default:
		}
	}
	return nil, nil
}
func (n *benchSink) Validate() error                               { return nil }
func (n *benchSink) GetConfig() map[string]interface{}             { return nil }
func (n *benchSink) SetConfig(config map[string]interface{}) error { return nil }

func (n *benchSink) reset(target int64) {
	n.count.Store(0)
	n.target.Store(target)
	select {
	case <-n.done:
	default:
	}
}

func (n *benchSink) wait(b *testing.B) {
	select {
	case <-n.done:
	case <-time.After(60 * time.Second):
		b.Fatalf("sink received %d of %d messages", n.count.Load(), n.target.Load())
	}
}

var benchRegistryReady atomic.Bool

// benchRegistry is the global registry (the real nodes register there on
// import) plus the benchmark's own source and sink types.
func benchRegistry(b *testing.B, sink *benchSink) *registry.NodeRegistry {
	reg := registry.GetGlobalRegistry()
	if benchRegistryReady.CompareAndSwap(false, true) {
		if err := reg.RegisterFactory("bench-source", func() registry.NodeExecutor { return passthroughNode{} }, registry.NodeMetadata{Type: "bench-source"}); err != nil {
			b.Fatal(err)
		}
	}
	// The sink factory is re-registered per benchmark so each run counts
	// into its own sink.
	_ = reg.Unregister("bench-sink")
	if err := reg.RegisterFactory("bench-sink", func() registry.NodeExecutor { return sink }, registry.NodeMetadata{Type: "bench-sink"}); err != nil {
		b.Fatal(err)
	}
	return reg
}

func benchEngine(b *testing.B, reg *registry.NodeRegistry) *FlowEngine {
	e := NewFlowEngine(EngineConfig{MessageBufferSize: 4096, DefaultTimeout: 30 * time.Second}, reg)
	if err := e.Start(); err != nil {
		b.Fatal(err)
	}
	return e
}

// benchWindow is how many injected messages may be outstanding (not yet
// counted at the far end) at once. The engine drops rather than blocks when
// a flow queue overflows, so the producer paces itself: this keeps the
// measurement a closed loop instead of a burst that the queue cannot hold.
const benchWindow = 256

// inject pushes n messages through the source node, never more than
// benchWindow ahead of done (the number of messages that completed).
func inject(b *testing.B, e *FlowEngine, flowID string, n int, done func() int64, payload func(i int) map[string]interface{}) {
	for i := 0; i < n; i++ {
		for int64(i)-done() >= benchWindow {
			time.Sleep(20 * time.Microsecond)
		}
		for {
			if err := e.InjectMessage(flowID, "src", payload(i)); err == nil {
				break
			}
			runtime.Gosched()
		}
	}
}

func runBench(b *testing.B, e *FlowEngine, flow *Flow, sink *benchSink, perMessage int64, payload func(i int) map[string]interface{}) {
	if err := e.Deploy(flow); err != nil {
		b.Fatal(err)
	}
	defer e.Undeploy(flow.ID)

	done := func() int64 { return sink.count.Load() / perMessage }

	// Warm up (JavaScript VMs, first allocations) outside the timer.
	sink.reset(perMessage * 10)
	inject(b, e, flow.ID, 10, done, payload)
	sink.wait(b)

	sink.reset(perMessage * int64(b.N))
	b.ResetTimer()
	start := time.Now()
	inject(b, e, flow.ID, b.N, done, payload)
	sink.wait(b)
	elapsed := time.Since(start)
	b.StopTimer()
	b.ReportMetric(float64(b.N)/elapsed.Seconds(), "msg/s")
	if dropped := e.MessagesDropped(); dropped > 0 {
		b.Fatalf("%d messages dropped", dropped)
	}
}

func node(id, typ string, config map[string]interface{}) *Node {
	if config == nil {
		config = map[string]interface{}{}
	}
	return &Node{ID: id, Type: typ, Config: config}
}

func wire(from, fromPort, to string) NodeConnection {
	return NodeConnection{ID: from + "-" + fromPort + "-" + to, SourceNode: from, SourcePort: fromPort, TargetNode: to, TargetPort: "input"}
}

// BenchmarkFunctionChain: source → function (JavaScript) → sink.
func BenchmarkFunctionChain(b *testing.B) {
	sink := &benchSink{done: make(chan struct{}, 1)}
	e := benchEngine(b, benchRegistry(b, sink))
	defer e.Stop()

	flow := NewFlow("bench-function", "bench")
	flow.Nodes["src"] = node("src", "bench-source", nil)
	flow.Nodes["fn"] = node("fn", "function", map[string]interface{}{"code": "msg.count = (msg.count || 0) + 1; return msg;", "useMsg": true})
	flow.Nodes["sink"] = node("sink", "bench-sink", nil)
	flow.Connections = []NodeConnection{wire("src", "output", "fn"), wire("fn", "output", "sink")}

	runBench(b, e, flow, sink, 1, func(i int) map[string]interface{} { return map[string]interface{}{"payload": i} })
}

// BenchmarkInjectFunctionDebug: source → function → debug, the flow every
// user builds first. The debug node has no output, so completion is read
// from the engine's per-node counters.
func BenchmarkInjectFunctionDebug(b *testing.B) {
	sink := &benchSink{done: make(chan struct{}, 1)}
	e := benchEngine(b, benchRegistry(b, sink))
	defer e.Stop()

	flow := NewFlow("bench-debug", "bench")
	flow.Nodes["src"] = node("src", "bench-source", nil)
	flow.Nodes["fn"] = node("fn", "function", map[string]interface{}{"code": "msg.count = (msg.count || 0) + 1; return msg;", "useMsg": true})
	flow.Nodes["dbg"] = node("dbg", "debug", nil)
	flow.Connections = []NodeConnection{wire("src", "output", "fn"), wire("fn", "output", "dbg")}
	if err := e.Deploy(flow); err != nil {
		b.Fatal(err)
	}
	defer e.Undeploy(flow.ID)

	payload := func(i int) map[string]interface{} { return map[string]interface{}{"payload": i} }
	handled := func() uint64 { return e.GetMetrics(flow.ID)["dbg"].Messages }
	waitFor := func(n uint64) {
		deadline := time.Now().Add(60 * time.Second)
		for handled() < n {
			if time.Now().After(deadline) {
				b.Fatalf("debug node handled %d of %d messages", handled(), n)
			}
			time.Sleep(time.Millisecond)
		}
	}
	inject(b, e, flow.ID, 10, func() int64 { return int64(handled()) }, payload)
	waitFor(10)

	b.ResetTimer()
	start := time.Now()
	inject(b, e, flow.ID, b.N, func() int64 { return int64(handled()) - 10 }, payload)
	waitFor(uint64(10 + b.N))
	elapsed := time.Since(start)
	b.StopTimer()
	b.ReportMetric(float64(b.N)/elapsed.Seconds(), "msg/s")
	if dropped := e.MessagesDropped(); dropped > 0 {
		b.Fatalf("%d messages dropped", dropped)
	}
}

// BenchmarkSwitchFanout: source → switch with three matching rules → three
// sinks; every input message produces three deliveries.
func BenchmarkSwitchFanout(b *testing.B) {
	sink := &benchSink{done: make(chan struct{}, 1)}
	e := benchEngine(b, benchRegistry(b, sink))
	defer e.Stop()

	rule := map[string]interface{}{"operator": "eq", "value": map[string]interface{}{"type": "str", "value": "a"}}
	flow := NewFlow("bench-switch", "bench")
	flow.Nodes["src"] = node("src", "bench-source", nil)
	flow.Nodes["sw"] = node("sw", "switch", map[string]interface{}{
		"property": map[string]interface{}{"type": "msg", "path": "payload"},
		"rules":    []interface{}{rule, rule, rule},
		"checkAll": true,
	})
	flow.Connections = []NodeConnection{wire("src", "output", "sw")}
	for i := 0; i < 3; i++ {
		id := fmt.Sprintf("sink%d", i)
		flow.Nodes[id] = node(id, "bench-sink", nil)
		flow.Connections = append(flow.Connections, wire("sw", fmt.Sprint(i), id))
	}

	runBench(b, e, flow, sink, 3, func(i int) map[string]interface{} { return map[string]interface{}{"payload": "a"} })
}

// BenchmarkSplitJoin: source → split (10 elements) → join → sink; one input
// array becomes ten messages and one joined array again.
func BenchmarkSplitJoin(b *testing.B) {
	sink := &benchSink{done: make(chan struct{}, 1)}
	e := benchEngine(b, benchRegistry(b, sink))
	defer e.Stop()

	flow := NewFlow("bench-splitjoin", "bench")
	flow.Nodes["src"] = node("src", "bench-source", nil)
	flow.Nodes["split"] = node("split", "split", nil)
	flow.Nodes["join"] = node("join", "join", nil)
	flow.Nodes["sink"] = node("sink", "bench-sink", nil)
	flow.Connections = []NodeConnection{wire("src", "output", "split"), wire("split", "output", "join"), wire("join", "output", "sink")}

	items := make([]interface{}, 10)
	for i := range items {
		items[i] = i
	}
	runBench(b, e, flow, sink, 1, func(i int) map[string]interface{} { return map[string]interface{}{"payload": items} })
}
