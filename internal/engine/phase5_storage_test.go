package engine

import (
    "path/filepath"
    "testing"
    "time"

    "github.com/GrimbiXcode/Go-RED/internal/nodes/file"
    "github.com/GrimbiXcode/Go-RED/internal/nodes/filein"
    "github.com/GrimbiXcode/Go-RED/internal/nodes/watch"
    "github.com/GrimbiXcode/Go-RED/internal/registry"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

// This file demonstrates the Phase 5 milestone from docs/NODE_PALETTE_PLAN.md
// ("Datei schreiben/lesen und Aenderungen an einer beobachteten Datei loesen
// einen Flow aus") against the real file/filein/watch node implementations -
// not mocks - deployed through a real FlowEngine, using an isolated registry
// (per internal/registry/AGENTS.md's testing guidance).
func TestPhase5_WriteWatchReadRoundTrip(t *testing.T) {
    dir := t.TempDir()
    path := filepath.Join(dir, "watched.txt")

    reg := registry.NewNodeRegistry()
    require.NoError(t, reg.RegisterFactory("up", func() registry.NodeExecutor { return passthroughNode{} }, registry.NodeMetadata{Type: "up"}))
    require.NoError(t, reg.RegisterFactory("file", func() registry.NodeExecutor { return &file.Node{} }, registry.NodeMetadata{Type: "file"}))
    require.NoError(t, reg.RegisterFactory("watch", func() registry.NodeExecutor { return &watch.Node{} }, registry.NodeMetadata{Type: "watch"}))
    require.NoError(t, reg.RegisterFactory("readTrigger", func() registry.NodeExecutor { return passthroughNode{} }, registry.NodeMetadata{Type: "readTrigger"}))
    require.NoError(t, reg.RegisterFactory("filein", func() registry.NodeExecutor { return &filein.Node{} }, registry.NodeMetadata{Type: "filein"}))

    watchSink := &captureSinkNode{}
    readSink := &captureSinkNode{}
    require.NoError(t, reg.RegisterFactory("watch-sink", func() registry.NodeExecutor { return watchSink }, registry.NodeMetadata{Type: "watch-sink"}))
    require.NoError(t, reg.RegisterFactory("read-sink", func() registry.NodeExecutor { return readSink }, registry.NodeMetadata{Type: "read-sink"}))

    e := newTestEngine(reg)
    defer e.Stop()

    flow := NewFlow("phase5-storage", "Phase 5 Storage")
    flow.Nodes["up"] = &Node{ID: "up", Type: "up"}
    flow.Nodes["writer"] = &Node{ID: "writer", Type: "file", Config: map[string]interface{}{
        "filename": map[string]interface{}{"type": "str", "value": path},
        "action":   "overwrite",
        "encoding": "utf8",
    }}
    flow.Nodes["watcher"] = &Node{ID: "watcher", Type: "watch", Config: map[string]interface{}{
        "files": dir,
    }}
    flow.Nodes["watchSink"] = &Node{ID: "watchSink", Type: "watch-sink"}
    flow.Nodes["readTrigger"] = &Node{ID: "readTrigger", Type: "readTrigger"}
    flow.Nodes["reader"] = &Node{ID: "reader", Type: "filein", Config: map[string]interface{}{
        "filename": map[string]interface{}{"type": "str", "value": path},
        "format":   "utf8",
    }}
    flow.Nodes["readSink"] = &Node{ID: "readSink", Type: "read-sink"}
    flow.Connections = []NodeConnection{
        {ID: "c1", SourceNode: "up", TargetNode: "writer"},
        {ID: "c2", SourceNode: "watcher", TargetNode: "watchSink"},
        {ID: "c3", SourceNode: "readTrigger", TargetNode: "reader"},
        {ID: "c4", SourceNode: "reader", TargetNode: "readSink"},
    }

    require.NoError(t, e.Deploy(flow))
    defer e.Undeploy(flow.ID)

    // Give the watch node's Start goroutine a moment to register its
    // fsnotify watch before triggering the write it needs to see.
    time.Sleep(50 * time.Millisecond)

    require.NoError(t, e.InjectMessage(flow.ID, "up", map[string]interface{}{"payload": "hello world"}))

    require.Eventually(t, func() bool { return watchSink.receivedCount() >= 1 }, 2*time.Second, 10*time.Millisecond,
        "watch node should fire after the file node writes to the watched directory")
    assert.Equal(t, path, watchSink.last()["filename"])

    require.NoError(t, e.InjectMessage(flow.ID, "readTrigger", map[string]interface{}{}))
    require.Eventually(t, func() bool { return readSink.receivedCount() >= 1 }, 2*time.Second, 10*time.Millisecond,
        "file in node should read back what the file node wrote")
    assert.Equal(t, "hello world", readSink.last()["payload"])
}
