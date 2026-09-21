# cmd/go-red

The HTTP server: flow engine, node registry, file state manager, REST API,
WebSocket hub and the built editor in one binary. The wire contract (every
route, payload and WebSocket message) is in `docs/PROTOCOL.md`.

## Files

| File | Holds |
|---|---|
| `main.go` | `main` (config → logging → registry → state manager → engine → hub → `http.Server`; SIGINT/SIGTERM shutdown with a 30 s deadline), `newRouter`, the REST handlers, `webHandler`/`spaHandler`/`editorNotBuiltHandler`, `sanitizeString`, `writeJSON`/`writeError`/`writeEngineError`/`statusForError`, `decodeJSON` |
| `config.go` | `Config`, `defaultConfig`, `loadConfig`, `applyEnv`, `loadConfigFile`, `validate` |
| `auth.go` | `validateToken`, `tokenFromRequest`, `isProtectedPath`, `requireAuth`, `originAllowed`, `corsMiddleware`, `rateLimiter` |
| `ops.go` | `handleVersion` (`GET /api/version`), `handleMetrics` (`GET /metrics`, Prometheus text format written by hand) |
| `websocket/hub.go` | `Hub`, `Client`, `WebSocketMessage`, the `MessageType` constants and `AllMessageTypes`, read/write pumps, per-client subscriptions |
| `websocket/integration.go` | `WebSocketHandler`: forwards engine events to clients, answers client queries, `FlowChanged`/`FlowDeleted` |
| `main_test.go`, `config_test.go`, `security_test.go`, `ops_test.go`, `websocket/hub_test.go` | tests, see below |

Node packages are registered by the blank imports in `main.go`; a new node
type must be added to that list or it never reaches the registry. Hot-path
rules (no per-message logging, no engine-wide locks) are in the root
`AGENTS.md`.

## Routes (`newRouter`)

Go 1.22 pattern routing on a plain `http.ServeMux`:

```
GET    /api/health                public, no auth
GET    /api/version               public, no auth
GET    /metrics                   Prometheus text format
GET    /api/flows                 []dto.FlowSummary
POST   /api/flows                 dto.FlowCreateRequest → 201 dto.Flow
POST   /api/flows/import          rate limited; Go-RED JSON or Node-RED flows.json
GET    /api/flows/{id}            dto.Flow
PUT    /api/flows/{id}            dto.FlowUpdateRequest (partial) → dto.Flow
DELETE /api/flows/{id}
POST   /api/flows/{id}/deploy     rate limited → dto.DeployResponse
POST   /api/flows/{id}/undeploy   → dto.DeployResponse
GET    /api/flows/{id}/export     ?format=node-red for a Node-RED export
GET    /api/nodes                 []registry.NodeMetadata
GET    /api/nodes/{type}
GET    /api/messages              ?flowId=&limit= (empty unless -message-log > 0)
GET    /ws                        only when a WebSocketHandler was passed
/                                 the editor (SPA fallback)
```

The REST API is the only write path. After a successful create, update,
delete or import the handler calls `Notifier.FlowChanged`/`FlowDeleted`
(the `WebSocketHandler`, or `noopNotifier` when the router is built without
one). Deploy and undeploy do not notify; their result reaches clients as the
engine's `flow:status` event.

Wire types live in `internal/dto` (`ToWire`, `ToWireSummary`,
`FlowUpdateRequest.ApplyTo`, `PopulateFromWire`); handlers never expose
`engine.Flow` directly. `go generate ./internal/dto/...` turns those types
plus `AllMessageTypes` into `web/src/types/generated.ts`.

## Middleware order

`corsMiddleware(allowedOrigins, requireAuth(token, mux))`: a request passes
the origin policy first, then the token check, then the mux. Inside the mux,
`POST /api/flows/import` and `POST /api/flows/{id}/deploy` are wrapped by
`rateLimiter.limit` (429 with `Retry-After: 1`).

- `corsMiddleware`: same-origin requests and static files pass untouched. A
  cross-origin request (Origin host != request Host) to `/api/*`, `/ws` or
  `/metrics` is refused with 403 unless its origin is allowed (or the list
  is `*`); allowed ones get CORS headers and a 204 preflight.
- `requireAuth`: with an empty token it returns `next` unchanged. Otherwise
  `isProtectedPath` (`/api/*` except health and version, `/ws`, `/metrics`)
  requires the token, compared in constant time, else 401 with
  `WWW-Authenticate`. `tokenFromRequest` reads `Authorization: Bearer`; on
  `/ws` only it also accepts the `gored.token.<token>` subprotocol and the
  `access_token` query parameter.
- `rateLimiter`: token bucket per client IP (`clientKey`), burst
  `perMinute/4` (at least 3), buckets pruned after 10 minutes of silence.
  `newRateLimiter(0)` returns nil and `limit` becomes a no-op.

## Configuration (`loadConfig`)

Precedence, lowest to highest: `defaultConfig()` → YAML file (`-config`,
else `GORED_CONFIG`; unknown keys are an error) → `GORED_*` environment →
flags. Flags are registered with the values so far as their defaults, which
is how an unset flag keeps what file and environment said. `validate`
rejects bad ports, non-positive queue sizes, negative limits, unknown log
levels, malformed tokens (`^[A-Za-z0-9._~-]{16,}$`) and origins that are not
`scheme://host[:port]`. `-version` prints `main.version` (set with
`-ldflags "-X main.version=..."`) and exits.

