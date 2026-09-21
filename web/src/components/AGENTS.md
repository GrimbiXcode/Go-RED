# web/src/components

Every component in this directory, what it owns and where its state comes
from. Components render store state and call store actions or
`store/editorActions`; they keep local state only for transient UI (open
menus, drag, input drafts). `index.ts` re-exports the ones other modules
import.

## Shell

| Component | Role |
|---|---|
| `FlowEditor.tsx` | The page. Reads `:flowId`, selects the flow, composes `Header`, `FlowTabs`, `NodePalette`, `FlowCanvas` (inside `ReactFlowProvider`), `SidebarTabs` with `Sidebar` and `DebugPanel`, `NodeEditTray`, `StatusBar`, `ExportModal`, `ImportModal`, `ShortcutHelp`, `NoFlowsState`; mounts `useEditorShortcuts`; on tray save prunes wires to removed output ports (`schema/ports`). |
| `Header.tsx` | Top bar: deploy, deploy all, stop, undo/redo, export/import and the main menu (language, theme, shortcuts). Props-driven (`HeaderProps`); `GoRedMark` is the logo. |
| `FlowTabs.tsx` | Flow tabs: click selects, double-click renames inline, drag reorders (`reorderFlows`), right-click menu (rename, duplicate, delete); `isModified(flow)` drives the dirty dot. |
| `StatusBar.tsx` | Bottom bar: `WebSocketStatus`, save state (`save-state` with `data-state`), node and connection counts. |
| `SidebarTabs.tsx` | Right-hand icon strip plus panel; clicking the active tab collapses the panel. Exports `InfoTabIcon`, `DebugTabIcon`. |
| `Sidebar.tsx` | The Info tab: flow details, or the selected node's details, status, counters, settings (`settingsRows`), description and help (`renderMarkdown`). |
| `DebugPanel.tsx` | The Debug tab: live feed from `runtimeStore` with node/text filter, pause and clear; exports `formatPayload`, `filterDebug`, `DebugEntry`. |
| `EmptyStates.tsx` | `NoFlowsState` (create or import) and the empty-flow illustration. |
| `ToastNotification.tsx` | `ToastHost` renders `notificationStore` toasts. |
| `TokenPrompt.tsx` | Dialog shown while `useAuthStore.required`; stores the token and reloads. |
| `WebSocketStatus.tsx` | Connection dot or label from `useWebSocket().state`. |

## Canvas

| Component | Role |
|---|---|
| `FlowCanvas.tsx` | The `@xyflow/react` canvas for one flow: converts `flow.nodes`/`connections` to canvas nodes and edges (`flowNodeToCanvasNode`, `connectionToEdge`), merges store updates without disturbing a drag or the selection (`mergeCanvasNodes`), handles palette drop, connect, delete, Ctrl+A, arrow-key nudge, zoom, node and pane context menus (`ContextMenu`), double-click quick add (`QuickAdd`) and dropping a node onto a wire (`edgeIdAt` plus `insertNodeOnEdge`/`spliceNodeIntoEdge`). Node type map: `default: NodeComponent`, `inject: InjectNode`, `debug: DebugNode`. |
| `canvasTypes.ts` | `CanvasNodeData` (`label`, `node`, `metadata`, `outputs`, `flowId`) and `CanvasNode`. |
| `NodeShell.tsx` | Visual shell of a node: category color through `--node-color`, icon well, label, optional control, Node-RED style status line; geometry lives in the `.gr-node*` CSS rules. |
| `NodeHandles.tsx` | Input and output handles from `outputs`, evenly spaced, labels for multi-output nodes (`port-label`). |
| `NodeComponent.tsx` | Generic node: `NodeShell` plus `NodeHandles`. |
| `InjectNode.tsx` | Inject node whose icon well is the trigger button (sends `message:send` through `wsClient`). |
| `DebugNode.tsx` | Debug node rendering; no interactive control. |
| `CategoryIcon.tsx` | `CategoryIcon` and `NodeIcon` (icon by name from `icons.ts`, category fallback). |
| `icons.ts` | The Lucide icon vocabulary a node type can name in `NodeMetadata.icon`; only listed icons ship. |
| `ContextMenu.tsx` | Positioned menu (`context-menu`, items `context-<id>`). |
| `QuickAdd.tsx` | Search popover on canvas double-click (`quick-add`, `quick-add-<type>`). |

## Palette, tray, dialogs

