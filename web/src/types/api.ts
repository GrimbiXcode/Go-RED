// API response types for Go-RED REST API
import type { Flow, FlowNode, NodeConnection, FlowConfig, FlowStatus } from './flow';
import type { NodeMetadata } from './node';
import type { MessageLogEntry, WebSocketMessageType } from './message';

export type { Flow, FlowNode, NodeConnection, FlowConfig, FlowStatus, NodeMetadata, MessageLogEntry, WebSocketMessageType };

// FlowSummary, FlowCreateRequest, and FlowUpdateRequest are generated from
// the Go backend (see internal/dto) via `go generate ./internal/dto/...` —
// do not hand-describe their shape here.
export type { FlowSummary, FlowCreateRequest, FlowUpdateRequest } from './generated';

export interface PaginatedResponse<T> {
  data: T[];
  pagination: Pagination;
}

export interface Pagination {
  page: number;
  pageSize: number;
  total: number;
  totalPages: number;
  hasNext: boolean;
  hasPrevious: boolean;
}

export interface NodeDetailResponse {
  metadata: NodeMetadata;
}

export interface DeployResponse {
  flowId: string;
  status: string;
  message?: string;
  errors?: string[];
}

export interface MessageLogResponse {
  messages: MessageLogEntry[];
}

export interface HealthCheckResponse {
  status: 'healthy' | 'degraded' | 'unhealthy';
  version: string;
  uptime: number;
  checks: Record<string, HealthCheck>;
}

export interface HealthCheck {
  status: 'up' | 'down';
  message?: string;
  lastChecked: string;
}

export interface StatsResponse {
  totalFlows: number;
  activeFlows: number;
  totalNodes: number;
  messagesProcessed: number;
  messagesPerSecond: number;
  averageProcessingTime: number;
}

export interface DeployRequest {
  flowId: string;
  force?: boolean;
}

export interface UndeployRequest {
  flowId: string;
}

export interface MessageLogRequest {
  flowId?: string;
  limit?: number;
  offset?: number;
}

export interface FlowExportRequest {
  flowId: string;
}

export interface FlowImportRequest {
  flow: Flow;
}
