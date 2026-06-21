# Go-RED Project Guidelines

## Overview
This is **Go-RED**, a Node-RED-inspired flow editor built in Go with a React-based WebUI. This file contains project-wide guidelines that apply to all agents working on this codebase.

---

## Project Structure

```
Go-RED/
├── cmd/
│   └── go-red/                    # Main application entry point
│       ├── main.go               # HTTP server, WebSocket hub
│       └── websocket/            # WebSocket communication layer
├── internal/
│   ├── engine/                   # Flow execution engine (core)
│   │   ├── engine.go             # FlowEngine, ActiveFlow management
│   │   ├── flow.go               # Flow structure and validation
│   │   └── message.go            # Message passing infrastructure
│   ├── nodes/                    # Built-in node implementations
│   │   ├── debug/                # Debug output node
│   │   ├── function/             # JavaScript function node
│   │   └── inject/               # Message injection node
│   ├── registry/                 # Node type registration system
│   └── state/                    # Flow persistence (file system)
├── data/flows/                   # Persisted flow data
├── web/                         # React WebUI
│   ├── src/                      # TypeScript source
│   │   ├── components/           # React components
│   │   ├── hooks/                # Custom React hooks
│   │   ├── types/                # TypeScript type definitions
│   │   ├── utils/                # Utility functions
│   │   └── test/                 # Frontend tests (Vitest)
│   └── dist/                     # Built frontend assets
├── docs/                        # Documentation
└── go.mod                       # Go module definition
```

---

## General Development Guidelines

### Code Style
- **Go**: Follow [Effective Go](https://go.dev/doc/effective_go) and use `gofmt`/`goimports`
- **TypeScript**: Use Prettier + ESLint configuration from `web/package.json`
- **Naming**: Use descriptive names. Avoid abbreviations unless widely understood (e.g., `config` not `cfg`)
- **Comments**: Write comments for public APIs, complex algorithms, and non-obvious decisions

### Git Workflow
- Use **feature branches** for new features: `feature/[name]`
- Use **bugfix branches** for fixes: `bugfix/[description]`
- Use **conventional commits** format
- Always include tests with new functionality
- Run `go test ./...` and `npm test` (in web/) before committing

### Testing
- **Backend**: Use Go's `testing` package with `testify` for assertions
- **Frontend**: Use Vitest with `@testing-library/react`
- **Coverage**: Aim for >80% test coverage for core functionality
- **Integration**: Test WebSocket communication between frontend and backend

### Performance Considerations
- Backend must handle >100,000 messages/second
- Use goroutines and channels for concurrent processing
- Avoid blocking operations in hot paths
- Frontend should remain responsive with many nodes/flows

---

## Architecture Principles

### Backend (Go)
1. **Flow Engine** is the core - manages flow execution, message routing
2. **Node Registry** enables plugin architecture - nodes can be added without modifying core
3. **State Manager** abstracts persistence - supports file system, database backends
4. **WebSocket Hub** provides real-time bidirectional communication

### Frontend (React + TypeScript)
1. **FlowProvider** manages global flow state using Zustand
2. **ReactFlow** library for flow visualization and editing
3. **Custom Hooks** encapsulate WebSocket and flow management logic
4. **TypeScript** for type safety throughout the application

### Communication
- **REST API** (`/api/*`) for CRUD operations on flows, nodes
- **WebSocket** (`/ws`) for real-time updates and message streaming
- **Message Format**: Follow existing patterns in `internal/engine/message.go`

---

## Security
- Sanitize all user input in API handlers
- Validate flow definitions before deployment
- Use DOM sanitization (DOMPurify) for any HTML rendering
- WebSocket connections should be authenticated in production
- Never expose internal error details to clients

---

## Documentation Standards
- All public functions should have godoc comments
- TypeScript interfaces should have JSDoc comments
- Update `docs/ARCHITECTURE.md` when making architectural changes
- Keep README.md up to date with setup and usage instructions

---

## Build & Deployment
- **Development**: `go run cmd/go-red/main.go` + `npm run dev` (in web/)
- **Production**: Build frontend (`npm run build`), then `go build`
- **Docker**: Use provided Dockerfile for containerization
- **Port**: Default is 8080, configurable via `-port` flag

---

## Agent Instructions

When working in this repository:

1. **Always read existing code** in the target directory before making changes
2. **Follow existing patterns** - this project uses specific conventions for:
   - Error handling (wrapping with context)
   - Concurrency (goroutines with proper WaitGroup usage)
   - WebSocket message formats
   - React component structure
3. **Write tests** for any new functionality
4. **Update type definitions** when adding new API endpoints or message types
5. **Respect the layer separation**: Backend (Go) and Frontend (TypeScript) should communicate only via defined APIs

---

## Priority Rules
- **Performance bugs** in the flow engine take highest priority
- **Security issues** must be addressed immediately
- **Breaking API changes** require migration guides in docs/
- **Frontend/Backend sync** - ensure API contracts are consistent

---

## Contact & Resources
- **Primary Contact**: Project maintainer (check GitHub)
- **Documentation**: See `docs/` directory
- **Go Version**: 1.25+
- **Node Version**: 18+

---

*Last updated: 2026-06-21*
*This file applies to all directories unless overridden by local AGENTS.md files*
