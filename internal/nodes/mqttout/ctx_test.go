package mqttout

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/GrimbiXcode/Go-RED/internal/registry"
	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// blockingToken never completes - it stands in for a publish the broker
// never acknowledges.
type blockingToken struct{}

func (blockingToken) Wait() bool                       { select {} }
func (blockingToken) WaitTimeout(d time.Duration) bool { time.Sleep(d); return false }
func (blockingToken) Done() <-chan struct{}            { return make(chan struct{}) }
func (blockingToken) Error() error                     { return nil }

// blockingClient is a fakeClient whose Publish never completes.
type blockingClient struct{ fakeClient }

func (c *blockingClient) Publish(string, byte, bool, interface{}) mqtt.Token { return blockingToken{} }

// blockingBroker is a fakeBroker that hands out a blockingClient.
type blockingBroker struct{ client *blockingClient }

func (b *blockingBroker) Client() mqtt.Client                 { return b.client }
func (b *blockingBroker) OnConnect(handler func(mqtt.Client)) { handler(b.client) }
func (b *blockingBroker) Execute(ctx interface{}, input map[string]interface{}) (map[string]interface{}, error) {
	return nil, nil
}
func (b *blockingBroker) Validate() error                               { return nil }
func (b *blockingBroker) GetConfig() map[string]interface{}             { return nil }
func (b *blockingBroker) SetConfig(config map[string]interface{}) error { return nil }

// A publish that the broker never acknowledges must not hold Execute for
// the full publishTimeout (10s) once the per-message context is cancelled:
// Execute returns promptly with an error wrapping context.Canceled.
func TestNode_Execute_CancelledContext_DoesNotWaitForPublish(t *testing.T) {
	broker := &blockingBroker{client: &blockingClient{}}
	rt := registry.NewNodeRuntime("f1", "mqttout-1", "mqtt out", nil, nil, nil, nil, func(nodeID string) (registry.NodeExecutor, bool) {
		if nodeID == "b1" {
			return broker, true
		}
		return nil, false
	})
	ctx, cancel := context.WithCancel(registry.WithRuntime(context.Background(), rt))
	defer cancel()

	go func() {
		time.Sleep(30 * time.Millisecond)
		cancel()
	}()

	n := &Node{Broker: "b1", Topic: "t"}
	start := time.Now()
	_, err := n.Execute(ctx, map[string]interface{}{"payload": "x"})
	elapsed := time.Since(start)

	require.Error(t, err)
	assert.True(t, errors.Is(err, context.Canceled), "error should wrap context.Canceled, got: %v", err)
	assert.Less(t, elapsed, time.Second)
}
