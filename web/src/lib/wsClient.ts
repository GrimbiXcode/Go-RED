import type { WebSocketMessage, WebSocketMessageType } from '../types/message';
import { getWebSocketUrl } from '../utils/api';
import { wsProtocols } from './auth';

/**
 * The single WebSocket connection of the editor.
 *
 * Every hook and component that needs the server connection goes through
 * this module-level client, so the browser tab holds exactly one socket no
 * matter how many components use it. The React side (useWebSocket) only
 * subscribes to this client's state and message streams.
 */

export interface ConnectionState {
  connected: boolean;
  connecting: boolean;
  error: Error | null;
  /** Consecutive failed connection attempts since the last successful open. */
  attempts: number;
}

export interface SendOptions {
  /** How long a queued message waits for the socket to open before failing (ms). */
  timeout?: number;
}

type MessageListener = (data: any) => void;
type StateListener = (state: ConnectionState) => void;

interface QueuedMessage {
  message: WebSocketMessage;
  resolve: () => void;
  reject: (error: Error) => void;
  timer: ReturnType<typeof setTimeout>;
}

// WebSocket.readyState values, spelled out so this works with test doubles
// that do not define the static constants.
const CONNECTING = 0;
const OPEN = 1;

const INITIAL_BACKOFF_MS = 1000;
const MAX_BACKOFF_MS = 30000;
const DEFAULT_SEND_TIMEOUT_MS = 5000;

export class WebSocketClient {
  private socket: WebSocket | null = null;
  private url: string | null = null;
  private state: ConnectionState = { connected: false, connecting: false, error: null, attempts: 0 };
  private messageListeners = new Map<WebSocketMessageType, Set<MessageListener>>();
  private stateListeners = new Set<StateListener>();
  private queue: QueuedMessage[] = [];
  private reconnectTimer: ReturnType<typeof setTimeout> | null = null;
  private closedByUser = false;

  /** Opens the connection if it is not already open or opening. */
  connect(url: string = getWebSocketUrl()): void {
    if (this.socket && this.url === url && this.isOpenOrOpening()) {
      return;
    }
    this.url = url;
    this.closedByUser = false;
    this.open();
  }

  getState = (): ConnectionState => this.state;

  onStateChange = (listener: StateListener): (() => void) => {
    this.stateListeners.add(listener);
    return () => {
      this.stateListeners.delete(listener);
    };
  };

  subscribe(type: WebSocketMessageType, listener: MessageListener): () => void {
    let listeners = this.messageListeners.get(type);
    if (!listeners) {
      listeners = new Set();
      this.messageListeners.set(type, listeners);
    }
    listeners.add(listener);
    return () => this.unsubscribe(type, listener);
  }

  unsubscribe(type: WebSocketMessageType, listener: MessageListener): void {
    const listeners = this.messageListeners.get(type);
    if (!listeners) return;
    listeners.delete(listener);
    if (listeners.size === 0) {
      this.messageListeners.delete(type);
    }
  }

  send(type: WebSocketMessageType, data: unknown, options?: SendOptions): Promise<void> {
    return this.sendRaw({ type, data, timestamp: new Date().toISOString() }, options);
  }

  /**
   * Sends a message now if the socket is open; otherwise queues it until the
   * socket opens, failing after `timeout` ms.
   */
  sendRaw(message: WebSocketMessage, options: SendOptions = {}): Promise<void> {
    const { timeout = DEFAULT_SEND_TIMEOUT_MS } = options;

    if (this.socket && this.socket.readyState === OPEN) {
      try {
        this.socket.send(JSON.stringify(message));
        return Promise.resolve();
      } catch (error) {
        return Promise.reject(toError(error));
      }
    }

    if (this.closedByUser) {
      return Promise.reject(new Error('WebSocket is closed'));
    }

    return new Promise<void>((resolve, reject) => {
      const entry: QueuedMessage = {
        message,
        resolve,
        reject,
        timer: setTimeout(() => {
          this.queue = this.queue.filter((queued) => queued !== entry);
          reject(new Error('WebSocket connection timeout'));
        }, timeout),
      };
      this.queue.push(entry);
      if (!this.isOpenOrOpening()) {
        this.open();
      }
    });
  }

  /** Drops the current socket (if any) and opens a new one right away. */
  reconnect(): void {
    this.clearReconnectTimer();
    this.closedByUser = false;
    this.discardSocket();
    this.setState({ attempts: 0, error: null });
    this.open();
  }

