package xmlnode

import (
    "context"
    "testing"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestNode_ConfigRoundTrip(t *testing.T) {
    n := &Node{}
    require.NoError(t, n.SetConfig(map[string]interface{}{}))
    assert.Equal(t, map[string]interface{}{}, n.GetConfig())
}

func TestNode_Execute_ParseSimpleLeaf(t *testing.T) {
    n := &Node{}
    output, err := n.Execute(context.Background(), map[string]interface{}{"payload": "<name>Ada</name>"})
    require.NoError(t, err)
    assert.Equal(t, map[string]interface{}{"name": "Ada"}, output["payload"])
}

func TestNode_Execute_ParseAttributes(t *testing.T) {
    n := &Node{}
    output, err := n.Execute(context.Background(), map[string]interface{}{"payload": `<person id="1" active="true"/>`})
    require.NoError(t, err)
    person := output["payload"].(map[string]interface{})["person"].(map[string]interface{})
    assert.Equal(t, "1", person["@id"])
    assert.Equal(t, "true", person["@active"])
}

func TestNode_Execute_ParseNestedElements(t *testing.T) {
    n := &Node{}
    output, err := n.Execute(context.Background(), map[string]interface{}{"payload": "<person><name>Ada</name><age>30</age></person>"})
    require.NoError(t, err)
    person := output["payload"].(map[string]interface{})["person"].(map[string]interface{})
    assert.Equal(t, "Ada", person["name"])
    assert.Equal(t, "30", person["age"])
}

func TestNode_Execute_ParseRepeatedElementsBecomeArray(t *testing.T) {
    n := &Node{}
    output, err := n.Execute(context.Background(), map[string]interface{}{"payload": "<items><item>a</item><item>b</item><item>c</item></items>"})
    require.NoError(t, err)
    items := output["payload"].(map[string]interface{})["items"].(map[string]interface{})
    assert.Equal(t, []interface{}{"a", "b", "c"}, items["item"])
}

func TestNode_Execute_ParseAttributesAndTextTogether(t *testing.T) {
    n := &Node{}
    output, err := n.Execute(context.Background(), map[string]interface{}{"payload": `<price currency="USD">9.99</price>`})
    require.NoError(t, err)
    price := output["payload"].(map[string]interface{})["price"].(map[string]interface{})
    assert.Equal(t, "USD", price["@currency"])
    assert.Equal(t, "9.99", price["#text"])
}

func TestNode_Execute_ParseInvalidXMLErrors(t *testing.T) {
    n := &Node{}
    _, err := n.Execute(context.Background(), map[string]interface{}{"payload": "<a><b></a>"})
    assert.Error(t, err)
}

func TestNode_Execute_Serialize(t *testing.T) {
    n := &Node{}
    output, err := n.Execute(context.Background(), map[string]interface{}{
        "payload": map[string]interface{}{
            "person": map[string]interface{}{
                "@id":  "1",
                "name": "Ada",
            },
        },
    })
    require.NoError(t, err)
    assert.Equal(t, `<person id="1"><name>Ada</name></person>`, output["payload"])
}

func TestNode_Execute_SerializeSimpleLeaf(t *testing.T) {
    n := &Node{}
    output, err := n.Execute(context.Background(), map[string]interface{}{
        "payload": map[string]interface{}{"name": "Ada"},
    })
    require.NoError(t, err)
    assert.Equal(t, "<name>Ada</name>", output["payload"])
}

func TestNode_Execute_SerializeRepeatedElements(t *testing.T) {
    n := &Node{}
    output, err := n.Execute(context.Background(), map[string]interface{}{
        "payload": map[string]interface{}{
            "items": map[string]interface{}{"item": []interface{}{"a", "b"}},
        },
    })
    require.NoError(t, err)
    assert.Equal(t, "<items><item>a</item><item>b</item></items>", output["payload"])
}

func TestNode_Execute_SerializeEscapesSpecialCharacters(t *testing.T) {
    n := &Node{}
    output, err := n.Execute(context.Background(), map[string]interface{}{
        "payload": map[string]interface{}{"note": `<script>alert("x")</script> & "quotes"`},
    })
    require.NoError(t, err)
    str := output["payload"].(string)
    assert.NotContains(t, str, "<script>")
    assert.Contains(t, str, "&lt;script&gt;")
}

func TestNode_Execute_SerializeMultiRootErrors(t *testing.T) {
    n := &Node{}
    _, err := n.Execute(context.Background(), map[string]interface{}{
        "payload": map[string]interface{}{"a": "1", "b": "2"},
    })
    assert.Error(t, err)
}

func TestNode_Execute_UnsupportedPayloadTypeErrors(t *testing.T) {
    n := &Node{}
    _, err := n.Execute(context.Background(), map[string]interface{}{"payload": float64(1)})
    assert.Error(t, err)
}

func TestNode_Execute_RoundTrip(t *testing.T) {
    // Only one distinct child tag name per level (plus attributes), which
    // survives a round trip exactly - see the package doc for the
    // documented limitation on cross-tag sibling order.
    n := &Node{}
    original := `<person id="1"><tag>a</tag><tag>b</tag></person>`

    parsed, err := n.Execute(context.Background(), map[string]interface{}{"payload": original})
    require.NoError(t, err)

    serialized, err := n.Execute(context.Background(), map[string]interface{}{"payload": parsed["payload"]})
    require.NoError(t, err)
    assert.Equal(t, original, serialized["payload"])
}

func TestNode_Execute_CrossTagSiblingOrderIsNotPreserved(t *testing.T) {
    // Documented limitation: children are keyed by tag name, so sibling
    // order *between different tag names* is lost (alphabetized on
    // serialize), even though this round-trips to equivalent *data*.
    n := &Node{}
    original := "<a><z>1</z><b>2</b></a>"

    parsed, err := n.Execute(context.Background(), map[string]interface{}{"payload": original})
    require.NoError(t, err)

    serialized, err := n.Execute(context.Background(), map[string]interface{}{"payload": parsed["payload"]})
    require.NoError(t, err)
    assert.Equal(t, "<a><b>2</b><z>1</z></a>", serialized["payload"], "b sorts before z")
    assert.NotEqual(t, original, serialized["payload"])
}
