package filein

import (
    "context"
    "os"
    "path/filepath"
    "testing"

    "github.com/GrimbiXcode/Go-RED/internal/registry"
    "github.com/GrimbiXcode/Go-RED/internal/typedvalue"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestNode_ConfigRoundTrip(t *testing.T) {
    n := &Node{}
    require.NoError(t, n.SetConfig(map[string]interface{}{
        "filename": map[string]interface{}{"type": "str", "value": "/tmp/in.txt"},
        "format":   "lines",
        "allProps": true,
    }))
    assert.Equal(t, typedvalue.Value{Type: typedvalue.TypeString, Value: "/tmp/in.txt"}, n.Filename)
    assert.Equal(t, "lines", n.Format)
    assert.True(t, n.AllProps)
}

func TestNode_Validate(t *testing.T) {
    assert.Error(t, (&Node{Format: "bogus"}).Validate())
    assert.NoError(t, (&Node{Format: ""}).Validate())
    assert.NoError(t, (&Node{Format: "utf8"}).Validate())
    assert.NoError(t, (&Node{Format: "lines"}).Validate())
}

func TestNode_ExecuteMulti_RawBytes(t *testing.T) {
    dir := t.TempDir()
    path := filepath.Join(dir, "data.bin")
    require.NoError(t, os.WriteFile(path, []byte{0x00, 0x01, 0xFF}, 0o644))

    n := &Node{Filename: typedvalue.Value{Type: typedvalue.TypeString, Value: path}}
    outputs, err := n.ExecuteMulti(nil, map[string]interface{}{})
    require.NoError(t, err)
    require.Contains(t, outputs, "output")
    assert.Equal(t, []byte{0x00, 0x01, 0xFF}, outputs["output"]["payload"])
    assert.Equal(t, path, outputs["output"]["filename"])
}

func TestNode_ExecuteMulti_UTF8(t *testing.T) {
    dir := t.TempDir()
    path := filepath.Join(dir, "data.txt")
    require.NoError(t, os.WriteFile(path, []byte("hello world"), 0o644))

    n := &Node{Filename: typedvalue.Value{Type: typedvalue.TypeString, Value: path}, Format: "utf8"}
    outputs, err := n.ExecuteMulti(nil, map[string]interface{}{"topic": "t"})
    require.NoError(t, err)
    require.Contains(t, outputs, "output")
    assert.Equal(t, "hello world", outputs["output"]["payload"])
    assert.Equal(t, "t", outputs["output"]["topic"])
}

// collectSubmits wraps a NodeRuntime whose SubmitToNode calls are recorded,
// used to verify lines 2..N of a "lines"-mode read are dispatched via
// SubmitToNode - the same same-port fan-out technique internal/nodes/split
// uses.
func collectSubmits() (context.Context, *[]map[string]interface{}) {
    var submitted []map[string]interface{}
    rt := registry.NewNodeRuntime("f1", "filein-1", "file in", nil, nil, nil, func(nodeID string, payload map[string]interface{}) {
        submitted = append(submitted, payload)
    }, nil)
    return registry.WithRuntime(context.Background(), rt), &submitted
}

func TestNode_ExecuteMulti_Lines(t *testing.T) {
    dir := t.TempDir()
    path := filepath.Join(dir, "lines.txt")
    require.NoError(t, os.WriteFile(path, []byte("a\nb\nc"), 0o644))

    n := &Node{Filename: typedvalue.Value{Type: typedvalue.TypeString, Value: path}, Format: "lines"}
    ctx, submitted := collectSubmits()

    outputs, err := n.ExecuteMulti(ctx, map[string]interface{}{"topic": "t"})
    require.NoError(t, err)
    require.Contains(t, outputs, "output")
    first := outputs["output"]
    assert.Equal(t, "a", first["payload"])
    assert.Equal(t, "t", first["topic"])
    firstParts := first["parts"].(map[string]interface{})
    assert.Equal(t, float64(0), firstParts["index"])
    assert.Equal(t, float64(3), firstParts["count"])

    require.Len(t, *submitted, 2)
    assert.Equal(t, "b", (*submitted)[0]["payload"])
    assert.Equal(t, "c", (*submitted)[1]["payload"])
    lastParts := (*submitted)[1]["parts"].(map[string]interface{})
    assert.Equal(t, float64(2), lastParts["index"])
}

func TestNode_ExecuteMulti_Lines_AllProps(t *testing.T) {
    dir := t.TempDir()
    path := filepath.Join(dir, "lines.txt")
    require.NoError(t, os.WriteFile(path, []byte("a\nb"), 0o644))

    n := &Node{Filename: typedvalue.Value{Type: typedvalue.TypeString, Value: path}, Format: "lines", AllProps: true}
    ctx, submitted := collectSubmits()

    outputs, err := n.ExecuteMulti(ctx, map[string]interface{}{"topic": "t", "extra": "kept"})
    require.NoError(t, err)
    assert.Equal(t, "kept", outputs["output"]["extra"])
    require.Len(t, *submitted, 1)
    assert.Equal(t, "kept", (*submitted)[0]["extra"])
}

func TestNode_ExecuteMulti_NoFilename_NoOp(t *testing.T) {
    n := &Node{Filename: typedvalue.Value{Type: typedvalue.TypeString, Value: ""}}
    outputs, err := n.ExecuteMulti(nil, map[string]interface{}{})
    require.NoError(t, err)
    assert.Empty(t, outputs)
}

func TestNode_ExecuteMulti_MissingFile_Errors(t *testing.T) {
    n := &Node{Filename: typedvalue.Value{Type: typedvalue.TypeString, Value: "/nonexistent/path/does-not-exist.txt"}}
    _, err := n.ExecuteMulti(nil, map[string]interface{}{})
    assert.Error(t, err)
}
