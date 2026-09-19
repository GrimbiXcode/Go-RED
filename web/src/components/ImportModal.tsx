import { useState, useCallback, useRef, type ChangeEvent } from 'react';
import { useTranslation } from 'react-i18next';
import { notify } from '../store/notificationStore';
import { importFlow } from '../utils/api';

interface ImportModalProps {
  isOpen: boolean;
  onClose: () => void;
  onFlowImported: (flowId: string) => void;
}

interface Preview {
  name: string;
  description: string;
  nodes: number;
  connections: number;
}

export function ImportModal({ isOpen, onClose, onFlowImported }: ImportModalProps) {
  const { t } = useTranslation();
  const [isImporting, setIsImporting] = useState(false);
  const [selectedFile, setSelectedFile] = useState<File | null>(null);
  const [preview, setPreview] = useState<Preview | null>(null);
  const [error, setError] = useState<string>('');
  const fileInputRef = useRef<HTMLInputElement>(null);

  const handleFileChange = useCallback(
    (event: ChangeEvent<HTMLInputElement>) => {
      const file = event.target.files?.[0];
      if (!file) {
        setError(t('import.noFile'));
        return;
      }
      if (!file.name.endsWith('.json')) {
        setError(t('import.notJson'));
        return;
      }

      setSelectedFile(file);
      setError('');
      setPreview(null);

      const reader = new FileReader();
      reader.onload = (loadEvent) => {
        try {
          const data = JSON.parse(String(loadEvent.target?.result ?? ''));
          if (!data || typeof data.name !== 'string' || !data.name) {
            setError(t('import.missingName'));
            return;
          }
          setPreview({
            name: data.name,
            description: data.description || '',
            nodes: data.nodes ? Object.keys(data.nodes).length : 0,
            connections: Array.isArray(data.connections) ? data.connections.length : 0,
          });
        } catch {
          setError(t('import.invalidJson'));
        }
      };
      reader.readAsText(file);
    },
    [t]
  );

  const handleImport = useCallback(async () => {
    if (!selectedFile) {
      setError(t('import.noFile'));
      return;
    }
    try {
      setIsImporting(true);
      setError('');
      const result = await importFlow(selectedFile);
      notify('success', t('import.success', { name: result.name }));
      onFlowImported(result.flowId);
      onClose();
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      setError(message);
      notify('error', t('import.failed', { message }));
    } finally {
      setIsImporting(false);
    }
  }, [selectedFile, onClose, onFlowImported, t]);

  const handleReset = useCallback(() => {
    setSelectedFile(null);
    setPreview(null);
    setError('');
    if (fileInputRef.current) fileInputRef.current.value = '';
  }, []);

  if (!isOpen) {
    return null;
  }

  return (
    <div className="fixed inset-0 bg-black/40 flex items-center justify-center z-50 p-4" role="dialog" aria-label={t('import.title')}>
      <div className="bg-white rounded shadow-xl w-full max-w-md">
        <div className="flex items-center justify-between px-4 py-3 border-b border-gray-200">
          <h3 className="font-semibold text-sm text-gray-800">{t('import.title')}</h3>
          <button className="text-gray-400 hover:text-gray-600" onClick={onClose} aria-label={t('common.close')}>
            ✕
          </button>
        </div>

        <div className="p-4">
          <p className="text-sm text-gray-600 mb-4">{t('import.text')}</p>

          <div className="mb-4">
            <label className="block text-xs font-medium text-gray-700 mb-2">{t('import.fileLabel')}</label>
            <div className="flex gap-2">
              <input type="file" ref={fileInputRef} onChange={handleFileChange} accept=".json" className="flex-1 text-xs" disabled={isImporting} />
              <button
                className="px-3 py-1.5 bg-gray-100 text-gray-700 rounded hover:bg-gray-200 text-xs"
                onClick={() => fileInputRef.current?.click()}
                disabled={isImporting}
              >
                {t('import.browse')}
              </button>
            </div>
          </div>

          {error && <div className="bg-gr-fuchsia-50 text-gr-fuchsia-700 p-3 rounded mb-4 text-xs">{t('import.errorPrefix', { message: error })}</div>}

          {preview && (
            <div className="bg-gr-blue-50 p-3 rounded mb-4">
              <h4 className="font-medium text-gr-blue-800 mb-2 text-xs">{t('import.preview')}</h4>
              <div className="text-xs text-gray-700 space-y-1">
                <div>
                  <strong>{t('import.name')}:</strong> {preview.name}
                </div>
                <div>
                  <strong>{t('import.description')}:</strong> {preview.description || t('import.none')}
                </div>
                <div>
                  <strong>{t('import.nodes')}:</strong> {preview.nodes}
                </div>
                <div>
                  <strong>{t('import.connections')}:</strong> {preview.connections}
                </div>
              </div>
            </div>
          )}
        </div>

        <div className="flex items-center justify-end gap-2 px-4 py-3 border-t border-gray-200">
          <button className="px-3 py-1.5 text-xs text-gray-600 hover:bg-gray-100 rounded" onClick={onClose} disabled={isImporting}>
            {t('import.cancel')}
          </button>
          <button
            className="px-3 py-1.5 text-xs text-gr-fuchsia-600 hover:bg-gr-fuchsia-50 rounded disabled:opacity-50 disabled:cursor-not-allowed disabled:hover:bg-transparent"
            onClick={handleReset}
            disabled={isImporting || !selectedFile}
          >
            {t('import.clear')}
          </button>
          <button
            className="px-3 py-1.5 bg-gr-blue-500 text-white rounded hover:bg-gr-blue-600 text-xs font-medium disabled:opacity-50 disabled:cursor-not-allowed"
            onClick={handleImport}
            disabled={isImporting || !selectedFile || !!error}
          >
            {isImporting ? t('import.importing') : t('import.action')}
          </button>
        </div>
      </div>
    </div>
  );
}

export default ImportModal;
