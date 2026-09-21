# web/src

Module map for the editor source. `../AGENTS.md` covers stack, scripts and
build; `components/`, `hooks/`, `types/` and `test/` have their own files.
Everything here was read from the code; if the code and this file disagree,
the code wins and this file needs a fix.

## Entry points

- `index.tsx`: `initTheme()`, imports `./i18n` and `./styles/tailwind.css`,
  renders `App` inside `BrowserRouter` and `React.StrictMode`.
- `App.tsx`: `useEffect(() => bindServerEvents(), [])`; routes `/` and
  `/flow/:flowId` to `FlowEditor`, `/styleguide` to `StyleGuide`, anything
  else redirects to `/`. `ToastHost` and `TokenPrompt` are mounted once here.

## Stores (`store/`)

State that outlives a component lives in Zustand stores; components
subscribe with selectors and call store actions. `store/index.ts` re-exports
everything.

### `flowStore.ts` (`useFlowStore`)

The document store and the only write path.

- State: `flows: FlowSummary[]`, `nodeTypes: NodeMetadata[]`,
  `selectedFlowId`, `flow: Flow | null` (the working copy), `history`
  (`past`/`future`, `HISTORY_LIMIT` 100, in `history.ts`), `saveState`
  (`idle | pending | saving | error`), `saveError`, and three counters:
  `editVersion` (bumped per edit), `savedVersion`, `deployedVersion`.
- Loading: `loadFlows`, `loadNodeTypes`, `selectFlow(id)`.
- Flow lifecycle: `createFlow`, `deleteFlow`, `renameFlow`, `renameFlowById`,
  `duplicateFlow`, `reorderFlows` (writes `Flow.order`), `deploy`,
  `undeploy`, `flushSave({ keepalive })`.
- Document edits, each one undo step: `addNode`, `addNodes`, `removeNodes`,
  `moveNodes`, `nudgeNodes`, `updateNode(id, patch, { removeConnections })`,
  `insertNodeOnEdge`, `spliceNodeIntoEdge`, `addConnection`,
  `removeConnections`, `undo`, `redo`.
- Server events: `applyFlowStatus`, `applyFlowList`, `applyFlowDeleted`.
- Selectors: `selectCanUndo`, `selectCanRedo`, `selectCanDeploy`,
  `selectCanUndeploy`, `hasUndeployedChanges`.

Autosave: every document edit schedules a debounced (`AUTOSAVE_DELAY_MS` =
600) `PUT /api/flows/{id}` of the whole flow through `api.updateFlow`. The
server stores that as the draft; the running instance changes only on
`deploy`, which flushes a pending save first. "Modified" therefore means
"draft differs from what runs", never "unsaved".
`__resetFlowStoreForTests()` clears timers and state.

### `editorStore.ts` (`useEditorStore`)

UI-only, never persisted: `selectedNodeIds`, `configNodeId` (open tray),
`sidebarTab` (`'info' | 'debug' | null`), `showExport`, `showImport`,
`showShortcuts`, `pendingSelection` (applied by the canvas after a paste).
`resetForFlow()` forgets per-flow UI state when another flow opens.

### `runtimeStore.ts` (`useRuntimeStore`)

Server-owned runtime data keyed by flow id: `nodeStatus`, `metrics`, `debug`
(at most `DEBUG_LIMIT` = 500 entries, a repeat of the last id is ignored),
`subscribed`. Written only by `bindServerEvents`; a flow that stops loses
its node status and metrics. Selectors: `selectNodeStatus`,
`selectNodeMetrics`, `selectDebug`.

### `notificationStore.ts`

Toasts. `notify(type, message, duration?)` works outside React so stores and
the event binding can raise one; `ToastHost` renders them.

### `bindServerEvents.ts`

Subscribes `wsClient` to `flow:status`, `flow:list`, `flow:delete`,
`flow:snapshot`, `node:status`, `debug:message`, `flow:metrics` and `error`,
sends `subscribe`/`unsubscribe` as `selectedFlowId` changes, re-subscribes
after a reconnect and calls `wsClient.connect()`. Returns a teardown
function. A new server event is handled here and nowhere else.

### `editorActions.ts`

Commands shared by shortcuts, context menus and the header, reading the
stores directly: `copySelection`, `pasteClipboard(at?)`,
`duplicateSelection`, `deleteSelection`, `toggleSelectionDisabled`,
`alignSelection(mode)`, `distributeSelection(axis)`,
`autoLayoutFlow(nodeTypes)`.

## `lib/`

