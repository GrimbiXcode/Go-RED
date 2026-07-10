package change

import (
    "context"
    "testing"

    "github.com/GrimbiXcode/Go-RED/internal/registry"
    "github.com/GrimbiXcode/Go-RED/internal/typedvalue"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestNode_ConfigRoundTrip(t *testing.T) {
    n := &Node{}
    require.NoError(t, n.SetConfig(map[string]interface{}{
        "rules": []interface{}{
            map[string]interface{}{
                "action": "set",
                "target": map[string]interface{}{"type": "msg", "path": "payload"},
                "value":  map[string]interface{}{"type": "str", "value": "hi"},
            },
        },
    }))

    require.Len(t, n.Rules, 1)
    assert.Equal(t, "set", n.Rules[0].Action)
    assert.Equal(t, typedvalue.PropertyRef{Type: typedvalue.TypeMsg, Path: "payload"}, n.Rules[0].Target)
    assert.Equal(t, typedvalue.Value{Type: typedvalue.TypeString, Value: "hi"}, n.Rules[0].Value)

    cfg := n.GetConfig()
    rules, ok := cfg["rules"].([]interface{})
    require.True(t, ok)
    require.Len(t, rules, 1)
}

func TestNode_Execute_Set(t *testing.T) {
    n := &Node{Rules: []Rule{{
        Action: "set",
        Target: typedvalue.PropertyRef{Type: typedvalue.TypeMsg, Path: "payload"},
        Value:  typedvalue.Value{Type: typedvalue.TypeString, Value: "new"},
    }}}

    output, err := n.Execute(context.Background(), map[string]interface{}{"payload": "old"})
    require.NoError(t, err)
    assert.Equal(t, "new", output["payload"])
}

func TestNode_Execute_SetNestedCreatesIntermediates(t *testing.T) {
    n := &Node{Rules: []Rule{{
        Action: "set",
        Target: typedvalue.PropertyRef{Type: typedvalue.TypeMsg, Path: "a.b.c"},
        Value:  typedvalue.Value{Type: typedvalue.TypeNumber, Value: "42"},
    }}}

    output, err := n.Execute(context.Background(), map[string]interface{}{})
    require.NoError(t, err)

    a := output["a"].(map[string]interface{})
    b := a["b"].(map[string]interface{})
    assert.Equal(t, float64(42), b["c"])
}

func TestNode_Execute_Delete(t *testing.T) {
    n := &Node{Rules: []Rule{{
        Action: "delete",
        Target: typedvalue.PropertyRef{Type: typedvalue.TypeMsg, Path: "secret"},
    }}}

    output, err := n.Execute(context.Background(), map[string]interface{}{"payload": "x", "secret": "y"})
    require.NoError(t, err)
    _, exists := output["secret"]
    assert.False(t, exists)
    assert.Equal(t, "x", output["payload"])
}

func TestNode_Execute_Move(t *testing.T) {
    n := &Node{Rules: []Rule{{
        Action: "move",
        Target: typedvalue.PropertyRef{Type: typedvalue.TypeMsg, Path: "oldName"},
        MoveTo: typedvalue.PropertyRef{Type: typedvalue.TypeMsg, Path: "newName"},
    }}}

    output, err := n.Execute(context.Background(), map[string]interface{}{"oldName": "value"})
    require.NoError(t, err)
    _, exists := output["oldName"]
    assert.False(t, exists)
    assert.Equal(t, "value", output["newName"])

    t.Run("missing source is a no-op", func(t *testing.T) {
        output, err := n.Execute(context.Background(), map[string]interface{}{})
        require.NoError(t, err)
        _, exists := output["newName"]
        assert.False(t, exists)
    })
}

func TestNode_Execute_Change(t *testing.T) {
    t.Run("plain string replace", func(t *testing.T) {
        n := &Node{Rules: []Rule{{
            Action: "change",
            Target: typedvalue.PropertyRef{Type: typedvalue.TypeMsg, Path: "payload"},
            From:   "foo",
            To:     "bar",
        }}}
        output, err := n.Execute(context.Background(), map[string]interface{}{"payload": "foo baz foo"})
        require.NoError(t, err)
        assert.Equal(t, "bar baz bar", output["payload"])
    })

    t.Run("regex replace", func(t *testing.T) {
        n := &Node{Rules: []Rule{{
            Action:    "change",
            Target:    typedvalue.PropertyRef{Type: typedvalue.TypeMsg, Path: "payload"},
            From:      `\d+`,
            To:        "#",
            FromRegex: true,
        }}}
        output, err := n.Execute(context.Background(), map[string]interface{}{"payload": "a1 b22 c333"})
        require.NoError(t, err)
        assert.Equal(t, "a# b# c#", output["payload"])
    })

    t.Run("invalid regex errors", func(t *testing.T) {
        n := &Node{Rules: []Rule{{
            Action:    "change",
            Target:    typedvalue.PropertyRef{Type: typedvalue.TypeMsg, Path: "payload"},
            From:      "(",
            FromRegex: true,
        }}}
        _, err := n.Execute(context.Background(), map[string]interface{}{"payload": "x"})
        assert.Error(t, err)
    })

    t.Run("non-string target is a no-op, not an error", func(t *testing.T) {
        n := &Node{Rules: []Rule{{
            Action: "change",
            Target: typedvalue.PropertyRef{Type: typedvalue.TypeMsg, Path: "payload"},
            From:   "1",
            To:     "2",
        }}}
        output, err := n.Execute(context.Background(), map[string]interface{}{"payload": float64(1)})
        require.NoError(t, err)
        assert.Equal(t, float64(1), output["payload"])
    })
}

func TestNode_Execute_MultipleRulesInOrder(t *testing.T) {
    n := &Node{Rules: []Rule{
        {Action: "set", Target: typedvalue.PropertyRef{Type: typedvalue.TypeMsg, Path: "payload"}, Value: typedvalue.Value{Type: typedvalue.TypeString, Value: "step1"}},
        {Action: "change", Target: typedvalue.PropertyRef{Type: typedvalue.TypeMsg, Path: "payload"}, From: "step1", To: "step2"},
    }}

    output, err := n.Execute(context.Background(), map[string]interface{}{})
    require.NoError(t, err)
    assert.Equal(t, "step2", output["payload"])
}

func TestNode_Execute_UnknownActionErrors(t *testing.T) {
    n := &Node{Rules: []Rule{{Action: "bogus"}}}
    _, err := n.Execute(context.Background(), map[string]interface{}{})
    assert.Error(t, err)
}

func TestNode_Execute_FlowContext(t *testing.T) {
    t.Run("set writes to flow context and a later rule can read it back", func(t *testing.T) {
        flowCtx := registry.NewContextStore()
        rt := registry.NewNodeRuntime("f1", "n1", "change", flowCtx, nil, nil, nil, nil)
        ctx := registry.WithRuntime(context.Background(), rt)

        n := &Node{Rules: []Rule{
            {Action: "set", Target: typedvalue.PropertyRef{Type: typedvalue.TypeFlow, Path: "counter"}, Value: typedvalue.Value{Type: typedvalue.TypeNumber, Value: "1"}},
            {Action: "set", Target: typedvalue.PropertyRef{Type: typedvalue.TypeMsg, Path: "payload"}, Value: typedvalue.Value{Type: typedvalue.TypeFlow, Value: "counter"}},
        }}

        output, err := n.Execute(ctx, map[string]interface{}{})
        require.NoError(t, err)
        assert.Equal(t, float64(1), output["payload"])

        v, ok := flowCtx.Get("counter")
        require.True(t, ok)
        assert.Equal(t, float64(1), v)
    })

    t.Run("writing to flow context without a NodeRuntime errors", func(t *testing.T) {
        n := &Node{Rules: []Rule{
            {Action: "set", Target: typedvalue.PropertyRef{Type: typedvalue.TypeFlow, Path: "counter"}, Value: typedvalue.Value{Type: typedvalue.TypeNumber, Value: "1"}},
        }}
        _, err := n.Execute(context.Background(), map[string]interface{}{})
        assert.Error(t, err)
    })
}
