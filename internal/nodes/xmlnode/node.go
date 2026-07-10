// Package xmlnode provides the XML node implementation (Node-RED type ID
// "xml" - named xmlnode here to avoid shadowing the imported
// encoding/xml package within this file).
//
// XML converts msg.payload bidirectionally based on its current type: a
// string is parsed into an object, an object is serialized into an XML
// string.
//
// This uses its own XML<->object convention (documented below), not
// Node-RED's exact xml2js-derived schema:
//
//   - The result is always a single-key object: the root element's tag
//     name maps to its content.
//   - Attributes become keys prefixed with "@" (e.g. "@id").
//   - Text content becomes a "#text" key if the element also has
//     attributes or child elements, or is the value directly for a simple
//     leaf element with neither.
//   - Repeated child elements with the same tag name become an array (in
//     document order); a single occurrence stays a bare value.
//
// Known limitation: sibling order *between different tag names* is not
// preserved on a round trip (the object is keyed by tag name, not an
// ordered list of children) - this is the same tradeoff most XML<->JSON
// converters make. Order *within* repeated occurrences of the same tag name
// is preserved. XML namespaces (beyond the local name), comments,
// processing instructions, and CDATA-vs-text distinction are not
// preserved.
package xmlnode

import (
    "encoding/xml"
    "fmt"
    "sort"
    "strings"

    "github.com/GrimbiXcode/Go-RED/internal/registry"
)

// Node holds an XML node's configuration. Nothing to configure yet.
type Node struct{}

// Execute parses input["payload"] if it's a string, or serializes it to an
// XML string if it's an object.
func (n *Node) Execute(ctx interface{}, input map[string]interface{}) (map[string]interface{}, error) {
    output := cloneMap(input)

    switch payload := input["payload"].(type) {
    case string:
        parsed, err := parse(payload)
        if err != nil {
            return nil, fmt.Errorf("xml: %w", err)
        }
        output["payload"] = parsed
    case map[string]interface{}:
        serialized, err := serialize(payload)
        if err != nil {
            return nil, fmt.Errorf("xml: %w", err)
        }
        output["payload"] = serialized
    default:
        return nil, fmt.Errorf("xml: payload must be a string (to parse) or an object (to serialize), got %T", payload)
    }
    return output, nil
}

// parse decodes an XML document into a single-key map: {rootTagName: content}.
func parse(text string) (map[string]interface{}, error) {
    d := xml.NewDecoder(strings.NewReader(text))
    for {
        tok, err := d.Token()
        if err != nil {
            return nil, err
        }
        if start, ok := tok.(xml.StartElement); ok {
            val, err := parseElement(d, start)
            if err != nil {
                return nil, err
            }
            return map[string]interface{}{start.Name.Local: val}, nil
        }
    }
}

// parseElement reads tokens until start's matching EndElement, returning
// either a bare string (a leaf with no attributes/children) or a map per
// the package doc's convention.
func parseElement(d *xml.Decoder, start xml.StartElement) (interface{}, error) {
    attrs := make(map[string]interface{}, len(start.Attr))
    for _, a := range start.Attr {
        attrs["@"+a.Name.Local] = a.Value
    }
    children := map[string]interface{}{}
    var text strings.Builder

    for {
        tok, err := d.Token()
        if err != nil {
            return nil, err
        }
        switch t := tok.(type) {
        case xml.StartElement:
            childVal, err := parseElement(d, t)
            if err != nil {
                return nil, err
            }
            appendChild(children, t.Name.Local, childVal)
        case xml.CharData:
            text.Write(t)
        case xml.EndElement:
            trimmed := strings.TrimSpace(text.String())
            if len(attrs) == 0 && len(children) == 0 {
                return trimmed, nil
            }
            result := make(map[string]interface{}, len(attrs)+len(children)+1)
            for k, v := range attrs {
                result[k] = v
            }
            for k, v := range children {
                result[k] = v
            }
            if trimmed != "" {
                result["#text"] = trimmed
            }
            return result, nil
        }
    }
}

