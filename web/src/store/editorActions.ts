import i18n from '../i18n';
import { useFlowStore, generateId } from './flowStore';
import { useEditorStore } from './editorStore';
import { notify } from './notificationStore';
import { clipFromSelection, materializeClip, readClipboard, writeClipboard } from '../lib/clipboard';
import { alignNodes, autoLayout, distributeNodes, type AlignMode, type DistributeAxis } from '../lib/layout';
import type { NodeMetadata } from '../types/node';

/**
 * Editing commands shared by the keyboard shortcuts, the context menus and
 * the header: they read the stores directly so any surface can trigger
 * them without prop drilling.
 */

const t = (key: string, options?: Record<string, unknown>) => i18n.t(key, options);

function selection(): string[] {
  return useEditorStore.getState().selectedNodeIds;
}

/** Copies the selected nodes and the wires between them to the editor clipboard. */
export function copySelection(): number {
  const { flow } = useFlowStore.getState();
  if (!flow) return 0;
  const clip = clipFromSelection(flow.nodes, flow.connections, selection());
  if (!clip) return 0;
  writeClipboard(clip);
  notify('info', t('toast.copied', { count: clip.nodes.length }));
  return clip.nodes.length;
}

/** Pastes the clipboard at a position (top-left of the pasted group) or slightly offset from the original. */
export function pasteClipboard(at?: { x: number; y: number }): string[] {
  const { flow, addNodes } = useFlowStore.getState();
  const clip = readClipboard();
  if (!flow || !clip || clip.nodes.length === 0) {
    notify('info', t('toast.nothingToPaste'));
    return [];
  }
  let offset = { x: 40, y: 40 };
  if (at) {
    const minX = Math.min(...clip.nodes.map((n) => n.position.x));
    const minY = Math.min(...clip.nodes.map((n) => n.position.y));
    offset = { x: at.x - minX, y: at.y - minY };
  }
  const { nodes, connections } = materializeClip(clip, offset, generateId);
  const ids = addNodes(nodes, connections);
  if (ids.length > 0) {
    useEditorStore.getState().requestSelection(ids);
    notify('success', t('toast.pasted', { count: ids.length }));
  }
  return ids;
}

/** Copies and pastes the selection in one go, offset from the original. */
export function duplicateSelection(): string[] {
  const { flow, addNodes } = useFlowStore.getState();
  if (!flow) return [];
  const clip = clipFromSelection(flow.nodes, flow.connections, selection());
  if (!clip) return [];
  const { nodes, connections } = materializeClip(clip, { x: 40, y: 40 }, generateId);
  const ids = addNodes(nodes, connections);
  if (ids.length > 0) useEditorStore.getState().requestSelection(ids);
  return ids;
}

export function deleteSelection(): void {
  const ids = selection();
  if (ids.length > 0) useFlowStore.getState().removeNodes(ids);
}

/** Enables or disables every selected node (disabled when any is enabled). */
export function toggleSelectionDisabled(): void {
  const { flow, updateNode } = useFlowStore.getState();
  if (!flow) return;
  const ids = selection().filter((id) => id in flow.nodes);
  if (ids.length === 0) return;
  const disable = ids.some((id) => !flow.nodes[id].disabled);
  for (const id of ids) updateNode(id, { disabled: disable });
}

export function alignSelection(mode: AlignMode): void {
  const { flow, moveNodes } = useFlowStore.getState();
  if (!flow) return;
  const nodes = selection().map((id) => flow.nodes[id]).filter(Boolean);
  moveNodes(alignNodes(nodes, mode));
}

export function distributeSelection(axis: DistributeAxis): void {
  const { flow, moveNodes } = useFlowStore.getState();
  if (!flow) return;
  const nodes = selection().map((id) => flow.nodes[id]).filter(Boolean);
  moveNodes(distributeNodes(nodes, axis));
}

/** Lays the whole flow out left to right. */
export function autoLayoutFlow(nodeTypes: NodeMetadata[]): void {
  const { flow, moveNodes } = useFlowStore.getState();
  if (!flow) return;
  const byType = new Map(nodeTypes.map((nt) => [nt.type, nt]));
  const hasPorts = (node: { type: string }) => {
    const meta = byType.get(node.type);
    if (!meta) return true;
    return (meta.inputs?.length ?? 0) > 0 || (meta.outputs?.length ?? 0) > 0 || !!meta.outputsFrom;
  };
  moveNodes(autoLayout(Object.values(flow.nodes), flow.connections, hasPorts));
}
