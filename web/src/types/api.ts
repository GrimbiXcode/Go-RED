// API response types for Go-RED REST API

export interface ApiResponse<T> {
  success: boolean;
  data?: T;
  error?: ApiError;
  timestamp: string;
}

export interface ApiError {
  code: string;
  message: string;
  details?: Record<string, any>;
  stack?: string;
}

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

export interface FlowListResponse {
  flows: FlowSummary[];
}

export interface FlowSummary {
  id: string;
  name: string;
  status: string;
  nodeCount: number;
  lastDeployed?: string;
  lastError?: string;
}

export interface FlowDetailResponse {
  flow: Flow;
}

export interface FlowCreateRequest {
  name: string;
  description?: string;
  nodes?: Record<string, FlowNode>;
  connections?: NodeConnection[];
  config?: FlowConfig;
}

export interface FlowUpdateRequest {
  name?: string;
  description?: string;
  nodes?: Record<string, FlowNode>;
  connections?: NodeConnection[];
  config?: FlowConfig;
}

export interface NodeListResponse {
  nodeTypes: NodeMetadata[];
}

export interface NodeDetailResponse {
  metadata: NodeMetadata;
}

export interface PluginInfo {
  id: string;
  name: string;
  version: string;
  description?: string;
  author?: string;
  nodeTypes: string[];
  path: string;
  loaded: boolean;
  error?: string;
}

export interface PluginListResponse {
  plugins: PluginInfo[];
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

type Flow = import('./flow').Flow;
type FlowNode = import('./flow').FlowNode;
type NodeConnection = import('./flow').NodeConnection;
type FlowConfig = import('./flow').FlowConfig;
type FlowStatus = import('./flow').FlowStatus;
type NodeMetadata = import('./node').NodeMetadata;
type MessageLogEntry = import('./message').MessageLogEntry;