# Go-RED Project Guidelines

Go-RED is a Node-RED-style flow editor and runtime written in Go, with a React
editor that is built into the binary. This file applies to the whole
repository; the `AGENTS.md` in a directory adds what is specific to it.
Everything here describes code that exists. When a statement here and the
code disagree, the code wins, and the file gets fixed.

## Layout

```
cmd/go-red/            server: main.go (wiring), config.go (flags/env/YAML),
                       auth.go (token, origin policy, rate limit), ops.go
                       (/api/version, /metrics), websocket/ (hub, handlers)
cmd/gentypes/          go generate: Go DTOs -> web/src/types/generated.ts
internal/engine/       flows, per-flow dispatch, runtime events, benchmarks
internal/registry/     node types, schema v2, NodeRuntime, context stores
internal/nodes/*/      47 built-in nodes, one package each; base/ = shared helpers
internal/nodered/      Node-RED flows.json import and export
internal/state/        flow files with schemaVersion, backups, quarantine
internal/dto/          wire types for REST and WebSocket (source of generated.ts)
internal/typedvalue/   msg/flow/global/str/num/json typed values and property refs
internal/webui/        embed of the built editor (Vite writes to webui/dist)
web/                   React 18 + TypeScript editor (own Go module so `./...` skips it)
docs/                  PROTOCOL, ARCHITECTURE, NODE_DEVELOPMENT, PERFORMANCE, plans
```

## Commands

| Task | Command |
|---|---|
| Everything CI checks | `make check` (gofmt, vet, race tests, generated types, web typecheck/lint/tests/build) |
| Go tests | `go test -race ./...` |
| Benchmarks | `go test ./internal/engine -run '^$' -bench . -benchtime 20000x` (numbers in `docs/PERFORMANCE.md`) |
| Editor checks | `cd web && npm run typecheck && npm run lint && npx vitest run` |
| End-to-end | `cd web && npm run build && npx playwright test` (real server on port 8081) |
| Regenerate types | `make generate-types` after changing `internal/dto`, `internal/registry` metadata or WebSocket message types; commit the result |
| Run it | `make start` (checks Go and Node, installs, builds, runs on :8080; `PORT=` and `DATA_DIR=` override) |
| Run for development | `make dev` (both servers, Ctrl+C stops them), which is `go run ./cmd/go-red` plus `cd web && npm run dev` (Vite proxies /api and /ws to :8080), or `go run ./cmd/go-red -web-dir internal/webui/dist` after `npm run build` |
| One binary with the editor | `make build-all` (Node 22 builds the editor, Go 1.25 embeds it) |

## Rules that hold everywhere

- **One write path.** The editor changes flows only through REST
  (`PUT /api/flows/{id}` autosaves the draft; deploy runs a snapshot). The
  WebSocket carries server events and read-only queries. `docs/PROTOCOL.md`
  is the contract; change it together with the code and the generated types.
- **Definition vs. instance.** The engine keeps the editable definition and a
  deploy-time snapshot apart; editing never touches what runs until the next
  deploy. Mutate definitions only through engine methods (under its lock).
- **Dispatch.** Every deployed flow has its own queue and dispatcher; node
  executions are bounded per flow and tracked so undeploy waits for them. No
  engine-wide lock or log per message. Nodes honor context cancellation
  (dial with context, deadlines, `ctx.Done()` in waits) and prove it in a
  `ctx_test.go`.
- **Errors.** Engine sentinel errors (`ErrFlowNotFound`, `ErrInvalidFlow`,
  ...) map to status codes in `cmd/go-red`; anything else is logged in full
  and answered generically. Never echo internal details to clients.
- **Input.** Flow IDs match `^[A-Za-z0-9][A-Za-z0-9_-]{0,127}$`, bodies are
  size-capped, user strings are sanitized, HTML rendered in the editor goes
  through DOMPurify.
- **Security defaults.** Cross-site browser requests are refused unless the
  origin is allowed; an optional bearer token guards the API, the WebSocket
  and `/metrics`; import and deploy are rate limited. Keep new endpoints
  behind the same middleware (`newRouter`).
- **Node schemas.** A node's `ConfigSchema` (schema v2: label, widget, order,
  group, items, visibleWhen, outputsFrom) is what the edit tray renders;
  `internal/nodes/schema_test.go` checks every registered node.
- **Persistence.** Flow files carry `schemaVersion`; a shape change needs a
  migration step in `internal/state/migrate.go` and a test.
- **Claims.** Throughput numbers come from the benchmarks, not from prose.

## Conventions

- Go: `gofmt`, `go vet`, godoc on exported identifiers, tests with
  `testify`, mocks and isolated registries in engine tests.
- TypeScript: ESLint (`--max-warnings 0`), Prettier, Vitest with Testing
  Library, Playwright for the real server; UI strings in `web/src/i18n/`
  (English default, German), colors only through the design tokens.
- Commits: conventional commits, one concern per commit, tests with every
  change, docs (`docs/*.md`, the relevant `AGENTS.md`) in the same change.
- Never commit binaries, `web/node_modules`, `internal/webui/dist` contents
  (only its `.gitkeep`) or `data/`.

## Where to read more

- `docs/PROTOCOL.md` - REST, WebSocket, schemas, auth, config, persistence format
- `docs/ARCHITECTURE.md` - components, dispatch, lifecycle, events, editor
- `docs/NODE_DEVELOPMENT.md` - writing a node, `internal/nodes/base`, cancellation
- `docs/PERFORMANCE.md` - benchmark results and how to read them
- `docs/NEXT_LEVEL_PLAN.md` - the phased plan and what each phase changed

Go 1.25+, Node 22 for building the editor (18+ runs the tests).
