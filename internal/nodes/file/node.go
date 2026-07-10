// Package file provides the File node implementation (Node-RED's "file"
// node - writes, appends to, or deletes a file).
//
// Filename is a typedvalue.Value, matching Node-RED's typed-input widget
// for this field: a fixed string (the common case), or resolved per
// message from msg/flow/global/env. It is never taken from msg.payload
// itself, only from the configured value/path - so a flow author who
// wants a message-driven filename must say so explicitly at deploy time,
// the same opt-in Node-RED itself requires.
//
// Concurrent messages at the same node instance run in their own engine
// goroutines (see internal/engine/engine.go's executeNode), so writes are
// serialized with a mutex to avoid interleaving two messages' bytes in the
// same file. Unlike Node-RED, which keeps a single fs.WriteStream open
// across messages for "append" mode with a static filename (an
// optimization), this implementation reopens the file for every message;
// simpler, and the actual bytes on disk end up identical.
package file

import (
    "context"
    "encoding/base64"
    "encoding/json"
    "fmt"
    "os"
    "path/filepath"
    "sync"

    "github.com/GrimbiXcode/Go-RED/internal/registry"
    "github.com/GrimbiXcode/Go-RED/internal/typedvalue"
)

// Node holds a File node's configuration.
type Node struct {
    // Filename resolves to the path to write/delete. Default: a fixed
    // empty string, which is a configuration error (see Validate).
    Filename typedvalue.Value
    // Action is "append" (default), "overwrite", or "delete".
    Action string
    // AppendNewline appends "\n" after the written payload (append/overwrite
    // only, and only when the payload isn't already raw bytes).
    AppendNewline bool
    // CreateDir creates the file's parent directory (and any missing
    // ancestors) before writing, if it doesn't already exist.
    CreateDir bool
    // Encoding is "utf8" (default: payload is written as its natural byte
    // representation) or "base64" (payload must be a base64 string,
    // decoded before writing - for binary content that arrived as text).
    Encoding string

    mu sync.Mutex
}

// ExecuteMulti resolves Filename, then appends to, overwrites, or deletes
// that file per Action. Sends on "output" only after a successful
// operation; a missing filename or (for append/overwrite) a missing
// msg.payload sends on no port rather than failing the node.
func (n *Node) ExecuteMulti(ctx interface{}, input map[string]interface{}) (map[string]map[string]interface{}, error) {
    valueResolver, _ := resolvers(ctx, input)
    raw, err := n.Filename.Resolve(valueResolver)
    if err != nil {
        return nil, fmt.Errorf("file: %w", err)
    }
    filename := toFilename(raw)
    if filename == "" {
        return map[string]map[string]interface{}{}, nil
    }

    n.mu.Lock()
    defer n.mu.Unlock()

    if n.Action == "delete" {
        if err := os.Remove(filename); err != nil && !os.IsNotExist(err) {
            return nil, fmt.Errorf("file: delete %s: %w", filename, err)
        }
        out := cloneMap(input)
        out["filename"] = filename
        return map[string]map[string]interface{}{"output": out}, nil
    }

    payload, hasPayload := input["payload"]
    if !hasPayload {
        return map[string]map[string]interface{}{}, nil
    }

    data, err := n.payloadToBytes(payload)
    if err != nil {
        return nil, fmt.Errorf("file: %w", err)
    }
    if n.AppendNewline {
        data = append(data, '\n')
    }

    if n.CreateDir {
        if err := os.MkdirAll(filepath.Dir(filename), 0o755); err != nil {
            return nil, fmt.Errorf("file: create directory for %s: %w", filename, err)
        }
    }

    flags := os.O_CREATE | os.O_WRONLY | os.O_APPEND
    if n.Action == "overwrite" {
        flags = os.O_CREATE | os.O_WRONLY | os.O_TRUNC
    }
    f, err := os.OpenFile(filename, flags, 0o644)
    if err != nil {
        return nil, fmt.Errorf("file: open %s: %w", filename, err)
    }
    _, writeErr := f.Write(data)
    closeErr := f.Close()
    if writeErr != nil {
        return nil, fmt.Errorf("file: write %s: %w", filename, writeErr)
    }
    if closeErr != nil {
        return nil, fmt.Errorf("file: close %s: %w", filename, closeErr)
    }

    out := cloneMap(input)
    out["filename"] = filename
    return map[string]map[string]interface{}{"output": out}, nil
}

// payloadToBytes converts msg.payload into the bytes to write. Objects and
// arrays are JSON-encoded, numbers/bools are stringified, matching
// Node-RED; a string is decoded from base64 first if Encoding is set to
// "base64".
func (n *Node) payloadToBytes(payload interface{}) ([]byte, error) {
    switch v := payload.(type) {
    case []byte:
        return v, nil
    case string:
        if n.Encoding == "base64" {
            decoded, err := base64.StdEncoding.DecodeString(v)
            if err != nil {
                return nil, fmt.Errorf("payload is not valid base64: %w", err)
            }
            return decoded, nil
        }
        return []byte(v), nil
    case bool, float64, int, int64:
        return []byte(fmt.Sprint(v)), nil
    case nil:
        return nil, fmt.Errorf("payload is nil")
    default:
        encoded, err := json.Marshal(v)
        if err != nil {
            return nil, fmt.Errorf("payload cannot be encoded: %w", err)
        }
        return encoded, nil
    }
}

