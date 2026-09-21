// Package mqttin provides the "mqtt in" node implementation - subscribes
// to a topic on a shared mqtt-broker config node (internal/nodes/mqttbroker)
// and emits one message per received publish.
package mqttin

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/GrimbiXcode/Go-RED/internal/nodes/base"
	"github.com/GrimbiXcode/Go-RED/internal/nodes/mqttbroker"
	"github.com/GrimbiXcode/Go-RED/internal/registry"
	mqtt "github.com/eclipse/paho.mqtt.golang"
)

const (
	// subscribeTimeout bounds how long an OnConnect handler waits for the
	// broker's SUBACK before giving up on that attempt (the flow context
	// can cut it shorter).
	subscribeTimeout = 10 * time.Second
	// unsubscribeTimeout bounds how long Start waits for the UNSUBACK when
	// the flow is undeployed, so Undeploy cannot hang on a wedged broker.
	unsubscribeTimeout = time.Second
)

// Node holds an MQTT-in node's configuration.
type Node struct {
	// Broker is the mqtt-broker config node's ID.
	Broker string
	Topic  string
	QoS    byte

	mu sync.Mutex
	// client is the paho client the subscription was last applied on, so
	// Start can unsubscribe from it when the flow context ends.
	client mqtt.Client
}

// Start resolves Broker via registry.NodeRuntime.GetNode, then registers a
// subscription (through mqttbroker.Broker.OnConnect, so it is (re)applied
// on every connect) that emits one message per received publish, until ctx
// is cancelled - at which point the subscription is removed again and no
// further message is emitted, even if the shared broker connection stays
// up for other flows.
func (n *Node) Start(ctx context.Context, emit func(payload map[string]interface{})) error {
	rt, ok := registry.RuntimeFromContext(ctx)
	if !ok {
		return fmt.Errorf("mqtt in: no runtime available")
	}
	exec, ok := rt.GetNode(n.Broker)
	if !ok {
		return fmt.Errorf("mqtt in: broker config node %q not found", n.Broker)
	}
	broker, ok := exec.(mqttbroker.Broker)
	if !ok {
		return fmt.Errorf("mqtt in: node %q is not an mqtt-broker", n.Broker)
	}

	rt.ReportStatus("connecting", "")
	broker.OnConnect(func(client mqtt.Client) {
		if ctx.Err() != nil {
			return // flow already undeployed; a late reconnect must not resubscribe
		}
		rt.ReportStatus("connected", n.Topic)
		token := client.Subscribe(n.Topic, n.QoS, func(_ mqtt.Client, msg mqtt.Message) {
			if ctx.Err() != nil {
				return
			}
			emit(map[string]interface{}{
				"topic":   msg.Topic(),
				"payload": string(msg.Payload()),
				"qos":     float64(msg.Qos()),
				"retain":  msg.Retained(),
			})
		})
		n.mu.Lock()
		n.client = client
		n.mu.Unlock()
		if err := awaitToken(ctx, token, subscribeTimeout); err != nil && ctx.Err() == nil {
			rt.ReportError(fmt.Errorf("mqtt in: subscribe to %q: %w", n.Topic, err))
		}
	})

	<-ctx.Done()

	n.mu.Lock()
	client := n.client
	n.client = nil
	n.mu.Unlock()
	if client != nil {
		// Bounded wait only: the broker may be gone, and Undeploy is
		// waiting on this Start to return.
		client.Unsubscribe(n.Topic).WaitTimeout(unsubscribeTimeout)
	}
	return nil
}

// awaitToken waits for token to complete, giving up when ctx ends or
// after timeout, whichever comes first (paho's own token.Wait blocks
// indefinitely and cannot be interrupted by a context).
func awaitToken(ctx context.Context, token mqtt.Token, timeout time.Duration) error {
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-token.Done():
		return token.Error()
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return fmt.Errorf("timed out after %s", timeout)
	}
}

// Execute exists only to satisfy registry.NodeExecutor (embedded in
// registry.EmittingNode) - MQTT in has no input port; the engine never
// calls it for a node with no wired input.
func (n *Node) Execute(ctx interface{}, input map[string]interface{}) (map[string]interface{}, error) {
	return nil, fmt.Errorf("mqtt in: has no input port")
}

func (n *Node) Validate() error {
	if n.Broker == "" {
		return fmt.Errorf("mqtt in: broker is required")
	}
	if n.Topic == "" {
		return fmt.Errorf("mqtt in: topic is required")
	}
	if n.QoS > 2 {
		return fmt.Errorf("mqtt in: qos must be 0, 1, or 2")
	}
	return nil
}

func (n *Node) GetConfig() map[string]interface{} {
	return map[string]interface{}{
		"broker": n.Broker,
		"topic":  n.Topic,
		"qos":    float64(n.QoS),
	}
}

func (n *Node) SetConfig(config map[string]interface{}) error {
	if v, ok := config["broker"].(string); ok {
		n.Broker = v
	}
	if v, ok := config["topic"].(string); ok {
		n.Topic = v
	}
	if v, ok := config["qos"].(float64); ok {
		n.QoS = byte(v)
	}
	return n.Validate()
}

func init() {
	reg := registry.GetGlobalRegistry()
	err := reg.RegisterFactory("mqtt in", func() registry.NodeExecutor {
		return &Node{}
	}, registry.NodeMetadata{
		ID:          "mqtt in",
		Type:        "mqtt in",
		Name:        "MQTT in",
		Description: "Subscribes to a topic on a shared mqtt-broker config node",
		Category:    "network",
		Inputs:      []registry.Port{},
		Outputs: []registry.Port{
			{ID: "output", Name: "Output", Description: "One message per received publish", Required: true},
		},
		ConfigSchema: registry.Schema{
			Properties: map[string]registry.Property{
				"broker": {
					Type:        "string",
					Description: "ID of an existing mqtt-broker config node",
					Default:     "",
					Label:       "Broker",
					Order:       1,
					Widget:      "nodeSelect",
					NodeTypes:   []string{"mqtt-broker"},
				},
				"topic": {
					Type:        "string",
					Description: "Topic filter to subscribe to (may include +/# wildcards)",
					Default:     "",
					Label:       "Topic",
					Placeholder: "sensors/#",
					Order:       2,
					Widget:      "text",
				},
				"qos": {
					Type:        "number",
					Description: "MQTT QoS (0, 1, or 2)",
					Default:     float64(0),
					Min:         base.FloatPtr(0),
					Max:         base.FloatPtr(2),
					Label:       "QoS",
					Order:       3,
					Widget:      "number",
				},
			},
			Required: []string{"broker", "topic"},
		},
		Help: "**Subscribes to an MQTT topic** on the selected broker. Each received message has `msg.payload` (the message body) and `msg.topic`. Wildcards `+` and `#` are allowed.",
		Icon: "radio-tower",
		Tags: []string{"network", "mqtt", "subscribe"},
	})
	if err != nil {
		panic(err)
	}
}
