# Go—RED

> **Node-RED-inspired Flow Editor in Go** - Complete utilization of Go features (Goroutines, Channels, Interfaces, Generics, etc.)

---

## Overview

Go—RED is a **flow-based programming editor** similar to Node-RED, but completely implemented in **Go**. The project leverages Go's strengths to create a **high-performance, scalable, and extensible** platform for data flows.

### Features

- Flow-based programming - Drag & Drop nodes, connections between nodes
- Real-time WebUI - Live updates via WebSocket
- Plugin System - Custom nodes in Go or JavaScript
- High Performance - Optimized for > 100,000 messages/second
- Go-specific implementation:
  - Goroutines & Channels for parallel processing
  - Interfaces for plugin architecture
  - Generics for type-safe nodes
  - Context for timeouts/cancellation
  - Reflection for dynamic node registration
- Extensible - Community plugins, custom nodes
- Scalable - Worker pools, message batching, caching

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

---

## Quick Start

### Prerequisites

- Go 1.21+
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
