package htmlnode

import (
    "context"
    "testing"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

const sampleHTML = `
<html><body>
  <div class="card" id="first"><p>Hello</p></div>
  <div class="card" id="second"><p>World</p></div>
  <a href="https://example.com">link</a>
</body></html>`

func TestNode_ConfigRoundTrip(t *testing.T) {
    n := &Node{}
    require.NoError(t, n.SetConfig(map[string]interface{}{"selector": "div.card", "ret": "text", "attr": "href"}))
    assert.Equal(t, "div.card", n.Selector)
    assert.Equal(t, "text", n.Ret)
    assert.Equal(t, "href", n.Attr)
}

func TestNode_SetConfig_Defaults(t *testing.T) {
    n := &Node{}
    require.NoError(t, n.SetConfig(map[string]interface{}{}))
    assert.Equal(t, "html", n.Ret)
}

func TestNode_Validate(t *testing.T) {
    assert.Error(t, (&Node{Ret: "bogus"}).Validate())
    assert.NoError(t, (&Node{Ret: "text"}).Validate())
}

func TestNode_Execute_TagSelector(t *testing.T) {
    n := &Node{Selector: "p", Ret: "text"}
    output, err := n.Execute(context.Background(), map[string]interface{}{"payload": sampleHTML})
    require.NoError(t, err)
    assert.Equal(t, []interface{}{"Hello", "World"}, output["payload"])
}

func TestNode_Execute_ClassSelector(t *testing.T) {
    n := &Node{Selector: ".card", Ret: "attr", Attr: "id"}
    output, err := n.Execute(context.Background(), map[string]interface{}{"payload": sampleHTML})
    require.NoError(t, err)
    assert.Equal(t, []interface{}{"first", "second"}, output["payload"])
}

func TestNode_Execute_IDSelector(t *testing.T) {
    n := &Node{Selector: "#second", Ret: "text"}
    output, err := n.Execute(context.Background(), map[string]interface{}{"payload": sampleHTML})
    require.NoError(t, err)
    assert.Equal(t, "World", output["payload"], "a single match is a bare value, not an array")
}

func TestNode_Execute_DescendantCombinator(t *testing.T) {
    n := &Node{Selector: "#second p", Ret: "text"}
    output, err := n.Execute(context.Background(), map[string]interface{}{"payload": sampleHTML})
    require.NoError(t, err)
    assert.Equal(t, "World", output["payload"])
}

func TestNode_Execute_DescendantCombinatorExcludesOtherBranch(t *testing.T) {
    n := &Node{Selector: "#first p", Ret: "text"}
    output, err := n.Execute(context.Background(), map[string]interface{}{"payload": sampleHTML})
    require.NoError(t, err)
    assert.Equal(t, "Hello", output["payload"])
}

func TestNode_Execute_AttrExtraction(t *testing.T) {
    n := &Node{Selector: "a", Ret: "attr", Attr: "href"}
    output, err := n.Execute(context.Background(), map[string]interface{}{"payload": sampleHTML})
    require.NoError(t, err)
    assert.Equal(t, "https://example.com", output["payload"])
}

func TestNode_Execute_HTMLExtraction(t *testing.T) {
    n := &Node{Selector: "#first", Ret: "html"}
    output, err := n.Execute(context.Background(), map[string]interface{}{"payload": sampleHTML})
    require.NoError(t, err)
    assert.Contains(t, output["payload"], "<p>Hello</p>")
    assert.Contains(t, output["payload"], `id="first"`)
}

func TestNode_Execute_NoMatchReturnsEmptyArray(t *testing.T) {
    n := &Node{Selector: ".does-not-exist", Ret: "text"}
    output, err := n.Execute(context.Background(), map[string]interface{}{"payload": sampleHTML})
    require.NoError(t, err)
    assert.Equal(t, []interface{}{}, output["payload"])
}

func TestNode_Execute_CombinedTagClassID(t *testing.T) {
    n := &Node{Selector: "div.card#first", Ret: "text"}
    output, err := n.Execute(context.Background(), map[string]interface{}{"payload": sampleHTML})
    require.NoError(t, err)
    assert.Equal(t, "Hello", output["payload"])
}

func TestNode_Execute_NonStringPayloadErrors(t *testing.T) {
    n := &Node{Selector: "p"}
    _, err := n.Execute(context.Background(), map[string]interface{}{"payload": float64(1)})
    assert.Error(t, err)
}

func TestParseSelector(t *testing.T) {
    sel := parseSelector("div.card.highlight#main p")
    require.Len(t, sel, 2)
    assert.Equal(t, "div", sel[0].Tag)
    assert.Equal(t, "main", sel[0].ID)
    assert.Equal(t, []string{"card", "highlight"}, sel[0].Classes)
    assert.Equal(t, "p", sel[1].Tag)
}
