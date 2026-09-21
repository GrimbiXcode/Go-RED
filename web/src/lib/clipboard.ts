import type { FlowNode, NodeConnection } from '../types/flow';

/**
 * Editor clipboard for nodes: a selection is serialized with the
 * connections between the selected nodes and kept in localStorage, so it
 * can be pasted into another flow or another tab of the same browser.
 */

const STORAGE_KEY = 'go-red.clipboard';
const CLIP_VERSION = 1;

export interface Clip {
  version: number;
  nodes: FlowNode[];
  connections: NodeConnection[];
}

export interface PasteResult {
  nodes: FlowNode[];
  connections: NodeConnection[];
}

let memory: Clip | null = null;

/** Picks the selected nodes and the connections that stay entirely inside the selection. */
export function clipFromSelection(nodes: Record<string, FlowNode>, connections: NodeConnection[], selectedIds: string[]): Clip | null {
  const selected = new Set(selectedIds.filter((id) => id in nodes));
  if (selected.size === 0) return null;
  return {
    version: CLIP_VERSION,
    nodes: Array.from(selected).map((id) => structuredClone(nodes[id])),
    connections: connections.filter((c) => selected.has(c.sourceNode) && selected.has(c.targetNode)).map((c) => ({ ...c })),
  };
}

export function writeClipboard(clip: Clip): void {
  memory = clip;
  try {
    window.localStorage.setItem(STORAGE_KEY, JSON.stringify(clip));
  } catch {
    // Storage unavailable: the in-memory copy still serves this tab.
  }
}

export function readClipboard(): Clip | null {
  try {
    const raw = window.localStorage.getItem(STORAGE_KEY);
    if (raw) {
      const parsed = JSON.parse(raw) as Clip;
      if (parsed && parsed.version === CLIP_VERSION && Array.isArray(parsed.nodes)) return parsed;
    }
  } catch {
    // Fall back to memory.
  }
  return memory;
}

export function hasClipboard(): boolean {
  return readClipboard() !== null;
}

/**
 * Materializes a clip: fresh ids, connections remapped between the copies,
 * positions shifted by the offset. Names are kept (Node-RED does the same).
 */
export function materializeClip(clip: Clip, offset: { x: number; y: number }, newId: () => string): PasteResult {
  const idMap = new Map<string, string>();
  const nodes = clip.nodes.map((node) => {
    const id = newId();
    idMap.set(node.id, id);
    return {
      ...structuredClone(node),
      id,
      position: { x: (node.position?.x ?? 0) + offset.x, y: (node.position?.y ?? 0) + offset.y },
    };
  });
  const connections = clip.connections
    .filter((c) => idMap.has(c.sourceNode) && idMap.has(c.targetNode))
    .map((c) => ({
      id: newId(),
      sourceNode: idMap.get(c.sourceNode)!,
      sourcePort: c.sourcePort,
      targetNode: idMap.get(c.targetNode)!,
      targetPort: c.targetPort,
    }));
  return { nodes, connections };
}
