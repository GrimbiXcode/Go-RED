import type { Property, Schema } from '../types/generated';

/** Editor controls the tray can render; mirrors the Widget* constants in internal/registry. */
export type Widget =
  | 'text'
  | 'textarea'
  | 'number'
  | 'boolean'
  | 'select'
  | 'typedInput'
  | 'code'
  | 'list'
  | 'keyValue'
  | 'credential'
  | 'duration'
  | 'json'
  | 'stringList'
  | 'nodeSelect';

const WIDGETS: Widget[] = ['text', 'textarea', 'number', 'boolean', 'select', 'typedInput', 'code', 'list', 'keyValue', 'credential', 'duration', 'json', 'stringList', 'nodeSelect'];

export function isWidget(value: unknown): value is Widget {
  return typeof value === 'string' && (WIDGETS as string[]).includes(value);
}

/** A property together with everything the tray needs to render it. */
export interface ResolvedProperty {
  key: string;
  schema: Property;
  label: string;
  widget: Widget;
  group: string;
  order: number;
}

export interface PropertyGroup {
  group: string;
  properties: ResolvedProperty[];
}

export interface SelectOption {
  value: string;
  label: string;
}

/** "keepAliveSec" → "Keep alive sec"; used when a schema has no label. */
export function humanize(key: string): string {
  const spaced = key.replace(/([a-z0-9])([A-Z])/g, '$1 $2').replace(/[_-]+/g, ' ');
  return spaced.charAt(0).toUpperCase() + spaced.slice(1).toLowerCase();
}

/** Picks a control for a property that declares no widget (schema v1 nodes and third parties). */
export function inferWidget(prop: Property): Widget {
  if (prop.enum && prop.enum.length > 0) return 'select';
  switch (prop.type) {
    case 'boolean':
      return 'boolean';
    case 'number':
    case 'integer':
      return 'number';
    case 'object':
    case 'array':
      return 'json';
    default:
      return 'text';
  }
}

export function widgetOf(prop: Property): Widget {
  return isWidget(prop.widget) ? prop.widget : inferWidget(prop);
}

/** Labelled choices of a select property: Options first, Enum as the unlabelled fallback. */
export function optionsOf(prop: Property): SelectOption[] {
  if (prop.options && prop.options.length > 0) return prop.options.map((o) => ({ value: o.value, label: o.label || o.value }));
  return (prop.enum || []).map((value) => ({ value, label: value }));
}

/**
 * Orders a schema's properties for display: explicit order first, then
 * unordered ones alphabetically by label (schema v1 nodes keep a stable
 * order that way).
 */
export function resolveProperties(schema?: Schema | null): ResolvedProperty[] {
  const entries = Object.entries(schema?.properties || {});
  const resolved = entries.map(([key, prop]) => ({
    key,
    schema: prop,
    label: prop.label || humanize(key),
    widget: widgetOf(prop),
    group: prop.group || '',
    order: prop.order || 0,
  }));
  return resolved.sort((a, b) => {
    if (a.order && b.order) return a.order - b.order;
    if (a.order) return -1;
    if (b.order) return 1;
    return a.label.localeCompare(b.label);
  });
}

/** Groups resolved properties, keeping the order of first appearance; the unnamed group comes first. */
export function groupProperties(properties: ResolvedProperty[]): PropertyGroup[] {
  const groups: PropertyGroup[] = [];
  for (const prop of properties) {
    let group = groups.find((g) => g.group === prop.group);
    if (!group) {
      group = { group: prop.group, properties: [] };
      groups.push(group);
    }
    group.properties.push(prop);
  }
  return groups.sort((a, b) => (a.group === '' ? -1 : b.group === '' ? 1 : 0));
}

/** The value a property has when the config does not set it. */
export function defaultOf(prop: Property): unknown {
  if (prop.default !== undefined && prop.default !== null) return prop.default;
  switch (widgetOf(prop)) {
    case 'boolean':
      return false;
    case 'list':
    case 'stringList':
      return [];
    case 'keyValue':
      return {};
    case 'typedInput': {
      const type = prop.typedInput?.default || prop.typedInput?.types?.[0] || 'str';
      return isRefType(type) ? { type, path: '' } : { type, value: '' };
    }
    default:
      return undefined;
  }
}

export function isRefType(type: string): boolean {
  return type === 'msg' || type === 'flow' || type === 'global';
}

/** Fills a config with the schema defaults for properties it does not set. */
export function withDefaults(schema: Schema | null | undefined, config: Record<string, unknown> | undefined): Record<string, unknown> {
  const next: Record<string, unknown> = { ...(config || {}) };
  for (const [key, prop] of Object.entries(schema?.properties || {})) {
    if (next[key] === undefined) {
      const value = defaultOf(prop);
      if (value !== undefined) next[key] = value;
    }
  }
  return next;
}

/** A property is shown unless its visibleWhen rule points at a value the config does not have. */
export function isVisible(prop: Property, config: Record<string, unknown>, schema?: Schema | null): boolean {
  const rule = prop.visibleWhen;
  if (!rule) return true;
  let actual = config[rule.property];
  if (actual === undefined) {
    const other = schema?.properties?.[rule.property];
    actual = other ? defaultOf(other) : undefined;
  }
  const text = actual === undefined || actual === null ? '' : String(actual);
  return rule.values.includes(text);
}
