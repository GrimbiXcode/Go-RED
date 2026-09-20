import { describe, it, expect } from 'vitest';
import { flowNodeToCanvasNode, connectionToEdge, mergeCanvasNodes } from '../components/FlowCanvas';
import type { CanvasNode } from '../components/canvasTypes';
import type { FlowNode, NodeRegistry } from '../types/flow';

const registry: NodeRegistry = {
  inject: { type: 'inject', name: 'Inject', category: 'input' },
};

function node(id: string, overrides: Partial<FlowNode> = {}): FlowNode {
  return { id, type: 'inject', position: { x: 1, y: 2 }, config: {}, disabled: false, ...overrides };
}

describe('flowNodeToCanvasNode', () => {
  it('uses the registered renderer for known node types and the default one otherwise', () => {
    expect(flowNodeToCanvasNode(node('a'), registry, 'f').type).toBe('inject');
    expect(flowNodeToCanvasNode(node('b', { type: 'switch' }), registry, 'f').type).toBe('default');
  });

  it('falls back to the type name as label and to the origin as position', () => {
    const converted = flowNodeToCanvasNode({ ...node('a'), position: undefined as unknown as { x: number; y: number } }, registry, 'f');
    expect(converted.position).toEqual({ x: 0, y: 0 });
    expect(converted.data.label).toBe('Inject');
    expect(flowNodeToCanvasNode(node('b', { name: 'Named' }), registry, 'f').data.label).toBe('Named');
    expect(converted.data.flowId).toBe('f');
  });
});

describe('connectionToEdge', () => {
  it('maps ports to handles', () => {
    expect(connectionToEdge({ id: 'c', sourceNode: 'a', targetNode: 'b', sourcePort: '1', targetPort: 'input' })).toEqual({
      id: 'c',
      source: 'a',
      target: 'b',
      sourceHandle: '1',
      targetHandle: 'input',
    });
  });
});

describe('mergeCanvasNodes', () => {
  const incoming = (): CanvasNode[] => [
    flowNodeToCanvasNode(node('a', { position: { x: 10, y: 10 } }), registry, 'f'),
    flowNodeToCanvasNode(node('b', { position: { x: 20, y: 20 } }), registry, 'f'),
  ];

  it('keeps the on-screen position of a node that is being dragged', () => {
    const current: CanvasNode[] = [{ ...incoming()[0], position: { x: 99, y: 99 }, dragging: true }];
    const merged = mergeCanvasNodes(current, incoming());
    expect(merged[0].position).toEqual({ x: 99, y: 99 });
    expect(merged[1].position).toEqual({ x: 20, y: 20 });
  });

  it('keeps the selection and takes the store position for idle nodes', () => {
    const current: CanvasNode[] = [{ ...incoming()[0], position: { x: 99, y: 99 }, selected: true, dragging: false }];
    const merged = mergeCanvasNodes(current, incoming());
    expect(merged[0].position).toEqual({ x: 10, y: 10 });
    expect(merged[0].selected).toBe(true);
  });

  it('keeps the measured size so xyflow does not hide the node again', () => {
    const current: CanvasNode[] = [{ ...incoming()[0], measured: { width: 140, height: 36 } }];
    const merged = mergeCanvasNodes(current, incoming());
    expect(merged[0].measured).toEqual({ width: 140, height: 36 });
    expect(merged[1].measured).toBeUndefined();
  });

  it('drops nodes that no longer exist', () => {
    const merged = mergeCanvasNodes(incoming(), incoming().slice(1));
    expect(merged.map((n) => n.id)).toEqual(['b']);
  });
});
