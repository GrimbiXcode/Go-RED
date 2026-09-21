# Go-RED Protocol

This file is the single description of how the editor (or any other client)
talks to the server. Every payload named here is a Go struct in
`internal/dto` (node metadata: `internal/registry`) that `go generate
./internal/dto/...` turns into `web/src/types/generated.ts`. Neither side
hand-writes a wire shape; when this file and the generated types disagree,
the generated types win and this file has a bug.

Two channels:

- **REST under `/api`** creates, edits, deploys and deletes flows. JSON in,
  JSON out.
- **WebSocket at `/ws`** pushes what the server knows and the client cannot
  ask for often enough: flow lifecycle, node status, debug output, counters.
  It never modifies a flow. A browser tab holds exactly one connection.

## REST

All bodies and responses are JSON. Errors are `ErrorResponse {error}` with a
readable message; the status code is `400` for a malformed body or an
unsafe flow id, `404` for an unknown flow or node type, `409` when a flow
with that id already exists or a flow that is not running is undeployed,
`422` when a definition cannot run (the flow is left in status `error`),
`500` for persistence failures (the message is generic, the details go to
the server log).

| Method and path | Body | Response | Notes |
|---|---|---|---|
| `GET /api/health` | – | `{status, version, flows, deployed, uptimeSeconds}` | liveness; always public |
| `GET /api/version` | – | `{version, goVersion, os, arch}` | always public |
| `GET /metrics` | – | Prometheus text format | see *Operations*; guarded by the token like the API |
| `GET /api/flows` | – | `FlowSummary[]` | every known flow, running or not |
| `POST /api/flows` | `FlowCreateRequest {id?, name, description?}` | `201 Flow` | `id` is optional; it must be a safe file name |
| `POST /api/flows/import` | `Flow` (a Go-RED export) **or** a Node-RED `flows.json` array | `201 {status: "imported", format, flowId, originalId, name, flows: [{flowId, originalId, name}], warnings: string[], message}` | `format` is `go-red` or `node-red`; every imported flow gets a fresh id, `originalId` echoes the file's; a Node-RED array yields one flow per tab (`flows`, first one in `flowId`) and `warnings` lists what could not be mapped (see *Node-RED import and export*). Rate limited (`429`, see *Authentication and origins*) |
| `GET /api/flows/{id}` | – | `Flow` | the **draft definition**, including `deployedAt` |
| `PUT /api/flows/{id}` | `FlowUpdateRequest {name?, description?, nodes?, connections?, config?, order?}` | `Flow` | saves the draft only; a running instance is untouched until the next deploy. A body with only `order` moves the tab and does not count as an edit (`updatedAt` stays) |
| `DELETE /api/flows/{id}` | – | `{status: "deleted", flowId}` | stops the flow first |
| `POST /api/flows/{id}/deploy` | – | `DeployResponse {flowId, status, updatedAt?, deployedAt?, message?}` | runs a snapshot of the draft; `422` with the reason when it cannot run; rate limited (`429`) |
| `POST /api/flows/{id}/undeploy` | – | `DeployResponse` | the definition stays, status becomes `draft` |
| `GET /api/flows/{id}/export` | – | `Flow` as a download | `?format=node-red` returns the flow as a Node-RED `flows.json` array instead |
| `GET /api/nodes` | – | `NodeMetadata[]` | palette and config schemas |
| `GET /api/nodes/{type}` | – | `NodeMetadata` | |
| `GET /api/messages?flowId=&limit=` | – | `Message[]` | the engine's message log, oldest first; for scripts and tests, the editor does not poll it. Empty unless the server runs with `-message-log N` (the log costs a lock per routed message) |

`Flow.status` is one of `draft`, `running`, `error`, `deploying`,
`undeploying`. `Flow.config.maxConcurrency` bounds how many node executions
of the flow run at the same time; `0` (the default) means the server-wide
`-max-inflight` value. `Flow.order` (also on `FlowSummary`) is the tab position;
`GET /api/flows` is sorted by it (then by `createdAt`), a new flow gets
`max + 1`, and the editor writes new positions with `PUT {order}` when tabs
are dragged. It is omitted while it is `0` (flows persisted before it
existed). A `Node` in a flow is `{id, type, name?, description?,
position, config, disabled}`: `description` is the user's own note on the
node instance (Markdown); a node with `disabled: true` stays in the
definition but is neither started on deploy nor routed to, and cannot be
injected into. A draft differs from what is running when
`updatedAt > deployedAt`; the editor's Deploy button uses exactly that.

## Authentication and origins

