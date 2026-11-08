import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, fireEvent, act } from '@testing-library/react';
import {
  SSO_REDIRECT_KEY,
  authHeaders,
  clearToken,
  getToken,
  markSignedOut,
  mayRedirectAutomatically,
  setToken,
  signOutAndReload,
  signedIn,
  useAuthStore,
  wsProtocols,
} from '../lib/auth';
import { fetchFlows } from '../utils/api';
import { WebSocketClient } from '../lib/wsClient';
import { TokenPrompt } from '../components/TokenPrompt';

class FakeWebSocket {
  static last: FakeWebSocket | null = null;
  readyState = 0;
  onopen: (() => void) | null = null;
  onclose: (() => void) | null = null;
  onerror: (() => void) | null = null;
  onmessage: ((event: { data: unknown }) => void) | null = null;
  constructor(
    public url: string,
    public protocols?: string | string[]
  ) {
    FakeWebSocket.last = this;
  }
  send() {}
  close() {}
}

beforeEach(() => {
  clearToken();
  useAuthStore.setState({ required: false });
});

afterEach(() => {
  vi.unstubAllGlobals();
  clearToken();
});

describe('auth token', () => {
  it('is stored in localStorage and turned into headers and subprotocols', () => {
    expect(getToken()).toBeNull();
    expect(authHeaders()).toEqual({});
    expect(wsProtocols()).toEqual(['gored']);

    setToken('secret-token-1234567890');
    expect(window.localStorage.getItem('go-red.token')).toBe('secret-token-1234567890');
    expect(authHeaders()).toEqual({ Authorization: 'Bearer secret-token-1234567890' });
    expect(wsProtocols()).toEqual(['gored', 'gored.token.secret-token-1234567890']);

    clearToken();
    expect(getToken()).toBeNull();
    expect(window.localStorage.getItem('go-red.token')).toBeNull();
  });

  it('is sent with API requests, and a 401 asks for a token', async () => {
    setToken('secret-token-1234567890');
    const fetchMock = vi.fn(async () => ({ ok: true, status: 200, json: async () => [] }));
    vi.stubGlobal('fetch', fetchMock);
    await fetchFlows();
    const [, options] = fetchMock.mock.calls[0] as unknown as [string, RequestInit];
    expect((options.headers as Record<string, string>).Authorization).toBe('Bearer secret-token-1234567890');
    expect(useAuthStore.getState().required).toBe(false);

    vi.stubGlobal('fetch', async () => ({ ok: false, status: 401, statusText: 'Unauthorized', json: async () => ({ error: 'authentication required' }) }));
    await expect(fetchFlows()).rejects.toThrow('authentication required');
    expect(useAuthStore.getState().required).toBe(true);
  });

  it('is offered as a WebSocket subprotocol', () => {
    vi.stubGlobal('WebSocket', FakeWebSocket);
    setToken('secret-token-1234567890');
    const client = new WebSocketClient();
    client.connect('ws://test/ws');
    expect(FakeWebSocket.last?.protocols).toEqual(['gored', 'gored.token.secret-token-1234567890']);
    client.close();
  });
});

describe('TokenPrompt', () => {
  it('stays hidden until a token is required, then stores what the user enters', () => {
    const onDone = vi.fn();
    render(<TokenPrompt onDone={onDone} />);
    expect(screen.queryByTestId('token-prompt')).not.toBeInTheDocument();

    act(() => useAuthStore.getState().setRequired(true));
    const input = screen.getByTestId('token-input');
    expect(screen.getByRole('button', { name: 'Continue' })).toBeDisabled();
    fireEvent.change(input, { target: { value: '  my-token-1234567890  ' } });
    fireEvent.submit(screen.getByTestId('token-prompt'));

    expect(getToken()).toBe('my-token-1234567890');
    expect(onDone).toHaveBeenCalledTimes(1);
    expect(useAuthStore.getState().required).toBe(false);
    expect(screen.queryByTestId('token-prompt')).not.toBeInTheDocument();
  });
});

