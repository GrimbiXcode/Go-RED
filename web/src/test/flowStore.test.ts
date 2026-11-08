import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import type { Flow } from '../types/flow';

vi.mock('../utils/api', async () => {
  const actual = await vi.importActual<typeof import('../utils/api')>('../utils/api');
  return {
  ApiError: actual.ApiError,
  revisionConflict: actual.revisionConflict,
  fetchFlows: vi.fn(),
  fetchFlow: vi.fn(),
  createFlow: vi.fn(),
  updateFlow: vi.fn(),
  deleteFlow: vi.fn(),
  deployFlow: vi.fn(),
  undeployFlow: vi.fn(),
  getNodes: vi.fn(),
  generateId: () => 'generated',
  };
});

import * as api from '../utils/api';
import { ApiError } from '../utils/api';
import { isSigningIn, onReauthentication, resumeAfterToken, startSignIn, useAuthStore } from '../lib/auth';
import {
  UNSAVED_STASH_KEY,
  resumeAfterReauthentication,
  stashUnsavedWork,
  useFlowStore,
  __resetFlowStoreForTests,
  AUTOSAVE_DELAY_MS,
  SYNC_DELAY_MS,
  hasUndeployedChanges,
  selectCanDeploy,
  selectCanUndo,
  selectCanRedo,
} from '../store/flowStore';

const T0 = '2026-01-01T00:00:00Z';

function makeFlow(overrides: Partial<Flow> = {}): Flow {
  return {
    id: 'f1',
    name: 'Flow',
    description: '',
    nodes: {},
    connections: [],
    status: 'draft',
    config: { timeout: 30, maxConcurrency: 100, retryPolicy: { maxRetries: 3, backoff: 1, maxBackoff: 30, retryOn: [] }, environment: {} },
    createdAt: T0,
    updatedAt: T0,
    revision: 'r1',
    version: '1.0',
    ...overrides,
  };
}

async function openFlow(flow = makeFlow()) {
  vi.mocked(api.fetchFlow).mockResolvedValue(flow);
  await useFlowStore.getState().selectFlow(flow.id);
  return useFlowStore.getState();
}

const store = () => useFlowStore.getState();

beforeEach(() => {
  __resetFlowStoreForTests();
  vi.useFakeTimers();
  vi.mocked(api.updateFlow).mockReset();
  let saves = 0;
  vi.mocked(api.updateFlow).mockImplementation(async (id, data) => {
    const { baseRevision: _base, ...content } = data;
    return { ...makeFlow(), ...(content as Partial<Flow>), id, updatedAt: '2026-01-01T00:00:10Z', revision: `saved-${++saves}` };
  });
  vi.mocked(api.deployFlow).mockReset();
  vi.mocked(api.deployFlow).mockImplementation(async (id) => ({
    flowId: id,
    status: 'running',
    updatedAt: '2026-01-01T00:00:10Z',
    revision: 'r1',
    deployedAt: '2026-01-01T00:00:20Z',
  }));
});

afterEach(() => {
  vi.useRealTimers();
});

