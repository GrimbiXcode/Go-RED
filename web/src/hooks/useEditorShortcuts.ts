import { useEffect } from 'react';

export interface EditorShortcutHandlers {
  undo: () => void;
  redo: () => void;
  deploy?: () => void;
  exportFlow?: () => void;
  help?: () => void;
  copy?: () => void;
  paste?: () => void;
  duplicate?: () => void;
}

function isEditableTarget(target: EventTarget | null): boolean {
  if (!(target instanceof HTMLElement)) return false;
  const tag = target.tagName;
  return tag === 'INPUT' || tag === 'TEXTAREA' || tag === 'SELECT' || target.isContentEditable;
}

/**
 * Global editor keyboard shortcuts: Ctrl/Cmd+Z undoes, Ctrl/Cmd+Shift+Z and
 * Ctrl/Cmd+Y redo, Ctrl/Cmd+S deploys, Ctrl/Cmd+E exports, Ctrl/Cmd+C/V/D
 * copy, paste and duplicate the selection, "?" opens the shortcut help.
 * All are ignored while typing in a form field (so the browser keeps its
 * own copy/paste there). Canvas-local keys (arrows, zoom, select all) live
 * in FlowCanvas.
 */
export function useEditorShortcuts({ undo, redo, deploy, exportFlow, help, copy, paste, duplicate }: EditorShortcutHandlers): void {
  useEffect(() => {
    const onKeyDown = (event: KeyboardEvent) => {
      if (isEditableTarget(event.target)) return;
      const mod = event.ctrlKey || event.metaKey;
      if (event.key === '?' && !mod && !event.altKey) {
        event.preventDefault();
        help?.();
        return;
      }
      if (!mod || event.altKey) return;
      const key = event.key.toLowerCase();
      const run = (action?: () => void) => {
        if (!action) return;
        event.preventDefault();
        action();
      };
      switch (key) {
        case 'z':
          run(event.shiftKey ? redo : undo);
          break;
        case 'y':
          run(redo);
          break;
        case 's':
          run(deploy);
          break;
        case 'e':
          run(exportFlow);
          break;
        case 'c':
          run(copy);
          break;
        case 'v':
          run(paste);
          break;
        case 'd':
          run(duplicate);
          break;
      }
    };
    window.addEventListener('keydown', onKeyDown);
    return () => window.removeEventListener('keydown', onKeyDown);
  }, [undo, redo, deploy, exportFlow, help, copy, paste, duplicate]);
}