// appendChild adds val under key, turning a repeated key into an
// []interface{} in the order encountered.
func appendChild(m map[string]interface{}, key string, val interface{}) {
    if existing, ok := m[key]; ok {
        if arr, ok := existing.([]interface{}); ok {
            m[key] = append(arr, val)
        } else {
            m[key] = []interface{}{existing, val}
        }
        return
    }
    m[key] = val
}

// serialize encodes a single-key {rootTagName: content} map back into XML.
func serialize(payload map[string]interface{}) (string, error) {
    if len(payload) != 1 {
        return "", fmt.Errorf("payload must have exactly one root key, got %d", len(payload))
    }
    var buf strings.Builder
    for root, val := range payload {
        if err := writeElement(&buf, root, val); err != nil {
            return "", err
        }
    }
    return buf.String(), nil
}

func writeElement(buf *strings.Builder, tag string, val interface{}) error {
    m, ok := val.(map[string]interface{})
    if !ok {
        // A bare leaf value (string, number, bool, nil, ...).
        buf.WriteString("<" + tag + ">")
        if val != nil {
            if err := xml.EscapeText(buf, []byte(fmt.Sprintf("%v", val))); err != nil {
                return err
            }
        }
        buf.WriteString("</" + tag + ">")
        return nil
    }

    var attrKeys, childKeys []string
    var text string
    for k := range m {
        switch {
        case strings.HasPrefix(k, "@"):
            attrKeys = append(attrKeys, k)
        case k == "#text":
            text = fmt.Sprintf("%v", m[k])
        default:
            childKeys = append(childKeys, k)
        }
    }
    sort.Strings(attrKeys)
    sort.Strings(childKeys) // see the package doc: cross-tag sibling order isn't preserved

    buf.WriteString("<" + tag)
    for _, k := range attrKeys {
        buf.WriteString(" " + strings.TrimPrefix(k, "@") + `="`)
        if err := xml.EscapeText(buf, []byte(fmt.Sprintf("%v", m[k]))); err != nil {
            return err
        }
        buf.WriteString(`"`)
    }
    buf.WriteString(">")

    if text != "" {
        if err := xml.EscapeText(buf, []byte(text)); err != nil {
            return err
        }
    }
    for _, k := range childKeys {
        child := m[k]
        if arr, ok := child.([]interface{}); ok {
            for _, item := range arr {
                if err := writeElement(buf, k, item); err != nil {
                    return err
                }
            }
            continue
        }
        if err := writeElement(buf, k, child); err != nil {
            return err
        }
    }
    buf.WriteString("</" + tag + ">")
    return nil
}

func (n *Node) Validate() error { return nil }

func (n *Node) GetConfig() map[string]interface{} { return map[string]interface{}{} }

func (n *Node) SetConfig(config map[string]interface{}) error { return nil }

func cloneMap(src map[string]interface{}) map[string]interface{} {
    dst := make(map[string]interface{}, len(src))
    for k, v := range src {
        dst[k] = v
    }
    return dst
}

func init() {
    reg := registry.GetGlobalRegistry()
    err := reg.RegisterFactory("xml", func() registry.NodeExecutor {
        return &Node{}
    }, registry.NodeMetadata{
        ID:          "xml",
        Type:        "xml",
        Name:        "XML",
        Description: "Converts msg.payload between an XML string and an object, based on its current type (see package docs for the object schema)",
        Category:    "parser",
        Inputs: []registry.Port{
            {ID: "input", Name: "Input", Description: "Message with an XML string or object payload", Required: true},
        },
        Outputs: []registry.Port{
            {ID: "output", Name: "Output", Description: "Message with the converted payload", Required: true},
        },
        ConfigSchema: registry.Schema{},
        Icon:         `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="#00ADD8"><path d="M8 3L2 12l6 9h2l-6-9 6-9zm8 0l6 9-6 9h-2l6-9-6-9z"/></svg>`,
        Tags:         []string{"parser", "xml"},
    })
    if err != nil {
        panic(err)
    }
}
