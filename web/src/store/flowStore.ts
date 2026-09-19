import { create } from 'zustand';
import type { Flow, FlowNode, NodeConnection, FlowStatus } from '../types/flow';
import type { FlowSummary } from '../types/api';
import type { NodeMetadata } from '../types/node';
import * as api from '../utils/api';
import {
  type FlowDocument,
  type History,
  canRedo,
  canUndo,
  emptyHistory,
  pushHistory,
  redoHistory,
  undoHistory,
} from './history';

/**
 * The flow document store.
 *
 * There is exactly one write path: every edit changes the working copy in
 * this store (undoable), and the working copy is saved to the server with a
 * debounced PUT of the whole flow. The server keeps that as the flow's
 * draft definition; the running instance only changes on Deploy. So
 * "dirty" means "the draft differs from what is running", never "unsaved".
 */

export type SaveState = 'idle' | 'pending' | 'saving' | 'error';

export const AUTOSAVE_DELAY_MS = 600;

export interface FlowStatusEvent {
  flowId: string;
  status: FlowStatus;
  updatedAt?: string;
  deployedAt?: string;
}

export interface NodeMove {
  id: string;
  position: { x: number; y: number };
}

export type NodePatch = Partial<Pick<FlowNode, 'name' | 'config' | 'disabled'>>;

interface FlowState {
  flows: FlowSummary[];
  flowsLoading: boolean;
  flowsError: string | null;

  nodeTypes: NodeMetadata[];
  nodeTypesLoading: boolean;
  nodeTypesError: string | null;

  selectedFlowId: string | null;
  flow: Flow | null;
  flowLoading: boolean;
  flowError: string | null;

  history: History;
  saveState: SaveState;
  saveError: string | null;
  /** Bumped on every local edit. */
  editVersion: number;
  /** editVersion the last successful save covered. */
  savedVersion: number;
  /** editVersion at the time of the last deploy in this session. */
  deployedVersion: number;

  loadFlows: () => Promise<void>;
  loadNodeTypes: () => Promise<void>;
  selectFlow: (flowId: string | null) => Promise<void>;
  createFlow: (name: string, description?: string) => Promise<Flow>;
  deleteFlow: (flowId: string) => Promise<void>;
  renameFlow: (name: string, description?: string) => void;
  flushSave: (options?: { keepalive?: boolean }) => Promise<void>;
  deploy: () => Promise<void>;
  undeploy: () => Promise<void>;

  addNode: (type: string, position: { x: number; y: number }, name?: string) => string | null;
  removeNodes: (ids: string[]) => void;
  moveNodes: (moves: NodeMove[]) => void;
  updateNode: (id: string, patch: NodePatch) => void;
  addConnection: (connection: Omit<NodeConnection, 'id'>) => void;
  removeConnections: (ids: string[]) => void;
  undo: () => void;
  redo: () => void;

  applyFlowStatus: (event: FlowStatusEvent) => void;
  applyFlowList: (flows: FlowSummary[]) => void;
  applyFlowDeleted: (flowId: string) => void;
}

let saveTimer: ReturnType<typeof setTimeout> | null = null;
let saveInFlight: Promise<void> | null = null;

function clearSaveTimer() {
  if (saveTimer) {
    clearTimeout(saveTimer);
    saveTimer = null;
  }
}

function errorMessage(error: unknown): string {
  return error instanceof Error ? error.message : String(error);
}

export function generateId(): string {
  return Math.random().toString(36).slice(2, 10) + Date.now().toString(36);
}

function documentOf(flow: Flow): FlowDocument {
  return { nodes: flow.nodes, connections: flow.connections };
}

function summaryFromFlow(flow: Flow): FlowSummary {
  return {
    id: flow.id,
    name: flow.name,
    description: flow.description,
    status: flow.status,
    nodeCount: Object.keys(flow.nodes || {}).length,
    createdAt: flow.createdAt,
    updatedAt: flow.updatedAt,
    deployedAt: flow.deployedAt,
  };
}

function replaceSummary(flows: FlowSummary[], summary: FlowSummary): FlowSummary[] {
  const index = flows.findIndex((flow) => flow.id === summary.id);
  if (index === -1) return [...flows, summary];
  const next = flows.slice();
  next[index] = { ...next[index], ...summary };
  return next;
}

