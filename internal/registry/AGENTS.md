# internal/registry — node types, metadata, runtime services

The registry is the catalogue of node types and the only package a node implementation
depends on. It imports nothing else from `internal/`; `engine` imports it, never the
other way round. Everything a node needs from the engine is handed in through
`NodeRuntime`.

## Files

| File | Contents |
|---|---|
| `registry.go` | `NodeExecutor` and the optional interfaces, `NodeFactory`, `Port`, `Property`, `Schema`, `NodeMetadata`, `Node`, `NodeRegistry` |
| `schema.go` | schema v2: `Widget*`/`Typed*` constants, `Option`, `TypedInputOptions`, `Condition`, `OutputsFrom`, `Schema.Check`, `NodeMetadata.Check`, `Schema.Validate` |
| `runtime.go` | `NodeRuntime`, `DebugOutput`, `NewNodeRuntime`, `WithRuntime`, `RuntimeFromContext` |
| `eventbus.go` | `EventBus` with `NodeErrorEvent`, `NodeStatusEvent`, `NodeCompleteEvent` |
| `context_store.go` | `ContextStore` (flow/global key-value context) |
| `ready.go` | `WithReady`, `SignalReady` for `EagerlyReadyNode` |

## Node interfaces

```go
type NodeExecutor interface {
    Execute(ctx interface{}, input map[string]interface{}) (map[string]interface{}, error)
    Validate() error
    GetConfig() map[string]interface{}
    SetConfig(config map[string]interface{}) error
}
type NodeFactory func() NodeExecutor
```

`ctx` is typed `interface{}` for historical reasons; the engine always passes a
`context.Context` (use `nodes/base.Context` / `base.Runtime`). Optional interfaces the
engine detects by type assertion:

- `MultiOutputExecutor` — `ExecuteMulti(ctx, input) (map[string]map[string]interface{}, error)`:
  one payload per output port ID (`Port.ID`); a missing or nil port sends nothing.
  Used instead of `Execute` when implemented.
- `Closeable` — `Close() error`, called once per instance on undeploy or failed deploy,
  after the flow context is cancelled and `Start` has returned.
- `EmittingNode` — `Start(ctx, emit func(payload)) error`, run in its own goroutine at
  deploy; must block until `ctx` is done and call `emit` for every originated message.
