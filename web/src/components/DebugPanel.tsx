import { useState, useRef, useEffect, useMemo, useCallback } from 'react';
import { useTranslation } from 'react-i18next';
import { useRuntimeStore, selectDebug } from '../store/runtimeStore';
import { useFlowStore } from '../store/flowStore';
import type { DebugMessage } from '../types/generated';

interface DebugPanelProps {
  flowId?: string;
}

function TrashIcon() {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={2} strokeLinecap="round" strokeLinejoin="round" className="w-3.5 h-3.5" aria-hidden="true">
      <polyline points="3 6 5 6 21 6" />
      <path d="M19 6l-1 14a2 2 0 0 1-2 2H8a2 2 0 0 1-2-2L5 6" />
      <path d="M10 11v6" />
      <path d="M14 11v6" />
    </svg>
  );
}

/** Renders a payload the way the debug sidebar shows it. */
export function formatPayload(payload: unknown): { text: string; kind: 'string' | 'number' | 'boolean' | 'null' | 'json' } {
  if (payload === null || payload === undefined) return { text: 'null', kind: 'null' };
  if (typeof payload === 'string') return { text: payload, kind: 'string' };
  if (typeof payload === 'number') return { text: String(payload), kind: 'number' };
  if (typeof payload === 'boolean') return { text: String(payload), kind: 'boolean' };
  return { text: JSON.stringify(payload, null, 2), kind: 'json' };
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
  error: 'border-l-4 border-gr-fuchsia-500',
  warn: 'border-l-4 border-amber-400',
  debug: 'border-l-4 border-gr-blue-300',
};

/**
 * Debug sidebar: the live feed of Debug node output and node errors for the
 * open flow. Entries arrive over the WebSocket (see bindServerEvents); the
 * panel only renders and filters them.
 */
export function DebugPanel({ flowId }: DebugPanelProps) {
  const { t } = useTranslation();
  const messages = useRuntimeStore(selectDebug(flowId));
  const subscribed = useRuntimeStore((state) => (flowId ? !!state.subscribed[flowId] : false));
  const clearDebug = useRuntimeStore((state) => state.clearDebug);
  const nodes = useFlowStore((state) => state.flow?.nodes);

  const [filterText, setFilterText] = useState('');
  const listRef = useRef<HTMLDivElement>(null);
  const stickToBottom = useRef(true);

  const visible = useMemo(() => filterDebug(messages, filterText), [messages, filterText]);

  const onScroll = useCallback(() => {
    const el = listRef.current;
    if (!el) return;
    stickToBottom.current = el.scrollHeight - el.scrollTop - el.clientHeight < 8;
  }, []);

  useEffect(() => {
    const el = listRef.current;
    if (el && stickToBottom.current) {
      el.scrollTop = el.scrollHeight;
    }
  }, [visible.length]);

  const nodeLabel = (msg: DebugMessage) => {
    const node = nodes?.[msg.nodeId];
    return msg.nodeName || node?.name || msg.nodeType || node?.type || msg.nodeId;
  };

  return (
    <div className="h-full flex flex-col" data-testid="debug-panel">
      <div className="flex items-center justify-between p-2 border-b border-gray-200">
        <h3 className="font-semibold text-sm text-gray-800 flex items-center gap-2">
          {t('debug.title')}
          {flowId && (
            <span
              className={`w-2 h-2 rounded-full ${subscribed ? 'bg-gr-blue-500' : 'bg-gray-300'}`}
              title={subscribed ? t('debug.live') : t('debug.offline')}
              data-testid="debug-live"
              data-live={subscribed}
            />
          )}
        </h3>
        <button
          className="flex items-center gap-1 px-2 py-1 text-xs text-gr-fuchsia-600 rounded hover:bg-gr-fuchsia-50 disabled:opacity-40"
          onClick={() => flowId && clearDebug(flowId)}
          disabled={!flowId || messages.length === 0}
          title={t('debug.clear')}
        >
          <TrashIcon /> {t('debug.clear')}
        </button>
      </div>

      <div className="p-2 border-b border-gray-200">
        <input
          type="search"
          className="w-full px-2 py-1.5 text-xs border border-gray-300 rounded focus:outline-none focus:ring-2 focus:ring-gr-blue-500"
          placeholder={t('debug.filter')}
          aria-label={t('debug.filter')}
          value={filterText}
          onChange={(event) => setFilterText(event.target.value)}
        />
      </div>

      <div className="flex-1 p-2 overflow-y-auto bg-gray-50" ref={listRef} onScroll={onScroll}>
        {!flowId ? (
          <div className="text-center py-4 text-xs text-gray-500">{t('sidebar.selectFlow')}</div>
        ) : visible.length === 0 ? (
          <div className="text-center py-4 text-xs text-gray-500">{messages.length === 0 ? t('debug.empty') : t('debug.noMatch')}</div>
        ) : (
          <div className="space-y-1.5">
            {visible.map((msg) => {
              const payload = formatPayload(msg.payload);
              return (
                <div
                  key={msg.id}
                  className={`px-2 py-1.5 bg-white rounded border border-gray-200 text-xs ${levelClass[msg.level] || levelClass.debug}`}
                  data-testid="debug-message"
                  data-level={msg.level}
                >
                  <div className="flex items-center gap-1.5 flex-wrap text-[10px] text-gray-500">
                    <span>{formatTimestamp(msg.timestamp)}</span>
                    <span className="font-medium text-gray-700">{nodeLabel(msg)}</span>
                    {msg.topic && <span className="px-1 rounded bg-gray-100 text-gray-600">{msg.topic}</span>}
                    {msg.level === 'error' && (
                      <span className="px-1 rounded bg-gr-fuchsia-50 text-gr-fuchsia-700 uppercase tracking-wide">{t('debug.errorLabel')}</span>
                    )}
                    <span className="ml-auto text-gray-400">{payload.kind === 'json' ? 'object' : payload.kind}</span>
                  </div>
                  <pre
                    className={`mt-1 whitespace-pre-wrap break-words font-mono text-[11px] ${
                      msg.level === 'error' ? 'text-gr-fuchsia-700' : 'text-gray-800'
                    }`}
                  >
                    {payload.text}
                  </pre>
                </div>
              );
            })}
          </div>
        )}
      </div>

      <div className="p-2 border-t border-gray-200 text-[10px] text-gray-500" data-testid="debug-summary">
        {t('debug.showing', { count: visible.length })}
        {filterText && messages.length !== visible.length && <span> {t('debug.ofTotal', { total: messages.length })}</span>}
      </div>
    </div>
  );
}

export default DebugPanel;