- `wsClient.ts`: `WebSocketClient` (`connect`, `send(type, data, { timeout })`,
  `sendRaw`, `subscribe(type, cb)` returning the unsubscribe function,
  `unsubscribe`, `reconnect`, `close`, `getState`, `onStateChange`) and the
  singleton `wsClient`. `ConnectionState` is
  `{ connected, connecting, error, attempts }`.
- `auth.ts`: `getToken`/`setToken`/`clearToken` (`localStorage`
  `go-red.token`), `authHeaders()`, `wsProtocols()`, `useAuthStore`
  (`required`), `authRequired()`.
- `theme.ts`: `useThemeStore`, `initTheme()`, `applyTheme`; preference
  `system | light | dark` in `localStorage` `go-red.theme`, written as
  `data-theme` on `<html>`.
- `clipboard.ts`: `clipFromSelection`, `writeClipboard`/`readClipboard`
  (in memory plus `localStorage`), `materializeClip` (fresh ids, offset).
- `layout.ts`: `NODE_WIDTH` 140, `NODE_HEIGHT` 36, `GRID` 20, `snap`,
  `alignNodes`, `distributeNodes`, `autoLayout` (dagre, left to right).

## `schema/` (the node config contract)

Node types describe their configuration in `NodeMetadata.configSchema`
(`Schema`/`Property` in `types/generated.ts`, produced by
`internal/registry`). The editor never hard-codes a node's fields.

- `properties.ts`: `Widget` (`text`, `textarea`, `number`, `boolean`,
  `select`, `typedInput`, `code`, `list`, `keyValue`, `credential`,
  `duration`, `json`, `stringList`, `nodeSelect`; mirrors the `Widget*`
  constants in `internal/registry/schema.go`), `resolveProperties` (widget, label,
  group, order), `groupProperties`, `withDefaults`, `isVisible`
  (`visibleWhen`), `optionsOf`, `isRefType`.
- `validate.ts`: `validateConfig(schema, config, t)` returning
  `{ key: message }`; `isEmptyValue`.
- `ports.ts`: `outputPortsFor`/`outputPortIds` (dynamic outputs from
  `outputsFrom`, labels via `renderPortLabel`), `connectionsOnMissingPorts`
  (wires to prune after a config change).

`components/NodeEditTray.tsx` and `components/config/` render from this;
`FlowCanvas` uses `ports.ts` for the handles.

## `i18n/`

`index.ts` initializes i18next with `en` and `de` (`detectLanguage`: stored
`go-red.language`, else the browser language, else English; `setLanguage`).
Keys are grouped by surface: `app`, `language`, `header`, `context`,
`quickAdd`, `shortcuts`, `theme`, `empty`, `tabs`, `palette`, `canvas`,
`sidebar`, `debug`, `config`, `validation`, `widgets`, `export`, `import`,
`status`, `toast`, `flowStatus`, `auth`, `common`. Add every key to both
files.

## `styles/`

`tailwind.css`: font imports, the Tailwind layers, the design tokens as CSS
variables on `:root`/`[data-theme='light']` and `[data-theme='dark']`
(backgrounds, foregrounds, lines, accent, danger, warn, ok, wire, one fill
per node category plus derived `-soft`/`-text` tints) and the `.gr-node*`
rules for canvas nodes. `tailwind.config.js` maps the variables to
utilities and sets the 11–24 px type scale. `test/tokens.test.ts` checks
contrast and that `utils/nodeCategories.ts` and the CSS agree.
`reactflow-overrides.css` restyles xyflow's own elements.

## `utils/`

- `api.ts`: the REST client (see `../AGENTS.md`), `getWebSocketUrl`,
  `ExportFormat`, `ImportResult`, `generateId`, `sleep`.
- `markdown.ts`: `renderMarkdown` (headings, lists, fenced code, bold,
  italics, inline code, links) sanitized with DOMPurify.
- `nodeCategories.ts`: `CATEGORY_ORDER`, `CATEGORY_COLORS`,
  `getCategoryColor`, `readableTextColor`, `contrastRatio`, `sortCategories`.

## `pages/StyleGuide.tsx`

Living style guide at `/styleguide`: tokens in both themes, node shells for
every category, widgets and debug entries. Extend it when adding a visual
element.

## Working here

- Read the store you touch before adding state; most "new state" belongs in
  an existing store or is a selector over one.
- A change that adds a server event or field goes in this order:
  `go generate ./internal/dto/...`, `bindServerEvents.ts`, the store, the
  component. `types/AGENTS.md` has the full checklist.
- Store and helper tests are plain Vitest files in `test/`; `test/AGENTS.md`
  shows the store reset and API mock pattern.
