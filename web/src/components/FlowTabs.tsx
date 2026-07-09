import type { FlowSummary } from '../types/api';

export interface FlowTabsProps {
  flows: FlowSummary[];
  selectedFlowId: string | null;
  onSelectFlow: (flowId: string) => void;
  onCreateNewFlow: () => void;
  onDeleteSelectedFlow: () => void;
}

const statusDotClass: Record<string, string> = {
  running: 'bg-gr-blue-500',
  deployed: 'bg-gr-blue-500',
  error: 'bg-gr-fuchsia-500',
};

function statusDot(status: string) {
  return statusDotClass[status] || 'bg-gray-300';
}

export function FlowTabs({
  flows,
  selectedFlowId,
  onSelectFlow,
  onCreateNewFlow,
  onDeleteSelectedFlow,
}: FlowTabsProps) {
  return (
    <div className="flex items-center bg-gray-100 border-b border-gray-300 shrink-0 overflow-x-auto">
      <div className="flex items-stretch">
        {flows.map((flow) => {
          const isActive = flow.id === selectedFlowId;
          return (
            <button
              key={flow.id}
              className={`group flex items-center gap-2 px-3 h-8 text-xs font-medium border-r border-gray-300 whitespace-nowrap ${
                isActive
                  ? 'bg-white text-gray-800'
                  : 'text-gray-600 hover:bg-gray-200'
              }`}
              onClick={() => onSelectFlow(flow.id)}
              title={flow.name}
            >
              <span className={`w-1.5 h-1.5 rounded-full ${statusDot(flow.status)}`} />
              <span className="max-w-[10rem] truncate">{flow.name}</span>
              {isActive && (
                <span
                  role="button"
                  aria-label="Flow löschen"
                  className="ml-1 rounded hover:bg-gray-200 px-1 text-gray-400 hover:text-gr-fuchsia-600 opacity-0 group-hover:opacity-100"
                  onClick={(e) => {
                    e.stopPropagation();
                    onDeleteSelectedFlow();
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
        className="w-8 h-8 flex items-center justify-center text-gray-500 hover:bg-gray-200 shrink-0"
        onClick={onCreateNewFlow}
        title="Neuer Flow"
        aria-label="Neuer Flow"
      >
        +
      </button>
    </div>
  );
}
