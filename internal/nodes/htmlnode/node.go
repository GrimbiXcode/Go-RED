// Package htmlnode provides the HTML node implementation (Node-RED type ID
// "html" - named htmlnode here to avoid shadowing the imported
// golang.org/x/net/html package within this file).
//
// HTML extracts content from an HTML string in msg.payload using a CSS
// selector and writes the result(s) to msg.payload.
//
// Scope cut vs. Node-RED (which uses cheerio, a full CSS selector engine -
// see docs/NODE_PALETTE_PLAN.md): only a minimal selector subset is
// supported, parsed and matched by hand rather than via a new dependency:
//   - simple selectors: an optional tag name, an optional #id, and any
//     number of .class selectors, e.g. "div.card#main"
//   - the descendant combinator (whitespace), e.g. "div.card p"
//
// Not supported: child (>), sibling (+, ~) combinators, attribute
// selectors ([href]), pseudo-classes (:nth-child, :first-child, ...), or
// comma-separated selector lists.
package htmlnode

import (
    "fmt"
    "strings"

    "github.com/GrimbiXcode/Go-RED/internal/registry"
    "golang.org/x/net/html"
)

// Node holds an HTML node's configuration.
type Node struct {
    // Selector is the (subset) CSS selector to match.
    Selector string
    // Ret is "html" (outer markup, the default), "text" (concatenated text
    // content), or "attr" (the value of Attr).
    Ret string
    // Attr is the attribute name to extract when Ret is "attr".
    Attr string
}

// Execute parses input["payload"] as HTML and extracts every element
// matching Selector. A single match is written as a bare value; multiple
// matches (or zero) are written as an array.
func (n *Node) Execute(ctx interface{}, input map[string]interface{}) (map[string]interface{}, error) {
    payload, ok := input["payload"].(string)
    if !ok {
        return nil, fmt.Errorf("html: payload must be a string, got %T", input["payload"])
    }

    doc, err := html.Parse(strings.NewReader(payload))
    if err != nil {
        return nil, fmt.Errorf("html: %w", err)
    }

    matches := selectAll(doc, parseSelector(n.Selector))

    results := make([]interface{}, len(matches))
    for i, m := range matches {
        switch n.Ret {
        case "text":
            results[i] = textContent(m)
        case "attr":
            results[i] = getAttr(m, n.Attr)
        default:
            results[i] = renderHTML(m)
        }
    }

    output := cloneMap(input)
    if len(results) == 1 {
        output["payload"] = results[0]
    } else {
        output["payload"] = results
    }
    return output, nil
}

// simpleSelector is one whitespace-separated part of Selector: an optional
// tag name plus any number of #id/.class requirements.
type simpleSelector struct {
    Tag     string
    ID      string
    Classes []string
}

func parseSelector(sel string) []simpleSelector {
    fields := strings.Fields(sel)
    result := make([]simpleSelector, len(fields))
    for i, f := range fields {
        result[i] = parseSimpleSelector(f)
    }
    return result
}

func parseSimpleSelector(s string) simpleSelector {
    var ss simpleSelector
    i := 0
    for i < len(s) && s[i] != '.' && s[i] != '#' {
        i++
    }
    ss.Tag = s[:i]
    for i < len(s) {
        j := i + 1
        for j < len(s) && s[j] != '.' && s[j] != '#' {
            j++
        }
        switch s[i] {
        case '.':
            ss.Classes = append(ss.Classes, s[i+1:j])
        case '#':
            ss.ID = s[i+1 : j]
        }
        i = j
    }
    return ss
}

func matchesSimple(n *html.Node, ss simpleSelector) bool {
    if n.Type != html.ElementNode {
        return false
    }
    if ss.Tag != "" && n.Data != ss.Tag {
        return false
    }
    if ss.ID != "" && getAttr(n, "id") != ss.ID {
        return false
    }
    for _, c := range ss.Classes {
        if !hasClass(n, c) {
            return false
        }
    }
    return true
}

// selectAll finds every node matching the last simpleSelector in the chain
// whose ancestors also satisfy the earlier ones, in order (a descendant
// combinator match - not a strict immediate-parent match).
func selectAll(root *html.Node, selectors []simpleSelector) []*html.Node {
    if len(selectors) == 0 {
        return nil
    }
    last := selectors[len(selectors)-1]

    var candidates []*html.Node
    var walk func(*html.Node)
    walk = func(n *html.Node) {
        if matchesSimple(n, last) {
            candidates = append(candidates, n)
        }
        for c := n.FirstChild; c != nil; c = c.NextSibling {
            walk(c)
        }
    }
    walk(root)

    if len(selectors) == 1 {
        return candidates
    }

    ancestorSelectors := selectors[:len(selectors)-1]
    result := candidates[:0]
    for _, cand := range candidates {
        if ancestorsMatch(cand, ancestorSelectors) {
            result = append(result, cand)
        }
    }
    return result
}

