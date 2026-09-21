import { useEffect, useRef, useState, type KeyboardEvent } from 'react';
import { useTranslation } from 'react-i18next';
import { CopyPlus, Pencil, Plus, Trash2, X } from 'lucide-react';
import type { FlowSummary } from '../types/api';
import { ContextMenu, type ContextMenuItem } from './ContextMenu';

export interface FlowTabsProps {
  flows: FlowSummary[];
  selectedFlowId: string | null;
  onSelectFlow: (flowId: string) => void;
  onCreateFlow: () => void;
  onDeleteFlow: (flowId: string) => void;
  onRenameFlow?: (flowId: string, name: string) => void;
  onDuplicateFlow?: (flowId: string) => void;
  onReorderFlows?: (orderedIds: string[]) => void;
}

const statusDotClass: Record<string, string> = {
  running: 'bg-ok',
  error: 'bg-danger',
};

function statusDot(status: string) {
  return statusDotClass[status] || 'bg-faint';
}

/** True when the draft differs from the running instance. */
export function isModified(flow: FlowSummary): boolean {
  return flow.status === 'running' && !!flow.deployedAt && flow.updatedAt > flow.deployedAt;
}

/**
 * Flow tabs: click selects, double-click renames inline, dragging
 * reorders, right-click opens rename / duplicate / delete, and a dot marks
 * running flows whose draft changed since the last deploy.
 */
