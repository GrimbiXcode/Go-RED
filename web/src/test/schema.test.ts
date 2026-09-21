import { describe, it, expect } from 'vitest';
import type { NodeMetadata, Property, Schema } from '../types/generated';
import {
  humanize,
  inferWidget,
  resolveProperties,
  groupProperties,
  isVisible,
  withDefaults,
  defaultOf,
  optionsOf,
} from '../schema/properties';
import { validateConfig, isEmptyValue } from '../schema/validate';
import { outputPortsFor, outputPortIds, renderPortLabel, connectionsOnMissingPorts } from '../schema/ports';
import { renderMarkdown } from '../utils/markdown';

const t = (key: string, options?: Record<string, unknown>) => (options ? `${key}:${JSON.stringify(options)}` : key);

function prop(overrides: Partial<Property>): Property {
  return { type: 'string', description: '', default: undefined, enum: [], pattern: '', ...overrides };
}

const switchSchema: Schema = {
  properties: {
    checkAll: prop({ type: 'boolean', default: true, label: 'Check all rules', order: 3, widget: 'boolean' }),
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
          value: prop({ type: 'object', label: 'Value', order: 2, widget: 'typedInput', typedInput: { types: ['str', 'num'], default: 'str' }, visibleWhen: { property: 'operator', values: ['eq', 'btwn'] } }),
          value2: prop({ type: 'object', label: 'and', order: 3, widget: 'typedInput', typedInput: { types: ['str', 'num'], default: 'str' }, visibleWhen: { property: 'operator', values: ['btwn'] } }),
        },
        required: ['operator', 'value'],
      },
    }),
  },
  required: [],
};

const switchMeta: NodeMetadata = {
  id: 'switch',
  type: 'switch',
  name: 'Switch',
  description: '',
  category: 'function',
  inputs: [{ id: 'input', name: 'Input', description: '', required: true }],
  outputs: [{ id: '0', name: '1', description: '', required: false }, { id: '1', name: '2', description: '', required: false }],
  configSchema: switchSchema,
  icon: '',
  tags: [],
  outputsFrom: { property: 'rules', label: '{{operator}} {{value.value}}' },
};

describe('property resolution', () => {
  it('humanizes keys and infers widgets for schema v1 properties', () => {
    expect(humanize('keepAliveSec')).toBe('Keep alive sec');
    expect(humanize('url')).toBe('Url');
    expect(inferWidget(prop({ enum: ['a', 'b'] }))).toBe('select');
    expect(inferWidget(prop({ type: 'boolean' }))).toBe('boolean');
    expect(inferWidget(prop({ type: 'number' }))).toBe('number');
    expect(inferWidget(prop({ type: 'object' }))).toBe('json');
    expect(inferWidget(prop({}))).toBe('text');
  });

  it('orders by explicit order, then alphabetically, and groups by first appearance', () => {
    const schema: Schema = {
      properties: {
        zeta: prop({ label: 'Zeta' }),
        alpha: prop({ label: 'Alpha' }),
        second: prop({ order: 2, group: 'Connection' }),
        first: prop({ order: 1, group: 'Connection' }),
        widgetless: prop({ order: 3, widget: 'slider' as never }),
      },
      required: [],
    };
    const resolved = resolveProperties(schema);
    expect(resolved.map((p) => p.key)).toEqual(['first', 'second', 'widgetless', 'alpha', 'zeta']);
    expect(resolved.find((p) => p.key === 'widgetless')?.widget).toBe('text');
    const groups = groupProperties(resolved);
    expect(groups.map((g) => g.group)).toEqual(['', 'Connection']);
    expect(groups[1].properties.map((p) => p.key)).toEqual(['first', 'second']);
  });

  it('applies visibleWhen against the config or the other property default', () => {
    const schema: Schema = {
      properties: {
        mode: prop({ enum: ['a', 'b'], default: 'a' }),
        gap: prop({ type: 'number', visibleWhen: { property: 'mode', values: ['b'] } }),
        flag: prop({ type: 'boolean', default: false }),
        detail: prop({ visibleWhen: { property: 'flag', values: ['true'] } }),
      },
      required: [],
    };
    expect(isVisible(schema.properties.gap, {}, schema)).toBe(false);
    expect(isVisible(schema.properties.gap, { mode: 'b' }, schema)).toBe(true);
    expect(isVisible(schema.properties.detail, { flag: true }, schema)).toBe(true);
    expect(isVisible(schema.properties.detail, {}, schema)).toBe(false);
  });

  it('fills defaults, including typed input and list defaults', () => {
    expect(defaultOf(prop({ type: 'object', widget: 'typedInput', typedInput: { types: ['msg', 'str'], default: 'str' } }))).toEqual({ type: 'str', value: '' });
    expect(defaultOf(prop({ type: 'object', widget: 'typedInput', typedInput: { types: ['msg'] } }))).toEqual({ type: 'msg', path: '' });
    const filled = withDefaults(switchSchema, { rules: [{ operator: 'else' }] });
    expect(filled.checkAll).toBe(true);
    expect(filled.property).toEqual({ type: 'msg', path: 'payload' });
    expect(filled.rules).toEqual([{ operator: 'else' }]);
    expect(optionsOf(prop({ enum: ['x'] }))).toEqual([{ value: 'x', label: 'x' }]);
  });
});

