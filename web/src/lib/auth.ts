import { create } from 'zustand';
import type { AuthInfo, AuthUser } from '../types/api';

/**
 * Authentication against servers started with -auth-token and/or OIDC
 * sign-in (-oidc-issuer).
 *
 * The token lives in localStorage so a reload keeps the session; REST calls
 * send it as a bearer header and the WebSocket handshake as a subprotocol
 * (browsers cannot set headers there). An OIDC session is an HttpOnly
 * cookie the browser sends by itself on same-origin requests.
 *
 * A 401 from the server flips `required` and loads GET /api/auth/info. When
 * single sign-on is the only method and nothing has been authenticated in
 * this page load yet, the page goes straight to the sign-in (once: a
 * sessionStorage mark stops a redirect loop when the cookie does not stick,
 * and an explicit sign-out sets it too). Otherwise, including a session
 * that expires while the user edits, the TokenPrompt dialog offers the
 * token field and/or the SSO button, so nothing is navigated away from
 * unasked.
 */

const TOKEN_KEY = 'go-red.token';
export const WS_PROTOCOL = 'gored';
export const WS_TOKEN_PROTOCOL_PREFIX = 'gored.token.';

let memoryToken: string | null = null;

/** The stored token, or null when none is set. */
export function getToken(): string | null {
  if (memoryToken === null) {
    try {
      memoryToken = window.localStorage.getItem(TOKEN_KEY) ?? '';
    } catch {
      memoryToken = '';
    }
  }
  return memoryToken || null;
}

export function setToken(token: string): void {
  memoryToken = token;
  try {
    window.localStorage.setItem(TOKEN_KEY, token);
  } catch {
    // localStorage may be unavailable; the token still works for this page.
  }
}

export function clearToken(): void {
  memoryToken = '';
  try {
    window.localStorage.removeItem(TOKEN_KEY);
  } catch {
    // ignore
  }
}

/** Authorization header for REST calls, if a token is stored. */
export function authHeaders(): Record<string, string> {
  const token = getToken();
  return token ? { Authorization: `Bearer ${token}` } : {};
}

/** Subprotocols for the WebSocket handshake: the editor's protocol, plus the token when there is one. */
export function wsProtocols(): string[] {
  const token = getToken();
  return token ? [WS_PROTOCOL, `${WS_TOKEN_PROTOCOL_PREFIX}${token}`] : [WS_PROTOCOL];
}

interface AuthState {
  /** The server answered 401: the user has to authenticate. */
  required: boolean;
  /** How this server accepts authentication (null until loaded). */
  info: AuthInfo | null;
  /** Who the editor is signed in as (null until loaded). */
  user: AuthUser | null;
  setRequired: (required: boolean) => void;
  setInfo: (info: AuthInfo | null) => void;
  setUser: (user: AuthUser | null) => void;
}

export const useAuthStore = create<AuthState>((set) => ({
  required: false,
  info: null,
  user: null,
  setRequired: (required) => set({ required }),
  setInfo: (info) => set({ info }),
  setUser: (user) => set({ user }),
}));

const API_BASE_URL = (import.meta as any).env?.VITE_API_BASE_URL || '/api';

/** sessionStorage key: when the editor last sent the browser to sign in on its own, or 'signed-out'. */
export const SSO_REDIRECT_KEY = 'go-red.ssoRedirect';
/** An automatic redirect within this window means the last one did not work. */
const SSO_RETRY_MS = 60_000;

function readMark(): string | null {
  try {
    return window.sessionStorage.getItem(SSO_REDIRECT_KEY);
  } catch {
    return null;
  }
}

function writeMark(value: string | null): void {
  try {
    if (value === null) window.sessionStorage.removeItem(SSO_REDIRECT_KEY);
    else window.sessionStorage.setItem(SSO_REDIRECT_KEY, value);
  } catch {
    // sessionStorage may be unavailable; then the dialog is always shown.
  }
}

/** Whether the editor may send the browser to the sign-in page by itself. */
export function mayRedirectAutomatically(now = Date.now()): boolean {
  const mark = readMark();
  if (mark === null) return true;
  const last = Number(mark);
  return Number.isFinite(last) && now - last > SSO_RETRY_MS;
}

/** The sign-in URL for info, returning to the current page afterwards. */
export function signInUrl(info: AuthInfo): string {
  const next = `${window.location.pathname}${window.location.search}`;
  return `${info.loginUrl ?? '/auth/login'}?next=${encodeURIComponent(next)}`;
}

/** Sends the browser to the OIDC sign-in. */
export function startSignIn(info: AuthInfo, automatic = false): void {
  writeMark(automatic ? String(Date.now()) : null);
  window.location.assign(signInUrl(info));
}

/** Remembers that the user signed out, so the next 401 shows the dialog instead of signing in again. */
export function markSignedOut(): void {
  writeMark('signed-out');
}

/** Whether the user signed out in this tab (the dialog then says so). */
export function isSignedOut(): boolean {
  return readMark() === 'signed-out';
}

/**
 * Ends the OIDC session: logout (the POST to /api/auth/logout) runs first,
 * then the page reloads into the signed-out dialog.
 */
export async function signOutAndReload(logout: () => Promise<void>, reload = () => window.location.reload()): Promise<void> {
  await logout();
  markSignedOut();
  reload();
}

/** The user is signed in: automatic sign-in may be used again next time. */
export function signedIn(user: AuthUser): void {
  useAuthStore.getState().setUser(user);
  if (user.method === 'oidc') writeMark(null);
}

let infoRequest: Promise<AuthInfo | null> | null = null;

/** Loads GET /api/auth/info once; null when it cannot be read. */
export function loadAuthInfo(): Promise<AuthInfo | null> {
  const known = useAuthStore.getState().info;
  if (known) return Promise.resolve(known);
  if (!infoRequest) {
    infoRequest = fetch(`${API_BASE_URL}/auth/info`)
      .then((response) => (response.ok ? (response.json() as Promise<AuthInfo>) : null))
      .catch(() => null)
      .then((info) => {
        infoRequest = null;
        useAuthStore.getState().setInfo(info);
        return info;
      });
  }
  return infoRequest;
}

/** Called by the API layer when a request came back unauthorized. */
export function authRequired(): void {
  if (useAuthStore.getState().required) return;
  useAuthStore.getState().setRequired(true);
  void loadAuthInfo().then((info) => {
    const signedInBefore = useAuthStore.getState().user !== null;
    if (info && info.methods.length === 1 && info.methods[0] === 'oidc' && !signedInBefore && mayRedirectAutomatically()) {
      startSignIn(info, true);
    }
  });
}