Without configuration the server accepts unauthenticated requests from its
own origin only: a browser request or WebSocket handshake whose `Origin`
host differs from the request `Host` is refused (`403 origin not allowed`),
which keeps a page on another site from deploying or importing anything
with a cross-site request. `-allowed-origins` (or `GORED_ALLOWED_ORIGINS`,
comma-separated `scheme://host[:port]`, or `*`) allows other origins: they
get CORS headers (`Access-Control-Allow-Origin` echoing the origin,
`Vary: Origin`, allowed methods and the `Authorization`/`Content-Type`
headers) and their `OPTIONS` preflight is answered with `204`. Non-browser
clients send no `Origin` and are never affected.

`-auth-token` (or `GORED_AUTH_TOKEN`; at least 16 characters of
`A-Z a-z 0-9 . _ ~ -`) turns authentication on for everything under
`/api/`, for `/ws` and for `/metrics`; `/api/health`, `/api/version` and
the editor's static files stay public. Clients present the token as

| Where | How |
|---|---|
| REST and `/metrics` | `Authorization: Bearer <token>` |
| WebSocket handshake | the subprotocol list `gored, gored.token.<token>` (browsers cannot set headers there; the server selects `gored`), or `?access_token=<token>` for scripts |

A missing or wrong token is `401` with `WWW-Authenticate: Bearer`. The
editor stores the token in `localStorage` (`go-red.token`) after asking for
it once, and sends it on every request and handshake.

Import and deploy are rate limited per client address: `-rate-limit N`
requests per minute (default 60, burst of a quarter of that, `0` disables);
over the limit the answer is `429` with `Retry-After: 1`.

## Operations

`GET /metrics` exposes, in the Prometheus text format:

| Series | Labels | Meaning |
|---|---|---|
| `gored_build_info` | `version`, `go` | always 1 |
| `gored_uptime_seconds` | – | seconds since start |
| `gored_flows` | `status` (`running`, `draft`, `error`) | known flows by status |
| `gored_node_messages_total` | `flow`, `node`, `type` | messages a node handled since its flow was deployed |
| `gored_node_errors_total` | `flow`, `node`, `type` | failed executions of that node |
| `gored_messages_dropped_total` | – | messages dropped because a flow queue was full |
| `gored_events_dropped_total` | – | runtime events dropped because the event queue was full |
| `gored_websocket_clients` | – | connected editor clients |
| `go_goroutines`, `process_start_time_seconds` | – | process health |

Server configuration comes from, in rising precedence, the defaults, a YAML
file (`-config go-red.yaml` or `GORED_CONFIG`; keys `port`, `dataDir`,
`webDir`, `maxInflight`, `maxMessages`, `messageLog`, `logLevel`,
`authToken`, `allowedOrigins`, `rateLimit`, `backupKeep`, `backupInterval`;
unknown keys are an error), `GORED_*` environment variables
(`GORED_PORT`, `GORED_DATA_DIR`, `GORED_WEB_DIR`, `GORED_MAX_INFLIGHT`,
`GORED_MAX_MESSAGES`, `GORED_MESSAGE_LOG`, `GORED_LOG_LEVEL`,
`GORED_AUTH_TOKEN`, `GORED_ALLOWED_ORIGINS`, `GORED_RATE_LIMIT`,
`GORED_BACKUP_KEEP`, `GORED_BACKUP_INTERVAL`) and the flags of the same
names (`go-red -help`).

Flows are persisted one file per flow under `<dataDir>/flows/<id>.json`:
the flow's JSON object plus a top-level `schemaVersion` (currently 2; files
without it count as version 1 and are migrated on load). Before a file is
overwritten a copy goes to `<dataDir>/backups/<id>.<UTC stamp>.json`, at
most one per `-backup-interval` (default 10 min) and `-backup-keep` (default
5) per flow; a file the server cannot parse at startup is moved to
`<dataDir>/quarantine/` and logged, so one bad file never keeps the others
from loading.

## Node-RED import and export

`internal/nodered` converts between Node-RED's `flows.json` (a flat array of
objects with `type`, `z`, `x`, `y`, `wires`) and Go-RED flows, in both
directions. The import is detected by the body's first byte (`[`).

