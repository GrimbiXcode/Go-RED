# web/src/hooks

Two hooks exist: `useWebSocket` and `useEditorShortcuts`. Application
state does not live in hooks; it lives in the Zustand stores under
`src/store/`. Hooks here only bind browser or connection concerns to React.
`index.ts` re-exports both.

## `useWebSocket.ts`

```ts
export function useWebSocket(): WebSocketHook;

export interface WebSocketHook {
  state: WebSocketState; // ConnectionState: { connected, connecting, error, attempts }
  sendMessage: (type: WebSocketMessageType, data: any, options?: SendMessageOptions) => Promise<void>;
  sendRawMessage: (message: WebSocketMessage, options?: SendMessageOptions) => Promise<void>;
  subscribe: (type: WebSocketMessageType, callback: (data: any) => void) => () => void;
  unsubscribe: (type: WebSocketMessageType, callback: (data: any) => void) => void;
  reconnect: () => void;
  close: () => void;
}
```

- Takes no arguments. The URL comes from `getWebSocketUrl()` in
  `utils/api.ts` (`ws(s)://<page host>/ws`).
- A thin binding over the singleton `wsClient` (`src/lib/wsClient.ts`)
  through `useSyncExternalStore`. Calling it in any number of components
  never opens a second socket; its effect calls `wsClient.connect()`, which
  is a no-op while the socket is open or opening.
- All returned functions are stable across renders and safe as effect
  dependencies. `subscribe` returns the unsubscribe function; return it from
  the effect.
- `sendMessage(type, data)` builds the `{ type, data, timestamp }` envelope.
  The promise resolves once sent and rejects after `options.timeout`
  (default 5 s) if the socket does not open, or immediately after `close()`.
- Server events that feed stores are not subscribed here but in
  `store/bindServerEvents.ts`. Components use this hook for the connection
  state (`WebSocketStatus`) or for one-off sends; the inject button sends
  `message:send` directly through `wsClient`.

Typical use:

```ts
const { state, subscribe } = useWebSocket();
useEffect(() => subscribe('flow:status', (data) => { /* ... */ }), [subscribe]);
```

## `useEditorShortcuts.ts`

```ts
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

export function useEditorShortcuts(handlers: EditorShortcutHandlers): void;
```

A `keydown` listener on `window` for the editor page: Ctrl/Cmd+Z undo,
Ctrl/Cmd+Shift+Z and Ctrl/Cmd+Y redo, Ctrl/Cmd+S deploy, Ctrl/Cmd+E export,
Ctrl/Cmd+C, V and D copy, paste and duplicate, `?` help. Everything is
ignored while the event target is an input, textarea, select or
contenteditable, so form fields keep the browser's own clipboard. Optional
handlers that are absent leave the browser default in place. Mounted once by
`FlowEditor` with handlers that call `flowStore` and `store/editorActions`.

Keys handled elsewhere: arrows (nudge), Ctrl/Cmd+A (select all), zoom and
delete in `components/FlowCanvas.tsx`; `/` (focus search) in
`components/NodePalette.tsx`; Escape in the dialogs and menus themselves.

## Adding a hook

- Only for browser and React glue: listeners, media queries, external
  stores. Data and editing logic go into `src/store/` or `src/lib/` so they
  are testable without React and reachable from non-component code
  (`editorActions` is called from menus, shortcuts and the header alike).
- Name it `useX.ts`, export it from `index.ts`, return stable callbacks
  (`useCallback`/`useMemo`) and remove every listener in the effect cleanup.
- Test the underlying logic as a plain module (the WebSocket client is
  tested in `test/wsClient.test.ts` without React); use `renderHook` from
  `@testing-library/react` only for React-specific behavior.

## Notes on the client underneath

- `wsClient.subscribe('*', cb)` receives every message (the `*` member of
  `MessageType`); typed listeners run first.
- Connection state changes are pushed through `onStateChange`; the hook's
  `state` is the same object `wsClient.getState()` returns, so a component
  that only needs `connected` should select it and not re-render on
  `attempts`.
