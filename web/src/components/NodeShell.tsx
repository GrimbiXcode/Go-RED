import type { CSSProperties, MouseEvent, ReactNode } from 'react';
import { getCategoryColor, readableTextColor } from '../utils/nodeCategories';
import type { NodeStatus } from '../types/generated';

/**
 * Visual shell of a canvas node (docs/NEXT_LEVEL_PLAN.md, Phase 4): a
 * 36 px card in the category color with an icon well on the left, the
 * label, an optional control on the right, and the Node-RED style
 * {fill, shape, text} status line underneath. Geometry and states live in
 * the `.gr-node*` rules of src/styles/tailwind.css; the fill arrives as
 * the `--node-color` custom property so ports can pick it up too.
 */

export interface WellAction {
  onClick: (event: MouseEvent<HTMLButtonElement>) => void;
  label: string;
  title?: string;
  disabled?: boolean;
}

export interface NodeShellProps {
  category: string;
  label: string;
  icon: ReactNode;
  selected?: boolean;
  disabled?: boolean;
  status?: NodeStatus;
  /** CSS color overriding the category color (NodeMetadata.color). */
  color?: string;
  /** Makes the icon well a button (the Inject node's trigger). */
  wellAction?: WellAction;
  /** Optional small control rendered at the right of the row. */
  action?: ReactNode;
  title?: string;
  /** Port <Handle> elements, positioned against the shell's height. */
  children?: ReactNode;
}

const fillClass: Record<string, string> = {
  red: 'bg-danger border-danger',
  green: 'bg-ok border-ok',
  yellow: 'bg-warn border-warn',
  blue: 'bg-accent border-accent',
  grey: 'bg-faint border-faint',
  gray: 'bg-faint border-faint',
};

/** True when a status has something to show. */
export function hasVisibleStatus(status?: NodeStatus): status is NodeStatus {
  return !!status && (!!status.text || !!status.fill);
}

export function NodeShell({ category, label, icon, selected, disabled, status, color, wellAction, action, title, children }: NodeShellProps) {
  const categoryColor = getCategoryColor(category);
  const showStatus = hasVisibleStatus(status);
  const fill = fillClass[status?.fill || ''] || fillClass.grey;
  const ring = status?.shape === 'ring';
  const style = {
    '--node-color': color || categoryColor.fill,
    '--node-fg': color ? readableTextColor(color) : '#ffffff',
  } as CSSProperties;

  return (
    <div className="relative" style={style} title={title} data-testid="node-shell" data-category={category} data-disabled={disabled ? 'true' : undefined}>
      <div className={`gr-node ${selected ? 'gr-node--selected' : ''} ${disabled ? 'gr-node--disabled' : ''}`}>
        {wellAction ? (
          <button
            type="button"
            className="gr-node__well gr-node__well--button nodrag nopan"
            onClick={wellAction.onClick}
            disabled={wellAction.disabled}
            title={wellAction.title}
            aria-label={wellAction.label}
          >
            {icon}
          </button>
        ) : (
          <div className="gr-node__well">{icon}</div>
        )}
        <span className="gr-node__label">{label}</span>
        {action && <span className="pr-2 shrink-0 flex items-center">{action}</span>}
      </div>

      {children}

      {showStatus && (
        <div className="gr-node__status" data-testid="node-status" data-fill={status.fill || ''}>
          <span className={`w-2 h-2 rounded-full shrink-0 border ${fill} ${ring ? '!bg-transparent' : ''}`} />
          <span className="truncate max-w-[12rem]">{status.text}</span>
        </div>
      )}
    </div>
  );
}
