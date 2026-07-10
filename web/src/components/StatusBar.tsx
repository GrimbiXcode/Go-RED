import type { Flow } from '../types/flow';
import { WebSocketStatus } from './WebSocketStatus';

export interface StatusBarProps {
  flow: Flow | null;
  // True for a couple seconds right after a drag-and-drop node move is
  // confirmed persisted by the backend - see useFlows' positionSavedAt.
  showPositionSaved?: boolean;
}

/**
 * Slim bottom status bar (Phase 6 of docs/FRONTEND_NODE_RED_REDESIGN.md,
 * optional/stretch item): connection status moves here from the header,
 * next to the active flow's node/connection counts — a dezent equivalent
 * of Node-RED's own status bar instead of a prominent header badge.
 */
export function StatusBar({ flow, showPositionSaved }: StatusBarProps) {
  const nodeCount = flow ? Object.keys(flow.nodes || {}).length : 0;
  const connectionCount = flow ? (flow.connections || []).length : 0;

  return (
    <div className="h-6 flex items-center justify-between px-3 bg-gray-100 border-t border-gray-200 text-[10px] text-gray-500 shrink-0">
      <div className="flex items-center gap-2">
        <WebSocketStatus variant="dot-light" />
        <span
          className={`text-gr-blue-600 transition-opacity duration-300 ${
            showPositionSaved ? 'opacity-100' : 'opacity-0'
          }`}
        >
          ✓ Position gespeichert
        </span>
      </div>
      {flow && (
        <span>
          {nodeCount} node{nodeCount !== 1 ? 's' : ''} · {connectionCount} connection{connectionCount !== 1 ? 's' : ''}
        </span>
      )}
    </div>
  );
}
