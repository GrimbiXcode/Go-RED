package mqttin

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

// fakeToken is a mqtt.Token that is always already complete/successful.
type fakeToken struct{}

func (fakeToken) Wait() bool                       { return true }
func (fakeToken) WaitTimeout(time.Duration) bool    { return true }
func (fakeToken) Done() <-chan struct{}             { ch := make(chan struct{}); close(ch); return ch }
func (fakeToken) Error() error                      { return nil }

// fakeClient records the last Subscribe call so a test can drive the
// stored MessageHandler directly, simulating an incoming publish without a
// real broker. Mutex-protected since Start's OnConnect callback runs in a
// goroutine the test doesn't otherwise synchronize with.
type fakeClient struct {
    mu              sync.Mutex
    subscribedTopic string
    subscribedQoS   byte
    handler         mqtt.MessageHandler
}

func (c *fakeClient) IsConnected() bool      { return true }
func (c *fakeClient) IsConnectionOpen() bool { return true }
func (c *fakeClient) Connect() mqtt.Token    { return fakeToken{} }
func (c *fakeClient) Disconnect(quiesce uint) {}
func (c *fakeClient) Publish(string, byte, bool, interface{}) mqtt.Token { return fakeToken{} }
func (c *fakeClient) Subscribe(topic string, qos byte, callback mqtt.MessageHandler) mqtt.Token {
    c.mu.Lock()
    c.subscribedTopic = topic
    c.subscribedQoS = qos
    c.handler = callback
    c.mu.Unlock()
    return fakeToken{}
}
func (c *fakeClient) SubscribeMultiple(map[string]byte, mqtt.MessageHandler) mqtt.Token {
    return fakeToken{}
}
func (c *fakeClient) Unsubscribe(...string) mqtt.Token       { return fakeToken{} }
func (c *fakeClient) AddRoute(string, mqtt.MessageHandler)   {}
func (c *fakeClient) OptionsReader() mqtt.ClientOptionsReader { return mqtt.ClientOptionsReader{} }

func (c *fakeClient) snapshot() (topic string, qos byte, handler mqtt.MessageHandler) {
    c.mu.Lock()
    defer c.mu.Unlock()
    return c.subscribedTopic, c.subscribedQoS, c.handler
}

// fakeBroker implements mqttbroker.Broker without any real network
// connection - OnConnect fires the handler immediately with a fakeClient.
type fakeBroker struct {
    client *fakeClient
}

func (b *fakeBroker) Client() mqtt.Client { return b.client }
func (b *fakeBroker) OnConnect(handler func(mqtt.Client)) {
    handler(b.client)
}

// The remaining methods only exist to satisfy registry.NodeExecutor, since
// NodeRuntime.GetNode returns that interface - mqttin type-asserts the
// result to mqttbroker.Broker and never calls these.
func (b *fakeBroker) Execute(ctx interface{}, input map[string]interface{}) (map[string]interface{}, error) {
    return nil, nil
}
func (b *fakeBroker) Validate() error                             { return nil }
func (b *fakeBroker) GetConfig() map[string]interface{}           { return nil }
func (b *fakeBroker) SetConfig(config map[string]interface{}) error { return nil }

// fakeMessage implements mqtt.Message for driving a subscription's
// callback directly in tests.
type fakeMessage struct {
    topic    string
    payload  []byte
    qos      byte
    retained bool
}

func (m fakeMessage) Duplicate() bool   { return false }
func (m fakeMessage) Qos() byte         { return m.qos }
func (m fakeMessage) Retained() bool    { return m.retained }
func (m fakeMessage) Topic() string     { return m.topic }
func (m fakeMessage) MessageID() uint16 { return 0 }
func (m fakeMessage) Payload() []byte   { return m.payload }
func (m fakeMessage) Ack()              {}

func TestNode_ConfigRoundTrip(t *testing.T) {
    n := &Node{}
    require.NoError(t, n.SetConfig(map[string]interface{}{"broker": "b1", "topic": "sensors/#", "qos": float64(1)}))
    assert.Equal(t, "b1", n.Broker)
    assert.Equal(t, "sensors/#", n.Topic)
    assert.Equal(t, byte(1), n.QoS)
}

func TestNode_Validate(t *testing.T) {
    assert.Error(t, (&Node{}).Validate())
    assert.Error(t, (&Node{Broker: "b1"}).Validate())
    assert.Error(t, (&Node{Broker: "b1", Topic: "t", QoS: 5}).Validate())
    assert.NoError(t, (&Node{Broker: "b1", Topic: "t"}).Validate())
}

func TestNode_Execute_NotSupported(t *testing.T) {
    _, err := (&Node{}).Execute(nil, map[string]interface{}{})
    assert.Error(t, err)
}

func TestNode_Start_SubscribesAndEmitsOnMessage(t *testing.T) {
    fc := &fakeClient{}
    broker := &fakeBroker{client: fc}
    rt := registry.NewNodeRuntime("f1", "mqttin-1", "mqtt in", nil, nil, nil, nil, func(nodeID string) (registry.NodeExecutor, bool) {
        if nodeID == "b1" {
            return broker, true
        }
        return nil, false
    })
    ctx, cancel := context.WithCancel(registry.WithRuntime(context.Background(), rt))
    defer cancel()

    n := &Node{Broker: "b1", Topic: "sensors/temp", QoS: 1}

    var received map[string]interface{}
    done := make(chan struct{})
    go func() {
        _ = n.Start(ctx, func(payload map[string]interface{}) {
            received = payload
            close(done)
        })
    }()

    var topic string
    var qos byte
    var handler mqtt.MessageHandler
    require.Eventually(t, func() bool {
        topic, qos, handler = fc.snapshot()
        return handler != nil
    }, time.Second, 5*time.Millisecond)
    assert.Equal(t, "sensors/temp", topic)
    assert.Equal(t, byte(1), qos)

    handler(fc, fakeMessage{topic: "sensors/temp", payload: []byte("21.5"), qos: 1, retained: true})

    select {
    case <-done:
    case <-time.After(time.Second):
        t.Fatal("emit was never called")
    }
    assert.Equal(t, "sensors/temp", received["topic"])
    assert.Equal(t, "21.5", received["payload"])
    assert.Equal(t, float64(1), received["qos"])
    assert.Equal(t, true, received["retain"])
}

func TestNode_Start_NoRuntime_Errors(t *testing.T) {
    n := &Node{Broker: "b1", Topic: "t"}
    err := n.Start(context.Background(), func(map[string]interface{}) {})
    assert.Error(t, err)
}

func TestNode_Start_UnknownBroker_Errors(t *testing.T) {
    rt := registry.NewNodeRuntime("f1", "mqttin-1", "mqtt in", nil, nil, nil, nil, func(nodeID string) (registry.NodeExecutor, bool) {
        return nil, false
    })
    ctx := registry.WithRuntime(context.Background(), rt)
    n := &Node{Broker: "missing", Topic: "t"}
    err := n.Start(ctx, func(map[string]interface{}) {})
    assert.Error(t, err)
}
