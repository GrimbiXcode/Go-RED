import { useState, useCallback } from 'react';
import { Trans, useTranslation } from 'react-i18next';
import { notify } from '../store/notificationStore';
import { exportFlow, type ExportFormat } from '../utils/api';

interface ExportModalProps {
  flowId: string;
  flowName: string;
  isOpen: boolean;
  onClose: () => void;
}

export function ExportModal({ flowId, flowName, isOpen, onClose }: ExportModalProps) {
  const { t } = useTranslation();
  const [isExporting, setIsExporting] = useState(false);
  const [format, setFormat] = useState<ExportFormat>('go-red');

  const handleExport = useCallback(async () => {
    try {
      setIsExporting(true);
      await exportFlow(flowId, format);
      notify('success', t('export.success', { name: flowName }));
      onClose();
    } catch (error) {
      notify('error', t('export.failed', { message: error instanceof Error ? error.message : String(error) }));
    } finally {
      setIsExporting(false);
    }
  }, [flowId, flowName, format, onClose, t]);

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

          <fieldset className="mb-4">
            <legend className="text-xs font-medium text-fg mb-2">{t('export.format')}</legend>
            <div className="space-y-1.5">
              {(['go-red', 'node-red'] as ExportFormat[]).map((option) => (
                <label key={option} className="flex items-start gap-2 text-xs text-fg cursor-pointer">
                  <input type="radio" name="export-format" className="mt-0.5 accent-accent" value={option} checked={format === option} onChange={() => setFormat(option)} />
                  <span>
                    {option === 'go-red' ? t('export.formatGoRed') : t('export.formatNodeRed')}
                    {option === 'node-red' && <span className="block text-2xs text-muted">{t('export.formatNodeRedHint')}</span>}
                  </span>
                </label>
              ))}
            </div>
          </fieldset>

          <div className="bg-surface p-3 rounded mb-4">
            <div className="text-xs text-muted">
              <div>
                <strong>{t('export.flowId')}:</strong> {flowId}
              </div>
              <div>
                <strong>{t('export.fileName')}:</strong> flow-{flowId}
                {format === 'node-red' ? '.node-red' : ''}.json
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
