// API client functions for Go-RED
import type {
  Flow,
  FlowCreateRequest,
  FlowUpdateRequest,
  FlowSummary,
  NodeMetadata,
  PluginInfo,
  DeployRequest,
  DeployResponse,
  UndeployRequest,
} from '../types/api';

const API_BASE_URL = (import.meta as any).env?.VITE_API_BASE_URL || '/api';

async function apiRequest<T, U = undefined>(
  method: 'GET' | 'POST' | 'PUT' | 'DELETE' | 'PATCH',
  endpoint: string,
  data?: U
): Promise<T> {
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
  const response = await apiRequest<FlowSummary[]>('GET', '/flows');
  return response;
};

export const fetchFlow = async (flowId: string): Promise<Flow> => {
  const response = await apiRequest<Flow>('GET', `/flows/${flowId}`);
  if (!response) {
    throw new Error('Flow not found in response');
  }
  return response;
};

export const createFlow = async (flowData: FlowCreateRequest): Promise<Flow> => {
  const response = await apiRequest<Flow, FlowCreateRequest>(
    'POST',
    '/flows',
    flowData
  );
  if (!response) {
    throw new Error('Flow not found in response');
  }
  return response;
};

export const updateFlow = async (
  flowId: string,
  flowData: FlowUpdateRequest
): Promise<Flow> => {
  const response = await apiRequest<Flow, FlowUpdateRequest>(
    'PUT',
    `/flows/${flowId}`,
    flowData
  );
  if (!response) {
    throw new Error('Flow not found in response');
  }
  return response;
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
  if (!response) {
    throw new Error('No data in response');
  }
  return response;
};

export const undeployFlow = async (flowId: string): Promise<DeployResponse> => {
  const response = await apiRequest<DeployResponse, UndeployRequest>(
    'POST',
    `/flows/${flowId}/undeploy`,
    { flowId }
  );
  if (!response) {
    throw new Error('No data in response');
  }
  return response;
};

export const getNodes = async (): Promise<NodeMetadata[]> => {
  const response = await apiRequest<NodeMetadata[]>('GET', '/nodes');
  return response || [];
};

export const getNode = async (nodeType: string): Promise<NodeMetadata> => {
  const response = await apiRequest<NodeMetadata>('GET', `/nodes/${nodeType}`);
  if (!response) {
    throw new Error('Node metadata not found in response');
  }
  return response;
};

export const getPlugins = async (): Promise<PluginInfo[]> => {
  const response = await apiRequest<PluginInfo[]>('GET', '/plugins');
  return response || [];
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

// Export a flow by downloading it as a JSON file
export const exportFlow = async (flowId: string): Promise<void> => {
  const response = await apiRequest<Record<string, any>>('GET', `/flows/${flowId}/export`);
  if (!response) {
    throw new Error('No data in response');
  }
  
  // Create download link
  const dataStr = JSON.stringify(response, null, 2);
  const dataBlob = new Blob([dataStr], { type: 'application/json' });
  const url = URL.createObjectURL(dataBlob);
  const link = document.createElement('a');
  link.href = url;
  link.download = `flow-${flowId}.json`;
  document.body.appendChild(link);
  link.click();
  document.body.removeChild(link);
  URL.revokeObjectURL(url);
};

// Import a flow from a JSON file
export const importFlow = async (file: File): Promise<{ flowId: string; name: string; message: string }> => {
  // Read file content
  const content = await file.text();
  const flowData = JSON.parse(content);
  
  const response = await apiRequest<{
    status: string;
    flowId: string;
    originalId: string;
    name: string;
    message: string;
  }, typeof flowData>('POST', '/flows/import', flowData);
  
  if (!response) {
    throw new Error('No data in response');
  }
  
  return {
    flowId: response.flowId,
    name: response.name,
    message: response.message,
  };
};