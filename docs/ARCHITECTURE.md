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

---

## Frontend Architecture (Web UI)

The editor in `web/` is a React 18 + TypeScript app built with Vite. Its
state lives in Zustand stores under `web/src/store/`; there is no React
context for application state.

| Store | Holds | Notes |
|---|---|---|
| `flowStore` | flow list, node types, the open flow's **working copy**, undo/redo history, autosave state | the only place that edits a flow |
| `editorStore` | selection, open config tray, sidebar tab, dialogs | UI-only, never persisted |
| `runtimeStore` | node status and the message log pushed by the server | server-owned, read-only for the UI |
| `notificationStore` | toasts | raised from stores and components alike |

### One write path

Every edit (add/move/remove node, connect, configure, rename) mutates the
working copy in `flowStore` and is recorded in the undo history. A debounced
`PUT /api/flows/{id}` (600 ms after the last edit, flushed before deploy,
on flow switch and on page unload) saves the working copy as the flow's
**draft definition**. The running instance is only replaced on
`POST /api/flows/{id}/deploy`, which the engine serves from a snapshot of
the definition. Consequently:

- nothing is ever lost to a reload: the draft is what the server holds;
- the Deploy button lights up when the draft differs from what is running
  (`updatedAt > deployedAt`, or a local edit since the last deploy);
- the WebSocket carries no mutations. It is an event and query channel:
  `flow:status`, `flow:list`, `flow:delete`, `node:status` are pushed after
  every REST mutation (the REST handlers notify the hub), `message:log` and
  `message:send` are the runtime query/action, `flow:get`/`state:sync` are
  read-only queries. Exactly one connection per browser tab (`lib/wsClient.ts`).

### Canvas

`@xyflow/react` (v12) renders the working copy. Node positions are written
back to the store once per drag; store updates that arrive mid-drag keep the
dragged node's on-screen position and the current selection.

### Routing and language

`/flow/:id` is the URL of an open flow; a reload restores it (the server
falls back to `index.html` for client routes). UI strings come from
`web/src/i18n/` (English default, German), selectable in the main menu.
