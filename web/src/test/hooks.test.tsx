import { renderHook } from '@testing-library/react';
import { vi } from 'vitest';
import { useWebSocket } from '../hooks/useWebSocket';
import { useFlows } from '../hooks/useFlows';

describe('useWebSocket hook', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('should initialize with connecting state', () => {
    const { result } = renderHook(() => useWebSocket());
    expect(result.current.state.connecting).toBe(true);
    expect(result.current.state.connected).toBe(false);
  });

  it('should have sendMessage function', () => {
    const { result } = renderHook(() => useWebSocket());
    expect(typeof result.current.sendMessage).toBe('function');
  });

  it('should have subscribe function', () => {
    const { result } = renderHook(() => useWebSocket());
    expect(typeof result.current.subscribe).toBe('function');
  });

  it('should have close function', () => {
    const { result } = renderHook(() => useWebSocket());
    expect(typeof result.current.close).toBe('function');
  });
});

describe('useFlows hook', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('should initialize with loading state', () => {
    const { result } = renderHook(() => useFlows());
    expect(result.current.loading).toBe(true);
  });

  it('should have flow management functions', () => {
    const { result } = renderHook(() => useFlows());
    expect(typeof result.current.loadFlows).toBe('function');
    expect(typeof result.current.createNewFlow).toBe('function');
    expect(typeof result.current.deployCurrentFlow).toBe('function');
    expect(typeof result.current.undeployCurrentFlow).toBe('function');
  });

  it('should have node management functions', () => {
    const { result } = renderHook(() => useFlows());
    expect(typeof result.current.addNode).toBe('function');
    expect(typeof result.current.removeNode).toBe('function');
    expect(typeof result.current.updateNode).toBe('function');
    expect(typeof result.current.addConnection).toBe('function');
    expect(typeof result.current.removeConnection).toBe('function');
  });
});
