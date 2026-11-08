# web/ — the Go-RED editor

React 18 + TypeScript single-page app, built with Vite, that talks to the Go
server over `/api/*` (REST) and `/ws` (WebSocket). This file covers the
package as a whole: stack, build, scripts and how the app is wired. The
directories under `src/` have their own `AGENTS.md` (`src/`,
`src/components/`, `src/hooks/`, `src/types/`, `src/test/`). Everything
below was read from the code; where the code and a doc disagree, the code
wins and the doc needs a fix.

## Stack

| Concern | Used | Where |
|---|---|---|
| UI | React 18, react-router-dom 6 (`/`, `/flow/:flowId`, `/styleguide`) | `src/App.tsx`, `src/index.tsx` |
| Canvas | `@xyflow/react` 12 | `src/components/FlowCanvas.tsx` |
| State | Zustand 4 stores | `src/store/` |
| Styling | Tailwind 3 with design tokens as CSS variables | `src/styles/tailwind.css`, `tailwind.config.js` |
| Text | i18next + react-i18next, English and German | `src/i18n/en.ts`, `src/i18n/de.ts` |
| Code fields | CodeMirror 6 (javascript, json) | `src/components/config/CodeEditor.tsx` |
| Icons | lucide-react, chosen by name | `src/components/icons.ts` |
| Auto-layout | `@dagrejs/dagre` | `src/lib/layout.ts` |
| Markdown | own subset renderer sanitized with DOMPurify | `src/utils/markdown.ts` |
| Fonts | Inter Variable, JetBrains Mono (`@fontsource`) | imported in `tailwind.css` |
| Unit tests | Vitest 1, Testing Library, jsdom | `src/test/` |
| E2E | Playwright against the real Go server | `e2e/`, `playwright.config.ts` |

`web/go.mod` exists only so Go tooling treats this directory as a separate
module and ignores it.

## Layout

```
src/
  App.tsx, index.tsx     routes, bindServerEvents(), theme and i18n bootstrap
  components/            editor UI (components/AGENTS.md)
  components/config/     edit-tray widgets
  hooks/                 useWebSocket, useEditorShortcuts
  i18n/                  en.ts, de.ts, index.ts (detectLanguage, setLanguage)
  lib/                   auth, clipboard, layout, theme, wsClient
  pages/StyleGuide.tsx   living style guide at /styleguide
  schema/                node config schema -> widgets, defaults, validation, ports
  store/                 Zustand stores, bindServerEvents, editorActions
  styles/                tailwind.css (tokens), reactflow-overrides.css
  test/                  Vitest tests and setup.ts
  types/                 generated.ts (from Go) plus hand-written UI types
  utils/                 api.ts (REST client), markdown.ts, nodeCategories.ts
e2e/editor.spec.ts       Playwright suite
e2e/oidc.spec.ts         OIDC sign-in against a second server (:8082) and
e2e/oidcProvider.ts      the fake OIDC provider it starts (:8083)
```

## Scripts (`package.json`)

| Script | Does |
|---|---|
| `npm run dev` | Vite on :5173, proxying `/api` and `/ws` to `localhost:8080` (start the Go server separately) |
| `npm run build` | production build into `../internal/webui/dist`, which the Go binary embeds |
| `npm run preview` | serve that build locally |
| `npm run lint` | ESLint with zero warnings allowed (`eslint.config.js`) |
| `npm run typecheck` | `tsc --noEmit` (strict, `noUnusedLocals`, `noUnusedParameters`) |
| `npm test` | Vitest in watch mode (`npx vitest run` for a single pass) |
| `npm run test:coverage` | Vitest with v8 coverage |
| `npm run e2e` | Playwright; expects a fresh `npm run build` |
| `npm run e2e:full` | build, then Playwright |

`make build-web`, `make build-all`, `make test-frontend`, `make e2e` and
`make generate-types` in the repository root wrap the same commands, and
run `npm ci` first whenever `package-lock.json` is newer than `node_modules`.
`make e2e-docker` runs the Playwright suite in the official image of the
pinned `@playwright/test` version (host Go toolchain and module cache
mounted read-only, no network, `E2E_ARGS` passed to `playwright test`).

ESLint rules to know: `no-console` (only `warn` and `error` allowed),
`@typescript-eslint/no-explicit-any` off, unused vars must start with `_`,
`src/types/generated.ts` is ignored.

## How the app is wired

- `index.tsx` calls `initTheme()`, imports `./i18n` and the stylesheet, and
  renders `<App/>` in a `BrowserRouter`. `index.html` applies the stored
  theme (`localStorage` key `go-red.theme`) before the first paint.
- `App.tsx` runs `bindServerEvents()` once and mounts `FlowEditor`,
  `ToastHost` and `TokenPrompt`.
