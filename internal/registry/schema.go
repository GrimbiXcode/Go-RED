package registry

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// Editor widgets a Property can ask for. The frontend has one control per
// widget (web/src/schema); unknown widgets fall back to the rendering by
// Type, so adding a widget here is only useful together with its control.
const (
	WidgetText       = "text"       // one-line text
	WidgetTextarea   = "textarea"   // multi-line text
	WidgetNumber     = "number"     // number with Min/Max
	WidgetBoolean    = "boolean"    // checkbox
	WidgetSelect     = "select"     // dropdown from Enum or Options
	WidgetTypedInput = "typedInput" // {type, value} or {type, path}, see TypedInputOptions
	WidgetCode       = "code"       // code editor, syntax from Language
	WidgetList       = "list"       // ordered list of objects described by Items
	WidgetKeyValue   = "keyValue"   // map[string]string
	WidgetCredential = "credential" // masked text
	WidgetDuration   = "duration"   // number with a unit picker, stored in Unit
	WidgetJSON       = "json"       // JSON editor for a whole object/array
	WidgetStringList = "stringList" // []string
	WidgetNodeSelect = "nodeSelect" // ID of another node in the flow (or IDs, with Multiple)
)

// Types a WidgetTypedInput may offer, matching internal/typedvalue.
const (
	TypedMsg    = "msg"
	TypedFlow   = "flow"
	TypedGlobal = "global"
	TypedString = "str"
	TypedNumber = "num"
	TypedBool   = "bool"
	TypedJSON   = "json"
	TypedEnv    = "env"
)

// PropertyRefTypes are the TypedInput types for a location (typedvalue.PropertyRef).
var PropertyRefTypes = []string{TypedMsg, TypedFlow, TypedGlobal}

// ValueTypes are the TypedInput types for a value (typedvalue.Value).
var ValueTypes = []string{TypedString, TypedNumber, TypedBool, TypedJSON, TypedEnv, TypedMsg, TypedFlow, TypedGlobal}

var (
	knownWidgets = map[string]bool{
		WidgetText: true, WidgetTextarea: true, WidgetNumber: true, WidgetBoolean: true,
		WidgetSelect: true, WidgetTypedInput: true, WidgetCode: true, WidgetList: true,
		WidgetKeyValue: true, WidgetCredential: true, WidgetDuration: true, WidgetJSON: true,
		WidgetStringList: true, WidgetNodeSelect: true,
	}
	knownLanguages  = map[string]bool{"javascript": true, "json": true, "mustache": true, "text": true}
	knownUnits      = map[string]bool{"ms": true, "s": true}
	knownTypedTypes = map[string]bool{
		TypedMsg: true, TypedFlow: true, TypedGlobal: true, TypedString: true,
		TypedNumber: true, TypedBool: true, TypedJSON: true, TypedEnv: true,
	}
)

// Option is one labelled choice of a WidgetSelect property.
type Option struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

// TypedInputOptions configures a WidgetTypedInput property.
type TypedInputOptions struct {
	// Types the user can pick from (Typed* constants). PropertyRefTypes
	// for a location, ValueTypes for a value.
	Types []string `json:"types"`
	// Default is the type preselected for an empty value.
	Default string `json:"default,omitempty"`
}

// Condition makes a property visible only while another property of the
// same schema has one of Values (compared as strings, so booleans are
// "true"/"false").
type Condition struct {
	Property string   `json:"property"`
	Values   []string `json:"values"`
}

// OutputsFrom derives a node's output ports from an array-typed config
// property. Label is a template for each port's label with {{field}}
// placeholders referring to the element (dotted paths allowed); the
// element index plus one is used when it is empty or yields nothing.
type OutputsFrom struct {
	Property string `json:"property"`
	Label    string `json:"label,omitempty"`
	// Min is the number of ports shown while the array is shorter.
	Min int `json:"min,omitempty"`
}

// Check reports the first thing that makes s unusable by the editor: an
// unknown widget, language, unit or typed-input type, a list without an
// item schema, a required or referenced property that does not exist.
// Nodes call it from tests so a typo cannot reach the UI.
func (s Schema) Check() error {
	for _, name := range s.Required {
		if _, ok := s.Properties[name]; !ok {
			return fmt.Errorf("required property %q is not defined", name)
		}
	}
	for name, prop := range s.Properties {
		if err := prop.check(s); err != nil {
			return fmt.Errorf("property %q: %w", name, err)
		}
	}
	return nil
}

