package mqttout

import (
    "context"
    "sync"
    "testing"
    "time"

    "github.com/GrimbiXcode/Go-RED/internal/registry"
    mqtt "github.com/eclipse/paho.mqtt.golang"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

type fakeToken struct{ err error }

func (t fakeToken) Wait() bool                    { return true }
func (t fakeToken) WaitTimeout(time.Duration) bool { return true }
func (t fakeToken) Done() <-chan struct{}          { ch := make(chan struct{}); close(ch); return ch }
func (t fakeToken) Error() error                   { return t.err }

// fakeClient records the last Publish call.
type fakeClient struct {
    mu              sync.Mutex
    publishedTopic  string
    publishedQoS    byte
    publishedRetain bool
    publishedData   interface{}
    publishErr      error
}

func (c *fakeClient) IsConnected() bool      { return true }
func (c *fakeClient) IsConnectionOpen() bool { return true }
func (c *fakeClient) Connect() mqtt.Token    { return fakeToken{} }
func (c *fakeClient) Disconnect(quiesce uint) {}
func (c *fakeClient) Publish(topic string, qos byte, retained bool, payload interface{}) mqtt.Token {
    c.mu.Lock()
    c.publishedTopic = topic
    c.publishedQoS = qos
    c.publishedRetain = retained
    c.publishedData = payload
    c.mu.Unlock()
    return fakeToken{err: c.publishErr}
}
func (c *fakeClient) Subscribe(string, byte, mqtt.MessageHandler) mqtt.Token { return fakeToken{} }
func (c *fakeClient) SubscribeMultiple(map[string]byte, mqtt.MessageHandler) mqtt.Token {
    return fakeToken{}
}
func (c *fakeClient) Unsubscribe(...string) mqtt.Token       { return fakeToken{} }
func (c *fakeClient) AddRoute(string, mqtt.MessageHandler)   {}
func (c *fakeClient) OptionsReader() mqtt.ClientOptionsReader { return mqtt.ClientOptionsReader{} }

type fakeBroker struct{ client *fakeClient }

func (b *fakeBroker) Client() mqtt.Client                 { return b.client }
func (b *fakeBroker) OnConnect(handler func(mqtt.Client)) { handler(b.client) }

func (b *fakeBroker) Execute(ctx interface{}, input map[string]interface{}) (map[string]interface{}, error) {
    return nil, nil
}
func (b *fakeBroker) Validate() error                             { return nil }
func (b *fakeBroker) GetConfig() map[string]interface{}           { return nil }
func (b *fakeBroker) SetConfig(config map[string]interface{}) error { return nil }

func runtimeWithBroker(broker *fakeBroker) context.Context {
    rt := registry.NewNodeRuntime("f1", "mqttout-1", "mqtt out", nil, nil, nil, nil, func(nodeID string) (registry.NodeExecutor, bool) {
        if nodeID == "b1" {
            return broker, true
        }
        return nil, false
    })
    return registry.WithRuntime(context.Background(), rt)
}

func TestNode_ConfigRoundTrip(t *testing.T) {
    n := &Node{}
    require.NoError(t, n.SetConfig(map[string]interface{}{"broker": "b1", "topic": "out/topic", "qos": float64(2), "retain": true}))
    assert.Equal(t, "b1", n.Broker)
    assert.Equal(t, "out/topic", n.Topic)
    assert.Equal(t, byte(2), n.QoS)
    assert.True(t, n.Retain)
}

func TestNode_Validate(t *testing.T) {
    assert.Error(t, (&Node{}).Validate())
    assert.Error(t, (&Node{Broker: "b1", QoS: 5}).Validate())
    assert.NoError(t, (&Node{Broker: "b1"}).Validate())
}

func TestNode_Execute_PublishesConfiguredTopic(t *testing.T) {
    fc := &fakeClient{}
    ctx := runtimeWithBroker(&fakeBroker{client: fc})

    n := &Node{Broker: "b1", Topic: "out/topic", QoS: 1, Retain: true}
    out, err := n.Execute(ctx, map[string]interface{}{"payload": "hello", "topic": "ignored"})
    require.NoError(t, err)
    assert.Equal(t, "hello", out["payload"])

    fc.mu.Lock()
    defer fc.mu.Unlock()
    assert.Equal(t, "out/topic", fc.publishedTopic)
    assert.Equal(t, byte(1), fc.publishedQoS)
    assert.True(t, fc.publishedRetain)
    assert.Equal(t, []byte("hello"), fc.publishedData)
}

func TestNode_Execute_FallsBackToMsgTopic(t *testing.T) {
    fc := &fakeClient{}
    ctx := runtimeWithBroker(&fakeBroker{client: fc})

    n := &Node{Broker: "b1"}
    _, err := n.Execute(ctx, map[string]interface{}{"payload": "x", "topic": "from-msg"})
    require.NoError(t, err)

    fc.mu.Lock()
    defer fc.mu.Unlock()
    assert.Equal(t, "from-msg", fc.publishedTopic)
}

func TestNode_Execute_NoTopic_Errors(t *testing.T) {
    fc := &fakeClient{}
    ctx := runtimeWithBroker(&fakeBroker{client: fc})

    n := &Node{Broker: "b1"}
    _, err := n.Execute(ctx, map[string]interface{}{"payload": "x"})
    assert.Error(t, err)
}

func TestNode_Execute_JSONPayload(t *testing.T) {
    fc := &fakeClient{}
    ctx := runtimeWithBroker(&fakeBroker{client: fc})

    n := &Node{Broker: "b1", Topic: "t"}
    _, err := n.Execute(ctx, map[string]interface{}{"payload": map[string]interface{}{"a": float64(1)}})
    require.NoError(t, err)

    fc.mu.Lock()
    defer fc.mu.Unlock()
    assert.JSONEq(t, `{"a":1}`, string(fc.publishedData.([]byte)))
}

func TestNode_Execute_PublishError(t *testing.T) {
    fc := &fakeClient{publishErr: assertErr{}}
    ctx := runtimeWithBroker(&fakeBroker{client: fc})

    n := &Node{Broker: "b1", Topic: "t"}
    _, err := n.Execute(ctx, map[string]interface{}{"payload": "x"})
    assert.Error(t, err)
}

type assertErr struct{}

func (assertErr) Error() string { return "publish failed" }

func TestNode_Execute_NoRuntime_Errors(t *testing.T) {
    n := &Node{Broker: "b1", Topic: "t"}
    _, err := n.Execute(context.Background(), map[string]interface{}{"payload": "x"})
    assert.Error(t, err)
}

func TestNode_Execute_UnknownBroker_Errors(t *testing.T) {
    ctx := runtimeWithBroker(nil)
    n := &Node{Broker: "wrong-id", Topic: "t"}
    _, err := n.Execute(ctx, map[string]interface{}{"payload": "x"})
    assert.Error(t, err)
}