- `EagerlyReadyNode` — marker (`EagerlyReady()`) on an `EmittingNode` whose `Start`
  subscribes to this flow's events synchronously; Deploy waits until it calls
  `SignalReady(ctx)` (bounded by the engine's 2 s timeout). Implemented by
  `catch`/`status`/`complete`. Not for nodes whose readiness depends on an external
  source (listeners, brokers).

## Registry

`GetGlobalRegistry()` is the process-wide singleton every built-in node registers into
from `init()`; `NewNodeRegistry()` gives tests an isolated one.

- `RegisterFactory(nodeType, factory, metadata)` builds a `Node{Type, Metadata, Factory}`
  and calls `RegisterNode`, which rejects an empty type and duplicates.
- `GetExecutor(type)` calls the factory; `InitializeNode(type, config)` then runs
  `SetConfig` and `Validate` — this is what the engine uses at deploy.
- `GetMetadata`, `GetAllNodes` (backs `GET /api/nodes`), `GetNodesByCategory`,
  `IsRegistered`, `Unregister`.

All methods are guarded by one `sync.RWMutex`. Errors are plain
`errors.New("node type not found: " + t)` style; there are no sentinel errors here.
There is no plugin loader: node types are Go packages compiled in and blank-imported
from `cmd/go-red/main.go`.

## Metadata and schema (the editor contract)

`NodeMetadata{ID, Type, Name, Description, Category, Inputs, Outputs []Port,
ConfigSchema Schema, Icon, Tags, OutputsFrom *OutputsFrom, Color, Help, DefaultName}`.
`Schema{Properties map[string]Property, Required []string}`.

`Property` has the value rules `Type` (`string`, `number`, `boolean`, `object`,
`array`), `Default`, `Enum`, `Min`/`Max` (`*float64`), `Pattern`, plus the schema v2
editor hints: `Label`, `Placeholder`, `Group`, `Order`, `Widget`, `Language`, `Unit`,
`Options`, `TypedInput`, `Items` (nested `*Schema` for `WidgetList`), `NodeTypes`,
`Multiple`, `VisibleWhen`. Widgets are the `Widget*` constants in `schema.go`
(`text`, `textarea`, `number`, `boolean`, `select`, `typedInput`, `code`, `list`,
`keyValue`, `credential`, `duration`, `json`, `stringList`, `nodeSelect`); typed-input
types are the `Typed*` constants matching `internal/typedvalue` (`PropertyRefTypes` for
locations, `ValueTypes` for values). Field meanings are documented in
`docs/PROTOCOL.md`, "Node schemas"; authoring rules in `docs/NODE_DEVELOPMENT.md`.

- `Schema.Check()` / `NodeMetadata.Check()` reject unknown widgets, languages, units and
  typed-input types, a list without `Items`, a select without `Enum`/`Options`, a
  required or `VisibleWhen` property that does not exist, and an `OutputsFrom` that does
  not name an array property. `internal/nodes/schema_test.go` runs `Check` on every
  registered node.
- `Schema.Validate(config) []string` applies the value rules (required non-empty,
  `Pattern`, `Enum`, `Min`/`Max`, skipping properties hidden by `VisibleWhen`). Nodes
  still validate in `SetConfig`; the editor mirrors these rules client-side.
- Adding a widget here is only useful together with its control in `web/src/schema`.

`NodeMetadata`, `Port`, `Schema`, `Property` and the helper types are part of the wire
contract: `cmd/gentypes` generates them into `web/src/types/generated.ts`. After any
change to these structs follow the regeneration steps in `internal/AGENTS.md`
("Wire contract and generated types").

## NodeRuntime

The engine builds one `NodeRuntime` per node execution (and per `Start`) with
`NewNodeRuntime(flowID, nodeID, nodeType, flowContext, globalContext, events, submit,
getNode)`, attaches it with `WithRuntime(ctx, rt)` and nodes read it with
`RuntimeFromContext(ctx)`. Every method is nil-safe, so a node executed in a unit test
with `context.Background()` gets `ok == false` and no-ops.

- Fields: `FlowID`, `NodeID`, `NodeType`, `FlowContext`, `GlobalContext`
  (`*ContextStore` with `Get`/`Set`/`Delete`/`Keys`).
- `Debug(DebugOutput{Payload, Topic, Level})` — debug sidebar entry (sink installed by
  the engine via `SetDebugSink`).
- `ReportStatus(status, detail)` — publishes a `NodeStatusEvent`; the engine turns it
  into an editor status indicator. `ReportError(err)` — publishes a `NodeErrorEvent`
  for a non-fatal problem (errors returned from `Execute` are published automatically).
- `OnError`, `OnStatus`, `OnComplete` — subscribe for the flow's lifetime; call once,
  from `Start`.
- `SubmitToNode(nodeID, payload)` — enqueue `payload` as if `nodeID` had just emitted
  it (same flow only). Used by `linkout` to reach `linkin`, and by `split`, `sortnode`
  and `filein` with their own `NodeID` to emit several messages on one port.
- `GetNode(nodeID) (NodeExecutor, bool)` — the live executor of another node in the
  same flow, for config nodes. Resolve it lazily in `Execute`/`Start`, never in
  `SetConfig`: deploy instantiates nodes in map order, so a referenced node may not
  exist yet during `SetConfig`.

## EventBus and ContextStore

Each active flow owns an `EventBus`; the engine publishes `NodeErrorEvent` on every
failed execution and `NodeCompleteEvent` on every successful one, and installs its own
`OnStatus` handler for the editor. Handlers run synchronously on the publishing
goroutine in registration order and must not block or call back into the engine in a
way that could deadlock. There is no replay: a subscriber that arrives late misses
earlier events (hence `EagerlyReadyNode`). A `complete` node whose output feeds back
into a node in its own scope re-triggers itself forever; scope it to specific nodes.

`ContextStore` is a mutex-guarded map. One per flow (`flow.get/set`), one per engine
(`global.get/set`); `nodes/base.Resolvers` wires both into `typedvalue` resolvers.

## Testing patterns

- `registry_test.go`: `NewNodeRegistry()` per test, `MockNodeExecutor` and the
  embedding mocks `mockMultiOutputExecutor`, `mockLifecycleExecutor`,
  `mockEagerlyReadyExecutor` (which calls `SignalReady` then blocks on `ctx.Done()`).
- `schema_test.go`: table tests for `Check` and `Validate`; add a case for every new
  rule.
- `runtime_test.go`: context round trip and nil-safety of every `NodeRuntime` method.
- `eventbus_test.go`, `context_store_test.go`, `ready_test.go`: one test each per
  behaviour; keep them when changing semantics.