| Node-RED | Go-RED |
|---|---|
| `tab` | one `Flow` per tab (`label` → name, `info` → description, `disabled` → every node disabled); nodes without a tab land in a flow called *Imported flow* |
| node `x`, `y` (center) | `position` (top-left, `x - 70`, `y - 15`); the export adds the offset back |
| `wires[i][j]` | a connection from output port `i` to the target's `input`; port `0` is `output`, a switch's ports are `"0"`, `"1"`, … |
| `d: true` | `disabled: true` |
| config nodes (`mqtt-broker`, `tls-config`, `websocket-listener`, `websocket-client`, `http proxy`, anything without `wires`) | copied into every flow whose nodes reference them (`broker`, `server`, `tls`, `proxy`); exported without `z` |
| `subflow`, `subflow:*`, `group` | skipped, with a warning |
| known types (inject, debug, function, switch, change, template, delay, trigger, exec, range, rbe, batch, split, join, link in/out, catch, status, complete, comment, sort, csv, html, json, xml, yaml, file, file in, watch, http in/request/response, mqtt in/out, tcp in/out/request, udp in/out, websocket in/out, junction) | properties are mapped field by field (`mapping.go`), e.g. `payload`/`payloadType` → typed `payload`, `func` → `code`, switch `rules[].t/v/vt` → `operator/value{type,value}`; an unsupported switch operator stays in place so the ports keep lining up, with a warning |
| unknown types | kept with their raw properties (minus `id`, `type`, `z`, `x`, `y`, `wires`) and reported in `warnings`; the flow cannot deploy until they are replaced |

Warnings are de-duplicated with a count and returned by the import; the
editor shows a preview (format, tabs, unsupported types) before importing
and the warnings as a toast afterwards.

## WebSocket

Every frame is one JSON document:

```json
{ "type": "debug:message", "data": { "...": "..." }, "timestamp": "2026-09-19T10:00:00Z", "requestId": "optional" }
```

`type` is one of the constants in `cmd/go-red/websocket/hub.go`; `data` is
the payload named below. Unknown types are answered with an `error`
message, never silently dropped.

### Client to server

| `type` | `data` | Answer |
|---|---|---|
| `subscribe` | `SubscribeRequest {flowId}` | `flow:snapshot` to this client, then the flow's runtime events until `unsubscribe`. Subscribing again is harmless and sends a fresh snapshot. Unknown flow: `error`. |
| `unsubscribe` | `SubscribeRequest {flowId}` | none |
| `flow:list` | – | `flow:list` |
| `flow:get` | `{flowId}` | `flow:get` with a `Flow`, or `error` |
| `state:sync` | – | `state:sync {flows: Flow[], nodeTypes: NodeMetadata[]}` |
| `message:send` | `{flowId, nodeId, payload}` | injects `payload` at the node of a running flow; answers `message:send {status: "sent", flowId, nodeId}` or `error` |
| `ping` | – | `pong {message: "pong"}` |

The editor subscribes to the flow it has open, unsubscribes from the previous
one when the user switches, and subscribes again after every reconnect: the
snapshot is the catch-up mechanism, so nothing is queued for a client that
was away.

### Server to every client

| `type` | `data` | When |
|---|---|---|
| `flow:status` | `FlowStatusEvent {flowId, status, updatedAt?, deployedAt?, error?}` | after every REST mutation of the flow and every runtime change: deploy, undeploy, a deploy that failed (`status: "error"` with the reason in `error`), a flow that failed to restore at start-up |
| `flow:list` | `{flows: FlowSummary[]}` | together with every `flow:status`, after create, import and delete |
| `flow:delete` | `{flowId}` | after `DELETE /api/flows/{id}` |

### Server to subscribed clients

| `type` | `data` | When |
|---|---|---|
| `flow:snapshot` | `FlowSnapshot {flowId, status, nodeStatus, metrics, debug}` | answer to `subscribe`: the latest `NodeStatus` per node, the counters per node and the debug history the server still has |
| `node:status` | `NodeStatusEvent {flowId, nodeId, status: NodeStatus}` | whenever a node reports its status (see below) |
| `debug:message` | `DebugMessage {id, flowId, nodeId, nodeName?, nodeType?, level, topic?, payload, timestamp}` | every message a Debug node receives (`level: "debug"`) and every node execution error (`level: "error"`, `payload` is the error text). The server keeps the last 200 entries per flow and sends them in the snapshot; the browser keeps 500. |
| `flow:metrics` | `FlowMetricsEvent {flowId, nodes: {nodeId: {messages, errors}}, timestamp}` | once per second while the flow runs, whenever a counter changed. Counters start at zero on every deploy. |

When `flow:status` says a flow is no longer `running`, the client drops the
flow's node status and counters; the debug history stays until the user
clears it or the flow is deleted. Undeploy therefore sends no per-node
"status cleared" events.

### Server to one client

| `type` | `data` | When |
|---|---|---|
| `error` | `{error, message, ...context}` | `error` is a short stable code (`invalid payload`, `unknown message type`, `flow not found`, `failed to inject message`), `message` is readable, the context names what failed (`type`, `flowId`, `nodeId`) |
| `pong`, `flow:get`, `flow:list`, `state:sync`, `message:send`, `flow:snapshot` | see above | answers to the client's own requests |

