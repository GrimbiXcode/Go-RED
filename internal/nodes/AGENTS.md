# internal/nodes — built-in nodes

One Go package per node type, each self-registering into the global registry from
`init()`. Node packages import `internal/registry`, `internal/nodes/base`,
`internal/typedvalue` and, in a few cases, a sibling node package; never
`internal/engine`. The general guide is `docs/NODE_DEVELOPMENT.md`; the field reference
for schemas is `docs/PROTOCOL.md`, "Node schemas".

## Layout

```
internal/nodes/
├── AGENTS.md
├── schema_test.go        checks every registered node's metadata (see below)
├── base/                 shared helpers: config.go, context.go, typed.go, values.go
└── <package>/            node.go, node_test.go, ctx_test.go for network/timing nodes
                          (httpin also has server.go and response_handle.go)
```

Registered type, package and optional interfaces (`M` = `MultiOutputExecutor`,
`E` = `EmittingNode`, `C` = `Closeable`, `R` = `EagerlyReadyNode`):

| Category | Type → package (interfaces) |
|---|---|
| `input` | `inject` (E C) |
| `output` | `debug` |
| `function` | `function` (goja), `change`, `range` → `rangenode`, `rbe` (M), `template`, `delay` (M E C), `trigger` (C), `exec` → `execnode` |
| `parser` | `json` → `jsonnode`, `yaml` → `yamlnode`, `csv` → `csvnode`, `xml` → `xmlnode`, `html` → `htmlnode` |
| `flow-control` | `junction`, `comment`, `catch` (E R), `status` (E R), `complete` (E R), `link in` → `linkin`, `link out` → `linkout` (M), `switch` → `switchnode` (M), `split` (M), `join` (M C), `sort` → `sortnode` (M), `batch` (M E C) |
| `storage` | `file` (M), `file in` → `filein` (M), `watch` (E) |
| `network` | `http in` → `httpin` (E), `http response` → `httpresponse`, `http request` → `httprequest`, `mqtt in` → `mqttin` (E), `mqtt out` → `mqttout`, `websocket in` → `websocketin` (E), `websocket out` → `websocketout`, `tcp in` → `tcpin` (E), `tcp out` → `tcpout`, `tcp request` → `tcprequest`, `udp in` → `udpin` (E), `udp out` → `udpout` |
| `config` | `tls-config` → `tlsconfig`, `http proxy` → `httpproxy`, `mqtt-broker` → `mqttbroker` (C), `websocket-listener` → `websocketlistener` (C), `websocket-client` → `websocketclient` (C) |

Package names differ from the registered type where the type is a Go keyword
(`switch`, `range`) or would shadow an import (`exec`, `json`, `yaml`, `csv`, `xml`,
`html`, `sort`); `NodeMetadata.Type` and the factory key stay the Node-RED name.

A new node must be blank-imported in `cmd/go-red/main.go` and in `schema_test.go`,
otherwise it is neither served nor checked.

## Registration and the executor contract

`internal/nodes/debug/node.go` and `internal/nodes/catch/node.go` are the reference
implementations. The shape every node follows:

```go
func init() {
    err := registry.GetGlobalRegistry().RegisterFactory("my-type",
        func() registry.NodeExecutor { return &Node{} },   // no arguments; config comes via SetConfig
        registry.NodeMetadata{ID: "my-type", Type: "my-type", Name: "...", Description: "...",
            Category: "...", Icon: "...", Inputs: ..., Outputs: ..., ConfigSchema: ..., Help: "..."})
    if err != nil { panic(err) }
}
```

- `registry.InitializeNode` runs factory → `SetConfig(config)` → `Validate()` at
  deploy. `SetConfig` receives the JSON-decoded editor config (numbers are `float64`);
  read it with `base.Config` accessors or `base.Decode` into a struct, then return
  `n.Validate()`. `GetConfig` returns the same keys in config-map form
  (`base.StringsToConfig`, `base.ValueToConfig`, `base.PropertyRefToConfig`).
- `Execute(ctx interface{}, input)`: `ctx` is a `context.Context` carrying the
  `NodeRuntime`; use `base.Context(ctx)` and `base.Runtime(ctx)`. Do not mutate
  `input`; copy with `base.CloneMap` and return the copy. A returned error fails the
  message: the engine counts it, writes a debug entry and publishes a `NodeErrorEvent`
  for Catch nodes.
- Implement `registry.MultiOutputExecutor` to pick output ports (`switchnode`, `rbe`,
  `file`); a nil or missing port sends nothing. One message per port per call: to emit
  several messages on the same port, return the first and send the rest with
  `rt.SubmitToNode(rt.NodeID, payload)` (`split`, `sortnode`, `filein`).
- Implement `registry.EmittingNode` for sources: `Start(ctx, emit)` must block until
  `ctx` is done, close its listener or subscription, and stop emitting. Nodes with no
  input port (`watch`, `tcpin`, ...) keep an `Execute` that only satisfies the interface.
- Implement `registry.Closeable` when you hold connections, timers or queued work
  (`delay`, `batch`, `join`, `trigger`, the config nodes).
- Event subscribers (`catch`, `status`, `complete`) implement `EagerlyReadyNode` and
  call `registry.SignalReady(ctx)` right after subscribing in `Start`.
