import { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { renderMarkdown } from '../utils/markdown';
import { outputPortsFor } from '../schema/ports';
import { humanize, resolveProperties } from '../schema/properties';
import { NodeIcon } from './CategoryIcon';
import type { NodeMetadata } from '../types/node';
import { getCategoryColor } from '../utils/nodeCategories';
import { useFlowStore } from '../store/flowStore';
import { useEditorStore } from '../store/editorStore';
import { useRuntimeStore, selectNodeStatus, selectNodeMetrics } from '../store/runtimeStore';
import { hasVisibleStatus } from './NodeShell';
import type { Flow, FlowNode } from '../types/flow';

function Field({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div>
      <label className="block text-sm font-medium text-muted mb-1">{label}</label>
      {children}
    </div>
  );
}

function Value({ children, mono }: { children: React.ReactNode; mono?: boolean }) {
  return <div className={`text-sm text-fg bg-surface p-2 rounded ${mono ? 'font-mono text-xs' : ''}`}>{children}</div>;
}

/** One row per configured property, in schema order, with a compact value. */
export function settingsRows(config: Record<string, unknown>, metadata?: NodeMetadata): [string, string][] {
  const resolved = metadata ? resolveProperties(metadata.configSchema) : [];
  const ordered = [...resolved.map((p) => p.key), ...Object.keys(config).filter((key) => !resolved.some((p) => p.key === key))];
  const rows: [string, string][] = [];
  for (const key of ordered) {
    if (!(key in config)) continue;
    const prop = resolved.find((p) => p.key === key);
    rows.push([prop?.label || humanize(key), formatSettingValue(config[key], prop?.widget)]);
  }
  return rows;
}

function formatSettingValue(value: unknown, widget?: string): string {
  if (value === undefined || value === null || value === '') return '–';
  if (typeof value === 'boolean') return value ? 'true' : 'false';
  if (typeof value === 'number') return String(value);
  if (typeof value === 'string') return value.length > 80 ? `${value.slice(0, 80)}…` : value;
  if (widget === 'typedInput' && typeof value === 'object') {
    const typed = value as { type?: string; path?: string; value?: string };
    return typed.path !== undefined ? `${typed.type}.${typed.path}` : `${typed.type}: ${typed.value ?? ''}`;
  }
  if (Array.isArray(value)) return value.length === 0 ? '[]' : `${value.length} × ${JSON.stringify(value[0]).slice(0, 40)}${value.length > 1 ? ', …' : ''}`;
  const json = JSON.stringify(value);
  return json.length > 80 ? `${json.slice(0, 80)}…` : json;
}

function formatDate(value?: string, never = '') {
  if (!value) return never;
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? value : date.toLocaleString();
}

function NodeDetails({ flow, node }: { flow: Flow; node: FlowNode }) {
  const { t } = useTranslation();
  const openConfig = useEditorStore((state) => state.openConfig);
  const nodeTypes = useFlowStore((state) => state.nodeTypes);
  const status = useRuntimeStore(selectNodeStatus(flow.id, node.id));
  const metrics = useRuntimeStore(selectNodeMetrics(flow.id, node.id));
  const metadata = nodeTypes.find((nt) => nt.type === node.type);

  return (
    <div className="p-4 h-full flex flex-col" data-testid="node-details">
      <div className="flex items-center justify-between mb-4">
        <h3 className="font-semibold text-fg">{t('sidebar.nodeProperties')}</h3>
        <button
          className="px-3 py-1 bg-accent text-accent-fg rounded text-sm hover:bg-accent-strong"
          onClick={() => openConfig(node.id)}
        >
          {t('sidebar.configure')}
        </button>
      </div>

      <div className="space-y-4 flex-1 overflow-y-auto">
        <Field label={t('sidebar.name')}>
          <Value>{node.name || metadata?.name || node.type}</Value>
        </Field>
        <Field label={t('sidebar.type')}>
          <div className="flex items-center gap-2">
            <span
              className="inline-flex items-center gap-1.5 px-2 py-0.5 rounded-full text-2xs font-medium text-white"
              style={{ background: metadata?.color || getCategoryColor(metadata?.category || 'custom').fill }}
              data-testid="node-type-chip"
            >
              <NodeIcon icon={metadata?.icon} category={metadata?.category || 'custom'} className="w-3 h-3" />
              {metadata?.name || node.type}
            </span>
            <span className="font-mono text-2xs text-faint">{node.type}</span>
          </div>
        </Field>
        <Field label={t('sidebar.id')}>
          <Value mono>{node.id}</Value>
        </Field>
        {node.disabled && (
          <div className="text-xs text-warn-text bg-warn-soft rounded p-2" data-testid="node-disabled">
            {t('sidebar.disabled')}
          </div>
        )}
        {node.description && (
          <Field label={t('sidebar.description')}>
            <div className="markdown text-xs text-fg bg-surface p-2 rounded" data-testid="node-description" dangerouslySetInnerHTML={{ __html: renderMarkdown(node.description) }} />
          </Field>
        )}
        {metadata?.outputsFrom && (
          <Field label={t('sidebar.outputs')}>
            <Value>
              {outputPortsFor(metadata, node.config)
                .map((port) => port.name)
                .join(', ') || '–'}
            </Value>
          </Field>
        )}
        <Field label={t('sidebar.position')}>
          <Value>
            X: {(node.position?.x ?? 0).toFixed(0)}, Y: {(node.position?.y ?? 0).toFixed(0)}
          </Value>
        </Field>

        {node.config && Object.keys(node.config).length > 0 && (
          <Field label={t('sidebar.configuration')}>
            <table className="w-full text-xs" data-testid="node-settings">
              <tbody>
                {settingsRows(node.config, metadata).map(([label, value]) => (
                  <tr key={label} className="border-b border-line last:border-b-0 align-top">
                    <td className="py-1 pr-2 text-muted whitespace-nowrap">{label}</td>
                    <td className="py-1 font-mono text-2xs text-fg break-all">{value}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </Field>
        )}
        {metadata?.help && (
          <details className="text-xs" data-testid="node-help">
            <summary className="cursor-pointer text-sm font-medium text-muted">{t('sidebar.about')}</summary>
            <div className="markdown mt-1 text-xs text-fg bg-surface p-2 rounded" dangerouslySetInnerHTML={{ __html: renderMarkdown(metadata.help) }} />
          </details>
        )}

        {hasVisibleStatus(status) && (
          <Field label={t('sidebar.status')}>
            <div
              className={`text-sm p-2 rounded ${
                status.fill === 'red'
                  ? 'bg-danger-soft text-danger-text'
                  : status.fill === 'yellow'
                    ? 'bg-warn-soft text-warn-text'
                    : 'bg-accent-soft text-accent-text'
              }`}
              data-testid="node-status-detail"
            >
              {status.text || status.fill}
            </div>
          </Field>
        )}

        {metrics && (
          <Field label={t('sidebar.activity')}>
            <div className="grid grid-cols-2 gap-2">
              <div className="bg-surface p-2 rounded">
                <div className="text-xs text-muted">{t('sidebar.messages')}</div>
                <div className="text-lg font-semibold text-fg" data-testid="node-messages">
                  {metrics.messages}
                </div>
              </div>
              <div className="bg-surface p-2 rounded">
                <div className="text-xs text-muted">{t('sidebar.errors')}</div>
                <div className={`text-lg font-semibold ${metrics.errors > 0 ? 'text-danger-text' : 'text-fg'}`}>{metrics.errors}</div>
              </div>
            </div>
          </Field>
        )}
      </div>
    </div>
  );
}

function FlowDetails({ flow }: { flow: Flow }) {
  const { t } = useTranslation();
  const renameFlow = useFlowStore((state) => state.renameFlow);
  const [name, setName] = useState(flow.name);
  const [description, setDescription] = useState(flow.description);

  useEffect(() => {
    setName(flow.name);
    setDescription(flow.description);
  }, [flow.id, flow.name, flow.description]);

  const commit = () => {
    if (name.trim() !== flow.name || description !== flow.description) {
      renameFlow(name, description);
    }
  };

  const statusClass =
    flow.status === 'running'
      ? 'bg-accent-soft text-accent-text'
      : flow.status === 'error'
        ? 'bg-danger-soft text-danger-text'
        : 'bg-surface text-fg';

  return (
    <div className="p-4 h-full flex flex-col" data-testid="flow-details">
      <h3 className="font-semibold text-fg mb-4">{t('sidebar.flowProperties')}</h3>

      <div className="space-y-4 flex-1 overflow-y-auto">
        <Field label={t('sidebar.name')}>
          <input
            className="w-full px-2 py-1.5 text-sm border border-line rounded focus:outline-none focus:ring-2 focus:ring-accent"
            value={name}
            placeholder={t('sidebar.namePlaceholder')}
            onChange={(event) => setName(event.target.value)}
            onBlur={commit}
            onKeyDown={(event) => {
              if (event.key === 'Enter') (event.target as HTMLInputElement).blur();
            }}
            aria-label={t('sidebar.name')}
          />
        </Field>

        <Field label={t('sidebar.description')}>
          <textarea
            className="w-full px-2 py-1.5 text-sm border border-line rounded focus:outline-none focus:ring-2 focus:ring-accent"
            rows={2}
            value={description}
            placeholder={t('sidebar.descriptionPlaceholder')}
            onChange={(event) => setDescription(event.target.value)}
            onBlur={commit}
            aria-label={t('sidebar.description')}
          />
        </Field>

        <Field label={t('sidebar.status')}>
          <div className={`text-sm p-2 rounded text-center font-medium ${statusClass}`} data-testid="flow-status">
            {t(`flowStatus.${flow.status}`)}
          </div>
        </Field>

        <Field label={t('sidebar.statistics')}>
          <div className="grid grid-cols-2 gap-2">
            <div className="bg-surface p-2 rounded">
              <div className="text-xs text-muted">{t('sidebar.nodes')}</div>
              <div className="text-lg font-semibold text-fg">{Object.keys(flow.nodes || {}).length}</div>
            </div>
            <div className="bg-surface p-2 rounded">
              <div className="text-xs text-muted">{t('sidebar.connections')}</div>
              <div className="text-lg font-semibold text-fg">{(flow.connections || []).length}</div>
            </div>
          </div>
        </Field>

        <Field label={t('sidebar.id')}>
          <Value mono>{flow.id}</Value>
        </Field>

        <div className="mt-auto pt-4 border-t border-line space-y-0.5">
          <div className="text-xs text-muted">
            {t('sidebar.created')}: {formatDate(flow.createdAt)}
          </div>
          <div className="text-xs text-muted">
            {t('sidebar.updated')}: {formatDate(flow.updatedAt)}
          </div>
          <div className="text-xs text-muted">
            {t('sidebar.deployed')}: {formatDate(flow.deployedAt, t('sidebar.never'))}
          </div>
        </div>
      </div>
    </div>
  );
}

/** Info tab: the selected node, or the flow itself when nothing is selected. */
export function Sidebar() {
  const { t } = useTranslation();
  const flow = useFlowStore((state) => state.flow);
  const selectedNodeIds = useEditorStore((state) => state.selectedNodeIds);

  if (!flow) {
    return (
      <div className="p-4">
        <div className="text-sm text-muted">{t('sidebar.selectFlow')}</div>
      </div>
    );
  }

  const selectedNode = selectedNodeIds.length === 1 ? flow.nodes[selectedNodeIds[0]] : undefined;
  if (selectedNode) {
    return <NodeDetails flow={flow} node={selectedNode} />;
  }
  return <FlowDetails flow={flow} />;
}
