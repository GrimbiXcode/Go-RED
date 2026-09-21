import { create } from 'zustand';

/**
 * Access token handling for servers started with -auth-token.
 *
 * The token lives in localStorage so a reload keeps the session; REST calls
 * send it as a bearer header and the WebSocket handshake as a subprotocol
 * (browsers cannot set headers there). A 401 from the server flips
 * `required`, which shows the TokenPrompt dialog.
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
  /** The server answered 401: the user has to enter a token. */
  required: boolean;
  setRequired: (required: boolean) => void;
}

export const useAuthStore = create<AuthState>((set) => ({
  required: false,
  setRequired: (required) => set({ required }),
}));

/** Called by the API layer when a request came back unauthorized. */
export function authRequired(): void {
  useAuthStore.getState().setRequired(true);
}