function isSameConnection(a: Omit<NodeConnection, 'id'>, b: NodeConnection): boolean {
  return (
    a.sourceNode === b.sourceNode &&
    a.targetNode === b.targetNode &&
    (a.sourcePort || 'output') === (b.sourcePort || 'output') &&
    (a.targetPort || 'input') === (b.targetPort || 'input')
  );
}

const initialDocumentState = {
  history: emptyHistory(),
  saveState: 'idle' as SaveState,
  saveError: null,
  editVersion: 0,
  savedVersion: 0,
  deployedVersion: 0,
};

export const useFlowStore = create<FlowState>((set, get) => {
  /** Applies an undoable document change and schedules an autosave. */
  const commit = (mutate: (current: FlowDocument) => FlowDocument | null) => {
    const { flow, history } = get();
    if (!flow) return;
    const current = documentOf(flow);
    const next = mutate(current);
    if (!next) return;
    set((state) => ({
      flow: state.flow ? { ...state.flow, nodes: next.nodes, connections: next.connections } : state.flow,
      history: pushHistory(history, current),
      editVersion: state.editVersion + 1,
      saveState: 'pending',
    }));
    scheduleSave();
  };

  /** Applies a document from the undo/redo stack (not itself recorded). */
  const restore = (document: FlowDocument, history: History) => {
    set((state) => ({
      flow: state.flow ? { ...state.flow, nodes: document.nodes, connections: document.connections } : state.flow,
      history,
      editVersion: state.editVersion + 1,
      saveState: 'pending',
    }));
    scheduleSave();
  };

  const scheduleSave = () => {
    clearSaveTimer();
    saveTimer = setTimeout(() => {
      saveTimer = null;
      void get().flushSave();
    }, AUTOSAVE_DELAY_MS);
  };

  return {
    flows: [],
    flowsLoading: false,
    flowsError: null,
    nodeTypes: [],
    nodeTypesLoading: false,
    nodeTypesError: null,
    selectedFlowId: null,
    flow: null,
    flowLoading: false,
    flowError: null,
    ...initialDocumentState,

    loadFlows: async () => {
      set({ flowsLoading: true, flowsError: null });
      try {
        const flows = await api.fetchFlows();
        set({ flows, flowsLoading: false });
      } catch (error) {
        set({ flowsLoading: false, flowsError: errorMessage(error) });
      }
    },

    loadNodeTypes: async () => {
      set({ nodeTypesLoading: true, nodeTypesError: null });
      try {
        const nodeTypes = await api.getNodes();
        set({ nodeTypes, nodeTypesLoading: false });
      } catch (error) {
        set({ nodeTypesLoading: false, nodeTypesError: errorMessage(error) });
      }
    },

    selectFlow: async (flowId) => {
      if (flowId === get().selectedFlowId && (get().flow || get().flowLoading)) return;
      await get().flushSave();
      if (flowId === null) {
        set({ selectedFlowId: null, flow: null, flowLoading: false, flowError: null, ...initialDocumentState });
        return;
      }
      set({ selectedFlowId: flowId, flowLoading: true, flowError: null, flow: null, ...initialDocumentState });
      try {
        const flow = await api.fetchFlow(flowId);
        if (get().selectedFlowId !== flowId) return;
        set({ flow, flowLoading: false });
      } catch (error) {
        if (get().selectedFlowId !== flowId) return;
        set({ flowLoading: false, flowError: errorMessage(error) });
      }
    },

    createFlow: async (name, description) => {
      const flow = await api.createFlow({ name, description });
      set((state) => ({ flows: replaceSummary(state.flows, summaryFromFlow(flow)) }));
      return flow;
    },

    deleteFlow: async (flowId) => {
      if (get().selectedFlowId === flowId) {
        clearSaveTimer();
      }
      await api.deleteFlow(flowId);
      get().applyFlowDeleted(flowId);
    },

    renameFlow: (name, description) => {
      const { flow } = get();
      if (!flow) return;
      const nextName = name.trim() || flow.name;
      const nextDescription = description ?? flow.description;
      if (nextName === flow.name && nextDescription === flow.description) return;
      set((state) => ({
        flow: state.flow ? { ...state.flow, name: nextName, description: nextDescription } : state.flow,
        flows: state.flows.map((f) => (f.id === flow.id ? { ...f, name: nextName, description: nextDescription } : f)),
        editVersion: state.editVersion + 1,
        saveState: 'pending',
      }));
      scheduleSave();
    },

    flushSave: async (options = {}) => {
      clearSaveTimer();
      if (saveInFlight) {
        await saveInFlight;
      }
      const { flow, editVersion, savedVersion } = get();
      if (!flow || editVersion === savedVersion) {
        if (get().saveState === 'pending') set({ saveState: 'idle' });
        return;
      }
      const version = editVersion;
      const flowId = flow.id;
      set({ saveState: 'saving' });
      saveInFlight = (async () => {
        try {
          const saved = await api.updateFlow(
            flowId,
            {
              name: flow.name,
              description: flow.description,
              nodes: flow.nodes,
              connections: flow.connections,
              config: flow.config,
            },
            { keepalive: options.keepalive }
          );
          set((state) => {
            if (!state.flow || state.flow.id !== flowId) return state;
            const stillDirty = state.editVersion !== version;
            const nextFlow = {
              ...state.flow,
              status: saved.status,
              updatedAt: saved.updatedAt,
              deployedAt: saved.deployedAt,
            };
            return {
              flow: nextFlow,
              flows: replaceSummary(state.flows, summaryFromFlow(nextFlow)),
              savedVersion: version,
              saveState: stillDirty ? 'pending' : 'idle',
              saveError: null,
            };
          });
        } catch (error) {
          set({ saveState: 'error', saveError: errorMessage(error) });
          throw error;
        } finally {
          saveInFlight = null;
        }
      })();
      try {
        await saveInFlight;
      } catch {
        return;
      }
      if (get().flow?.id === flowId && get().editVersion !== version) {
        scheduleSave();
      }
    },

    deploy: async () => {
      const flowId = get().selectedFlowId;
      if (!flowId) throw new Error('No flow selected');
      await get().flushSave();
      if (get().saveState === 'error') {
        throw new Error(get().saveError || 'Saving the flow failed');
      }
      const response = await api.deployFlow(flowId);
      set((state) => {
        const flow =
          state.flow && state.flow.id === flowId
            ? {
                ...state.flow,
                status: response.status,
                updatedAt: response.updatedAt || state.flow.updatedAt,
                deployedAt: response.deployedAt || state.flow.deployedAt,
              }
            : state.flow;
        return {
          flow,
          flows: flow && flow.id === flowId ? replaceSummary(state.flows, summaryFromFlow(flow)) : state.flows,
          deployedVersion: state.editVersion,
        };
      });
    },

    undeploy: async () => {
      const flowId = get().selectedFlowId;
      if (!flowId) throw new Error('No flow selected');
      const response = await api.undeployFlow(flowId);
      get().applyFlowStatus({
        flowId,
        status: response.status,
        updatedAt: response.updatedAt,
        deployedAt: response.deployedAt,
      });
    },

    addNode: (type, position, name) => {
      const id = generateId();
      let added = false;
      commit((doc) => {
        added = true;
        const node: FlowNode = {
          id,
          type,
          name: name || '',
          position,
          config: {},
          disabled: false,
        };
        return { nodes: { ...doc.nodes, [id]: node }, connections: doc.connections };
      });
      return added ? id : null;
    },

    removeNodes: (ids) => {
      if (ids.length === 0) return;
      commit((doc) => {
        const remove = new Set(ids.filter((id) => id in doc.nodes));
        if (remove.size === 0) return null;
        const nodes: Record<string, FlowNode> = {};
        for (const [id, node] of Object.entries(doc.nodes)) {
          if (!remove.has(id)) nodes[id] = node;
        }
        const connections = doc.connections.filter((c) => !remove.has(c.sourceNode) && !remove.has(c.targetNode));
        return { nodes, connections };
      });
    },

    moveNodes: (moves) => {
      if (moves.length === 0) return;
      commit((doc) => {
        let changed = false;
        const nodes = { ...doc.nodes };
        for (const move of moves) {
          const node = nodes[move.id];
          if (!node) continue;
          if (node.position && node.position.x === move.position.x && node.position.y === move.position.y) continue;
          nodes[move.id] = { ...node, position: { x: move.position.x, y: move.position.y } };
          changed = true;
        }
        return changed ? { nodes, connections: doc.connections } : null;
      });
    },

    updateNode: (id, patch) => {
      commit((doc) => {
        const node = doc.nodes[id];
        if (!node) return null;
        const next: FlowNode = { ...node, ...patch };
        if (
          next.name === node.name &&
          next.disabled === node.disabled &&
          JSON.stringify(next.config) === JSON.stringify(node.config)
        ) {
          return null;
        }
        return { nodes: { ...doc.nodes, [id]: next }, connections: doc.connections };
      });
    },

    addConnection: (connection) => {
      commit((doc) => {
        if (!doc.nodes[connection.sourceNode] || !doc.nodes[connection.targetNode]) return null;
        if (doc.connections.some((existing) => isSameConnection(connection, existing))) return null;
        const next: NodeConnection = {
          id: generateId(),
          sourceNode: connection.sourceNode,
          sourcePort: connection.sourcePort || 'output',
          targetNode: connection.targetNode,
          targetPort: connection.targetPort || 'input',
        };
        return { nodes: doc.nodes, connections: [...doc.connections, next] };
      });
    },

    removeConnections: (ids) => {
      if (ids.length === 0) return;
      commit((doc) => {
        const remove = new Set(ids);
        const connections = doc.connections.filter((c) => !remove.has(c.id));
        return connections.length === doc.connections.length ? null : { nodes: doc.nodes, connections };
      });
    },

    undo: () => {
      const { flow, history } = get();
      if (!flow) return;
      const result = undoHistory(history, documentOf(flow));
      if (result) restore(result.document, result.history);
    },

    redo: () => {
      const { flow, history } = get();
      if (!flow) return;
      const result = redoHistory(history, documentOf(flow));
      if (result) restore(result.document, result.history);
    },

    applyFlowStatus: (event) =>
      set((state) => {
        const flows = state.flows.map((f) =>
          f.id === event.flowId
            ? {
                ...f,
                status: event.status,
                updatedAt: event.updatedAt || f.updatedAt,
                deployedAt: event.deployedAt || f.deployedAt,
              }
            : f
        );
        const flow =
          state.flow && state.flow.id === event.flowId
            ? {
                ...state.flow,
                status: event.status,
                updatedAt: event.updatedAt || state.flow.updatedAt,
                deployedAt: event.deployedAt || state.flow.deployedAt,
              }
            : state.flow;
        return { flows, flow };
      }),

    applyFlowList: (flows) => set({ flows }),

    applyFlowDeleted: (flowId) =>
      set((state) => ({
        flows: state.flows.filter((f) => f.id !== flowId),
        ...(state.selectedFlowId === flowId
          ? { selectedFlowId: null, flow: null, flowLoading: false, flowError: null, ...initialDocumentState }
          : {}),
      })),
  };
});

