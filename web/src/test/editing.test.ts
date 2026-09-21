import { describe, it, expect, vi, beforeEach } from 'vitest';
import type { Flow, FlowNode } from '../types/flow';

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
import { useFlowStore, __resetFlowStoreForTests } from '../store/flowStore';
import { useEditorStore } from '../store/editorStore';
import { copySelection, pasteClipboard, duplicateSelection, alignSelection, distributeSelection, autoLayoutFlow, toggleSelectionDisabled } from '../store/editorActions';
import { clipFromSelection, materializeClip, readClipboard } from '../lib/clipboard';
import { alignNodes, distributeNodes, autoLayout, snap } from '../lib/layout';

function node(id: string, x: number, y: number, type = 'function'): FlowNode {
  return { id, type, position: { x, y }, config: {}, disabled: false };
}

function makeFlow(): Flow {
  return {
    id: 'f1',
    name: 'Flow',
    description: '',
    nodes: { a: node('a', 0, 0, 'inject'), b: node('b', 200, 40), c: node('c', 400, 80, 'debug') },
    connections: [
      { id: 'ab', sourceNode: 'a', sourcePort: 'output', targetNode: 'b', targetPort: 'input' },
      { id: 'bc', sourceNode: 'b', sourcePort: 'output', targetNode: 'c', targetPort: 'input' },
    ],
    status: 'draft',
    config: { timeout: 30, maxConcurrency: 100, retryPolicy: { maxRetries: 3, backoff: 1, maxBackoff: 30, retryOn: [] }, environment: {} },
    createdAt: '2026-01-01T00:00:00Z',
    updatedAt: '2026-01-01T00:00:00Z',
    version: '1.0',
  };
}

const store = () => useFlowStore.getState();

beforeEach(async () => {
  __resetFlowStoreForTests();
  vi.useFakeTimers();
  window.localStorage.removeItem('go-red.clipboard');
  vi.mocked(api.fetchFlow).mockResolvedValue(makeFlow());
  vi.mocked(api.updateFlow).mockImplementation(async (id, data) => ({ ...makeFlow(), ...(data as Partial<Flow>), id }));
  await store().selectFlow('f1');
  useEditorStore.setState({ selectedNodeIds: [], pendingSelection: null });
});

describe('clipboard', () => {
  it('copies the selection with the wires inside it and pastes fresh ids', () => {
    const flow = makeFlow();
    const clip = clipFromSelection(flow.nodes, flow.connections, ['a', 'b']);
    expect(clip?.nodes.map((n) => n.id)).toEqual(['a', 'b']);
    expect(clip?.connections.map((c) => c.id)).toEqual(['ab']);

    let n = 0;
    const pasted = materializeClip(clip!, { x: 10, y: 10 }, () => `new${++n}`);
    expect(pasted.nodes.map((node) => [node.id, node.position])).toEqual([
      ['new1', { x: 10, y: 10 }],
      ['new2', { x: 210, y: 50 }],
    ]);
    expect(pasted.connections).toEqual([{ id: 'new3', sourceNode: 'new1', sourcePort: 'output', targetNode: 'new2', targetPort: 'input' }]);
    expect(clipFromSelection(flow.nodes, flow.connections, ['zzz'])).toBeNull();
  });

  it('copy, paste and duplicate go through the store as single undo steps', () => {
    useEditorStore.setState({ selectedNodeIds: ['a', 'b'] });
    expect(copySelection()).toBe(2);
    expect(readClipboard()?.nodes).toHaveLength(2);

    const ids = pasteClipboard({ x: 500, y: 500 });
    expect(ids).toHaveLength(2);
    expect(Object.keys(store().flow!.nodes)).toHaveLength(5);
    expect(store().flow!.connections).toHaveLength(3);
    expect(store().flow!.nodes[ids[0]].position).toEqual({ x: 500, y: 500 });
    expect(useEditorStore.getState().pendingSelection).toEqual(ids);

    store().undo();
    expect(Object.keys(store().flow!.nodes)).toHaveLength(3);

    useEditorStore.setState({ selectedNodeIds: ['c'] });
    const dup = duplicateSelection();
    expect(dup).toHaveLength(1);
    expect(store().flow!.nodes[dup[0]].position).toEqual({ x: 440, y: 120 });
    expect(store().flow!.nodes[dup[0]].type).toBe('debug');
  });
});

