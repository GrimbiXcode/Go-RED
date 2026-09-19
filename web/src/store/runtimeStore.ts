import { create } from 'zustand';
import type { NodeStatus } from '../types/flow';
import type { Message, MessageLogEntry } from '../types/message';
import { wsClient } from '../lib/wsClient';

interface RuntimeState {
  /** Latest runtime status per flow and node, as pushed by the server. */
  nodeStatus: Record<string, Record<string, NodeStatus>>;
  /** Message log of the flow that was last requested. */
  messages: MessageLogEntry[];
  messagesFlowId: string | null;
  messagesLoading: boolean;
  messagesError: string | null;

  loadMessages: (flowId: string, limit?: number) => Promise<void>;
  clearMessages: () => void;
  applyNodeStatus: (flowId: string, nodeId: string, status: NodeStatus) => void;
  applyMessageLog: (flowId: string | undefined, messages: Message[]) => void;
  forgetFlow: (flowId: string) => void;
}

/** Converts a wire Message into a log entry the debug panel can render. */
export function toLogEntry(msg: Message): MessageLogEntry {
  return {
    id: msg.id,
    flowId: msg.flowId,
    message: msg,
    timestamp: msg.timestamp,
    level: 'info',
    nodeId: msg.path.length > 0 ? msg.path[msg.path.length - 1] : '',
  };
}

/**
 * Runtime information that the server owns and pushes: node status and the
 * message log. Nothing in here is edited by the user.
 */
export const useRuntimeStore = create<RuntimeState>((set, get) => ({
  nodeStatus: {},
  messages: [],
  messagesFlowId: null,
  messagesLoading: false,
  messagesError: null,

  loadMessages: async (flowId, limit = 100) => {
    set({ messagesLoading: true, messagesError: null, messagesFlowId: flowId });
    try {
      // Answered by a message:log event (see bindServerEvents).
      await wsClient.send('message:log', { flowId, limit });
    } catch (error) {
      set({ messagesLoading: false, messagesError: error instanceof Error ? error.message : String(error) });
    }
  },

  clearMessages: () => set({ messages: [] }),

  applyNodeStatus: (flowId, nodeId, status) =>
    set((state) => ({
      nodeStatus: {
        ...state.nodeStatus,
        [flowId]: { ...(state.nodeStatus[flowId] || {}), [nodeId]: status },
      },
    })),

  applyMessageLog: (flowId, messages) => {
    const requested = get().messagesFlowId;
    if (flowId && requested && flowId !== requested) {
      return; // answer to an older request for another flow
    }
    const entries = messages.map(toLogEntry).sort((a, b) => Date.parse(b.timestamp) - Date.parse(a.timestamp));
    set({ messages: entries, messagesLoading: false, messagesError: null });
  },

  forgetFlow: (flowId) =>
    set((state) => {
      const { [flowId]: _dropped, ...rest } = state.nodeStatus;
      return {
        nodeStatus: rest,
        messages: state.messagesFlowId === flowId ? [] : state.messages,
        messagesFlowId: state.messagesFlowId === flowId ? null : state.messagesFlowId,
      };
    }),
}));

/** Selects one node's runtime status; components subscribe per node. */
export const selectNodeStatus = (flowId: string | undefined, nodeId: string) => (state: RuntimeState) =>
  flowId ? state.nodeStatus[flowId]?.[nodeId] : undefined;
