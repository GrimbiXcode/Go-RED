package template

import (
    "context"
    "testing"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestNode_ConfigRoundTrip(t *testing.T) {
    n := &Node{}
    require.NoError(t, n.SetConfig(map[string]interface{}{
        "field": "topic", "template": "hi {{payload}}", "syntax": "mustache", "output": "str",
    }))
    assert.Equal(t, "topic", n.Field)
    assert.Equal(t, "hi {{payload}}", n.Template)

    cfg := n.GetConfig()
    assert.Equal(t, "topic", cfg["field"])
}

func TestNode_SetConfig_Defaults(t *testing.T) {
    n := &Node{}
    require.NoError(t, n.SetConfig(map[string]interface{}{}))
    assert.Equal(t, "payload", n.Field)
    assert.Equal(t, "mustache", n.Syntax)
    assert.Equal(t, "str", n.OutputFormat)
}

func TestNode_Validate(t *testing.T) {
    n := &Node{Syntax: "bogus"}
    assert.Error(t, n.Validate())

    n = &Node{OutputFormat: "bogus"}
    assert.Error(t, n.Validate())

    n = &Node{Syntax: "plain", OutputFormat: "json"}
    assert.NoError(t, n.Validate())
}

func TestNode_Execute_SimpleSubstitution(t *testing.T) {
    n := &Node{Field: "payload", Template: "hello {{payload}}!", Syntax: "mustache", OutputFormat: "str"}
    output, err := n.Execute(context.Background(), map[string]interface{}{"payload": "world"})
    require.NoError(t, err)
    assert.Equal(t, "hello world!", output["payload"])
}

func TestNode_Execute_NestedPath(t *testing.T) {
    n := &Node{Field: "greeting", Template: "hi {{payload.name}}", Syntax: "mustache", OutputFormat: "str"}
    input := map[string]interface{}{"payload": map[string]interface{}{"name": "Ada"}}
    output, err := n.Execute(context.Background(), input)
    require.NoError(t, err)
    assert.Equal(t, "hi Ada", output["greeting"])
    // original field is untouched since Field targets a different property
    assert.Equal(t, input["payload"], output["payload"])
}

func TestNode_Execute_MissingPropertyRendersEmpty(t *testing.T) {
    n := &Node{Field: "payload", Template: "[{{payload.missing}}]", Syntax: "mustache", OutputFormat: "str"}
    output, err := n.Execute(context.Background(), map[string]interface{}{})
    require.NoError(t, err)
    assert.Equal(t, "[]", output["payload"])
}

func TestNode_Execute_NumberFormatting(t *testing.T) {
    n := &Node{Field: "payload", Template: "count={{payload}}", Syntax: "mustache", OutputFormat: "str"}

    output, err := n.Execute(context.Background(), map[string]interface{}{"payload": float64(5)})
    require.NoError(t, err)
    assert.Equal(t, "count=5", output["payload"])

    output, err = n.Execute(context.Background(), map[string]interface{}{"payload": float64(5.5)})
    require.NoError(t, err)
    assert.Equal(t, "count=5.5", output["payload"])
}

func TestNode_Execute_PlainSyntaxDoesNotSubstitute(t *testing.T) {
    n := &Node{Field: "payload", Template: "literal {{payload}}", Syntax: "plain"}
    output, err := n.Execute(context.Background(), map[string]interface{}{"payload": "x"})
    require.NoError(t, err)
    assert.Equal(t, "literal {{payload}}", output["payload"])
}

func TestNode_Execute_JSONOutput(t *testing.T) {
    t.Run("valid JSON is parsed into the target property", func(t *testing.T) {
        n := &Node{Field: "payload", Template: `{"count": {{payload}}}`, Syntax: "mustache", OutputFormat: "json"}
        output, err := n.Execute(context.Background(), map[string]interface{}{"payload": float64(3)})
        require.NoError(t, err)
        parsed, ok := output["payload"].(map[string]interface{})
        require.True(t, ok)
        assert.Equal(t, float64(3), parsed["count"])
    })

    t.Run("invalid JSON errors", func(t *testing.T) {
        n := &Node{Field: "payload", Template: "not json", Syntax: "plain", OutputFormat: "json"}
        _, err := n.Execute(context.Background(), map[string]interface{}{})
        assert.Error(t, err)
    })
}

func TestNode_Execute_DifferentValueTypes(t *testing.T) {
    tests := []struct {
        name     string
        payload  interface{}
        expected string
    }{
        {"bool true", true, "true"},
        {"nested object renders as JSON", map[string]interface{}{"a": float64(1)}, `{"a":1}`},
        {"array renders as JSON", []interface{}{"a", "b"}, `["a","b"]`},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            n := &Node{Field: "out", Template: "{{payload}}", Syntax: "mustache", OutputFormat: "str"}
            output, err := n.Execute(context.Background(), map[string]interface{}{"payload": tt.payload})
            require.NoError(t, err)
            assert.Equal(t, tt.expected, output["out"])
        })
    }
}
