/**
 * Guards for the editor's window-wide keyboard shortcuts (canvas keys,
 * undo/redo/deploy/clipboard, palette search). A shortcut must not act on
 * the canvas the user cannot see or is not working in: not while typing in
 * a field, not while focus is inside a dialog (the edit tray, quick add, a
 * modal), and not at all while a modal dialog covers the editor.
 */

/** True for targets that take text input (the browser keeps its own keys there). */
export function isEditableTarget(target: EventTarget | null): boolean {
  if (!(target instanceof HTMLElement)) return false;
  const tag = target.tagName;
  return tag === 'INPUT' || tag === 'TEXTAREA' || tag === 'SELECT' || target.isContentEditable === true;
}

/** True when focus is inside a dialog or a modal dialog covers the editor. */
export function inDialog(event: Pick<KeyboardEvent, 'target'>, doc: Document = document): boolean {
  if (event.target instanceof Element && event.target.closest('[role="dialog"]')) return true;
  return doc.querySelector('[aria-modal="true"]') !== null;
}

/** True when a key press belongs to a field, a dialog or a modal and not to the editor. */
export function editorKeysBlocked(event: Pick<KeyboardEvent, 'target'>, doc: Document = document): boolean {
  return isEditableTarget(event.target) || inDialog(event, doc);
}
