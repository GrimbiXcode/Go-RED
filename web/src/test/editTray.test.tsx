import { describe, it, expect, beforeEach, vi } from 'vitest';
import { render, screen, fireEvent, within } from '@testing-library/react';
import { NodeEditTray } from '../components/NodeEditTray';
import { useFlowStore } from '../store/flowStore';
import type { NodeMetadata, Property } from '../types/generated';
import type { Flow, FlowNode } from '../types/flow';

function prop(overrides: Partial<Property>): Property {
  return { type: 'string', description: '', default: undefined, enum: [], pattern: '', ...overrides };
}

const switchType: NodeMetadata = {
  id: 'switch',
  type: 'switch',
  name: 'Switch',
  description: 'Routes',
  category: 'function',
  inputs: [{ id: 'input', name: 'Input', description: '', required: true }],
  outputs: [],
  configSchema: {
    properties: {
      checkAll: prop({ type: 'boolean', default: true, label: 'Check all rules', order: 3, widget: 'boolean', description: 'Stop after the first match when off' }),
      property: prop({ type: 'object', default: { type: 'msg', path: 'payload' }, label: 'Property', order: 1, widget: 'typedInput', typedInput: { types: ['msg', 'flow', 'global'], default: 'msg' } }),
      rules: prop({
        type: 'array',
        default: [],
        label: 'Rules',
        order: 2,
        widget: 'list',
        items: {
          properties: {
            operator: prop({ default: 'eq', label: 'Operator', order: 1, widget: 'select', options: [{ value: 'eq', label: '==' }, { value: 'btwn', label: 'is between' }, { value: 'else', label: 'otherwise' }] }),
            value: prop({ type: 'object', default: { type: 'str', value: '' }, label: 'Value', order: 2, widget: 'typedInput', typedInput: { types: ['str', 'num'], default: 'str' }, visibleWhen: { property: 'operator', values: ['eq', 'btwn'] } }),
            value2: prop({ type: 'object', default: { type: 'str', value: '' }, label: 'and', order: 3, widget: 'typedInput', typedInput: { types: ['str', 'num'], default: 'str' }, visibleWhen: { property: 'operator', values: ['btwn'] } }),
          },
          required: ['operator', 'value'],
        },
      }),
    },
    required: [],
  },
  icon: '',
  tags: [],
  help: '**Routes messages** by rules.',
  outputsFrom: { property: 'rules', label: '{{operator}} {{value.value}}' },
};

const httpInType: NodeMetadata = {
  id: 'http in',
  type: 'http in',
  name: 'HTTP In',
  description: '',
  category: 'network',
  inputs: [],
  outputs: [{ id: 'output', name: 'Output', description: '', required: true }],
  configSchema: {
    properties: {
      method: prop({ enum: ['GET', 'POST'], default: 'GET', label: 'Method', order: 1, widget: 'select' }),
      path: prop({ label: 'Path', order: 2, widget: 'text', placeholder: '/hook' }),
      responseTimeoutMs: prop({ type: 'number', default: 30000, label: 'Response timeout', order: 3, widget: 'duration', unit: 'ms' }),
      broker: prop({ label: 'Broker', order: 4, widget: 'nodeSelect', nodeTypes: ['mqtt-broker'] }),
    },
    required: ['path'],
  },
  icon: '',
  tags: [],
};

const functionType: NodeMetadata = {
  id: 'function',
  type: 'function',
  name: 'Function',
  description: '',
  category: 'function',
  inputs: [{ id: 'input', name: 'Input', description: '', required: true }],
  outputs: [{ id: 'output', name: 'Output', description: '', required: true }],
  configSchema: {
    properties: {
      code: prop({ default: 'return input;', label: 'Function', order: 1, widget: 'code', language: 'javascript' }),
      useMsg: prop({ type: 'boolean', default: false, label: 'Expose the message as msg', order: 2, widget: 'boolean' }),
    },
    required: ['code'],
  },
  icon: '',
  tags: [],
};

const flow: Flow = {
  id: 'f',
  name: 'F',
  description: '',
  nodes: {
    sw: { id: 'sw', type: 'switch', position: { x: 0, y: 0 }, config: {}, disabled: false },
    b1: { id: 'b1', type: 'mqtt-broker', name: 'Local broker', position: { x: 0, y: 0 }, config: {}, disabled: false },
    h1: { id: 'h1', type: 'http in', position: { x: 0, y: 0 }, config: { method: 'POST', path: '' }, disabled: false },
    fn: { id: 'fn', type: 'function', position: { x: 0, y: 0 }, config: { code: 'return input;' }, disabled: false },
  },
  connections: [],
  status: 'draft',
  config: { timeout: 30, maxConcurrency: 1, retryPolicy: { maxRetries: 0, backoff: 0, maxBackoff: 0, retryOn: [] }, environment: {} },
  createdAt: '',
  updatedAt: '',
  version: '1.0',
};

function renderTray(node: FlowNode) {
  const onSave = vi.fn();
  const onClose = vi.fn();
  render(<NodeEditTray node={node} onSave={onSave} onClose={onClose} />);
  return { onSave, onClose };
}

