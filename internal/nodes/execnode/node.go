// Package execnode provides the Exec node implementation (Node-RED type ID
// "exec" - named execnode here to avoid shadowing the imported stdlib
// os/exec package within this file).
//
// # Security
//
// This node runs an external command with the same OS privileges as the
// Go-RED process. Anyone able to deploy a flow that uses it can therefore
// run arbitrary commands on the host - this is inherent to the feature
// (Node-RED's own exec node has the identical property) and cannot be
// fully engineered away, only its sharpest edges blunted:
//
//   - Command is fixed at deploy time via node configuration and is never
//     read from the message; a compromised or malicious upstream node in
//     the same flow cannot redirect execution to a different binary.
//   - Arguments are always passed as a Go argv slice via
//     exec.CommandContext(ctx, Command, args...), never through a shell
//     (no "sh -c", unlike Node-RED's default useSpawn=false mode). Nothing
//     in Args or in msg.payload (if AppendPayload is set) is ever
//     interpreted for shell metacharacters (;, |, $(), backticks, ...),
//     because there is no shell in the path at all.
//   - Every run is bounded by TimeoutMs and by a fixed cap on captured
//     stdout/stderr bytes, so a runaway or wedged command can't hang a
//     node indefinitely or exhaust memory.
//   - The node refuses to run at all unless the enableEnvVar environment
//     variable is set on the Go-RED process - an explicit, instance-level
//     opt-in, not just something gated by editor/API auth. A fresh
//     Go-RED deployment cannot execute commands by accident.
package execnode

import (
    "bytes"
    "context"
    "errors"
    "fmt"
    "os"
    "os/exec"
    "time"

    "github.com/GrimbiXcode/Go-RED/internal/registry"
)

// enableEnvVar must be set (to any non-empty value) on the Go-RED process
// for this node to run commands at all.
const enableEnvVar = "GORED_ENABLE_EXEC"

// maxOutputBytes caps how much of stdout/stderr is captured per run.
const maxOutputBytes = 1 << 20 // 1 MiB

// Node holds an Exec node's configuration.
type Node struct {
    // Command is the executable name or path. Fixed at deploy time - never
    // taken from the message. Required.
    Command string
    // Args are fixed extra arguments, in order, before any AppendPayload
    // arguments.
    Args []string
    // AppendPayload, if true, appends msg.payload - which must be a string
    // or an array of strings - as trailing argv entries.
    AppendPayload bool
    // TimeoutMs bounds how long the command may run.
    TimeoutMs int64
}

// Execute runs Command with Args (+ msg.payload if AppendPayload), and
// returns a copy of input with payload set to captured stdout, "stderr" set
// to captured stderr, and "exitCode" set to the process's exit code.
func (n *Node) Execute(ctx interface{}, input map[string]interface{}) (map[string]interface{}, error) {
    if os.Getenv(enableEnvVar) == "" {
        return nil, fmt.Errorf("exec: disabled - set %s=1 on the Go-RED process to allow the exec node to run commands", enableEnvVar)
    }
    if n.Command == "" {
        return nil, fmt.Errorf("exec: no command configured")
    }

    args := append([]string(nil), n.Args...)
    if n.AppendPayload {
        extra, err := payloadArgs(input["payload"])
        if err != nil {
            return nil, fmt.Errorf("exec: %w", err)
        }
        args = append(args, extra...)
    }

    timeout := time.Duration(n.TimeoutMs) * time.Millisecond
    if timeout <= 0 {
        timeout = 30 * time.Second
    }
    parent, ok := ctx.(context.Context)
    if !ok {
        parent = context.Background()
    }
    execCtx, cancel := context.WithTimeout(parent, timeout)
    defer cancel()

    // exec.CommandContext + a separate argv slice is OS-level argument
    // passing, not shell parsing - see the security note in the package
    // doc for why this matters.
    cmd := exec.CommandContext(execCtx, n.Command, args...)

    var stdout, stderr limitedBuffer
    stdout.max = maxOutputBytes
    stderr.max = maxOutputBytes
    cmd.Stdout = &stdout
    cmd.Stderr = &stderr

    runErr := cmd.Run()

    if runErr != nil && execCtx.Err() == context.DeadlineExceeded {
        return nil, fmt.Errorf("exec: command timed out after %s", timeout)
    }

    exitCode := 0
    if runErr != nil {
        var exitErr *exec.ExitError
        if errors.As(runErr, &exitErr) {
            exitCode = exitErr.ExitCode()
        } else {
            return nil, fmt.Errorf("exec: %w", runErr)
        }
    }

    output := cloneMap(input)
    output["payload"] = stdout.String()
    output["stderr"] = stderr.String()
    output["exitCode"] = float64(exitCode)
    return output, nil
}

