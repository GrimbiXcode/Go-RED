// Package nodered converts between Node-RED's flows.json export (a flat
// array of nodes with "wires") and Go-RED flows, so flows built in
// Node-RED can be brought over and Go-RED flows can be handed back.
//
// Node-RED node types that Go-RED implements under the same name are
// mapped property by property (see convertConfig / exportConfig). Nodes of
// other types are kept with their type and raw properties, and reported
// as warnings: they show up on the canvas and fail at deploy with a
// readable message, which beats silently dropping them.
package nodered

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/GrimbiXcode/Go-RED/internal/engine"
)

// Raw is one element of a Node-RED export.
type Raw = map[string]interface{}

// ImportResult is what Import produced: one flow per Node-RED tab, plus
// warnings about what could not be carried over exactly.
type ImportResult struct {
	Flows    []*engine.Flow
	Warnings []string
}

// Node-RED nodes are drawn around their center; Go-RED positions are the
// top-left corner of a 140 x 36 card.
const (
	offsetX = 70
	offsetY = 15
)

// configTypes are Node-RED config nodes (no wires, shared by reference).
var configTypes = map[string]bool{
	"mqtt-broker": true, "tls-config": true, "websocket-listener": true, "websocket-client": true, "http proxy": true,
}

// referenceProperties name the config properties that hold a config node id.
var referenceProperties = []string{"broker", "server", "tls", "proxy"}

// IsExport reports whether data looks like a Node-RED export (a JSON array).
func IsExport(data []byte) bool {
	trimmed := strings.TrimSpace(string(data))
	return strings.HasPrefix(trimmed, "[")
}

type warnings struct {
	counts map[string]int
	order  []string
}

func (w *warnings) add(format string, args ...interface{}) {
	if w.counts == nil {
		w.counts = map[string]int{}
	}
	msg := fmt.Sprintf(format, args...)
	if _, seen := w.counts[msg]; !seen {
		w.order = append(w.order, msg)
	}
	w.counts[msg]++
}

func (w *warnings) list() []string {
	out := make([]string, 0, len(w.order))
	for _, msg := range w.order {
		if n := w.counts[msg]; n > 1 {
			out = append(out, fmt.Sprintf("%s (%d×)", msg, n))
		} else {
			out = append(out, msg)
		}
	}
	return out
}

