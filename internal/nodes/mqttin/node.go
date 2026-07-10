// Package mqttin provides the "mqtt in" node implementation - subscribes
// to a topic on a shared mqtt-broker config node (internal/nodes/mqttbroker)
// and emits one message per received publish.
package mqttin

import (
    "context"
    "fmt"

    "github.com/GrimbiXcode/Go-RED/internal/nodes/mqttbroker"
    "github.com/GrimbiXcode/Go-RED/internal/registry"
    mqtt "github.com/eclipse/paho.mqtt.golang"
)

// Node holds an MQTT-in node's configuration.
type Node struct {
    // Broker is the mqtt-broker config node's ID.
    Broker string
    Topic  string
    QoS    byte
}

// Start resolves Broker via registry.NodeRuntime.GetNode, then registers a
// subscription (through mqttbroker.Broker.OnConnect, so it is (re)applied
// on every connect) that emits one message per received publish, until ctx
// is cancelled.
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

    broker.OnConnect(func(client mqtt.Client) {
        token := client.Subscribe(n.Topic, n.QoS, func(_ mqtt.Client, msg mqtt.Message) {
            emit(map[string]interface{}{
                "topic":   msg.Topic(),
                "payload": string(msg.Payload()),
                "qos":     float64(msg.Qos()),
                "retain":  msg.Retained(),
            })
        })
        token.Wait()
    })

    <-ctx.Done()
    return nil
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
                "broker": {Type: "string", Description: "ID of an existing mqtt-broker config node", Default: ""},
                "topic":  {Type: "string", Description: "Topic filter to subscribe to (may include +/# wildcards)", Default: ""},
                "qos":    {Type: "number", Description: "MQTT QoS (0, 1, or 2)", Default: float64(0), Min: floatPtr(0), Max: floatPtr(2)},
            },
            Required: []string{"broker", "topic"},
        },
        Icon: `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="#00ADD8"><path d="M12 2L2 7l10 5 10-5zM2 17l10 5 10-5M2 12l10 5 10-5"/></svg>`,
        Tags: []string{"network", "mqtt", "subscribe"},
    })
    if err != nil {
        panic(err)
    }
}

func floatPtr(f float64) *float64 { return &f }
