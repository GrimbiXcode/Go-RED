import { describe, it, expect, vi, afterEach } from 'vitest';
import { cleanup, render, screen, fireEvent, waitFor } from '@testing-library/react';
import { Header } from '../components/Header';
import { useAuthStore } from '../lib/auth';
import { StaleDraftDialog } from '../components/StaleDraftDialog';
import { useFlowStore, __resetFlowStoreForTests } from '../store/flowStore';
import type { Flow } from '../types/flow';
import { FlowTabs } from '../components/FlowTabs';
import { NodePalette } from '../components/NodePalette';
import { StatusBar } from '../components/StatusBar';
import type { NodeMetadata } from '../types/node';
import type { FlowSummary } from '../types/api';

const noop = () => {};

function headerProps(overrides: Partial<Parameters<typeof Header>[0]> = {}) {
  return {
    hasFlow: true,
    hasChanges: false,
    canDeploy: false,
    canUndeploy: false,
    canUndo: false,
    canRedo: false,
    onDeploy: noop,
    onUndeploy: noop,
    onUndo: noop,
    onRedo: noop,
    onExport: noop,
    onImport: noop,
    ...overrides,
  };
}

describe('Header', () => {
  it('disables Deploy when there is nothing to deploy and enables it with changes', () => {
    const onDeploy = vi.fn();
    const { rerender } = render(<Header {...headerProps({ onDeploy })} />);
    const deploy = screen.getByTestId('deploy-button');
    expect(deploy).toBeDisabled();
    expect(deploy).toHaveTextContent('Deploy');

    rerender(<Header {...headerProps({ onDeploy, canDeploy: true, hasChanges: true })} />);
    expect(deploy).toBeEnabled();
    expect(deploy).toHaveTextContent('● Deploy');
    fireEvent.click(deploy);
    expect(onDeploy).toHaveBeenCalledTimes(1);
  });

  it('wires undo/redo buttons and disables them without history', () => {
    const onUndo = vi.fn();
    const onRedo = vi.fn();
    render(<Header {...headerProps({ onUndo, onRedo, canUndo: true, canRedo: false })} />);
    fireEvent.click(screen.getByRole('button', { name: 'Undo' }));
    expect(onUndo).toHaveBeenCalledTimes(1);
    expect(screen.getByRole('button', { name: 'Redo' })).toBeDisabled();
  });

  it('opens the menu with export, import and language entries', () => {
    const onImport = vi.fn();
    render(<Header {...headerProps({ onImport })} />);
    fireEvent.click(screen.getByRole('button', { name: 'Main menu' }));
    expect(screen.getByRole('menu')).toBeInTheDocument();
    fireEvent.click(screen.getByRole('menuitem', { name: 'Import…' }));
    expect(onImport).toHaveBeenCalledTimes(1);
    expect(screen.queryByRole('menu')).not.toBeInTheDocument();
  });
});

describe('Header account menu', () => {
  afterEach(() => {
    cleanup(); // unmount before resetting the store the header listens to
    useAuthStore.setState({ user: null });
    vi.unstubAllGlobals();
  });

  it('shows the signed-in OIDC user and signs out through the API', async () => {
    useAuthStore.setState({ user: { method: 'oidc', subject: 'u1', name: 'Ada Lovelace', email: 'ada@example.com' } });
    const fetchMock = vi.fn(async () => ({ ok: true, status: 204, json: async () => ({}) }));
    vi.stubGlobal('fetch', fetchMock);
    const reload = vi.fn();
    vi.stubGlobal('location', { ...window.location, reload });

    render(<Header {...headerProps()} />);
    fireEvent.click(screen.getByRole('button', { name: 'Main menu' }));
    expect(screen.getByTestId('signed-in-user')).toHaveTextContent('Ada Lovelace');
    fireEvent.click(screen.getByRole('menuitem', { name: 'Sign out' }));

    await waitFor(() => expect(reload).toHaveBeenCalledTimes(1));
    const [url, options] = fetchMock.mock.calls[0] as unknown as [string, RequestInit];
    expect(url).toBe('/api/auth/logout');
    expect(options.method).toBe('POST');
  });

  it('has no account section without an OIDC user', () => {
    useAuthStore.setState({ user: { method: 'token' } });
    render(<Header {...headerProps()} />);
    fireEvent.click(screen.getByRole('button', { name: 'Main menu' }));
    expect(screen.queryByTestId('signed-in-user')).not.toBeInTheDocument();
    expect(screen.queryByRole('menuitem', { name: 'Sign out' })).not.toBeInTheDocument();
  });
});

