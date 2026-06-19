import { useState, useRef, useEffect } from 'react';
import { useMessageLog } from '../hooks/useMessageLog';

interface MessageLogPanelProps {
  selectedFlowId?: string;
  isOpen: boolean;
  onClose: () => void;
}

export function MessageLogPanel({ selectedFlowId, isOpen, onClose }: MessageLogPanelProps) {
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

  // Sync filter text with hook
  useEffect(() => {
    setFilterText(filterText);
  }, [filterText, setFilterText]);

  // Auto-scroll to bottom when new messages arrive
  useEffect(() => {
    if (logContainerRef.current) {
      logContainerRef.current.scrollTop = logContainerRef.current.scrollHeight;
    }
  }, [messages]);

  // Load messages when panel opens
  useEffect(() => {
    if (isOpen && selectedFlowId) {
      loadMessages(selectedFlowId, 100);
    }
  }, [isOpen, selectedFlowId]);

  // Refresh messages
  const handleRefresh = () => {
    if (selectedFlowId) {
      loadMessages(selectedFlowId, 100);
    }
  };

  // Format payload for display
  const formatPayload = (payload: any): string => {
    if (payload === null || payload === undefined) {
      return 'null';
    }
    if (typeof payload === 'object') {
      return JSON.stringify(payload, null, 2);
    }
    return String(payload);
  };

  // Format timestamp
  const formatTimestamp = (timestamp: string): string => {
    const date = new Date(timestamp);
    return date.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' });
  };

  if (!isOpen) {
    return null;
  }

  return (
    <div className="fixed bottom-0 right-0 top-0 w-96 bg-white border-l border-gray-200 shadow-xl z-40 flex flex-col">
      {/* Header */}
      <div className="flex items-center justify-between p-3 border-b border-gray-200 bg-gray-50">
        <h3 className="font-semibold text-gray-800">Message Log</h3>
        <div className="flex items-center gap-2">
          <button
            className="px-2 py-1 text-xs bg-blue-100 text-blue-600 rounded hover:bg-blue-200"
            onClick={handleRefresh}
            title="Refresh messages"
          >
            🔄 Refresh
          </button>
          <button
            className="px-2 py-1 text-xs bg-gray-100 text-gray-600 rounded hover:bg-gray-200"
            onClick={clearMessages}
            title="Clear log"
          >
            🗑️ Clear
          </button>
          <button
            className="px-2 py-1 text-xs bg-gray-100 text-gray-600 rounded hover:bg-gray-200"
            onClick={onClose}
            title="Close"
          >
            ✕
          </button>
        </div>
      </div>

      {/* Filter */}
      <div className="p-2 border-b border-gray-200">
        <input
          type="text"
          className="w-full p-2 text-sm border border-gray-300 rounded focus:outline-none focus:ring-2 focus:ring-blue-500"
          placeholder="Filter messages..."
          value={filterText}
          onChange={(e) => setFilterTextState(e.target.value)}
        />
      </div>

      {/* Log Content */}
      <div
        className="flex-1 p-3 overflow-y-auto bg-gray-50"
        ref={logContainerRef}
      >
        {loading ? (
          <div className="text-center py-4 text-gray-500">
            Loading messages...
          </div>
        ) : error ? (
          <div className="text-center py-4 text-red-500">
            Error: {error.message}
          </div>
        ) : messages.length === 0 ? (
          <div className="text-center py-4 text-gray-500">
            No messages yet. Deploy a flow and send messages to see them here.
          </div>
        ) : (
          <div className="space-y-2">
            {messages.map((msg) => (
              <div
                key={msg.id}
                className="p-3 bg-white rounded border border-gray-200 shadow-sm"
              >
                <div className="flex items-start justify-between mb-1">
                  <div className="flex items-center gap-2">
                    <span className="text-xs text-gray-500">{formatTimestamp(msg.timestamp)}</span>
                    <span className="text-xs px-2 py-1 bg-blue-100 text-blue-600 rounded">
                      {msg.flowId}
                    </span>
                    {msg.nodeId && (
                      <span className="text-xs px-2 py-1 bg-green-100 text-green-600 rounded">
                        {msg.nodeId}
                      </span>
                    )}
                  </div>
                </div>
                
                <div className="text-sm font-medium text-gray-800 mb-1">
                  Message ID: {msg.id}
                </div>
                
                <div className="text-sm text-gray-700">
                  <div className="font-medium mb-1">Payload:</div>
                  <pre className="text-xs bg-gray-50 p-2 rounded overflow-auto">
                    {formatPayload(msg.message.payload)}
                  </pre>
                </div>
                
                {msg.message.metadata && Object.keys(msg.message.metadata).length > 0 && (
                  <div className="text-sm text-gray-600 mt-2">
                    <div className="font-medium mb-1">Metadata:</div>
                    <pre className="text-xs bg-gray-50 p-2 rounded overflow-auto">
                      {JSON.stringify(msg.message.metadata, null, 2)}
                    </pre>
                  </div>
                )}
              </div>
            ))}
          </div>
        )}
      </div>

      {/* Footer with count */}
      <div className="p-2 border-t border-gray-200 bg-gray-50 text-xs text-gray-500">
        Showing {messages.length} message{messages.length !== 1 ? 's' : ''}
        {selectedFlowId && <span> for flow: {selectedFlowId}</span>}
      </div>
    </div>
  );
}

export default MessageLogPanel;