// payloadArgs converts msg.payload into extra argv entries: a bare string
// becomes one argument, an array of strings becomes one argument each.
// Anything else is rejected rather than silently stringified, since a
// caller opting into AppendPayload should get a clear error if upstream
// data isn't in the expected shape.
func payloadArgs(payload interface{}) ([]string, error) {
    switch v := payload.(type) {
    case nil:
        return nil, nil
    case string:
        return []string{v}, nil
    case []interface{}:
        args := make([]string, 0, len(v))
        for _, item := range v {
            s, ok := item.(string)
            if !ok {
                return nil, fmt.Errorf("payload array must contain only strings to append as arguments, got %T", item)
            }
            args = append(args, s)
        }
        return args, nil
    default:
        return nil, fmt.Errorf("payload must be a string or array of strings to append as arguments, got %T", payload)
    }
}

// limitedBuffer is an io.Writer that stops accumulating past max bytes
// (silently discarding the rest) rather than growing unbounded for a
// runaway command.
type limitedBuffer struct {
    buf bytes.Buffer
    max int
}

func (w *limitedBuffer) Write(p []byte) (int, error) {
    remaining := w.max - w.buf.Len()
    if remaining <= 0 {
        return len(p), nil
    }
    if len(p) > remaining {
        w.buf.Write(p[:remaining])
        return len(p), nil
    }
    w.buf.Write(p)
    return len(p), nil
}

func (w *limitedBuffer) String() string { return w.buf.String() }

func (n *Node) Validate() error {
    if n.Command == "" {
        return fmt.Errorf("exec: command is required")
    }
    if n.TimeoutMs < 0 {
        return fmt.Errorf("exec: timeoutMs cannot be negative")
    }
    return nil
}

func (n *Node) GetConfig() map[string]interface{} {
    args := make([]interface{}, len(n.Args))
    for i, a := range n.Args {
        args[i] = a
    }
    return map[string]interface{}{
        "command":       n.Command,
        "args":          args,
        "appendPayload": n.AppendPayload,
        "timeoutMs":     n.TimeoutMs,
    }
}

func (n *Node) SetConfig(config map[string]interface{}) error {
    if c, ok := config["command"].(string); ok {
        n.Command = c
    }
    n.Args = nil
    if raw, ok := config["args"].([]interface{}); ok {
        for _, v := range raw {
            if s, ok := v.(string); ok {
                n.Args = append(n.Args, s)
            }
        }
    }
    if ap, ok := config["appendPayload"].(bool); ok {
        n.AppendPayload = ap
    }
    n.TimeoutMs = 30000
    if tm, ok := config["timeoutMs"].(float64); ok {
        n.TimeoutMs = int64(tm)
    }
    return n.Validate()
}

func cloneMap(src map[string]interface{}) map[string]interface{} {
    dst := make(map[string]interface{}, len(src))
    for k, v := range src {
        dst[k] = v
    }
    return dst
}

func init() {
    reg := registry.GetGlobalRegistry()
    err := reg.RegisterFactory("exec", func() registry.NodeExecutor {
        return &Node{TimeoutMs: 30000}
    }, registry.NodeMetadata{
        ID:          "exec",
        Type:        "exec",
        Name:        "Exec",
        Description: "Runs a fixed external command (argv-based, never via a shell) and returns stdout/stderr/exit code. Disabled unless GORED_ENABLE_EXEC is set on the server - see package docs for the full security rationale.",
        Category:    "function",
        Inputs: []registry.Port{
            {ID: "input", Name: "Input", Description: "Message that triggers the command", Required: true},
        },
        Outputs: []registry.Port{
            {ID: "output", Name: "Output", Description: "payload=stdout, stderr=stderr, exitCode=exit code", Required: true},
        },
        ConfigSchema: registry.Schema{
            Properties: map[string]registry.Property{
                "command":       {Type: "string", Description: "Executable name or path (never taken from the message)", Default: ""},
                "args":          {Type: "array", Description: "Fixed extra arguments, in order", Default: []interface{}{}},
                "appendPayload": {Type: "boolean", Description: "Append msg.payload (string or array of strings) as trailing arguments", Default: false},
                "timeoutMs":     {Type: "number", Description: "Maximum run time in milliseconds", Default: float64(30000), Min: floatPtr(0)},
            },
            Required: []string{"command"},
        },
        Icon: `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="#00ADD8"><path d="M4 4h16v16H4zM7 8l4 4-4 4M13 16h4"/></svg>`,
        Tags: []string{"function", "exec", "command", "shell"},
    })
    if err != nil {
        panic(err)
    }
}

func floatPtr(f float64) *float64 { return &f }
