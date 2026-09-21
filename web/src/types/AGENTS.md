# web/src/types

Six files. The wire format is generated from Go; everything else here is
UI-only and hand-written.

| File | Content |
|---|---|
| `generated.ts` | Generated, never edited by hand. `MessageType`, `FlowStatus`, `Position`, `Node`, `Connection`, `RetryPolicy`, `FlowConfig`, `Flow`, `FlowSummary`, `FlowCreateRequest`, `FlowUpdateRequest`, `Message`, `DeployResponse`, `ErrorResponse`, `FlowStatusEvent`, `NodeStatus`, `NodeStatusEvent`, `DebugMessage`, `NodeMetrics`, `FlowMetricsEvent`, `FlowSnapshot`, `SubscribeRequest`, `Port`, `Option`, `TypedInputOptions`, `Condition`, `Property`, `Schema`, `OutputsFrom`, `NodeMetadata`, `WebSocketMessage`. |
| `flow.ts` | Re-exports `Flow`, `FlowStatus`, `FlowConfig`, `NodeStatus`, `Node as FlowNode`, `Connection as NodeConnection`. Hand-written: `NodeRegistry` (type to metadata map used by `FlowCanvas`), `FlowMetadata`, `FlowState`. |
| `node.ts` | Re-exports `NodeMetadata`, `Port`, `Property as PropertySchema`, `Schema`, `Option`, `TypedInputOptions`, `Condition`, `OutputsFrom`. Hand-written: `NodeCategory`, `NodeProperty`, `NodeTypeDefinition`, `NodePaletteItem`. |
| `message.ts` | Re-exports `Message`, `WebSocketMessage`, `MessageType as WebSocketMessageType`. Hand-written: `MessageBatch`, `NodeMessage`, `FlowMessage`. |
| `api.ts` | Re-exports the flow, node and message types plus `FlowSummary`, `FlowCreateRequest`, `FlowUpdateRequest`, `DeployResponse`, `ErrorResponse`. Hand-written: `PaginatedResponse`, `Pagination`, `NodeDetailResponse`, `HealthCheckResponse`, `HealthCheck`, `StatsResponse`, `FlowExportRequest`, `FlowImportRequest`. |
| `index.ts` | `export *` of `flow`, `node`, `message` and `api` (not of `generated.ts` directly). |

Of the hand-written types only `NodeRegistry` is used outside this
directory. The others describe shapes with no backend counterpart (there is
no pagination, stats or structured health endpoint). Do not build on them;
remove them when you touch the file.

## Where `generated.ts` comes from

`internal/dto/flow.go` carries
`//go:generate go run ../../cmd/gentypes -out ../../web/src/types/generated.ts`.
`cmd/gentypes` reads the exported structs of `internal/dto`,
`internal/registry` (`NodeMetadata`, `Port`, `Property`, `Schema`, ...) and
`cmd/go-red/websocket` (`WebSocketMessage`, the `MessageType` union from
`AllMessageTypes`) and writes TypeScript. `make generate-types` runs it;
`make check-types` (CI) fails when the committed file is stale. ESLint
ignores the file.

Consequences:

- A backend shape is described once, in Go. A hand-written TypeScript
  interface that mirrors a Go struct is a regression; re-export from
  `generated.ts` instead, renaming where the old name is established, as
  `flow.ts` does for `FlowNode` and `NodeConnection`.
- `Node.config` is `Record<string, any>` and `Message.metadata` is
  `Record<string, string>`; the frontend cannot narrow them further than the
  Go side does. Node configuration is typed at runtime by the node type's
  `configSchema` (`src/schema/`).
- Optional Go fields (`omitempty`) become optional TypeScript properties.
  Check `generated.ts` instead of assuming a field is always present
  (`Node.name`, `Connection.sourcePort` and `targetPort` are optional).
- The `FlowStatus` union is `draft | running | error | deploying |
  undeploying`; the engine's own status names are translated in
  `internal/dto` and never reach the editor.

## Changing an interface (required steps)

Run these whenever you touch `internal/dto/**`,
`internal/registry/registry.go` (`NodeMetadata`, `Port`, `Property`,
`Schema`), `cmd/go-red/websocket/hub.go` (`WebSocketMessage`,
`MessageType`) or anything in this directory:

1. `go build ./... && go vet ./...`
2. `go generate ./internal/dto/...` (or `make generate-types`)
3. `git diff --exit-code -- web/src/types/generated.ts`. A diff means the
   file was stale or hand-edited: commit the regenerated file, never patch it.
4. `cd web && npm run typecheck`. Fix every call site in the same change.
5. `cd web && npx vitest run`. `test/types.test.ts` and the store tests must
   still pass.
6. If REST or WebSocket payloads changed: start the server and the dev
   editor once and exercise the affected path (create, deploy, inject), and
   update `docs/PROTOCOL.md`.

A new wire shape (struct field, enum value, message type) that does not go
through `internal/dto`, `internal/registry` or `cmd/go-red/websocket` and
show up in `generated.ts` is not allowed.

## Type tests and runtime checks

`test/types.test.ts` builds sample `Flow`, `FlowNode`, `NodeConnection`,
`FlowConfig` and `NodeStatus` values and checks their shape and the
`FlowStatus` union; it stops compiling when `generated.ts` drifts. There
are no runtime type guards in this directory; WebSocket payloads are checked
field by field in `store/bindServerEvents.ts` before they reach a store, and
`schema/validate.ts` validates node configuration against its schema.

## Using the types

- Import wire types through the domain files (`../types/flow`,
  `../types/node`, `../types/message`, `../types/api`) or, for event
  payloads and schema types that have no alias, directly from
  `../types/generated` (as `store/runtimeStore.ts` does for `NodeStatus`,
  `DebugMessage`, `NodeMetrics`, `FlowSnapshot`). Both are fine; do not
  duplicate a type to avoid the import.
- `Flow.nodes` is a `Record<string, Node>` keyed by node id and
  `Flow.connections` a `Connection[]`; `FlowSummary` (from `GET /api/flows`)
  carries `nodeCount` instead of the nodes. `Flow.order` drives the tab order.
- `WebSocketMessageType` is the `MessageType` union from `hub.go`; adding a
  message type means adding the Go constant, not a string here.