describe('layout helpers', () => {
  const nodes = [node('a', 0, 0), node('b', 100, 50), node('c', 300, 20)];

  it('aligns on edges and centers', () => {
    expect(alignNodes(nodes, 'left').map((p) => p.position.x)).toEqual([0, 0, 0]);
    expect(alignNodes(nodes, 'right').map((p) => p.position.x)).toEqual([300, 300, 300]);
    expect(alignNodes(nodes, 'centerX').map((p) => p.position.x)).toEqual([150, 150, 150]);
    expect(alignNodes(nodes, 'top').map((p) => p.position.y)).toEqual([0, 0, 0]);
    expect(alignNodes(nodes, 'bottom').map((p) => p.position.y)).toEqual([50, 50, 50]);
    expect(alignNodes(nodes, 'centerY').map((p) => p.position.y)).toEqual([25, 25, 25]);
    expect(alignNodes([nodes[0]], 'left')).toEqual([]);
  });

  it('distributes evenly between the outermost nodes', () => {
    expect(distributeNodes(nodes, 'horizontal').map((p) => [p.id, p.position.x])).toEqual([
      ['a', 0],
      ['b', 150],
      ['c', 300],
    ]);
    expect(distributeNodes(nodes, 'vertical').map((p) => [p.id, p.position.y])).toEqual([
      ['a', 0],
      ['c', 25],
      ['b', 50],
    ]);
    expect(distributeNodes(nodes.slice(0, 2), 'horizontal')).toEqual([]);
  });

  it('lays a chain out left to right on the grid and parks port-less nodes on the left', () => {
    const flow = makeFlow();
    const note = node('note', 999, 999, 'comment');
    const placements = autoLayout([...Object.values(flow.nodes), note], flow.connections, (n) => n.type !== 'comment');
    const byId = Object.fromEntries(placements.map((p) => [p.id, p.position]));
    expect(byId.a.x).toBeLessThan(byId.b.x);
    expect(byId.b.x).toBeLessThan(byId.c.x);
    expect(byId.a.y).toBe(byId.b.y);
    for (const p of placements) {
      expect(p.position.x % 20).toBe(0);
    }
    expect(byId.note).toEqual({ x: 40, y: 40 });
    expect(snap(31)).toBe(40);
  });

  it('align, distribute, layout and enable/disable act on the selection through the store', () => {
    useEditorStore.setState({ selectedNodeIds: ['a', 'b', 'c'] });
    alignSelection('top');
    expect(Object.values(store().flow!.nodes).map((n) => n.position.y)).toEqual([0, 0, 0]);
    distributeSelection('horizontal');
    expect(Object.values(store().flow!.nodes).map((n) => n.position.x)).toEqual([0, 200, 400]);
    toggleSelectionDisabled();
    expect(Object.values(store().flow!.nodes).every((n) => n.disabled)).toBe(true);
    toggleSelectionDisabled();
    expect(Object.values(store().flow!.nodes).some((n) => n.disabled)).toBe(false);
    autoLayoutFlow([]);
    expect(store().flow!.nodes.a.position.x).toBeLessThan(store().flow!.nodes.c.position.x);
  });
});

describe('edges and nodes', () => {
  it('inserts a new node on a wire and rewires both ends in one step', () => {
    const id = store().insertNodeOnEdge('function', 'ab', { x: 100, y: 0 })!;
    expect(id).toBeTruthy();
    const conns = store().flow!.connections;
    expect(conns.find((c) => c.id === 'ab')).toBeUndefined();
    expect(conns.some((c) => c.sourceNode === 'a' && c.targetNode === id && c.targetPort === 'input')).toBe(true);
    expect(conns.some((c) => c.sourceNode === id && c.targetNode === 'b' && c.sourcePort === 'output')).toBe(true);
    store().undo();
    expect(store().flow!.connections.map((c) => c.id)).toEqual(['ab', 'bc']);
    expect(store().insertNodeOnEdge('function', 'nope', { x: 0, y: 0 })).toBeNull();
  });

  it('splices an unconnected node into a wire and refuses connected ones', () => {
    const loose = store().addNode('function', { x: 100, y: 100 })!;
    store().spliceNodeIntoEdge(loose, 'bc');
    const conns = store().flow!.connections;
    expect(conns.some((c) => c.id === 'bc')).toBe(false);
    expect(conns.filter((c) => c.sourceNode === loose || c.targetNode === loose)).toHaveLength(2);

    const before = store().flow!.connections;
    store().spliceNodeIntoEdge('a', 'ab');
    expect(store().flow!.connections).toBe(before);
  });

  it('nudges nodes and records one history step per nudge', () => {
    store().nudgeNodes(['a', 'b'], 20, 0);
    expect(store().flow!.nodes.a.position).toEqual({ x: 20, y: 0 });
    expect(store().flow!.nodes.b.position).toEqual({ x: 220, y: 40 });
    store().undo();
    expect(store().flow!.nodes.a.position).toEqual({ x: 0, y: 0 });
  });
});

describe('flows', () => {
  it('duplicates a flow through the API and reorders tabs persisting only moved ones', async () => {
    vi.mocked(api.createFlow).mockResolvedValue({ ...makeFlow(), id: 'f2', name: 'Flow (copy)' });
    const copy = await store().duplicateFlow('f1');
    expect(api.createFlow).toHaveBeenCalledWith({ name: 'Flow (copy)', description: '' });
    expect(api.updateFlow).toHaveBeenCalledWith('f2', expect.objectContaining({ nodes: makeFlow().nodes }));
    expect(copy.id).toBe('f2');
    expect(store().flows.map((f) => f.id)).toContain('f2');

    vi.mocked(api.updateFlow).mockClear();
    useFlowStore.setState({
      flows: [
        { id: 'x', name: 'X', status: 'draft', nodeCount: 0, createdAt: '', updatedAt: '', order: 1 },
        { id: 'y', name: 'Y', status: 'draft', nodeCount: 0, createdAt: '', updatedAt: '', order: 2 },
        { id: 'z', name: 'Z', status: 'draft', nodeCount: 0, createdAt: '', updatedAt: '', order: 3 },
      ],
    });
    await store().reorderFlows(['z', 'x', 'y']);
    expect(store().flows.map((f) => f.id)).toEqual(['z', 'x', 'y']);
    expect(vi.mocked(api.updateFlow).mock.calls.map(([id, data]) => [id, (data as { order?: number }).order])).toEqual([
      ['z', 1],
      ['x', 2],
      ['y', 3],
    ]);
  });
});
