import type { Property, NodeMetadata } from '../../types/generated';
import type { Flow } from '../../types/flow';

/** What a widget may need beyond its own value. */
export interface WidgetContext {
  /** The open flow, for widgets that reference other nodes. */
  flow: Flow | null;
  nodeTypes: NodeMetadata[];
  /** The node being edited; never offered as a reference target. */
  nodeId?: string;
}

export interface WidgetProps {
  /** Unique id of the field, also the key of its error ("rules.0.value"). */
  id: string;
  property: Property;
  value: unknown;
  onChange: (value: unknown) => void;
  /** Widgets holding text the user is still typing (JSON) report parse problems here; null clears. */
  setInvalid: (message: string | null) => void;
  error?: string;
  context: WidgetContext;
}

export const inputClass =
  'w-full px-2 py-1.5 border rounded text-xs border-line bg-panel focus:outline-none focus:ring-2 focus:ring-accent disabled:bg-surface';
export const inputErrorClass = 'border-danger focus:ring-danger';
export const smallButtonClass = 'px-1.5 py-1 text-xs text-muted rounded hover:bg-sunken hover:text-fg disabled:opacity-30';

export function fieldClass(error?: string): string {
  return error ? `${inputClass} ${inputErrorClass}` : inputClass;
}
