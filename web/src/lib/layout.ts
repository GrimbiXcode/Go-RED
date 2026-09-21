import dagre from '@dagrejs/dagre';
import type { FlowNode, NodeConnection } from '../types/flow';

/** Approximate canvas size of a node, used for alignment and layout (NodeShell is 36 px high). */
export const NODE_WIDTH = 140;
export const NODE_HEIGHT = 36;
export const GRID = 20;

export type AlignMode = 'left' | 'centerX' | 'right' | 'top' | 'centerY' | 'bottom';
export type DistributeAxis = 'horizontal' | 'vertical';

export interface Placement {
  id: string;
  position: { x: number; y: number };
}

export function snap(value: number, grid = GRID): number {
  return Math.round(value / grid) * grid;
}

/** Positions that line the given nodes up on one edge or center line. */
export function alignNodes(nodes: FlowNode[], mode: AlignMode): Placement[] {
  if (nodes.length < 2) return [];
  const xs = nodes.map((n) => n.position.x);
  const ys = nodes.map((n) => n.position.y);
  const minX = Math.min(...xs);
  const maxX = Math.max(...xs);
  const minY = Math.min(...ys);
  const maxY = Math.max(...ys);
  const centerX = (minX + maxX) / 2;
  const centerY = (minY + maxY) / 2;
  return nodes.map((node) => {
    let { x, y } = node.position;
    switch (mode) {
      case 'left':
        x = minX;
        break;
      case 'centerX':
        x = centerX;
        break;
      case 'right':
        x = maxX;
        break;
      case 'top':
        y = minY;
        break;
      case 'centerY':
        y = centerY;
        break;
      case 'bottom':
        y = maxY;
        break;
    }
    return { id: node.id, position: { x, y } };
  });
}

/** Positions that spread the nodes evenly between the outermost two along one axis. */
export function distributeNodes(nodes: FlowNode[], axis: DistributeAxis): Placement[] {
  if (nodes.length < 3) return [];
  const key = axis === 'horizontal' ? 'x' : 'y';
  const sorted = [...nodes].sort((a, b) => a.position[key] - b.position[key]);
  const first = sorted[0].position[key];
  const last = sorted[sorted.length - 1].position[key];
  const step = (last - first) / (sorted.length - 1);
  return sorted.map((node, index) => ({
    id: node.id,
    position: { ...node.position, [key]: first + step * index },
  }));
}

/**
 * Left-to-right layered layout of the whole flow (dagre). Config nodes and
 * comments (no ports) are placed in a column on the left.
 */
export function autoLayout(nodes: FlowNode[], connections: NodeConnection[], hasPorts: (node: FlowNode) => boolean): Placement[] {
  const graph = new dagre.graphlib.Graph();
  graph.setGraph({ rankdir: 'LR', nodesep: 30, ranksep: 80, marginx: 40, marginy: 40 });
  graph.setDefaultEdgeLabel(() => ({}));
  const wired = nodes.filter(hasPorts);
  const loose = nodes.filter((node) => !hasPorts(node));
  for (const node of wired) graph.setNode(node.id, { width: NODE_WIDTH, height: NODE_HEIGHT });
  const known = new Set(wired.map((node) => node.id));
  for (const connection of connections) {
    if (known.has(connection.sourceNode) && known.has(connection.targetNode)) graph.setEdge(connection.sourceNode, connection.targetNode);
  }
  dagre.layout(graph);

  const placements: Placement[] = wired.map((node) => {
    const pos = graph.node(node.id);
    return { id: node.id, position: { x: snap(pos.x - NODE_WIDTH / 2), y: snap(pos.y - NODE_HEIGHT / 2) } };
  });
  loose.forEach((node, index) => {
    placements.push({ id: node.id, position: { x: 40, y: 40 + index * (NODE_HEIGHT + 24) } });
  });
  return placements;
}
