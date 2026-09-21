# web/src/test

Vitest unit and component tests for the editor. End-to-end tests live in
`web/e2e/` and run against the real Go server; they are described at the
end because they share the `data-testid` contract with the components.

## Running

- `npm test` starts Vitest in watch mode; `npx vitest run` runs once;
  `npm run test:coverage` adds v8 coverage.
- `vitest.config.ts`: jsdom environment, `globals: true`,
  `setupFiles: ./src/test/setup.ts`, include pattern
  `src/**/*.{test,spec}.{ts,tsx,...}`. Tests live here; the files still
  import `describe`/`it`/`expect` from `vitest` explicitly.

## `setup.ts`

Loaded before every file:

- `@testing-library/jest-dom` matchers; `cleanup()` and
  `vi.restoreAllMocks()` after each test.
- Imports `../i18n`, so components render the real English strings and
  tests can query by visible text.
- A `ResizeObserver` stub (xyflow measures nodes with it; jsdom has none).
- `Range.prototype.getClientRects`/`getBoundingClientRect` stubs for
  CodeMirror.
- A `MockWebSocket` on `globalThis.WebSocket`: opens asynchronously, echoes
  `send()` back through `onmessage`, `close()` fires `onclose`.
  `wsClient.test.ts` installs its own, more controllable double.
- A `globalThis.fetch` mock resolving `{ success: true, data: {} }`. Store
  tests do not rely on it; they mock `utils/api` instead.

## Files

| File | Covers |
|---|---|
| `flowStore.test.ts` | document editing, undo/redo, one autosave per burst of edits, deploy flushing the save, failed saves, switching flows, server events |
| `editing.test.ts` | `lib/clipboard`, `lib/layout`, `store/editorActions` (copy/paste/duplicate, align, distribute, auto-layout, enable/disable), inserting and splicing nodes on wires, nudging, flow duplicate and tab reorder |
| `runtimeStore.test.ts` | snapshot, node status, debug ring buffer, metrics, forgetting a flow |
| `wsClient.test.ts` | `WebSocketClient`: single socket, queue and flush, typed and wildcard listeners, exponential reconnect, `close()` |
| `schema.test.ts` | `schema/properties`, `schema/validate`, `schema/ports`, `utils/markdown` |
| `tokens.test.ts` | design tokens: contrast of text/background pairs, category colors in sync between `styles/tailwind.css` and `utils/nodeCategories.ts` |
| `types.test.ts` | shapes of the generated wire types (`Flow`, `FlowNode`, `NodeConnection`, `FlowStatus`, `FlowConfig`, `NodeStatus`) |
| `utils.test.tsx` | `generateId`, `sleep`, `getWebSocketUrl` |
| `components.test.tsx` | `Header`, `FlowTabs`, `NodePalette`, `StatusBar` |
| `editTray.test.tsx` | `NodeEditTray` with switch, http-in and function schemas: widgets, visibility, validation, dynamic ports |
| `debugPanel.test.tsx` | `formatPayload`, `filterDebug`, `DebugPanel` rendering from `runtimeStore` |
| `flowCanvas.test.tsx` | `flowNodeToCanvasNode`, `connectionToEdge`, `mergeCanvasNodes` |
| `auth.test.tsx` | token storage, headers and subprotocols; `TokenPrompt` |

## Patterns

Store tests mock the REST client at module level and reset the store before
each test (from `flowStore.test.ts`):

```ts
vi.mock('../utils/api', () => ({ fetchFlow: vi.fn(), updateFlow: vi.fn(), deployFlow: vi.fn(), /* ... */ }));

beforeEach(() => {
  __resetFlowStoreForTests();
  vi.useFakeTimers();
  vi.mocked(api.updateFlow).mockImplementation(async (id, data) => ({ ...makeFlow(), ...data, id }));
});
```

Then call actions on `useFlowStore.getState()`, advance the fake clock past
`AUTOSAVE_DELAY_MS` (`vi.advanceTimersByTimeAsync`) and assert on
`vi.mocked(api.updateFlow).mock.calls`. Other stores are reset with
`setState`: `useEditorStore.setState({ selectedNodeIds: [] })`,
`useRuntimeStore.setState({ nodeStatus: {}, metrics: {}, debug: {}, subscribed: {} })`,
`useAuthStore.setState({ required: false })`.

Component tests render with `@testing-library/react`, seed the stores with
`setState` (`useFlowStore.setState({ flow, nodeTypes })`) instead of mocking
hooks, and query by role, visible text or `data-testid`. Prefer `getByRole`
for buttons, tabs and menus; use `data-testid` where no accessible role
exists (status chips, panels, fields inside the tray).

The canvas does not lay out in jsdom, so canvas behavior is unit-tested
through the pure helpers `FlowCanvas.tsx` exports and, for real interaction,
in the e2e suite.

Do not assert on i18n keys, Tailwind class names or store internals; test
what a user or another module can observe.

## End-to-end (`web/e2e/`, Playwright)

`playwright.config.ts` builds `../cmd/go-red` to `../bin/go-red-e2e` and
starts it on port 8081 (`GO_RED_E2E_PORT`) with a temporary data directory
(`GO_RED_E2E_DATA`). The server serves the editor embedded in that binary,
so run `npm run build` first or use `npm run e2e:full`. Chromium only, one
worker, traces kept on failure. `editor.spec.ts` seeds a flow through the
REST API (`seedFlow`) and covers: SPA fallback on reload; autosave, deploy,
stop, redeploy and reload; live node status and errors; dynamic output
ports and pruned wires; tray validation and the code editor; tray edits
persisting; deploy errors shown as alerts; theme and language switching;
palette search and collapsed categories; empty states; copy/paste/duplicate
and the context menu; quick add and Ctrl+S; tab rename, reorder and `?`;
Node-RED import and export.

Test ids the components expose; the e2e and unit suites select by them, so
keep them stable:
`app-header`, `deploy-button`, `deploy-options`, `flow-status`,
`save-state` (with `data-state`), `flow-tab-<id>`, `tab-rename-input`,
`tab-dirty`, `flow-details`, `node-details`, `node-status`,
`node-status-detail`, `node-messages`, `node-settings`, `node-help`,
`node-type-help`, `node-type-chip`, `node-description`, `node-disabled`,
`node-shell`, `port-label`, `node-select-<id>`, `flow-canvas`,
`canvas-empty`, `empty-flows`, `context-menu`, `context-<item>`,
`quick-add`, `quick-add-<type>`, `node-palette`, `palette-node-<type>`,
`node-config`, `config-tab-<tab>`, `config-done`, `config-problems`,
`field-<id>`, `field-error`, `list-<id>`, `list-add-<id>`,
`list-item-<id>-<index>`, `typed-input-<id>`, `string-list-<id>`,
`key-value-<id>`, `duration-<id>`, `code-editor`, `debug-panel`,
`debug-live` (with `data-live`), `debug-message`, `debug-summary`,
`debug-pause`, `debug-paused`, `json-node`, `import-preview`,
`import-unsupported`, `shortcut-help`, `token-prompt`, `token-input`,
`ws-status`.

The suite also selects by role and English label (`getByRole('button',
{ name: 'Stop' })`, `getByRole('tab', ...)`, `getByRole('menuitem', ...)`,
`getByRole('alert')`), so renaming a label in `i18n/en.ts` or dropping an
ARIA role breaks it.
