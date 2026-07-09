import { useState, useCallback } from 'react';
import type { FlowNode } from '../types/flow';
import type { NodeMetadata } from '../types/node';
import { getCategoryColor } from '../utils/nodeCategories';

interface NodeConfigModalProps {
  node: FlowNode;
  nodeTypes: NodeMetadata[];
  onClose: () => void;
  onSave: (config: Record<string, any>) => void;
  onDelete?: () => void;
}

/**
 * Node edit tray (Phase 5 of docs/FRONTEND_NODE_RED_REDESIGN.md): slides in
 * from the right at the same width/position as the Info/Debug sidebar
 * (see SidebarTabs.tsx), instead of a centered modal dialog with a
 * backdrop — matches Node-RED's edit dialog, which never blocks the rest
 * of the editor.
 */
export function NodeConfigModal({ node, nodeTypes, onClose, onSave, onDelete }: NodeConfigModalProps) {
  const [config, setConfig] = useState<Record<string, any>>({ ...node.config });

  const nodeMetadata = nodeTypes.find((nt) => nt.type === node.type);
  const category = nodeMetadata?.category || 'custom';
  const color = getCategoryColor(category);

  const handleInputChange = useCallback((key: string, value: any) => {
    setConfig((prev) => ({
      ...prev,
      [key]: value,
    }));
  }, []);

  const handleSubmit = useCallback((e: React.FormEvent) => {
    e.preventDefault();
    onSave(config);
  }, [config, onSave]);

  const renderInputField = (key: string, schema: any, value: any) => {
    switch (schema.type) {
      case 'string':
        return (
          <div>
            <label className="block text-xs font-medium text-gray-600 mb-1">
              {schema.description || key}
            </label>
            <input
              type="text"
              value={value || ''}
              onChange={(e) => handleInputChange(key, e.target.value)}
              className="w-full px-2 py-1.5 border rounded text-xs border-gray-300 focus:outline-none focus:ring-2 focus:ring-gr-blue-500"
              placeholder={schema.default || ''}
            />
          </div>
        );
      case 'number':
      case 'integer':
        return (
          <div>
            <label className="block text-xs font-medium text-gray-600 mb-1">
              {schema.description || key}
            </label>
            <input
              type="number"
              value={value || ''}
              onChange={(e) => handleInputChange(key, schema.type === 'integer' ? parseInt(e.target.value) || 0 : parseFloat(e.target.value) || 0)}
              className="w-full px-2 py-1.5 border rounded text-xs border-gray-300 focus:outline-none focus:ring-2 focus:ring-gr-blue-500"
              min={schema.min}
              max={schema.max}
              placeholder={schema.default || ''}
            />
          </div>
        );
      case 'boolean':
        return (
          <div className="flex items-center gap-2">
            <label className="text-xs font-medium text-gray-600">
              {schema.description || key}
            </label>
            <input
              type="checkbox"
              checked={value || false}
              onChange={(e) => handleInputChange(key, e.target.checked)}
              className="h-4 w-4"
            />
          </div>
        );
      case 'object':
        return (
          <div>
            <label className="block text-xs font-medium text-gray-600 mb-1">
              {schema.description || key}
            </label>
            <textarea
              value={JSON.stringify(value || {}, null, 2)}
              onChange={(e) => {
                try {
                  handleInputChange(key, JSON.parse(e.target.value));
                } catch {
                  handleInputChange(key, e.target.value);
                }
              }}
              className="w-full px-2 py-1.5 border rounded text-xs font-mono border-gray-300 focus:outline-none focus:ring-2 focus:ring-gr-blue-500"
              rows={5}
              placeholder="Enter JSON object"
            />
          </div>
        );
      case 'array':
        return (
          <div>
            <label className="block text-xs font-medium text-gray-600 mb-1">
              {schema.description || key}
            </label>
            <textarea
              value={JSON.stringify(value || [], null, 2)}
              onChange={(e) => {
                try {
                  handleInputChange(key, JSON.parse(e.target.value));
                } catch {
                  handleInputChange(key, e.target.value);
                }
              }}
              className="w-full px-2 py-1.5 border rounded text-xs font-mono border-gray-300 focus:outline-none focus:ring-2 focus:ring-gr-blue-500"
              rows={3}
              placeholder="Enter JSON array"
            />
          </div>
        );
      default:
        return (
          <div>
            <label className="block text-xs font-medium text-gray-600 mb-1">
              {schema.description || key}
            </label>
            <input
              type="text"
              value={value || ''}
              onChange={(e) => handleInputChange(key, e.target.value)}
              className="w-full px-2 py-1.5 border rounded text-xs border-gray-300 focus:outline-none focus:ring-2 focus:ring-gr-blue-500"
            />
          </div>
        );
    }
  };

  const renderEnumSelector = (key: string, schema: any, value: any) => {
    return (
      <div>
        <label className="block text-xs font-medium text-gray-600 mb-1">
          {schema.description || key}
        </label>
        <select
          value={value || ''}
          onChange={(e) => handleInputChange(key, e.target.value)}
          className="w-full px-2 py-1.5 border rounded text-xs border-gray-300 focus:outline-none focus:ring-2 focus:ring-gr-blue-500"
        >
          {schema.enum?.map((option: any) => (
            <option key={option} value={option}>
              {option}
            </option>
          ))}
        </select>
      </div>
    );
  };

  return (
    <div className="fixed top-11 bottom-0 right-9 w-80 bg-white border-l border-gray-200 shadow-xl z-30 flex flex-col animate-slide-in-right">
      <div className="flex items-center gap-2 px-3 py-2 border-b border-gray-200">
        <span className={`w-3 h-3 rounded-sm shrink-0 ${color.swatch}`} aria-hidden="true" />
        <div className="flex-1 min-w-0">
          <div className="text-sm font-semibold text-gray-800 truncate">
            {nodeMetadata?.name || node.type}
          </div>
          <div className="text-[10px] text-gray-400 truncate">{node.id}</div>
        </div>
        <button
          className="text-gray-400 hover:text-gray-600 shrink-0"
          onClick={onClose}
          aria-label="Schließen"
        >
          ✕
        </button>
      </div>

      <form onSubmit={handleSubmit} className="flex-1 overflow-y-auto p-3 space-y-3">
        {nodeMetadata?.configSchema ? (
          <div className="space-y-3">
            {Object.entries(nodeMetadata.configSchema.properties || {}).map(([key, schema]) => {
              const value = config[key];
              if (schema.enum) {
                return renderEnumSelector(key, schema, value);
              }
              return renderInputField(key, schema, value);
            })}
          </div>
        ) : (
          <div className="space-y-3">
            <div className="text-xs text-gray-600">
              No configuration schema available for this node type.
            </div>
            <div>
              <label className="block text-xs font-medium text-gray-600 mb-1">
                Custom Configuration (JSON)
              </label>
              <textarea
                value={JSON.stringify(config || {}, null, 2)}
                onChange={(e) => {
                  try {
                    setConfig(JSON.parse(e.target.value));
                  } catch {
                  }
                }}
                className="w-full px-2 py-1.5 border border-gray-300 rounded text-xs font-mono focus:outline-none focus:ring-2 focus:ring-gr-blue-500"
                rows={10}
                placeholder="Enter configuration as JSON"
              />
            </div>
          </div>
        )}
      </form>

      <div className="flex items-center justify-between gap-2 p-3 border-t border-gray-200">
        {onDelete ? (
          <button
            type="button"
            className="px-3 py-1.5 text-xs text-gr-fuchsia-600 hover:bg-gr-fuchsia-50 rounded"
            onClick={onDelete}
          >
            Löschen
          </button>
        ) : (
          <span />
        )}
        <div className="flex items-center gap-2">
          <button
            type="button"
            className="px-3 py-1.5 text-xs text-gray-600 hover:bg-gray-100 rounded"
            onClick={onClose}
          >
            Abbrechen
          </button>
          <button
            type="button"
            className="px-3 py-1.5 text-xs font-medium bg-gr-blue-500 text-white rounded hover:bg-gr-blue-600"
            onClick={handleSubmit}
          >
            Fertig
          </button>
        </div>
      </div>
    </div>
  );
}
