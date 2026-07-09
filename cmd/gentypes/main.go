// Command gentypes generates the canonical TypeScript wire types
// (web/src/types/generated.ts) from the Go structs that define the
// frontend/backend contract: internal/dto, internal/registry, and
// cmd/go-red/websocket.
//
// It is invoked via `go generate ./internal/dto/...` (see the go:generate
// directive in internal/dto/flow.go) or `make generate-types`. Do not edit
// generated.ts by hand — change the Go structs and regenerate.
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"reflect"
	"strings"
	"time"

	"github.com/GrimbiXcode/Go-RED/cmd/go-red/websocket"
	"github.com/GrimbiXcode/Go-RED/internal/dto"
	"github.com/GrimbiXcode/Go-RED/internal/registry"
)

var (
	durationType   = reflect.TypeOf(time.Duration(0))
	timeType       = reflect.TypeOf(time.Time{})
	rawMessageType = reflect.TypeOf(json.RawMessage(nil))
)

// enumInfo describes a Go string-constant enum to render as a TypeScript
// string-literal union instead of an interface.
type enumInfo struct {
	name   string
	values []string
}

// generator walks Go struct types via reflection and renders them as
// TypeScript interfaces, resolving nested struct/slice/map field types
// automatically and emitting dependencies before dependents.
type generator struct {
	enumNames      map[reflect.Type]string
	enumOrder      []enumInfo
	emittedStructs map[reflect.Type]bool
	structOrder    []string
}

func newGenerator() *generator {
	return &generator{
		enumNames:      make(map[reflect.Type]string),
		emittedStructs: make(map[reflect.Type]bool),
	}
}

func (g *generator) registerEnum(t reflect.Type, name string, values []string) {
	g.enumNames[t] = name
	g.enumOrder = append(g.enumOrder, enumInfo{name: name, values: values})
}

// resolveType maps a Go reflect.Type to its TypeScript type expression,
// registering (and recursively resolving) any struct type it depends on.
func (g *generator) resolveType(t reflect.Type) string {
	if t == durationType || t == timeType {
		log.Fatalf("gentypes: raw %s used in a wire DTO field — wire types must use "+
			"seconds-as-int or RFC3339 strings, not %s directly (see internal/dto)", t, t)
	}
	if t == rawMessageType {
		return "any"
	}
	if name, ok := g.enumNames[t]; ok {
		return name
	}

	switch t.Kind() {
	case reflect.Ptr:
		return g.resolveType(t.Elem())
	case reflect.Slice, reflect.Array:
		return g.resolveType(t.Elem()) + "[]"
	case reflect.Map:
		return "Record<string, " + g.resolveType(t.Elem()) + ">"
	case reflect.String:
		return "string"
	case reflect.Bool:
		return "boolean"
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64:
		return "number"
	case reflect.Interface:
		return "any"
	case reflect.Struct:
		g.ensureStruct(t)
		return t.Name()
	default:
		log.Fatalf("gentypes: unsupported field type %s (kind %s)", t, t.Kind())
		return ""
	}
}

// ensureStruct renders a Go struct type as a TypeScript interface exactly
// once, appending it to structOrder after any nested struct types it
// depends on (so the output never references a type before its definition).
func (g *generator) ensureStruct(t reflect.Type) {
	if g.emittedStructs[t] {
		return
	}
	g.emittedStructs[t] = true

	var fields []string
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if f.PkgPath != "" {
			continue // unexported field
		}
		tag := f.Tag.Get("json")
		if tag == "-" || tag == "" {
			continue
		}
		parts := strings.Split(tag, ",")
		name := parts[0]
		omitempty := false
		for _, p := range parts[1:] {
			if p == "omitempty" {
				omitempty = true
			}
		}
		optional := omitempty || f.Type.Kind() == reflect.Ptr
		tsType := g.resolveType(f.Type)

		suffix := ""
		if optional {
			suffix = "?"
		}
		fields = append(fields, fmt.Sprintf("  %s%s: %s;", name, suffix, tsType))
	}

	body := fmt.Sprintf("export interface %s {\n%s\n}", t.Name(), strings.Join(fields, "\n"))
	g.structOrder = append(g.structOrder, body)
}

func renderEnum(e enumInfo) string {
	quoted := make([]string, len(e.values))
	for i, v := range e.values {
		quoted[i] = "'" + v + "'"
	}
	return fmt.Sprintf("export type %s = %s;", e.name, strings.Join(quoted, " | "))
}

func messageTypeValues() []string {
	values := make([]string, len(websocket.AllMessageTypes))
	for i, v := range websocket.AllMessageTypes {
		values[i] = string(v)
	}
	return values
}

func flowStatusValues() []string {
	return []string{
		string(dto.FlowStatusDraft),
		string(dto.FlowStatusRunning),
		string(dto.FlowStatusError),
		string(dto.FlowStatusDeploying),
		string(dto.FlowStatusUndeploying),
	}
}

func main() {
	outPath := flag.String("out", "", "output file path for the generated TypeScript")
	flag.Parse()
	if *outPath == "" {
		fmt.Fprintln(os.Stderr, "gentypes: -out is required")
		os.Exit(1)
	}

	g := newGenerator()
	g.registerEnum(reflect.TypeOf(websocket.MessageType("")), "MessageType", messageTypeValues())
	g.registerEnum(reflect.TypeOf(dto.FlowStatus("")), "FlowStatus", flowStatusValues())

	// Entry points; ensureStruct recursively pulls in every nested type
	// (Position, NodeStatus, Node, Connection, RetryPolicy, FlowConfig, Port,
	// Property, Schema, ...) so they don't need to be listed individually.
	g.ensureStruct(reflect.TypeOf(dto.Flow{}))
	g.ensureStruct(reflect.TypeOf(dto.FlowSummary{}))
	g.ensureStruct(reflect.TypeOf(dto.FlowCreateRequest{}))
	g.ensureStruct(reflect.TypeOf(dto.FlowUpdateRequest{}))
	g.ensureStruct(reflect.TypeOf(dto.Message{}))
	g.ensureStruct(reflect.TypeOf(registry.NodeMetadata{}))
	g.ensureStruct(reflect.TypeOf(websocket.WebSocketMessage{}))

	var buf bytes.Buffer
	buf.WriteString("// Code generated by cmd/gentypes from internal/dto, internal/registry, and\n")
	buf.WriteString("// cmd/go-red/websocket. DO NOT EDIT — run `go generate ./internal/dto/...`\n")
	buf.WriteString("// (or `make generate-types`) to regenerate after changing those packages.\n\n")

	for _, e := range g.enumOrder {
		buf.WriteString(renderEnum(e))
		buf.WriteString("\n\n")
	}
	for _, s := range g.structOrder {
		buf.WriteString(s)
		buf.WriteString("\n\n")
	}

	content := strings.TrimRight(buf.String(), "\n") + "\n"
	if err := os.WriteFile(*outPath, []byte(content), 0o644); err != nil {
		log.Fatalf("gentypes: failed to write %s: %v", *outPath, err)
	}
}
