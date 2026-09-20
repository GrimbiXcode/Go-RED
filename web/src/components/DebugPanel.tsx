import { useState, useRef, useEffect, useMemo, useCallback } from 'react';
import { useTranslation } from 'react-i18next';
import { Pause, Play, Trash2 } from 'lucide-react';
import { useRuntimeStore, selectDebug } from '../store/runtimeStore';
import { useFlowStore } from '../store/flowStore';
import { JsonTree } from './JsonTree';
import type { DebugMessage } from '../types/generated';

interface DebugPanelProps {
  flowId?: string;
}

export type PayloadKind = 'string' | 'number' | 'boolean' | 'null' | 'json';

/** Renders a payload the way the debug sidebar shows it. */
export function formatPayload(payload: unknown): { text: string; kind: PayloadKind } {
  if (payload === null || payload === undefined) return { text: 'null', kind: 'null' };
  if (typeof payload === 'string') return { text: payload, kind: 'string' };
  if (typeof payload === 'number') return { text: String(payload), kind: 'number' };
  if (typeof payload === 'boolean') return { text: String(payload), kind: 'boolean' };
  return { text: JSON.stringify(payload, null, 2), kind: 'json' };
}

/** "string", "number", "Object{3}", "Array(2)": the type label next to the node name. */
export function payloadTypeLabel(payload: unknown): string {
  if (payload === null || payload === undefined) return 'null';
  if (Array.isArray(payload)) return `Array(${payload.length})`;
  if (typeof payload === 'object') return `Object{${Object.keys(payload as object).length}}`;
  return typeof payload;
}

function formatTimestamp(timestamp: string): string {
  const date = new Date(timestamp);
  if (Number.isNaN(date.getTime())) return timestamp;
  return date.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' });
}

export function filterDebug(messages: DebugMessage[], query: string): DebugMessage[] {
  const needle = query.trim().toLowerCase();
  if (!needle) return messages;
  return messages.filter((msg) => {
    if (msg.nodeName?.toLowerCase().includes(needle)) return true;
    if (msg.nodeId.toLowerCase().includes(needle)) return true;
    if (msg.topic?.toLowerCase().includes(needle)) return true;
    return formatPayload(msg.payload).text.toLowerCase().includes(needle);
  });
}

const levelClass: Record<string, string> = {
  error: 'border-l-danger',
  warn: 'border-l-warn',
  debug: 'border-l-accent',
};

export interface DebugEntryProps {
  message: DebugMessage;
  label: string;
}

/** One feed entry: time · node · type on the first line, the payload below (JSON as a tree). */
export function DebugEntry({ message, label }: DebugEntryProps) {
  const { t } = useTranslation();
  const isError = message.level === 'error';
  const payload = formatPayload(message.payload);

  return (
    <div
      className={`px-2 py-1.5 bg-panel rounded-md border border-line border-l-4 ${levelClass[message.level] || levelClass.debug}`}
      data-testid="debug-message"
      data-level={message.level}
    >
      <div className="flex items-center gap-1.5 flex-wrap text-2xs text-muted">
        <span className="font-mono">{formatTimestamp(message.timestamp)}</span>
        <span className="font-medium text-fg">{label}</span>
        {message.topic && <span className="px-1 rounded bg-sunken text-muted">{message.topic}</span>}
        {isError ? (
          <span className="px-1 rounded bg-danger-soft text-danger-text uppercase tracking-wide">{t('debug.errorLabel')}</span>
        ) : (
          <span className="font-mono text-faint">
            {t('debug.payloadPath')} : {payloadTypeLabel(message.payload)}
          </span>
        )}
      </div>
      <div className="mt-1">
        {payload.kind === 'json' ? (
          <JsonTree value={message.payload} />
        ) : (
          <pre className={`whitespace-pre-wrap break-words font-mono text-2xs leading-4 ${isError ? 'text-danger-text' : 'text-fg'}`}>{payload.text}</pre>
        )}
      </div>
    </div>
  );
}

/**
 * Debug sidebar: the live feed of Debug node output and node errors for the
 * open flow. Entries arrive over the WebSocket (see bindServerEvents); the
 * panel filters by node and text, can pause the display, and clears.
 */
