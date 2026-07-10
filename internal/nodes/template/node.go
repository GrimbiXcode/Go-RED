// Package template provides the Template node implementation.
//
// Template renders Template against the message and writes the result to
// Field (default "payload"), optionally parsing the rendered text as JSON.
//
// Scope cut vs. Node-RED (see docs/NODE_PALETTE_PLAN.md): only variable
// substitution is supported - "{{payload}}", "{{payload.count}}" - reading
// from the message. There is no support (yet) for Mustache sections
// ({{#each}}), partials, or flow/global/env lookups within a template; use
// Syntax "plain" for literal text with no substitution at all.
package template

import (
    "encoding/json"
    "fmt"
    "regexp"
    "strconv"
    "strings"

    "github.com/GrimbiXcode/Go-RED/internal/registry"
    "github.com/GrimbiXcode/Go-RED/internal/typedvalue"
)

var tagPattern = regexp.MustCompile(`\{\{\s*([a-zA-Z0-9_.]+)\s*\}\}`)

// Node holds a Template node's configuration.
type Node struct {
    // Field is the dot-path message property the rendered result is
    // written to.
    Field string
    // Template is the template text.
    Template string
    // Syntax is "mustache" (render {{...}} placeholders, the default) or
    // "plain" (use Template verbatim, no substitution).
    Syntax string
    // OutputFormat is "str" (the default) or "json" (parse the rendered
    // text as JSON before writing it to Field).
    OutputFormat string
}

// Execute renders Template and writes the result to Field in a copy of
// input.
func (n *Node) Execute(ctx interface{}, input map[string]interface{}) (map[string]interface{}, error) {
    output := cloneMap(input)

    rendered := n.Template
    if n.Syntax != "plain" {
        rendered = render(n.Template, input)
    }

    var result interface{} = rendered
    if n.OutputFormat == "json" {
        var parsed interface{}
        if err := json.Unmarshal([]byte(rendered), &parsed); err != nil {
            return nil, fmt.Errorf("template: rendered output is not valid JSON: %w", err)
        }
        result = parsed
    }

    target := typedvalue.PropertyRef{Type: typedvalue.TypeMsg, Path: n.Field}
    if err := target.Set(typedvalue.PropertyResolver{Message: output}, result); err != nil {
        return nil, fmt.Errorf("template: %w", err)
    }
    return output, nil
}

// render replaces every {{path}} in tmpl with the string form of msg's
// value at that dot-path (empty string if missing).
func render(tmpl string, msg map[string]interface{}) string {
    return tagPattern.ReplaceAllStringFunc(tmpl, func(match string) string {
        path := strings.TrimSpace(tagPattern.FindStringSubmatch(match)[1])
        val, _ := (typedvalue.Value{Type: typedvalue.TypeMsg, Value: path}).Resolve(typedvalue.Resolver{Message: msg})
        return toStr(val)
    })
}

func toStr(v interface{}) string {
    switch t := v.(type) {
    case nil:
        return ""
    case string:
        return t
    case float64:
        return strconv.FormatFloat(t, 'f', -1, 64)
    case bool:
        return strconv.FormatBool(t)
    default:
        b, err := json.Marshal(t)
        if err != nil {
            return fmt.Sprintf("%v", t)
        }
        return string(b)
    }
}

func (n *Node) Validate() error {
    switch n.Syntax {
    case "", "mustache", "plain":
    default:
        return fmt.Errorf("template: unsupported syntax %q", n.Syntax)
    }
    switch n.OutputFormat {
    case "", "str", "json":
    default:
        return fmt.Errorf("template: unsupported output format %q", n.OutputFormat)
    }
    return nil
}

func (n *Node) GetConfig() map[string]interface{} {
    return map[string]interface{}{
        "field":    n.Field,
        "template": n.Template,
        "syntax":   n.Syntax,
        "output":   n.OutputFormat,
    }
}

func (n *Node) SetConfig(config map[string]interface{}) error {
    n.Field = "payload"
    if f, ok := config["field"].(string); ok && f != "" {
        n.Field = f
    }
    if t, ok := config["template"].(string); ok {
        n.Template = t
    }
    n.Syntax = "mustache"
    if s, ok := config["syntax"].(string); ok && s != "" {
        n.Syntax = s
    }
    n.OutputFormat = "str"
    if o, ok := config["output"].(string); ok && o != "" {
        n.OutputFormat = o
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
    err := reg.RegisterFactory("template", func() registry.NodeExecutor {
        return &Node{Field: "payload", Syntax: "mustache", OutputFormat: "str"}
    }, registry.NodeMetadata{
        ID:          "template",
        Type:        "template",
        Name:        "Template",
        Description: "Renders {{msg.property}} placeholders in a template and writes the result to a message property",
        Category:    "function",
        Inputs: []registry.Port{
            {ID: "input", Name: "Input", Description: "Message to render the template against", Required: true},
        },
        Outputs: []registry.Port{
            {ID: "output", Name: "Output", Description: "Message with the rendered result", Required: true},
        },
        ConfigSchema: registry.Schema{
            Properties: map[string]registry.Property{
                "field":    {Type: "string", Description: "Message property to write the result to", Default: "payload"},
                "template": {Type: "string", Description: "Template text, e.g. \"hello {{payload.name}}\"", Default: ""},
                "syntax":   {Type: "string", Description: "mustache (render placeholders) or plain (literal text)", Default: "mustache", Enum: []string{"mustache", "plain"}},
                "output":   {Type: "string", Description: "str (leave as text) or json (parse the rendered text as JSON)", Default: "str", Enum: []string{"str", "json"}},
            },
        },
        Icon: `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="#00ADD8"><path d="M14 2H6a2 2 0 00-2 2v16a2 2 0 002 2h12a2 2 0 002-2V8l-6-6zm2 16H8v-2h8v2zm0-4H8v-2h8v2zm-3-5V3.5L18.5 9H13z"/></svg>`,
        Tags: []string{"function", "template", "mustache"},
    })
    if err != nil {
        panic(err)
    }
}
