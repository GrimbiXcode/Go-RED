// Package csvnode provides the CSV node implementation (Node-RED type ID
// "csv" - named csvnode here to avoid shadowing the imported
// encoding/csv package within this file).
//
// CSV converts msg.payload bidirectionally based on its current type: a
// string is parsed into an array of rows, an array is serialized into a
// CSV string.
package csvnode

import (
    "encoding/csv"
    "fmt"
    "sort"
    "strings"

    "github.com/GrimbiXcode/Go-RED/internal/registry"
)

// Node holds a CSV node's configuration.
type Node struct {
    // HasHeaderRow, when parsing, treats the first row as field names and
    // produces an array of objects instead of an array of arrays; when
    // serializing, writes Columns (or an object's keys) as a header row.
    HasHeaderRow bool
    // Columns, if set, is used as parse headers (instead of the first row)
    // or as the explicit column order when serializing. If unset when
    // serializing objects, keys are taken from the first row and sorted for
    // determinism (map iteration order is not stable).
    Columns []string
    // Delimiter is a single character, default ",".
    Delimiter string
}

// Execute parses input["payload"] if it's a string, or serializes it to a
// CSV string if it's an array.
func (n *Node) Execute(ctx interface{}, input map[string]interface{}) (map[string]interface{}, error) {
    output := cloneMap(input)

    switch payload := input["payload"].(type) {
    case string:
        parsed, err := n.parse(payload)
        if err != nil {
            return nil, fmt.Errorf("csv: %w", err)
        }
        output["payload"] = parsed
    case []interface{}:
        serialized, err := n.serialize(payload)
        if err != nil {
            return nil, fmt.Errorf("csv: %w", err)
        }
        output["payload"] = serialized
    default:
        return nil, fmt.Errorf("csv: payload must be a string (to parse) or an array (to serialize), got %T", payload)
    }
    return output, nil
}

func (n *Node) reader(text string) *csv.Reader {
    r := csv.NewReader(strings.NewReader(text))
    if n.Delimiter != "" {
        r.Comma = rune(n.Delimiter[0])
    }
    return r
}

func (n *Node) parse(text string) (interface{}, error) {
    rows, err := n.reader(text).ReadAll()
    if err != nil {
        return nil, err
    }
    if len(rows) == 0 {
        return []interface{}{}, nil
    }

    headers := n.Columns
    dataRows := rows
    if n.HasHeaderRow {
        headers = rows[0]
        dataRows = rows[1:]
    }

    result := make([]interface{}, len(dataRows))
    if len(headers) == 0 {
        for i, row := range dataRows {
            arr := make([]interface{}, len(row))
            for j, cell := range row {
                arr[j] = cell
            }
            result[i] = arr
        }
        return result, nil
    }

    for i, row := range dataRows {
        obj := make(map[string]interface{}, len(headers))
        for j, h := range headers {
            if j < len(row) {
                obj[h] = row[j]
            } else {
                obj[h] = ""
            }
        }
        result[i] = obj
    }
    return result, nil
}

func (n *Node) serialize(rows []interface{}) (string, error) {
    var buf strings.Builder
    w := csv.NewWriter(&buf)
    if n.Delimiter != "" {
        w.Comma = rune(n.Delimiter[0])
    }

    headers := n.Columns
    if len(headers) == 0 && len(rows) > 0 {
        if obj, ok := rows[0].(map[string]interface{}); ok {
            for k := range obj {
                headers = append(headers, k)
            }
            sort.Strings(headers)
        }
    }

    if n.HasHeaderRow && len(headers) > 0 {
        if err := w.Write(headers); err != nil {
            return "", err
        }
    }

    for i, item := range rows {
        var record []string
        switch v := item.(type) {
        case map[string]interface{}:
            record = make([]string, len(headers))
            for j, h := range headers {
                record[j] = fmt.Sprintf("%v", v[h])
            }
        case []interface{}:
            record = make([]string, len(v))
            for j, cell := range v {
                record[j] = fmt.Sprintf("%v", cell)
            }
        default:
            return "", fmt.Errorf("row %d: unsupported type %T, expected an object or array", i, item)
        }
        if err := w.Write(record); err != nil {
            return "", err
        }
    }

    w.Flush()
    if err := w.Error(); err != nil {
        return "", err
    }
    return buf.String(), nil
}

func (n *Node) Validate() error { return nil }

func (n *Node) GetConfig() map[string]interface{} {
    columns := make([]interface{}, len(n.Columns))
    for i, c := range n.Columns {
        columns[i] = c
    }
    return map[string]interface{}{
        "hasHeaderRow": n.HasHeaderRow,
        "columns":      columns,
        "delimiter":    n.Delimiter,
    }
}

func (n *Node) SetConfig(config map[string]interface{}) error {
    if h, ok := config["hasHeaderRow"].(bool); ok {
        n.HasHeaderRow = h
    }
    n.Columns = nil
    if raw, ok := config["columns"].([]interface{}); ok {
        for _, v := range raw {
            if s, ok := v.(string); ok {
                n.Columns = append(n.Columns, s)
            }
        }
    }
    if d, ok := config["delimiter"].(string); ok {
        n.Delimiter = d
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
    err := reg.RegisterFactory("csv", func() registry.NodeExecutor {
        return &Node{}
    }, registry.NodeMetadata{
        ID:          "csv",
        Type:        "csv",
        Name:        "CSV",
        Description: "Converts msg.payload between a CSV string and an array of rows, based on its current type",
        Category:    "parser",
        Inputs: []registry.Port{
            {ID: "input", Name: "Input", Description: "Message with a CSV string or an array payload", Required: true},
        },
        Outputs: []registry.Port{
            {ID: "output", Name: "Output", Description: "Message with the converted payload", Required: true},
        },
        ConfigSchema: registry.Schema{
            Properties: map[string]registry.Property{
                "hasHeaderRow": {Type: "boolean", Description: "First row is field names (parse) / write a header row (serialize)", Default: false},
                "columns":      {Type: "array", Description: "Explicit column names/order; overrides the first row when parsing", Default: []interface{}{}},
                "delimiter":    {Type: "string", Description: "Field delimiter character", Default: ","},
            },
        },
        Icon: `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="#00ADD8"><path d="M3 3h18v2H3zm0 8h18v2H3zm0 8h18v2H3zM3 3v18h2V3zm7 0v18h2V3zm7 0v18h2V3z"/></svg>`,
        Tags: []string{"parser", "csv"},
    })
    if err != nil {
        panic(err)
    }
}
