// Package mqttout provides the "mqtt out" node implementation - publishes
// msg.payload to a topic on a shared mqtt-broker config node
// (internal/nodes/mqttbroker).
package mqttout

import (
    "context"
    "encoding/json"
    "fmt"
    "time"

    "github.com/GrimbiXcode/Go-RED/internal/nodes/mqttbroker"
    "github.com/GrimbiXcode/Go-RED/internal/registry"
)

// publishTimeout bounds how long Execute waits for the broker to
// acknowledge a publish (or, if not yet connected, to become connected -
// see the mqttbroker package doc: the client is created eagerly in the
// broker's own SetConfig, but the initial network handshake still happens
// in the background).
const publishTimeout = 10 * time.Second

// Node holds an MQTT-out node's configuration.
type Node struct {
    // Broker is the mqtt-broker config node's ID.
    Broker string
    // Topic overrides msg.topic when non-empty.
    Topic  string
    QoS    byte
    Retain bool
}

// Execute publishes input's payload (converted to bytes: passed through
// as-is for a string/[]byte, JSON-encoded otherwise) to Topic (or
// msg.topic if Topic is unset) via the shared broker connection.
func (n *Node) Execute(ctx interface{}, input map[string]interface{}) (map[string]interface{}, error) {
    c, ok := ctx.(context.Context)
    if !ok {
        c = context.Background()
    }
    rt, ok := registry.RuntimeFromContext(c)
    if !ok {
        return nil, fmt.Errorf("mqtt out: no runtime available")
    }
    exec, ok := rt.GetNode(n.Broker)
    if !ok {
        return nil, fmt.Errorf("mqtt out: broker config node %q not found", n.Broker)
    }
    broker, ok := exec.(mqttbroker.Broker)
    if !ok {
        return nil, fmt.Errorf("mqtt out: node %q is not an mqtt-broker", n.Broker)
    }

    topic := n.Topic
    if topic == "" {
        if t, ok := input["topic"].(string); ok {
            topic = t
        }
    }
    if topic == "" {
        return nil, fmt.Errorf("mqtt out: no topic configured and msg.topic is empty")
    }

    payload, err := toPayloadBytes(input["payload"])
    if err != nil {
        return nil, fmt.Errorf("mqtt out: %w", err)
    }

    client := broker.Client()
    if client == nil {
        return nil, fmt.Errorf("mqtt out: broker %q has no client yet", n.Broker)
    }
    token := client.Publish(topic, n.QoS, n.Retain, payload)
    if !token.WaitTimeout(publishTimeout) {
        return nil, fmt.Errorf("mqtt out: publish to %q timed out after %s", topic, publishTimeout)
    }
    if err := token.Error(); err != nil {
        return nil, fmt.Errorf("mqtt out: publish to %q: %w", topic, err)
    }

    return input, nil
}

func toPayloadBytes(payload interface{}) ([]byte, error) {
    switch v := payload.(type) {
    case []byte:
        return v, nil
    case string:
        return []byte(v), nil
    case nil:
        return []byte{}, nil
    case bool, float64:
        return []byte(fmt.Sprint(v)), nil
    default:
        encoded, err := json.Marshal(v)
        if err != nil {
            return nil, fmt.Errorf("payload cannot be encoded: %w", err)
        }
        return encoded, nil
    }
}

func (n *Node) Validate() error {
    if n.Broker == "" {
        return fmt.Errorf("mqtt out: broker is required")
    }
    if n.QoS > 2 {
        return fmt.Errorf("mqtt out: qos must be 0, 1, or 2")
    }
    return nil
}

func (n *Node) GetConfig() map[string]interface{} {
    return map[string]interface{}{
        "broker": n.Broker,
        "topic":  n.Topic,
        "qos":    float64(n.QoS),
        "retain": n.Retain,
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
    if v, ok := config["retain"].(bool); ok {
        n.Retain = v
    }
    return n.Validate()
}

func init() {
    reg := registry.GetGlobalRegistry()
    err := reg.RegisterFactory("mqtt out", func() registry.NodeExecutor {
        return &Node{}
    }, registry.NodeMetadata{
        ID:          "mqtt out",
        Type:        "mqtt out",
        Name:        "MQTT out",
        Description: "Publishes msg.payload to a topic on a shared mqtt-broker config node",
        Category:    "network",
        Inputs: []registry.Port{
            {ID: "input", Name: "Input", Description: "Message to publish", Required: true},
        },
        Outputs: []registry.Port{
            {ID: "output", Name: "Output", Description: "Message after a successful publish", Required: true},
        },
        ConfigSchema: registry.Schema{
            Properties: map[string]registry.Property{
                "broker": {Type: "string", Description: "ID of an existing mqtt-broker config node", Default: ""},
                "topic":  {Type: "string", Description: "Topic to publish to; empty uses msg.topic", Default: ""},
                "qos":    {Type: "number", Description: "MQTT QoS (0, 1, or 2)", Default: float64(0), Min: floatPtr(0), Max: floatPtr(2)},
                "retain": {Type: "boolean", Description: "Set the MQTT retain flag", Default: false},
            },
            Required: []string{"broker"},
        },
        Icon: `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="#00ADD8"><path d="M12 2L2 7l10 5 10-5zM2 17l10 5 10-5M2 12l10 5 10-5"/></svg>`,
        Tags: []string{"network", "mqtt", "publish"},
    })
    if err != nil {
        panic(err)
    }
}

func floatPtr(f float64) *float64 { return &f }
