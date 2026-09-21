import { useState } from 'react';
import { ChevronDown, ChevronRight } from 'lucide-react';

export interface JsonTreeProps {
  value: unknown;
  /** Key under which the value sits, if any. */
  name?: string;
  depth?: number;
  /** Levels opened by default (0 = collapsed). */
  expandDepth?: number;
}

function isPlainObject(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

function Primitive({ value }: { value: unknown }) {
  if (value === null || value === undefined) return <span className="text-faint">null</span>;
  switch (typeof value) {
    case 'string':
      return <span className="text-ok-text break-all">&quot;{value}&quot;</span>;
    case 'number':
      return <span className="text-accent-text">{String(value)}</span>;
    case 'boolean':
      return <span className="text-warn-text">{String(value)}</span>;
    default:
      return <span className="text-fg">{String(value)}</span>;
  }
}

/** Collapsible rendering of a JSON value, one row per key, in the debug feed. */
export function JsonTree({ value, name, depth = 0, expandDepth = 1 }: JsonTreeProps) {
  const [open, setOpen] = useState(depth < expandDepth);
  const isArray = Array.isArray(value);
  const isObject = isPlainObject(value);

  if (!isArray && !isObject) {
    return (
      <div className="font-mono text-2xs leading-4 flex gap-1" style={{ paddingLeft: depth * 12 }}>
        {name !== undefined && <span className="text-muted shrink-0">{name}:</span>}
        <Primitive value={value} />
      </div>
    );
  }

  const entries: [string, unknown][] = isArray ? (value as unknown[]).map((item, i) => [String(i), item]) : Object.entries(value as Record<string, unknown>);
  const summary = isArray ? `Array(${entries.length})` : `Object{${entries.length}}`;
  const Chevron = open ? ChevronDown : ChevronRight;

  return (
    <div>
      <button
        type="button"
        className="font-mono text-2xs leading-4 flex items-center gap-1 text-left hover:bg-surface rounded pr-1"
        style={{ paddingLeft: Math.max(0, depth * 12 - 4) }}
        onClick={() => setOpen((o) => !o)}
        aria-expanded={open}
        data-testid="json-node"
      >
        <Chevron className="w-3 h-3 text-faint shrink-0" aria-hidden="true" />
        {name !== undefined && <span className="text-muted">{name}:</span>}
        <span className="text-fg">{summary}</span>
      </button>
      {open && entries.map(([key, child]) => <JsonTree key={key} name={key} value={child} depth={depth + 1} expandDepth={expandDepth} />)}
    </div>
  );
}