- REST (`src/utils/api.ts`): `fetchFlows`, `fetchFlow`, `createFlow`,
  `updateFlow`, `deleteFlow`, `deployFlow`, `undeployFlow`, `getNodes`,
  `getNode`, `exportFlow` (triggers a download), `importFlow(file)`,
  `fetchAuthUser`, `signOut`. Every
  call sends `authHeaders()`; a 401 calls `authRequired()`; a non-2xx
  response throws an `Error` with the server's `error` text.
  `VITE_API_BASE_URL` overrides the `/api` base.
- WebSocket (`src/lib/wsClient.ts`): one module-level `WebSocketClient` per
  tab. It reconnects with exponential backoff (1 s to 30 s), queues sends
  until the socket is open (5 s timeout) and offers the `gored` subprotocol
  plus `gored.token.<token>` when a token is stored. It carries server events
  and read-only queries only; every flow change goes through REST.
- Stores (`src/store/`): `flowStore` (flow list, node types, the open flow as
  an undoable working copy, debounced autosave via `PUT /api/flows/{id}`,
  deploy), `editorStore` (UI state), `runtimeStore` (server-owned node
  status, metrics, debug log), `notificationStore` (toasts).
  `bindServerEvents.ts` routes WebSocket events into them and keeps the
  runtime subscription on the open flow. `editorActions.ts` holds
  copy/paste/duplicate/align/distribute/auto-layout.
- Auth (`src/lib/auth.ts`, `components/TokenPrompt.tsx`): the token is kept
  in `localStorage` (`go-red.token`); an OIDC session is an HttpOnly cookie
  the browser sends by itself. After a 401, `authRequired()` loads
  `GET /api/auth/info`: with OIDC as the only method it goes to
  `/auth/login?next=<page>` on its own (only before anything was
  authenticated in this page load, at most once a minute and not after a
  sign-out, via the `go-red.ssoRedirect` sessionStorage mark), otherwise
  the prompt offers the SSO button and/or the token field (a token reloads
  the page on save). `App` loads `GET /api/auth/me` into `useAuthStore.user`;
  the header menu shows an OIDC user and *Sign out* (saves, then
  `POST /api/auth/logout`; with a `logoutUrl` the browser goes to the
  provider's logout, which comes back to the editor, else it reloads;
  either way into the signed-out dialog). After a sign-out the SSO button
  adds `reauth=1`, so the provider checks the user again.
  Unsaved work survives re-authentication: `flowStore` registers
  `onReauthentication` hooks; `startSignIn` and sign-out stash the unsaved
  draft in `sessionStorage` (`go-red.unsavedFlow`) with its base revision
  and content; `selectFlow` reapplies it on the server's current version
  (`rebase`) as one undoable step. A successful save drops that flow's stash. The leave
  prompt is skipped only while `isSigningIn()` (stash written, or nothing to
  stash), which `pageshow` from the back/forward cache resets. A token
  entered mid-session retries in place (`resumeAfterToken`) instead of
  reloading.
- Concurrent editing (`flowStore`, `store/rebase.ts`, PROTOCOL.md
  *Concurrent editing*): the store keeps `baseRevision` and `baseContent`,
  the server's stored version the working copy is based on, taken from GET
  and PUT responses only. Saves send `baseRevision`; a 409
  (`api.revisionConflict`) calls `pullRemote`, which reapplies this tab's
  changes since the base with `rebase` (field by field, local wins a field
  both changed, deletion wins, wires by endpoints, values compared through
  a canonical form so the server's normalization is not a change), clears
  the undo history and saves again (at most 5 times in a row). A
  `flow:status` with another revision schedules `syncWithServer` after
  `SYNC_DELAY_MS` if the tab is visible and has nothing unsaved;
  `FlowEditor` saves on `visibilitychange` to hidden and syncs when shown.
- Types (`src/types/`): `generated.ts` comes from
  `go generate ./internal/dto/...` and is never edited by hand; the other
  files re-export it and add UI-only types.

## Rules that apply everywhere under web/

- One write path: edits go through `useFlowStore` actions, never through
  direct `fetch` calls or component state that outlives the interaction.
- User-visible strings come from `src/i18n` via `t('...')`, added to both
  `en.ts` and `de.ts`. No literals in components.
- Colors are the semantic Tailwind utilities (`bg-panel`, `text-muted`,
  `border-line`, `bg-accent`, `bg-cat-<category>`); no raw `gray-*` classes.
  Dark mode is `[data-theme="dark"]` on `<html>`, set by `src/lib/theme.ts`.
- HTML from data (node help, descriptions) goes through `renderMarkdown`,
  which sanitizes with DOMPurify.
- Elements the tests hook into carry `data-testid` (list in
  `src/test/AGENTS.md`); keep them when refactoring.
- Before pushing:
  `npm run lint && npm run typecheck && npx vitest run && npm run build`.
  After changing Go wire types: `go generate ./internal/dto/...` and commit
  `generated.ts` (CI fails on a stale file through `make check-types`).

## Reference

- `docs/PROTOCOL.md`: every REST route and WebSocket message.
- `docs/ARCHITECTURE.md`, section "Frontend Architecture (Web UI)".
- `docs/NEXT_LEVEL_PLAN.md`: the phases that produced the current shape.
