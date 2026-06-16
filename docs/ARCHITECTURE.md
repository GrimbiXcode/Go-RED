# Go—RED Architecture

## Overview

This document describes the technical architecture of Go—RED.

---

## System Architecture

### Layered Architecture

Go—RED follows a layered architecture with clear separation of concerns:

```
Presentation Layer (React WebUI)
│
Application Layer (Flow Engine, Node Registry, Plugin Loader)
│
Infrastructure Layer (Message Bus, Worker Pool, State Manager)
```

### Component Diagram

```
Client (Browser) → WebSocket/HTTP → Go—RED Server → File System/Database
```

### Core Components

1. **Flow Engine** - Orchestrates flow execution and message routing
2. **Node Registry** - Manages all available node types
3. **Plugin System** - Enables custom nodes via plugins
4. **Message Bus** - Handles communication between nodes
5. **State Manager** - Persists flows and configurations

---

## Data Flow

Messages flow through the system as follows:

1. Message injected into flow (via Inject node or external trigger)
2. Flow Engine routes message to connected nodes
3. Each node processes message in its own goroutine
4. Output messages are routed to next nodes
5. Process continues until message reaches end of flow

---

## Design Decisions

### Why Go?

- Goroutines for lightweight concurrency
- Channels for safe communication
- Interfaces for flexible plugin system
- Compilation for fast execution
- Standard library support for HTTP/WebSocket

### Why React?

- Component-based architecture
- Rich ecosystem (React Flow for flow editor)
- TypeScript for type safety
- Large developer community

---

## Performance Considerations

- Worker pools limit concurrent goroutines
- Message batching reduces overhead
- Node caching improves performance
- Object pooling reduces GC pressure
