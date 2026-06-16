// Node metadata and port definitions for Go-RED

export interface NodeMetadata {
  id: string;
  type: string;
  name: string;
  description?: string;
  category: NodeCategory;
  icon?: string;
  color?: string;
  inputs: Port[];
  outputs: Port[];
  configSchema?: PropertySchema;
  defaultConfig?: Record<string, any>;
  tags?: string[];
  author?: string;
  version?: string;
  pluginId?: string;
}

export type NodeCategory = 
  | 'input'
  | 'output'
  | 'function'
  | 'storage'
  | 'network'
  | 'protocol'
  | 'parser'
  | 'social'
  | 'dashboard'
  | 'custom';

export interface Port {
  id: string;
  name: string;
  description?: string;
  type: PortType;
  required?: boolean;
  schema?: PropertySchema;
}

export type PortType = 'any' | 'string' | 'number' | 'boolean' | 'object' | 'array' | 'buffer' | 'message';

export interface PropertySchema {
  type: SchemaType;
  description?: string;
  default?: any;
  required?: boolean;
  enum?: any[];
  min?: number;
  max?: number;
  pattern?: string;
  properties?: Record<string, PropertySchema>;
  items?: PropertySchema;
  oneOf?: PropertySchema[];
  allOf?: PropertySchema[];
  format?: string;
  examples?: any[];
}

export type SchemaType = 
  | 'string'
  | 'number'
  | 'integer'
  | 'boolean'
  | 'object'
  | 'array'
  | 'null';

export interface NodeProperty {
  id: string;
  name: string;
  value: any;
  type: string;
  description?: string;
}

export interface NodeTypeDefinition {
  metadata: NodeMetadata;
  executor: string;
  scriptPath?: string;
  goPluginPath?: string;
}

export interface NodePaletteItem {
  type: string;
  name: string;
  category: NodeCategory;
  icon?: string;
  color?: string;
  description?: string;
  tags?: string[];
}

export interface NodeRegistry {
  [nodeType: string]: NodeMetadata;
}