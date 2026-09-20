import { useTranslation } from 'react-i18next';
import type { FlowSummary } from '../types/api';

export interface FlowTabsProps {
  flows: FlowSummary[];
  selectedFlowId: string | null;
  onSelectFlow: (flowId: string) => void;
  onCreateFlow: () => void;
  onDeleteFlow: (flowId: string) => void;
}

const statusDotClass: Record<string, string> = {
  running: 'bg-ok',
  error: 'bg-danger',
};

function statusDot(status: string) {
  return statusDotClass[status] || 'bg-faint';
}

export function FlowTabs({ flows, selectedFlowId, onSelectFlow, onCreateFlow, onDeleteFlow }: FlowTabsProps) {
  const { t } = useTranslation();

  return (
    <div className="flex items-center bg-sunken border-b border-line shrink-0 overflow-x-auto" role="tablist">
      <div className="flex items-stretch">
        {flows.map((flow) => {
          const isActive = flow.id === selectedFlowId;
          return (
            <button
              key={flow.id}
              role="tab"
              aria-selected={isActive}
              className={`group flex items-center gap-2 px-3 h-8 text-xs font-medium border-r border-line whitespace-nowrap ${
                isActive ? 'bg-panel text-fg shadow-[inset_0_2px_0_var(--accent)]' : 'text-muted hover:bg-surface hover:text-fg'
              }`}
              onClick={() => onSelectFlow(flow.id)}
              title={flow.name}
            >
              <span className={`w-1.5 h-1.5 rounded-full ${statusDot(flow.status)}`} title={t(`flowStatus.${flow.status}`)} />
              <span className="max-w-[10rem] truncate">{flow.name}</span>
              {isActive && (
                <span
                  role="button"
                  aria-label={t('tabs.deleteFlow')}
                  className="ml-1 rounded hover:bg-line px-1 text-faint hover:text-danger-text opacity-0 group-hover:opacity-100"
                  onClick={(event) => {
                    event.stopPropagation();
                    onDeleteFlow(flow.id);
                  }}
                >
                  ×
                </span>
              )}
            </button>
          );
        })}
      </div>
      <button
        className="w-8 h-8 flex items-center justify-center text-muted hover:bg-line shrink-0"
        onClick={onCreateFlow}
        title={t('tabs.newFlow')}
        aria-label={t('tabs.newFlow')}
      >
        +
      </button>
    </div>
  );
}