- Report state with `rt.ReportStatus(status, detail)` (`httpin`, `mqttin`, `tcpin`)
  and non-fatal problems with `rt.ReportError(err)` (`watch`), so the editor and Catch
  nodes see them without failing the message.
- Sequence metadata is a plain `"parts"` key on the message map (`split`, `join`,
  `sortnode`); there is no dedicated engine field.

## Context cancellation

The engine cancels a node's context on undeploy, redeploy and per-node timeout, then
waits for the execution before closing resources. A node that ignores its context holds
undeploy up and can leak. Rules (details in `docs/NODE_DEVELOPMENT.md`, "Context
cancellation"): dial with a context (`DialContext`, `http.NewRequestWithContext`),
derive deadlines from `ctx.Deadline()`, never `time.Sleep` (select on a timer and
`ctx.Done()`), return from `Start` and close listeners when `ctx` ends, release queued
work in `Close()`, wrap the context error so `errors.Is(err, context.Canceled)` works.
Every network or timing node has a `ctx_test.go` proving that a cancelled context makes
`Execute` return promptly and `Start` return after cancel; add one for a new such node.

## Config nodes

`Category: "config"`, no ports, referenced by ID from a consumer's string property
(`Widget: registry.WidgetNodeSelect` with `NodeTypes`). The consumer resolves the live
instance lazily with `rt.GetNode(id)` inside `Execute`/`Start` and type-asserts a small
interface exported by the config package: `mqttbroker.Broker`, `tlsconfig.Provider`,
`httpproxy.Provider`; `websocketin`/`websocketout` reach `websocketlistener`/
`websocketclient` the same way. Deploy instantiates nodes in map order and launches
`Start` goroutines without ordering, so a config node must be usable right after its own
`SetConfig`: `mqttbroker` connects, `websocketclient` dials and `websocketlistener`
mounts its handler there, and they implement `Closeable`, not `EmittingNode`.

## Shared process-wide services

- `httpin` owns one `*http.Server` for all `http in` and `websocket-listener` nodes,
  started lazily on `GORED_HTTP_NODE_PORT` (default 1880, `0` for a random port in
  tests), separate from the editor's `-port`. `httpin.RegisterHandler(path, handler)`
  mounts extra handlers; `httpin.KeyResponseHandle` carries a `*httpin.ResponseHandle`
  on the message so `httpresponse` can answer the pending request.
- `execnode` is disabled unless `GORED_ENABLE_EXEC` is set; it runs argv-based commands,
  never a shell. `httprequest` enforces timeouts, a response-size cap and a redirect cap
  but deliberately no URL block-list (see its package doc).

## Config schema rules (enforced by `schema_test.go`)

Every registered node must pass `NodeMetadata.Check()` and have `Help` (Markdown) and
`Description`. Every property needs `Label`, `Widget` and a unique `Order > 0`;
`WidgetDuration` needs `Unit`, `WidgetCode` needs `Language`, `WidgetTypedInput` is
`Type: "object"`, `WidgetList`/`WidgetStringList` are `Type: "array"`,
`WidgetKeyValue`/`WidgetJSON` are `object` or `array`. A node's own `Default`s must pass
`Schema.Validate`, except required placeholders (empty URL, port 0). `Icon` is a Lucide
name from the set in `web/src/components/icons.ts`; add the icon there in the same
change if it is new. Category colours live in the frontend and are keyed by the
category string, so a new category needs a frontend change too.

## `base` helpers (use them, do not copy them)

```go
c := base.Config(config)                       // Has, String, Int, Int64, Float, Bool,
host := c.String("host", "localhost")          // Duration (number = ms, or "1.5s"),
                                               // StringSlice, Map, Slice
base.Decode(config, &settings)                 // JSON round trip into a tagged struct
base.ToFloat, base.ParseFloat, base.ToInt, base.ToBool, base.ToString, base.ToBytes
base.CloneMap, base.FloatPtr, base.StringsToConfig
base.ParseValue / base.ValueToConfig           // typedvalue.Value  <-> {type, value}
base.ParsePropertyRef / base.PropertyRefToConfig // typedvalue.PropertyRef <-> {type, path}
base.Context(ctx), base.Runtime(ctx)           // context.Context / *registry.NodeRuntime
base.Resolvers(ctx, msg), base.Resolver(ctx, msg) // typedvalue resolvers over msg/flow/global
```

`base` has table-driven tests for every helper; a node package tests only what it adds.

## Testing patterns

- `node_test.go`: construct the node directly, call `SetConfig`/`Validate`, then
  `Execute(context.Background(), input)`. Without a runtime every `NodeRuntime` method
  is a no-op, so plain executions need no engine. For runtime behaviour build one with
  `registry.NewNodeRuntime(...)` (nil event bus and callbacks are allowed), install a
  collector with `SetDebugSink`, and pass `registry.WithRuntime(ctx, rt)`.
- `ctx_test.go`: cancelled or short-deadline contexts against a real local listener
  (`net.Listen("tcp", "127.0.0.1:0")`), fake clients for brokers, assertions on prompt
  return and `errors.Is(err, context.Canceled)`.
- End-to-end behaviour through the engine lives in `internal/engine/phaseN_*_test.go`.
- `schema_test.go` fails the build of a node whose metadata is malformed; run
  `go test ./internal/nodes/...` after touching any schema.
