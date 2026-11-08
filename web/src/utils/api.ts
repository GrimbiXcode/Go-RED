// API client functions for Go-RED
import type {
  Flow,
  FlowCreateRequest,
  FlowUpdateRequest,
  FlowSummary,
  NodeMetadata,
  DeployResponse,
  AuthUser,
} from '../types/api';

import { authHeaders, authRequired } from '../lib/auth';
import type { LogoutResponse } from '../types/generated';

const API_BASE_URL = (import.meta as any).env?.VITE_API_BASE_URL || '/api';

export interface RequestOptions {
  /** Let the request outlive the page (used to flush a save on unload). */
  keepalive?: boolean;
}

async function apiRequest<T, U = undefined>(
  method: 'GET' | 'POST' | 'PUT' | 'DELETE' | 'PATCH',
  endpoint: string,
  data?: U,
  requestOptions: RequestOptions = {}
): Promise<T> {
  const url = `${API_BASE_URL}${endpoint}`;
  const options: RequestInit = {
    method,
    headers: {
      'Content-Type': 'application/json',
      ...authHeaders(),
    },
    keepalive: requestOptions.keepalive,
  };
  if (data) {
    options.body = JSON.stringify(data);
  }
  const response = await fetch(url, options);
  if (response.status === 401) {
    authRequired();
  }
  if (!response.ok) {
    const errorData = await response.json().catch(() => ({}));
    const detail = errorData.error || errorData.message || response.statusText;
    throw new Error(detail ? String(detail) : `Request failed with status ${response.status}`);
  }
  if (response.status === 204) {
    return undefined as T;
  }
  return await response.json();
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
  flowData: FlowUpdateRequest,
  options: RequestOptions = {}
): Promise<Flow> => {
  const response = await apiRequest<Flow, FlowUpdateRequest>(
    'PUT',
    `/flows/${flowId}`,
    flowData,
    options
  );
  if (!response) {
    throw new Error('Flow not found in response');
  }
  return response;
};

export const deleteFlow = async (flowId: string): Promise<void> => {
  await apiRequest<void>('DELETE', `/flows/${flowId}`);
};

export const deployFlow = async (flowId: string): Promise<DeployResponse> => {
  const response = await apiRequest<DeployResponse>('POST', `/flows/${flowId}/deploy`);
  if (!response) {
    throw new Error('No data in response');
  }
  return response;
};

export const undeployFlow = async (flowId: string): Promise<DeployResponse> => {
  const response = await apiRequest<DeployResponse>('POST', `/flows/${flowId}/undeploy`);
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

export type ExportFormat = 'go-red' | 'node-red';

// Export a flow by downloading it as a JSON file, in Go-RED's own format or as a Node-RED flows.json.
export const exportFlow = async (flowId: string, format: ExportFormat = 'go-red'): Promise<void> => {
  const query = format === 'node-red' ? '?format=node-red' : '';
  const response = await apiRequest<unknown>('GET', `/flows/${flowId}/export${query}`);
  if (!response) {
    throw new Error('No data in response');
  }

  downloadJson(response, format === 'node-red' ? `flow-${flowId}.node-red.json` : `flow-${flowId}.json`);
};

/** Saves data as a pretty-printed JSON file through the browser's download. */
export function downloadJson(data: unknown, filename: string): void {
  const dataBlob = new Blob([JSON.stringify(data, null, 2)], { type: 'application/json' });
  const url = URL.createObjectURL(dataBlob);
  const link = document.createElement('a');
  link.href = url;
  link.download = filename;
  document.body.appendChild(link);
  link.click();
  document.body.removeChild(link);
  URL.revokeObjectURL(url);
}

export interface ImportedFlow {
  flowId: string;
  originalId: string;
  name: string;
}

export interface ImportResult {
  format: 'go-red' | 'node-red';
  flowId: string;
  name: string;
  flows: ImportedFlow[];
  warnings: string[];
  message: string;
}

// Import a flow file: a Go-RED flow object or a Node-RED export array (one flow per tab).
export const importFlow = async (file: File): Promise<ImportResult> => {
  const content = await file.text();
  const data: unknown = JSON.parse(content);

  const response = await apiRequest<ImportResult, unknown>('POST', '/flows/import', data);
  if (!response) {
    throw new Error('No data in response');
  }
  return {
    format: response.format || 'go-red',
    flowId: response.flowId,
    name: response.name,
    flows: response.flows || [{ flowId: response.flowId, originalId: '', name: response.name }],
    warnings: response.warnings || [],
    message: response.message,
  };
};

/** Who the editor is authenticated as (method "none" without authentication). */
export const fetchAuthUser = (): Promise<AuthUser> => apiRequest<AuthUser>('GET', '/auth/me');

/** Ends the OIDC session; logoutUrl, when set, ends the provider's session too. */
export const signOut = (): Promise<LogoutResponse> => apiRequest<LogoutResponse>('POST', '/auth/logout');
