// Flow and Node Connection types for Go-RED
//
// The wire-format types below are generated from the Go backend (see
// internal/dto) via `go generate ./internal/dto/...` — do not hand-describe
// their shape here. Only UI-only types that never cross the wire are
// hand-written in this file.
export type {
  Connection as NodeConnection,
  Flow,
  Node as FlowNode,
  FlowStatus,
  FlowConfig,
  NodeStatus,
} from './generated';

import type { Flow, FlowStatus } from './generated';

export interface FlowMetadata {
  id: string;
  name: string;
  description?: string;
  nodeCount: number;
  connectionCount: number;
  status: FlowStatus;
  lastDeployed?: string;
  lastError?: string;
}

export interface FlowState {
  flows: Flow[];
  activeFlows: string[];
  lastUpdated: string;
}

// Node registry type for available node types
// Using any to avoid circular dependency with node.ts
export interface NodeRegistry {
  [nodeType: string]: any;
}
