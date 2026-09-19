import { useState, useCallback } from 'react';
import { useTranslation } from 'react-i18next';
import type { FlowNode } from '../types/flow';
import type { PropertySchema } from '../types/node';
import { getCategoryColor } from '../utils/nodeCategories';
import { useFlowStore } from '../store/flowStore';

interface NodeConfigModalProps {
  node: FlowNode;
  onClose: () => void;
  onSave: (config: Record<string, any>, name: string) => void;
  onDelete?: () => void;
}

const inputClass =
  'w-full px-2 py-1.5 border rounded text-xs border-gray-300 focus:outline-none focus:ring-2 focus:ring-gr-blue-500';

/**
 * Node edit tray: slides in from the right at the same width/position as
 * the Info/Debug sidebar, instead of a centered modal dialog with a
 * backdrop, so the rest of the editor stays usable.
 */
export function NodeConfigModal({ node, onClose, onSave, onDelete }: NodeConfigModalProps) {
  const { t } = useTranslation();
  const nodeTypes = useFlowStore((state) => state.nodeTypes);
  const [config, setConfig] = useState<Record<string, any>>({ ...node.config });
  const [name, setName] = useState(node.name || '');

  const nodeMetadata = nodeTypes.find((nt) => nt.type === node.type);
  const category = nodeMetadata?.category || 'custom';
  const color = getCategoryColor(category);

  const handleInputChange = useCallback((key: string, value: any) => {
    setConfig((prev) => ({ ...prev, [key]: value }));
  }, []);

  const handleSubmit = useCallback(
    (event?: React.FormEvent) => {
      event?.preventDefault();
      onSave(config, name.trim());
    },
    [config, name, onSave]
  );

  const renderInputField = (key: string, schema: PropertySchema, value: any) => {
    const label = schema.description || key;
    switch (schema.type) {
      case 'number':
      case 'integer':
        return (
          <div key={key}>
            <label className="block text-xs font-medium text-gray-600 mb-1">{label}</label>
            <input
              type="number"
              value={value ?? ''}
              onChange={(event) =>
                handleInputChange(
                  key,
                  event.target.value === ''
                    ? undefined
                    : schema.type === 'integer'
                      ? parseInt(event.target.value, 10)
                      : parseFloat(event.target.value)
                )
              }
              className={inputClass}
              min={schema.min ?? undefined}
              max={schema.max ?? undefined}
              placeholder={schema.default != null ? String(schema.default) : ''}
            />
          </div>
        );
      case 'boolean':
        return (
          <div key={key} className="flex items-center gap-2">
            <input
              id={`cfg-${key}`}
              type="checkbox"
              checked={!!value}
              onChange={(event) => handleInputChange(key, event.target.checked)}
              className="h-4 w-4"
            />
            <label htmlFor={`cfg-${key}`} className="text-xs font-medium text-gray-600">
              {label}
            </label>
          </div>
        );
      case 'object':
      case 'array': {
        const empty = schema.type === 'object' ? {} : [];
        return (
          <div key={key}>
            <label className="block text-xs font-medium text-gray-600 mb-1">{label}</label>
            <textarea
              defaultValue={JSON.stringify(value ?? empty, null, 2)}
              onChange={(event) => {
                try {
                  handleInputChange(key, JSON.parse(event.target.value));
                } catch {
                  // Keep the last valid value while the JSON is being edited.
                }
              }}
              className={`${inputClass} font-mono`}
              rows={schema.type === 'object' ? 5 : 3}
              placeholder={schema.type === 'object' ? t('config.jsonObject') : t('config.jsonArray')}
            />
          </div>
        );
      }
      case 'string':
      default: {
        const multiline = key === 'code' || key === 'template' || key === 'script';
        return (
          <div key={key}>
            <label className="block text-xs font-medium text-gray-600 mb-1">{label}</label>
            {multiline ? (
              <textarea
                value={value ?? ''}
                onChange={(event) => handleInputChange(key, event.target.value)}
                className={`${inputClass} font-mono`}
                rows={8}
                spellCheck={false}
              />
            ) : (
              <input
                type="text"
                value={value ?? ''}
                onChange={(event) => handleInputChange(key, event.target.value)}
                className={inputClass}
                placeholder={schema.default != null ? String(schema.default) : ''}
              />
            )}
          </div>
        );
      }
    }
  };

  const renderEnumSelector = (key: string, schema: PropertySchema, value: any) => (
    <div key={key}>
      <label className="block text-xs font-medium text-gray-600 mb-1">{schema.description || key}</label>
      <select value={value ?? schema.default ?? ''} onChange={(event) => handleInputChange(key, event.target.value)} className={inputClass}>
        {schema.enum?.map((option) => (
          <option key={option} value={option}>
            {option}
          </option>
        ))}
      </select>
    </div>
  );

  const properties = Object.entries(nodeMetadata?.configSchema?.properties || {}).sort(([a], [b]) => a.localeCompare(b));

  return (
    <div
      className="fixed top-11 bottom-6 right-9 w-80 bg-white border-l border-gray-200 shadow-xl z-30 flex flex-col animate-slide-in-right"
      role="dialog"
      aria-label={nodeMetadata?.name || node.type}
      data-testid="node-config"
    >
      <div className="flex items-center gap-2 px-3 py-2 border-b border-gray-200">
        <span className={`w-3 h-3 rounded-sm shrink-0 ${color.swatch}`} aria-hidden="true" />
        <div className="flex-1 min-w-0">
          <div className="text-sm font-semibold text-gray-800 truncate">{nodeMetadata?.name || node.type}</div>
          <div className="text-[10px] text-gray-400 truncate font-mono">{node.id}</div>
        </div>
        <button className="text-gray-400 hover:text-gray-600 shrink-0" onClick={onClose} aria-label={t('config.close')}>
          ✕
        </button>
      </div>

      <form onSubmit={handleSubmit} className="flex-1 overflow-y-auto p-3 space-y-3">
        <div>
          <label className="block text-xs font-medium text-gray-600 mb-1">{t('config.nodeName')}</label>
          <input
            type="text"
            value={name}
            onChange={(event) => setName(event.target.value)}
            className={inputClass}
            placeholder={t('config.nodeNamePlaceholder')}
          />
        </div>

        {properties.length > 0 ? (
          <div className="space-y-3">
            {properties.map(([key, schema]) => (schema.enum ? renderEnumSelector(key, schema, config[key]) : renderInputField(key, schema, config[key])))}
          </div>
        ) : (
          <div className="space-y-3">
            <div className="text-xs text-gray-600">{t('config.noSchema')}</div>
            <div>
              <label className="block text-xs font-medium text-gray-600 mb-1">{t('config.customJson')}</label>
              <textarea
                defaultValue={JSON.stringify(config || {}, null, 2)}
                onChange={(event) => {
                  try {
                    setConfig(JSON.parse(event.target.value));
                  } catch {
                    // Keep the last valid config while the JSON is being edited.
                  }
                }}
                className={`${inputClass} font-mono`}
                rows={10}
                placeholder={t('config.customJsonPlaceholder')}
              />
            </div>
          </div>
        )}
      </form>

      <div className="flex items-center justify-between gap-2 p-3 border-t border-gray-200">
        {onDelete ? (
          <button type="button" className="px-3 py-1.5 text-xs text-gr-fuchsia-600 hover:bg-gr-fuchsia-50 rounded" onClick={onDelete}>
            {t('config.delete')}
          </button>
        ) : (
          <span />
        )}
        <div className="flex items-center gap-2">
          <button type="button" className="px-3 py-1.5 text-xs text-gray-600 hover:bg-gray-100 rounded" onClick={onClose}>
            {t('config.cancel')}
          </button>
          <button
            type="button"
            className="px-3 py-1.5 text-xs font-medium bg-gr-blue-500 text-white rounded hover:bg-gr-blue-600"
            onClick={() => handleSubmit()}
          >
            {t('config.done')}
          </button>
        </div>
      </div>
    </div>
  );
}
