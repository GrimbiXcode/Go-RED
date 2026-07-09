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
    <div className="fixed inset-0 bg-black/40 flex items-center justify-center z-50 p-4">
      <div className="bg-white rounded shadow-xl w-full max-w-md">
        <div className="flex items-center justify-between px-4 py-3 border-b border-gray-200">
          <h3 className="font-semibold text-sm text-gray-800">Export Flow</h3>
          <button
            className="text-gray-400 hover:text-gray-600"
            onClick={onClose}
            aria-label="Schließen"
          >
            ✕
          </button>
        </div>

        <div className="p-4">
          <p className="text-sm text-gray-600 mb-4">
            You are about to export the flow <strong>{flowName}</strong> as a JSON file.
            This file can be imported back into Go-RED or shared with others.
          </p>

          <div className="bg-gray-50 p-3 rounded mb-4">
            <div className="text-xs text-gray-600">
              <div><strong>Flow ID:</strong> {flowId}</div>
              <div><strong>File Name:</strong> flow-{flowId}.json</div>
            </div>
          </div>
        </div>

        <div className="flex items-center justify-end gap-2 px-4 py-3 border-t border-gray-200">
          <button
            className="px-3 py-1.5 text-xs text-gray-600 hover:bg-gray-100 rounded"
            onClick={onClose}
            disabled={isExporting}
          >
            Cancel
          </button>
          <button
            className="px-3 py-1.5 bg-gr-blue-500 text-white rounded hover:bg-gr-blue-600 text-xs font-medium disabled:opacity-50 disabled:cursor-not-allowed"
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
