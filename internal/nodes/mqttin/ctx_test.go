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

// blockingToken never completes - it stands in for a SUBACK the broker
// never sends.
type blockingToken struct{}

func (blockingToken) Wait() bool                       { select {} }
func (blockingToken) WaitTimeout(d time.Duration) bool { time.Sleep(d); return false }
func (blockingToken) Done() <-chan struct{}            { return make(chan struct{}) }
func (blockingToken) Error() error                     { return nil }

// stallingClient is a fakeClient whose Subscribe never completes and which
// records Unsubscribe calls.
type stallingClient struct {
	fakeClient
	mu           sync.Mutex
	unsubscribed []string
}

func (c *stallingClient) Subscribe(topic string, qos byte, callback mqtt.MessageHandler) mqtt.Token {
	c.fakeClient.Subscribe(topic, qos, callback)
	return blockingToken{}
}

func (c *stallingClient) Unsubscribe(topics ...string) mqtt.Token {
	c.mu.Lock()
	c.unsubscribed = append(c.unsubscribed, topics...)
	c.mu.Unlock()
	return fakeToken{}
}

type stallingBroker struct{ client *stallingClient }

func (b *stallingBroker) Client() mqtt.Client                 { return b.client }
func (b *stallingBroker) OnConnect(handler func(mqtt.Client)) { handler(b.client) }
func (b *stallingBroker) Execute(ctx interface{}, input map[string]interface{}) (map[string]interface{}, error) {
	return nil, nil
}
func (b *stallingBroker) Validate() error                               { return nil }
func (b *stallingBroker) GetConfig() map[string]interface{}             { return nil }
func (b *stallingBroker) SetConfig(config map[string]interface{}) error { return nil }

// Cancelling the flow context must (1) unblock an OnConnect handler still
// waiting for its SUBACK so Start returns promptly, (2) unsubscribe the
// topic on the shared client, and (3) stop the message callback from
// emitting anything that arrives afterwards.
func TestNode_Start_CancelUnblocksSubscribeAndUnsubscribes(t *testing.T) {
	sc := &stallingClient{}
	broker := &stallingBroker{client: sc}
	rt := registry.NewNodeRuntime("f1", "mqttin-1", "mqtt in", nil, nil, nil, nil, func(nodeID string) (registry.NodeExecutor, bool) {
		if nodeID == "b1" {
			return broker, true
		}
		return nil, false
	})
	ctx, cancel := context.WithCancel(registry.WithRuntime(context.Background(), rt))
	defer cancel()

	n := &Node{Broker: "b1", Topic: "sensors/temp"}

	var mu sync.Mutex
	emitted := 0
	done := make(chan error, 1)
	go func() {
		done <- n.Start(ctx, func(map[string]interface{}) {
			mu.Lock()
			emitted++
			mu.Unlock()
		})
	}()

	var handler mqtt.MessageHandler
	require.Eventually(t, func() bool {
		_, _, handler = sc.snapshot()
		return handler != nil
	}, time.Second, 5*time.Millisecond)

	cancel()
	select {
	case err := <-done:
		assert.NoError(t, err)
	case <-time.After(time.Second):
		t.Fatal("Start did not return within 1s of cancellation while a subscribe was pending")
	}

	sc.mu.Lock()
	assert.Equal(t, []string{"sensors/temp"}, sc.unsubscribed)
	sc.mu.Unlock()

	handler(sc, fakeMessage{topic: "sensors/temp", payload: []byte("late")})
	mu.Lock()
	assert.Equal(t, 0, emitted, "nothing should be emitted after the flow context ended")
	mu.Unlock()
}