func (p Property) check(parent Schema) error {
	if p.Widget != "" && !knownWidgets[p.Widget] {
		return fmt.Errorf("unknown widget %q", p.Widget)
	}
	if p.Language != "" && !knownLanguages[p.Language] {
		return fmt.Errorf("unknown language %q", p.Language)
	}
	if p.Unit != "" && !knownUnits[p.Unit] {
		return fmt.Errorf("unknown unit %q", p.Unit)
	}
	if p.Pattern != "" {
		if _, err := regexp.Compile(p.Pattern); err != nil {
			return fmt.Errorf("invalid pattern: %w", err)
		}
	}
	switch p.Widget {
	case WidgetList:
		if p.Items == nil {
			return fmt.Errorf("list widget needs Items")
		}
		if err := p.Items.Check(); err != nil {
			return fmt.Errorf("items: %w", err)
		}
	case WidgetTypedInput:
		if p.TypedInput == nil || len(p.TypedInput.Types) == 0 {
			return fmt.Errorf("typedInput widget needs TypedInput.Types")
		}
		for _, t := range p.TypedInput.Types {
			if !knownTypedTypes[t] {
				return fmt.Errorf("unknown typed input type %q", t)
			}
		}
		if p.TypedInput.Default != "" && !contains(p.TypedInput.Types, p.TypedInput.Default) {
			return fmt.Errorf("typed input default %q is not one of its types", p.TypedInput.Default)
		}
	case WidgetSelect:
		if len(p.Enum) == 0 && len(p.Options) == 0 {
			return fmt.Errorf("select widget needs Enum or Options")
		}
	}
	if p.VisibleWhen != nil {
		if _, ok := parent.Properties[p.VisibleWhen.Property]; !ok {
			return fmt.Errorf("visibleWhen refers to unknown property %q", p.VisibleWhen.Property)
		}
		if len(p.VisibleWhen.Values) == 0 {
			return fmt.Errorf("visibleWhen needs values")
		}
	}
	return nil
}

// Check extends Schema.Check with the metadata-level rules: OutputsFrom
// must name an array property.
func (m NodeMetadata) Check() error {
	if err := m.ConfigSchema.Check(); err != nil {
		return err
	}
	if m.OutputsFrom != nil {
		prop, ok := m.ConfigSchema.Properties[m.OutputsFrom.Property]
		if !ok {
			return fmt.Errorf("outputsFrom refers to unknown property %q", m.OutputsFrom.Property)
		}
		if prop.Type != "array" {
			return fmt.Errorf("outputsFrom property %q must be an array, is %q", m.OutputsFrom.Property, prop.Type)
		}
	}
	return nil
}

// Validate checks a configuration against the schema's value rules:
// required properties are present and non-empty, strings match Pattern
// and Enum, numbers respect Min/Max. Properties hidden by VisibleWhen are
// skipped. It returns one message per problem, sorted by property name,
// or nil. Nodes still run their own SetConfig validation; this is the part
// the editor shares with them.
func (s Schema) Validate(config map[string]interface{}) []string {
	var problems []string
	for _, name := range s.Required {
		prop := s.Properties[name]
		if !prop.visible(s, config) {
			continue
		}
		if isEmptyValue(config[name]) {
			problems = append(problems, fmt.Sprintf("%s is required", name))
		}
	}
	for name, prop := range s.Properties {
		value, ok := config[name]
		if !ok || isEmptyValue(value) || !prop.visible(s, config) {
			continue
		}
		if msg := prop.validateValue(value); msg != "" {
			problems = append(problems, fmt.Sprintf("%s %s", name, msg))
		}
	}
	sort.Strings(problems)
	return problems
}

func (p Property) visible(parent Schema, config map[string]interface{}) bool {
	if p.VisibleWhen == nil {
		return true
	}
	actual, ok := config[p.VisibleWhen.Property]
	if !ok {
		actual = parent.Properties[p.VisibleWhen.Property].Default
	}
	return contains(p.VisibleWhen.Values, fmt.Sprint(actual))
}

func (p Property) validateValue(value interface{}) string {
	switch v := value.(type) {
	case string:
		if p.Pattern != "" {
			if re, err := regexp.Compile(p.Pattern); err == nil && !re.MatchString(v) {
				return fmt.Sprintf("must match %s", p.Pattern)
			}
		}
		if len(p.Enum) > 0 && v != "" && !contains(p.Enum, v) {
			return fmt.Sprintf("must be one of %s", strings.Join(p.Enum, ", "))
		}
	case float64:
		if p.Min != nil && v < *p.Min {
			return fmt.Sprintf("must be at least %v", *p.Min)
		}
		if p.Max != nil && v > *p.Max {
			return fmt.Sprintf("must be at most %v", *p.Max)
		}
	case int:
		return p.validateValue(float64(v))
	}
	return ""
}

func isEmptyValue(v interface{}) bool {
	switch x := v.(type) {
	case nil:
		return true
	case string:
		return strings.TrimSpace(x) == ""
	case []interface{}:
		return len(x) == 0
	case map[string]interface{}:
		return len(x) == 0
	}
	return false
}

func contains(list []string, s string) bool {
	for _, item := range list {
		if item == s {
			return true
		}
	}
	return false
}
