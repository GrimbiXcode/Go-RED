import type { FlowConfig, FlowNode, NodeConnection } from '../types/flow';

/**
 * Pull-and-reapply for a working copy that fell behind the server.
 *
 * Saves are ordered by the server's revision. When another session saved
 * first, this tab pulls that version and reapplies only what it changed
 * itself since its base version, field by field. Fields it did not touch
 * take the server's value, so nothing the other session did is lost; a
 * field both changed takes this tab's value, because this save comes later.
 * A node deleted on either side stays deleted. There is no conflict to ask
 * about, so no dialog.
 */

/** The part of a flow the editor edits and autosaves. */
export interface FlowContent {
  name: string;
  description: string;
  nodes: Record<string, FlowNode>;
  connections: NodeConnection[];
  config: FlowConfig;
}

/**
 * A stable form for comparing values that went through the server: object
 * keys sorted, and absent, null, '', false and empty collections all the
 * same (the server omits empty names, descriptions and ports, sends
 * `disabled: false` and may answer a missing config with null).
 */
function canonical(value: unknown): unknown {
  if (value === undefined || value === null || value === '' || value === false) return undefined;
  if (Array.isArray(value)) {
    const items = value.map(canonical);
    return items.length === 0 ? undefined : items;
  }
  if (typeof value === 'object') {
    const out: Record<string, unknown> = {};
    for (const key of Object.keys(value as Record<string, unknown>).sort()) {
      const item = canonical((value as Record<string, unknown>)[key]);
      if (item !== undefined) out[key] = item;
    }
    return Object.keys(out).length === 0 ? undefined : out;
  }
  return value;
}

/** Whether two values are the same once normalized (see canonical). */
export function sameValue(a: unknown, b: unknown): boolean {
  return JSON.stringify(canonical(a)) === JSON.stringify(canonical(b));
}

/** Local's value if this tab changed it since base, else remote's. */
function pick<T>(base: T, local: T, remote: T): T {
  return sameValue(local, base) ? remote : local;
}

function rebaseNode(base: FlowNode, local: FlowNode, remote: FlowNode): FlowNode {
  const out: Record<string, unknown> = {};
  const keys = new Set([...Object.keys(base), ...Object.keys(local), ...Object.keys(remote)]);
  for (const key of keys) {
    const value = pick((base as any)[key], (local as any)[key], (remote as any)[key]);
    if (value !== undefined) out[key] = value;
  }
  return out as unknown as FlowNode;
}

/** A connection's identity: its endpoints, with the default port names filled in. */
export function connectionKey(c: NodeConnection): string {
  return `${c.sourceNode}:${c.sourcePort || 'output'}->${c.targetNode}:${c.targetPort || 'input'}`;
}

function rebaseConnections(base: NodeConnection[], local: NodeConnection[], remote: NodeConnection[], nodes: Record<string, FlowNode>): NodeConnection[] {
  const inBase = new Set(base.map(connectionKey));
  const inLocal = new Map(local.map((c) => [connectionKey(c), c]));
  const inRemote = new Set(remote.map(connectionKey));
  const out: NodeConnection[] = [];
  const seen = new Set<string>();
  const keep = (c: NodeConnection) => {
    const key = connectionKey(c);
    if (seen.has(key) || !nodes[c.sourceNode] || !nodes[c.targetNode]) return;
    seen.add(key);
    out.push(c);
  };
  // Remote's wires, unless this tab removed one that was in the base.
  for (const c of remote) {
    if (inLocal.has(connectionKey(c)) || !inBase.has(connectionKey(c))) keep(inLocal.get(connectionKey(c)) ?? c);
  }
  // This tab's new wires, unless remote removed one that was in the base.
  for (const c of local) {
    const key = connectionKey(c);
    if (!inBase.has(key) || inRemote.has(key)) keep(c);
  }
  return out;
}

/** Remote with this tab's changes since base reapplied on top. */
export function rebase(base: FlowContent, local: FlowContent, remote: FlowContent): FlowContent {
  const nodes: Record<string, FlowNode> = {};
  for (const id of new Set([...Object.keys(local.nodes), ...Object.keys(remote.nodes)])) {
    const b = base.nodes[id];
    const l = local.nodes[id];
    const r = remote.nodes[id];
    if (!b) {
      // Added since base: keep it from whichever side added it.
      const added = l ?? r;
      if (added) nodes[id] = l && r ? rebaseNode({} as FlowNode, l, r) : added;
    } else if (l && r) {
      nodes[id] = rebaseNode(b, l, r);
    }
    // In base but gone from either side: deleted, and deletion wins.
  }

  const config = {} as Record<string, unknown>;
  for (const key of new Set([...Object.keys(base.config ?? {}), ...Object.keys(local.config ?? {}), ...Object.keys(remote.config ?? {})])) {
    config[key] = pick((base.config as any)?.[key], (local.config as any)?.[key], (remote.config as any)?.[key]);
  }

  return {
    name: pick(base.name, local.name, remote.name),
    description: pick(base.description, local.description, remote.description),
    nodes,
    connections: rebaseConnections(base.connections, local.connections, remote.connections, nodes),
    config: config as unknown as FlowConfig,
  };
}

/** Whether two contents are the same once normalized. */
export function sameContent(a: FlowContent, b: FlowContent): boolean {
  return (
    sameValue(a.name, b.name) &&
    sameValue(a.description, b.description) &&
    sameValue(a.nodes, b.nodes) &&
    sameValue(a.config, b.config) &&
    sameValue(a.connections.map(connectionKey).sort(), b.connections.map(connectionKey).sort())
  );
}
