package switchnode

import (
    "context"
    "testing"

    "github.com/GrimbiXcode/Go-RED/internal/registry"
    "github.com/GrimbiXcode/Go-RED/internal/typedvalue"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func numRule(op, val string) Rule {
    return Rule{Operator: op, Value: typedvalue.Value{Type: typedvalue.TypeNumber, Value: val}}
}

func TestNode_ConfigRoundTrip(t *testing.T) {
    n := &Node{}
    require.NoError(t, n.SetConfig(map[string]interface{}{
        "property": map[string]interface{}{"type": "msg", "path": "topic"},
        "checkAll": false,
        "rules": []interface{}{
            map[string]interface{}{"operator": "eq", "value": map[string]interface{}{"type": "str", "value": "a"}},
        },
    }))

    assert.Equal(t, typedvalue.PropertyRef{Type: typedvalue.TypeMsg, Path: "topic"}, n.Property)
    assert.False(t, n.CheckAll)
    require.Len(t, n.Rules, 1)
    assert.Equal(t, "eq", n.Rules[0].Operator)
    assert.Equal(t, typedvalue.Value{Type: typedvalue.TypeString, Value: "a"}, n.Rules[0].Value)

    cfg := n.GetConfig()
    assert.Equal(t, false, cfg["checkAll"])
}

func TestNode_SetConfig_Defaults(t *testing.T) {
    n := &Node{}
    require.NoError(t, n.SetConfig(map[string]interface{}{}))
    assert.Equal(t, typedvalue.PropertyRef{Type: typedvalue.TypeMsg, Path: "payload"}, n.Property)
    assert.True(t, n.CheckAll, "checkAll defaults to true, matching Node-RED")
    assert.Empty(t, n.Rules)
}

func TestNode_ExecuteMulti_BasicRouting(t *testing.T) {
    t.Run("routes to the port matching the single matching rule", func(t *testing.T) {
        n := &Node{
            Property: typedvalue.PropertyRef{Type: typedvalue.TypeMsg, Path: "payload"},
            CheckAll: true,
            Rules: []Rule{
                numRule("eq", "1"),
                numRule("eq", "2"),
            },
        }

        outputs, err := n.ExecuteMulti(context.Background(), map[string]interface{}{"payload": float64(2)})
        require.NoError(t, err)
        assert.NotContains(t, outputs, "0")
        require.Contains(t, outputs, "1")
        assert.Equal(t, float64(2), outputs["1"]["payload"])
    })

    t.Run("checkAll=true routes to every matching rule", func(t *testing.T) {
        n := &Node{
            Property: typedvalue.PropertyRef{Type: typedvalue.TypeMsg, Path: "payload"},
            CheckAll: true,
            Rules: []Rule{
                {Operator: "gt", Value: typedvalue.Value{Type: typedvalue.TypeNumber, Value: "0"}},
                {Operator: "lt", Value: typedvalue.Value{Type: typedvalue.TypeNumber, Value: "10"}},
            },
        }

        outputs, err := n.ExecuteMulti(context.Background(), map[string]interface{}{"payload": float64(5)})
        require.NoError(t, err)
        assert.Len(t, outputs, 2)
    })

    t.Run("checkAll=false stops at the first match", func(t *testing.T) {
        n := &Node{
            Property: typedvalue.PropertyRef{Type: typedvalue.TypeMsg, Path: "payload"},
            CheckAll: false,
            Rules: []Rule{
                {Operator: "gt", Value: typedvalue.Value{Type: typedvalue.TypeNumber, Value: "0"}},
                {Operator: "lt", Value: typedvalue.Value{Type: typedvalue.TypeNumber, Value: "10"}},
            },
        }

        outputs, err := n.ExecuteMulti(context.Background(), map[string]interface{}{"payload": float64(5)})
        require.NoError(t, err)
        assert.Len(t, outputs, 1)
        assert.Contains(t, outputs, "0")
    })

    t.Run("no matching rule sends on no port", func(t *testing.T) {
        n := &Node{Property: typedvalue.PropertyRef{Type: typedvalue.TypeMsg, Path: "payload"}, Rules: []Rule{numRule("eq", "1")}}
        outputs, err := n.ExecuteMulti(context.Background(), map[string]interface{}{"payload": float64(2)})
        require.NoError(t, err)
        assert.Empty(t, outputs)
    })

    t.Run("mutating one output's payload does not affect another", func(t *testing.T) {
        n := &Node{
            Property: typedvalue.PropertyRef{Type: typedvalue.TypeMsg, Path: "payload"},
            CheckAll: true,
            Rules: []Rule{
                {Operator: "nnull"},
                {Operator: "nnull"},
            },
        }
        outputs, err := n.ExecuteMulti(context.Background(), map[string]interface{}{"payload": "x"})
        require.NoError(t, err)
        outputs["0"]["payload"] = "mutated"
        assert.Equal(t, "x", outputs["1"]["payload"])
    })
}

func TestEvaluateRule_Operators(t *testing.T) {
    resolver := typedvalue.Resolver{}

    tests := []struct {
        name     string
        rule     Rule
        testVal  interface{}
        exists   bool
        expected bool
    }{
        {"eq numeric match", numRule("eq", "5"), float64(5), true, true},
        {"eq numeric non-match", numRule("eq", "5"), float64(6), true, false},
        {"eq string coercion", Rule{Operator: "eq", Value: typedvalue.Value{Type: typedvalue.TypeString, Value: "5"}}, float64(5), true, true},
        {"neq", numRule("neq", "5"), float64(6), true, true},
        {"lt true", numRule("lt", "5"), float64(3), true, true},
        {"lt false", numRule("lt", "5"), float64(9), true, false},
        {"lte equal", numRule("lte", "5"), float64(5), true, true},
        {"gt true", numRule("gt", "5"), float64(9), true, true},
        {"gte equal", numRule("gte", "5"), float64(5), true, true},
        {"cont", Rule{Operator: "cont", Value: typedvalue.Value{Type: typedvalue.TypeString, Value: "ell"}}, "hello", true, true},
        {"regex", Rule{Operator: "regex", Value: typedvalue.Value{Type: typedvalue.TypeString, Value: "^h.*o$"}}, "hello", true, true},
        {"regex no match", Rule{Operator: "regex", Value: typedvalue.Value{Type: typedvalue.TypeString, Value: "^x"}}, "hello", true, false},
        {"true op on true", Rule{Operator: "true"}, true, true, true},
        {"true op on false", Rule{Operator: "true"}, false, true, false},
        {"false op on false", Rule{Operator: "false"}, false, true, true},
        {"null op on nil", Rule{Operator: "null"}, nil, true, true},
        {"null op on non-nil", Rule{Operator: "null"}, "x", true, false},
        {"nnull op on non-nil", Rule{Operator: "nnull"}, "x", true, true},
        {"empty string", Rule{Operator: "empty"}, "", true, true},
        {"empty non-empty string", Rule{Operator: "empty"}, "x", true, false},
        {"nempty non-empty string", Rule{Operator: "nempty"}, "x", true, true},
        {"else always matches", Rule{Operator: "else"}, "anything", true, true},
        {"missing property never matches a comparison", numRule("eq", "5"), nil, false, false},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            matched, err := evaluateRule(tt.rule, tt.testVal, tt.exists, resolver)
            require.NoError(t, err)
            assert.Equal(t, tt.expected, matched)
        })
    }

    t.Run("btwn", func(t *testing.T) {
        rule := Rule{
            Operator: "btwn",
            Value:    typedvalue.Value{Type: typedvalue.TypeNumber, Value: "5"},
            Value2:   typedvalue.Value{Type: typedvalue.TypeNumber, Value: "10"},
        }
        matched, err := evaluateRule(rule, float64(7), true, resolver)
        require.NoError(t, err)
        assert.True(t, matched)

        matched, err = evaluateRule(rule, float64(20), true, resolver)
        require.NoError(t, err)
        assert.False(t, matched)
    })

    t.Run("unknown operator errors", func(t *testing.T) {
        _, err := evaluateRule(Rule{Operator: "bogus"}, "x", true, resolver)
        assert.Error(t, err)
    })

    t.Run("invalid regex errors", func(t *testing.T) {
        _, err := evaluateRule(Rule{Operator: "regex", Value: typedvalue.Value{Type: typedvalue.TypeString, Value: "("}}, "x", true, resolver)
        assert.Error(t, err)
    })
}

