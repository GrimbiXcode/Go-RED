import '@testing-library/jest-dom';
import { cleanup } from '@testing-library/react';
import { afterEach } from 'vitest';

// Runs after each test
afterEach(() => {
  cleanup();
});

// Mock WebSocket
global.WebSocket = class MockWebSocket {
  constructor(url: string) {
    this.url = url;
    this.readyState = WebSocket.OPEN;
    this.onopen = null;
    this.onclose = null;
    this.onerror = null;
    this.onmessage = null;
    setTimeout(() => {
      if (this.onopen) this.onopen(new Event('open'));
    }, 0);
  }
  url: string;
  readyState: number;
  onopen: ((this: WebSocket, ev: Event) => any) | null;
  onclose: ((this: WebSocket, ev: CloseEvent) => any) | null;
  onerror: ((this: WebSocket, ev: Event) => any) | null;
  onmessage: ((this: WebSocket, ev: MessageEvent) => any) | null;
  
  send(data: string | ArrayBufferLike | Blob | ArrayBufferView) {
    if (this.onmessage) {
      this.onmessage(new MessageEvent('message', { data }));
    }
  }
  
  close() {
    this.readyState = WebSocket.CLOSED;
    if (this.onclose) this.onclose(new CloseEvent('close'));
  }
} as any;

// Mock fetch
global.fetch = vi.fn(async (url: RequestInfo | URL, init?: RequestInit) => {
  const response = {
    ok: true,
    status: 200,
    json: async () => ({ success: true, data: {} }),
    text: async () => '{}',
  };
  return Promise.resolve(response);
});