  /** Closes the connection and stops reconnecting until connect() is called again. */
  close(): void {
    this.closedByUser = true;
    this.clearReconnectTimer();
    this.discardSocket();
    this.rejectQueue(new Error('WebSocket is closed'));
    this.setState({ connected: false, connecting: false });
  }

  private isOpenOrOpening(): boolean {
    return !!this.socket && (this.socket.readyState === OPEN || this.socket.readyState === CONNECTING);
  }

  private open(): void {
    this.clearReconnectTimer();
    if (this.isOpenOrOpening()) {
      return;
    }
    if (!this.url) {
      this.url = getWebSocketUrl();
    }

    let socket: WebSocket;
    try {
      socket = new WebSocket(this.url, wsProtocols());
    } catch (error) {
      this.setState({ connected: false, connecting: false, error: toError(error) });
      this.scheduleReconnect();
      return;
    }

    this.socket = socket;
    this.setState({ connecting: true });

    socket.onopen = () => {
      if (this.socket !== socket) return;
      this.setState({ connected: true, connecting: false, error: null, attempts: 0 });
      this.flushQueue();
    };

    socket.onmessage = (event: MessageEvent) => {
      this.handleMessage(event.data);
    };

    socket.onerror = () => {
      if (this.socket !== socket) return;
      this.setState({ error: new Error('WebSocket connection error') });
    };

    socket.onclose = () => {
      if (this.socket !== socket) return;
      this.socket = null;
      this.setState({ connected: false, connecting: false });
      if (!this.closedByUser) {
        this.scheduleReconnect();
      }
    };
  }

  private discardSocket(): void {
    const socket = this.socket;
    if (!socket) return;
    this.socket = null;
    socket.onopen = null;
    socket.onmessage = null;
    socket.onerror = null;
    socket.onclose = null;
    try {
      socket.close();
    } catch {
      // Already closed.
    }
  }

  private scheduleReconnect(): void {
    if (this.reconnectTimer || this.closedByUser) return;
    const attempts = this.state.attempts + 1;
    const delay = Math.min(INITIAL_BACKOFF_MS * 2 ** (attempts - 1), MAX_BACKOFF_MS);
    this.setState({ attempts, connecting: true });
    this.reconnectTimer = setTimeout(() => {
      this.reconnectTimer = null;
      this.open();
    }, delay);
  }

  private clearReconnectTimer(): void {
    if (this.reconnectTimer) {
      clearTimeout(this.reconnectTimer);
      this.reconnectTimer = null;
    }
  }

  private flushQueue(): void {
    const socket = this.socket;
    if (!socket || socket.readyState !== OPEN) return;
    const pending = this.queue;
    this.queue = [];
    for (const entry of pending) {
      clearTimeout(entry.timer);
      try {
        socket.send(JSON.stringify(entry.message));
        entry.resolve();
      } catch (error) {
        entry.reject(toError(error));
      }
    }
  }

  private rejectQueue(error: Error): void {
    const pending = this.queue;
    this.queue = [];
    for (const entry of pending) {
      clearTimeout(entry.timer);
      entry.reject(error);
    }
  }

  private handleMessage(raw: unknown): void {
    if (typeof raw !== 'string') return;

    let message: WebSocketMessage;
    try {
      message = JSON.parse(raw);
    } catch (error) {
      console.warn('Ignoring malformed WebSocket message', error);
      return;
    }
    if (!message || typeof message.type !== 'string') return;

    const listeners = this.messageListeners.get(message.type);
    if (listeners) {
      for (const listener of Array.from(listeners)) {
        try {
          listener(message.data);
        } catch (error) {
          console.error(`WebSocket listener for ${message.type} failed`, error);
        }
      }
    }

    const wildcard = this.messageListeners.get('*');
    if (wildcard) {
      for (const listener of Array.from(wildcard)) {
        try {
          listener(message);
        } catch (error) {
          console.error('WebSocket wildcard listener failed', error);
        }
      }
    }
  }

  private setState(patch: Partial<ConnectionState>): void {
    this.state = { ...this.state, ...patch };
    for (const listener of Array.from(this.stateListeners)) {
      listener(this.state);
    }
  }
}

function toError(value: unknown): Error {
  return value instanceof Error ? value : new Error(String(value));
}

/** The one client instance shared by the whole editor. */
export const wsClient = new WebSocketClient();