export function DebugPanel({ flowId }: DebugPanelProps) {
  const { t } = useTranslation();
  const messages = useRuntimeStore(selectDebug(flowId));
  const subscribed = useRuntimeStore((state) => (flowId ? !!state.subscribed[flowId] : false));
  const clearDebug = useRuntimeStore((state) => state.clearDebug);
  const nodes = useFlowStore((state) => state.flow?.nodes);

  const [filterText, setFilterText] = useState('');
  const [nodeFilter, setNodeFilter] = useState('');
  const [paused, setPaused] = useState(false);
  const frozen = useRef<DebugMessage[]>([]);
  const listRef = useRef<HTMLDivElement>(null);
  const stickToBottom = useRef(true);

  const labelOf = useCallback(
    (msg: DebugMessage) => {
      const node = nodes?.[msg.nodeId];
      return msg.nodeName || node?.name || msg.nodeType || node?.type || msg.nodeId;
    },
    [nodes]
  );

  const source = paused ? frozen.current : messages;
  const newWhilePaused = paused ? Math.max(0, messages.length - frozen.current.length) : 0;
  const nodeOptions = useMemo(() => {
    const seen = new Map<string, string>();
    for (const msg of messages) if (!seen.has(msg.nodeId)) seen.set(msg.nodeId, labelOf(msg));
    return Array.from(seen.entries()).sort((a, b) => a[1].localeCompare(b[1]));
  }, [messages, labelOf]);
  const visible = useMemo(
    () => filterDebug(nodeFilter ? source.filter((msg) => msg.nodeId === nodeFilter) : source, filterText),
    [source, nodeFilter, filterText]
  );

  const togglePause = () => {
    if (!paused) frozen.current = messages;
    setPaused((p) => !p);
  };

  const onScroll = useCallback(() => {
    const el = listRef.current;
    if (!el) return;
    stickToBottom.current = el.scrollHeight - el.scrollTop - el.clientHeight < 8;
  }, []);

  useEffect(() => {
    const el = listRef.current;
    if (el && stickToBottom.current) el.scrollTop = el.scrollHeight;
  }, [visible.length]);

  const toolButton = 'w-7 h-7 flex items-center justify-center rounded text-muted hover:bg-surface hover:text-fg disabled:opacity-40 disabled:hover:bg-transparent';

  return (
    <div className="h-full flex flex-col" data-testid="debug-panel">
      <div className="flex items-center gap-1 px-2 py-1.5 border-b border-line">
        <h3 className="font-semibold text-sm text-fg flex items-center gap-2 flex-1">
          {t('debug.title')}
          {flowId && (
            <span
              className={`w-2 h-2 rounded-full ${subscribed ? 'bg-ok' : 'bg-faint'}`}
              title={subscribed ? t('debug.live') : t('debug.offline')}
              data-testid="debug-live"
              data-live={subscribed}
            />
          )}
        </h3>
        <button
          type="button"
          className={`${toolButton} ${paused ? 'text-warn-text' : ''}`}
          onClick={togglePause}
          disabled={!flowId}
          aria-label={paused ? t('debug.resume') : t('debug.pause')}
          aria-pressed={paused}
          title={paused ? t('debug.resume') : t('debug.pause')}
          data-testid="debug-pause"
        >
          {paused ? <Play className="w-3.5 h-3.5" aria-hidden="true" /> : <Pause className="w-3.5 h-3.5" aria-hidden="true" />}
        </button>
        <button
          type="button"
          className={toolButton}
          onClick={() => {
            if (flowId) clearDebug(flowId);
            frozen.current = [];
          }}
          disabled={!flowId || messages.length === 0}
          aria-label={t('debug.clear')}
          title={t('debug.clear')}
        >
          <Trash2 className="w-3.5 h-3.5" aria-hidden="true" />
        </button>
      </div>

      <div className="flex gap-1 p-2 border-b border-line">
        <select
          className="max-w-[40%] px-1.5 py-1 text-xs border border-line rounded bg-panel text-fg focus:outline-none focus:ring-2 focus:ring-accent"
          value={nodeFilter}
          onChange={(event) => setNodeFilter(event.target.value)}
          aria-label={t('debug.nodeFilter')}
        >
          <option value="">{t('debug.allNodes')}</option>
          {nodeOptions.map(([id, label]) => (
            <option key={id} value={id}>
              {label}
            </option>
          ))}
        </select>
        <input
          type="search"
          className="flex-1 min-w-0 px-2 py-1 text-xs border border-line rounded bg-panel text-fg placeholder:text-faint focus:outline-none focus:ring-2 focus:ring-accent"
          placeholder={t('debug.filter')}
          aria-label={t('debug.filter')}
          value={filterText}
          onChange={(event) => setFilterText(event.target.value)}
        />
      </div>

      {paused && (
        <div className="px-2 py-1 text-2xs bg-warn-soft text-warn-text border-b border-line" data-testid="debug-paused">
          {t('debug.paused', { count: newWhilePaused })}
        </div>
      )}

      <div className="flex-1 p-2 overflow-y-auto bg-surface" ref={listRef} onScroll={onScroll}>
        {!flowId ? (
          <div className="text-center py-4 text-xs text-muted">{t('sidebar.selectFlow')}</div>
        ) : visible.length === 0 ? (
          <div className="text-center py-4 text-xs text-muted">{source.length === 0 ? t('debug.empty') : t('debug.noMatch')}</div>
        ) : (
          <div className="space-y-1.5">
            {visible.map((msg) => (
              <DebugEntry key={msg.id} message={msg} label={labelOf(msg)} />
            ))}
          </div>
        )}
      </div>

      <div className="px-2 py-1 border-t border-line text-2xs text-muted bg-sunken" data-testid="debug-summary">
        {t('debug.showing', { count: visible.length })}
        {(filterText || nodeFilter) && source.length !== visible.length && <span> {t('debug.ofTotal', { total: source.length })}</span>}
      </div>
    </div>
  );
}

export default DebugPanel;
