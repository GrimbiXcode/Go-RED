# internal/engine — flow engine

The engine owns flow definitions, deploys them into per-flow runtimes, routes messages
between nodes and publishes runtime events. It imports `internal/registry` and nothing
else from `internal/` (node packages appear only in `_test.go` files).

## Files

| File | Contents |
|---|---|
| `engine.go` | `EngineConfig`, `StateManager`, `FlowEngine`, `ActiveFlow`, sentinel errors, `ValidateFlowID`, lifecycle, dispatch, routing, message log |
| `flow.go` | `Flow`, `Node`, `NodeConnection`, `FlowConfig`, `RetryPolicy`, `FlowStatus`, `NewFlow`, `Validate`, `Clone` |
| `message.go` | `Message`, `NewMessage`, `NewMessageWithContext`, `Clone`, `AddToPath` |
| `events.go` | `Event` types, the event hub, debug log, node status, counters, `/metrics` helpers |
| `engine_test.go`, `flow_test.go`, `events_test.go`, `dispatch_test.go`, `eagerly_ready_test.go`, `order_test.go` | unit tests |
| `phase0_test.go` … `phase6_network_test.go`, `flow_integration_test.go` | end-to-end tests with real nodes |
| `bench_test.go` | benchmarks behind `docs/PERFORMANCE.md` |

## Two views of a flow

`FlowEngine.flows` holds the editable definition of every known flow (deployed or not);
`FlowEngine.active` holds an `ActiveFlow` per deployed flow. `deployLocked` takes
`def.Clone()` as the runtime snapshot; `ActiveFlow.Flow` is never mutated afterwards.
Editing a definition (`UpdateFlow`) does not touch the running flow until `DeployFlow`
is called again. `Deploy` on a running flow is the redeploy: the old runtime is stopped
first. `Flow.UpdatedAt` later than `Flow.DeployedAt` means "has undeployed changes".

`GetFlow`/`GetAllFlows` return clones. `Deploy` and `AddFlow` take ownership of the
pointer; callers must not mutate it afterwards.

## Public API (all on `*FlowEngine`)

- Construction: `NewFlowEngine(config, registry)`, `DefaultEngineConfig()`,
  `SetStateManager`/`GetStateManager`, `Start` (event dispatcher + metrics loop),
  `Stop` (stops every flow, idempotent).
- Definitions: `CreateFlow(id, name, description)`, `AddFlow(flow)`,
  `UpdateFlow(id, func(*Flow) error)`, `DeleteFlow`, `GetFlow`, `GetAllFlows` (sorted by
  `Order`, `CreatedAt`, ID), `GetFlowStatus`, `LoadAllFlows` (redeploys flows that were
  `FlowStatusActive` when saved; failures become `FlowStatusError`).
