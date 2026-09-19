import { describe, it, expect, beforeEach } from 'vitest';
import { useRuntimeStore, selectDebug, selectNodeStatus, selectNodeMetrics, DEBUG_LIMIT } from '../store/runtimeStore';
import type { DebugMessage, FlowSnapshot } from '../types/generated';

function message(id: string, overrides: Partial<DebugMessage> = {}): DebugMessage {
  return { id, flowId: 'f', nodeId: 'n3', level: 'debug', payload: id, timestamp: '2026-09-19T10:00:00Z', ...overrides };
}

function snapshot(overrides: Partial<FlowSnapshot> = {}): FlowSnapshot {
  return {
    flowId: 'f',
    status: 'running',
    nodeStatus: { n1: { fill: 'green', shape: 'dot', text: 'listening' } },
    metrics: { n1: { messages: 3, errors: 1 } },
    debug: [message('a'), message('b')],
    ...overrides,
  };
}

describe('runtimeStore', () => {
  beforeEach(() => {
    useRuntimeStore.setState({ nodeStatus: {}, metrics: {}, debug: {}, subscribed: {} });
  });

  it('a snapshot replaces everything known about a flow', () => {
    useRuntimeStore.getState().applyDebugMessage(message('stale'));
    useRuntimeStore.getState().applySnapshot(snapshot());
    const state = useRuntimeStore.getState();
    expect(selectNodeStatus('f', 'n1')(state)?.text).toBe('listening');
    expect(selectNodeMetrics('f', 'n1')(state)).toEqual({ messages: 3, errors: 1 });
    expect(selectDebug('f')(state).map((m) => m.id)).toEqual(['a', 'b']);
  });

  it('a snapshot only keeps the newest DEBUG_LIMIT entries', () => {
    const debug = Array.from({ length: DEBUG_LIMIT + 10 }, (_, i) => message(`m${i}`));
    useRuntimeStore.getState().applySnapshot(snapshot({ debug }));
    const kept = selectDebug('f')(useRuntimeStore.getState());
    expect(kept).toHaveLength(DEBUG_LIMIT);
    expect(kept[0].id).toBe('m10');
  });

  it('debug messages append per flow, drop duplicates and stay capped', () => {
    const { applyDebugMessage } = useRuntimeStore.getState();
    applyDebugMessage(message('1'));
    applyDebugMessage(message('1'));
    applyDebugMessage(message('2'));
    applyDebugMessage(message('other', { flowId: 'g' }));
    expect(selectDebug('f')(useRuntimeStore.getState()).map((m) => m.id)).toEqual(['1', '2']);
    expect(selectDebug('g')(useRuntimeStore.getState()).map((m) => m.id)).toEqual(['other']);

    for (let i = 0; i < DEBUG_LIMIT + 5; i++) applyDebugMessage(message(`x${i}`));
    const entries = selectDebug('f')(useRuntimeStore.getState());
    expect(entries).toHaveLength(DEBUG_LIMIT);
    expect(entries[entries.length - 1].id).toBe(`x${DEBUG_LIMIT + 4}`);
  });

  it('node status and metrics events update one flow without touching others', () => {
    const { applyNodeStatus, applyMetrics } = useRuntimeStore.getState();
    applyNodeStatus('f', 'n1', { fill: 'red', shape: 'ring', text: 'disconnected' });
    applyNodeStatus('g', 'n1', { fill: 'green', shape: 'dot', text: 'connected' });
    applyMetrics('f', { n1: { messages: 1, errors: 0 } });
    const state = useRuntimeStore.getState();
    expect(selectNodeStatus('f', 'n1')(state)?.fill).toBe('red');
    expect(selectNodeStatus('g', 'n1')(state)?.fill).toBe('green');
    expect(selectNodeMetrics('f', 'n1')(state)?.messages).toBe(1);
    expect(selectNodeMetrics('g', 'n1')(state)).toBeUndefined();
    expect(selectNodeStatus(undefined, 'n1')(state)).toBeUndefined();
  });

  it('a flow that stops loses its node status and counters but keeps its debug log', () => {
    useRuntimeStore.getState().applySnapshot(snapshot());
    useRuntimeStore.getState().applyFlowStatus('f', 'running');
    expect(selectNodeStatus('f', 'n1')(useRuntimeStore.getState())).toBeDefined();

    useRuntimeStore.getState().applyFlowStatus('f', 'draft');
    const state = useRuntimeStore.getState();
    expect(selectNodeStatus('f', 'n1')(state)).toBeUndefined();
    expect(selectNodeMetrics('f', 'n1')(state)).toBeUndefined();
    expect(selectDebug('f')(state)).toHaveLength(2);
  });

  it('clearDebug empties one flow and forgetFlow drops the flow entirely', () => {
    useRuntimeStore.getState().applySnapshot(snapshot());
    useRuntimeStore.getState().setSubscribed('f', true);
    useRuntimeStore.getState().clearDebug('f');
    expect(selectDebug('f')(useRuntimeStore.getState())).toEqual([]);
    expect(useRuntimeStore.getState().subscribed.f).toBe(true);

    useRuntimeStore.getState().forgetFlow('f');
    const state = useRuntimeStore.getState();
    expect(state.nodeStatus.f).toBeUndefined();
    expect(state.metrics.f).toBeUndefined();
    expect(state.debug.f).toBeUndefined();
    expect(state.subscribed.f).toBeUndefined();
  });

  it('selectDebug returns the same empty array for unknown flows (no re-render churn)', () => {
    const state = useRuntimeStore.getState();
    expect(selectDebug('nope')(state)).toBe(selectDebug(undefined)(state));
  });
});