describe('StaleDraftDialog', () => {
  const flow = {
    id: 'f1',
    name: 'Theirs',
    description: '',
    nodes: {},
    connections: [],
    status: 'draft',
    config: { timeout: 30, maxConcurrency: 0, retryPolicy: { maxRetries: 0, backoff: 1, maxBackoff: 1, retryOn: [] }, environment: {} },
    createdAt: '2026-01-01T00:00:00Z',
    updatedAt: '2026-01-01T00:05:00Z',
    revision: 'r1',
    version: '1',
  } as Flow;
  const stash = { flowId: 'f1', name: 'Mine', description: '', nodes: {}, connections: [], config: flow.config, baseUpdatedAt: '2026-01-01T00:00:00Z', stashedAt: 1 };

  afterEach(() => {
    cleanup();
    __resetFlowStoreForTests();
    vi.unstubAllGlobals();
  });

  it('stays hidden without a stale stash', () => {
    useFlowStore.setState({ flow, staleStash: null });
    render(<StaleDraftDialog />);
    expect(screen.queryByTestId('stale-draft')).not.toBeInTheDocument();
  });

  it('offers keep, server and download', () => {
    const resolveStaleStash = vi.fn();
    useFlowStore.setState({ flow, staleStash: stash, resolveStaleStash });
    const createObjectURL = vi.fn((blob: Blob) => (blob ? 'blob:x' : ''));
    vi.stubGlobal('URL', { ...URL, createObjectURL, revokeObjectURL: vi.fn() });
    render(<StaleDraftDialog />);
    expect(screen.getByTestId('stale-draft')).toHaveTextContent('“Mine”');

    fireEvent.click(screen.getByRole('button', { name: 'Download my changes' }));
    expect(createObjectURL).toHaveBeenCalledTimes(1);
    expect(resolveStaleStash).not.toHaveBeenCalled();

    fireEvent.click(screen.getByRole('button', { name: 'Use the server version' }));
    expect(resolveStaleStash).toHaveBeenCalledWith('server');
    fireEvent.click(screen.getByRole('button', { name: 'Keep my changes' }));
    expect(resolveStaleStash).toHaveBeenCalledWith('keep');
  });
});

describe('FlowTabs', () => {
  const flows: FlowSummary[] = [
    { id: 'a', name: 'Alpha', status: 'running', nodeCount: 2, createdAt: '', updatedAt: '', revision: 'r1' },
    { id: 'b', name: 'Beta', status: 'draft', nodeCount: 0, createdAt: '', updatedAt: '', revision: 'r1' },
  ];

  it('renders a tab per flow, marks the selected one and reports clicks', () => {
    const onSelectFlow = vi.fn();
    const onCreateFlow = vi.fn();
    render(<FlowTabs flows={flows} selectedFlowId="a" onSelectFlow={onSelectFlow} onCreateFlow={onCreateFlow} onDeleteFlow={noop} />);

    const tabs = screen.getAllByRole('tab');
    expect(tabs).toHaveLength(2);
    expect(tabs[0]).toHaveAttribute('aria-selected', 'true');

    fireEvent.click(screen.getByRole('tab', { name: /Beta/ }));
    expect(onSelectFlow).toHaveBeenCalledWith('b');

    fireEvent.click(screen.getByRole('button', { name: 'New flow' }));
    expect(onCreateFlow).toHaveBeenCalledTimes(1);
  });

  it('offers deletion only on the active tab', () => {
    const onDeleteFlow = vi.fn();
    render(<FlowTabs flows={flows} selectedFlowId="b" onSelectFlow={noop} onCreateFlow={noop} onDeleteFlow={onDeleteFlow} />);
    const deleteButtons = screen.getAllByRole('button', { name: 'Delete flow' });
    expect(deleteButtons).toHaveLength(1);
    fireEvent.click(deleteButtons[0]);
    expect(onDeleteFlow).toHaveBeenCalledWith('b');
  });
});