describe('single sign-on', () => {
  const ssoOnly = { methods: ['oidc'], loginUrl: '/auth/login' };
  let assign: ReturnType<typeof vi.fn>;

  beforeEach(() => {
    window.sessionStorage.clear();
    useAuthStore.setState({ required: false, info: null, user: null });
    assign = vi.fn();
    vi.stubGlobal('location', { ...window.location, pathname: '/flow/demo', search: '?x=1', assign, reload: vi.fn() });
  });

  afterEach(() => {
    window.sessionStorage.clear();
  });

  const unauthorized = (info: unknown) =>
    vi.fn(async (url: string) =>
      url.endsWith('/auth/info')
        ? { ok: true, status: 200, json: async () => info }
        : { ok: false, status: 401, statusText: 'Unauthorized', json: async () => ({ error: 'authentication required' }) }
    );

  it('sends the browser to the sign-in when single sign-on is the only method', async () => {
    vi.stubGlobal('fetch', unauthorized(ssoOnly));
    await expect(fetchFlows()).rejects.toThrow();
    await vi.waitFor(() => expect(assign).toHaveBeenCalledWith('/auth/login?next=%2Fflow%2Fdemo%3Fx%3D1'));
    expect(mayRedirectAutomatically()).toBe(false);
  });

  it('does not redirect again right after an automatic redirect, or after signing out', async () => {
    vi.stubGlobal('fetch', unauthorized(ssoOnly));
    window.sessionStorage.setItem(SSO_REDIRECT_KEY, String(Date.now()));
    await expect(fetchFlows()).rejects.toThrow();
    await vi.waitFor(() => expect(useAuthStore.getState().info).toEqual(ssoOnly));
    expect(assign).not.toHaveBeenCalled();
    expect(mayRedirectAutomatically(Date.now() + 120_000)).toBe(true);

    markSignedOut();
    expect(mayRedirectAutomatically(Date.now() + 120_000)).toBe(false);
    render(<TokenPrompt />);
    expect(screen.getByTestId('signed-out')).toBeInTheDocument();
    expect(screen.queryByTestId('token-input')).not.toBeInTheDocument();
    fireEvent.click(screen.getByTestId('sso-button'));
    // After a sign-out the provider is asked to check the user again.
    expect(assign).toHaveBeenCalledWith('/auth/login?next=%2Fflow%2Fdemo%3Fx%3D1&reauth=1');
    expect(mayRedirectAutomatically()).toBe(true);
  });

  it('shows the dialog instead of navigating away when a session expires while editing', async () => {
    vi.stubGlobal('fetch', unauthorized(ssoOnly));
    signedIn({ method: 'oidc', subject: 'u1', name: 'Ada' });
    await expect(fetchFlows()).rejects.toThrow();
    await vi.waitFor(() => expect(useAuthStore.getState().info).toEqual(ssoOnly));
    expect(useAuthStore.getState().required).toBe(true);
    expect(assign).not.toHaveBeenCalled();
  });

  it('offers both single sign-on and the token when the server takes both', async () => {
    vi.stubGlobal('fetch', unauthorized({ methods: ['token', 'oidc'], loginUrl: '/auth/login' }));
    render(<TokenPrompt />);
    await act(async () => {
      await expect(fetchFlows()).rejects.toThrow();
      await vi.waitFor(() => expect(useAuthStore.getState().info).not.toBeNull());
    });
    expect(screen.getByTestId('sso-button')).toBeInTheDocument();
    expect(screen.getByTestId('token-input')).toBeInTheDocument();
    expect(assign).not.toHaveBeenCalled();
  });

  it('clears the redirect mark once signed in, and signs out through the API', async () => {
    window.sessionStorage.setItem(SSO_REDIRECT_KEY, String(Date.now()));
    signedIn({ method: 'oidc', subject: 'u1', name: 'Ada' });
    expect(useAuthStore.getState().user?.name).toBe('Ada');
    expect(mayRedirectAutomatically()).toBe(true);

    const logout = vi.fn(async () => undefined);
    const reload = vi.fn();
    await signOutAndReload(logout, reload);
    expect(logout).toHaveBeenCalledTimes(1);
    expect(reload).toHaveBeenCalledTimes(1);
    expect(mayRedirectAutomatically()).toBe(false);

    // With a provider logout URL the browser goes there instead of reloading.
    reload.mockClear();
    await signOutAndReload(async () => ({ logoutUrl: 'https://idp.example/end-session?client_id=c' }), reload);
    expect(assign).toHaveBeenCalledWith('https://idp.example/end-session?client_id=c');
    expect(reload).not.toHaveBeenCalled();
    expect(mayRedirectAutomatically()).toBe(false);
  });
});
