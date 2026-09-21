import { useCallback, useEffect, useMemo, useSyncExternalStore } from 'react';
import type { WebSocketMessage, WebSocketMessageType } from '../types/message';
import { wsClient, type ConnectionState, type SendOptions } from '../lib/wsClient';

export type WebSocketState = ConnectionState;
export type SendMessageOptions = SendOptions;

export interface WebSocketHook {
  state: WebSocketState;
  sendMessage: (type: WebSocketMessageType, data: any, options?: SendMessageOptions) => Promise<void>;
  sendRawMessage: (message: WebSocketMessage, options?: SendMessageOptions) => Promise<void>;
  subscribe: (type: WebSocketMessageType, callback: (data: any) => void) => () => void;
  unsubscribe: (type: WebSocketMessageType, callback: (data: any) => void) => void;
  reconnect: () => void;
  close: () => void;
}

/**
 * React binding for the editor's single WebSocket connection (see
 * lib/wsClient.ts). Calling this hook in any number of components never
 * opens more than one socket; it only subscribes the component to the
 * shared connection state. All returned functions are stable across
 * renders, so they are safe to list as effect dependencies.
 */
export function useWebSocket(): WebSocketHook {
  const state = useSyncExternalStore(wsClient.onStateChange, wsClient.getState, wsClient.getState);

  useEffect(() => {
    wsClient.connect();
  }, []);

  const sendMessage = useCallback(
    (type: WebSocketMessageType, data: any, options?: SendMessageOptions) => wsClient.send(type, data, options),
    []
  );
  const sendRawMessage = useCallback(
    (message: WebSocketMessage, options?: SendMessageOptions) => wsClient.sendRaw(message, options),
    []
  );
  const subscribe = useCallback(
    (type: WebSocketMessageType, callback: (data: any) => void) => wsClient.subscribe(type, callback),
    []
  );
  const unsubscribe = useCallback(
    (type: WebSocketMessageType, callback: (data: any) => void) => wsClient.unsubscribe(type, callback),
    []
  );
  const reconnect = useCallback(() => wsClient.reconnect(), []);
  const close = useCallback(() => wsClient.close(), []);

  return useMemo(
    () => ({ state, sendMessage, sendRawMessage, subscribe, unsubscribe, reconnect, close }),
    [state, sendMessage, sendRawMessage, subscribe, unsubscribe, reconnect, close]
  );
}

export default useWebSocket;
