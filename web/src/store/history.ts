import type { FlowNode, NodeConnection } from '../types/flow';

/** The editable part of a flow: what undo/redo and autosave operate on. */
export interface FlowDocument {
  nodes: Record<string, FlowNode>;
  connections: NodeConnection[];
}

export const HISTORY_LIMIT = 100;

export interface History {
  past: FlowDocument[];
  future: FlowDocument[];
}

export const emptyHistory = (): History => ({ past: [], future: [] });

/** Records `current` as the state to return to on undo and clears redo. */
export function pushHistory(history: History, current: FlowDocument): History {
  const past = history.past.length >= HISTORY_LIMIT ? history.past.slice(1) : history.past;
  return { past: [...past, current], future: [] };
}

export function canUndo(history: History): boolean {
  return history.past.length > 0;
}

export function canRedo(history: History): boolean {
  return history.future.length > 0;
}

/** Returns the document to restore and the history after undoing, or null. */
export function undoHistory(history: History, current: FlowDocument): { document: FlowDocument; history: History } | null {
  if (history.past.length === 0) return null;
  const document = history.past[history.past.length - 1];
  return {
    document,
    history: { past: history.past.slice(0, -1), future: [current, ...history.future] },
  };
}

/** Returns the document to restore and the history after redoing, or null. */
export function redoHistory(history: History, current: FlowDocument): { document: FlowDocument; history: History } | null {
  if (history.future.length === 0) return null;
  const [document, ...future] = history.future;
  return {
    document,
    history: { past: [...history.past, current], future },
  };
}
