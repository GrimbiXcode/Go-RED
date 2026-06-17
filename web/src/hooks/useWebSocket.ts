import { useState, useEffect, useCallback, useRef } from 'react';
import type { WebSocketMessage, WebSocketMessageType } from '../types/message';
import { getWebSocketUrl } from '../utils/api';

export interface WebSocketState {
  connected: boolean;
  connecting: boolean;
  error: Error | null;
  lastMessage: WebSocketMessage | null;
  lastError: WebSocketMessage | null;
}

export interface SendMessageOptions {
  retry?: boolean;
  timeout?: number;
}

export interface WebSocketHook {
  state: WebSocketState;
  sendMessage: (type: WebSocketMessageType, data: any, options?: SendMessageOptions) => Promise<void>;
  sendRawMessage: (message: WebSocketMessage) => Promise<void>;
  subscribe: (type: WebSocketMessageType, callback: (data: any) => void) => () => void;
  unsubscribe: (type: WebSocketMessageType, callback: (data: any) => void) => void;
  reconnect: () => void;
  close: () => void;
}

export function useWebSocket(): WebSocketHook {
  const [state, setState] = useState<WebSocketState>({
    connected: false,
    connecting: true,
    error: null,
    lastMessage: null,
    lastError: null,
  });
  const wsRef = useRef<WebSocket | null>(null);
  const subscribersRef = useRef<Map<WebSocketMessageType, Set<(data: any) => void>>>(new Map());
  const messageQueueRef = useRef<WebSocketMessage[]>([]);
  const reconnectAttemptsRef = useRef<number>(0);
  const maxReconnectAttempts = useRef<number>(10);
  const reconnectDelayRef = useRef<number>(1000);
  const createConnection = useCallback((url: string) => {
    try {
      const ws = new WebSocket(url);
      wsRef.current = ws;
      ws.onopen = () => {
        setState((prev) => ({
          ...prev,
          connected: true,
          connecting: false,
          error: null,
        }));
        if (messageQueueRef.current.length > 0) {
          messageQueueRef.current.forEach((message) => {
            if (ws.readyState === WebSocket.OPEN) {
              ws.send(JSON.stringify(message));
            }
          });
          messageQueueRef.current = [];
        }
        reconnectAttemptsRef.current = 0;
      };
      ws.onclose = () => {
        setState((prev) => ({
          ...prev,
          connected: false,
          connecting: false,
        }));
        if (reconnectAttemptsRef.current < maxReconnectAttempts.current) {
          setTimeout(() => {
            setState((prev) => ({ ...prev, connecting: true }));
            createConnection(url);
            reconnectAttemptsRef.current++;
          }, reconnectDelayRef.current * (reconnectAttemptsRef.current + 1));
        }
      };
      ws.onerror = (event) => {
        setState((prev) => ({
          ...prev,
          error: new Error(`WebSocket error: ${event.type}`),
        }));
      };
      ws.onmessage = (event) => {
        try {
          const message: WebSocketMessage = JSON.parse(event.data as string);
          setState((prev) => ({
            ...prev,
            lastMessage: message,
            lastError: message.type === 'error' ? message : prev.lastError,
          }));
          if (message.type && subscribersRef.current.has(message.type)) {
            const callbacks = subscribersRef.current.get(message.type)!;
            callbacks.forEach((callback) => {
              try {
                callback(message.data);
              } catch (error) {
                console.error(`Error in WebSocket subscriber for ${message.type}:`, error);
              }
            });
          }
          if (subscribersRef.current.has('*')) {
            const callbacks = subscribersRef.current.get('*')!;
            callbacks.forEach((callback) => {
              try {
                callback(message);
              } catch (error) {
                console.error('Error in WebSocket all-messages subscriber:', error);
              }
            });
          }
        } catch (error) {
          console.error('Error parsing WebSocket message:', error);
        }
      };
    } catch (error) {
      setState((prev) => ({
        ...prev,
        error: error as Error,
        connecting: false,
      }));
    }
  }, []);
  const subscribe = useCallback((
    type: WebSocketMessageType,
    callback: (data: any) => void
  ): (() => void) => {
    if (!subscribersRef.current.has(type)) {
      subscribersRef.current.set(type, new Set());
    }
    const callbacks = subscribersRef.current.get(type)!;
    callbacks.add(callback);
    return () => {
      callbacks.delete(callback);
      if (callbacks.size === 0) {
        subscribersRef.current.delete(type);
      }
    };
  }, []);
  const unsubscribe = useCallback((
    type: WebSocketMessageType,
    callback: (data: any) => void
  ) => {
    if (subscribersRef.current.has(type)) {
      const callbacks = subscribersRef.current.get(type)!;
      callbacks.delete(callback);
      if (callbacks.size === 0) {
        subscribersRef.current.delete(type);
      }
    }
  }, []);
  const sendMessage = useCallback(
    async (type: WebSocketMessageType, data: any, options: SendMessageOptions = {}) => {
      const { retry = true, timeout = 5000 } = options;
      const message: WebSocketMessage = {
        type,
        data,
        timestamp: new Date().toISOString(),
      };
      return sendRawMessage(message, { retry, timeout });
    },
    []
  );
  const sendRawMessage = useCallback(
    async (message: WebSocketMessage, options: SendMessageOptions = {}) => {
      const { retry = true, timeout = 5000 } = options;
      const attemptSend = async (): Promise<void> => {
        return new Promise((resolve, reject) => {
          const ws = wsRef.current;
          if (!ws) {
            reject(new Error('WebSocket not initialized'));
            return;
          }
          if (ws.readyState === WebSocket.OPEN) {
            try {
              ws.send(JSON.stringify(message));
              resolve();
            } catch (error) {
              reject(error);
            }
          } else if (ws.readyState === WebSocket.CONNECTING) {
            messageQueueRef.current.push(message);
            const timeoutId = setTimeout(() => {
              reject(new Error('WebSocket connection timeout'));
            }, timeout);
            const checkConnected = () => {
              if (ws.readyState === WebSocket.OPEN) {
                clearTimeout(timeoutId);
                ws.removeEventListener('open', checkConnected);
                resolve();
              }
            };
            ws.addEventListener('open', checkConnected);
          } else {
            reject(new Error(`WebSocket not open: ${ws.readyState}`));
          }
        });
      };
      if (retry) {
        let attempts = 0;
        const maxAttempts = 3;
        while (attempts < maxAttempts) {
          try {
            await attemptSend();
            return;
          } catch (error) {
            attempts++;
            if (attempts >= maxAttempts) {
              throw error;
            }
            await new Promise((resolve) => setTimeout(resolve, 100 * attempts));
          }
        }
      } else {
        await attemptSend();
      }
    },
    []
  );
  const reconnect = useCallback(() => {
    if (wsRef.current) {
      wsRef.current.close();
    }
    reconnectAttemptsRef.current = 0;
    setState({
      connected: false,
      connecting: true,
      error: null,
      lastMessage: null,
      lastError: null,
    });
  }, []);
  const close = useCallback(() => {
    if (wsRef.current) {
      wsRef.current.close();
      wsRef.current = null;
    }
    setState({
      connected: false,
      connecting: false,
      error: null,
      lastMessage: null,
      lastError: null,
    });
  }, []);
  useEffect(() => {
    const url = getWebSocketUrl();
    createConnection(url);
    return () => {
      close();
    };
  }, [createConnection, close]);
  return {
    state,
    sendMessage,
    sendRawMessage,
    subscribe,
    unsubscribe,
    reconnect,
    close,
  };
}

export default useWebSocket;