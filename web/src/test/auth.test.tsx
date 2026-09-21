import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, fireEvent, act } from '@testing-library/react';
import { authHeaders, clearToken, getToken, setToken, useAuthStore, wsProtocols } from '../lib/auth';
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
