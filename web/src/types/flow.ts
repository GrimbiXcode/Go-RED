// Flow and Node Connection types for Go-RED

export interface NodeConnection {
  id: string;
  sourceNode: string;
  sourcePort: string;
  targetNode: string;
  targetPort: string;
}

export interface Flow {
  id: string;
  name: string;
  description?: string;
  nodes: Record<string, FlowNode>;
  connections: NodeConnection[];
  status: FlowStatus;
  config?: FlowConfig;
  createdAt: string;
  updatedAt: string;
}

export interface FlowNode {
  id: string;
  type: string;
  name?: string;
  position: { x: number; y: number };
  config: Record<string, any>;
  status?: NodeStatus;
  disabled?: boolean;
}

export type FlowStatus = 'draft' | 'deployed' | 'running' | 'stopped' | 'error' | 'paused';

export interface FlowConfig {
  autoDeploy?: boolean;
  timeout?: number;
  maxMessages?: number;
  environment?: Record<string, string>;
}

export interface NodeStatus {
  state: 'idle' | 'processing' | 'error' | 'completed';
  message?: string;
  timestamp?: string;
  processingCount?: number;
  errorCount?: number;
}

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