// Package mqttbroker provides the "mqtt-broker" config node - a single
// shared paho.mqtt.golang connection, referenced by ID from mqtt in/mqtt
// out nodes rather than each opening its own connection (see
// docs/NODE_PALETTE_PLAN.md, Phase 6's config-node concept).
//
// # Connection lifecycle and why it isn't in Start()
//
// The underlying mqtt.Client is created and told to Connect (with
// SetConnectRetry/SetAutoReconnect, so it keeps retrying in the background
// rather than failing Deploy if the broker isn't up yet) inside SetConfig,
// not inside an EmittingNode.Start. The engine launches every
// registry.EmittingNode's Start in its own goroutine with no ordering
// guarantee relative to each other (internal/engine/engine.go's
// startEmittingNodes) - if this node deferred connecting to Start, an
// mqtt-in node's own Start could run concurrently and find no client yet.
// SetConfig, by contrast, runs synchronously inside Deploy's single-node-
// at-a-time initialization loop, which always finishes for every node
// (config nodes included, regardless of Go's unordered map iteration
// order) before any Start is launched - so the client is guaranteed to
// exist by the time a consumer resolves it via
// registry.NodeRuntime.GetNode(brokerID).
//
// Consumers don't call Subscribe directly against a possibly-not-yet-
// connected client; they register a handler via OnConnect, which fires
// once immediately if already connected and again on every future
// reconnect (paho's own OnConnectHandler is a single, broker-wide
// callback - this node fans it out to every registered consumer).
//
// TLS is a plain UseTLS/InsecureSkipVerify pair on this node directly
// rather than a reference to a separate tls-config node: SetConfig has no
// registry.NodeRuntime (and thus no GetNode) to resolve a config-node
// reference, and deferring that resolution to this node's own Start would
// reintroduce the same goroutine-ordering race described above. See
// docs/NODE_PALETTE_PLAN.md for this documented scope cut.
package mqttbroker

import (
    "crypto/tls"
    "fmt"
    "sync"
    "time"

    "github.com/GrimbiXcode/Go-RED/internal/registry"
    mqtt "github.com/eclipse/paho.mqtt.golang"
)

// Broker is implemented by mqtt-broker nodes (this package's Node),
// exposing the shared client to mqtt in/mqtt out.
type Broker interface {
    // Client returns the shared paho client. Callers should not change its
    // options; use OnConnect to run work whenever the connection is (or
    // becomes) ready.
    Client() mqtt.Client
    // OnConnect registers handler to run once immediately if the broker is
    // already connected, and again every time it (re)connects.
    OnConnect(handler func(mqtt.Client))
}

// Node holds an MQTT-broker node's configuration and shared client.
type Node struct {
    URL                string
    ClientID           string
    Username           string
    Password           string
    CleanSession       bool
    KeepAliveSec       int
    UseTLS             bool
    InsecureSkipVerify bool

    mu        sync.Mutex
    client    mqtt.Client
    connected bool
    onConnect []func(mqtt.Client)
}

// Client implements Broker.
func (n *Node) Client() mqtt.Client {
    n.mu.Lock()
    defer n.mu.Unlock()
    return n.client
}

// OnConnect implements Broker.
func (n *Node) OnConnect(handler func(mqtt.Client)) {
    n.mu.Lock()
    client := n.client
    alreadyConnected := n.connected
    n.onConnect = append(n.onConnect, handler)
    n.mu.Unlock()

    if alreadyConnected {
        handler(client)
    }
}

func (n *Node) handleConnect(client mqtt.Client) {
    n.mu.Lock()
    n.connected = true
    handlers := append([]func(mqtt.Client){}, n.onConnect...)
    n.mu.Unlock()

    for _, h := range handlers {
        h(client)
    }
}

func (n *Node) handleConnectionLost(client mqtt.Client, err error) {
    n.mu.Lock()
    n.connected = false
    n.mu.Unlock()
}

// connect builds mqtt.ClientOptions from Node's configuration and starts
// connecting in the background (non-blocking).
func (n *Node) connect() {
    opts := mqtt.NewClientOptions()
    opts.AddBroker(n.URL)
    opts.SetClientID(n.ClientID)
    if n.Username != "" {
        opts.SetUsername(n.Username)
        opts.SetPassword(n.Password)
    }
    opts.SetCleanSession(n.CleanSession)
    keepAlive := n.KeepAliveSec
    if keepAlive <= 0 {
        keepAlive = 60
    }
    opts.SetKeepAlive(time.Duration(keepAlive) * time.Second)
    opts.SetAutoReconnect(true)
    opts.SetConnectRetry(true)
    opts.SetConnectRetryInterval(5 * time.Second)
    opts.SetOnConnectHandler(n.handleConnect)
    opts.SetConnectionLostHandler(n.handleConnectionLost)
    if n.UseTLS {
        opts.SetTLSConfig(&tls.Config{InsecureSkipVerify: n.InsecureSkipVerify})
    }

    n.mu.Lock()
    n.client = mqtt.NewClient(opts)
    client := n.client
    n.mu.Unlock()

    client.Connect()
}