describe('validateConfig', () => {
  it('reports required, range, pattern, select and typed input problems', () => {
    const schema: Schema = {
      properties: {
        url: prop({ pattern: '^https?://', order: 1 }),
        port: prop({ type: 'number', min: 1, max: 65535, order: 2 }),
        mode: prop({ enum: ['a', 'b'], order: 3 }),
        amount: prop({ type: 'object', widget: 'typedInput', typedInput: { types: ['num', 'json'] }, order: 4 }),
        wait: prop({ type: 'number', widget: 'duration', unit: 'ms', order: 5 }),
        hidden: prop({ visibleWhen: { property: 'mode', values: ['b'] }, order: 6 }),
      },
      required: ['url', 'port', 'hidden'],
    };
    expect(validateConfig(schema, { url: 'http://x', port: 80 }, t)).toEqual({});
    expect(validateConfig(schema, { url: 'ftp://x', port: 0, mode: 'c', amount: { type: 'num', value: 'abc' }, wait: 'soon' }, t)).toEqual({
      url: 'validation.pattern:{"pattern":"^https?://"}',
      port: 'validation.min:{"min":1}',
      mode: 'validation.oneOf',
      amount: 'validation.number',
      wait: 'validation.number',
    });
    expect(validateConfig(schema, { url: '  ', mode: 'b', amount: { type: 'json', value: '{' } }, t)).toEqual({
      url: 'validation.required',
      port: 'validation.required',
      hidden: 'validation.required',
      amount: 'validation.json',
    });
  });

  it('validates list items with their own schema and skips hidden item fields', () => {
    const errors = validateConfig(switchSchema, { rules: [{ operator: 'eq', value: { type: 'str', value: '' } }, { operator: 'else' }, { operator: 'btwn', value: { type: 'num', value: '1' }, value2: { type: 'num', value: 'x' } }] }, t);
    expect(errors).toEqual({ 'rules.0.value': 'validation.required', 'rules.2.value2': 'validation.number' });
  });

  it('knows what counts as empty', () => {
    expect(isEmptyValue(undefined)).toBe(true);
    expect(isEmptyValue('  ')).toBe(true);
    expect(isEmptyValue([])).toBe(true);
    expect(isEmptyValue({})).toBe(true);
    expect(isEmptyValue(0)).toBe(false);
    expect(isEmptyValue(false)).toBe(false);
    expect(isEmptyValue({ type: 'str', value: '' }, 'typedInput')).toBe(true);
    expect(isEmptyValue({ type: 'msg', path: 'payload' }, 'typedInput')).toBe(false);
  });
});

describe('dynamic ports', () => {
  it('derives one output per rule with option labels, none without rules', () => {
    expect(outputPortsFor(switchMeta, { rules: [] })).toEqual([]);
    const ports = outputPortsFor(switchMeta, { rules: [{ operator: 'eq', value: { type: 'str', value: 'on' } }, { operator: 'else' }] });
    expect(ports.map((p) => [p.id, p.name])).toEqual([['0', '== on'], ['1', 'otherwise']]);
    expect(outputPortsFor({ ...switchMeta, outputsFrom: { property: 'rules', min: 1 } }, {}).map((p) => p.name)).toEqual(['1']);
    expect(outputPortsFor({ ...switchMeta, outputsFrom: undefined }, {})).toEqual(switchMeta.outputs);
  });

  it('renders label templates with nested paths and falls back to the index', () => {
    expect(renderPortLabel('{{a.b}}', { a: { b: 'x' } }, 0)).toBe('x');
    expect(renderPortLabel('{{missing}}', {}, 4)).toBe('5');
    expect(renderPortLabel(undefined, {}, 1)).toBe('2');
  });

  it('uses the default output handle for port-less types and finds dangling connections', () => {
    expect(outputPortIds({ ...switchMeta, inputs: [], outputs: [], outputsFrom: undefined }, {})).toEqual(['output']);
    expect(outputPortIds(switchMeta, { rules: [{ operator: 'else' }] })).toEqual(['0']);
    const connections = [
      { id: 'c1', sourceNode: 'sw', sourcePort: '0', targetNode: 'a', targetPort: 'input' },
      { id: 'c2', sourceNode: 'sw', sourcePort: '1', targetNode: 'b', targetPort: 'input' },
      { id: 'c3', sourceNode: 'other', sourcePort: '1', targetNode: 'b', targetPort: 'input' },
    ];
    expect(connectionsOnMissingPorts(connections, 'sw', ['0']).map((c) => c.id)).toEqual(['c2']);
  });
});

describe('renderMarkdown', () => {
  it('renders the supported subset and strips scripts', () => {
    const html = renderMarkdown('# Title\n\nSome **bold** and `code` with a [link](https://x.y).\n\n* one\n* two\n\n```\nlet a = 1 < 2;\n```\n<script>alert(1)</script>');
    expect(html).toContain('<h4>Title</h4>');
    expect(html).toContain('<strong>bold</strong>');
    expect(html).toContain('<code>code</code>');
    expect(html).toContain('<a href="https://x.y" target="_blank" rel="noopener noreferrer">link</a>');
    expect(html).toContain('<ul><li>one</li><li>two</li></ul>');
    expect(html).toContain('<pre><code>let a = 1 &lt; 2;</code></pre>');
    expect(html).not.toContain('<script');
  });
});
