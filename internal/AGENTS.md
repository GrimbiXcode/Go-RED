# internal/ — backend packages

Guidelines for everything under `internal/`. Package-specific files:
`engine/AGENTS.md`, `registry/AGENTS.md`, `state/AGENTS.md`, `nodes/AGENTS.md`.
The root `AGENTS.md` still applies.

## Packages

```
internal/
├── dto/         wire types for REST/WebSocket; source of web/src/types/generated.ts
├── engine/      flow definitions, per-flow runtime, routing, runtime events
├── nodered/     Node-RED flows.json import/export (engine.Flow <-> Node-RED arrays)
├── nodes/       one package per built-in node type, plus nodes/base helpers
├── registry/    NodeExecutor and optional node interfaces, NodeMetadata/schema, NodeRuntime
├── state/       FileStateManager: flow files, backups, quarantine, schema migrations
├── typedvalue/  Node-RED style typed inputs ({type, value} / {type, path}) and resolvers
└── webui/       the built editor embedded with go:embed (Dist()); dist/ is written by npm run build
```

## Dependency rules (verified against the imports)

```
registry     imports no other internal package
typedvalue   imports no other internal package
webui        imports no other internal package
engine       -> registry
state        -> engine        (implements engine.StateManager)
nodered      -> engine
dto          -> engine
nodes/base   -> registry, typedvalue
nodes/<x>    -> registry, nodes/base, typedvalue, and at most a sibling node package
                (httprequest -> httpproxy, tlsconfig; httpresponse, websocketlistener -> httpin;
                 mqttin, mqttout -> mqttbroker)
cmd/go-red   -> engine, registry, state, dto, nodered, webui, nodes/* (blank imports)
cmd/gentypes -> dto, registry, cmd/go-red/websocket
```

- `registry` must never import `engine` (cycle). Anything a node needs from the engine
  goes through `registry.NodeRuntime`, which `engine` fills in.
- `engine` imports node packages only in `_test.go` files. Production code in `engine`
  knows nodes only through `registry` interfaces.
- `nodes/*` never import `engine`. A node package that needs another node's live
  instance uses `NodeRuntime.GetNode` and a small Go interface exported by the sibling
  package (see `nodes/AGENTS.md`, config nodes).
- `internal/` packages are only imported by `cmd/`.

## Shared conventions

**Errors.** `engine` exports sentinel errors (`ErrFlowNotFound`, `ErrFlowExists`,
`ErrFlowNotDeployed`, `ErrInvalidFlowID`, `ErrInvalidFlow`, `ErrNodeInit`, `ErrPersist`)
and wraps them with `fmt.Errorf("%w: ...")`. `state` adds `ErrCorruptFlowFile` and
`ErrSchemaVersionTooNew`. Handlers in `cmd/go-red/main.go` map them with `errors.Is`
(`statusForError`) and never echo an unclassified error to a client. `registry` and
the node packages return plain `errors.New`/`fmt.Errorf` errors.

**Logging.** `log/slog` with key-value pairs (`slog.Warn("node execution failed",
"flow", id, "node", nodeID, "err", err)`). Nothing logs per message on the hot path;
drops and overloads are counted and logged sparingly (first occurrence, then every
1000th).

**Concurrency.** Every deployed flow has its own queue, dispatcher goroutine and
bounded execution slots; there is no engine-wide worker pool or per-message lock.
Nodes receive a `context.Context` that is cancelled on undeploy, redeploy and per-node
timeout, and must honour it (dial with context, `select` on `ctx.Done()`, close
listeners when `Start`'s context ends). See `engine/AGENTS.md` and `nodes/AGENTS.md`.

**Definition vs. runtime.** The engine keeps the editable `Flow` definition apart from
the deploy-time snapshot an `ActiveFlow` executes. Editing never changes a running flow
until it is redeployed. `GetFlow`/`GetAllFlows` return clones; `Deploy`/`AddFlow` take
ownership of the pointer passed in.

**Tests.** `testing` + `testify` (`assert`/`require`), table-driven where it fits.
CI (`.github/workflows/ci.yml`) runs `gofmt -l`, `go vet`, `go test -race -count=1 ./...`,
the engine benchmarks once, and the generated-types check.
Engine tests use isolated `registry.NewNodeRegistry()` instances with hand-rolled mock
nodes where they test engine mechanics, and the global registry with the real
inject/function/debug nodes for end-to-end flows. State tests use `t.TempDir()` and an
injected clock. Every network or timing node has a `ctx_test.go`.

## Wire contract and generated types

The frontend/backend contract is defined by Go structs only:

- `internal/dto` (flows, messages, events, error responses),
- `internal/registry` (`NodeMetadata`, `Port`, `Schema`, `Property` and the schema v2
  helper types),
- `cmd/go-red/websocket` (`WebSocketMessage`, `MessageType`).

`cmd/gentypes` turns them into `web/src/types/generated.ts`
(`go generate ./internal/dto/...` or `make generate-types`). CI regenerates and fails on
a diff (`make check-types` does the same locally). Whenever you change one of those
three packages or anything under `web/src/types/`:

1. `go build ./... && go vet ./...`
2. `go generate ./internal/dto/...`
3. `git diff --exit-code -- web/src/types/generated.ts` — a diff means the file was
   stale; commit it. Never edit `generated.ts` by hand.
4. `cd web && npx tsc --noEmit` — fix every call site in the same change.
5. `cd web && npm test`
6. For changes to REST or WebSocket payloads, smoke-test manually
   (`go run cmd/go-red/main.go` + `npm run dev`: create, deploy, inject).

Do not add a wire field, enum value or WebSocket message type that does not go through
these packages, and do not hand-write a TypeScript interface for a backend shape instead
of re-exporting it from `generated.ts`.

Engine types (`engine.Flow`, `engine.Message`, `engine.FlowStatus`, `time.Duration`
fields) are the internal representation, not the wire shape; `internal/dto` does the
conversion in one place (`convert.go`, `events.go`).

## Where the detail lives

- `docs/PROTOCOL.md` — REST and WebSocket messages, node schema fields, delivery
  guarantees, how to change the protocol.
- `docs/ARCHITECTURE.md` — layering, data flow, persistence and security overview.
- `docs/NODE_DEVELOPMENT.md` — writing a node, `nodes/base` helpers, context rules.
- `docs/PERFORMANCE.md` — benchmark results from `internal/engine/bench_test.go` and
  how to read them. Measure before claiming throughput.
- `docs/NODE_PALETTE_PLAN.md` — which Node-RED core nodes exist here and why some do not.
