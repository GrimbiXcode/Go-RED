// Message types for Go-RED flow communication

export interface Message {
  id: string;
  payload: any;
  metadata?: MessageMetadata;
  timestamp: string;
  sourceNode?: string;
  sourcePort?: string;
}

export interface MessageMetadata {
  topic?: string;
  qos?: number;
  retain?: boolean;
  flowId?: string;
  nodeId?: string;
  connectionId?: string;
  [key: string]: any;
}

export interface MessageBatch {
  messages: Message[];
  timestamp: string;
  flowId: string;
}

export interface NodeMessage extends Message {
  nodeId: string;
  port: string;
  flowId: string;
  sequence?: number;
}

export interface WebSocketMessage {
  type: WebSocketMessageType;
  data: any;
  timestamp: string;
  requestId?: string;
}

export type WebSocketMessageType = 
  | '*'
  | 'flow:list'
  | 'flow:get'
  | 'flow:create'
  | 'flow:update'
  | 'flow:delete'
  | 'flow:deploy'
  | 'flow:undeploy'
  | 'flow:status'
  | 'flow:export'
  | 'flow:import'
  | 'node:add'
  | 'node:remove'
  | 'node:update'
  | 'node:config'
  | 'node:status'
  | 'connection:add'
  | 'connection:remove'
  | 'connection:update'
  | 'message:send'
  | 'message:log'
  | 'message:debug'
  | 'error'
  | 'warning'
  | 'info'
  | 'ping'
  | 'pong'
  | 'state:sync';

export interface FlowMessage extends Message {
  flowId: string;
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