// Selectors --------------------------------------------------------------

export const selectCanUndo = (state: FlowState) => canUndo(state.history);
export const selectCanRedo = (state: FlowState) => canRedo(state.history);

/** True when the draft differs from what is (or was last) deployed. */
export function hasUndeployedChanges(state: Pick<FlowState, 'flow' | 'editVersion' | 'deployedVersion' | 'savedVersion' | 'saveState'>): boolean {
  const { flow } = state;
  if (!flow) return false;
  if (state.editVersion !== state.deployedVersion) return true;
  if (state.saveState === 'error') return true;
  if (!flow.deployedAt) return true;
  return Date.parse(flow.updatedAt) > Date.parse(flow.deployedAt);
}

export const selectCanDeploy = (state: FlowState) =>
  !!state.flow && (state.flow.status !== 'running' || hasUndeployedChanges(state));

export const selectCanUndeploy = (state: FlowState) => !!state.flow && state.flow.status === 'running';

/** Test helper: clears pending autosave timers and in-flight bookkeeping. */
export function __resetFlowStoreForTests(): void {
  clearSaveTimer();
  saveInFlight = null;
  useFlowStore.setState({
    flows: [],
    flowsLoading: false,
    flowsError: null,
    nodeTypes: [],
    nodeTypesLoading: false,
    nodeTypesError: null,
    selectedFlowId: null,
    flow: null,
    flowLoading: false,
    flowError: null,
    ...initialDocumentState,
  });
}