describe('flowStore document editing', () => {
  it('loads the selected flow with empty history', async () => {
    const state = await openFlow();
    expect(state.flow?.id).toBe('f1');
    expect(selectCanUndo(state)).toBe(false);
    expect(state.saveState).toBe('idle');
  });

  it('adds nodes, records history and autosaves once for a burst of edits', async () => {
    await openFlow();
    const a = store().addNode('inject', { x: 10, y: 10 });
    const b = store().addNode('debug', { x: 200, y: 10 });
    expect(a).toBeTruthy();
    expect(Object.keys(store().flow!.nodes)).toHaveLength(2);
    expect(store().saveState).toBe('pending');
    expect(api.updateFlow).not.toHaveBeenCalled();

    await vi.advanceTimersByTimeAsync(AUTOSAVE_DELAY_MS + 1);

    expect(api.updateFlow).toHaveBeenCalledTimes(1);
    const [, payload] = vi.mocked(api.updateFlow).mock.calls[0];
    expect(Object.keys(payload.nodes!)).toEqual([a, b]);
    expect(store().saveState).toBe('idle');
    expect(store().flow?.updatedAt).toBe('2026-01-01T00:00:10Z');
  });

  it('undo and redo walk the history and schedule saves', async () => {
    await openFlow();
    const a = store().addNode('inject', { x: 0, y: 0 })!;
    store().addNode('debug', { x: 100, y: 0 });
    expect(Object.keys(store().flow!.nodes)).toHaveLength(2);

    store().undo();
    expect(Object.keys(store().flow!.nodes)).toEqual([a]);
    expect(selectCanRedo(store())).toBe(true);

    store().undo();
    expect(Object.keys(store().flow!.nodes)).toHaveLength(0);
    expect(selectCanUndo(store())).toBe(false);

    store().redo();
    store().redo();
    expect(Object.keys(store().flow!.nodes)).toHaveLength(2);
    expect(selectCanRedo(store())).toBe(false);

    // A new edit after undo discards the redo branch.
    store().undo();
    store().addNode('function', { x: 50, y: 50 });
    expect(selectCanRedo(store())).toBe(false);
  });

  it('removing nodes also removes their connections', async () => {
    await openFlow();
    const a = store().addNode('inject', { x: 0, y: 0 })!;
    const b = store().addNode('debug', { x: 100, y: 0 })!;
    store().addConnection({ sourceNode: a, targetNode: b, sourcePort: 'output', targetPort: 'input' });
    expect(store().flow!.connections).toHaveLength(1);

    store().removeNodes([b]);
    expect(Object.keys(store().flow!.nodes)).toEqual([a]);
    expect(store().flow!.connections).toHaveLength(0);

    store().undo();
    expect(store().flow!.connections).toHaveLength(1);
  });

  it('ignores duplicate connections and connections to unknown nodes', async () => {
    await openFlow();
    const a = store().addNode('inject', { x: 0, y: 0 })!;
    const b = store().addNode('debug', { x: 100, y: 0 })!;
    const historyBefore = store().history.past.length;

    store().addConnection({ sourceNode: a, targetNode: b });
    store().addConnection({ sourceNode: a, targetNode: b, sourcePort: 'output', targetPort: 'input' });
    store().addConnection({ sourceNode: a, targetNode: 'ghost' });

    expect(store().flow!.connections).toHaveLength(1);
    expect(store().history.past.length).toBe(historyBefore + 1);
  });

  it('moveNodes records one history entry per drag and skips no-op moves', async () => {
    await openFlow();
    const a = store().addNode('inject', { x: 0, y: 0 })!;
    const before = store().history.past.length;

    store().moveNodes([{ id: a, position: { x: 0, y: 0 } }]);
    expect(store().history.past.length).toBe(before);

    store().moveNodes([{ id: a, position: { x: 40, y: 60 } }]);
    expect(store().flow!.nodes[a].position).toEqual({ x: 40, y: 60 });
    expect(store().history.past.length).toBe(before + 1);
  });

  it('updateNode changes name and config and is undoable', async () => {
    await openFlow();
    const a = store().addNode('function', { x: 0, y: 0 })!;
    store().updateNode(a, { name: 'Transform', config: { code: 'return msg;' } });
    expect(store().flow!.nodes[a].name).toBe('Transform');
    expect(store().flow!.nodes[a].config).toEqual({ code: 'return msg;' });

    store().updateNode(a, { name: 'Transform', config: { code: 'return msg;' } });
    store().undo();
    expect(store().flow!.nodes[a].name).toBe('');
  });
  it('updateNode can drop connections in the same undo step and keeps the description', async () => {
    await openFlow();
    const sw = store().addNode('switch', { x: 0, y: 0 })!;
    const a = store().addNode('debug', { x: 100, y: 0 })!;
    const b = store().addNode('debug', { x: 100, y: 100 })!;
    store().addConnection({ sourceNode: sw, targetNode: a, sourcePort: '0', targetPort: 'input' });
    store().addConnection({ sourceNode: sw, targetNode: b, sourcePort: '1', targetPort: 'input' });
    const stale = store().flow!.connections.find((c) => c.sourcePort === '1')!;

    store().updateNode(sw, { config: { rules: [{ operator: 'else' }] }, description: 'Only one rule now' }, { removeConnections: [stale.id] });
    expect(store().flow!.connections.map((c) => c.sourcePort)).toEqual(['0']);
    expect(store().flow!.nodes[sw].description).toBe('Only one rule now');

    store().undo();
    expect(store().flow!.connections).toHaveLength(2);
    expect(store().flow!.nodes[sw].description).toBeUndefined();

    // An empty description is not stored.
    store().redo();
    store().updateNode(sw, { description: '' });
    expect('description' in store().flow!.nodes[sw]).toBe(false);
  });
});

