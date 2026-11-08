import { create } from 'zustand';
import type { Flow, FlowNode, NodeConnection, FlowStatus } from '../types/flow';
import type { FlowSummary } from '../types/api';
import type { NodeMetadata } from '../types/node';
import * as api from '../utils/api';
import i18n from '../i18n';
import { rebase, sameContent, type FlowContent } from './rebase';
import { onReauthentication } from '../lib/auth';
import { notify } from './notificationStore';
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
  /** The flow's revision now; another one than the open flow's base means someone else saved. */
  revision?: string;
}

export interface NodeMove {
  id: string;
  position: { x: number; y: number };
}

export type NodePatch = Partial<Pick<FlowNode, 'name' | 'config' | 'disabled' | 'description'>>;

/** Side effects an edit may carry: connections that no longer have a port. */
export interface UpdateNodeOptions {
  removeConnections?: string[];
}

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
  /**
   * The server revision the working copy is based on, and that version's
   * content as the server stored it: set from GET and PUT responses (and a
   * conflict's current flow), never by status events, which only say that
   * someone saved, not what. Saves send baseRevision; when the server has
   * moved on, the tab pulls and reapplies its own changes (rebase.ts).
   */
  baseRevision: string | null;
  baseContent: FlowContent | null;

  loadFlows: () => Promise<void>;
  loadNodeTypes: () => Promise<void>;
  selectFlow: (flowId: string | null) => Promise<void>;
  createFlow: (name: string, description?: string) => Promise<Flow>;
  deleteFlow: (flowId: string) => Promise<void>;
  renameFlow: (name: string, description?: string) => void;
  flushSave: (options?: { keepalive?: boolean }) => Promise<void>;
  /**
   * Pulls a newer server version of the open flow: adopted as it is when
   * this tab has no unsaved changes, else with those changes reapplied on
   * top (and then saved).
   */
  pullRemote: (remote: Flow) => void;
  /** Fetches the open flow and pulls it if the server has a newer revision. */
  syncWithServer: () => Promise<void>;
  deploy: () => Promise<void>;
  undeploy: () => Promise<void>;

  addNode: (type: string, position: { x: number; y: number }, name?: string) => string | null;
  removeNodes: (ids: string[]) => void;
  moveNodes: (moves: NodeMove[]) => void;
  updateNode: (id: string, patch: NodePatch, options?: UpdateNodeOptions) => void;
  /** Adds several nodes (with their ids) and the connections between them in one undo step. */
  addNodes: (nodes: FlowNode[], connections: NodeConnection[]) => string[];
  /** Moves nodes by a delta (arrow keys). */
  nudgeNodes: (ids: string[], dx: number, dy: number) => void;
  /** Adds a node of `type` in the middle of a connection, rewiring both ends. */
  insertNodeOnEdge: (type: string, connectionId: string, position: { x: number; y: number }) => string | null;
  /** Rewires a connection through an existing, unconnected node. */
  spliceNodeIntoEdge: (nodeId: string, connectionId: string) => void;
  /** Renames any flow, open or not. */
  renameFlowById: (flowId: string, name: string) => Promise<void>;
  /** Creates a copy of a flow (nodes, connections, config) next to it. */
  duplicateFlow: (flowId: string) => Promise<Flow>;
  /** Persists a new tab order. */
  reorderFlows: (orderedIds: string[]) => Promise<void>;
  addConnection: (connection: Omit<NodeConnection, 'id'>) => void;
  removeConnections: (ids: string[]) => void;
  undo: () => void;
  redo: () => void;

  applyFlowStatus: (event: FlowStatusEvent) => void;
  applyFlowList: (flows: FlowSummary[]) => void;
  applyFlowDeleted: (flowId: string) => void;
}

/**
 * Unsaved work kept across a sign-in round trip (sessionStorage, so only
 * this tab and only until it closes): exactly what the autosave PUT sends.
 */
