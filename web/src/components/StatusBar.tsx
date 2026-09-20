import { useTranslation } from 'react-i18next';
import type { Flow } from '../types/flow';
import type { SaveState } from '../store/flowStore';
import { WebSocketStatus } from './WebSocketStatus';

export interface StatusBarProps {
  flow: Flow | null;
  saveState: SaveState;
  saveError: string | null;
}

/**
 * Slim bottom status bar: connection status, autosave state and the active
 * flow's node/connection counts.
 */
export function StatusBar({ flow, saveState, saveError }: StatusBarProps) {
  const { t } = useTranslation();
  const nodeCount = flow ? Object.keys(flow.nodes || {}).length : 0;
  const connectionCount = flow ? (flow.connections || []).length : 0;

  let saveLabel: string | null = null;
  let saveClass = 'text-muted';
  if (flow) {
    switch (saveState) {
      case 'saving':
        saveLabel = t('status.saving');
        break;
      case 'pending':
        saveLabel = t('status.pending');
        break;
      case 'error':
        saveLabel = t('status.saveFailed', { message: saveError || '' });
        saveClass = 'text-danger-text';
        break;
      default:
        saveLabel = t('status.saved');
    }
  }

  return (
    <div className="h-6 flex items-center justify-between px-3 bg-sunken border-t border-line text-2xs text-muted shrink-0">
      <div className="flex items-center gap-3">
        <WebSocketStatus variant="dot-light" />
        {saveLabel && (
          <span className={saveClass} data-testid="save-state" data-state={saveState}>
            {saveLabel}
          </span>
        )}
      </div>
      {flow && <span>{t('status.counts', { nodes: nodeCount, connections: connectionCount })}</span>}
    </div>
  );
}
