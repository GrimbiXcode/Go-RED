package websocketout

import (
    "context"
    "sync"
    "testing"

    "github.com/GrimbiXcode/Go-RED/internal/registry"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

type fakeServer struct {
    mu       sync.Mutex
    sent     [][]byte
    sentText []bool
    sendErr  error
}

func (f *fakeServer) Send(data []byte, isText bool) error {
    f.mu.Lock()
    defer f.mu.Unlock()
    f.sent = append(f.sent, data)
    f.sentText = append(f.sentText, isText)
    return f.sendErr
}

func (f *fakeServer) Execute(ctx interface{}, input map[string]interface{}) (map[string]interface{}, error) {
    return nil, nil
}
func (f *fakeServer) Validate() error                             { return nil }
func (f *fakeServer) GetConfig() map[string]interface{}           { return nil }
func (f *fakeServer) SetConfig(config map[string]interface{}) error { return nil }

func runtimeWithServer(s *fakeServer) context.Context {
    rt := registry.NewNodeRuntime("f1", "wsout-1", "websocket out", nil, nil, nil, nil, func(nodeID string) (registry.NodeExecutor, bool) {
        if nodeID == "s1" {
            return s, true
        }
        return nil, false
    })
    return registry.WithRuntime(context.Background(), rt)
}

func TestNode_Validate(t *testing.T) {
    assert.Error(t, (&Node{}).Validate())
    assert.NoError(t, (&Node{Server: "s1"}).Validate())
}

func TestNode_ConfigRoundTrip(t *testing.T) {
    n := &Node{}
    require.NoError(t, n.SetConfig(map[string]interface{}{"server": "s1"}))
    assert.Equal(t, "s1", n.Server)
}

func TestNode_Execute_SendsStringAsText(t *testing.T) {
    fs := &fakeServer{}
    ctx := runtimeWithServer(fs)

    n := &Node{Server: "s1"}
    out, err := n.Execute(ctx, map[string]interface{}{"payload": "hello"})
    require.NoError(t, err)
    assert.Equal(t, "hello", out["payload"])

    fs.mu.Lock()
    defer fs.mu.Unlock()
    require.Len(t, fs.sent, 1)
    assert.Equal(t, []byte("hello"), fs.sent[0])
    assert.True(t, fs.sentText[0])
}

func TestNode_Execute_SendsBytesAsBinary(t *testing.T) {
    fs := &fakeServer{}
    ctx := runtimeWithServer(fs)

    n := &Node{Server: "s1"}
    _, err := n.Execute(ctx, map[string]interface{}{"payload": []byte{1, 2, 3}})
    require.NoError(t, err)

    fs.mu.Lock()
    defer fs.mu.Unlock()
    assert.Equal(t, []byte{1, 2, 3}, fs.sent[0])
    assert.False(t, fs.sentText[0])
}

func TestNode_Execute_SendsObjectAsJSON(t *testing.T) {
    fs := &fakeServer{}
    ctx := runtimeWithServer(fs)

    n := &Node{Server: "s1"}
    _, err := n.Execute(ctx, map[string]interface{}{"payload": map[string]interface{}{"a": float64(1)}})
    require.NoError(t, err)

    fs.mu.Lock()
    defer fs.mu.Unlock()
    assert.JSONEq(t, `{"a":1}`, string(fs.sent[0]))
    assert.True(t, fs.sentText[0])
}

func TestNode_Execute_SendError(t *testing.T) {
    fs := &fakeServer{sendErr: assertErr{}}
    ctx := runtimeWithServer(fs)

    n := &Node{Server: "s1"}
    _, err := n.Execute(ctx, map[string]interface{}{"payload": "x"})
    assert.Error(t, err)
}

type assertErr struct{}

func (assertErr) Error() string { return "send failed" }

func TestNode_Execute_NoRuntime_Errors(t *testing.T) {
    n := &Node{Server: "s1"}
    _, err := n.Execute(context.Background(), map[string]interface{}{"payload": "x"})
    assert.Error(t, err)
}

func TestNode_Execute_UnknownServer_Errors(t *testing.T) {
    ctx := runtimeWithServer(nil)
    n := &Node{Server: "wrong-id"}
    _, err := n.Execute(ctx, map[string]interface{}{"payload": "x"})
    assert.Error(t, err)
}
