import type { NodeMetadata, Port, Schema } from '../types/generated';
import type { NodeConnection } from '../types/flow';
import { optionsOf } from './properties';

function getPath(value: unknown, path: string): unknown {
  return path.split('.').reduce<unknown>((current, segment) => {
    if (current === null || current === undefined || typeof current !== 'object') return undefined;
    return (current as Record<string, unknown>)[segment];
  }, value);
}

/**
 * Renders an output label template ("{{operator}} {{value.value}}") for one
 * list element. A placeholder that names a select property of the item
 * schema shows the option's label. Blank results fall back to the index.
 */
export function renderPortLabel(template: string | undefined, item: unknown, index: number, itemSchema?: Schema | null): string {
  const fallback = String(index + 1);
  if (!template) return fallback;
  const text = template
    .replace(/\{\{\s*([\w.]+)\s*\}\}/g, (_match, path: string) => {
      const raw = getPath(item, path);
      if (raw === undefined || raw === null) return '';
      const prop = itemSchema?.properties?.[path];
      if (prop) {
        const option = optionsOf(prop).find((o) => o.value === String(raw));
        if (option) return option.label;
      }
      return typeof raw === 'object' ? JSON.stringify(raw) : String(raw);
    })
    .replace(/\s+/g, ' ')
    .trim();
  return text || fallback;
}

/** Output ports of a node: fixed from the metadata, or one per element of the outputsFrom property. */
export function outputPortsFor(metadata: NodeMetadata | null | undefined, config: Record<string, unknown> | undefined): Port[] {
  if (!metadata) return [];
  const from = metadata.outputsFrom;
  if (!from) return metadata.outputs || [];
  const raw = config?.[from.property];
  const items = Array.isArray(raw) ? raw : [];
  const count = Math.max(items.length, from.min || 0);
  const itemSchema = metadata.configSchema?.properties?.[from.property]?.items;
  return Array.from({ length: count }, (_, index) => ({
    id: String(index),
    name: renderPortLabel(from.label, items[index], index, itemSchema),
    description: '',
    required: false,
  }));
}

/** IDs of the output handles the canvas draws for a node (the default "output" when a type declares no ports). */
export function outputPortIds(metadata: NodeMetadata | null | undefined, config: Record<string, unknown> | undefined): string[] {
  if (!metadata) return [];
  const outputs = outputPortsFor(metadata, config);
  const noPortsDefined = (metadata.inputs || []).length === 0 && outputs.length === 0 && !metadata.outputsFrom;
  return noPortsDefined ? ['output'] : outputs.map((port) => port.id);
}

/** Connections leaving a node from a port it no longer has. */
export function connectionsOnMissingPorts(connections: NodeConnection[], nodeId: string, portIds: string[]): NodeConnection[] {
  const known = new Set(portIds);
  return connections.filter((c) => c.sourceNode === nodeId && !known.has(c.sourcePort || 'output'));
}
