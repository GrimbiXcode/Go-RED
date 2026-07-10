package execnode

import (
    "context"
    "strings"
    "testing"
    "time"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestNode_ConfigRoundTrip(t *testing.T) {
    n := &Node{}
    require.NoError(t, n.SetConfig(map[string]interface{}{
        "command": "echo", "args": []interface{}{"a", "b"}, "appendPayload": true, "timeoutMs": float64(1000),
    }))
    assert.Equal(t, "echo", n.Command)
    assert.Equal(t, []string{"a", "b"}, n.Args)
    assert.True(t, n.AppendPayload)
    assert.Equal(t, int64(1000), n.TimeoutMs)

    cfg := n.GetConfig()
    assert.Equal(t, "echo", cfg["command"])
}

func TestNode_SetConfig_RequiresCommand(t *testing.T) {
    n := &Node{}
    err := n.SetConfig(map[string]interface{}{})
    assert.Error(t, err, "command is required")
}

func TestNode_Validate(t *testing.T) {
    assert.Error(t, (&Node{}).Validate())
    assert.Error(t, (&Node{Command: "echo", TimeoutMs: -1}).Validate())
    assert.NoError(t, (&Node{Command: "echo"}).Validate())
}

func TestNode_Execute_DisabledByDefault(t *testing.T) {
    n := &Node{Command: "echo", TimeoutMs: 5000}
    _, err := n.Execute(context.Background(), map[string]interface{}{})
    require.Error(t, err)
    assert.Contains(t, err.Error(), enableEnvVar)
}

func TestNode_Execute_CapturesStdout(t *testing.T) {
    t.Setenv(enableEnvVar, "1")
    n := &Node{Command: "echo", Args: []string{"hello"}, TimeoutMs: 5000}

    output, err := n.Execute(context.Background(), map[string]interface{}{})
    require.NoError(t, err)
    assert.Equal(t, "hello\n", output["payload"])
    assert.Equal(t, "", output["stderr"])
    assert.Equal(t, float64(0), output["exitCode"])
}

func TestNode_Execute_AppendPayload(t *testing.T) {
    t.Setenv(enableEnvVar, "1")

    t.Run("string payload becomes one extra argument", func(t *testing.T) {
        n := &Node{Command: "echo", Args: []string{"fixed"}, AppendPayload: true, TimeoutMs: 5000}
        output, err := n.Execute(context.Background(), map[string]interface{}{"payload": "dynamic"})
        require.NoError(t, err)
        assert.Equal(t, "fixed dynamic\n", output["payload"])
    })

    t.Run("array payload becomes one argument per element", func(t *testing.T) {
        n := &Node{Command: "echo", AppendPayload: true, TimeoutMs: 5000}
        output, err := n.Execute(context.Background(), map[string]interface{}{"payload": []interface{}{"a", "b", "c"}})
        require.NoError(t, err)
        assert.Equal(t, "a b c\n", output["payload"])
    })

    t.Run("non-string array element errors", func(t *testing.T) {
        n := &Node{Command: "echo", AppendPayload: true, TimeoutMs: 5000}
        _, err := n.Execute(context.Background(), map[string]interface{}{"payload": []interface{}{float64(1)}})
        assert.Error(t, err)
    })

    t.Run("unsupported payload type errors", func(t *testing.T) {
        n := &Node{Command: "echo", AppendPayload: true, TimeoutMs: 5000}
        _, err := n.Execute(context.Background(), map[string]interface{}{"payload": float64(42)})
        assert.Error(t, err)
    })
}

func TestNode_Execute_ShellMetacharactersAreNotInterpreted(t *testing.T) {
    t.Setenv(enableEnvVar, "1")
    // If this were ever run through a shell (e.g. "sh -c echo "+payload),
    // this payload would attempt command substitution/chaining. Because
    // exec.CommandContext passes it as a single literal argv entry, echo
    // must print it back verbatim instead.
    dangerous := "; rm -rf / #`id`$(whoami)"
    n := &Node{Command: "echo", AppendPayload: true, TimeoutMs: 5000}

    output, err := n.Execute(context.Background(), map[string]interface{}{"payload": dangerous})
    require.NoError(t, err)
    assert.Equal(t, dangerous+"\n", output["payload"])
}

func TestNode_Execute_NonZeroExitCode(t *testing.T) {
    t.Setenv(enableEnvVar, "1")
    n := &Node{Command: "false", TimeoutMs: 5000}

    output, err := n.Execute(context.Background(), map[string]interface{}{})
    require.NoError(t, err, "a non-zero exit code is reported via exitCode, not a Go error")
    assert.Equal(t, float64(1), output["exitCode"])
}

func TestNode_Execute_CommandNotFound(t *testing.T) {
    t.Setenv(enableEnvVar, "1")
    n := &Node{Command: "this-binary-should-not-exist-anywhere-xyz", TimeoutMs: 5000}

    _, err := n.Execute(context.Background(), map[string]interface{}{})
    assert.Error(t, err)
}

func TestNode_Execute_Timeout(t *testing.T) {
    t.Setenv(enableEnvVar, "1")
    n := &Node{Command: "sleep", Args: []string{"5"}, TimeoutMs: 50}

    start := time.Now()
    _, err := n.Execute(context.Background(), map[string]interface{}{})
    elapsed := time.Since(start)

    require.Error(t, err)
    assert.Contains(t, err.Error(), "timed out")
    assert.Less(t, elapsed, 2*time.Second, "should time out quickly, not wait for the full sleep")
}

func TestNode_Execute_RespectsParentContextCancellation(t *testing.T) {
    t.Setenv(enableEnvVar, "1")
    n := &Node{Command: "sleep", Args: []string{"5"}, TimeoutMs: 5000}

    ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
    defer cancel()

    start := time.Now()
    _, err := n.Execute(ctx, map[string]interface{}{})
    elapsed := time.Since(start)

    require.Error(t, err)
    assert.Less(t, elapsed, 2*time.Second)
}

func TestNode_Execute_NoCommandConfiguredErrors(t *testing.T) {
    t.Setenv(enableEnvVar, "1")
    n := &Node{}
    _, err := n.Execute(context.Background(), map[string]interface{}{})
    assert.Error(t, err)
}

func TestLimitedBuffer(t *testing.T) {
    t.Run("caps writes at max bytes", func(t *testing.T) {
        var w limitedBuffer
        w.max = 5
        n, err := w.Write([]byte("hello world"))
        require.NoError(t, err)
        assert.Equal(t, len("hello world"), n, "reports all bytes consumed even though only some were kept")
        assert.Equal(t, "hello", w.String())
    })

    t.Run("further writes past the cap are discarded", func(t *testing.T) {
        var w limitedBuffer
        w.max = 5
        _, _ = w.Write([]byte("hello"))
        _, err := w.Write([]byte(" world"))
        require.NoError(t, err)
        assert.Equal(t, "hello", w.String())
    })

    t.Run("writes under the cap accumulate normally", func(t *testing.T) {
        var w limitedBuffer
        w.max = 100
        _, _ = w.Write([]byte("a"))
        _, _ = w.Write([]byte("b"))
        assert.Equal(t, "ab", w.String())
    })
}

func TestPayloadArgs(t *testing.T) {
    tests := []struct {
        name    string
        payload interface{}
        want    []string
        wantErr bool
    }{
        {"nil payload", nil, nil, false},
        {"string payload", "x", []string{"x"}, false},
        {"string array payload", []interface{}{"a", "b"}, []string{"a", "b"}, false},
        {"non-string array element", []interface{}{1}, nil, true},
        {"unsupported type", 42, nil, true},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got, err := payloadArgs(tt.payload)
            if tt.wantErr {
                assert.Error(t, err)
                return
            }
            require.NoError(t, err)
            assert.Equal(t, tt.want, got)
        })
    }
}

func TestNode_Execute_StderrCaptured(t *testing.T) {
    t.Setenv(enableEnvVar, "1")
    // "ls" of a path that doesn't exist writes to stderr and exits non-zero
    // - a portable (GNU/BSD) way to exercise the stderr + exitCode path
    // without a shell.
    n := &Node{Command: "ls", Args: []string{"/this/path/should/not/exist/xyz"}, TimeoutMs: 5000}

    output, err := n.Execute(context.Background(), map[string]interface{}{})
    require.NoError(t, err)
    assert.NotEqual(t, float64(0), output["exitCode"])
    stderr, _ := output["stderr"].(string)
    assert.True(t, strings.TrimSpace(stderr) != "", "expected some stderr output")
}
