import { useState, useRef, useEffect } from 'react';
import { useMessageLog } from '../hooks/useMessageLog';

interface MessageLogPanelProps {
  selectedFlowId?: string;
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

export function MessageLogPanel({ selectedFlowId }: MessageLogPanelProps) {
  const {
    messages,
    loading,
    error,
    loadMessages,
    clearMessages,
    setFilterText,
  } = useMessageLog(selectedFlowId);

  const [filterText, setFilterTextState] = useState('');
  const logContainerRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    setFilterText(filterText);
  }, [filterText, setFilterText]);

  useEffect(() => {
    if (logContainerRef.current) {
      logContainerRef.current.scrollTop = logContainerRef.current.scrollHeight;
    }
  }, [messages]);

  useEffect(() => {
    if (selectedFlowId) {
      loadMessages(selectedFlowId, 100);
    }
  }, [selectedFlowId]);

  const handleRefresh = () => {
    if (selectedFlowId) {
      loadMessages(selectedFlowId, 100);
    }
  };

  const formatPayload = (payload: any): string => {
    if (payload === null || payload === undefined) {
      return 'null';
    }
    if (typeof payload === 'object') {
      return JSON.stringify(payload, null, 2);
    }
    return String(payload);
  };

  const formatTimestamp = (timestamp: string): string => {
    const date = new Date(timestamp);
    return date.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' });
  };

  return (
    <div className="h-full flex flex-col">
      <div className="flex items-center justify-between p-2 border-b border-gray-200">
        <h3 className="font-semibold text-sm text-gray-800">Debug</h3>
        <div className="flex items-center gap-1">
          <button
            className="flex items-center gap-1 px-2 py-1 text-xs bg-gr-blue-50 text-gr-blue-700 rounded hover:bg-gr-blue-100"
            onClick={handleRefresh}
            title="Refresh messages"
          >
            <RefreshIcon /> Refresh
          </button>
          <button
            className="flex items-center gap-1 px-2 py-1 text-xs text-gr-fuchsia-600 rounded hover:bg-gr-fuchsia-50"
            onClick={clearMessages}
            title="Clear log"
          >
            <TrashIcon /> Clear
          </button>
        </div>
      </div>

      <div className="p-2 border-b border-gray-200">
        <input
          type="text"
          className="w-full px-2 py-1.5 text-xs border border-gray-300 rounded focus:outline-none focus:ring-2 focus:ring-gr-blue-500"
          placeholder="Filter messages..."
          value={filterText}
          onChange={(e) => setFilterTextState(e.target.value)}
        />
      </div>

      <div
        className="flex-1 p-2 overflow-y-auto bg-gray-50"
        ref={logContainerRef}
      >
        {loading ? (
          <div className="text-center py-4 text-xs text-gray-500">
            Loading messages...
          </div>
        ) : error ? (
          <div className="text-center py-4 text-xs text-red-500">
            Error: {error.message}
          </div>
        ) : messages.length === 0 ? (
          <div className="text-center py-4 text-xs text-gray-500">
            No messages yet. Deploy a flow and send messages to see them here.
          </div>
        ) : (
          <div className="space-y-2">
            {messages.map((msg) => (
              <div
                key={msg.id}
                className="p-2 bg-white rounded border border-gray-200 shadow-sm text-xs"
              >
                <div className="flex items-center gap-1.5 mb-1 flex-wrap">
                  <span className="text-gray-500">{formatTimestamp(msg.timestamp)}</span>
                  <span className="px-1.5 py-0.5 bg-gr-blue-50 text-gr-blue-700 rounded">
                    {msg.flowId}
                  </span>
                  {msg.nodeId && (
                    <span className="px-1.5 py-0.5 bg-gr-skyblue-50 text-gr-skyblue-700 rounded">
                      {msg.nodeId}
                    </span>
                  )}
                </div>

                <div className="font-medium text-gray-800 mb-1">
                  Message ID: {msg.id}
                </div>

                <div className="text-gray-700">
                  <div className="font-medium mb-1">Payload:</div>
                  <pre className="bg-gray-50 p-1.5 rounded overflow-auto">
                    {formatPayload(msg.message.payload)}
                  </pre>
                </div>

                {msg.message.metadata && Object.keys(msg.message.metadata).length > 0 && (
                  <div className="text-gray-600 mt-2">
                    <div className="font-medium mb-1">Metadata:</div>
                    <pre className="bg-gray-50 p-1.5 rounded overflow-auto">
                      {JSON.stringify(msg.message.metadata, null, 2)}
                    </pre>
                  </div>
                )}
              </div>
            ))}
          </div>
        )}
      </div>

      <div className="p-2 border-t border-gray-200 text-[10px] text-gray-500">
        Showing {messages.length} message{messages.length !== 1 ? 's' : ''}
        {selectedFlowId && <span> for flow: {selectedFlowId}</span>}
      </div>
    </div>
  );
}

export default MessageLogPanel;
