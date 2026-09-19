import { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { renderMarkdown } from '../utils/markdown';
import { outputPortsFor } from '../schema/ports';
import { useFlowStore } from '../store/flowStore';
import { useEditorStore } from '../store/editorStore';
import { useRuntimeStore, selectNodeStatus, selectNodeMetrics } from '../store/runtimeStore';
import { hasVisibleStatus } from './NodeShell';
import type { Flow, FlowNode } from '../types/flow';

function Field({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div>
      <label className="block text-sm font-medium text-gray-600 mb-1">{label}</label>
      {children}
    </div>
  );
}

function Value({ children, mono }: { children: React.ReactNode; mono?: boolean }) {
  return <div className={`text-sm text-gray-800 bg-gray-50 p-2 rounded ${mono ? 'font-mono text-xs' : ''}`}>{children}</div>;
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
        <h3 className="font-semibold text-gray-700">{t('sidebar.nodeProperties')}</h3>
        <button
          className="px-3 py-1 bg-gr-blue-500 text-white rounded text-sm hover:bg-gr-blue-600"
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
          <Value>{metadata?.name ? `${metadata.name} (${node.type})` : node.type}</Value>
        </Field>
        <Field label={t('sidebar.id')}>
          <Value mono>{node.id}</Value>
        </Field>
        {node.disabled && (
          <div className="text-xs text-amber-700 bg-amber-50 rounded p-2" data-testid="node-disabled">
            {t('sidebar.disabled')}
          </div>
        )}
        {node.description && (
          <Field label={t('sidebar.description')}>
            <div className="markdown text-xs text-gray-800 bg-gray-50 p-2 rounded" data-testid="node-description" dangerouslySetInnerHTML={{ __html: renderMarkdown(node.description) }} />
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
          <details className="text-xs">
            <summary className="cursor-pointer text-sm font-medium text-gray-600">{t('sidebar.configuration')}</summary>
            <pre className="mt-1 text-xs text-gray-700 bg-gray-50 p-2 rounded overflow-auto">{JSON.stringify(node.config, null, 2)}</pre>
          </details>
        )}
        {metadata?.help && (
          <details className="text-xs" data-testid="node-help">
            <summary className="cursor-pointer text-sm font-medium text-gray-600">{t('sidebar.about')}</summary>
            <div className="markdown mt-1 text-xs text-gray-700 bg-gray-50 p-2 rounded" dangerouslySetInnerHTML={{ __html: renderMarkdown(metadata.help) }} />
          </details>
        )}

        {hasVisibleStatus(status) && (
          <Field label={t('sidebar.status')}>
            <div
              className={`text-sm p-2 rounded ${
                status.fill === 'red'
                  ? 'bg-gr-fuchsia-50 text-gr-fuchsia-700'
                  : status.fill === 'yellow'
                    ? 'bg-amber-50 text-amber-700'
                    : 'bg-gr-blue-50 text-gr-blue-700'
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
              <div className="bg-gray-50 p-2 rounded">
                <div className="text-xs text-gray-500">{t('sidebar.messages')}</div>
                <div className="text-lg font-semibold text-gray-800" data-testid="node-messages">
                  {metrics.messages}
                </div>
              </div>
              <div className="bg-gray-50 p-2 rounded">
                <div className="text-xs text-gray-500">{t('sidebar.errors')}</div>
                <div className={`text-lg font-semibold ${metrics.errors > 0 ? 'text-gr-fuchsia-600' : 'text-gray-800'}`}>{metrics.errors}</div>
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
      ? 'bg-gr-blue-50 text-gr-blue-700'
      : flow.status === 'error'
        ? 'bg-gr-fuchsia-50 text-gr-fuchsia-700'
        : 'bg-gray-50 text-gray-700';

  return (
    <div className="p-4 h-full flex flex-col" data-testid="flow-details">
      <h3 className="font-semibold text-gray-700 mb-4">{t('sidebar.flowProperties')}</h3>

      <div className="space-y-4 flex-1 overflow-y-auto">
        <Field label={t('sidebar.name')}>
          <input
            className="w-full px-2 py-1.5 text-sm border border-gray-300 rounded focus:outline-none focus:ring-2 focus:ring-gr-blue-500"
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
            className="w-full px-2 py-1.5 text-sm border border-gray-300 rounded focus:outline-none focus:ring-2 focus:ring-gr-blue-500"
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
            <div className="bg-gray-50 p-2 rounded">
              <div className="text-xs text-gray-500">{t('sidebar.nodes')}</div>
              <div className="text-lg font-semibold text-gray-800">{Object.keys(flow.nodes || {}).length}</div>
            </div>
            <div className="bg-gray-50 p-2 rounded">
              <div className="text-xs text-gray-500">{t('sidebar.connections')}</div>
              <div className="text-lg font-semibold text-gray-800">{(flow.connections || []).length}</div>
            </div>
          </div>
        </Field>

        <Field label={t('sidebar.id')}>
          <Value mono>{flow.id}</Value>
        </Field>

        <div className="mt-auto pt-4 border-t border-gray-200 space-y-0.5">
          <div className="text-xs text-gray-500">
            {t('sidebar.created')}: {formatDate(flow.createdAt)}
          </div>
          <div className="text-xs text-gray-500">
            {t('sidebar.updated')}: {formatDate(flow.updatedAt)}
          </div>
          <div className="text-xs text-gray-500">
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
        <div className="text-sm text-gray-500">{t('sidebar.selectFlow')}</div>
      </div>
    );
  }

  const selectedNode = selectedNodeIds.length === 1 ? flow.nodes[selectedNodeIds[0]] : undefined;
  if (selectedNode) {
    return <NodeDetails flow={flow} node={selectedNode} />;
  }
  return <FlowDetails flow={flow} />;
}
