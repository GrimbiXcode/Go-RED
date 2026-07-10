package engine

import (
    "net"
    "net/http"
    "os"
    "strings"
    "testing"
    "time"

    "github.com/GrimbiXcode/Go-RED/internal/nodes/httpin"
    "github.com/GrimbiXcode/Go-RED/internal/nodes/httprequest"
    "github.com/GrimbiXcode/Go-RED/internal/nodes/httpresponse"
    "github.com/GrimbiXcode/Go-RED/internal/nodes/tcpin"
    "github.com/GrimbiXcode/Go-RED/internal/registry"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

// This file demonstrates the Phase 6 milestone from docs/NODE_PALETTE_PLAN.md
// ("MQTT- und HTTP-Roundtrip-Flow gegen einen lokalen Test-Broker/-Server")
// against the real http in/http response/http request/tcp in node
// implementations - not mocks - deployed through a real FlowEngine, using
// an isolated registry (per internal/registry/AGENTS.md's testing
// guidance).
//
// A live MQTT roundtrip is not exercised here - it would need an actual
// broker process, which this sandboxed test environment doesn't have (see
// internal/nodes/mqttbroker/mqttin/mqttout's own unit tests, which instead
// exercise the same logic against paho.mqtt.golang's mqtt.Client interface
// via a hand-written fake, since it's a real Go interface, not a
// broker-side integration). A TCP roundtrip (real sockets, no external
// process needed) is used here as the second network-protocol milestone
// alongside HTTP.

func TestMain(m *testing.M) {
    os.Setenv("GORED_HTTP_NODE_PORT", "0")
    os.Exit(m.Run())
}

func TestPhase6_HTTPRoundtrip(t *testing.T) {
    reg := registry.NewNodeRegistry()
    require.NoError(t, reg.RegisterFactory("http in", func() registry.NodeExecutor { return &httpin.Node{} }, registry.NodeMetadata{Type: "http in"}))
    require.NoError(t, reg.RegisterFactory("http response", func() registry.NodeExecutor { return &httpresponse.Node{} }, registry.NodeMetadata{Type: "http response"}))
    require.NoError(t, reg.RegisterFactory("http request", func() registry.NodeExecutor { return &httprequest.Node{} }, registry.NodeMetadata{Type: "http request"}))
    require.NoError(t, reg.RegisterFactory("up", func() registry.NodeExecutor { return passthroughNode{} }, registry.NodeMetadata{Type: "up"}))

    resultSink := &captureSinkNode{}
    require.NoError(t, reg.RegisterFactory("result-sink", func() registry.NodeExecutor { return resultSink }, registry.NodeMetadata{Type: "result-sink"}))

    e := newTestEngine(reg)
    defer e.Stop()

    // Flow 1: an HTTP server endpoint that echoes back whatever body it
    // received.
    serverFlow := NewFlow("phase6-http-server", "Phase 6 HTTP Server")
    serverFlow.Nodes["listener"] = &Node{ID: "listener", Type: "http in", Config: map[string]interface{}{
        "method": "POST", "path": "/api/greet",
    }}
    serverFlow.Nodes["responder"] = &Node{ID: "responder", Type: "http response"}
    serverFlow.Connections = []NodeConnection{
        {ID: "c1", SourceNode: "listener", TargetNode: "responder"},
    }
    require.NoError(t, e.Deploy(serverFlow))
    defer e.Undeploy(serverFlow.ID)

    require.Eventually(t, func() bool { return httpin.SharedAddr() != "" }, 2*time.Second, 5*time.Millisecond)
    time.Sleep(50 * time.Millisecond) // let the route registration goroutine run

    addr := strings.Replace(httpin.SharedAddr(), "0.0.0.0", "127.0.0.1", 1)

    // Flow 2: an http request node that calls Flow 1's endpoint, deployed
    // and triggered separately - demonstrating http in/http response and
    // http request are independently useful, real, engine-deployed nodes,
    // not just unit-testable in isolation.
    clientFlow := NewFlow("phase6-http-client", "Phase 6 HTTP Client")
    clientFlow.Nodes["up"] = &Node{ID: "up", Type: "up"}
    clientFlow.Nodes["requester"] = &Node{ID: "requester", Type: "http request", Config: map[string]interface{}{
        "url":    map[string]interface{}{"type": "str", "value": "http://" + addr + "/api/greet"},
        "method": "POST",
    }}
    clientFlow.Nodes["resultSink"] = &Node{ID: "resultSink", Type: "result-sink"}
    clientFlow.Connections = []NodeConnection{
        {ID: "c1", SourceNode: "up", TargetNode: "requester"},
        {ID: "c2", SourceNode: "requester", TargetNode: "resultSink"},
    }
    require.NoError(t, e.Deploy(clientFlow))
    defer e.Undeploy(clientFlow.ID)

    require.NoError(t, e.InjectMessage(clientFlow.ID, "up", map[string]interface{}{"payload": "hello from client"}))

    require.Eventually(t, func() bool { return resultSink.receivedCount() >= 1 }, 2*time.Second, 10*time.Millisecond,
        "http request node should have completed the roundtrip and sent its output onward")
    last := resultSink.last()
    assert.Equal(t, float64(http.StatusOK), last["statusCode"])
    assert.Equal(t, "hello from client", last["payload"])
}

func TestPhase6_TCPRoundtrip(t *testing.T) {
    reg := registry.NewNodeRegistry()
    require.NoError(t, reg.RegisterFactory("tcp in", func() registry.NodeExecutor { return &tcpin.Node{} }, registry.NodeMetadata{Type: "tcp in"}))

    resultSink := &captureSinkNode{}
    require.NoError(t, reg.RegisterFactory("result-sink", func() registry.NodeExecutor { return resultSink }, registry.NodeMetadata{Type: "result-sink"}))

    e := newTestEngine(reg)
    defer e.Stop()

    ln, err := net.Listen("tcp", "127.0.0.1:0")
    require.NoError(t, err)
    port := ln.Addr().(*net.TCPAddr).Port
    require.NoError(t, ln.Close())

    flow := NewFlow("phase6-tcp", "Phase 6 TCP")
    flow.Nodes["listener"] = &Node{ID: "listener", Type: "tcp in", Config: map[string]interface{}{
        "port": float64(port), "server": true, "datatype": "utf8",
    }}
    flow.Nodes["resultSink"] = &Node{ID: "resultSink", Type: "result-sink"}
    flow.Connections = []NodeConnection{
        {ID: "c1", SourceNode: "listener", TargetNode: "resultSink"},
    }
    require.NoError(t, e.Deploy(flow))
    defer e.Undeploy(flow.ID)

    var conn net.Conn
    require.Eventually(t, func() bool {
        var dialErr error
        conn, dialErr = net.Dial("tcp", ln.Addr().String())
        return dialErr == nil
    }, 2*time.Second, 10*time.Millisecond)
    defer conn.Close()

    _, err = conn.Write([]byte("ping from a real socket"))
    require.NoError(t, err)

    require.Eventually(t, func() bool { return resultSink.receivedCount() >= 1 }, 2*time.Second, 10*time.Millisecond,
        "tcp in node should have emitted a message for the incoming connection's data")
    assert.Equal(t, "ping from a real socket", resultSink.last()["payload"])
}
