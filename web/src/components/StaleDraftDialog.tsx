import { useTranslation } from 'react-i18next';
import { AlertTriangle, Download } from 'lucide-react';
import { useFlowStore } from '../store/flowStore';
import { downloadJson } from '../utils/api';

/**
 * Shown when unsaved work stashed before a sign-in belongs to a flow that
 * someone has saved since: keep it (one undoable step over the server's
 * version), take the server's version, or download it to merge by hand.
 */
export function StaleDraftDialog() {
  const { t } = useTranslation();
  const stash = useFlowStore((state) => state.staleStash);
  const flow = useFlowStore((state) => state.flow);
  const resolve = useFlowStore((state) => state.resolveStaleStash);

  if (!stash || !flow || flow.id !== stash.flowId) return null;

  const download = () => {
    // The flow as the user left it, in Go-RED's export format (importable).
    const { flowId: _flowId, baseUpdatedAt: _base, stashedAt: _at, ...document } = stash;
    downloadJson({ ...flow, ...document }, `flow-${stash.flowId}.unsaved.json`);
  };

  const button = 'px-3 py-1.5 rounded text-xs font-medium';
  return (
    <div className="fixed inset-0 bg-black/40 flex items-center justify-center z-50 p-4" role="dialog" aria-label={t('staleDraft.title')} data-testid="stale-draft">
      <div className="bg-panel rounded-md shadow-float w-full max-w-md">
        <div className="flex items-center gap-2 px-4 py-3 border-b border-line">
          <AlertTriangle className="w-4 h-4 text-warn-text" aria-hidden="true" />
          <h3 className="font-semibold text-sm text-fg">{t('staleDraft.title')}</h3>
        </div>
        <p className="p-4 text-sm text-muted">{t('staleDraft.text', { name: stash.name })}</p>
        <div className="flex flex-wrap items-center justify-end gap-2 px-4 py-3 border-t border-line">
          <button className={`${button} mr-auto flex items-center gap-1.5 text-fg hover:bg-surface`} onClick={download}>
            <Download className="w-3.5 h-3.5" aria-hidden="true" />
            {t('staleDraft.download')}
          </button>
          <button className={`${button} border border-line-strong text-fg hover:bg-surface`} onClick={() => resolve('server')}>
            {t('staleDraft.server')}
          </button>
          <button className={`${button} bg-accent text-accent-fg hover:bg-accent-strong`} onClick={() => resolve('keep')}>
            {t('staleDraft.keep')}
          </button>
        </div>
      </div>
    </div>
  );
}

export default StaleDraftDialog;