describe('flowStore save and deploy', () => {
  it('deploy flushes the pending save before deploying', async () => {
    await openFlow();
    store().addNode('debug', { x: 0, y: 0 });
    const calls: string[] = [];
    vi.mocked(api.updateFlow).mockImplementation(async (id, data) => {
      calls.push('save');
      return { ...makeFlow(), ...(data as Partial<Flow>), id, updatedAt: '2026-01-01T00:00:10Z' };
    });
    vi.mocked(api.deployFlow).mockImplementation(async (id) => {
      calls.push('deploy');
      return { flowId: id, status: 'running', updatedAt: '2026-01-01T00:00:10Z', deployedAt: '2026-01-01T00:00:20Z' };
    });

    await store().deploy();

    expect(calls).toEqual(['save', 'deploy']);
    expect(store().flow?.status).toBe('running');
    expect(store().flow?.deployedAt).toBe('2026-01-01T00:00:20Z');
    expect(hasUndeployedChanges(store())).toBe(false);
    expect(selectCanDeploy(store())).toBe(false);
  });

  it('reports undeployed changes after an edit and clears them on deploy', async () => {
    await openFlow(makeFlow({ status: 'running', updatedAt: T0, deployedAt: '2026-01-01T00:00:05Z' }));
    expect(hasUndeployedChanges(store())).toBe(false);

    store().addNode('debug', { x: 0, y: 0 });
    expect(hasUndeployedChanges(store())).toBe(true);
    expect(selectCanDeploy(store())).toBe(true);

    await store().deploy();
    expect(hasUndeployedChanges(store())).toBe(false);
  });

  it('a draft that never ran can always be deployed', async () => {
    await openFlow(makeFlow({ status: 'draft' }));
    expect(hasUndeployedChanges(store())).toBe(true);
    expect(selectCanDeploy(store())).toBe(true);
  });

  it('a failed save is surfaced, blocks deploy and is retried on the next flush', async () => {
    await openFlow();
    store().addNode('debug', { x: 0, y: 0 });
    vi.mocked(api.updateFlow).mockRejectedValueOnce(new Error('boom')).mockRejectedValueOnce(new Error('boom'));

    await vi.advanceTimersByTimeAsync(AUTOSAVE_DELAY_MS + 1);
    expect(store().saveState).toBe('error');
    expect(store().saveError).toBe('boom');

    // Deploy retries the save first; while that keeps failing it refuses to deploy.
    await expect(store().deploy()).rejects.toThrow('boom');
    expect(api.deployFlow).not.toHaveBeenCalled();
    expect(api.updateFlow).toHaveBeenCalledTimes(2);

    // Once the server accepts the save, the store recovers.
    await store().flushSave();
    expect(store().saveState).toBe('idle');
    expect(api.updateFlow).toHaveBeenCalledTimes(3);
  });

  it('switching flows flushes the previous flow first', async () => {
    await openFlow();
    store().addNode('debug', { x: 0, y: 0 });
    vi.mocked(api.fetchFlow).mockResolvedValue(makeFlow({ id: 'f2', name: 'Other' }));

    await store().selectFlow('f2');

    expect(api.updateFlow).toHaveBeenCalledTimes(1);
    expect(vi.mocked(api.updateFlow).mock.calls[0][0]).toBe('f1');
    expect(store().flow?.id).toBe('f2');
    expect(selectCanUndo(store())).toBe(false);
  });

  it('server events update status and drop deleted flows', async () => {
    await openFlow();
    store().applyFlowList([{ id: 'f1', name: 'Flow', status: 'draft', nodeCount: 0, createdAt: T0, updatedAt: T0, revision: 'r1' }]);
    store().applyFlowStatus({ flowId: 'f1', status: 'running', deployedAt: '2026-01-01T00:00:20Z' });
    expect(store().flow?.status).toBe('running');
    expect(store().flows[0].status).toBe('running');

    store().applyFlowDeleted('f1');
    expect(store().flow).toBeNull();
    expect(store().flows).toHaveLength(0);
  });
});