func toFilename(v interface{}) string {
    switch s := v.(type) {
    case string:
        return s
    case nil:
        return ""
    default:
        return fmt.Sprint(s)
    }
}

// Execute exists only to satisfy registry.NodeExecutor (embedded in
// registry.MultiOutputExecutor) - the engine always calls ExecuteMulti for
// a node implementing it, never this.
func (n *Node) Execute(ctx interface{}, input map[string]interface{}) (map[string]interface{}, error) {
    outputs, err := n.ExecuteMulti(ctx, input)
    if err != nil {
        return nil, err
    }
    return outputs["output"], nil
}

func (n *Node) Validate() error {
    switch n.Action {
    case "append", "overwrite", "delete":
    default:
        return fmt.Errorf("file: unsupported action %q", n.Action)
    }
    switch n.Encoding {
    case "", "utf8", "base64":
    default:
        return fmt.Errorf("file: unsupported encoding %q", n.Encoding)
    }
    return nil
}

func (n *Node) GetConfig() map[string]interface{} {
    return map[string]interface{}{
        "filename":      valueToConfig(n.Filename),
        "action":        n.Action,
        "appendNewline": n.AppendNewline,
        "createDir":     n.CreateDir,
        "encoding":      n.Encoding,
    }
}

func (n *Node) SetConfig(config map[string]interface{}) error {
    n.Filename = parseValue(config["filename"])
    n.Action = "append"
    if a, ok := config["action"].(string); ok && a != "" {
        n.Action = a
    }
    if an, ok := config["appendNewline"].(bool); ok {
        n.AppendNewline = an
    }
    if cd, ok := config["createDir"].(bool); ok {
        n.CreateDir = cd
    }
    n.Encoding = "utf8"
    if e, ok := config["encoding"].(string); ok && e != "" {
        n.Encoding = e
    }
    return n.Validate()
}

func resolvers(ctx interface{}, input map[string]interface{}) (typedvalue.Resolver, typedvalue.PropertyResolver) {
    valueResolver := typedvalue.Resolver{Message: input}
    propResolver := typedvalue.PropertyResolver{Message: input}

    c, ok := ctx.(context.Context)
    if !ok {
        return valueResolver, propResolver
    }
    rt, ok := registry.RuntimeFromContext(c)
    if !ok {
        return valueResolver, propResolver
    }
    if rt.FlowContext != nil {
        valueResolver.FlowContext = rt.FlowContext
        propResolver.FlowContext = rt.FlowContext
    }
    if rt.GlobalContext != nil {
        valueResolver.GlobalContext = rt.GlobalContext
        propResolver.GlobalContext = rt.GlobalContext
    }
    return valueResolver, propResolver
}

func cloneMap(src map[string]interface{}) map[string]interface{} {
    dst := make(map[string]interface{}, len(src))
    for k, v := range src {
        dst[k] = v
    }
    return dst
}

func parseValue(raw interface{}) typedvalue.Value {
    m, ok := raw.(map[string]interface{})
    if !ok {
        return typedvalue.Value{}
    }
    t, _ := m["type"].(string)
    v, _ := m["value"].(string)
    return typedvalue.Value{Type: typedvalue.Type(t), Value: v}
}

func valueToConfig(v typedvalue.Value) map[string]interface{} {
    return map[string]interface{}{"type": string(v.Type), "value": v.Value}
}

func init() {
    reg := registry.GetGlobalRegistry()
    err := reg.RegisterFactory("file", func() registry.NodeExecutor {
        return &Node{Filename: typedvalue.Value{Type: typedvalue.TypeString, Value: ""}, Action: "append", Encoding: "utf8"}
    }, registry.NodeMetadata{
        ID:          "file",
        Type:        "file",
        Name:        "File",
        Description: "Writes, appends to, or deletes a file using msg.payload as the content",
        Category:    "storage",
        Inputs: []registry.Port{
            {ID: "input", Name: "Input", Description: "Message with the content to write", Required: true},
        },
        Outputs: []registry.Port{
            {ID: "output", Name: "Output", Description: "Message after a successful write/delete, filename set", Required: true},
        },
        ConfigSchema: registry.Schema{
            Properties: map[string]registry.Property{
                "filename":      {Type: "object", Description: `Path to write, e.g. {"type":"str","value":"/tmp/out.txt"}`, Default: map[string]interface{}{"type": "str", "value": ""}},
                "action":        {Type: "string", Description: "append, overwrite, or delete", Default: "append", Enum: []string{"append", "overwrite", "delete"}},
                "appendNewline": {Type: "boolean", Description: "Append a newline after the written payload", Default: false},
                "createDir":     {Type: "boolean", Description: "Create the parent directory if it doesn't exist", Default: false},
                "encoding":      {Type: "string", Description: "utf8 (payload written as-is) or base64 (payload is a base64 string, decoded before writing)", Default: "utf8", Enum: []string{"utf8", "base64"}},
            },
            Required: []string{"filename", "action"},
        },
        Icon: `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="#00ADD8"><path d="M6 2h9l5 5v15H6zm8 1.5V8h4.5z"/></svg>`,
        Tags: []string{"storage", "file", "write"},
    })
    if err != nil {
        panic(err)
    }
}
