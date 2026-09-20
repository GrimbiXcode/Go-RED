import { useEffect, useRef, type ReactNode } from 'react';

export interface ContextMenuItem {
  id: string;
  label: string;
  icon?: ReactNode;
  shortcut?: string;
  disabled?: boolean;
  danger?: boolean;
  onSelect?: () => void;
  /** A separator line; other fields are ignored. */
  separator?: boolean;
}

export interface ContextMenuProps {
  x: number;
  y: number;
  items: ContextMenuItem[];
  onClose: () => void;
}

/** Right-click menu anchored at a screen position; closes on outside click, Escape or selection. */
export function ContextMenu({ x, y, items, onClose }: ContextMenuProps) {
  const ref = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const onDown = (event: MouseEvent) => {
      if (ref.current && !ref.current.contains(event.target as Node)) onClose();
    };
    const onKey = (event: KeyboardEvent) => {
      if (event.key === 'Escape') onClose();
    };
    document.addEventListener('mousedown', onDown);
    document.addEventListener('keydown', onKey);
    return () => {
      document.removeEventListener('mousedown', onDown);
      document.removeEventListener('keydown', onKey);
    };
  }, [onClose]);

  // Keep the menu inside the viewport.
  const width = 224;
  const height = items.length * 30 + 8;
  const left = Math.min(x, (typeof window !== 'undefined' ? window.innerWidth : x + width) - width - 8);
  const top = Math.min(y, (typeof window !== 'undefined' ? window.innerHeight : y + height) - height - 8);

  return (
    <div
      ref={ref}
      role="menu"
      className="fixed z-40 w-56 bg-panel text-fg rounded-md shadow-float border border-line py-1 animate-fade-in"
      style={{ left: Math.max(4, left), top: Math.max(4, top) }}
      data-testid="context-menu"
      onContextMenu={(event) => event.preventDefault()}
    >
      {items.map((item, index) =>
        item.separator ? (
          <div key={`sep-${index}`} className="my-1 border-t border-line" />
        ) : (
          <button
            key={item.id}
            role="menuitem"
            className={`w-full text-left px-3 py-1.5 text-xs flex items-center gap-2 hover:bg-accent-soft disabled:opacity-40 disabled:cursor-not-allowed disabled:hover:bg-transparent ${
              item.danger ? 'text-danger-text' : 'text-fg'
            }`}
            disabled={item.disabled}
            onClick={() => {
              item.onSelect?.();
              onClose();
            }}
            data-testid={`context-${item.id}`}
          >
            {item.icon && <span className="w-3.5 h-3.5 shrink-0 text-muted [&>svg]:w-3.5 [&>svg]:h-3.5">{item.icon}</span>}
            <span className="flex-1">{item.label}</span>
            {item.shortcut && <kbd className="text-2xs text-faint font-mono">{item.shortcut}</kbd>}
          </button>
        )
      )}
    </div>
  );
}
