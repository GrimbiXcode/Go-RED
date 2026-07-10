package mqttbroker

import (
    "testing"

    mqtt "github.com/eclipse/paho.mqtt.golang"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

// fakeMQTTClient is a minimal mqtt.Client stub - just enough to satisfy the
// interface for tests that only exercise Node's OnConnect fan-out logic,
// never an actual network connection.
type fakeMQTTClient struct{}

func (fakeMQTTClient) IsConnected() bool                          { return true }
func (fakeMQTTClient) IsConnectionOpen() bool                     { return true }
func (fakeMQTTClient) Connect() mqtt.Token                        { return nil }
func (fakeMQTTClient) Disconnect(quiesce uint)                    {}
func (fakeMQTTClient) Publish(string, byte, bool, interface{}) mqtt.Token { return nil }
func (fakeMQTTClient) Subscribe(string, byte, mqtt.MessageHandler) mqtt.Token { return nil }
func (fakeMQTTClient) SubscribeMultiple(map[string]byte, mqtt.MessageHandler) mqtt.Token {
    return nil
}
func (fakeMQTTClient) Unsubscribe(...string) mqtt.Token       { return nil }
func (fakeMQTTClient) AddRoute(string, mqtt.MessageHandler)   {}
func (fakeMQTTClient) OptionsReader() mqtt.ClientOptionsReader { return mqtt.ClientOptionsReader{} }

func TestNode_ConfigRoundTrip(t *testing.T) {
    n := &Node{}
    require.NoError(t, n.SetConfig(map[string]interface{}{
        "url":                "tcp://localhost:1883",
        "clientId":           "my-client",
        "username":           "u",
        "password":           "p",
        "cleanSession":       false,
        "keepAliveSec":       float64(30),
        "useTLS":             true,
        "insecureSkipVerify": true,
    }))
    assert.Equal(t, "tcp://localhost:1883", n.URL)
    assert.Equal(t, "my-client", n.ClientID)
    assert.Equal(t, "u", n.Username)
    assert.Equal(t, "p", n.Password)
    assert.False(t, n.CleanSession)
    assert.Equal(t, 30, n.KeepAliveSec)
    assert.True(t, n.UseTLS)
    assert.True(t, n.InsecureSkipVerify)
    assert.NotNil(t, n.Client(), "SetConfig should have built the underlying client")
    require.NoError(t, n.Close())
}

func TestNode_Validate(t *testing.T) {
    assert.Error(t, (&Node{}).Validate())
    assert.NoError(t, (&Node{URL: "tcp://localhost:1883"}).Validate())
}

func TestNode_OnConnect_FiresOnceAlreadyConnected(t *testing.T) {
    n := &Node{}
    client := fakeMQTTClient{}
    n.mu.Lock()
    n.client = client
    n.connected = true
    n.mu.Unlock()

    var got mqtt.Client
    n.OnConnect(func(c mqtt.Client) { got = c })

    assert.Equal(t, client, got)
}

func TestNode_OnConnect_FiresOnHandleConnect(t *testing.T) {
    n := &Node{}
    var calls int
    n.OnConnect(func(c mqtt.Client) { calls++ })
    n.OnConnect(func(c mqtt.Client) { calls++ })

    assert.Equal(t, 0, calls, "not connected yet - handlers shouldn't have fired")

    n.handleConnect(fakeMQTTClient{})
    assert.Equal(t, 2, calls, "both handlers should fire once connected")

    n.handleConnect(fakeMQTTClient{})
    assert.Equal(t, 4, calls, "handlers fire again on every (re)connect")
}

func TestNode_HandleConnectionLost_ResetsConnectedState(t *testing.T) {
    n := &Node{}
    n.handleConnect(fakeMQTTClient{})
    n.mu.Lock()
    assert.True(t, n.connected)
    n.mu.Unlock()

    n.handleConnectionLost(fakeMQTTClient{}, assertError{})
    n.mu.Lock()
    assert.False(t, n.connected)
    n.mu.Unlock()

    calls := 0
    n.OnConnect(func(c mqtt.Client) { calls++ })
    assert.Equal(t, 0, calls, "should not fire immediately - connection was marked lost")
}

type assertError struct{}

func (assertError) Error() string { return "connection lost" }

func TestNode_Close_NoClient_NoPanic(t *testing.T) {
    n := &Node{}
    assert.NotPanics(t, func() {
        require.NoError(t, n.Close())
    })
}

func TestNode_Execute_NotSupported(t *testing.T) {
    _, err := (&Node{}).Execute(nil, map[string]interface{}{})
    assert.Error(t, err)
}
