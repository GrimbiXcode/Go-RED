import { useState, useCallback } from 'react';
import { Trans, useTranslation } from 'react-i18next';
import { notify } from '../store/notificationStore';
import { exportFlow } from '../utils/api';

interface ExportModalProps {
  flowId: string;
  flowName: string;
  isOpen: boolean;
  onClose: () => void;
}

export function ExportModal({ flowId, flowName, isOpen, onClose }: ExportModalProps) {
  const { t } = useTranslation();
  const [isExporting, setIsExporting] = useState(false);

  const handleExport = useCallback(async () => {
    try {
      setIsExporting(true);
      await exportFlow(flowId);
      notify('success', t('export.success', { name: flowName }));
      onClose();
    } catch (error) {
      notify('error', t('export.failed', { message: error instanceof Error ? error.message : String(error) }));
    } finally {
      setIsExporting(false);
    }
  }, [flowId, flowName, onClose, t]);

  if (!isOpen) {
    return null;
  }

  return (
    <div className="fixed inset-0 bg-black/40 flex items-center justify-center z-50 p-4" role="dialog" aria-label={t('export.title')}>
      <div className="bg-panel rounded shadow-xl w-full max-w-md">
        <div className="flex items-center justify-between px-4 py-3 border-b border-line">
          <h3 className="font-semibold text-sm text-fg">{t('export.title')}</h3>
          <button className="text-faint hover:text-muted" onClick={onClose} aria-label={t('common.close')}>
            ✕
          </button>
        </div>

        <div className="p-4">
          <p className="text-sm text-muted mb-4">
            <Trans i18nKey="export.text" values={{ name: flowName }} components={{ 1: <strong /> }} />
          </p>

          <div className="bg-surface p-3 rounded mb-4">
            <div className="text-xs text-muted">
              <div>
                <strong>{t('export.flowId')}:</strong> {flowId}
              </div>
              <div>
                <strong>{t('export.fileName')}:</strong> flow-{flowId}.json
              </div>
            </div>
          </div>
        </div>

        <div className="flex items-center justify-end gap-2 px-4 py-3 border-t border-line">
          <button className="px-3 py-1.5 text-xs text-muted hover:bg-sunken rounded" onClick={onClose} disabled={isExporting}>
            {t('export.cancel')}
          </button>
          <button
            className="px-3 py-1.5 bg-accent text-accent-fg rounded hover:bg-accent-strong text-xs font-medium disabled:opacity-50 disabled:cursor-not-allowed"
            onClick={handleExport}
            disabled={isExporting}
          >
            {isExporting ? t('export.exporting') : t('export.action')}
          </button>
        </div>
      </div>
    </div>
  );
}

export default ExportModal;
