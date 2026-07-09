// Message types for Go-RED flow communication
//
// Message and WebSocketMessage/WebSocketMessageType are generated from the
// Go backend (see internal/dto.Message and cmd/go-red/websocket) via
// `go generate ./internal/dto/...` — do not hand-describe their shape here.
// Message.metadata is Record<string, string> (Go cannot produce a richer,
// per-key-typed metadata object) — this file previously declared a
// MessageMetadata type with typed qos/retain fields that the backend never
// actually populated; it has been removed.
export type {
  Message,
  WebSocketMessage,
  MessageType as WebSocketMessageType,
} from './generated';

import type { Message } from './generated';

export interface MessageBatch {
  messages: Message[];
  timestamp: string;
  flowId: string;
}

export interface NodeMessage extends Message {
  nodeId: string;
  port: string;
  sequence?: number;
}

export interface FlowMessage extends Message {
  nodeId: string;
  port: string;
}

export interface MessageLogEntry {
  id: string;
  flowId: string;
  nodeId: string;
  message: Message;
  timestamp: string;
  level: 'debug' | 'info' | 'warn' | 'error';
}
