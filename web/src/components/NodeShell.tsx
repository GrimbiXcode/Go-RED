import type { ReactNode } from 'react';
import { getCategoryColor } from '../utils/nodeCategories';
import type { NodeStatus } from '../types/generated';

/**
 * Shared visual shell for canvas node cards: a single flat, category-colored
 * row, a dashed Gopher Blue outline on selection, and an optional status
 * line below the node (Node-RED's {fill, shape, text} indicator). Used by
 * NodeComponent, InjectNode and DebugNode so the three don't each
 * re-implement this.
 */

export interface NodeShellProps {
  category: string;
  label: string;
  icon: ReactNode;
  selected?: boolean;
  status?: NodeStatus;
  /** Optional small control rendered at the right of the row (e.g. Inject's trigger button). */
  action?: ReactNode;
  title?: string;
  /** Port <Handle> elements — rendered so their default top:50% positioning
   * (see NodeHandles) lines up with this shell's colored row. */
  children?: ReactNode;
}

const fillClass: Record<string, string> = {
  red: 'bg-gr-fuchsia-500 border-gr-fuchsia-500',
  green: 'bg-emerald-500 border-emerald-500',
  yellow: 'bg-amber-400 border-amber-400',
  blue: 'bg-gr-blue-500 border-gr-blue-500',
  grey: 'bg-gray-400 border-gray-400',
  gray: 'bg-gray-400 border-gray-400',
};

/** True when a status has something to show. */
export function hasVisibleStatus(status?: NodeStatus): status is NodeStatus {
  return !!status && (!!status.text || !!status.fill);
}

export function NodeShell({ category, label, icon, selected, status, action, title, children }: NodeShellProps) {
  const color = getCategoryColor(category);
  const showStatus = hasVisibleStatus(status);
  const fill = fillClass[status?.fill || ''] || fillClass.grey;
  const ring = status?.shape === 'ring';

  return (
    <div style={{ minWidth: 'var(--gr-node-min-width)' }} title={title}>
      <div
        className={`flex items-center gap-2 px-2 text-white rounded-gr-node ${color.swatch} ${
          selected ? 'outline outline-2 outline-dashed outline-gr-blue-500 outline-offset-2' : ''
        }`}
        style={{ height: 'var(--gr-node-height)' }}
      >
        {icon}
        <span className="text-sm font-medium truncate flex-1">{label}</span>
        {action}
      </div>

      {children}

      {showStatus && (
        <div
          className="mt-1 flex items-center gap-1 text-[10px] text-gray-600 whitespace-nowrap"
          data-testid="node-status"
          data-fill={status.fill || ''}
        >
          <span className={`w-2 h-2 rounded-full shrink-0 border ${fill} ${ring ? '!bg-transparent' : ''}`} />
          <span className="truncate max-w-[12rem]">{status.text}</span>
        </div>
      )}
    </div>
  );
}