export const UNSAVED_STASH_KEY = 'go-red.unsavedFlow';
/** A stash older than this is not replayed. */
const UNSAVED_STASH_MAX_AGE_MS = 60 * 60 * 1000;

export interface UnsavedStash extends FlowContent {
  flowId: string;
  /** The server version the work was made on, to reapply it on whatever the server has later. */
  baseRevision: string | null;
  baseContent: FlowContent | null;
  stashedAt: number;
}

/** The part of a flow the editor edits and autosaves. */
export function contentOf(flow: Flow): FlowContent {
  return { name: flow.name, description: flow.description, nodes: flow.nodes, connections: flow.connections, config: flow.config };
}

/** Stores or removes the stash; false when sessionStorage refused (full or unavailable). */
function writeStash(stash: UnsavedStash | null): boolean {
  try {
    if (stash) window.sessionStorage.setItem(UNSAVED_STASH_KEY, JSON.stringify(stash));
    else window.sessionStorage.removeItem(UNSAVED_STASH_KEY);
    return true;
  } catch {
    return false;
  }
}

/**
 * Removes a stash for flowId once that flow saved successfully: the work is
 * on the server, and an older stash would later look like someone else's
 * newer version had overwritten it.
 */
function dropStashFor(flowId: string): void {
  try {
    const raw = window.sessionStorage.getItem(UNSAVED_STASH_KEY);
    if (raw && (JSON.parse(raw) as UnsavedStash).flowId === flowId) writeStash(null);
  } catch {
    // Unreadable: nothing to drop.
  }
}

/**
 * The stash for flowId, if there is a usable one. It stays stored until it
 * is applied or dropped, so a reload before that keeps it; an expired one
 * is removed. A stash for another flow is left alone.
 */
function readStash(flowId: string): UnsavedStash | null {
  let stash: UnsavedStash | null = null;
  try {
    const raw = window.sessionStorage.getItem(UNSAVED_STASH_KEY);
    stash = raw ? (JSON.parse(raw) as UnsavedStash) : null;
  } catch {
    stash = null;
  }
  if (!stash || stash.flowId !== flowId) return null;
  if (Date.now() - stash.stashedAt > UNSAVED_STASH_MAX_AGE_MS) {
    writeStash(null);
    return null;
  }
  return stash;
}

let saveTimer: ReturnType<typeof setTimeout> | null = null;
let saveInFlight: Promise<void> | null = null;

/** Debounce for following other sessions' saves (an active one saves every AUTOSAVE_DELAY_MS). */
export const SYNC_DELAY_MS = 300;
let syncTimer: ReturnType<typeof setTimeout> | null = null;
/** The newest revision a flow:status event reported for the open flow. */
let latestRemoteRevision: string | null = null;
/** Conflicts in a row before a save gives up (another session saving nonstop). */
const MAX_CONFLICT_RETRIES = 5;

function clearSaveTimer() {
  if (saveTimer) {
    clearTimeout(saveTimer);
    saveTimer = null;
  }
}

