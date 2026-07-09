import type { ReactNode } from 'react';

/**
 * Right-hand sidebar shell with a Node-RED-style vertical icon tab strip
 * (Phase 4 of docs/FRONTEND_NODE_RED_REDESIGN.md): a narrow strip of tab
 * icons sits fixed at the far right edge, with the active tab's content
 * shown in a panel to its left. Clicking the active tab's icon again
 * collapses the panel. Replaces the old always-visible Sidebar column plus
 * the separately toggled MessageLogPanel overlay.
 */

export interface SidebarTabDef {
  id: string;
  label: string;
  icon: ReactNode;
  content: ReactNode;
}

export interface SidebarTabsProps {
  tabs: SidebarTabDef[];
  activeTabId: string | null;
  onSelectTab: (id: string | null) => void;
}

export function SidebarTabs({ tabs, activeTabId, onSelectTab }: SidebarTabsProps) {
  const activeTab = tabs.find((t) => t.id === activeTabId) || null;

  return (
    <div className="flex h-full shrink-0">
      {activeTab && (
        <div className="w-80 bg-white border-l border-gray-200 overflow-y-auto">
          {activeTab.content}
        </div>
      )}

      <div className="w-9 bg-gray-100 border-l border-gray-200 flex flex-col items-center py-2 gap-1 shrink-0">
        {tabs.map((tab) => {
          const isActive = tab.id === activeTabId;
          return (
            <button
              key={tab.id}
              className={`w-7 h-7 flex items-center justify-center rounded ${
                isActive ? 'bg-gr-blue-100 text-gr-blue-700' : 'text-gray-500 hover:bg-gray-200'
              }`}
              onClick={() => onSelectTab(isActive ? null : tab.id)}
              title={tab.label}
              aria-pressed={isActive}
            >
              {tab.icon}
            </button>
          );
        })}
      </div>
    </div>
  );
}

const iconProps = {
  viewBox: '0 0 24 24',
  fill: 'none' as const,
  stroke: 'currentColor',
  strokeWidth: 2,
  strokeLinecap: 'round' as const,
  strokeLinejoin: 'round' as const,
  className: 'w-4 h-4',
};

export function InfoTabIcon() {
  return (
    <svg {...iconProps} aria-hidden="true">
      <circle cx="12" cy="12" r="9" />
      <line x1="12" y1="11" x2="12" y2="16" />
      <circle cx="12" cy="7.5" r="0.75" fill="currentColor" stroke="none" />
    </svg>
  );
}

export function DebugTabIcon() {
  return (
    <svg {...iconProps} aria-hidden="true">
      <rect x="3" y="4" width="18" height="16" rx="2" />
      <polyline points="7 9 10 12 7 15" />
      <line x1="13" y1="15" x2="17" y2="15" />
    </svg>
  );
}
