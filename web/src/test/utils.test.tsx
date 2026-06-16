import { describe, it, expect, vi, beforeEach } from 'vitest';
import {
  fetchFlows,
  fetchFlow,
  createFlow,
  updateFlow,
  deleteFlow,
  deployFlow,
  undeployFlow,
  getNodes,
  getWebSocketUrl,
  generateId,
  sleep,
} from '../utils/api';

describe('API utility functions', () => {
  beforeEach(() => {
    global.fetch = vi.fn();
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
      global.window = { location: { protocol: 'https:', host: 'example.com' } } as any;
      const url = getWebSocketUrl();
      expect(url).toBe('wss://example.com/ws');
    });

    it('should return ws URL for http', () => {
      global.window = { location: { protocol: 'http:', host: 'localhost:3000' } } as any;
      const url = getWebSocketUrl();
      expect(url).toBe('ws://localhost:3000/ws');
    });
  });
});