function clearSyncTimer() {
  if (syncTimer) {
    clearTimeout(syncTimer);
    syncTimer = null;
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
    order: flow.order,
    nodeCount: Object.keys(flow.nodes || {}).length,
    createdAt: flow.createdAt,
    updatedAt: flow.updatedAt,
    revision: flow.revision,
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
  baseRevision: null as string | null,
  baseContent: null as FlowContent | null,
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

  /** Replaces the open flow's content, keeping everything else (status, revision...). */
  const withContent = (flow: Flow, content: FlowContent): Flow => ({ ...flow, ...content });

  /**
   * Puts edits that could not be saved before a sign-in back over the open
   * flow, reapplied on whatever the server has now, as one undoable step
   * (undo returns to the server's version), and lets autosave send them.
   */
  const restoreStash = (stash: UnsavedStash) => {
    const { flow } = get();
    if (!flow || flow.id !== stash.flowId) return;
    writeStash(null);
    if (!stash.baseContent) return;
    const server = contentOf(flow);
    const merged = rebase(stash.baseContent, stash, server);
    if (sameContent(merged, server)) return;
    set((state) => ({
      flow: withContent(flow, merged),
      flows: state.flows.map((f) => (f.id === flow.id ? { ...f, name: merged.name, description: merged.description } : f)),
      history: pushHistory(state.history, documentOf(flow)),
      editVersion: state.editVersion + 1,
      saveState: 'pending',
    }));
    scheduleSave();
    notify('info', i18n.t('toast.unsavedRestored'));
  };

  /** Whether the open flow has changes that are not saved yet (or being saved). */
  const hasLocalChanges = () => saveInFlight !== null || get().editVersion !== get().savedVersion;

  /** Follows another session's save soon, once this tab is idle and visible. */
  const scheduleSync = () => {
    clearSyncTimer();
    syncTimer = setTimeout(() => {
      syncTimer = null;
      if (typeof document !== 'undefined' && document.hidden) return; // caught up on visibilitychange
      if (hasLocalChanges()) return; // the next save is checked against the server anyway
      // The echo of this tab's own save can arrive before its response; by
      // now the response may have made it the base, and there is nothing to pull.
      if (latestRemoteRevision === get().baseRevision) return;
      void get().syncWithServer();
    }, SYNC_DELAY_MS);
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
        latestRemoteRevision = flow.revision;
        set({ flow, flowLoading: false, baseRevision: flow.revision, baseContent: contentOf(flow) });
        const stash = readStash(flowId);
        if (stash) restoreStash(stash);
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
      for (let attempt = 0; ; attempt++) {
        const { flow, editVersion, savedVersion, baseRevision } = get();
        if (!flow || editVersion === savedVersion) {
          if (get().saveState === 'pending') set({ saveState: 'idle' });
          return;
        }
        const version = editVersion;
        const flowId = flow.id;
        let conflict: Flow | null = null;
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
                baseRevision: baseRevision ?? undefined,
              },
              { keepalive: options.keepalive }
            );
            dropStashFor(flowId);
            set((state) => {
              if (!state.flow || state.flow.id !== flowId) return state;
              const stillDirty = state.editVersion !== version;
              const nextFlow = {
                ...state.flow,
                status: saved.status,
                updatedAt: saved.updatedAt,
                deployedAt: saved.deployedAt,
                revision: saved.revision,
              };
              return {
                flow: nextFlow,
                flows: replaceSummary(state.flows, summaryFromFlow(nextFlow)),
                savedVersion: version,
                // The base is what the server stored, not what was sent: the
                // server normalizes, and later comparisons must not see that
                // as a change of this tab's.
                baseRevision: saved.revision,
                baseContent: contentOf(saved),
                saveState: stillDirty ? 'pending' : 'idle',
                saveError: null,
              };
            });
          } catch (error) {
            conflict = api.revisionConflict(error);
            if (!conflict) {
              set({ saveState: 'error', saveError: errorMessage(error) });
              throw error;
            }
          } finally {
            saveInFlight = null;
          }
        })();
        try {
          await saveInFlight;
        } catch {
          return;
        }
        if (conflict) {
          // Someone else saved first: pull their version, reapply ours on
          // top, and save again on their revision.
          if (attempt >= MAX_CONFLICT_RETRIES) {
            set({ saveState: 'error', saveError: i18n.t('toast.saveKeepsConflicting') });
            return;
          }
          get().pullRemote(conflict);
          clearSaveTimer();
          continue;
        }
        if (get().flow?.id !== flowId) return;
        if (get().editVersion !== version) {
          scheduleSave();
        } else if (latestRemoteRevision && latestRemoteRevision !== get().baseRevision) {
          scheduleSync(); // someone saved while this save was on its way
        }
        return;
      }
    },

    pullRemote: (remote) => {
      const { flow, baseContent, baseRevision } = get();
      if (!flow || flow.id !== remote.id || remote.revision === baseRevision) return;
      const server = contentOf(remote);
      const local = get().editVersion !== get().savedVersion;
      const merged = local && baseContent ? rebase(baseContent, contentOf(flow), server) : server;
      const pending = local && !sameContent(merged, server);
      // Undo history is dropped: undoing to a document from before the pull
      // would silently revert the other session's changes.
      set((state) => ({
        flow: withContent({ ...state.flow!, status: remote.status, updatedAt: remote.updatedAt, deployedAt: remote.deployedAt, revision: remote.revision }, merged),
        flows: replaceSummary(state.flows, summaryFromFlow(withContent(remote, merged))),
        history: emptyHistory(),
        baseRevision: remote.revision,
        baseContent: server,
        editVersion: state.editVersion + 1,
        savedVersion: pending ? state.savedVersion : state.editVersion + 1,
        saveState: pending ? 'pending' : state.saveState === 'error' ? 'error' : 'idle',
      }));
      if (pending) scheduleSave();
    },

    syncWithServer: async () => {
      const flowId = get().flow?.id;
      if (!flowId) return;
      try {
        const remote = await api.fetchFlow(flowId);
        latestRemoteRevision = remote.revision;
        if (get().flow?.id === flowId) get().pullRemote(remote);
      } catch {
        // Offline or signed out: the next save or event tries again.
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

    updateNode: (id, patch, options) => {
      commit((doc) => {
        const node = doc.nodes[id];
        if (!node) return null;
        const next: FlowNode = { ...node, ...patch };
        if (!next.description) delete next.description;
        const remove = new Set(options?.removeConnections || []);
        const connections = remove.size > 0 ? doc.connections.filter((c) => !remove.has(c.id)) : doc.connections;
        if (
          next.name === node.name &&
          next.disabled === node.disabled &&
          (next.description || '') === (node.description || '') &&
          JSON.stringify(next.config) === JSON.stringify(node.config) &&
          connections === doc.connections
        ) {
          return null;
        }
        return { nodes: { ...doc.nodes, [id]: next }, connections };
      });
    },

    addNodes: (incoming, connections) => {
      if (incoming.length === 0) return [];
      const ids: string[] = [];
      commit((doc) => {
        const nodes = { ...doc.nodes };
        for (const node of incoming) {
          if (node.id in nodes) continue;
          nodes[node.id] = node;
          ids.push(node.id);
        }
        if (ids.length === 0) return null;
        const added = connections.filter((c) => c.sourceNode in nodes && c.targetNode in nodes && !doc.connections.some((e) => isSameConnection(c, e)));
        return { nodes, connections: [...doc.connections, ...added] };
      });
      return ids;
    },

    nudgeNodes: (ids, dx, dy) => {
      const { flow } = get();
      if (!flow || (dx === 0 && dy === 0)) return;
      const moves = ids
        .filter((id) => id in flow.nodes)
        .map((id) => ({ id, position: { x: flow.nodes[id].position.x + dx, y: flow.nodes[id].position.y + dy } }));
      get().moveNodes(moves);
    },

    insertNodeOnEdge: (type, connectionId, position) => {
      const id = generateId();
      let inserted = false;
      commit((doc) => {
        const edge = doc.connections.find((c) => c.id === connectionId);
        if (!edge) return null;
        inserted = true;
        const node: FlowNode = { id, type, name: '', position, config: {}, disabled: false };
        const connections = doc.connections.filter((c) => c.id !== connectionId);
        connections.push(
          { id: generateId(), sourceNode: edge.sourceNode, sourcePort: edge.sourcePort, targetNode: id, targetPort: 'input' },
          { id: generateId(), sourceNode: id, sourcePort: 'output', targetNode: edge.targetNode, targetPort: edge.targetPort }
        );
        return { nodes: { ...doc.nodes, [id]: node }, connections };
      });
      return inserted ? id : null;
    },

    spliceNodeIntoEdge: (nodeId, connectionId) => {
      commit((doc) => {
        const edge = doc.connections.find((c) => c.id === connectionId);
        if (!edge || !(nodeId in doc.nodes) || edge.sourceNode === nodeId || edge.targetNode === nodeId) return null;
        if (doc.connections.some((c) => c.sourceNode === nodeId || c.targetNode === nodeId)) return null;
        const connections = doc.connections.filter((c) => c.id !== connectionId);
        connections.push(
          { id: generateId(), sourceNode: edge.sourceNode, sourcePort: edge.sourcePort, targetNode: nodeId, targetPort: 'input' },
          { id: generateId(), sourceNode: nodeId, sourcePort: 'output', targetNode: edge.targetNode, targetPort: edge.targetPort }
        );
        return { nodes: doc.nodes, connections };
      });
    },

    renameFlowById: async (flowId, name) => {
      const trimmed = name.trim();
      if (!trimmed) return;
      if (get().flow?.id === flowId) {
        get().renameFlow(trimmed);
        return;
      }
      const saved = await api.updateFlow(flowId, { name: trimmed });
      set((state) => ({ flows: replaceSummary(state.flows, summaryFromFlow(saved)) }));
    },

    duplicateFlow: async (flowId) => {
      if (get().flow?.id === flowId) await get().flushSave();
      const source = await api.fetchFlow(flowId);
      const created = await api.createFlow({ name: `${source.name} (copy)`, description: source.description });
      const copy = await api.updateFlow(created.id, { nodes: source.nodes, connections: source.connections, config: source.config });
      set((state) => ({ flows: replaceSummary(state.flows, summaryFromFlow(copy)) }));
      return copy;
    },

    reorderFlows: async (orderedIds) => {
      const current = get().flows;
      const rank = new Map(orderedIds.map((id, index) => [id, index]));
      const next = [...current].sort((a, b) => (rank.get(a.id) ?? 1e9) - (rank.get(b.id) ?? 1e9)).map((flow, index) => ({ ...flow, order: index + 1 }));
      set({ flows: next });
      await Promise.all(
        next.filter((flow) => current.find((f) => f.id === flow.id)?.order !== flow.order).map((flow) => api.updateFlow(flow.id, { order: flow.order }))
      );
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

    applyFlowStatus: (event) => {
      if (event.revision && get().flow?.id === event.flowId) {
        latestRemoteRevision = event.revision;
        if (event.revision !== get().baseRevision) scheduleSync();
      }
      set((state) => {
        const flows = state.flows.map((f) =>
          f.id === event.flowId
            ? {
                ...f,
                status: event.status,
                updatedAt: event.updatedAt || f.updatedAt,
                deployedAt: event.deployedAt || f.deployedAt,
                revision: event.revision || f.revision,
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
      });
    },

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
  clearSyncTimer();
  saveInFlight = null;
  latestRemoteRevision = null;
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

/**
 * Keeps what could not be saved when the page leaves for the sign-in.
 * Returns whether leaving is safe: nothing unsaved, or the stash was written.
 */
export function stashUnsavedWork(): boolean {
  const { flow, editVersion, savedVersion } = useFlowStore.getState();
  if (!flow || editVersion === savedVersion) return true;
  const { baseRevision, baseContent } = useFlowStore.getState();
  return writeStash({ flowId: flow.id, ...contentOf(flow), baseRevision, baseContent, stashedAt: Date.now() });
}

/** Retries what failed while the editor was unauthenticated (token entered in place). */
export function resumeAfterReauthentication(): void {
  const state = useFlowStore.getState();
  if (state.flowsError) void state.loadFlows();
  if (state.nodeTypesError) void state.loadNodeTypes();
  if (state.saveState === 'error') void state.flushSave();
  else if (state.flowError && state.selectedFlowId) void state.selectFlow(state.selectedFlowId);
}

onReauthentication({ beforeSignIn: stashUnsavedWork, resume: resumeAfterReauthentication });
