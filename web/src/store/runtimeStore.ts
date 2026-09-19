import { create } from 'zustand';
import type { NodeStatus, DebugMessage, NodeMetrics, FlowSnapshot, FlowStatus } from '../types/generated';

/** How many debug entries are kept per flow in the browser. */
export const DEBUG_LIMIT = 500;

interface RuntimeState {
  /** Latest status per flow and node, as reported by nodes at runtime. */
  nodeStatus: Record<string, Record<string, NodeStatus>>;
  /** Per-node counters per flow since the flow was deployed. */
  metrics: Record<string, Record<string, NodeMetrics>>;
  /** Debug sidebar entries per flow, oldest first. */
  debug: Record<string, DebugMessage[]>;
  /** Flows this client is subscribed to (runtime events arrive for these). */
  subscribed: Record<string, boolean>;

  applySnapshot: (snapshot: FlowSnapshot) => void;
  applyNodeStatus: (flowId: string, nodeId: string, status: NodeStatus) => void;
  applyDebugMessage: (message: DebugMessage) => void;
  applyMetrics: (flowId: string, nodes: Record<string, NodeMetrics>) => void;
  /** A flow that stopped (or failed to start) has no live node state. */
  applyFlowStatus: (flowId: string, status: FlowStatus) => void;
  clearDebug: (flowId: string) => void;
  forgetFlow: (flowId: string) => void;
  setSubscribed: (flowId: string, subscribed: boolean) => void;
}

function withoutKey<T>(record: Record<string, T>, key: string): Record<string, T> {
  if (!(key in record)) return record;
  const { [key]: _dropped, ...rest } = record;
  return rest;
}

/**
 * Runtime information that the server owns and pushes: node status, debug
 * output and counters. Nothing in here is edited by the user.
 */
export const useRuntimeStore = create<RuntimeState>((set) => ({
  nodeStatus: {},
  metrics: {},
  debug: {},
  subscribed: {},

  applySnapshot: (snapshot) =>
    set((state) => ({
      nodeStatus: { ...state.nodeStatus, [snapshot.flowId]: snapshot.nodeStatus || {} },
      metrics: { ...state.metrics, [snapshot.flowId]: snapshot.metrics || {} },
      debug: { ...state.debug, [snapshot.flowId]: (snapshot.debug || []).slice(-DEBUG_LIMIT) },
    })),

  applyNodeStatus: (flowId, nodeId, status) =>
    set((state) => ({
      nodeStatus: {
        ...state.nodeStatus,
        [flowId]: { ...(state.nodeStatus[flowId] || {}), [nodeId]: status },
      },
    })),

  applyDebugMessage: (message) =>
    set((state) => {
      const entries = state.debug[message.flowId] || [];
      if (entries.length > 0 && entries[entries.length - 1].id === message.id) return state;
      const next = entries.length >= DEBUG_LIMIT ? [...entries.slice(entries.length - DEBUG_LIMIT + 1), message] : [...entries, message];
      return { debug: { ...state.debug, [message.flowId]: next } };
    }),

  applyMetrics: (flowId, nodes) =>
    set((state) => ({ metrics: { ...state.metrics, [flowId]: nodes || {} } })),

  applyFlowStatus: (flowId, status) =>
    set((state) => {
      if (status === 'running') return state;
      return {
        nodeStatus: withoutKey(state.nodeStatus, flowId),
        metrics: withoutKey(state.metrics, flowId),
      };
    }),

  clearDebug: (flowId) => set((state) => ({ debug: { ...state.debug, [flowId]: [] } })),

  forgetFlow: (flowId) =>
    set((state) => ({
      nodeStatus: withoutKey(state.nodeStatus, flowId),
      metrics: withoutKey(state.metrics, flowId),
      debug: withoutKey(state.debug, flowId),
      subscribed: withoutKey(state.subscribed, flowId),
    })),

  setSubscribed: (flowId, subscribed) =>
    set((state) => ({
      subscribed: subscribed ? { ...state.subscribed, [flowId]: true } : withoutKey(state.subscribed, flowId),
    })),
}));

const EMPTY_DEBUG: DebugMessage[] = [];

/** Selects one node's runtime status; components subscribe per node. */
export const selectNodeStatus = (flowId: string | undefined, nodeId: string) => (state: RuntimeState) =>
  flowId ? state.nodeStatus[flowId]?.[nodeId] : undefined;

/** Selects one node's counters. */
export const selectNodeMetrics = (flowId: string | undefined, nodeId: string) => (state: RuntimeState) =>
  flowId ? state.metrics[flowId]?.[nodeId] : undefined;

/** Selects a flow's debug entries (stable empty array when there are none). */
export const selectDebug = (flowId: string | undefined) => (state: RuntimeState) =>
  (flowId && state.debug[flowId]) || EMPTY_DEBUG;
