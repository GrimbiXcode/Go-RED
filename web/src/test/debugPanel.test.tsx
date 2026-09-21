import { describe, it, expect, beforeEach } from 'vitest';
import { render, screen, fireEvent, act } from '@testing-library/react';
import { DebugPanel, formatPayload, filterDebug } from '../components/DebugPanel';
import { useRuntimeStore } from '../store/runtimeStore';
import { useFlowStore } from '../store/flowStore';
import type { DebugMessage } from '../types/generated';

function message(id: string, overrides: Partial<DebugMessage> = {}): DebugMessage {
  return { id, flowId: 'f', nodeId: 'n3', nodeName: 'Out', nodeType: 'debug', level: 'debug', payload: { count: 1 }, timestamp: '2026-09-19T10:00:00Z', ...overrides };
}

describe('formatPayload', () => {
  it('shows primitives as text and everything else as pretty JSON', () => {
    expect(formatPayload('tick')).toEqual({ text: 'tick', kind: 'string' });
    expect(formatPayload(42)).toEqual({ text: '42', kind: 'number' });
    expect(formatPayload(true)).toEqual({ text: 'true', kind: 'boolean' });
    expect(formatPayload(null)).toEqual({ text: 'null', kind: 'null' });
    expect(formatPayload(undefined).kind).toBe('null');
    expect(formatPayload({ a: 1 })).toEqual({ text: '{\n  "a": 1\n}', kind: 'json' });
  });
});

describe('filterDebug', () => {
  const entries = [
    message('1', { nodeName: 'Out', topic: 'sensors/temp', payload: { value: 21 } }),
    message('2', { nodeName: 'Errors', nodeId: 'fn', level: 'error', payload: 'boom' }),
  ];

  it('matches node name, node id, topic and payload text, case-insensitively', () => {
    expect(filterDebug(entries, '')).toBe(entries);
    expect(filterDebug(entries, 'TEMP').map((m) => m.id)).toEqual(['1']);
    expect(filterDebug(entries, 'fn').map((m) => m.id)).toEqual(['2']);
    expect(filterDebug(entries, 'boom').map((m) => m.id)).toEqual(['2']);
    expect(filterDebug(entries, '"value": 21').map((m) => m.id)).toEqual(['1']);
    expect(filterDebug(entries, 'nothing')).toEqual([]);
  });
});

describe('DebugPanel', () => {
  beforeEach(() => {
    useRuntimeStore.setState({ nodeStatus: {}, metrics: {}, debug: {}, subscribed: {} });
    useFlowStore.setState({ flow: null });
  });

  it('asks for a flow when none is open', () => {
    render(<DebugPanel />);
    expect(screen.getByText('Select a flow to view its details')).toBeInTheDocument();
    expect(screen.queryByTestId('debug-live')).not.toBeInTheDocument();
  });

  it('shows the empty hint, the live indicator and the entry count', () => {
    render(<DebugPanel flowId="f" />);
    expect(screen.getByText(/Nothing yet/)).toBeInTheDocument();
    expect(screen.getByTestId('debug-live')).toHaveAttribute('data-live', 'false');
    expect(screen.getByTestId('debug-summary')).toHaveTextContent('0 entries');

    act(() => useRuntimeStore.getState().setSubscribed('f', true));
    expect(screen.getByTestId('debug-live')).toHaveAttribute('data-live', 'true');
  });

  it('renders live entries with node label, topic, level and payload', () => {
    render(<DebugPanel flowId="f" />);
    act(() => {
      useRuntimeStore.getState().applyDebugMessage(message('1', { topic: 'sensors/temp', payload: 'warm' }));
      useRuntimeStore.getState().applyDebugMessage(message('2', { nodeName: undefined, nodeType: undefined, nodeId: 'fn', level: 'error', payload: 'boom' }));
    });

    const entries = screen.getAllByTestId('debug-message');
    expect(entries).toHaveLength(2);
    expect(entries[0]).toHaveAttribute('data-level', 'debug');
    expect(entries[0]).toHaveTextContent('Out');
    expect(entries[0]).toHaveTextContent('sensors/temp');
    expect(entries[0]).toHaveTextContent('warm');
    expect(entries[1]).toHaveAttribute('data-level', 'error');
    expect(entries[1]).toHaveTextContent('error');
    expect(entries[1]).toHaveTextContent('fn');
    expect(screen.getByTestId('debug-summary')).toHaveTextContent('2 entries');
  });

  it('falls back to the flow definition for the node label', () => {
    useFlowStore.setState({
      flow: {
        id: 'f',
        name: 'F',
        description: '',
        nodes: { fn: { id: 'fn', type: 'function', name: 'Transform', position: { x: 0, y: 0 }, config: {}, disabled: false } },
        connections: [],
        status: 'running',
        config: { timeout: 30, maxConcurrency: 1, retryPolicy: { maxRetries: 0, backoff: 0, maxBackoff: 0, retryOn: [] }, environment: {} },
        createdAt: '',
        updatedAt: '',
        version: '1.0',
      },
    });
    render(<DebugPanel flowId="f" />);
    act(() => useRuntimeStore.getState().applyDebugMessage(message('1', { nodeName: undefined, nodeType: undefined, nodeId: 'fn', level: 'error', payload: 'boom' })));
    expect(screen.getByTestId('debug-message')).toHaveTextContent('Transform');
  });

  it('filters entries and clears them', () => {
    useRuntimeStore.getState().applyDebugMessage(message('1', { payload: 'alpha' }));
    useRuntimeStore.getState().applyDebugMessage(message('2', { payload: 'beta' }));
    render(<DebugPanel flowId="f" />);
    expect(screen.getAllByTestId('debug-message')).toHaveLength(2);

    fireEvent.change(screen.getByRole('searchbox'), { target: { value: 'beta' } });
    expect(screen.getAllByTestId('debug-message')).toHaveLength(1);
    expect(screen.getByTestId('debug-summary')).toHaveTextContent('1 entry of 2');

    fireEvent.change(screen.getByRole('searchbox'), { target: { value: 'zzz' } });
    expect(screen.getByText('No entries match the filter')).toBeInTheDocument();

    fireEvent.change(screen.getByRole('searchbox'), { target: { value: '' } });
    fireEvent.click(screen.getByRole('button', { name: 'Clear' }));
    expect(screen.queryAllByTestId('debug-message')).toHaveLength(0);
    expect(screen.getByText(/Nothing yet/)).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Clear' })).toBeDisabled();
  });
});
