import { create } from 'zustand';

export type SidebarTab = 'info' | 'debug';

interface EditorState {
  /** IDs of the nodes currently selected on the canvas. */
  selectedNodeIds: string[];
  /** Node whose configuration tray is open, if any. */
  configNodeId: string | null;
  /** Open sidebar tab, or null when the sidebar is collapsed. */
  sidebarTab: SidebarTab | null;
  showExport: boolean;
  showImport: boolean;
  showShortcuts: boolean;
  /** Nodes the canvas should select on its next render (after a paste). */
  pendingSelection: string[] | null;

  setSelection: (ids: string[]) => void;
  clearSelection: () => void;
  openConfig: (nodeId: string) => void;
  closeConfig: () => void;
  setSidebarTab: (tab: SidebarTab | null) => void;
  toggleSidebarTab: (tab: SidebarTab) => void;
  setShowExport: (show: boolean) => void;
  setShowImport: (show: boolean) => void;
  setShowShortcuts: (show: boolean) => void;
  requestSelection: (ids: string[]) => void;
  clearPendingSelection: () => void;
  /** Forget UI state that belongs to a flow when another flow is opened. */
  resetForFlow: () => void;
}

const sameIds = (a: string[], b: string[]) => a.length === b.length && a.every((id, i) => id === b[i]);

/** UI-only editor state: selection, open panels, dialogs. Never persisted. */
export const useEditorStore = create<EditorState>((set) => ({
  selectedNodeIds: [],
  configNodeId: null,
  sidebarTab: 'info',
  showExport: false,
  showImport: false,
  showShortcuts: false,
  pendingSelection: null,

  setSelection: (ids) =>
    set((state) => (sameIds(state.selectedNodeIds, ids) ? state : { selectedNodeIds: ids })),
  clearSelection: () => set((state) => (state.selectedNodeIds.length === 0 ? state : { selectedNodeIds: [] })),
  openConfig: (nodeId) => set({ configNodeId: nodeId }),
  closeConfig: () => set({ configNodeId: null }),
  setSidebarTab: (tab) => set({ sidebarTab: tab }),
  toggleSidebarTab: (tab) => set((state) => ({ sidebarTab: state.sidebarTab === tab ? null : tab })),
  setShowExport: (show) => set({ showExport: show }),
  setShowImport: (show) => set({ showImport: show }),
  setShowShortcuts: (show) => set({ showShortcuts: show }),
  requestSelection: (ids) => set({ pendingSelection: ids, selectedNodeIds: ids }),
  clearPendingSelection: () => set({ pendingSelection: null }),
  resetForFlow: () => set({ selectedNodeIds: [], configNodeId: null, pendingSelection: null }),
}));
