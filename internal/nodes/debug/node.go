// Package debug provides the Debug node implementation.
package debug

import (
    "fmt"
    "log"
    "os"
    "sync"
    "time"

    "github.com/GrimbiXcode/Go-RED/internal/registry"
)

// DebugNode implements a node that outputs messages to the debug console.
type DebugNode struct {
    config DebugConfig
    mu sync.Mutex
    outputBuffer []string
    maxBufferSize int
}

// DebugConfig contains the configuration for a Debug node.
type DebugConfig struct {
    Enabled bool `json:"enabled"`
    OutputToConsole bool `json:"outputToConsole"`
    MaxBufferSize int `json:"maxBufferSize"`
    Prefix string `json:"prefix"`
    ShowTimestamp bool `json:"showTimestamp"`
    ShowPath bool `json:"showPath"`
}

func NewDebugNode() *DebugNode {
    return &DebugNode{
        config: DebugConfig{
            Enabled: true,
            OutputToConsole: true,
            MaxBufferSize: 100,
            Prefix: "",
            ShowTimestamp: true,
            ShowPath: true,
        },
        outputBuffer: make([]string, 0),
        maxBufferSize: 100,
    }
}

func (n *DebugNode) Execute(ctx interface{}, input map[string]interface{}) (map[string]interface{}, error) {
    if !n.config.Enabled {
        return input, nil
    }
    
    message := n.formatMessage(input)
    
    if n.config.OutputToConsole {
        log.Println(message)
        fmt.Fprintln(os.Stderr, message)
    }
    
    n.mu.Lock()
    n.outputBuffer = append(n.outputBuffer, message)
    if len(n.outputBuffer) > n.maxBufferSize {
        n.outputBuffer = n.outputBuffer[len(n.outputBuffer)-n.maxBufferSize:]
    }
    n.mu.Unlock()
    
    return input, nil
}

func (n *DebugNode) formatMessage(input map[string]interface{}) string {
    message := ""
    if n.config.Prefix != "" {
        message += "[" + n.config.Prefix + "] "
    }
    if n.config.ShowTimestamp {
        message += time.Now().Format("2006-01-02 15:04:05.000") + " "
    }
    if payload, ok := input["payload"]; ok {
        message += fmt.Sprintf("Payload: %v", payload)
    } else {
        message += fmt.Sprintf("Data: %v", input)
    }
    if n.config.ShowPath {
        if path, ok := input["_path"].([]string); ok {
            message += " | Path: " + fmt.Sprintf("%v", path)
        }
    }
    return message
}

func (n *DebugNode) GetOutput() []string {
    n.mu.Lock()
    defer n.mu.Unlock()
    output := make([]string, len(n.outputBuffer))
    copy(output, n.outputBuffer)
    return output
}

func (n *DebugNode) ClearOutput() {
    n.mu.Lock()
    defer n.mu.Unlock()
    n.outputBuffer = make([]string, 0)
}

func (n *DebugNode) Validate() error {
    if n.config.MaxBufferSize < 0 {
        return fmt.Errorf("maxBufferSize cannot be negative")
    }
    return nil
}

func (n *DebugNode) GetConfig() map[string]interface{} {
    return map[string]interface{}{
        "enabled": n.config.Enabled,
        "outputToConsole": n.config.OutputToConsole,
        "maxBufferSize": n.config.MaxBufferSize,
        "prefix": n.config.Prefix,
        "showTimestamp": n.config.ShowTimestamp,
        "showPath": n.config.ShowPath,
    }
}

func (n *DebugNode) SetConfig(config map[string]interface{}) error {
    if enabled, ok := config["enabled"].(bool); ok {
        n.config.Enabled = enabled
    }
    if outputToConsole, ok := config["outputToConsole"].(bool); ok {
        n.config.OutputToConsole = outputToConsole
    }
    if maxBufferSize, ok := config["maxBufferSize"].(float64); ok {
        n.config.MaxBufferSize = int(maxBufferSize)
        n.maxBufferSize = int(maxBufferSize)
    }
    if prefix, ok := config["prefix"].(string); ok {
        n.config.Prefix = prefix
    }
    if showTimestamp, ok := config["showTimestamp"].(bool); ok {
        n.config.ShowTimestamp = showTimestamp
    }
    if showPath, ok := config["showPath"].(bool); ok {
        n.config.ShowPath = showPath
    }
    return n.Validate()
}

func init() {
    reg := registry.GetGlobalRegistry()
    err := reg.RegisterFactory("debug", func() registry.NodeExecutor {
        return NewDebugNode()
    }, registry.NodeMetadata{
        ID: "debug",
        Type: "debug",
        Name: "Debug",
        Description: "Outputs messages to the debug console",
        Category: "output",
        Inputs: []registry.Port{
            {ID: "input", Name: "Input", Description: "Message to debug", Required: true},
        },
        Outputs: []registry.Port{
            {ID: "output", Name: "Output", Description: "Pass-through output", Required: true},
        },
        ConfigSchema: registry.Schema{
            Properties: map[string]registry.Property{
                "enabled": {Type: "boolean", Description: "Whether debug output is enabled", Default: true},
                "outputToConsole": {Type: "boolean", Description: "Output to console", Default: true},
                "maxBufferSize": {Type: "number", Description: "Maximum number of messages to keep in buffer", Default: 100, Min: floatPtr(0)},
                "prefix": {Type: "string", Description: "Prefix for debug messages", Default: ""},
                "showTimestamp": {Type: "boolean", Description: "Show timestamps in debug output", Default: true},
                "showPath": {Type: "boolean", Description: "Show message path in debug output", Default: true},
            },
        },
        Icon: `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="#FF5722"><path d="M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm1 15h-2v-2h2v2zm0-4h-2V7h2v6z"/></svg>`,
        Tags: []string{"output", "debug", "log"},
    })
    if err != nil {
        panic(err)
    }
}

func floatPtr(f float64) *float64 {
    return &f
}
