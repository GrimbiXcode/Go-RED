import type { ReactNode } from 'react';
import { Bug, Info } from 'lucide-react';

/**
 * Right-hand sidebar shell with a Node-RED-style vertical icon tab strip
 * (Phase 4 of docs/FRONTEND_NODE_RED_REDESIGN.md): a narrow strip of tab
 * icons sits fixed at the far right edge, with the active tab's content
 * shown in a panel to its left. Clicking the active tab's icon again
 * collapses the panel. Replaces the old always-visible Sidebar column plus
 * the separately toggled message log overlay (now the DebugPanel tab).
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
        <div className="w-80 bg-panel border-l border-line overflow-y-auto shrink-0">
          {activeTab.content}
        </div>
      )}

      <div className="w-9 bg-sunken border-l border-line flex flex-col items-center py-2 gap-1 shrink-0">
        {tabs.map((tab) => {
          const isActive = tab.id === activeTabId;
          return (
            <button
              key={tab.id}
              className={`w-7 h-7 flex items-center justify-center rounded ${
                isActive ? 'bg-accent-soft text-accent-text' : 'text-muted hover:bg-surface hover:text-fg'
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

export function InfoTabIcon() {
  return <Info className="w-4 h-4" aria-hidden="true" />;
}

export function DebugTabIcon() {
  return <Bug className="w-4 h-4" aria-hidden="true" />;
}