describe('NodeEditTray', () => {
  beforeEach(() => {
    useFlowStore.setState({ flow, nodeTypes: [switchType, httpInType, functionType] });
  });

  it('renders schema fields in order, with the name on top and rules editable in a list', () => {
    const { onSave } = renderTray(flow.nodes.sw);
    const tray = screen.getByTestId('node-config');
    expect(within(tray).getByPlaceholderText('Optional display name')).toBeInTheDocument();

    const fields = within(tray).getAllByTestId(/^field-/).map((el) => el.getAttribute('data-testid'));
    expect(fields).toEqual(['field-property', 'field-rules', 'field-checkAll']);
    expect(within(tray).getByText('Stop after the first match when off')).toBeInTheDocument();

    // A new rule shows operator and value, but not the second value.
    fireEvent.click(screen.getByTestId('list-add-rules'));
    const item = screen.getByTestId('list-item-rules-0');
    expect(within(item).getByTestId('field-rules.0.operator')).toBeInTheDocument();
    expect(within(item).getByTestId('field-rules.0.value')).toBeInTheDocument();
    expect(within(item).queryByTestId('field-rules.0.value2')).not.toBeInTheDocument();

    // The empty value is required, so Done is blocked with an inline error.
    const done = screen.getByTestId('config-done');
    expect(done).toBeDisabled();
    expect(within(item).getByTestId('field-error')).toHaveTextContent('Required');

    fireEvent.change(within(item).getByRole('textbox'), { target: { value: 'on' } });
    expect(done).toBeEnabled();

    // "is between" reveals the second value; "otherwise" needs no value.
    fireEvent.change(within(item).getAllByRole('combobox')[0], { target: { value: 'btwn' } });
    expect(within(item).getByTestId('field-rules.0.value2')).toBeInTheDocument();
    fireEvent.change(within(item).getAllByRole('combobox')[0], { target: { value: 'else' } });
    expect(within(item).queryByTestId('field-rules.0.value')).not.toBeInTheDocument();

    fireEvent.click(done);
    expect(onSave).toHaveBeenCalledTimes(1);
    const patch = onSave.mock.calls[0][0];
    expect(patch.config.rules).toEqual([{ operator: 'else', value: { type: 'str', value: 'on' }, value2: { type: 'str', value: '' } }]);
    expect(patch.config.property).toEqual({ type: 'msg', path: 'payload' });
    expect(patch.config.checkAll).toBe(true);
    expect(patch.disabled).toBe(false);
  });

  it('validates required fields, offers node references and converts durations', () => {
    const { onSave } = renderTray(flow.nodes.h1);
    const done = screen.getByTestId('config-done');
    expect(done).toBeDisabled();
    expect(screen.getByTestId('config-problems')).toHaveTextContent('1 problem');
    expect(screen.getByTestId('field-error')).toHaveAttribute('data-field', 'path');

    fireEvent.change(screen.getByPlaceholderText('/hook'), { target: { value: '/e2e' } });
    expect(done).toBeEnabled();

    // Node references list only nodes of the requested types, never the node itself.
    const broker = screen.getByTestId('node-select-broker') as HTMLSelectElement;
    expect(Array.from(broker.options).map((o) => o.textContent)).toEqual(['None', 'Local broker (mqtt-broker)']);
    fireEvent.change(broker, { target: { value: 'b1' } });

    // 30000 ms is shown as 30 s and stored back in ms.
    const duration = screen.getByTestId('duration-responseTimeoutMs');
    expect(within(duration).getByRole('spinbutton')).toHaveValue(30);
    expect(within(duration).getByRole('combobox')).toHaveValue('s');
    fireEvent.change(within(duration).getByRole('spinbutton'), { target: { value: '5' } });

    fireEvent.click(done);
    expect(onSave.mock.calls[0][0].config).toEqual({ method: 'POST', path: '/e2e', responseTimeoutMs: 5000, broker: 'b1' });
  });

  it('edits code in CodeMirror and keeps description and enabled state on their tabs', () => {
    const { onSave } = renderTray(flow.nodes.fn);
    const editor = screen.getByTestId('code-editor');
    expect(editor).toHaveAttribute('data-language', 'javascript');
    expect(editor.querySelector('.cm-content')).toHaveTextContent('return input;');

    fireEvent.click(screen.getByTestId('config-tab-description'));
    fireEvent.change(screen.getByPlaceholderText('Notes about this node (Markdown)'), { target: { value: 'Doubles **everything**' } });
    expect(screen.getByText('No help available for this node type.')).toBeInTheDocument();

    fireEvent.click(screen.getByTestId('config-tab-appearance'));
    fireEvent.click(screen.getByLabelText('Enabled'));

    fireEvent.click(screen.getByTestId('config-done'));
    expect(onSave.mock.calls[0][0]).toMatchObject({ description: 'Doubles **everything**', disabled: true, config: { code: 'return input;', useMsg: false } });
  });

  it('shows the node type help as Markdown', () => {
    renderTray(flow.nodes.sw);
    fireEvent.click(screen.getByTestId('config-tab-description'));
    expect(screen.getByTestId('node-type-help').innerHTML).toContain('<strong>Routes messages</strong>');
  });
});