- Runtime: `Deploy(flow)`, `DeployFlow(id)`, `Undeploy(id)`, `IsDeployed`,
  `InjectMessage(flowID, nodeID, payload)`, `SubmitMessage(msg)`, `GetFlowContext`
  (a running flow's `ContextStore` and `EventBus`), `GlobalContext`.
- Observability: `SubscribeEvents`, `GetDebugLog`/`ClearDebugLog`, `GetNodeStatuses`,
  `GetMetrics`, `NodeStats`, `FlowCounts`, `EventsDropped`, `MessagesDropped`,
  `AddMessageToLog`/`GetMessageLog`/`GetMessageLogForFlow`/`ClearMessageLog`/
  `SetMaxMessageLog`.

Sentinel errors: `ErrFlowNotFound`, `ErrFlowExists`, `ErrFlowNotDeployed`,
`ErrInvalidFlowID`, `ErrInvalidFlow`, `ErrNodeInit`, `ErrPersist`. Return them wrapped
(`fmt.Errorf("%w: %s", ErrFlowNotFound, id)`); `cmd/go-red/main.go` maps them to HTTP
status codes with `errors.Is` in `statusForError`. `ErrPersist` may wrap file-system
details and is never echoed to clients. `ValidateFlowID` enforces
`^[A-Za-z0-9][A-Za-z0-9_-]{0,127}$` because IDs become file names and URLs.

## Locking

- `e.mu` (RWMutex) protects `flows` and `active`. Functions with the `Locked` suffix
  (`deployLocked`, `stopActiveLocked`, `persistLocked`, `nextOrderLocked`) require it
  held; everything else takes it.
- `persistLocked` calls the state manager under `e.mu` so nothing mutates the flow while
  it is serialized. Persist failures are logged and returned as `ErrPersist`.
- `messageLogMu`, `debugMu`, `statusMu` and `stopMu` are independent and never nested
  with `e.mu` inside a node execution.
- Node counters are atomics; `ActiveFlow.lastMetrics` is touched only by `metricsLoop`.

## Dispatch (per flow, no engine-wide pool)

`deployLocked` creates per flow: `msgChan` (capacity `EngineConfig.MessageBufferSize`,
default 1000), `slots` (capacity `FlowConfig.MaxConcurrency`, or
`EngineConfig.MaxInflightPerFlow` = 1024 when 0), a `ContextStore`, an `EventBus`, a
`nodeCounter` per node, a context derived from the engine context, and one dispatcher
goroutine (`processFlowMessages`). Nodes are instantiated with
`registry.InitializeNode` (factory → `SetConfig` → `Validate`); the first failure closes
already-created `Closeable` nodes, sets `FlowStatusError`, persists, publishes a
`FlowStatusEvent` and a `DebugEvent`, and returns `ErrNodeInit`. Disabled nodes are
neither instantiated, started nor routed to.

`processMessage` looks up the targets (`findTargetNodes`: root nodes for an empty
`Path`, otherwise the connections leaving the last node in `Path`, filtered by
`Message.OutputPort` when set), takes a slot per target (waiting on the slot or the flow
context), and runs each target in its own goroutine tracked by `ActiveFlow.inflight`.
Each execution gets `context.WithTimeout(activeFlow.ctx, EngineConfig.DefaultTimeout)`
with a `registry.NodeRuntime` attached via `registry.WithRuntime`; a message never
inherits the context of the node that produced it.

`executeNode` calls `ExecuteMulti` for `registry.MultiOutputExecutor` nodes (one
submitted message per non-nil port, `OutputPort` set) and `Execute` otherwise (single
output, every outgoing connection). Errors go to `handleNodeError` (counter, `slog`,
`DebugEvent` with `DebugLevelError`, `EventBus.PublishError`); success to
`handleNodeComplete` (`EventBus.PublishComplete`).

`submitMessage` never blocks: a stopped flow or a full queue drops the message and
counts it (`MessagesDropped`, `ActiveFlow.dropped`). `InjectMessage` returns an error
on a full queue instead.

`startEmittingNodes` runs `Start(ctx, emit)` of every `registry.EmittingNode` in its
own goroutine tracked by `ActiveFlow.wg`. For nodes that also implement
`registry.EagerlyReadyNode`, Deploy waits until they call `registry.SignalReady` (or
`eagerReadyTimeout` = 2 s) so a message injected right after `Deploy` cannot race ahead
of a Catch/Status/Complete subscription. `NodeRuntime.SubmitToNode` and `linkout` go
through `submitToFlowNode`, which enqueues a message whose `Path` ends at the target
node (same flow only).

Undeploy (`stopActiveLocked`): cancel the flow context, wait for the dispatcher and
`Start` goroutines, wait up to `DefaultTimeout + 1s` for in-flight executions
(`waitTimeout`, warns if they ignore cancellation), then `Close()` every
`registry.Closeable`, then remove from `active`. `Undeploy` on a known but not running
flow is a no-op; on an unknown flow returns `ErrFlowNotFound`.

`EngineConfig.MaxRetries`/`RetryBackoff` and `FlowConfig.RetryPolicy`/`Timeout` exist in
the structs and on disk but nothing in the engine reads them; there is no retry logic.

## Runtime events (`events.go`)

`Event` is implemented by `FlowStatusEvent` (deployed/undeployed/deploy failure),
`NodeStatusEvent` (`NodeStatus{Fill, Shape, Text, Timestamp}`), `DebugEvent`
(`DebugLevelDebug|Warn|Error`, from Debug nodes via `NodeRuntime.Debug` and from node
errors) and `FlowMetricsEvent` (per-node `NodeMetrics{Messages, Errors}`, published at
most once per second while counters change). Publishing goes through a bounded queue
(`eventQueueSize` = 4096) drained by one dispatcher goroutine; a full queue drops and
counts (`EventsDropped`). Subscribers (`SubscribeEvents`) run on the dispatcher
goroutine and must not block. The WebSocket hub in `cmd/go-red/websocket` is the main
subscriber; it re-reads `GetNodeStatuses`/`GetDebugLog`/`GetMetrics` on subscribe
because the stream may drop.

Debug entries are kept per flow in a ring of `debugLogSize` = 200. Node statuses
reported through `NodeRuntime.ReportStatus` are turned into `NodeStatus` by
`fillForStatus` (keyword heuristics for the colour) and remembered until undeploy.

The message log (`GET /api/messages`) is a fixed-size ring, off by default
(`EngineConfig.MessageLogSize` = 0); with size 0 `AddMessageToLog` returns before taking
any lock.

## Testing patterns in this package

- `createTestEngine()` (`engine_test.go`) uses the global registry with the real
  `inject`/`function`/`debug` nodes; `newTestEngine(reg)` (`phase0_test.go`) takes an
  isolated `registry.NewNodeRegistry()` populated with mock nodes (`passthroughNode`,
  `captureSinkNode`, `routerNode`; `blockingNode`/`contextProbeNode` in
  `dispatch_test.go`). Register the same pointer you inspect later.
- `memoryStateManager` (`engine_test.go`) is the in-memory `StateManager` for
  lifecycle and `LoadAllFlows` tests.
- `phaseN_*_test.go` deploy real node chains and assert on a capture node, the debug
  log or events; `events_test.go` subscribes with `SubscribeEvents` and collects.
- `dispatch_test.go` checks the slot bound, that undeploy waits for in-flight
  executions, that a full queue drops and counts, and that redeploy cycles leak no
  goroutines (`runtime.NumGoroutine`). Keep those green when touching dispatch.
- Benchmarks: `BenchmarkFunctionChain`, `BenchmarkInjectFunctionDebug`,
  `BenchmarkSwitchFanout`, `BenchmarkSplitJoin`. Update `docs/PERFORMANCE.md` from
  `go test -bench . -run ^$ ./internal/engine` when the hot path changes.
- Run `go test -race ./internal/engine/...` before committing.

## Rules

- No engine-wide lock, channel or log line on the per-message path.
- Do not add fields to `Flow`/`Message` that the wire needs without going through
  `internal/dto` (see `internal/AGENTS.md`, wire contract). `Message.Context` and
  `Message.OutputPort` are `json:"-"` engine-only state.
- New optional node capabilities are interfaces in `internal/registry` that the engine
  type-asserts; do not make the engine depend on concrete node packages.
- Undeploy must always terminate: anything a node can block on must watch the flow
  context, and cleanup must be best-effort (`closeNodeExecutors` logs, never returns).
