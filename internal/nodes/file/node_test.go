package file

import (
    "os"
    "path/filepath"
    "testing"

    "github.com/GrimbiXcode/Go-RED/internal/typedvalue"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestNode_ConfigRoundTrip(t *testing.T) {
    n := &Node{}
    require.NoError(t, n.SetConfig(map[string]interface{}{
        "filename":      map[string]interface{}{"type": "str", "value": "/tmp/x.txt"},
        "action":        "overwrite",
        "appendNewline": true,
        "createDir":     true,
        "encoding":      "base64",
    }))
    assert.Equal(t, typedvalue.Value{Type: typedvalue.TypeString, Value: "/tmp/x.txt"}, n.Filename)
    assert.Equal(t, "overwrite", n.Action)
    assert.True(t, n.AppendNewline)
    assert.True(t, n.CreateDir)
    assert.Equal(t, "base64", n.Encoding)
}

func TestNode_SetConfig_Defaults(t *testing.T) {
    n := &Node{}
    require.NoError(t, n.SetConfig(map[string]interface{}{}))
    assert.Equal(t, "append", n.Action)
    assert.Equal(t, "utf8", n.Encoding)
}

func TestNode_Validate(t *testing.T) {
    assert.Error(t, (&Node{Action: "bogus"}).Validate())
    assert.NoError(t, (&Node{Action: "append"}).Validate())
    assert.Error(t, (&Node{Action: "append", Encoding: "bogus"}).Validate())
}

func TestNode_ExecuteMulti_AppendThenOverwrite(t *testing.T) {
    dir := t.TempDir()
    path := filepath.Join(dir, "out.txt")

    n := &Node{Filename: typedvalue.Value{Type: typedvalue.TypeString, Value: path}, Action: "append", AppendNewline: true, Encoding: "utf8"}

    outputs, err := n.ExecuteMulti(nil, map[string]interface{}{"payload": "hello"})
    require.NoError(t, err)
    require.Contains(t, outputs, "output")
    assert.Equal(t, path, outputs["output"]["filename"])

    outputs, err = n.ExecuteMulti(nil, map[string]interface{}{"payload": "world"})
    require.NoError(t, err)
    require.Contains(t, outputs, "output")

    data, err := os.ReadFile(path)
    require.NoError(t, err)
    assert.Equal(t, "hello\nworld\n", string(data))

    n2 := &Node{Filename: typedvalue.Value{Type: typedvalue.TypeString, Value: path}, Action: "overwrite"}
    outputs, err = n2.ExecuteMulti(nil, map[string]interface{}{"payload": "replaced"})
    require.NoError(t, err)
    require.Contains(t, outputs, "output")

    data, err = os.ReadFile(path)
    require.NoError(t, err)
    assert.Equal(t, "replaced", string(data))
}

func TestNode_ExecuteMulti_Delete(t *testing.T) {
    dir := t.TempDir()
    path := filepath.Join(dir, "gone.txt")
    require.NoError(t, os.WriteFile(path, []byte("bye"), 0o644))

    n := &Node{Filename: typedvalue.Value{Type: typedvalue.TypeString, Value: path}, Action: "delete"}
    outputs, err := n.ExecuteMulti(nil, map[string]interface{}{})
    require.NoError(t, err)
    require.Contains(t, outputs, "output")

    _, statErr := os.Stat(path)
    assert.True(t, os.IsNotExist(statErr))

    // Deleting an already-missing file is not an error.
    outputs, err = n.ExecuteMulti(nil, map[string]interface{}{})
    require.NoError(t, err)
    require.Contains(t, outputs, "output")
}

func TestNode_ExecuteMulti_CreateDir(t *testing.T) {
    dir := t.TempDir()
    path := filepath.Join(dir, "nested", "deep", "out.txt")

    n := &Node{Filename: typedvalue.Value{Type: typedvalue.TypeString, Value: path}, Action: "overwrite", CreateDir: true}
    outputs, err := n.ExecuteMulti(nil, map[string]interface{}{"payload": "x"})
    require.NoError(t, err)
    require.Contains(t, outputs, "output")

    data, err := os.ReadFile(path)
    require.NoError(t, err)
    assert.Equal(t, "x", string(data))
}

func TestNode_ExecuteMulti_NoFilename_NoOp(t *testing.T) {
    n := &Node{Filename: typedvalue.Value{Type: typedvalue.TypeString, Value: ""}, Action: "append"}
    outputs, err := n.ExecuteMulti(nil, map[string]interface{}{"payload": "x"})
    require.NoError(t, err)
    assert.Empty(t, outputs)
}

func TestNode_ExecuteMulti_NoPayload_NoOp(t *testing.T) {
    dir := t.TempDir()
    path := filepath.Join(dir, "untouched.txt")

    n := &Node{Filename: typedvalue.Value{Type: typedvalue.TypeString, Value: path}, Action: "append"}
    outputs, err := n.ExecuteMulti(nil, map[string]interface{}{})
    require.NoError(t, err)
    assert.Empty(t, outputs)

    _, statErr := os.Stat(path)
    assert.True(t, os.IsNotExist(statErr))
}

func TestNode_ExecuteMulti_JSONPayload(t *testing.T) {
    dir := t.TempDir()
    path := filepath.Join(dir, "out.json")

    n := &Node{Filename: typedvalue.Value{Type: typedvalue.TypeString, Value: path}, Action: "overwrite"}
    outputs, err := n.ExecuteMulti(nil, map[string]interface{}{"payload": map[string]interface{}{"a": float64(1)}})
    require.NoError(t, err)
    require.Contains(t, outputs, "output")

    data, err := os.ReadFile(path)
    require.NoError(t, err)
    assert.JSONEq(t, `{"a":1}`, string(data))
}

func TestNode_Execute_DelegatesToExecuteMulti(t *testing.T) {
    dir := t.TempDir()
    path := filepath.Join(dir, "out.txt")

    n := &Node{Filename: typedvalue.Value{Type: typedvalue.TypeString, Value: path}, Action: "overwrite"}
    out, err := n.Execute(nil, map[string]interface{}{"payload": "x"})
    require.NoError(t, err)
    assert.Equal(t, path, out["filename"])
}