| Component | Role |
|---|---|
| `NodePalette.tsx` | Left palette grouped by category, search (`/` focuses it), collapsed categories remembered in `localStorage`; `filterNodeTypes`; items are `palette-node-<type>` and draggable. |
| `NodeEditTray.tsx` | Slide-in tray for `editorStore.configNodeId` with tabs `properties`, `description`, `appearance`; renders `PropertyFields` from `configSchema`, validates on every change (`validateConfig`), Done (`config-done`) only while valid; `onSave(patch: NodePatch)`. |
| `ExportModal.tsx` | Download as Go-RED JSON or Node-RED `flows.json` (`api.exportFlow`). |
| `ImportModal.tsx` | File picker with preview (`previewNodeRed` lists unsupported types), then `api.importFlow`. |
| `ShortcutHelp.tsx` | Keyboard reference opened with `?` (`shortcut-help`). |
| `JsonTree.tsx` | Collapsible JSON view used by debug entries. |

## `config/` (edit-tray widgets)

- `types.ts`: `WidgetProps` (`id`, `property`, `value`, `onChange`, `error`,
  `context`, `setInvalid`), `WidgetContext`, shared input class names.
- `widgets.tsx`: one component per `Widget` and `widgetRegistry` (`text`,
  `textarea`, `number`, `boolean`, `select`, `typedInput`, `code`, `list`,
  `keyValue`, `credential`, `duration`, `json`, `stringList`, `nodeSelect`).
- `PropertyFields.tsx`: renders a schema's resolved, grouped, visible
  properties with their widgets and errors (`field-<id>`, `field-error`).
- `ListWidget.tsx`: arrays of sub-schemas (`list-<id>`, `list-add-<id>`,
  `list-item-<id>-<index>`). `TypedInputWidget.tsx`: Node-RED style
  `{ type, value }` input. `CodeEditor.tsx`: CodeMirror 6 with `javascript`,
  `json`, `mustache`, `text` (`code-editor`).

Adding a widget: extend `Widget` in `schema/properties.ts`, add the component
and registry entry here, mirror the constant in `internal/registry/schema.go`, cover it
in `test/editTray.test.tsx` and show it in `pages/StyleGuide.tsx`.

## Conventions

- Props interfaces are exported next to the component (`HeaderProps`,
  `NodeEditTrayProps`, ...). Store-bound components take at most ids.
- Text through `useTranslation()`; every new key goes into `i18n/en.ts` and
  `i18n/de.ts`.
- Colors only through the semantic utilities (`bg-panel`, `text-muted`,
  `border-line`, `bg-accent`, `bg-cat-*`) so both themes work without
  per-component code.
- Dialogs use `role="dialog"` with an `aria-label`, menus `role="menu"` and
  `menuitem`, tabs `role="tab"` with `aria-selected`; the e2e suite selects
  by these roles and by `data-testid`. Keep both when refactoring.
- Node help and descriptions are Markdown rendered by `utils/markdown.ts`
  (DOMPurify); never put unsanitized input into `dangerouslySetInnerHTML`.
- Canvas behavior that does not need the DOM lives in exported pure helpers
  (`flowNodeToCanvasNode`, `mergeCanvasNodes`, `filterNodeTypes`,
  `previewNodeRed`, `settingsRows`, ...) so it can be unit-tested; the rest
  is covered by the e2e suite.
- Component tests: `test/components.test.tsx`, `test/editTray.test.tsx`,
  `test/debugPanel.test.tsx`, `test/flowCanvas.test.tsx`, `test/auth.test.tsx`.

## How a node gets on screen

1. `FlowEditor` selects the flow; `useFlowStore` holds `flow.nodes` (a
   record keyed by id) and `nodeTypes`.
2. `FlowCanvas` builds a `NodeRegistry` (type to `NodeMetadata`) and maps
   each `FlowNode` through `flowNodeToCanvasNode`, which resolves the label,
   the metadata and the current output ports (`schema/ports.outputPortsFor`,
   so a switch with three rules shows three handles).
3. xyflow renders the node with the component from the type map
   (`NodeComponent`, `InjectNode` or `DebugNode`), each of which composes
   `NodeShell` (look, status line from `useRuntimeStore`) and `NodeHandles`.
4. Store updates (autosave results, server events) are folded in with
   `mergeCanvasNodes`, which keeps the on-screen position of a node being
   dragged and the current selection.
5. Selection goes to `editorStore.selectedNodeIds`; a double-click sets
   `configNodeId`, which opens `NodeEditTray`; its `onSave` calls
   `updateNode`, and the cycle starts again from the store.

Runtime data (status, counters, debug entries) never lives on the canvas
node; components read it per node with `selectNodeStatus`/`selectNodeMetrics`
so a status event re-renders one node, not the canvas.
