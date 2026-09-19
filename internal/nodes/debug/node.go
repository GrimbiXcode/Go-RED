// Package debug provides the Debug node implementation.
//
// A Debug node shows the messages it receives in the editor's debug
// sidebar (through registry.NodeRuntime.Debug) and passes them on
// unchanged. Optionally it also prints them to the server's stderr.
package debug

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/GrimbiXcode/Go-RED/internal/registry"
)

// Output selects what part of the message the sidebar shows.
const (
	// OutputPayload shows msg.payload when the message has one, else the
	// whole message.
	OutputPayload = "payload"
	// OutputFull always shows the whole message.
	OutputFull = "full"
)

// DebugNode implements a node that outputs messages to the debug sidebar.
type DebugNode struct {
	config        DebugConfig
	mu            sync.Mutex
	outputBuffer  []string
	maxBufferSize int
}

// DebugConfig contains the configuration for a Debug node.
type DebugConfig struct {
	Enabled bool `json:"enabled"`
	// Output is OutputPayload or OutputFull.
	Output string `json:"output"`
	// OutputToConsole additionally prints every message to stderr.
	OutputToConsole bool   `json:"outputToConsole"`
	MaxBufferSize   int    `json:"maxBufferSize"`
	Prefix          string `json:"prefix"`
	ShowTimestamp   bool   `json:"showTimestamp"`
	ShowPath        bool   `json:"showPath"`
}

func NewDebugNode() *DebugNode {
	return &DebugNode{
		config: DebugConfig{
			Enabled:         true,
			Output:          OutputPayload,
			OutputToConsole: false,
			MaxBufferSize:   100,
			Prefix:          "",
			ShowTimestamp:   true,
			ShowPath:        true,
		},
		outputBuffer:  make([]string, 0),
		maxBufferSize: 100,
	}
}

func (n *DebugNode) Execute(ctx interface{}, input map[string]interface{}) (map[string]interface{}, error) {
	if !n.config.Enabled {
		return input, nil
	}

	if goCtx, ok := ctx.(context.Context); ok {
		if rt, ok := registry.RuntimeFromContext(goCtx); ok {
			rt.Debug(registry.DebugOutput{
				Payload: n.sidebarPayload(input),
				Topic:   topicOf(input),
			})
		}
	}

	message := n.formatMessage(input)

	if n.config.OutputToConsole {
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

// sidebarPayload picks what the sidebar entry shows for input.
func (n *DebugNode) sidebarPayload(input map[string]interface{}) interface{} {
	if input == nil {
		return nil
	}
	if n.config.Output != OutputFull {
		if payload, ok := input["payload"]; ok {
			return payload
		}
	}
	return input
}

func topicOf(input map[string]interface{}) string {
	if topic, ok := input["topic"].(string); ok {
		return topic
	}
	return ""
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
	if n.config.Output != OutputPayload && n.config.Output != OutputFull {
		return fmt.Errorf("output must be %q or %q", OutputPayload, OutputFull)
	}
	return nil
}

func (n *DebugNode) GetConfig() map[string]interface{} {
	return map[string]interface{}{
		"enabled":         n.config.Enabled,
		"output":          n.config.Output,
		"outputToConsole": n.config.OutputToConsole,
		"maxBufferSize":   n.config.MaxBufferSize,
		"prefix":          n.config.Prefix,
		"showTimestamp":   n.config.ShowTimestamp,
		"showPath":        n.config.ShowPath,
	}
}

func (n *DebugNode) SetConfig(config map[string]interface{}) error {
	if enabled, ok := config["enabled"].(bool); ok {
		n.config.Enabled = enabled
	}
	if output, ok := config["output"].(string); ok && output != "" {
		n.config.Output = output
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
		ID:          "debug",
		Type:        "debug",
		Name:        "Debug",
		Description: "Shows messages in the debug sidebar",
		Category:    "output",
		Inputs: []registry.Port{
			{ID: "input", Name: "Input", Description: "Message to debug", Required: true},
		},
		Outputs: []registry.Port{
			{ID: "output", Name: "Output", Description: "Pass-through output", Required: true},
		},
		ConfigSchema: registry.Schema{
			Properties: map[string]registry.Property{
				"output": {
					Type:        "string",
					Description: "What to show: msg.payload or the full message",
					Default:     OutputPayload,
					Label:       "Output",
					Order:       1,
					Widget:      "select",
					Options:     []registry.Option{{Value: "payload", Label: "msg.payload"}, {Value: "full", Label: "complete message"}},
				},
				"enabled": {
					Type:        "boolean",
					Description: "Whether debug output is enabled",
					Default:     true,
					Label:       "Enabled",
					Order:       2,
					Widget:      "boolean",
				},
				"outputToConsole": {
					Type:        "boolean",
					Description: "Also print to the server console",
					Default:     false,
					Label:       "Also print to the server console",
					Group:       "Server console",
					Order:       3,
					Widget:      "boolean",
				},
				"prefix": {
					Type:        "string",
					Description: "Prefix for console output",
					Default:     "",
					Label:       "Prefix",
					Group:       "Server console",
					Order:       4,
					Widget:      "text",
					VisibleWhen: &registry.Condition{Property: "outputToConsole", Values: []string{"true"}},
				},
				"showTimestamp": {
					Type:        "boolean",
					Description: "Show timestamps in console output",
					Default:     true,
					Label:       "Show timestamp",
					Group:       "Server console",
					Order:       5,
					Widget:      "boolean",
					VisibleWhen: &registry.Condition{Property: "outputToConsole", Values: []string{"true"}},
				},
				"showPath": {
					Type:        "boolean",
					Description: "Show message path in console output",
					Default:     true,
					Label:       "Show message path",
					Group:       "Server console",
					Order:       6,
					Widget:      "boolean",
					VisibleWhen: &registry.Condition{Property: "outputToConsole", Values: []string{"true"}},
				},
				"maxBufferSize": {
					Type:        "number",
					Description: "Maximum number of messages to keep in buffer",
					Default:     100,
					Min:         floatPtr(0),
					Label:       "Buffer size",
					Group:       "Server console",
					Order:       7,
					Widget:      "number",
					VisibleWhen: &registry.Condition{Property: "outputToConsole", Values: []string{"true"}},
				},
			},
		},
		Help: "**Shows messages in the Debug sidebar.** By default only `msg.payload` is shown; switch to the complete message to see every property. Node errors also appear in the sidebar.",
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