// Close disconnects the shared client, waiting up to 250ms for in-flight
// work to complete.
func (n *Node) Close() error {
    n.mu.Lock()
    client := n.client
    n.mu.Unlock()
    if client != nil {
        client.Disconnect(250)
    }
    return nil
}

// Execute exists only to satisfy registry.NodeExecutor - a config node has
// no message inputs/outputs of its own and is never wired into a flow's
// message path.
func (n *Node) Execute(ctx interface{}, input map[string]interface{}) (map[string]interface{}, error) {
    return nil, fmt.Errorf("mqtt-broker: config nodes have no message input")
}

func (n *Node) Validate() error {
    if n.URL == "" {
        return fmt.Errorf("mqtt-broker: url is required")
    }
    return nil
}

func (n *Node) GetConfig() map[string]interface{} {
    return map[string]interface{}{
        "url":                n.URL,
        "clientId":           n.ClientID,
        "username":           n.Username,
        "password":           n.Password,
        "cleanSession":       n.CleanSession,
        "keepAliveSec":       n.KeepAliveSec,
        "useTLS":             n.UseTLS,
        "insecureSkipVerify": n.InsecureSkipVerify,
    }
}

func (n *Node) SetConfig(config map[string]interface{}) error {
    if v, ok := config["url"].(string); ok {
        n.URL = v
    }
    if v, ok := config["clientId"].(string); ok {
        n.ClientID = v
    }
    if v, ok := config["username"].(string); ok {
        n.Username = v
    }
    if v, ok := config["password"].(string); ok {
        n.Password = v
    }
    n.CleanSession = true
    if v, ok := config["cleanSession"].(bool); ok {
        n.CleanSession = v
    }
    if v, ok := config["keepAliveSec"].(float64); ok {
        n.KeepAliveSec = int(v)
    }
    if v, ok := config["useTLS"].(bool); ok {
        n.UseTLS = v
    }
    if v, ok := config["insecureSkipVerify"].(bool); ok {
        n.InsecureSkipVerify = v
    }
    if err := n.Validate(); err != nil {
        return err
    }
    n.connect()
    return nil
}

func init() {
    reg := registry.GetGlobalRegistry()
    err := reg.RegisterFactory("mqtt-broker", func() registry.NodeExecutor {
        return &Node{ClientID: fmt.Sprintf("go-red-%d", time.Now().UnixNano())}
    }, registry.NodeMetadata{
        ID:          "mqtt-broker",
        Type:        "mqtt-broker",
        Name:        "MQTT Broker",
        Description: "Shared MQTT broker connection, referenced by ID from mqtt in/mqtt out",
        Category:    "config",
        Inputs:      []registry.Port{},
        Outputs:     []registry.Port{},
        ConfigSchema: registry.Schema{
            Properties: map[string]registry.Property{
                "url":                {Type: "string", Description: "Broker URL, e.g. tcp://localhost:1883 or ssl://localhost:8883", Default: ""},
                "clientId":           {Type: "string", Description: "MQTT client ID (default: a generated unique ID)", Default: ""},
                "username":           {Type: "string", Description: "Broker username", Default: ""},
                "password":           {Type: "string", Description: "Broker password", Default: ""},
                "cleanSession":       {Type: "boolean", Description: "Start a clean MQTT session on each connect", Default: true},
                "keepAliveSec":       {Type: "number", Description: "MQTT keep-alive interval in seconds", Default: float64(60), Min: floatPtr(1)},
                "useTLS":             {Type: "boolean", Description: "Connect over TLS", Default: false},
                "insecureSkipVerify": {Type: "boolean", Description: "Skip broker certificate verification (testing only)", Default: false},
            },
            Required: []string{"url"},
        },
        Icon: `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="#00ADD8"><path d="M12 2L2 7l10 5 10-5zM2 17l10 5 10-5M2 12l10 5 10-5"/></svg>`,
        Tags: []string{"config", "mqtt", "broker"},
    })
    if err != nil {
        panic(err)
    }
}

func floatPtr(f float64) *float64 { return &f }