describe('unsaved work across re-authentication', () => {
  const stashed = () => JSON.parse(window.sessionStorage.getItem(UNSAVED_STASH_KEY) ?? 'null');

  beforeEach(() => {
    window.sessionStorage.clear();
    useAuthStore.setState({ user: null, required: false, info: null });
  });

  afterEach(() => {
    window.sessionStorage.clear();
    vi.unstubAllGlobals();
    onReauthentication({ beforeSignIn: stashUnsavedWork, resume: resumeAfterReauthentication });
  });

  it('stashes an edit that could not be saved and restores it when the flow opens again', async () => {
    await openFlow();
    vi.mocked(api.updateFlow).mockRejectedValueOnce(new Error('authentication required'));
    const id = store().addNode('debug', { x: 10, y: 10 })!;
    await vi.advanceTimersByTimeAsync(AUTOSAVE_DELAY_MS + 1);
    expect(store().saveState).toBe('error');

    // The dialog's sign-in button: the flow store's hook stashes first.
    vi.stubGlobal('location', { ...window.location, pathname: '/flow/f1', search: '', assign: vi.fn() });
    startSignIn({ methods: ['oidc'], loginUrl: '/auth/login' });
    expect(stashed()).toMatchObject({ flowId: 'f1', name: 'Flow' });
    expect(Object.keys(stashed().nodes)).toEqual([id]);

    // Back from the provider: a fresh page opens the flow from the server.
    __resetFlowStoreForTests();
    await openFlow();
    expect(Object.keys(store().flow!.nodes)).toEqual([id]);
    expect(store().saveState).toBe('pending');
    expect(selectCanUndo(store())).toBe(true);
    expect(window.sessionStorage.getItem(UNSAVED_STASH_KEY)).toBeNull();

    await vi.advanceTimersByTimeAsync(AUTOSAVE_DELAY_MS + 1);
    expect(store().saveState).toBe('idle');
    const calls = vi.mocked(api.updateFlow).mock.calls;
    const [, payload] = calls[calls.length - 1];
    expect(Object.keys(payload.nodes!)).toEqual([id]);

    store().undo();
    expect(store().flow!.nodes).toEqual({});
  });

  it('stashes nothing when everything is saved, and keeps a stash for another flow', async () => {
    await openFlow();
    stashUnsavedWork();
    expect(stashed()).toBeNull();

    const other = { flowId: 'f2', name: 'Other', description: '', nodes: {}, connections: [], config: makeFlow().config, stashedAt: Date.now() };
    window.sessionStorage.setItem(UNSAVED_STASH_KEY, JSON.stringify(other));
    __resetFlowStoreForTests();
    await openFlow();
    expect(store().saveState).toBe('idle');
    expect(stashed()).toEqual(other);
  });

  it('does not replay a stash older than an hour', async () => {
    const old = { flowId: 'f1', name: 'Old', description: '', nodes: { x: { id: 'x' } }, connections: [], config: makeFlow().config, stashedAt: Date.now() - 2 * 60 * 60 * 1000 };
    window.sessionStorage.setItem(UNSAVED_STASH_KEY, JSON.stringify(old));
    await openFlow();
    expect(store().flow!.name).toBe('Flow');
    expect(store().saveState).toBe('idle');
    expect(stashed()).toBeNull();
  });

  const debugNode = (id: string, name = id) => ({ id, type: 'debug', name, position: { x: 0, y: 0 }, config: {}, disabled: false });

  it('reapplies a stash on a version someone else saved since, keeping their changes', async () => {
    // Made on r1 with node a; this tab renamed a. Since then someone renamed
    // the flow and added a node (r2).
    const baseContent = { name: 'Flow', description: '', nodes: { a: debugNode('a') }, connections: [], config: makeFlow().config };
    window.sessionStorage.setItem(
      UNSAVED_STASH_KEY,
      JSON.stringify({ flowId: 'f1', ...baseContent, nodes: { a: debugNode('a', 'Mine') }, baseRevision: 'r1', baseContent, stashedAt: Date.now() })
    );
    await openFlow(makeFlow({ name: 'Theirs', revision: 'r2', nodes: { a: debugNode('a'), theirs: debugNode('theirs') } }));

    expect(store().flow!.name).toBe('Theirs');
    expect(store().flow!.nodes.a.name).toBe('Mine');
    expect(Object.keys(store().flow!.nodes).sort()).toEqual(['a', 'theirs']);
    expect(stashed()).toBeNull();
    await vi.advanceTimersByTimeAsync(AUTOSAVE_DELAY_MS + 1);
    expect(vi.mocked(api.updateFlow).mock.calls[0][1].baseRevision).toBe('r2');

    store().undo();
    expect(store().flow!.nodes.a.name).toBe('a');
  });

  it('keeps the base revision when a status event reports someone else saving', async () => {
    await openFlow();
    vi.mocked(api.fetchFlow).mockResolvedValue(makeFlow({ revision: 'r1' }));
    store().applyFlowStatus({ flowId: 'f1', status: 'draft', updatedAt: '2026-01-01T00:05:00Z', revision: 'r9' });
    expect(store().baseRevision).toBe('r1');

    vi.mocked(api.updateFlow).mockRejectedValueOnce(new Error('authentication required'));
    store().addNode('debug', { x: 0, y: 0 });
    await vi.advanceTimersByTimeAsync(AUTOSAVE_DELAY_MS + 1);
    stashUnsavedWork();
    expect(stashed().baseRevision).toBe('r1');
    expect(stashed().baseContent.nodes).toEqual({});
  });

  it('drops the stash for a flow once it saves successfully', async () => {
    await openFlow();
    vi.mocked(api.updateFlow).mockRejectedValueOnce(new Error('authentication required'));
    store().addNode('debug', { x: 0, y: 0 });
    await vi.advanceTimersByTimeAsync(AUTOSAVE_DELAY_MS + 1);
    expect(stashUnsavedWork()).toBe(true);
    expect(stashed()).not.toBeNull();

    // The sign-in did not leave the page after all; the next save works.
    await store().flushSave();
    expect(store().saveState).toBe('idle');
    expect(stashed()).toBeNull();
  });

  it('keeps the leave prompt when the stash cannot be written, and after coming back from the cache', async () => {
    vi.stubGlobal('location', { ...window.location, pathname: '/flow/f1', search: '', assign: vi.fn() });
    await openFlow();
    vi.mocked(api.updateFlow).mockRejectedValueOnce(new Error('authentication required'));
    store().addNode('debug', { x: 0, y: 0 });
    await vi.advanceTimersByTimeAsync(AUTOSAVE_DELAY_MS + 1);

    const setItem = vi.spyOn(Storage.prototype, 'setItem').mockImplementation(() => {
      throw new Error('QuotaExceededError');
    });
    startSignIn({ methods: ['oidc'], loginUrl: '/auth/login' });
    expect(isSigningIn()).toBe(false);
    setItem.mockRestore();

    startSignIn({ methods: ['oidc'], loginUrl: '/auth/login' });
    expect(isSigningIn()).toBe(true);
    const back = new Event('pageshow');
    Object.defineProperty(back, 'persisted', { value: true });
    window.dispatchEvent(back);
    expect(isSigningIn()).toBe(false);
  });

  it('a token entered mid-session retries the failed save in place instead of reloading', async () => {
    await openFlow();
    vi.mocked(api.updateFlow).mockRejectedValueOnce(new Error('authentication required'));
    store().addNode('inject', { x: 0, y: 0 });
    await vi.advanceTimersByTimeAsync(AUTOSAVE_DELAY_MS + 1);
    expect(store().saveState).toBe('error');

    const reload = vi.fn();
    useAuthStore.setState({ user: { method: 'token' } });
    resumeAfterToken(reload);
    await vi.advanceTimersByTimeAsync(0);
    expect(reload).not.toHaveBeenCalled();
    expect(api.updateFlow).toHaveBeenCalledTimes(2);
    expect(store().saveState).toBe('idle');

    // Nothing was authenticated in this page load yet: reload instead.
    useAuthStore.setState({ user: null });
    resumeAfterToken(reload);
    expect(reload).toHaveBeenCalledTimes(1);
  });
});