func TestNode_ExecuteMulti_FlowContext(t *testing.T) {
    t.Run("resolves a rule value from flow context", func(t *testing.T) {
        flowCtx := registry.NewContextStore()
        flowCtx.Set("threshold", float64(10))
        rt := registry.NewNodeRuntime("f1", "n1", "switch", flowCtx, nil, nil, nil, nil)
        ctx := registry.WithRuntime(context.Background(), rt)

        n := &Node{
            Property: typedvalue.PropertyRef{Type: typedvalue.TypeMsg, Path: "payload"},
            Rules:    []Rule{{Operator: "gt", Value: typedvalue.Value{Type: typedvalue.TypeFlow, Value: "threshold"}}},
        }

        outputs, err := n.ExecuteMulti(ctx, map[string]interface{}{"payload": float64(15)})
        require.NoError(t, err)
        assert.Contains(t, outputs, "0")
    })
}

func TestNode_Execute_DelegatesToExecuteMulti(t *testing.T) {
    n := &Node{Property: typedvalue.PropertyRef{Type: typedvalue.TypeMsg, Path: "payload"}, Rules: []Rule{numRule("eq", "1")}}
    output, err := n.Execute(context.Background(), map[string]interface{}{"payload": float64(1)})
    require.NoError(t, err)
    assert.Equal(t, float64(1), output["payload"])
}