// ancestorsMatch walks up from n's parent, consuming selectors from the end
// as each is matched by some ancestor (in order, but not necessarily
// consecutively) - true only if every selector was consumed.
func ancestorsMatch(n *html.Node, selectors []simpleSelector) bool {
    idx := len(selectors) - 1
    for p := n.Parent; p != nil && idx >= 0; p = p.Parent {
        if matchesSimple(p, selectors[idx]) {
            idx--
        }
    }
    return idx < 0
}

func hasClass(n *html.Node, class string) bool {
    for _, c := range strings.Fields(getAttr(n, "class")) {
        if c == class {
            return true
        }
    }
    return false
}

func getAttr(n *html.Node, name string) string {
    for _, a := range n.Attr {
        if a.Key == name {
            return a.Val
        }
    }
    return ""
}

func textContent(n *html.Node) string {
    var b strings.Builder
    var walk func(*html.Node)
    walk = func(n *html.Node) {
        if n.Type == html.TextNode {
            b.WriteString(n.Data)
        }
        for c := n.FirstChild; c != nil; c = c.NextSibling {
            walk(c)
        }
    }
    walk(n)
    return b.String()
}

func renderHTML(n *html.Node) string {
    var b strings.Builder
    if err := html.Render(&b, n); err != nil {
        return ""
    }
    return b.String()
}

func (n *Node) Validate() error {
    switch n.Ret {
    case "", "html", "text", "attr":
        return nil
    default:
        return fmt.Errorf("html: unsupported ret %q", n.Ret)
    }
}

func (n *Node) GetConfig() map[string]interface{} {
    return map[string]interface{}{
        "selector": n.Selector,
        "ret":      n.Ret,
        "attr":     n.Attr,
    }
}

func (n *Node) SetConfig(config map[string]interface{}) error {
    if s, ok := config["selector"].(string); ok {
        n.Selector = s
    }
    n.Ret = "html"
    if r, ok := config["ret"].(string); ok && r != "" {
        n.Ret = r
    }
    if a, ok := config["attr"].(string); ok {
        n.Attr = a
    }
    return n.Validate()
}

func cloneMap(src map[string]interface{}) map[string]interface{} {
    dst := make(map[string]interface{}, len(src))
    for k, v := range src {
        dst[k] = v
    }
    return dst
}

func init() {
    reg := registry.GetGlobalRegistry()
    err := reg.RegisterFactory("html", func() registry.NodeExecutor {
        return &Node{Ret: "html"}
    }, registry.NodeMetadata{
        ID:          "html",
        Type:        "html",
        Name:        "HTML",
        Description: "Extracts content from an HTML string using a CSS selector (tag/.class/#id and descendant combinator only - see package docs)",
        Category:    "parser",
        Inputs: []registry.Port{
            {ID: "input", Name: "Input", Description: "Message with an HTML string payload", Required: true},
        },
        Outputs: []registry.Port{
            {ID: "output", Name: "Output", Description: "Message with the extracted content", Required: true},
        },
        ConfigSchema: registry.Schema{
            Properties: map[string]registry.Property{
                "selector": {Type: "string", Description: "CSS selector subset: tag, .class, #id, descendant combinator", Default: ""},
                "ret":      {Type: "string", Description: "html (outer markup), text (text content), or attr (an attribute value)", Default: "html", Enum: []string{"html", "text", "attr"}},
                "attr":     {Type: "string", Description: "Attribute name to extract when ret is \"attr\"", Default: ""},
            },
        },
        Icon: `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="#00ADD8"><path d="M3 2l1.6 18L12 22l7.4-2L21 2H3zm14.8 6.3H8.6l.2 2.2h8.8l-.6 6.8-4.9 1.4-5-1.4-.3-3.7h2.2l.2 1.9 2.9.8 2.9-.8.3-3.1H6.7l-.6-6.3h11.9z"/></svg>`,
        Tags: []string{"parser", "html", "selector"},
    })
    if err != nil {
        panic(err)
    }
}
