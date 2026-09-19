import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import type { Flow } from '../types/flow';

vi.mock('../utils/api', () => ({
  fetchFlows: vi.fn(),
  fetchFlow: vi.fn(),
  createFlow: vi.fn(),
  updateFlow: vi.fn(),
  deleteFlow: vi.fn(),
  deployFlow: vi.fn(),
  undeployFlow: vi.fn(),
  getNodes: vi.fn(),
  generateId: () => 'generated',
}));

import * as api from '../utils/api';
import {
  useFlowStore,
  __resetFlowStoreForTests,
  AUTOSAVE_DELAY_MS,
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
  vi.mocked(api.updateFlow).mockImplementation(async (id, data) => ({
    ...makeFlow(),
    ...(data as Partial<Flow>),
    id,
    updatedAt: '2026-01-01T00:00:10Z',
  }));
  vi.mocked(api.deployFlow).mockReset();
  vi.mocked(api.deployFlow).mockImplementation(async (id) => ({
    flowId: id,
    status: 'running',
    updatedAt: '2026-01-01T00:00:10Z',
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
    store().applyFlowList([{ id: 'f1', name: 'Flow', status: 'draft', nodeCount: 0, createdAt: T0, updatedAt: T0 }]);
    store().applyFlowStatus({ flowId: 'f1', status: 'running', deployedAt: '2026-01-01T00:00:20Z' });
    expect(store().flow?.status).toBe('running');
    expect(store().flows[0].status).toBe('running');

    store().applyFlowDeleted('f1');
    expect(store().flow).toBeNull();
    expect(store().flows).toHaveLength(0);
  });
});
