# Go—RED Implementation Plan

## Overview

This document outlines the implementation roadmap for Go—RED.

---

## Phases

### Phase 1: Core Infrastructure (4-6 weeks)
- Flow Engine with goroutines and channels
- Node Registry with interface-based design
- Basic nodes (Inject, Debug, Function)
- State Manager (JSON file storage)
- CLI for flow management

**Milestone:** Flow with Inject → Function → Debug executable

### Phase 2: Plugin System (3-4 weeks)
- Go Plugin Loader
- JavaScript Sandbox (goja)
- Plugin Registry
- HTTP, WebSocket, Timer nodes

**Milestone:** Custom nodes via Go plugins or JS scripts

### Phase 3: WebUI (4-6 weeks)
- React Flow Editor
- Node Palette
- Node Configuration forms
- Real-time updates via WebSocket

**Milestone:** Fully functional WebUI

### Phase 4: REST API (2-3 weeks)
- Complete REST API
- WebUI integration
- Error handling
- Authentication (optional)

**Milestone:** Full backend-frontend integration

### Phase 5: Production (3-4 weeks)
- Performance optimization
- Docker container
- CI/CD pipeline
- Security audit

**Milestone:** Production-ready release

---

## Current Status

✅ Phase 1: Core Infrastructure - COMPLETED
✅ Phase 2: Plugin System - PARTIAL (Function node with JS)
⏳ Phase 3: WebUI - IN PROGRESS
⏳ Phase 4: REST API - IN PROGRESS
⏳ Phase 5: Production - PENDING

---

## Node Palette

All six planned phases of the Node-RED core palette are now implemented (47 node types:
`inject`, `debug`, `function`, `junction`, `comment`, `catch`, `status`, `complete`,
`link in`, `link out`, `switch`, `change`, `range`, `template`, `delay`, `trigger`,
`rbe`, `exec`, `csv`, `json`, `xml`, `yaml`, `html`, `split`, `join`, `sort`, `batch`,
`file`, `file in`, `watch`, `tls-config`, `http proxy`, `mqtt-broker`, `mqtt in`,
`mqtt out`, `http in`, `http response`, `http request`, `websocket-listener`,
`websocket-client`, `websocket in`, `websocket out`, `tcp in`, `tcp out`,
`tcp request`, `udp in`, `udp out`). `link call`, `global-config` (as its own NR type
ID), and `unknown` remain deliberately out of scope - see the plan doc's Phase 1
section for why. `exec` only runs if `GORED_ENABLE_EXEC` is set on the server; `http
request` does not block SSRF by default (parity with Node-RED - see
`internal/nodes/httprequest`'s package doc for the full security rationale). See
[Node Palette Plan](NODE_PALETTE_PLAN.md) for the full inventory, engine changes
(including Phase 6's config-node concept), and per-phase scope cuts.