Keys: port, dataDir, webDir, maxInflight, maxMessages, messageLog, logLevel,
authToken, allowedOrigins, rateLimit, backupKeep, backupInterval (table in
the root README). A new setting touches `Config`, `defaultConfig`,
`applyEnv`, the flag list, `validate`, `config_test.go` and that table.

## The editor

`npm run build` in `web/` writes to `internal/webui/dist`, which
`internal/webui` embeds (`go:embed all:dist`). `webHandler` serves that
build; `-web-dir`/`GORED_WEB_DIR` is an optional development override that
serves a directory instead. A binary built without a frontend build answers
extension-less paths with a 503 hint page (`editorNotBuiltHandler`); the API
works regardless. `spaHandler` serves existing files as-is, falls back to
`index.html` for extension-less paths (client-side routes survive a reload,
`Cache-Control: no-cache`) and answers a missing file with an extension with
a plain 404.

## Errors

`statusForError` maps the engine's sentinel errors: `ErrFlowNotFound` → 404,
`ErrFlowExists` and `ErrFlowNotDeployed` → 409, `ErrInvalidFlowID` → 400,
`ErrInvalidFlow` and `ErrNodeInit` → 422, anything else → 500.
`writeEngineError` sends the error text for the classified ones and a
generic `"internal error"` (after logging the real one) for 500s, so file
system or internal details never reach a client. Every error body is
`dto.ErrorResponse{Error: ...}`. Malformed JSON is a 400
`"invalid request body"`; bodies are capped at `maxBodyBytes` (10 MiB) with
`http.MaxBytesReader`.

On the WebSocket `publicError` does the same job: engine errors are
user-facing except `engine.ErrPersist`, which becomes `"failed to save flow"`.

## Sanitization

`sanitizeString` strips NUL bytes and truncates to 10000 characters. It is
applied to flow name and description on create, update and import (the name
is also trimmed and required) and, through `sanitizeNodes` and the update
handler, to every node's `Type`, `Name` and `Description`. Node `config`
maps are passed through as sent. Imported flows always get a fresh UUID
(the original id is echoed back as `originalId`); flow ids are validated by
the engine (`ErrInvalidFlowID`).

## WebSocket

One `WebSocketMessage{type, data, timestamp, requestId?}` per frame; inbound
frames capped at 512 KiB; 256 buffered outbound messages per client; pings
every 30 s with a 60 s pong deadline. A full send buffer drops the message
(logged) rather than blocking the hub; the hub's broadcast channel holds 1024.
`Hub.CheckOrigin` is set by `main` from the allowed-origins list (default
`sameOrigin`); the upgrader selects the `gored` subprotocol.

Message types (`MessageType` in `hub.go`; `AllMessageTypes` must list every
constant because `cmd/gentypes` enumerates it by reflection):

- Client queries answered on the same type: `flow:list`, `flow:get`,
  `state:sync` (all flows plus node types), `ping` → `pong`.
- `subscribe`/`unsubscribe` (`dto.SubscribeRequest`): a client receives
  runtime events only for flows it subscribed to. Subscribing answers with
  `flow:snapshot` (`dto.FlowSnapshot`: status, node statuses, metrics, debug
  log), the catch-up source of truth because events can be dropped under load.
- Runtime events to subscribers (`Hub.BroadcastToFlow`): `node:status`,
  `debug:message`, `flow:metrics`, translated from `engine.Event` values in
  `onEngineEvent`.
- Events to every client (`Hub.Broadcast`): `flow:status` (also after every
  REST change), `flow:list` (after any change), `flow:delete`.
- `message:send` injects a payload at a node (`engine.InjectMessage`).
- `error` carries `{error: <short code>, message, ...fields}`; unknown types
  get one too. A panic in a handler is recovered in `Client.dispatch`.

## Tests

- `newTestServer(t)` / `newTestServerWith(t, routerOptions{...})`
  (`main_test.go`): a started engine with an in-memory `StateManager` and
  the real `newRouter` without a WebSocket handler, so tests hit exactly the
  routes `main` wires. An empty `WebDir` becomes a temp dir. Use
  `newTestServerWith` for auth, origin and rate-limit cases.
- `do(t, h, method, target, body)` marshals `body` as JSON and returns the
  `httptest.ResponseRecorder`; `decodeBody(t, w, &v)` unmarshals it.
  `security_test.go` adds `request(method, target, headers)` and
  `serve(h, req)` for header-level tests.
- `config_test.go` drives `loadConfig(args, envOf(map))` directly;
  `websocket/hub_test.go` covers the hub without a network (`TestMessageType`
  lists every constant, so extend it when adding one). CI runs
  `go test -race ./...`.

## Changing things here

- New route: add it to `newRouter`, use `dto` types for the body, return
  errors through `writeEngineError`, add a `main_test.go` case, document it
  in `docs/PROTOCOL.md`.
- New WebSocket message: constant plus `AllMessageTypes` in `hub.go`, a case
  in `HandleMessage` or `onEngineEvent`, `go generate ./internal/dto/...`, a
  handler in `web/src/store/bindServerEvents.ts` if the editor consumes it,
  `docs/PROTOCOL.md`.
