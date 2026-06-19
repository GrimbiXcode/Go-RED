import { useState, useEffect, useCallback, useMemo } from 'react';
import { useWebSocket } from './useWebSocket';
import type { MessageLogEntry } from '../types/message';

export interface MessageLogState {
  messages: MessageLogEntry[];
  loading: boolean;
  error: Error | null;
  filterFlowId: string | null;
  filterText: string;
}

export interface MessageLogActions {
  loadMessages: (flowId?: string, limit?: number) => Promise<void>;
  clearMessages: () => void;
  setFilterFlowId: (flowId: string | null) => void;
  setFilterText: (text: string) => void;
  subscribeToMessages: () => void;
  unsubscribeFromMessages: () => void;
}

export type UseMessageLogReturn = MessageLogState & MessageLogActions;

// Message from backend (simplified for WebSocket)
interface BackendMessage {
  id: string;
  payload: Record<string, any>;
  metadata: Record<string, string>;
  flowId: string;
  path: string[];
  timestamp: string;
}

// Convert backend message to frontend MessageLogEntry
function convertBackendMessage(msg: BackendMessage): MessageLogEntry {
  return {
    id: msg.id,
    flowId: msg.flowId,
    message: {
      id: msg.id,
      payload: msg.payload,
      metadata: {
        ...msg.metadata,
        flowId: msg.flowId,
        path: msg.path.join(' -> '),
      },
      timestamp: msg.timestamp,
      sourceNode: msg.path.length > 0 ? msg.path[msg.path.length - 1] : undefined,
    },
    timestamp: msg.timestamp,
    level: 'info',
    nodeId: msg.path.length > 0 ? msg.path[msg.path.length - 1] : '',
  };
}

export function useMessageLog(selectedFlowId?: string): UseMessageLogReturn {
  const ws = useWebSocket();
  const [state, setState] = useState<MessageLogState>({
    messages: [],
    loading: false,
    error: null,
    filterFlowId: null,
    filterText: '',
  });

  const loadMessages = useCallback(async (flowId?: string, limit?: number) => {
    try {
      setState((prev) => ({ ...prev, loading: true, error: null }));
      
      // Request messages via WebSocket
      const requestData = {
        flowId: flowId || selectedFlowId || undefined,
        limit: limit || 100,
      };
      
      // Only send if we have a valid flowId or are requesting all messages
      if (requestData.flowId || requestData.limit) {
        ws.sendMessage('message:log', requestData);
      }
      
    } catch (error) {
      setState((prev) => ({
        ...prev,
        loading: false,
        error: error as Error,
      }));
    }
  }, [ws, selectedFlowId]);

  const clearMessages = useCallback(() => {
    setState((prev) => ({
      ...prev,
      messages: [],
    }));
  }, []);

  const setFilterFlowId = useCallback((flowId: string | null) => {
    setState((prev) => ({
      ...prev,
      filterFlowId: flowId,
    }));
  }, []);

  const setFilterText = useCallback((text: string) => {
    setState((prev) => ({
      ...prev,
      filterText: text,
    }));
  }, []);

  // Handle incoming message:log responses
  useEffect(() => {
    const subscription = ws.subscribe('message:log', (data: any) => {
      if (data && data.messages) {
        const backendMessages: BackendMessage[] = data.messages;
        const convertedMessages = backendMessages.map(convertBackendMessage);
        
        // Sort by timestamp (newest first)
        convertedMessages.sort((a, b) => 
          new Date(b.timestamp).getTime() - new Date(a.timestamp).getTime()
        );
        
        setState((prev) => ({
          ...prev,
          messages: convertedMessages,
          loading: false,
        }));
      }
    });

    return () => {
      subscription();
    };
  }, [ws]);

  // Auto-filter messages based on state
  const filteredMessages = useMemo(() => {
    let result = [...state.messages];
    
    // Filter by flow ID
    const effectiveFlowId = state.filterFlowId || selectedFlowId;
    if (effectiveFlowId) {
      result = result.filter((msg) => msg.flowId === effectiveFlowId);
    }
    
    // Filter by text
    if (state.filterText) {
      const query = state.filterText.toLowerCase();
      result = result.filter((msg) => 
        msg.message.id.toLowerCase().includes(query) ||
        JSON.stringify(msg.message.payload).toLowerCase().includes(query) ||
        msg.message.sourceNode?.toLowerCase().includes(query) ||
        msg.nodeId.toLowerCase().includes(query)
      );
    }
    
    return result;
  }, [state.messages, state.filterFlowId, state.filterText, selectedFlowId]);

  // Subscribe to messages when component mounts or flow changes
  useEffect(() => {
    if (selectedFlowId) {
      loadMessages(selectedFlowId);
    }
  }, [selectedFlowId]);

  // Merge filtered messages into state for external use
  const effectiveState = {
    ...state,
    messages: filteredMessages,
  };

  return {
    ...effectiveState,
    loadMessages,
    clearMessages,
    setFilterFlowId,
    setFilterText,
    subscribeToMessages: () => {}, // Already subscribed via useEffect
    unsubscribeFromMessages: () => {}, // Handled by useEffect cleanup
  };
}

export default useMessageLog;
