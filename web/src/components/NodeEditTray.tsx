import { useCallback, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import type { FlowNode } from '../types/flow';
import { getCategoryColor } from '../utils/nodeCategories';
import { renderMarkdown } from '../utils/markdown';
import { useFlowStore, type NodePatch } from '../store/flowStore';
import { withDefaults } from '../schema/properties';
import { validateConfig } from '../schema/validate';
import { PropertyFields } from './config/PropertyFields';
import { widgetRegistry } from './config/widgets';
import { inputClass } from './config/types';

export type EditTab = 'properties' | 'description' | 'appearance';

export interface NodeEditTrayProps {
  node: FlowNode;
  onClose: () => void;
  onSave: (patch: NodePatch) => void;
  onDelete?: () => void;
}

const tabClass = (active: boolean) =>
  `px-3 py-1.5 text-xs border-b-2 ${active ? 'border-gr-blue-500 text-gr-blue-700 font-medium' : 'border-transparent text-gray-500 hover:text-gray-700'}`;

/**
 * Node edit tray: slides in at the right, like Node-RED's edit dialog but
 * without blocking the canvas. Properties are rendered from the node
 * type's schema (web/src/schema), validated on every change, and Done is
 * only enabled while the form is valid.
 */
export function NodeEditTray({ node, onClose, onSave, onDelete }: NodeEditTrayProps) {
  const { t } = useTranslation();
  const nodeTypes = useFlowStore((state) => state.nodeTypes);
  const flow = useFlowStore((state) => state.flow);

  const metadata = useMemo(() => nodeTypes.find((nt) => nt.type === node.type) || null, [nodeTypes, node.type]);
  const schema = metadata?.configSchema || null;
  const hasSchema = Object.keys(schema?.properties || {}).length > 0;

  const [config, setConfig] = useState<Record<string, unknown>>(() => withDefaults(schema, node.config));
  const [name, setName] = useState(node.name || '');
  const [description, setDescription] = useState(node.description || '');
  const [disabled, setDisabled] = useState(!!node.disabled);
  const [tab, setTab] = useState<EditTab>('properties');
  const [invalid, setInvalidState] = useState<Record<string, string>>({});

  const translate = useCallback((key: string, options?: Record<string, unknown>) => t(key, options), [t]);
  const errors = useMemo(() => (hasSchema ? validateConfig(schema, config, translate) : {}), [hasSchema, schema, config, translate]);
  const problemCount = Object.keys(errors).length + Object.keys(invalid).length;
  const canSave = problemCount === 0;

  const setInvalid = useCallback((key: string, message: string | null) => {
    setInvalidState((prev) => {
      if (message === null) {
        if (!(key in prev)) return prev;
        const { [key]: _dropped, ...rest } = prev;
        return rest;
      }
      return prev[key] === message ? prev : { ...prev, [key]: message };
    });
  }, []);

  const handleChange = useCallback((key: string, value: unknown) => {
    setConfig((prev) => ({ ...prev, [key]: value }));
  }, []);

  const submit = () => {
    if (!canSave) return;
    onSave({ config, name: name.trim(), description: description.trim(), disabled });
  };

  const category = metadata?.category || 'custom';
  const color = getCategoryColor(category);
  const context = useMemo(() => ({ flow, nodeTypes, nodeId: node.id }), [flow, nodeTypes, node.id]);
  const JsonWidget = widgetRegistry.json;
  const helpHtml = useMemo(() => (metadata?.help ? renderMarkdown(metadata.help) : ''), [metadata?.help]);

  const tabs: { id: EditTab; label: string }[] = [
    { id: 'properties', label: t('config.tabs.properties') },
    { id: 'description', label: t('config.tabs.description') },
    { id: 'appearance', label: t('config.tabs.appearance') },
  ];

  return (
    <div
      className="fixed top-11 bottom-6 right-9 w-96 bg-white border-l border-gray-200 shadow-xl z-30 flex flex-col animate-slide-in-right"
      role="dialog"
      aria-label={metadata?.name || node.type}
      data-testid="node-config"
    >
      <div className="flex items-center gap-2 px-3 py-2 border-b border-gray-200">
        <span className={`w-3 h-3 rounded-sm shrink-0 ${color.swatch}`} style={metadata?.color ? { backgroundColor: metadata.color } : undefined} aria-hidden="true" />
        <div className="flex-1 min-w-0">
          <div className="text-sm font-semibold text-gray-800 truncate">{metadata?.name || node.type}</div>
          <div className="text-[10px] text-gray-400 truncate font-mono">{node.id}</div>
        </div>
        <button className="text-gray-400 hover:text-gray-600 shrink-0" onClick={onClose} aria-label={t('config.close')}>
          ✕
        </button>
      </div>

      <div className="px-3 pt-3">
        <label htmlFor="node-name" className="block text-xs font-medium text-gray-600 mb-1">
          {t('config.nodeName')}
        </label>
        <input id="node-name" type="text" value={name} onChange={(event) => setName(event.target.value)} className={inputClass} placeholder={t('config.nodeNamePlaceholder')} />
      </div>

      <div className="flex border-b border-gray-200 px-3 mt-2" role="tablist">
        {tabs.map((entry) => (
          <button
            key={entry.id}
            type="button"
            role="tab"
            aria-selected={tab === entry.id}
            className={tabClass(tab === entry.id)}
            onClick={() => setTab(entry.id)}
            data-testid={`config-tab-${entry.id}`}
          >
            {entry.label}
          </button>
        ))}
      </div>

      <form
        onSubmit={(event) => {
          event.preventDefault();
          submit();
        }}
        className="flex-1 overflow-y-auto p-3"
      >
        {tab === 'properties' &&
          (hasSchema && schema ? (
            <PropertyFields schema={schema} config={config} onChange={handleChange} errors={errors} context={context} setInvalid={setInvalid} />
          ) : (
            <div className="space-y-2">
              <div className="text-xs text-gray-600">{t('config.noSchema')}</div>
              <label className="block text-xs font-medium text-gray-600">{t('config.customJson')}</label>
              <JsonWidget
                id="config"
                property={{ type: 'object', description: '', default: undefined, enum: [], pattern: '', label: t('config.customJson') }}
                value={config}
                onChange={(value) => setConfig((value && typeof value === 'object' ? value : {}) as Record<string, unknown>)}
                setInvalid={(message) => setInvalid('config', message)}
                context={context}
              />
            </div>
          ))}

        {tab === 'description' && (
          <div className="space-y-3">
            <div>
              <label htmlFor="node-description" className="block text-xs font-medium text-gray-600 mb-1">
                {t('config.tabs.description')}
              </label>
              <textarea
                id="node-description"
                className={`${inputClass} font-mono`}
                rows={6}
                value={description}
                onChange={(event) => setDescription(event.target.value)}
                placeholder={t('config.descriptionPlaceholder')}
              />
            </div>
            <div>
              <div className="text-xs font-medium text-gray-600 mb-1">{t('config.about', { type: metadata?.name || node.type })}</div>
              {helpHtml ? (
                <div className="markdown text-xs text-gray-700 bg-gray-50 rounded p-2" data-testid="node-type-help" dangerouslySetInnerHTML={{ __html: helpHtml }} />
              ) : (
                <div className="text-xs text-gray-400">{t('config.noHelp')}</div>
              )}
            </div>
          </div>
        )}

        {tab === 'appearance' && (
          <div className="space-y-4">
            <div>
              <div className="flex items-center gap-2">
                <input id="node-enabled" type="checkbox" className="h-4 w-4 accent-gr-blue-500" checked={!disabled} onChange={(event) => setDisabled(!event.target.checked)} />
                <label htmlFor="node-enabled" className="text-xs font-medium text-gray-600">
                  {t('config.enabled')}
                </label>
              </div>
              <div className="mt-1 text-[10px] text-gray-400 leading-snug">{t('config.enabledHelp')}</div>
            </div>
            <div className="text-xs text-gray-600 space-y-1">
              <div>
                <span className="text-gray-400">{t('sidebar.type')}: </span>
                {metadata?.name || node.type} <span className="font-mono text-gray-400">({node.type})</span>
              </div>
              <div className="flex items-center gap-2">
                <span className="text-gray-400">{t('config.category')}: </span>
                <span className={`inline-block w-3 h-3 rounded-sm ${color.swatch}`} style={metadata?.color ? { backgroundColor: metadata.color } : undefined} />
                {category}
              </div>
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
          {problemCount > 0 && (
            <span className="text-[11px] text-gr-fuchsia-600" data-testid="config-problems">
              {t('config.problems', { count: problemCount })}
            </span>
          )}
          <button type="button" className="px-3 py-1.5 text-xs text-gray-600 hover:bg-gray-100 rounded" onClick={onClose}>
            {t('config.cancel')}
          </button>
          <button
            type="button"
            className="px-3 py-1.5 text-xs font-medium bg-gr-blue-500 text-white rounded hover:bg-gr-blue-600 disabled:opacity-40 disabled:cursor-not-allowed"
            onClick={submit}
            disabled={!canSave}
            data-testid="config-done"
          >
            {t('config.done')}
          </button>
        </div>
      </div>
    </div>
  );
}

export default NodeEditTray;
