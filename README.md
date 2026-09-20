# Go—RED

> **Node-RED-inspired Flow Editor in Go** - Complete utilization of Go features (Goroutines, Channels, Interfaces, Generics, etc.)

---

## Overview

Go—RED is a **flow-based programming editor** similar to Node-RED, but completely implemented in **Go**. The project leverages Go's strengths to create a **high-performance, scalable, and extensible** platform for data flows.

### Features

- Flow-based programming - Drag & Drop nodes, connections between nodes
- Real-time WebUI - Live updates via WebSocket
- Plugin System - Custom nodes in Go or JavaScript
- Measured performance - see `docs/PERFORMANCE.md` for the benchmark
  numbers (`go test -bench` in `internal/engine`); the engine runs each flow
  with its own queue and a bounded set of goroutines, and every node honors
  context cancellation
- Node-RED import and export - `flows.json` in, `flows.json` out
- Operations - optional bearer token, origin policy, rate limits, backups,
  `/api/version`, Prometheus `/metrics`, configuration by flags, `GORED_*`
  environment or a YAML file
- Extensible - custom nodes in Go (see `docs/NODE_DEVELOPMENT.md`)

### Target Platforms

- Cloud - Docker, Kubernetes, Serverless
- Desktop - Linux, macOS, Windows

---

## Documentation

| Document | Description |
|----------|-------------|
| [Architecture](docs/ARCHITECTURE.md) | Technical architecture, components, data flow |
| [Implementation Plan](docs/IMPLEMENTATION_PLAN.md) | Detailed roadmap with phases and timeline |
| [Node Development](docs/NODE_DEVELOPMENT.md) | Guide for developing custom nodes |
| [Node Palette Plan](docs/NODE_PALETTE_PLAN.md) | Plan for full Node-RED core node palette parity |
| [Next Level Plan](docs/NEXT_LEVEL_PLAN.md) | Full project audit (verified bugs, architecture, visual review) and the phased plan to fix it |

---

## Quick Start

### Prerequisites

- Go 1.25+
- Node.js 18+ (for WebUI)

### Installation

1. Clone repository:
```bash
git clone https://github.com/GrimbiXcode/Go-RED.git
cd Go-RED
```

2. Install Go dependencies:
```bash
go mod download
```

3. Start Go—RED:
```bash
go run cmd/go-red/main.go
```

Application will be available at http://localhost:8080

### Configuration

Every setting has a flag, a `GORED_*` environment variable and a key in an
optional YAML file (`-config go-red.yaml` or `GORED_CONFIG`); flags win over
the environment, which wins over the file.

| Flag | Env | Default | Meaning |
|---|---|---|---|
| `-port` | `GORED_PORT` | `8080` | HTTP port |
| `-data-dir` | `GORED_DATA_DIR` | `data` | flows, backups, quarantine |
| `-web-dir` | `GORED_WEB_DIR` | `web/dist` | built editor |
| `-auth-token` | `GORED_AUTH_TOKEN` | – | bearer token for the API, the WebSocket and `/metrics` (≥ 16 chars of `A-Z a-z 0-9 . _ ~ -`) |
| `-allowed-origins` | `GORED_ALLOWED_ORIGINS` | – | extra browser origins (`scheme://host[:port]`, comma-separated, or `*`); the editor's own origin is always allowed |
| `-rate-limit` | `GORED_RATE_LIMIT` | `60` | import/deploy requests per client and minute (`0` disables) |
| `-max-inflight` | `GORED_MAX_INFLIGHT` | `1024` | concurrent node executions per flow (a flow's `maxConcurrency` overrides it) |
| `-max-messages` | `GORED_MAX_MESSAGES` | `1000` | message queue size per flow |
| `-message-log` | `GORED_MESSAGE_LOG` | `0` | routed messages kept for `GET /api/messages` |
| `-backup-keep` / `-backup-interval` | `GORED_BACKUP_KEEP` / `GORED_BACKUP_INTERVAL` | `5` / `10m` | backups per flow file and the minimum time between two |
| `-log-level` | `GORED_LOG_LEVEL` | `info` | `debug`, `info`, `warn`, `error` |

```yaml
# go-red.yaml
port: 8080
authToken: change-me-to-something-long
allowedOrigins: [https://dashboard.example]
rateLimit: 30
```

The server refuses cross-site browser requests unless their origin is
allowed, and with a token set the editor asks for it once and keeps it in
the browser. `GET /api/health` and `GET /api/version` are always public;
`GET /metrics` serves Prometheus metrics. See `docs/PROTOCOL.md`.

---

## Project Structure

```
Go-RED/
├── cmd/
│   └── go-red/
│       └── main.go              # Main application
├── internal/
│   ├── engine/                  # Flow Engine
│   │   ├── flow.go              # Flow management
│   │   ├── engine.go            # Flow execution engine
│   │   └── message.go           # Message structure
│   ├── nodes/                   # Built-in Nodes
│   │   ├── debug/
│   │   │   └── node.go
│   │   ├── function/
│   │   │   └── node.go
│   │   └── inject/
│   │       └── node.go
│   ├── registry/                # Node Registry
│   │   └── registry.go
│   └── state/                   # State Manager
│       └── manager.go
├── docs/                       # Documentation
├── web/                        # WebUI
│   └── index.html
├── go.mod
├── Makefile
├── Dockerfile
└── README.md
```

---

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for details.

---

## License

[MIT License](LICENSE)
