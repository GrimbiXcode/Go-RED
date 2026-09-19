import { useState, useRef, useEffect, useMemo } from 'react';
import { useTranslation } from 'react-i18next';
import { useRuntimeStore } from '../store/runtimeStore';
import type { MessageLogEntry } from '../types/message';

interface MessageLogPanelProps {
  flowId?: string;
}

function RefreshIcon() {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={2} strokeLinecap="round" strokeLinejoin="round" className="w-3.5 h-3.5" aria-hidden="true">
      <polyline points="1 4 1 10 7 10" />
      <path d="M3.51 15a9 9 0 1 0 2.13-9.36L1 10" />
    </svg>
  );
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

function formatPayload(payload: unknown): string {
  if (payload === null || payload === undefined) return 'null';
  if (typeof payload === 'object') return JSON.stringify(payload, null, 2);
  return String(payload);
}

function formatTimestamp(timestamp: string): string {
  return new Date(timestamp).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' });
}

export function filterMessages(messages: MessageLogEntry[], query: string): MessageLogEntry[] {
  const needle = query.trim().toLowerCase();
  if (!needle) return messages;
  return messages.filter(
    (msg) =>
      msg.message.id.toLowerCase().includes(needle) ||
      JSON.stringify(msg.message.payload).toLowerCase().includes(needle) ||
      msg.nodeId.toLowerCase().includes(needle)
  );
}

export function MessageLogPanel({ flowId }: MessageLogPanelProps) {
  const { t } = useTranslation();
  const messages = useRuntimeStore((state) => state.messages);
  const loading = useRuntimeStore((state) => state.messagesLoading);
  const error = useRuntimeStore((state) => state.messagesError);
  const loadMessages = useRuntimeStore((state) => state.loadMessages);
  const clearMessages = useRuntimeStore((state) => state.clearMessages);

  const [filterText, setFilterText] = useState('');
  const logContainerRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (flowId) void loadMessages(flowId, 100);
  }, [flowId, loadMessages]);

  const visible = useMemo(() => filterMessages(messages, filterText), [messages, filterText]);

  useEffect(() => {
    if (logContainerRef.current) {
      logContainerRef.current.scrollTop = 0;
    }
  }, [visible.length]);

  return (
    <div className="h-full flex flex-col" data-testid="debug-panel">
      <div className="flex items-center justify-between p-2 border-b border-gray-200">
        <h3 className="font-semibold text-sm text-gray-800">{t('debug.title')}</h3>
        <div className="flex items-center gap-1">
          <button
            className="flex items-center gap-1 px-2 py-1 text-xs bg-gr-blue-50 text-gr-blue-700 rounded hover:bg-gr-blue-100 disabled:opacity-50"
            onClick={() => flowId && void loadMessages(flowId, 100)}
            disabled={!flowId}
            title={t('debug.refresh')}
          >
            <RefreshIcon /> {t('debug.refresh')}
          </button>
          <button
            className="flex items-center gap-1 px-2 py-1 text-xs text-gr-fuchsia-600 rounded hover:bg-gr-fuchsia-50"
            onClick={clearMessages}
            title={t('debug.clear')}
          >
            <TrashIcon /> {t('debug.clear')}
          </button>
        </div>
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

      <div className="flex-1 p-2 overflow-y-auto bg-gray-50" ref={logContainerRef}>
        {loading ? (
          <div className="text-center py-4 text-xs text-gray-500">{t('debug.loading')}</div>
        ) : error ? (
          <div className="text-center py-4 text-xs text-gr-fuchsia-600">{t('debug.error', { message: error })}</div>
        ) : visible.length === 0 ? (
          <div className="text-center py-4 text-xs text-gray-500">{t('debug.empty')}</div>
        ) : (
          <div className="space-y-2">
            {visible.map((msg) => (
              <div key={msg.id} className="p-2 bg-white rounded border border-gray-200 shadow-sm text-xs" data-testid="debug-message">
                <div className="flex items-center gap-1.5 mb-1 flex-wrap">
                  <span className="text-gray-500">{formatTimestamp(msg.timestamp)}</span>
                  {msg.nodeId && <span className="px-1.5 py-0.5 bg-gr-skyblue-50 text-gr-skyblue-700 rounded">{msg.nodeId}</span>}
                  <span className="text-gray-400 font-mono">{msg.id}</span>
                </div>

                <div className="text-gray-700">
                  <div className="font-medium mb-1">{t('debug.payload')}:</div>
                  <pre className="bg-gray-50 p-1.5 rounded overflow-auto">{formatPayload(msg.message.payload)}</pre>
                </div>

                {msg.message.metadata && Object.keys(msg.message.metadata).length > 0 && (
                  <div className="text-gray-600 mt-2">
                    <div className="font-medium mb-1">{t('debug.metadata')}:</div>
                    <pre className="bg-gray-50 p-1.5 rounded overflow-auto">{JSON.stringify(msg.message.metadata, null, 2)}</pre>
                  </div>
                )}
              </div>
            ))}
          </div>
        )}
      </div>

      <div className="p-2 border-t border-gray-200 text-[10px] text-gray-500" data-testid="debug-summary">
        {t('debug.showing', { count: visible.length })}
        {flowId && <span> {t('debug.forFlow', { id: flowId })}</span>}
      </div>
    </div>
  );
}

export default MessageLogPanel;