// Import parses a Node-RED export. Every tab becomes a flow; nodes on a tab
// become its nodes, "wires" become connections, config nodes are copied
// into each flow that references them. Subflows and groups are skipped.
func Import(data []byte) (*ImportResult, error) {
	var raws []Raw
	if err := json.Unmarshal(data, &raws); err != nil {
		return nil, fmt.Errorf("not a Node-RED export: %w", err)
	}

	w := &warnings{}
	var tabs []Raw
	byTab := map[string][]Raw{}
	var tabOrder []string
	configs := map[string]Raw{}
	subflows := map[string]bool{}

	for _, raw := range raws {
		typ := str(raw["type"])
		switch {
		case typ == "tab":
			tabs = append(tabs, raw)
		case typ == "subflow":
			subflows[str(raw["id"])] = true
			w.add("subflow %q is not supported and was skipped", firstNonEmpty(str(raw["name"]), str(raw["id"])))
		case typ == "group":
			// Groups are a drawing aid; their members keep their positions.
		case strings.HasPrefix(typ, "subflow:"):
			w.add("subflow instances are not supported and were skipped")
		case typ == "":
			w.add("an element without a type was skipped")
		default:
			if _, hasWires := raw["wires"]; !hasWires || configTypes[typ] {
				configs[str(raw["id"])] = raw
				continue
			}
			z := str(raw["z"])
			if subflows[z] {
				continue
			}
			if _, seen := byTab[z]; !seen {
				tabOrder = append(tabOrder, z)
			}
			byTab[z] = append(byTab[z], raw)
		}
	}

	result := &ImportResult{}
	seenTabs := map[string]bool{}
	build := func(id, name, info string, disabled bool, nodes []Raw) {
		flow := engine.NewFlow(id, name)
		flow.Description = info
		refs := map[string]bool{}
		for _, raw := range nodes {
			node := convertNode(raw, w)
			if disabled {
				node.Disabled = true
			}
			flow.Nodes[node.ID] = node
			for _, prop := range referenceProperties {
				if ref, ok := node.Config[prop].(string); ok && ref != "" {
					refs[ref] = true
				}
			}
		}
		for _, raw := range nodes {
			flow.Connections = append(flow.Connections, convertWires(raw, flow, w)...)
		}
		for ref := range refs {
			cfg, ok := configs[ref]
			if !ok {
				w.add("config node %q is referenced but not part of the export", ref)
				continue
			}
			if _, exists := flow.Nodes[ref]; !exists {
				flow.Nodes[ref] = convertNode(cfg, w)
			}
		}
		result.Flows = append(result.Flows, flow)
	}

	for _, tab := range tabs {
		id := str(tab["id"])
		seenTabs[id] = true
		build(id, firstNonEmpty(str(tab["label"]), str(tab["name"]), "Imported flow"), str(tab["info"]), boolean(tab["disabled"]), byTab[id])
	}
	for _, z := range tabOrder {
		if !seenTabs[z] {
			build(firstNonEmpty(z, "imported"), "Imported flow", "", false, byTab[z])
		}
	}
	if len(result.Flows) == 0 && len(configs) > 0 {
		var nodes []Raw
		for _, id := range sortedKeys(configs) {
			nodes = append(nodes, configs[id])
		}
		build("imported", "Imported configuration", "", false, nodes)
	}

	result.Warnings = w.list()
	return result, nil
}

// Export renders a flow as a Node-RED export: a tab element followed by
// its nodes with "wires".
func Export(flow *engine.Flow) []Raw {
	out := []Raw{{
		"id":       flow.ID,
		"type":     "tab",
		"label":    flow.Name,
		"disabled": false,
		"info":     flow.Description,
	}}
	for _, id := range sortedNodeIDs(flow) {
		node := flow.Nodes[id]
		raw := Raw{
			"id":   node.ID,
			"type": node.Type,
			"name": node.Name,
		}
		for k, v := range exportConfig(node.Type, node.Config) {
			raw[k] = v
		}
		if !configTypes[node.Type] {
			raw["z"] = flow.ID
			raw["x"] = node.X + offsetX
			raw["y"] = node.Y + offsetY
			raw["wires"] = wiresOf(node, flow)
			if node.Disabled {
				raw["d"] = true
			}
		}
		out = append(out, raw)
	}
	return out
}

// convertNode maps one Node-RED node to an engine node.
func convertNode(raw Raw, w *warnings) *engine.Node {
	typ := str(raw["type"])
	node := &engine.Node{
		ID:       str(raw["id"]),
		Type:     typ,
		Name:     str(raw["name"]),
		X:        math.Max(0, num(raw["x"])-offsetX),
		Y:        math.Max(0, num(raw["y"])-offsetY),
		Disabled: boolean(raw["d"]),
	}
	if typ == "comment" && node.Name == "" {
		node.Name = "Comment"
	}
	config, known := convertConfig(typ, raw, w)
	if !known {
		w.add("node type %q is not available in Go-RED; the node was kept but cannot run", typ)
	}
	node.Config = config
	if info := str(raw["info"]); info != "" && typ != "comment" {
		node.Description = info
	}
	return node
}

