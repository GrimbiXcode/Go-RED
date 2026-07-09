import type { ReactNode } from 'react';
import { getCategoryColor } from '../utils/nodeCategories';

/**
 * Shared visual shell for canvas node cards (Phase 3 of
 * docs/FRONTEND_NODE_RED_REDESIGN.md): a single flat, category-colored row
 * instead of the old white-card-with-colored-header-bar, a dashed Gopher
 * Blue outline on selection, and an optional status line below the node
 * instead of a status dot in the header. Used by NodeComponent, InjectNode
 * and DebugNode so the three don't each re-implement this.
 */

export interface NodeStatusLike {
  state: string;
  message?: string;
}

export interface NodeShellProps {
  category: string;
  label: string;
  icon: ReactNode;
  selected?: boolean;
  status?: NodeStatusLike;
  /** Optional small control rendered at the right of the row (e.g. Inject's trigger button). */
  action?: ReactNode;
  title?: string;
  /** Port <Handle> elements — rendered so their default top:50% positioning
   * (see NodeHandles) lines up with this shell's colored row. */
  children?: ReactNode;
}

const statusDotClass: Record<string, string> = {
  error: 'bg-gr-fuchsia-500',
  processing: 'bg-gr-skyblue-500',
  completed: 'bg-gr-blue-500',
  running: 'bg-gr-blue-500',
  deployed: 'bg-gr-blue-500',
};

export function NodeShell({ category, label, icon, selected, status, action, title, children }: NodeShellProps) {
  const color = getCategoryColor(category);
  const showStatus = !!status?.state && status.state !== 'idle';

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
        <div className="mt-1 flex items-center justify-center gap-1 text-[10px] text-gray-600 whitespace-nowrap">
          <span className={`w-1.5 h-1.5 rounded-full shrink-0 ${statusDotClass[status!.state] || 'bg-gray-400'}`} />
          <span className="truncate">{status!.message || status!.state}</span>
        </div>
      )}
    </div>
  );
}
