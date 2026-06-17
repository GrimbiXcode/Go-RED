import '@testing-library/jest-dom';
import { cleanup } from '@testing-library/react';
import { afterEach, vi } from 'vitest';

// Runs after each test
afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
});

// Mock WebSocket
class MockWebSocket {
  url: string;
  readyState: number;
  binaryType: string;
  extensions: string;
  protocol: string;
  bufferedAmount: number;
  onopen: ((this: MockWebSocket, ev: Event) => any) | null;
  onclose: ((this: MockWebSocket, ev: CloseEvent) => any) | null;
  onerror: ((this: MockWebSocket, ev: Event) => any) | null;
  onmessage: ((this: MockWebSocket, ev: MessageEvent) => any) | null;
  
  constructor(url: string | URL) {
    this.url = String(url);
    this.readyState = 1; // OPEN
    this.onopen = null;
    this.onclose = null;
    this.onerror = null;
    this.onmessage = null;
    this.binaryType = 'blob';
    this.extensions = '';
    this.protocol = '';
    this.bufferedAmount = 0;
    
    setTimeout(() => {
      if (this.onopen) {
        const event = new Event('open');
        this.onopen(event);
      }
    }, 0);
  }
  
  addEventListener(_type: string, _listener: EventListenerOrEventListenerObject, _options?: boolean | AddEventListenerOptions): void {}
  removeEventListener(_type: string, _listener: EventListenerOrEventListenerObject, _options?: boolean | EventListenerOptions): void {}
  dispatchEvent(_event: Event): boolean { return true; }
  
  CONNECTING = 0;
  OPEN = 1;
  CLOSING = 2;
  CLOSED = 3;
  
  send(data: string | ArrayBufferLike | Blob | ArrayBufferView) {
    if (this.onmessage) {
      const event = new MessageEvent('message', { data: JSON.stringify(data) });
      this.onmessage(event);
    }
  }
  
  close(code?: number, reason?: string) {
    this.readyState = 3; // CLOSED
    if (this.onclose) {
      const event = new CloseEvent('close', { code, reason });
      this.onclose(event);
    }
  }
}

// Assign to globalThis for compatibility
Object.defineProperty(globalThis, 'WebSocket', { value: MockWebSocket });

// Mock fetch
Object.defineProperty(globalThis, 'fetch', {
  value: vi.fn(async (_url: RequestInfo | URL, _init?: RequestInit) => {
    const mockResponse: Response = {
      ok: true,
      status: 200,
      statusText: 'OK',
      headers: new Headers(),
      redirected: false,
      type: 'basic',
      url: '',
      async json() { return { success: true, data: {} }; },
      async text() { return '{}'; },
      clone: function() { return this; },
    } as any;
    return Promise.resolve(mockResponse);
  }),
});