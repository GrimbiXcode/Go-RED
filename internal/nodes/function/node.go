// Package function provides the Function node implementation.
package function

import (
	"context"
	"errors"
	"log"
	"sync"
	"time"

	"github.com/GrimbiXcode/Go-RED/internal/registry"
	"github.com/dop251/goja"
)

type FunctionNode struct {
	config FunctionConfig
	vm *goja.Runtime
	mu sync.Mutex
	compiledFunc goja.Callable
	hasError bool
	errorMessage string
}

type FunctionConfig struct {
	Code string `json:"code"`
	UseMsg bool `json:"useMsg"`
}

func NewFunctionNode() *FunctionNode {
	return &FunctionNode{
		config: FunctionConfig{Code: "return input;", UseMsg: false},
		vm: nil,
		compiledFunc: nil,
		hasError: false,
		errorMessage: "",
	}
}

func (n *FunctionNode) Execute(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	
	if n.hasError {
		return nil, errors.New("JavaScript compilation error: " + n.errorMessage)
	}
	
	vm := goja.New()
	n.setupSafeGlobals(vm)
	
	if n.config.UseMsg {
		msgObj := vm.NewObject()
		setObjectProperty(vm, msgObj, "payload", input["payload"])
		setObjectProperty(vm, msgObj, "topic", input["topic"])
		for k, v := range input {
			if k != "payload" && k != "topic" {
				setObjectProperty(vm, msgObj, k, v)
			}
		}
		vm.Set("msg", msgObj)
	} else {
		vm.Set("input", input)
	}
	
	val, err := n.compiledFunc(nil, vm.GlobalObject())
	if err != nil {
		return nil, errors.New("JavaScript execution error: " + err.Error())
	}
	
	if val == nil || goja.IsUndefined(val) {
		return input, nil
	}
	
	export := val.Export()
	if result, ok := export.(map[string]interface{}); ok {
		return result, nil
	}
	
	return map[string]interface{}{"payload": export}, nil
}

func (n *FunctionNode) compileCode() error {
	n.vm = goja.New()
	n.setupSafeGlobals()
	
	fullCode := "function process() {" + n.config.Code + "}"
	program, err := goja.Compile("", fullCode, false)
	if err != nil {
		n.hasError = true
		n.errorMessage = err.Error()
		return err
	}
	
	_, err = n.vm.RunProgram(program)
	if err != nil {
		n.hasError = true
		n.errorMessage = err.Error()
		return err
	}
	
	processFunc := n.vm.Get("process")
	if processFunc == nil || goja.IsUndefined(processFunc) {
		n.hasError = true
		n.errorMessage = "process function not found"
		return errors.New("process function not found in JavaScript code")
	}
	
	n.compiledFunc = processFunc.(goja.Callable)
	n.hasError = false
	n.errorMessage = ""
	return nil
}

func (n *FunctionNode) setupSafeGlobals() {
	math := n.vm.NewObject()
	math.Set("abs", n.vm.ToValue(func(x float64) float64 { return abs(x) }))
	math.Set("ceil", n.vm.ToValue(func(x float64) float64 { return ceil(x) }))
	math.Set("floor", n.vm.ToValue(func(x float64) float64 { return floor(x) }))
	math.Set("round", n.vm.ToValue(func(x float64) float64 { return round(x) }))
	math.Set("max", n.vm.ToValue(func(args ...float64) float64 {
		max := args[0]
		for _, v := range args[1:] {
			if v > max { max = v }
		}
		return max
	}))
	math.Set("min", n.vm.ToValue(func(args ...float64) float64 {
		min := args[0]
		for _, v := range args[1:] {
			if v < min { min = v }
		}
		return min
	}))
	n.vm.Set("Math", math)
	
	console := n.vm.NewObject()
	console.Set("log", n.vm.ToValue(func(args ...interface{}) { log.Println(args...) }))
	console.Set("error", n.vm.ToValue(func(args ...interface{}) { log.Println("ERROR:", args...) }))
	n.vm.Set("console", console)
}

func abs(x float64) float64 { if x < 0 { return -x }; return x }
func ceil(x float64) float64 { return float64(int64(x + 0.5)) }
func floor(x float64) float64 { return float64(int64(x)) }
func round(x float64) float64 { return float64(int64(x + 0.5)) }

func setObjectProperty(vm *goja.Runtime, obj *goja.Object, key string, value interface{}) error {
	val := vm.ToValue(value)
	return obj.Set(key, val)
}

func (n *FunctionNode) Validate() error {
	if n.config.Code == "" {
		return errors.New("code cannot be empty")
	}
	
	oldVM := n.vm
	oldFunc := n.compiledFunc
	oldError := n.hasError
	oldErrorMsg := n.errorMessage
	
	err := n.compileCode()
	
	n.vm = oldVM
	n.compiledFunc = oldFunc
	n.hasError = oldError
	n.errorMessage = oldErrorMsg
	
	return err
}

func (n *FunctionNode) GetConfig() map[string]interface{} {
	return map[string]interface{}{"code": n.config.Code, "useMsg": n.config.UseMsg}
}

func (n *FunctionNode) SetConfig(config map[string]interface{}) error {
	if code, ok := config["code"].(string); ok {
		n.config.Code = code
		n.mu.Lock()
		n.hasError = false
		n.errorMessage = ""
		n.mu.Unlock()
	}
	if useMsg, ok := config["useMsg"].(bool); ok {
		n.config.UseMsg = useMsg
	}
	return n.Validate()
}

func (n *FunctionNode) GetError() (bool, string) {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.hasError, n.errorMessage
}

func init() {
	reg := registry.GetGlobalRegistry()
	err := reg.RegisterFactory("function", func() registry.NodeExecutor {
		return NewFunctionNode()
	}, registry.NodeMetadata{
		ID: "function",
		Type: "function",
		Name: "Function",
		Description: "Executes JavaScript code to process messages",
		Category: "function",
		Inputs: []registry.Port{
			{ID: "input", Name: "Input", Description: "Input message", Required: true},
		},
		Outputs: []registry.Port{
			{ID: "output", Name: "Output", Description: "Output message", Required: true},
		},
		ConfigSchema: registry.Schema{
			Properties: map[string]registry.Property{
				"code": {Type: "string", Description: "JavaScript code to execute", Default: "return input;"},
				"useMsg": {Type: "boolean", Description: "Use msg object instead of input", Default: false},
			},
			Required: []string{"code"},
		},
		Icon: "<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="#2196F3"><path d="M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm-2 15l-5-5 1.41-1.41L10 14.17l7.59-7.59L19 8l-9 9z"/></svg>",
		Tags: []string{"function", "javascript", "script", "process"},
	})
	if err != nil { panic(err) }
}