`info` is reserved and not sent today.

## Node status

`NodeStatus {fill?, shape?, text?, timestamp?}` follows Node-RED: `fill` is
`red`, `green`, `yellow`, `blue` or `grey`; `shape` is `dot` or `ring`; an
empty status hides the indicator. Nodes report through
`NodeRuntime.ReportStatus(status, detail)`; the engine derives `fill` from
the status word (`error`, `fail`, `disconnect`, `closed` → red;
`connecting`, `waiting`, `pending`, `retry` → yellow; `connected`,
`listening`, `ready`, `ok`, `running` → green; anything else → blue) and
sets `text` to `status` plus `detail`. `http in` nodes share one listener
(`GORED_HTTP_NODE_PORT`, default 1880). Reporting nodes today: `mqtt in`
(`connecting`, then `connected <topic>`), `http in` (`listening <METHOD>
<path> on <addr>`), `tcp in` (`listening <addr>`).

## Debug node

Configuration `output`: `payload` (default) sends `msg.payload` when the
message has one, otherwise the whole message; `full` always sends the whole
message. `console: true` additionally prints the entry to the server's
stderr. `topic` is copied from `msg.topic`.

## Node schemas (the editor contract)

`GET /api/nodes` describes every node type with a `NodeMetadata`; its
`configSchema` tells the edit tray how to render and validate the node's
`config`. The value rules are `type`, `default`, `enum`, `min`, `max`,
`pattern` and the schema's `required` list. The remaining fields are editor
hints ("schema v2"); a property without `widget` is rendered by type (enum
→ select, boolean → checkbox, number → number, object/array → JSON editor,
string → text), so older schemas keep working.

| Property field | Meaning |
|---|---|
| `label`, `description`, `placeholder` | caption, help text under the field, input placeholder |
| `group`, `order` | section heading and sort position |
| `widget` | `text`, `textarea`, `number`, `boolean`, `select`, `typedInput`, `code`, `list`, `keyValue`, `credential`, `duration`, `json`, `stringList`, `nodeSelect` |
| `options` | labelled choices for `select` (`enum` is the unlabelled form) |
| `language` | syntax of a `code` field: `javascript`, `json`, `mustache`, `text` |
| `unit` | unit a `duration` value is stored in: `ms` or `s` (the widget converts) |
| `typedInput` | `{types, default?}`; types are `msg`, `flow`, `global` (stored as `{type, path}`) and `str`, `num`, `bool`, `json`, `env` (stored as `{type, value}`) |
| `items` | schema of one element of a `list` array |
| `nodeTypes`, `multiple` | `nodeSelect` offers nodes of these types from the same flow; with `multiple` the value is an array of IDs |
| `visibleWhen` | `{property, values}`: shown only while the other property's value, as a string, is one of `values` |

`icon` is the kebab-case name of a Lucide icon (`"timer"`); the editor
renders it from its own curated set and shows the category icon for
unknown names. Inline `<svg>` markup is accepted for third-party nodes and
sanitized.

Metadata-level fields: `outputsFrom {property, label?, min?}` makes the
number of output ports follow an array property (one port per Switch
rule; port IDs are the element indexes, `label` is a template such as
`{{operator}} {{value.value}}` in which a select property renders its
option label), `color` overrides the category color on the canvas, `help`
is the node's documentation in Markdown and `defaultName` the canvas label
of an unnamed node.

The editor validates on every keystroke and only enables *Done* while the
config passes; the server keeps its own checks in each node's `SetConfig`
and, for the shared rules, `registry.Schema.Validate`. A schema is checked
for well-formedness by `registry.NodeMetadata.Check` in the node tests.

## Delivery guarantees

Runtime events leave the engine through a bounded queue (4096 entries) and a
single dispatcher goroutine; publishing never blocks a node. Under overload
events are dropped and counted (`slog` warns), which is why the snapshot on
(re)subscribe, not the event stream, is the source of truth for node status
and counters. Debug entries carry ids; the client ignores a repeat of the
last id.

## Changing the protocol

- Add a field: extend the Go struct in `internal/dto`, run `go generate
  ./internal/dto/...`, commit the regenerated `generated.ts`, update this
  file.
- Add a message type: add the constant to `hub.go` (the hub test lists every
  type), handle it in `integration.go` and `store/bindServerEvents.ts`,
  document it here.
- Renaming or removing a field or a type is a breaking change and needs a
  migration note in `docs/`.
