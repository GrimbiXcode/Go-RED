import { describe, it, expect, vi, beforeEach } from 'vitest';
import {
  getWebSocketUrl,
  generateId,
  sleep,
} from '../utils/api';

describe('API utility functions', () => {
  beforeEach(() => {
    vi.fn();
  });

  describe('generateId', () => {
    it('should generate a unique ID', () => {
      const id1 = generateId();
      const id2 = generateId();
      expect(id1).not.toBe(id2);
      expect(typeof id1).toBe('string');
      expect(id1.length).toBeGreaterThan(0);
    });
  });

  describe('sleep', () => {
    it('should resolve after specified time', async () => {
      const start = Date.now();
      await sleep(100);
      const end = Date.now();
      expect(end - start).toBeGreaterThanOrEqual(90); // Allow some margin
    });
  });

  describe('getWebSocketUrl', () => {
    it('should return wss URL for https', () => {
      Object.defineProperty(globalThis, 'window', {
        value: { location: { protocol: 'https:', host: 'example.com' } },
        writable: true,
      });
      const url = getWebSocketUrl();
      expect(url).toBe('wss://example.com/ws');
    });

    it('should return ws URL for http', () => {
      Object.defineProperty(globalThis, 'window', {
        value: { location: { protocol: 'http:', host: 'localhost:3000' } },
        writable: true,
      });
      const url = getWebSocketUrl();
      expect(url).toBe('ws://localhost:3000/ws');
    });
  });
});
