import { describe, it, expect } from 'vitest';
import { rebase, sameContent, sameValue, type FlowContent } from '../store/rebase';
import type { FlowConfig, FlowNode, NodeConnection } from '../types/flow';

const config: FlowConfig = { timeout: 30, maxConcurrency: 0, retryPolicy: { maxRetries: 0, backoff: 1, maxBackoff: 1, retryOn: [] }, environment: {} };

function node(id: string, extra: Partial<FlowNode> = {}): FlowNode {
  return { id, type: 'debug', name: id, position: { x: 0, y: 0 }, config: {}, disabled: false, ...extra };
}

function wire(id: string, source: string, target: string, ports: Partial<NodeConnection> = {}): NodeConnection {
  return { id, sourceNode: source, targetNode: target, ...ports };
}

function content(nodes: FlowNode[], connections: NodeConnection[] = [], extra: Partial<FlowContent> = {}): FlowContent {
  return { name: 'Flow', description: '', nodes: Object.fromEntries(nodes.map((n) => [n.id, n])), connections, config, ...extra };
}

describe('rebase', () => {
  const base = content([node('a'), node('b')], [wire('w1', 'a', 'b')]);

  it('takes everything from the server when this tab changed nothing', () => {
    const remote = content([node('a', { name: 'Renamed' }), node('b'), node('c')], [wire('w1', 'a', 'b'), wire('w2', 'b', 'c')], { name: 'Theirs' });
    expect(rebase(base, base, remote)).toEqual(remote);
  });

  it('keeps edits to different nodes from both sides', () => {
    const local = content([node('a', { name: 'Mine' }), node('b')], base.connections);
    const remote = content([node('a'), node('b', { name: 'Theirs' })], base.connections);
    const merged = rebase(base, local, remote);
    expect(merged.nodes.a.name).toBe('Mine');
    expect(merged.nodes.b.name).toBe('Theirs');
  });

  it('merges different fields of the same node', () => {
    const local = content([node('a', { position: { x: 50, y: 60 } }), node('b')], base.connections);
    const remote = content([node('a', { name: 'Theirs', config: { level: 2 } }), node('b')], base.connections);
    const merged = rebase(base, local, remote);
    expect(merged.nodes.a).toEqual(node('a', { name: 'Theirs', config: { level: 2 }, position: { x: 50, y: 60 } }));
  });

  it('lets the later save (this tab) win a field both changed', () => {
    const local = content([node('a', { name: 'Mine' }), node('b')], base.connections);
    const remote = content([node('a', { name: 'Theirs' }), node('b')], base.connections);
    expect(rebase(base, local, remote).nodes.a.name).toBe('Mine');
  });

  it('keeps nodes added on either side and lets a deletion on either side win', () => {
    const local = content([node('a', { name: 'Edited here' }), node('b'), node('mine')], base.connections);
    const remote = content([node('b'), node('theirs')], []);
    const merged = rebase(base, local, remote);
    expect(Object.keys(merged.nodes).sort()).toEqual(['b', 'mine', 'theirs']);
    expect(merged.connections).toEqual([]);

    const deletedHere = rebase(base, content([node('a')], []), content([node('a'), node('b', { name: 'Edited there' })], base.connections));
    expect(Object.keys(deletedHere.nodes)).toEqual(['a']);
  });

  it('merges wires as a set of endpoints, without duplicates or dangling ends', () => {
    const local = content([node('a'), node('b'), node('c')], [wire('w1', 'a', 'b'), wire('mine', 'b', 'c'), wire('dup-l', 'a', 'c')]);
    const remote = content([node('a'), node('b'), node('c')], [wire('dup-r', 'a', 'c', { sourcePort: 'output', targetPort: 'input' })]);
    const merged = rebase(content([node('a'), node('b'), node('c')], [wire('w1', 'a', 'b')]), local, remote);
    // w1 was removed remotely, the same a->c wire added on both sides stays once.
    expect(merged.connections.map((c) => c.id).sort()).toEqual(['dup-l', 'mine']);
  });

  it('takes name, description and config fields from the side that changed them', () => {
    const local = content(Object.values(base.nodes), base.connections, { description: 'Mine' });
    const remote = content(Object.values(base.nodes), base.connections, { name: 'Theirs', config: { ...config, timeout: 60 } });
    const merged = rebase(base, local, remote);
    expect([merged.name, merged.description, merged.config.timeout]).toEqual(['Theirs', 'Mine', 60]);
  });

  it('does not see a change in how the server echoes an untouched node', () => {
    // The tab's copy has fields the server omits when empty; the server's
    // copy has an explicit disabled:false and no description.
    const tabCopy = { ...node('a'), description: '', name: 'a' } as FlowNode;
    const serverEcho = { id: 'a', type: 'debug', name: 'a', position: { x: 0, y: 0 }, config: {}, disabled: false } as FlowNode;
    const remote = content([{ ...serverEcho, name: 'Theirs' }, node('b')], base.connections);
    const merged = rebase(content([serverEcho, node('b')], base.connections), content([tabCopy, node('b')], base.connections), remote);
    expect(merged.nodes.a.name).toBe('Theirs');
  });
});

describe('sameValue and sameContent', () => {
  it('treat absent, empty and default values alike and ignore key order', () => {
    expect(sameValue({ a: 1, b: '' }, { a: 1 })).toBe(true);
    expect(sameValue({ x: null, y: [], z: {} }, {})).toBe(true);
    expect(sameValue({ b: 2, a: 1 }, { a: 1, b: 2 })).toBe(true);
    expect(sameValue({ disabled: true }, { disabled: false })).toBe(false);
    expect(sameValue([1, 2], [2, 1])).toBe(false);
  });

  it('compare wires by endpoints with default ports', () => {
    const a = content([node('a'), node('b')], [wire('1', 'a', 'b')]);
    const b = content([node('a'), node('b')], [wire('2', 'a', 'b', { sourcePort: 'output', targetPort: 'input' })]);
    expect(sameContent(a, b)).toBe(true);
    expect(sameContent(a, content([node('a'), node('b')], []))).toBe(false);
  });
});
