import { describe, it, expect, vi } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import { Header } from '../components/Header';
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

describe('FlowTabs', () => {
  const flows: FlowSummary[] = [
    { id: 'a', name: 'Alpha', status: 'running', nodeCount: 2, createdAt: '', updatedAt: '' },
    { id: 'b', name: 'Beta', status: 'draft', nodeCount: 0, createdAt: '', updatedAt: '' },
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

  it('groups nodes by category and expands the first category by default', () => {
    render(<NodePalette nodeTypes={nodeTypes} loading={false} />);
    expect(screen.getByRole('button', { name: /input/i })).toHaveAttribute('aria-expanded', 'true');
    expect(screen.getByRole('button', { name: /output/i })).toHaveAttribute('aria-expanded', 'false');
    expect(screen.getByTestId('palette-node-inject')).toBeInTheDocument();
    expect(screen.queryByTestId('palette-node-debug')).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: /output/i }));
    expect(screen.getByTestId('palette-node-debug')).toBeInTheDocument();
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