describe('saves ordered by revision', () => {
  const debugNode = (id: string, name = id) => ({ id, type: 'debug', name, position: { x: 0, y: 0 }, config: {}, disabled: false });
  const conflictWith = (flow: Flow) => new ApiError('the flow was changed by someone else', 409, { code: 'revision_conflict', flow });

  it('sends the base revision and moves it to the saved one', async () => {
    await openFlow();
    store().addNode('debug', { x: 0, y: 0 });
    await vi.advanceTimersByTimeAsync(AUTOSAVE_DELAY_MS + 1);
    expect(vi.mocked(api.updateFlow).mock.calls[0][1].baseRevision).toBe('r1');
    expect(store().baseRevision).toBe('saved-1');
    store().addNode('debug', { x: 50, y: 0 });
    await vi.advanceTimersByTimeAsync(AUTOSAVE_DELAY_MS + 1);
    expect(vi.mocked(api.updateFlow).mock.calls[1][1].baseRevision).toBe('saved-1');
  });

  it('on a conflict pulls the other version, reapplies its own change and saves again', async () => {
    await openFlow(makeFlow({ nodes: { a: debugNode('a'), b: debugNode('b') } }));
    store().updateNode('a', { name: 'Mine' });
    vi.mocked(api.updateFlow).mockRejectedValueOnce(conflictWith(makeFlow({ revision: 'r2', nodes: { a: debugNode('a'), b: debugNode('b', 'Theirs') } })));
    await vi.advanceTimersByTimeAsync(AUTOSAVE_DELAY_MS + 1);

    const calls = vi.mocked(api.updateFlow).mock.calls;
    expect(calls).toHaveLength(2);
    expect(calls[1][1].baseRevision).toBe('r2');
    expect(calls[1][1].nodes!.a.name).toBe('Mine');
    expect(calls[1][1].nodes!.b.name).toBe('Theirs');
    expect(store().flow!.nodes.b.name).toBe('Theirs');
    expect(store().saveState).toBe('idle');
    expect(selectCanUndo(store())).toBe(false);
  });

  it('gives up after repeated conflicts instead of looping', async () => {
    await openFlow();
    store().addNode('debug', { x: 0, y: 0 });
    let n = 1;
    vi.mocked(api.updateFlow).mockImplementation(async () => {
      throw conflictWith(makeFlow({ revision: `other-${++n}` }));
    });
    await vi.advanceTimersByTimeAsync(AUTOSAVE_DELAY_MS + 1);
    expect(store().saveState).toBe('error');
    expect(vi.mocked(api.updateFlow).mock.calls.length).toBe(6);
  });

  it('an idle tab follows a save from another session; a busy one waits for its own save', async () => {
    await openFlow();
    vi.mocked(api.fetchFlow).mockClear();
    vi.mocked(api.fetchFlow).mockResolvedValue(makeFlow({ revision: 'r2', nodes: { theirs: debugNode('theirs') } }));

    // Its own save's echo is not a reason to fetch.
    store().applyFlowStatus({ flowId: 'f1', status: 'draft', revision: 'r1' });
    await vi.advanceTimersByTimeAsync(SYNC_DELAY_MS + 1);
    expect(api.fetchFlow).not.toHaveBeenCalled();

    store().applyFlowStatus({ flowId: 'f1', status: 'draft', revision: 'r2' });
    store().applyFlowStatus({ flowId: 'f1', status: 'draft', revision: 'r2' });
    await vi.advanceTimersByTimeAsync(SYNC_DELAY_MS + 1);
    expect(api.fetchFlow).toHaveBeenCalledTimes(1);
    expect(Object.keys(store().flow!.nodes)).toEqual(['theirs']);
    expect(store().baseRevision).toBe('r2');
    expect(store().saveState).toBe('idle');
    expect(api.updateFlow).not.toHaveBeenCalled();

    // The echo of its own save arriving before the save's response is no reason either.
    vi.mocked(api.fetchFlow).mockClear();
    let finish!: () => void;
    vi.mocked(api.updateFlow).mockImplementationOnce(
      (id, data) =>
        new Promise((resolve) => {
          finish = () => resolve({ ...makeFlow(), ...(data as Partial<Flow>), id, revision: 'r-own' });
        })
    );
    store().addNode('debug', { x: 10, y: 0 });
    await vi.advanceTimersByTimeAsync(AUTOSAVE_DELAY_MS + 1);
    store().applyFlowStatus({ flowId: 'f1', status: 'draft', revision: 'r-own' });
    finish();
    await vi.advanceTimersByTimeAsync(SYNC_DELAY_MS + 1);
    expect(store().baseRevision).toBe('r-own');
    expect(api.fetchFlow).not.toHaveBeenCalled();

    // With an edit pending it does not fetch; its save is checked instead.
    vi.mocked(api.fetchFlow).mockClear();
    store().addNode('debug', { x: 0, y: 0 });
    store().applyFlowStatus({ flowId: 'f1', status: 'draft', revision: 'r3' });
    await vi.advanceTimersByTimeAsync(SYNC_DELAY_MS + 1);
    expect(api.fetchFlow).not.toHaveBeenCalled();
  });
});
