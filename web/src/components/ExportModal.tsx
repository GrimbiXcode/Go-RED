import { useState, useCallback } from 'react';
import { useToast } from './ToastNotification';
import { exportFlow } from '../utils/api';

interface ExportModalProps {
  flowId: string;
  flowName: string;
  isOpen: boolean;
  onClose: () => void;
}

export function ExportModal({ flowId, flowName, isOpen, onClose }: ExportModalProps) {
  const { showToast } = useToast();
  const [isExporting, setIsExporting] = useState(false);

  const handleExport = useCallback(async () => {
    try {
      setIsExporting(true);
      await exportFlow(flowId);
      showToast('success', `Flow '${flowName}' exported successfully!`);
      onClose();
    } catch (error) {
      showToast('error', `Failed to export flow: ${error instanceof Error ? error.message : String(error)}`);
    } finally {
      setIsExporting(false);
    }
  }, [flowId, flowName, onClose, showToast]);

  if (!isOpen) {
    return null;
  }

  return (
    <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50 p-4">
      <div className="bg-white rounded-lg shadow-xl w-full max-w-md">
        <div className="flex items-center justify-between p-4 border-b border-gray-200">
          <h3 className="font-semibold text-gray-800">Export Flow</h3>
          <button
            className="text-gray-400 hover:text-gray-600"
            onClick={onClose}
          >
            ✕
          </button>
        </div>

        <div className="p-4">
          <p className="text-gray-600 mb-4">
            You are about to export the flow <strong>{flowName}</strong> as a JSON file.
            This file can be imported back into Go-RED or shared with others.
          </p>

          <div className="bg-gray-50 p-3 rounded mb-4">
            <div className="text-sm text-gray-600">
              <div><strong>Flow ID:</strong> {flowId}</div>
              <div><strong>File Name:</strong> flow-{flowId}.json</div>
            </div>
          </div>
        </div>

        <div className="flex items-center justify-end gap-2 p-4 border-t border-gray-200">
          <button
            className="px-4 py-2 bg-gray-200 text-gray-700 rounded hover:bg-gray-300 text-sm"
            onClick={onClose}
            disabled={isExporting}
          >
            Cancel
          </button>
          <button
            className="px-4 py-2 bg-blue-500 text-white rounded hover:bg-blue-600 text-sm disabled:opacity-50 disabled:cursor-not-allowed"
            onClick={handleExport}
            disabled={isExporting}
          >
            {isExporting ? 'Exporting...' : 'Export Flow'}
          </button>
        </div>
      </div>
    </div>
  );
}

export default ExportModal;
