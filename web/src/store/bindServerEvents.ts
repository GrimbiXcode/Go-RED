import { wsClient } from '../lib/wsClient';
import { useFlowStore } from './flowStore';
import { useRuntimeStore } from './runtimeStore';
import { notify } from './notificationStore';

/**
 * Routes server-originated WebSocket events into the stores, keeps the
 * runtime subscription in step with the open flow, and opens the
 * connection. Call once when the app mounts; the returned function detaches
 * everything again.
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
        useRuntimeStore.getState().applyFlowStatus(data.flowId, data.status);
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
    wsClient.subscribe('flow:snapshot', (data) => {
      if (data && typeof data.flowId === 'string') {
        useRuntimeStore.getState().applySnapshot(data);
        useRuntimeStore.getState().setSubscribed(data.flowId, true);
      }
    }),
    wsClient.subscribe('node:status', (data) => {
      if (data && typeof data.flowId === 'string' && typeof data.nodeId === 'string' && data.status) {
        useRuntimeStore.getState().applyNodeStatus(data.flowId, data.nodeId, data.status);
      }
    }),
    wsClient.subscribe('debug:message', (data) => {
      if (data && typeof data.flowId === 'string' && typeof data.id === 'string') {
        useRuntimeStore.getState().applyDebugMessage(data);
      }
    }),
    wsClient.subscribe('flow:metrics', (data) => {
      if (data && typeof data.flowId === 'string' && data.nodes) {
        useRuntimeStore.getState().applyMetrics(data.flowId, data.nodes);
      }
    }),
    wsClient.subscribe('error', (data) => {
      const message = (data && (data.message || data.error)) || 'Server error';
      notify('error', String(message));
    }),
  ];

  // Runtime events only arrive for subscribed flows: follow the open flow
  // and re-subscribe after every (re)connect.
  let subscribedFlowId: string | null = null;
  const subscribeTo = (flowId: string | null) => {
    if (subscribedFlowId && subscribedFlowId !== flowId) {
      void wsClient.send('unsubscribe', { flowId: subscribedFlowId }).catch(() => undefined);
      useRuntimeStore.getState().setSubscribed(subscribedFlowId, false);
    }
    subscribedFlowId = flowId;
    if (flowId) {
      void wsClient.send('subscribe', { flowId }).catch(() => undefined);
    }
  };

  const unsubscribeFlowChanges = useFlowStore.subscribe((state, previous) => {
    if (state.selectedFlowId !== previous.selectedFlowId) {
      subscribeTo(state.selectedFlowId);
    }
  });

  let wasConnected = wsClient.getState().connected;
  const unsubscribeConnection = wsClient.onStateChange((state) => {
    if (state.connected && !wasConnected && subscribedFlowId) {
      void wsClient.send('subscribe', { flowId: subscribedFlowId }).catch(() => undefined);
    }
    wasConnected = state.connected;
  });

  wsClient.connect();
  subscribeTo(useFlowStore.getState().selectedFlowId);

  return () => {
    for (const unsubscribe of unsubscribers) unsubscribe();
    unsubscribeFlowChanges();
    unsubscribeConnection();
  };
}
