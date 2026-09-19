import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { WebSocketClient } from '../lib/wsClient';

/** A controllable stand-in for the browser WebSocket. */
class FakeWebSocket {
  static instances: FakeWebSocket[] = [];
  readyState = 0;
  sent: string[] = [];
  onopen: (() => void) | null = null;
  onclose: (() => void) | null = null;
  onerror: (() => void) | null = null;
  onmessage: ((event: { data: unknown }) => void) | null = null;

  constructor(public url: string) {
    FakeWebSocket.instances.push(this);
  }

  open() {
    this.readyState = 1;
    this.onopen?.();
  }

  receive(message: unknown) {
    this.onmessage?.({ data: typeof message === 'string' ? message : JSON.stringify(message) });
  }

  drop() {
    this.readyState = 3;
    this.onclose?.();
  }

  send(data: string) {
    if (this.readyState !== 1) throw new Error('not open');
    this.sent.push(data);
  }

  close() {
    this.readyState = 3;
  }
}

const latest = () => FakeWebSocket.instances[FakeWebSocket.instances.length - 1];

beforeEach(() => {
  FakeWebSocket.instances = [];
  vi.stubGlobal('WebSocket', FakeWebSocket);
  vi.useFakeTimers();
});

afterEach(() => {
  vi.unstubAllGlobals();
  vi.useRealTimers();
});

describe('WebSocketClient', () => {
  it('opens exactly one socket no matter how often connect is called', () => {
    const client = new WebSocketClient();
    client.connect('ws://test/ws');
    client.connect('ws://test/ws');
    client.connect('ws://test/ws');
    expect(FakeWebSocket.instances).toHaveLength(1);
    expect(client.getState().connecting).toBe(true);

    latest().open();
    expect(client.getState()).toMatchObject({ connected: true, connecting: false, attempts: 0 });
  });

  it('queues messages until the socket opens, then flushes them in order', async () => {
    const client = new WebSocketClient();
    client.connect('ws://test/ws');
    const first = client.send('ping', { n: 1 });
    const second = client.send('ping', { n: 2 });
    expect(latest().sent).toHaveLength(0);

    latest().open();
    await Promise.all([first, second]);
    expect(latest().sent.map((raw) => JSON.parse(raw).data.n)).toEqual([1, 2]);
  });

  it('rejects a queued message that never gets a connection', async () => {
    const client = new WebSocketClient();
    client.connect('ws://test/ws');
    const pending = client.send('ping', {}, { timeout: 500 });
    const assertion = expect(pending).rejects.toThrow('timeout');
    await vi.advanceTimersByTimeAsync(600);
    await assertion;
  });

  it('dispatches messages to typed and wildcard listeners', () => {
    vi.spyOn(console, 'warn').mockImplementation(() => {});
    const client = new WebSocketClient();
    client.connect('ws://test/ws');
    latest().open();

    const typed = vi.fn();
    const wildcard = vi.fn();
    const unsubscribe = client.subscribe('flow:status', typed);
    client.subscribe('*', wildcard);

    latest().receive({ type: 'flow:status', data: { flowId: 'f1', status: 'running' }, timestamp: 't' });
    latest().receive('not json');
    latest().receive({ type: 'other', data: 1, timestamp: 't' });

    expect(typed).toHaveBeenCalledTimes(1);
    expect(typed).toHaveBeenCalledWith({ flowId: 'f1', status: 'running' });
    expect(wildcard).toHaveBeenCalledTimes(2);

    unsubscribe();
    latest().receive({ type: 'flow:status', data: {}, timestamp: 't' });
    expect(typed).toHaveBeenCalledTimes(1);
  });

  it('reconnects with exponential backoff after the socket drops', async () => {
    const client = new WebSocketClient();
    const states: boolean[] = [];
    client.onStateChange((state) => states.push(state.connected));
    client.connect('ws://test/ws');
    latest().open();

    latest().drop();
    expect(client.getState().connected).toBe(false);
    expect(FakeWebSocket.instances).toHaveLength(1);

    await vi.advanceTimersByTimeAsync(1000);
    expect(FakeWebSocket.instances).toHaveLength(2);

    latest().drop();
    await vi.advanceTimersByTimeAsync(1999);
    expect(FakeWebSocket.instances).toHaveLength(2);
    await vi.advanceTimersByTimeAsync(1);
    expect(FakeWebSocket.instances).toHaveLength(3);

    latest().open();
    expect(client.getState()).toMatchObject({ connected: true, attempts: 0 });
    expect(states).toContain(true);
  });

  it('close stops reconnecting and rejects queued messages', async () => {
    const client = new WebSocketClient();
    client.connect('ws://test/ws');
    const pending = client.send('ping', {});
    const assertion = expect(pending).rejects.toThrow('closed');
    client.close();
    await assertion;

    await vi.advanceTimersByTimeAsync(60_000);
    expect(FakeWebSocket.instances).toHaveLength(1);
    expect(client.getState()).toMatchObject({ connected: false, connecting: false });
  });
});
