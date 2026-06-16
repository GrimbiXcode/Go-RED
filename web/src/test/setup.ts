import '@testing-library/jest-dom';
import { cleanup } from '@testing-library/react';
import { afterEach, vi } from 'vitest';

// Runs after each test
afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
});

// Mock WebSocket
global.WebSocket = class MockWebSocket {
  constructor(url: string | URL) {
    this.url = url.toString();
    this.readyState = WebSocket.OPEN;
    this.onopen = null;
    this.onclose = null;
    this.onerror = null;
    this.onmessage = null;
    this.binaryType = 'blob';
    setTimeout(() => {
      if (this.onopen) this.onopen(new Event('open'));
    }, 0);
  }
  url: string;
  readyState: number;
  binaryType: BinaryType;
  onopen: ((this: WebSocket, ev: Event) => any) | null;
  onclose: ((this: WebSocket, ev: CloseEvent) => any) | null;
  onerror: ((this: WebSocket, ev: Event) => any) | null;
  onmessage: ((this: WebSocket, ev: MessageEvent) => any) | null;
  
  send(data: string | ArrayBufferLike | Blob | ArrayBufferView) {
    if (this.onmessage) {
      this.onmessage(new MessageEvent('message', { data: JSON.stringify(data) }));
    }
  }
  
  close(code?: number, reason?: string) {
    this.readyState = WebSocket.CLOSED;
    if (this.onclose) this.onclose(new CloseEvent('close', { code, reason }));
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