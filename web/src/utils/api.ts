// API client functions for Go-RED
import type {
  ApiResponse,
  Flow,
  FlowCreateRequest,
  FlowUpdateRequest,
  FlowListResponse,
  FlowDetailResponse,
  FlowSummary,
  NodeListResponse,
  NodeDetailResponse,
  NodeMetadata,
  PluginListResponse,
  PluginInfo,
  PluginLoadRequest,
  DeployRequest,
  DeployResponse,
  UndeployRequest,
  UndeployResponse,
  MessageLogRequest,
  MessageLogResponse,
  HealthCheckResponse,
  StatsResponse,
  FlowExportRequest,
  FlowImportRequest,
} from '../types/api';

const API_BASE_URL = import.meta.env.VITE_API_BASE_URL || '/api';

async function apiRequest<T, U = undefined>(
  method: 'GET' | 'POST' | 'PUT' | 'DELETE' | 'PATCH',
  endpoint: string,
  data?: U
): Promise<ApiResponse<T>> {
  const url = `${API_BASE_URL}${endpoint}`;
  const options: RequestInit = {
    method,
    headers: {
      'Content-Type': 'application/json',
    },
  };
  if (data) {
    options.body = JSON.stringify(data);
  }
  try {
    const response = await fetch(url, options);
    if (!response.ok) {
      const errorData = await response.json().catch(() => ({}));
      throw new Error(
        `API request failed with status ${response.status}: ${errorData.message || response.statusText}`
      );
    }
    return await response.json();
  } catch (error) {
    console.error(`API request error: ${error}`);
    throw error;
  }
}

export const fetchFlows = async (): Promise<FlowSummary[]> => {
  const response = await apiRequest<FlowListResponse>('GET', '/flows');
  return response.data?.flows || [];
};

export const fetchFlow = async (flowId: string): Promise<Flow> => {
  const response = await apiRequest<FlowDetailResponse>('GET', `/flows/${flowId}`);
  return response.data?.flow;
};

export const createFlow = async (flowData: FlowCreateRequest): Promise<Flow> => {
  const response = await apiRequest<FlowDetailResponse, FlowCreateRequest>(
    'POST',
    '/flows',
    flowData
  );
  return response.data?.flow;
};

export const updateFlow = async (
  flowId: string,
  flowData: FlowUpdateRequest
): Promise<Flow> => {
  const response = await apiRequest<FlowDetailResponse, FlowUpdateRequest>(
    'PUT',
    `/flows/${flowId}`,
    flowData
  );
  return response.data?.flow;
};

export const deleteFlow = async (flowId: string): Promise<void> => {
  await apiRequest<void>('DELETE', `/flows/${flowId}`);
};

export const deployFlow = async (flowId: string, force = false): Promise<DeployResponse> => {
  const response = await apiRequest<DeployResponse, DeployRequest>(
    'POST',
    `/flows/${flowId}/deploy`,
    { flowId, force }
  );
  return response.data;
};

export const undeployFlow = async (flowId: string): Promise<DeployResponse> => {
  const response = await apiRequest<DeployResponse, UndeployRequest>(
    'POST',
    `/flows/${flowId}/undeploy`,
    { flowId }
  );
  return response.data;
};

export const getNodes = async (): Promise<NodeMetadata[]> => {
  const response = await apiRequest<NodeListResponse>('GET', '/nodes');
  return response.data?.nodeTypes || [];
};

export const getNode = async (nodeType: string): Promise<NodeMetadata> => {
  const response = await apiRequest<NodeDetailResponse>('GET', `/nodes/${nodeType}`);
  return response.data?.metadata;
};

export const getPlugins = async (): Promise<PluginInfo[]> => {
  const response = await apiRequest<PluginListResponse>('GET', '/plugins');
  return response.data?.plugins || [];
};

export const getWebSocketUrl = (): string => {
  const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
  const host = window.location.host;
  return `${protocol}//${host}/ws`;
};

export const sleep = (ms: number): Promise<void> => {
  return new Promise((resolve) => setTimeout(resolve, ms));
};

export const generateId = (): string => {
  return Math.random().toString(36).substring(2, 9) + Date.now().toString(36);
};