describe('NodePalette', () => {
  const nodeTypes: NodeMetadata[] = [
    { id: 'inject', type: 'inject', name: 'Inject', description: 'Injects a message', category: 'input', inputs: [], outputs: [], configSchema: { properties: {}, required: [] }, icon: '', tags: ['trigger'] },
    { id: 'debug', type: 'debug', name: 'Debug', description: 'Prints messages', category: 'output', inputs: [], outputs: [], configSchema: { properties: {}, required: [] }, icon: '', tags: [] },
    { id: 'function', type: 'function', name: 'Function', description: 'Runs JavaScript', category: 'function', inputs: [], outputs: [], configSchema: { properties: {}, required: [] }, icon: '', tags: ['javascript'] },
  ];

  it('groups nodes by category, all open by default, and remembers collapsed categories', () => {
    window.localStorage.removeItem('go-red.palette.collapsed');
    const { unmount } = render(<NodePalette nodeTypes={nodeTypes} loading={false} />);
    expect(screen.getByRole('button', { name: /^input/i })).toHaveAttribute('aria-expanded', 'true');
    expect(screen.getByRole('button', { name: /^output/i })).toHaveAttribute('aria-expanded', 'true');
    expect(screen.getByTestId('palette-node-inject')).toBeInTheDocument();
    expect(screen.getByTestId('palette-node-debug')).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: /^output/i }));
    expect(screen.queryByTestId('palette-node-debug')).not.toBeInTheDocument();
    expect(JSON.parse(window.localStorage.getItem('go-red.palette.collapsed') || '[]')).toEqual(['output']);

    unmount();
    render(<NodePalette nodeTypes={nodeTypes} loading={false} />);
    expect(screen.queryByTestId('palette-node-debug')).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: /^output/i }));
    expect(screen.getByTestId('palette-node-debug')).toBeInTheDocument();
    window.localStorage.removeItem('go-red.palette.collapsed');
  });

  it('focuses the search box on / unless the user is typing', () => {
    render(<NodePalette nodeTypes={nodeTypes} loading={false} />);
    const search = screen.getByRole('searchbox', { name: 'Search nodes…' });
    fireEvent.keyDown(window, { key: '/' });
    expect(search).toHaveFocus();
    search.blur();
    const other = document.createElement('input');
    document.body.appendChild(other);
    other.focus();
    fireEvent.keyDown(other, { key: '/' });
    expect(search).not.toHaveFocus();
    other.remove();
  });

  it('filters by name, type, description and tag and expands matches', () => {
    render(<NodePalette nodeTypes={nodeTypes} loading={false} />);
    const search = screen.getByRole('searchbox', { name: 'Search nodes…' });

    fireEvent.change(search, { target: { value: 'javascript' } });
    expect(screen.getByTestId('palette-node-function')).toBeInTheDocument();
    expect(screen.queryByTestId('palette-node-inject')).not.toBeInTheDocument();

    fireEvent.change(search, { target: { value: 'nothing-matches' } });
    expect(screen.getByText('No nodes match your search')).toBeInTheDocument();
  });

  it('puts the node type into the drag payload', () => {
    render(<NodePalette nodeTypes={nodeTypes} loading={false} />);
    const setData = vi.fn();
    fireEvent.dragStart(screen.getByTestId('palette-node-inject'), { dataTransfer: { setData, effectAllowed: '' } });
    expect(setData).toHaveBeenCalledWith('application/reactflow', JSON.stringify({ nodeType: 'inject' }));
  });
});

describe('StatusBar', () => {
  const flow = {
    id: 'f',
    name: 'F',
    description: '',
    nodes: { a: { id: 'a', type: 'inject', position: { x: 0, y: 0 }, config: {}, disabled: false } },
    connections: [],
    status: 'draft' as const,
    config: { timeout: 30, maxConcurrency: 1, retryPolicy: { maxRetries: 0, backoff: 0, maxBackoff: 0, retryOn: [] }, environment: {} },
    createdAt: '',
    updatedAt: '',
    revision: 'r1',
    version: '1.0',
  };

  it('shows the autosave state and the node counts', () => {
    const { rerender } = render(<StatusBar flow={flow} saveState="idle" saveError={null} />);
    expect(screen.getByTestId('save-state')).toHaveTextContent('All changes saved');
    expect(screen.getByText('1 nodes · 0 connections')).toBeInTheDocument();

    rerender(<StatusBar flow={flow} saveState="error" saveError="disk full" />);
    expect(screen.getByTestId('save-state')).toHaveTextContent('Save failed: disk full');
  });
});
