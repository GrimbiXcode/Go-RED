import { useState, useCallback, useRef, type ChangeEvent } from 'react';
import { useTranslation } from 'react-i18next';
import { notify } from '../store/notificationStore';
import { importFlow } from '../utils/api';
import { useFlowStore } from '../store/flowStore';

interface ImportModalProps {
  isOpen: boolean;
  onClose: () => void;
  onFlowImported: (flowId: string) => void;
}

interface Preview {
  format: 'go-red' | 'node-red';
  name: string;
  description: string;
  flows: number;
  nodes: number;
  connections: number;
  unsupported: string[];
}

/** Summarizes a Node-RED export (array of nodes with wires) for the preview. */
export function previewNodeRed(data: unknown[], knownTypes: Set<string>): Preview {
  const tabs = data.filter((item) => (item as { type?: string })?.type === 'tab') as { label?: string; info?: string }[];
  const nodes = data.filter((item) => {
    const type = (item as { type?: string })?.type;
    return type && type !== 'tab' && type !== 'group' && type !== 'subflow' && !type.startsWith('subflow:');
  }) as { type: string; wires?: unknown[][] }[];
  const unsupported = Array.from(new Set(nodes.map((n) => n.type).filter((type) => !knownTypes.has(type)))).sort();
  const connections = nodes.reduce((sum, n) => sum + (n.wires || []).reduce((s, targets) => s + (Array.isArray(targets) ? targets.length : 0), 0), 0);
  return {
    format: 'node-red',
    name: tabs.map((tab) => tab.label).filter(Boolean).join(', ') || 'Imported flow',
    description: tabs[0]?.info || '',
    flows: Math.max(tabs.length, 1),
    nodes: nodes.length,
    connections,
    unsupported,
  };
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
          if (Array.isArray(data)) {
            setPreview(previewNodeRed(data, new Set(useFlowStore.getState().nodeTypes.map((nt) => nt.type))));
            return;
          }
          if (!data || typeof data.name !== 'string' || !data.name) {
            setError(t('import.missingName'));
            return;
          }
          setPreview({
            format: 'go-red',
            name: data.name,
            description: data.description || '',
            flows: 1,
            nodes: data.nodes ? Object.keys(data.nodes).length : 0,
            connections: Array.isArray(data.connections) ? data.connections.length : 0,
            unsupported: [],
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
      if (result.format === 'node-red') notify('success', t('import.successMany', { count: result.flows.length }));
      else notify('success', t('import.success', { name: result.name }));
      if (result.warnings.length > 0) {
        notify('warning', t('import.warningsToast', { count: result.warnings.length, first: result.warnings[0] }), 12000);
      }
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
      <div className="bg-panel rounded shadow-xl w-full max-w-md">
        <div className="flex items-center justify-between px-4 py-3 border-b border-line">
          <h3 className="font-semibold text-sm text-fg">{t('import.title')}</h3>
          <button className="text-faint hover:text-muted" onClick={onClose} aria-label={t('common.close')}>
            ✕
          </button>
        </div>

        <div className="p-4">
          <p className="text-sm text-muted mb-4">{t('import.text')}</p>

          <div className="mb-4">
            <label className="block text-xs font-medium text-fg mb-2">{t('import.fileLabel')}</label>
            <div className="flex gap-2">
              <input type="file" ref={fileInputRef} onChange={handleFileChange} accept=".json" className="flex-1 text-xs" disabled={isImporting} />
              <button
                className="px-3 py-1.5 bg-sunken text-fg rounded hover:bg-line text-xs"
                onClick={() => fileInputRef.current?.click()}
                disabled={isImporting}
              >
                {t('import.browse')}
              </button>
            </div>
          </div>

          {error && <div className="bg-danger-soft text-danger-text p-3 rounded mb-4 text-xs">{t('import.errorPrefix', { message: error })}</div>}

          {preview && (
            <div className="bg-accent-soft p-3 rounded mb-4">
              <h4 className="font-medium text-accent-text mb-2 text-xs">{t('import.preview')}</h4>
              <div className="text-xs text-fg space-y-1" data-testid="import-preview" data-format={preview.format}>
                <div>
                  <strong>{t('import.format')}:</strong> {preview.format === 'node-red' ? t('import.formatNodeRed') : t('import.formatGoRed')}
                </div>
                {preview.format === 'node-red' && (
                  <div>
                    <strong>{t('import.flows')}:</strong> {preview.flows}
                  </div>
                )}
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
                {preview.unsupported.length > 0 && (
                  <div className="text-warn-text" data-testid="import-unsupported">
                    <strong>{t('import.unsupported')}:</strong> {preview.unsupported.join(', ')}
                  </div>
                )}
              </div>
            </div>
          )}
        </div>

        <div className="flex items-center justify-end gap-2 px-4 py-3 border-t border-line">
          <button className="px-3 py-1.5 text-xs text-muted hover:bg-sunken rounded" onClick={onClose} disabled={isImporting}>
            {t('import.cancel')}
          </button>
          <button
            className="px-3 py-1.5 text-xs text-danger-text hover:bg-danger-soft rounded disabled:opacity-50 disabled:cursor-not-allowed disabled:hover:bg-transparent"
            onClick={handleReset}
            disabled={isImporting || !selectedFile}
          >
            {t('import.clear')}
          </button>
          <button
            className="px-3 py-1.5 bg-accent text-accent-fg rounded hover:bg-accent-strong text-xs font-medium disabled:opacity-50 disabled:cursor-not-allowed"
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
