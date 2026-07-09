// Node metadata and port definitions for Go-RED
//
// NodeMetadata and Port are generated from the Go backend (see
// internal/registry) via `go generate ./internal/dto/...` — do not
// hand-describe their shape here. PropertySchema is a rename of the
// generated Property type (registry.Property is a flat schema — Go has no
// recursive properties/items/oneOf/allOf, unlike this file's previous
// hand-written PropertySchema, which described a shape the backend never
// produced).
export type { NodeMetadata, Port, Property as PropertySchema, Schema } from './generated';

import type { NodeMetadata } from './generated';

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
