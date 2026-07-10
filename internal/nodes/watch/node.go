// Package watch provides the Watch node implementation - a
// registry.EmittingNode with no input port that emits a message whenever
// one of its configured files or directories changes.
//
// Built on fsnotify (inotify/kqueue/ReadDirectoryChangesW under the hood)
// rather than Node-RED's node-watch, so event names differ: this node
// reports fsnotify's own Op string ("create"/"write"/"remove"/"rename"/
// "chmod", lowercased) instead of node-watch's coarser "update"/"remove".
//
// Recursive watching is a one-time directory walk at Start: every
// subdirectory that exists when the node starts is watched, but a
// subdirectory created later is not automatically picked up (fsnotify has
// no native recursive mode, unlike node-watch - adding one would mean
// watching fsnotify's own Create events for new directories and calling
// Add on them, which is deferred as a documented scope cut; see
// docs/NODE_PALETTE_PLAN.md). Device/socket/FIFO file-type detection
// (Node-RED reports "blockdevice"/"characterdevice"/"socket"/"fifo") is
// also not implemented, since os.FileInfo exposes no portable way to tell
// them apart from a regular file; this node only distinguishes "file" and
// "directory".
package watch

import (
    "context"
    "fmt"
    "os"
    "path/filepath"
    "strings"

    "github.com/GrimbiXcode/Go-RED/internal/registry"
    "github.com/fsnotify/fsnotify"
)

// Node holds a Watch node's configuration.
type Node struct {
    // Files is the list of file/directory paths to watch.
    Files []string
    // Recursive watches every subdirectory found under a directory in
    // Files at Start time (see package doc for the scope cut on
    // directories created afterward).
    Recursive bool
}

// Start watches Files (and, if Recursive, their subdirectories as they
// exist at call time) until ctx is cancelled, emitting one message per
// filesystem event.
func (n *Node) Start(ctx context.Context, emit func(payload map[string]interface{})) error {
    watcher, err := fsnotify.NewWatcher()
    if err != nil {
        return fmt.Errorf("watch: %w", err)
    }
    defer watcher.Close()

    topic := n.topic()
    for _, f := range n.Files {
        if f == "" {
            continue
        }
        if err := addWatch(watcher, f, n.Recursive); err != nil {
            return fmt.Errorf("watch: %w", err)
        }
    }

    rt, _ := runtimeFrom(ctx)
    for {
        select {
        case <-ctx.Done():
            return nil
        case event, ok := <-watcher.Events:
            if !ok {
                return nil
            }
            emit(eventMessage(event, topic))
        case err, ok := <-watcher.Errors:
            if !ok {
                return nil
            }
            rt.ReportError(fmt.Errorf("watch: %w", err))
        }
    }
}

func addWatch(watcher *fsnotify.Watcher, path string, recursive bool) error {
    info, err := os.Stat(path)
    if err != nil {
        return err
    }
    if !info.IsDir() || !recursive {
        return watcher.Add(path)
    }
    return filepath.Walk(path, func(p string, fi os.FileInfo, err error) error {
        if err != nil {
            return err
        }
        if fi.IsDir() {
            return watcher.Add(p)
        }
        return nil
    })
}

func eventMessage(event fsnotify.Event, topic string) map[string]interface{} {
    msg := map[string]interface{}{
        "payload":  event.Name,
        "topic":    topic,
        "file":     filepath.Base(event.Name),
        "filename": event.Name,
        "event":    strings.ToLower(event.Op.String()),
    }
    if stat, err := os.Stat(event.Name); err == nil {
        if stat.IsDir() {
            msg["type"] = "directory"
        } else {
            msg["type"] = "file"
            msg["size"] = float64(stat.Size())
        }
    } else {
        msg["type"] = "n/a"
    }
    return msg
}

func (n *Node) topic() string {
    if len(n.Files) == 1 {
        return n.Files[0]
    }
    return strings.Join(n.Files, ",")
}

func runtimeFrom(ctx interface{}) (*registry.NodeRuntime, bool) {
    c, ok := ctx.(context.Context)
    if !ok {
        return nil, false
    }
    return registry.RuntimeFromContext(c)
}

// Execute exists only to satisfy registry.NodeExecutor (embedded in
// registry.EmittingNode) - Watch has no input port and never receives an
// incoming message to react to.
func (n *Node) Execute(ctx interface{}, input map[string]interface{}) (map[string]interface{}, error) {
    return nil, fmt.Errorf("watch: has no input port")
}

func (n *Node) Validate() error {
    if len(n.Files) == 0 {
        return fmt.Errorf("watch: at least one file or directory is required")
    }
    return nil
}

func (n *Node) GetConfig() map[string]interface{} {
    return map[string]interface{}{
        "files":     strings.Join(n.Files, ","),
        "recursive": n.Recursive,
    }
}

func (n *Node) SetConfig(config map[string]interface{}) error {
    n.Files = nil
    if f, ok := config["files"].(string); ok {
        for _, part := range strings.Split(f, ",") {
            part = strings.TrimSpace(part)
            if part != "" {
                n.Files = append(n.Files, part)
            }
        }
    }
    if r, ok := config["recursive"].(bool); ok {
        n.Recursive = r
    }
    return n.Validate()
}

func init() {
    reg := registry.GetGlobalRegistry()
    err := reg.RegisterFactory("watch", func() registry.NodeExecutor {
        return &Node{}
    }, registry.NodeMetadata{
        ID:          "watch",
        Type:        "watch",
        Name:        "Watch",
        Description: "Emits a message whenever a watched file or directory changes",
        Category:    "storage",
        Inputs:      []registry.Port{},
        Outputs: []registry.Port{
            {ID: "output", Name: "Output", Description: "One message per filesystem change event", Required: true},
        },
        ConfigSchema: registry.Schema{
            Properties: map[string]registry.Property{
                "files":     {Type: "string", Description: "Comma-separated list of files/directories to watch", Default: ""},
                "recursive": {Type: "boolean", Description: "Also watch subdirectories that exist when the node starts", Default: false},
            },
            Required: []string{"files"},
        },
        Icon: `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="#00ADD8"><path d="M12 4.5C7 4.5 2.7 7.6 1 12c1.7 4.4 6 7.5 11 7.5s9.3-3.1 11-7.5c-1.7-4.4-6-7.5-11-7.5zM12 17a5 5 0 110-10 5 5 0 010 10z"/></svg>`,
        Tags: []string{"storage", "watch", "file", "inotify"},
    })
    if err != nil {
        panic(err)
    }
}