// convertWires turns a node's "wires" into connections.
func convertWires(raw Raw, flow *engine.Flow, w *warnings) []engine.NodeConnection {
	wires, _ := raw["wires"].([]interface{})
	source := str(raw["id"])
	typ := str(raw["type"])
	var out []engine.NodeConnection
	for index, targetsRaw := range wires {
		targets, _ := targetsRaw.([]interface{})
		if len(targets) == 0 {
			continue
		}
		port := outputPort(typ, index)
		if port == "" {
			w.add("node type %q has one output in Go-RED; wires from output %d were dropped", typ, index+1)
			continue
		}
		for _, targetRaw := range targets {
			target := str(targetRaw)
			if _, ok := flow.Nodes[target]; !ok {
				w.add("a wire to node %q outside its tab was dropped", target)
				continue
			}
			out = append(out, engine.NodeConnection{
				ID:         fmt.Sprintf("%s-%d-%s", source, index, target),
				SourceNode: source,
				SourcePort: port,
				TargetNode: target,
				TargetPort: "input",
			})
		}
	}
	return out
}

// outputPort is the Go-RED port id for Node-RED output index i, or "" when
// the Go-RED node has no such output.
func outputPort(nodeType string, index int) string {
	if nodeType == "switch" {
		return strconv.Itoa(index)
	}
	if index == 0 {
		return "output"
	}
	return ""
}

// wiresOf renders a node's outgoing connections as Node-RED wires.
func wiresOf(node *engine.Node, flow *engine.Flow) [][]string {
	outputs := 1
	switch node.Type {
	case "switch":
		if rules, ok := node.Config["rules"].([]interface{}); ok {
			outputs = len(rules)
		} else {
			outputs = 0
		}
	case "comment", "link out", "tcp out", "udp out":
		if node.Type == "comment" || node.Type == "link out" {
			outputs = 0
		}
	}
	wires := make([][]string, outputs)
	for i := range wires {
		wires[i] = []string{}
	}
	for _, conn := range flow.Connections {
		if conn.SourceNode != node.ID {
			continue
		}
		index := 0
		if node.Type == "switch" {
			index, _ = strconv.Atoi(conn.SourcePort)
		}
		if index < 0 || index >= len(wires) {
			continue
		}
		wires[index] = append(wires[index], conn.TargetNode)
	}
	return wires
}

// ---------------------------------------------------------------- helpers

func str(v interface{}) string {
	switch x := v.(type) {
	case string:
		return x
	case float64:
		if x == math.Trunc(x) {
			return strconv.FormatInt(int64(x), 10)
		}
		return strconv.FormatFloat(x, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(x)
	case nil:
		return ""
	default:
		return fmt.Sprint(x)
	}
}

func num(v interface{}) float64 {
	switch x := v.(type) {
	case float64:
		return x
	case int:
		return float64(x)
	case string:
		f, err := strconv.ParseFloat(strings.TrimSpace(x), 64)
		if err != nil {
			return 0
		}
		return f
	default:
		return 0
	}
}

func boolean(v interface{}) bool {
	switch x := v.(type) {
	case bool:
		return x
	case string:
		return x == "true"
	default:
		return false
	}
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

func sortedKeys(m map[string]Raw) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func sortedNodeIDs(flow *engine.Flow) []string {
	ids := make([]string, 0, len(flow.Nodes))
	for id := range flow.Nodes {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

func typed(vt, v string) map[string]interface{} {
	if vt == "" {
		vt = "str"
	}
	return map[string]interface{}{"type": vt, "value": v}
}

func ref(pt, path string) map[string]interface{} {
	if pt == "" {
		pt = "msg"
	}
	return map[string]interface{}{"type": pt, "path": path}
}

func typedOf(v interface{}) (string, string) {
	m, _ := v.(map[string]interface{})
	return str(m["type"]), str(m["value"])
}

func refOf(v interface{}) (string, string) {
	m, _ := v.(map[string]interface{})
	return str(m["type"]), str(m["path"])
}

func stringList(v interface{}) []string {
	items, _ := v.([]interface{})
	out := make([]string, 0, len(items))
	for _, item := range items {
		if s := str(item); s != "" {
			out = append(out, s)
		}
	}
	return out
}

func toInterfaces(items []string) []interface{} {
	out := make([]interface{}, len(items))
	for i, s := range items {
		out[i] = s
	}
	return out
}
