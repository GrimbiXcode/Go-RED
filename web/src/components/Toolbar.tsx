import { useState, useCallback } from 'react';
import type { Flow } from '../types/flow';
import type { FlowSummary } from '../types/api';
import { WebSocketStatus } from './WebSocketStatus';

interface ToolbarProps {
  flows: FlowSummary[];
  selectedFlow: Flow | null;
  onCreateNewFlow: () => void;
  onSelectFlow: (flowId: string) => void;
  onDeploy: () => void;
  onUndeploy: () => void;
  onSave: () => void;
  onToggleMessageLog: () => void;
  onExport: () => void;
  onImport: () => void;
}

export function Toolbar({
  flows,
  selectedFlow,
  onCreateNewFlow,
  onSelectFlow,
  onDeploy,
  onUndeploy,
  onSave,
  onToggleMessageLog,
  onExport,
  onImport,
}: ToolbarProps) {
  const [showFlowsDropdown, setShowFlowsDropdown] = useState(false);

  const handleDeployClick = useCallback(() => {
    if (selectedFlow && ['draft', 'stopped', 'error'].includes(selectedFlow.status)) {
      onDeploy();
    }
  }, [selectedFlow, onDeploy]);

  const handleUndeployClick = useCallback(() => {
    if (selectedFlow && ['deployed', 'running'].includes(selectedFlow.status)) {
      onUndeploy();
    }
  }, [selectedFlow, onUndeploy]);

  const canDeploy = selectedFlow && ['draft', 'stopped', 'error'].includes(selectedFlow.status);
  const canUndeploy = selectedFlow && ['deployed', 'running'].includes(selectedFlow.status);

  return (
    <div className="flex items-center justify-between p-2 bg-white border-b border-gray-200 shadow-sm">
      <div className="flex items-center gap-4">
        <div className="relative">
          <button
            className="flex items-center gap-2 px-3 py-2 bg-white border border-gray-300 rounded hover:bg-gray-50"
            onClick={() => setShowFlowsDropdown(!showFlowsDropdown)}
          >
            <span className="text-sm font-medium">
              {selectedFlow ? selectedFlow.name : 'Select Flow'}
            </span>
            <span className="text-sm">▼</span>
          </button>

          {showFlowsDropdown && (
            <div className="absolute z-10 mt-1 w-64 bg-white border border-gray-200 rounded shadow-lg">
              <div className="p-2 border-b border-gray-200">
                <button
                  className="w-full text-left text-sm text-blue-600 hover:bg-blue-50 p-2 rounded"
                  onClick={() => {
                    onCreateNewFlow();
                    setShowFlowsDropdown(false);
                  }}
                >
                  + New Flow
                </button>
              </div>
              
              <div className="max-h-64 overflow-y-auto">
                {flows.length === 0 ? (
                  <div className="p-4 text-sm text-gray-500">No flows available</div>
                ) : (
                  flows.map((flow) => (
                    <button
                      key={flow.id}
                      className={`w-full text-left p-2 text-sm hover:bg-gray-50 flex items-center justify-between ${
                        selectedFlow?.id === flow.id ? 'bg-blue-50' : ''
                      }`}
                      onClick={() => {
                        onSelectFlow(flow.id);
                        setShowFlowsDropdown(false);
                      }}
                    >
                      <span className="truncate flex-1">{flow.name}</span>
                      <span className={`text-xs px-2 py-1 rounded ${
                        flow.status === 'deployed' ? 'bg-green-100 text-green-600' :
                        flow.status === 'running' ? 'bg-blue-100 text-blue-600' :
                        flow.status === 'error' ? 'bg-red-100 text-red-600' :
                        'bg-gray-100 text-gray-600'
                      }`}>
                        {flow.status}
                      </span>
                    </button>
                  ))
                )}
              </div>
            </div>
          )}
        </div>

        <button
          className="px-3 py-2 bg-gray-100 text-gray-700 rounded hover:bg-gray-200 text-sm disabled:opacity-50 disabled:cursor-not-allowed"
          disabled={!selectedFlow}
          onClick={onSave}
          title={!selectedFlow ? 'Select a flow to save' : 'Save flow'}
        >
          💾 Save
        </button>
        
        <button
          className="px-3 py-2 bg-gray-100 text-gray-700 rounded hover:bg-gray-200 text-sm disabled:opacity-50 disabled:cursor-not-allowed"
          disabled={!selectedFlow}
          onClick={onExport}
          title={!selectedFlow ? 'Select a flow to export' : 'Export flow'}
        >
          📤 Export
        </button>
        
        <button
          className="px-3 py-2 bg-gray-100 text-gray-700 rounded hover:bg-gray-200 text-sm"
          onClick={onImport}
          title="Import flow"
        >
          📥 Import
        </button>
        
        <button
          className="px-3 py-2 bg-gray-100 text-gray-700 rounded hover:bg-gray-200 text-sm"
          onClick={onToggleMessageLog}
          title="Show/hide message log"
        >
          📜 Log
        </button>
      </div>

      <div className="flex-1 text-center">
        {selectedFlow && (
          <div className="text-sm font-medium text-gray-700 truncate">
            {selectedFlow.name}
          </div>
        )}
      </div>

      <div className="flex items-center gap-2">
        <button
          className={`px-3 py-2 rounded text-sm font-medium ${
            canDeploy
              ? 'bg-green-500 text-white hover:bg-green-600'
              : 'bg-gray-300 text-gray-500 cursor-not-allowed'
          }`}
          onClick={handleDeployClick}
          disabled={!canDeploy}
          title={canDeploy ? 'Deploy flow' : 'Flow is already deployed or running'}
        >
          ▶ Deploy
        </button>

        <button
          className={`px-3 py-2 rounded text-sm font-medium ${
            canUndeploy
              ? 'bg-red-500 text-white hover:bg-red-600'
              : 'bg-gray-300 text-gray-500 cursor-not-allowed'
          }`}
          onClick={handleUndeployClick}
          disabled={!canUndeploy}
          title={canUndeploy ? 'Undeploy flow' : 'Flow is not deployed'}
        >
          □ Stop
        </button>

        <WebSocketStatus />

        <div className="flex items-center gap-1">
          <button
            className="w-8 h-8 flex items-center justify-center bg-gray-100 rounded hover:bg-gray-200"
            title="Zoom in"
          >
            +
          </button>
          <button
            className="w-8 h-8 flex items-center justify-center bg-gray-100 rounded hover:bg-gray-200"
            title="Zoom out"
          >
            −
          </button>
          <button
            className="w-8 h-8 flex items-center justify-center bg-gray-100 rounded hover:bg-gray-200"
            title="Fit to view"
          >
            ⛶
          </button>
        </div>
      </div>
    </div>
  );
}