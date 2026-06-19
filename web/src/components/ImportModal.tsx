import { useState, useCallback, useRef, ChangeEvent } from 'react';
import { useToast } from './ToastNotification';
import { importFlow } from '../utils/api';
import type { Flow } from '../types/flow';

interface ImportModalProps {
  isOpen: boolean;
  onClose: () => void;
  onFlowImported: (flowId: string) => void;
}

export function ImportModal({ isOpen, onClose, onFlowImported }: ImportModalProps) {
  const { showToast } = useToast();
  const [isImporting, setIsImporting] = useState(false);
  const [selectedFile, setSelectedFile] = useState<File | null>(null);
  const [fileContent, setFileContent] = useState<string>('');
  const [importedFlow, setImportedFlow] = useState<Flow | null>(null);
  const [error, setError] = useState<string>('');
  const fileInputRef = useRef<HTMLInputElement>(null);

  const handleFileChange = useCallback((event: ChangeEvent<HTMLInputElement>) => {
    const file = event.target.files?.[0];
    if (!file) {
      setError('No file selected');
      return;
    }

    // Validate file type
    if (!file.name.endsWith('.json')) {
      setError('Please select a JSON file');
      return;
    }

    setSelectedFile(file);
    setError('');
    
    // Read and validate file content
    const reader = new FileReader();
    reader.onload = (e) => {
      try {
        const content = e.target?.result as string;
        setFileContent(content);
        const flowData = JSON.parse(content);
        
        // Basic validation
        if (!flowData.name) {
          setError('Invalid flow file: missing "name" field');
          return;
        }
        
        setImportedFlow({
          id: flowData.id || '',
          name: flowData.name,
          description: flowData.description || '',
          nodes: flowData.nodes || {},
          connections: flowData.connections || [],
          status: flowData.status || 'draft',
          config: flowData.config || {},
          createdAt: flowData.createdAt || new Date().toISOString(),
          updatedAt: flowData.updatedAt || new Date().toISOString(),
        });
      } catch {
        setError('Invalid JSON file');
      }
    };
    reader.readAsText(file);
  }, []);

  const handleImport = useCallback(async () => {
    if (!selectedFile || !fileContent) {
      setError('No file selected');
      return;
    }

    try {
      setIsImporting(true);
      setError('');
      
      const result = await importFlow(selectedFile);
      showToast('success', `Flow '${result.name}' imported successfully!`);
      onFlowImported(result.flowId);
      onClose();
    } catch (err) {
      const errorMessage = err instanceof Error ? err.message : String(err);
      setError(errorMessage);
      showToast('error', `Failed to import flow: ${errorMessage}`);
    } finally {
      setIsImporting(false);
    }
  }, [selectedFile, fileContent, onClose, onFlowImported, showToast]);

  const handleReset = useCallback(() => {
    setSelectedFile(null);
    setFileContent('');
    setImportedFlow(null);
    setError('');
    if (fileInputRef.current) {
      fileInputRef.current.value = '';
    }
  }, []);

  if (!isOpen) {
    return null;
  }

  return (
    <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50 p-4">
      <div className="bg-white rounded-lg shadow-xl w-full max-w-md">
        <div className="flex items-center justify-between p-4 border-b border-gray-200">
          <h3 className="font-semibold text-gray-800">Import Flow</h3>
          <button
            className="text-gray-400 hover:text-gray-600"
            onClick={onClose}
          >
            ✕
          </button>
        </div>

        <div className="p-4">
          <p className="text-gray-600 mb-4">
            Select a JSON file to import a flow. The file should have been previously exported from Go-RED.
          </p>

          <div className="mb-4">
            <label className="block text-sm font-medium text-gray-700 mb-2">
              Flow File
            </label>
            <div className="flex gap-2">
              <input
                type="file"
                ref={fileInputRef}
                onChange={handleFileChange}
                accept=".json"
                className="flex-1 text-sm"
                disabled={isImporting}
              />
              <button
                className="px-3 py-2 bg-gray-200 text-gray-700 rounded hover:bg-gray-300 text-sm"
                onClick={() => fileInputRef.current?.click()}
                disabled={isImporting}
              >
                Browse
              </button>
            </div>
          </div>

          {error && (
            <div className="bg-red-50 text-red-600 p-3 rounded mb-4 text-sm">
              Error: {error}
            </div>
          )}

          {importedFlow && (
            <div className="bg-green-50 p-3 rounded mb-4">
              <h4 className="font-medium text-green-800 mb-2">Flow Preview</h4>
              <div className="text-sm text-gray-700 space-y-1">
                <div><strong>Name:</strong> {importedFlow.name}</div>
                <div><strong>Description:</strong> {importedFlow.description || 'None'}</div>
                <div><strong>Nodes:</strong> {importedFlow.nodes ? Object.keys(importedFlow.nodes).length : 0}</div>
                <div><strong>Connections:</strong> {importedFlow.connections ? importedFlow.connections.length : 0}</div>
              </div>
            </div>
          )}
        </div>

        <div className="flex items-center justify-end gap-2 p-4 border-t border-gray-200">
          <button
            className="px-4 py-2 bg-gray-200 text-gray-700 rounded hover:bg-gray-300 text-sm"
            onClick={onClose}
            disabled={isImporting}
          >
            Cancel
          </button>
          <button
            className="px-4 py-2 bg-gray-200 text-gray-700 rounded hover:bg-gray-300 text-sm"
            onClick={handleReset}
            disabled={isImporting || !selectedFile}
          >
            Clear
          </button>
          <button
            className="px-4 py-2 bg-blue-500 text-white rounded hover:bg-blue-600 text-sm disabled:opacity-50 disabled:cursor-not-allowed"
            onClick={handleImport}
            disabled={isImporting || !selectedFile || !!error}
          >
            {isImporting ? 'Importing...' : 'Import Flow'}
          </button>
        </div>
      </div>
    </div>
  );
}

export default ImportModal;
