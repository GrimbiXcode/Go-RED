import { wsClient } from '../lib/wsClient';
import { useFlowStore } from './flowStore';
import { useRuntimeStore } from './runtimeStore';
import { notify } from './notificationStore';

/**
 * Routes server-originated WebSocket events into the stores and opens the
 * connection. Call once when the app mounts; the returned function detaches
 * the listeners again.
 */
export function bindServerEvents(): () => void {
  const unsubscribers = [
    wsClient.subscribe('flow:status', (data) => {
      if (data && typeof data.flowId === 'string' && typeof data.status === 'string') {
        useFlowStore.getState().applyFlowStatus({
          flowId: data.flowId,
          status: data.status,
          updatedAt: data.updatedAt,
          deployedAt: data.deployedAt,
        });
      }
    }),
    wsClient.subscribe('flow:list', (data) => {
      if (data && Array.isArray(data.flows)) {
        useFlowStore.getState().applyFlowList(data.flows);
      }
    }),
    wsClient.subscribe('flow:delete', (data) => {
      if (data && typeof data.flowId === 'string') {
        useFlowStore.getState().applyFlowDeleted(data.flowId);
        useRuntimeStore.getState().forgetFlow(data.flowId);
      }
    }),
    wsClient.subscribe('node:status', (data) => {
      if (data && typeof data.flowId === 'string' && typeof data.nodeId === 'string' && data.status) {
        useRuntimeStore.getState().applyNodeStatus(data.flowId, data.nodeId, data.status);
      }
    }),
    wsClient.subscribe('message:log', (data) => {
      if (data && Array.isArray(data.messages)) {
        useRuntimeStore.getState().applyMessageLog(data.flowId, data.messages);
      }
    }),
    wsClient.subscribe('error', (data) => {
      const message = (data && (data.message || data.error)) || 'Server error';
      notify('error', String(message));
    }),
  ];

  wsClient.connect();

  return () => {
    for (const unsubscribe of unsubscribers) unsubscribe();
  };
}
