import React, { useState, useCallback } from 'react';
import type { FlowNode } from '../types/flow';
import type { NodeMetadata } from '../types/node';

interface NodeConfigModalProps {
  node: FlowNode;
  nodeTypes: NodeMetadata[];
  onClose: () => void;
  onSave: (config: Record<string, any>) => void;
}

export function NodeConfigModal({ node, nodeTypes, onClose, onSave }: NodeConfigModalProps) {
  const [config, setConfig] = useState<Record<string, any>>({ ...node.config });
  const [errors, setErrors] = useState<Record<string, string>>({});

  const nodeMetadata = nodeTypes.find((nt) => nt.type === node.type);

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
    const error = errors[key];
    switch (schema.type) {
      case 'string':
        return (
          <div>
            <label className="block text-sm font-medium text-gray-600 mb-1">
              {schema.description || key}
            </label>
            <input
              type="text"
              value={value || ''}
              onChange={(e) => handleInputChange(key, e.target.value)}
              className={`w-full p-2 border rounded text-sm ${error ? 'border-red-500' : 'border-gray-300'}`}
              placeholder={schema.default || ''}
            />
            {error && <div className="text-xs text-red-500 mt-1">{error}</div>}
          </div>
        );
      case 'number':
      case 'integer':
        return (
          <div>
            <label className="block text-sm font-medium text-gray-600 mb-1">
              {schema.description || key}
            </label>
            <input
              type="number"
              value={value || ''}
              onChange={(e) => handleInputChange(key, schema.type === 'integer' ? parseInt(e.target.value) || 0 : parseFloat(e.target.value) || 0)}
              className={`w-full p-2 border rounded text-sm ${error ? 'border-red-500' : 'border-gray-300'}`}
              min={schema.min}
              max={schema.max}
              placeholder={schema.default || ''}
            />
            {error && <div className="text-xs text-red-500 mt-1">{error}</div>}
          </div>
        );
      case 'boolean':
        return (
          <div className="flex items-center gap-2">
            <label className="text-sm font-medium text-gray-600">
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
            <label className="block text-sm font-medium text-gray-600 mb-1">
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
              className={`w-full p-2 border rounded text-sm font-mono ${error ? 'border-red-500' : 'border-gray-300'}`}
              rows={5}
              placeholder="Enter JSON object"
            />
            {error && <div className="text-xs text-red-500 mt-1">{error}</div>}
          </div>
        );
      case 'array':
        return (
          <div>
            <label className="block text-sm font-medium text-gray-600 mb-1">
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
              className={`w-full p-2 border rounded text-sm font-mono ${error ? 'border-red-500' : 'border-gray-300'}`}
              rows={3}
              placeholder="Enter JSON array"
            />
            {error && <div className="text-xs text-red-500 mt-1">{error}</div>}
          </div>
        );
      default:
        return (
          <div>
            <label className="block text-sm font-medium text-gray-600 mb-1">
              {schema.description || key}
            </label>
            <input
              type="text"
              value={value || ''}
              onChange={(e) => handleInputChange(key, e.target.value)}
              className={`w-full p-2 border rounded text-sm ${error ? 'border-red-500' : 'border-gray-300'}`}
            />
            {error && <div className="text-xs text-red-500 mt-1">{error}</div>}
          </div>
        );
    }
  };

  const renderEnumSelector = (key: string, schema: any, value: any) => {
    const error = errors[key];
    return (
      <div>
        <label className="block text-sm font-medium text-gray-600 mb-1">
          {schema.description || key}
        </label>
        <select
          value={value || ''}
          onChange={(e) => handleInputChange(key, e.target.value)}
          className={`w-full p-2 border rounded text-sm ${error ? 'border-red-500' : 'border-gray-300'}`}
        >
          {schema.enum?.map((option: any) => (
            <option key={option} value={option}>
              {option}
            </option>
          ))}
        </select>
        {error && <div className="text-xs text-red-500 mt-1">{error}</div>}
      </div>
    );
  };

  return (
    <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50 p-4">
      <div className="bg-white rounded-lg shadow-xl w-full max-w-2xl max-h-[90vh] overflow-y-auto">
        <div className="flex items-center justify-between p-4 border-b border-gray-200">
          <h3 className="font-semibold text-gray-800">
            Configure Node: {node.type}
          </h3>
          <button
            className="text-gray-400 hover:text-gray-600"
            onClick={onClose}
          >
            ✕
          </button>
        </div>

        <form onSubmit={handleSubmit} className="p-4 space-y-4">
          {nodeMetadata?.configSchema ? (
            <div className="space-y-4">
              {Object.entries(nodeMetadata.configSchema.properties || {}).map(([key, schema]) => {
                const value = config[key];
                if (schema.enum) {
                  return renderEnumSelector(key, schema, value);
                }
                return renderInputField(key, schema, value);
              })}
            </div>
          ) : (
            <div className="space-y-4">
              <div className="text-sm text-gray-600">
                No configuration schema available for this node type.
              </div>
              <div>
                <label className="block text-sm font-medium text-gray-600 mb-1">
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
                  className="w-full p-2 border border-gray-300 rounded text-sm font-mono"
                  rows={10}
                  placeholder="Enter configuration as JSON"
                />
              </div>
            </div>
          )}
        </form>

        <div className="flex items-center justify-end gap-2 p-4 border-t border-gray-200">
          <button
            className="px-4 py-2 bg-gray-200 text-gray-700 rounded hover:bg-gray-300 text-sm"
            onClick={onClose}
          >
            Cancel
          </button>
          <button
            className="px-4 py-2 bg-blue-500 text-white rounded hover:bg-blue-600 text-sm"
            onClick={handleSubmit}
          >
            Save Configuration
          </button>
        </div>
      </div>
    </div>
  );
}