export function FlowTabs({ flows, selectedFlowId, onSelectFlow, onCreateFlow, onDeleteFlow, onRenameFlow, onDuplicateFlow, onReorderFlows }: FlowTabsProps) {
  const { t } = useTranslation();
  const [renaming, setRenaming] = useState<{ id: string; value: string } | null>(null);
  const [dragId, setDragId] = useState<string | null>(null);
  const [dropTarget, setDropTarget] = useState<string | null>(null);
  const [menu, setMenu] = useState<{ x: number; y: number; id: string } | null>(null);
  const input = useRef<HTMLInputElement>(null);

  const renamingId = renaming?.id ?? null;
  useEffect(() => {
    if (renamingId) input.current?.select();
  }, [renamingId]);

  const commitRename = () => {
    if (!renaming) return;
    const flow = flows.find((f) => f.id === renaming.id);
    const value = renaming.value.trim();
    if (flow && value && value !== flow.name) onRenameFlow?.(flow.id, value);
    setRenaming(null);
  };

  const onRenameKey = (event: KeyboardEvent<HTMLInputElement>) => {
    if (event.key === 'Enter') commitRename();
    else if (event.key === 'Escape') setRenaming(null);
  };

  const reorder = (draggedId: string, targetId: string) => {
    if (draggedId === targetId) return;
    const ids = flows.map((f) => f.id);
    const from = ids.indexOf(draggedId);
    const to = ids.indexOf(targetId);
    if (from === -1 || to === -1) return;
    ids.splice(from, 1);
    ids.splice(to, 0, draggedId);
    onReorderFlows?.(ids);
  };

  const menuFlow = menu ? flows.find((f) => f.id === menu.id) : undefined;
  const menuItems: ContextMenuItem[] = menuFlow
    ? [
        { id: 'rename', label: t('tabs.rename'), icon: <Pencil />, onSelect: () => setRenaming({ id: menuFlow.id, value: menuFlow.name }) },
        { id: 'duplicate', label: t('tabs.duplicate'), icon: <CopyPlus />, disabled: !onDuplicateFlow, onSelect: () => onDuplicateFlow?.(menuFlow.id) },
        { id: 'sep', label: '', separator: true },
        { id: 'delete', label: t('tabs.deleteFlow'), icon: <Trash2 />, danger: true, onSelect: () => onDeleteFlow(menuFlow.id) },
      ]
    : [];

  return (
    <div className="flex items-center bg-sunken border-b border-line shrink-0 overflow-x-auto" role="tablist">
      <div className="flex items-stretch">
        {flows.map((flow) => {
          const isActive = flow.id === selectedFlowId;
          const isRenaming = renaming?.id === flow.id;
          return (
            <div
              key={flow.id}
              role="tab"
              tabIndex={0}
              aria-selected={isActive}
              draggable={!isRenaming}
              className={`group flex items-center gap-2 px-3 h-8 text-xs font-medium border-r border-line whitespace-nowrap cursor-pointer select-none ${
                isActive ? 'bg-panel text-fg shadow-[inset_0_2px_0_var(--accent)]' : 'text-muted hover:bg-surface hover:text-fg'
              } ${dropTarget === flow.id && dragId !== flow.id ? 'shadow-[inset_2px_0_0_var(--accent)]' : ''}`}
              onClick={() => onSelectFlow(flow.id)}
              onKeyDown={(event) => {
                if (event.key === 'Enter' || event.key === ' ') onSelectFlow(flow.id);
              }}
              onDoubleClick={() => onRenameFlow && setRenaming({ id: flow.id, value: flow.name })}
              onContextMenu={(event) => {
                event.preventDefault();
                setMenu({ x: event.clientX, y: event.clientY, id: flow.id });
              }}
              onDragStart={(event) => {
                setDragId(flow.id);
                event.dataTransfer.effectAllowed = 'move';
                event.dataTransfer.setData('text/plain', flow.id);
              }}
              onDragOver={(event) => {
                if (!dragId) return;
                event.preventDefault();
                setDropTarget(flow.id);
              }}
              onDragLeave={() => setDropTarget((current) => (current === flow.id ? null : current))}
              onDrop={(event) => {
                event.preventDefault();
                if (dragId) reorder(dragId, flow.id);
                setDragId(null);
                setDropTarget(null);
              }}
              onDragEnd={() => {
                setDragId(null);
                setDropTarget(null);
              }}
              title={flow.description || flow.name}
              data-testid={`flow-tab-${flow.id}`}
            >
              <span className={`w-1.5 h-1.5 rounded-full shrink-0 ${statusDot(flow.status)}`} title={t(`flowStatus.${flow.status}`)} />
              {isRenaming ? (
                <input
                  ref={input}
                  className="w-32 px-1 py-0.5 text-xs bg-panel border border-accent rounded focus:outline-none"
                  value={renaming.value}
                  placeholder={t('tabs.renamePlaceholder')}
                  onChange={(event) => setRenaming({ id: flow.id, value: event.target.value })}
                  onBlur={commitRename}
                  onKeyDown={onRenameKey}
                  onClick={(event) => event.stopPropagation()}
                  aria-label={t('tabs.rename')}
                  data-testid="tab-rename-input"
                />
              ) : (
                <span className="max-w-[10rem] truncate">{flow.name}</span>
              )}
              {isModified(flow) && <span className="w-1.5 h-1.5 rounded-full bg-accent shrink-0" title={t('tabs.modified')} data-testid="tab-dirty" />}
              {isActive && !isRenaming && (
                <span
                  role="button"
                  aria-label={t('tabs.deleteFlow')}
                  className="ml-1 rounded hover:bg-surface p-0.5 text-faint hover:text-danger-text opacity-0 group-hover:opacity-100"
                  onClick={(event) => {
                    event.stopPropagation();
                    onDeleteFlow(flow.id);
                  }}
                >
                  <X className="w-3 h-3" aria-hidden="true" />
                </span>
              )}
            </div>
          );
        })}
      </div>
      <button
        className="w-8 h-8 flex items-center justify-center text-muted hover:bg-surface hover:text-fg shrink-0"
        onClick={onCreateFlow}
        title={t('tabs.newFlow')}
        aria-label={t('tabs.newFlow')}
      >
        <Plus className="w-4 h-4" aria-hidden="true" />
      </button>
      {menu && <ContextMenu x={menu.x} y={menu.y} items={menuItems} onClose={() => setMenu(null)} />}
    </div>
  );
}
