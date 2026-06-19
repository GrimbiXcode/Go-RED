
import type { Flow } from '../types/flow';
import type { FlowNode } from '../types/flow';

interface SidebarProps {
  flow: Flow | null;
  selectedNode: FlowNode | null;
  onConfigureNode: () => void;
}

export function Sidebar({ flow, selectedNode, onConfigureNode }: SidebarProps) {
  if (!flow) {
    return (
      <div className="p-4">
        <div className="text-sm text-gray-500">Select a flow to view details</div>
      </div>
    );
  }

  if (selectedNode) {
    return (
      <div className="p-4 h-full flex flex-col">
        <div className="flex items-center justify-between mb-4">
          <h3 className="font-semibold text-gray-700">Node Properties</h3>
          <button
            className="px-3 py-1 bg-blue-500 text-white rounded text-sm hover:bg-blue-600"
            onClick={onConfigureNode}
          >
            Configure
          </button>
        </div>

        <div className="space-y-4 flex-1 overflow-y-auto">
          <div>
            <label className="block text-sm font-medium text-gray-600 mb-1">ID</label>
            <div className="text-sm text-gray-800 bg-gray-50 p-2 rounded">{selectedNode.id}</div>
          </div>

          <div>
            <label className="block text-sm font-medium text-gray-600 mb-1">Type</label>
            <div className="text-sm text-gray-800 bg-gray-50 p-2 rounded">{selectedNode.type}</div>
          </div>

          <div>
            <label className="block text-sm font-medium text-gray-600 mb-1">Position</label>
            <div className="text-sm text-gray-800 bg-gray-50 p-2 rounded">
              X: {(selectedNode.position?.x ?? 0).toFixed(1)}, Y: {(selectedNode.position?.y ?? 0).toFixed(1)}
            </div>
          </div>

          {selectedNode.config && Object.keys(selectedNode.config).length > 0 && (
            <div>
              <label className="block text-sm font-medium text-gray-600 mb-1">Configuration</label>
              <pre className="text-xs text-gray-700 bg-gray-50 p-2 rounded overflow-auto">
                {JSON.stringify(selectedNode.config, null, 2)}
              </pre>
            </div>
          )}

          {selectedNode.status && (
            <div>
              <label className="block text-sm font-medium text-gray-600 mb-1">Status</label>
              <div className={`text-sm p-2 rounded ${
                selectedNode.status.state === 'error' ? 'bg-red-50 text-red-700' :
                selectedNode.status.state === 'processing' ? 'bg-yellow-50 text-yellow-700' :
                'bg-green-50 text-green-700'
              }`}>
                {selectedNode.status.state}
                {selectedNode.status.message && (
                  <div className="text-xs mt-1">{selectedNode.status.message}</div>
                )}
              </div>
            </div>
          )}
        </div>
      </div>
    );
  }

  return (
    <div className="p-4 h-full flex flex-col">
      <h3 className="font-semibold text-gray-700 mb-4">Flow Properties</h3>

      <div className="space-y-4 flex-1 overflow-y-auto">
        <div>
          <label className="block text-sm font-medium text-gray-600 mb-1">ID</label>
          <div className="text-sm text-gray-800 bg-gray-50 p-2 rounded">{flow.id}</div>
        </div>

        <div>
          <label className="block text-sm font-medium text-gray-600 mb-1">Name</label>
          <div className="text-sm text-gray-800 bg-gray-50 p-2 rounded">{flow.name}</div>
        </div>

        {flow.description && (
          <div>
            <label className="block text-sm font-medium text-gray-600 mb-1">Description</label>
            <div className="text-sm text-gray-800 bg-gray-50 p-2 rounded">{flow.description}</div>
          </div>
        )}

        <div>
          <label className="block text-sm font-medium text-gray-600 mb-1">Status</label>
          <div className={`text-sm p-2 rounded text-center font-medium ${
            flow.status === 'deployed' ? 'bg-green-50 text-green-700' :
            flow.status === 'running' ? 'bg-blue-50 text-blue-700' :
            flow.status === 'error' ? 'bg-red-50 text-red-700' :
            'bg-gray-50 text-gray-700'
          }`}>
            {flow.status}
          </div>
        </div>

        <div>
          <label className="block text-sm font-medium text-gray-600 mb-1">Statistics</label>
          <div className="grid grid-cols-2 gap-2">
            <div className="bg-gray-50 p-2 rounded">
              <div className="text-xs text-gray-500">Nodes</div>
              <div className="text-lg font-semibold text-gray-800">{flow.nodes ? Object.keys(flow.nodes).length : 0}</div>
            </div>
            <div className="bg-gray-50 p-2 rounded">
              <div className="text-xs text-gray-500">Connections</div>
              <div className="text-lg font-semibold text-gray-800">{flow.connections ? flow.connections.length : 0}</div>
            </div>
          </div>
        </div>

        {flow.config && Object.keys(flow.config).length > 0 && (
          <div>
            <label className="block text-sm font-medium text-gray-600 mb-1">Configuration</label>
            <pre className="text-xs text-gray-700 bg-gray-50 p-2 rounded overflow-auto">
              {JSON.stringify(flow.config, null, 2)}
            </pre>
          </div>
        )}

        <div className="mt-auto pt-4 border-t border-gray-200">
          <div className="text-xs text-gray-500">
            Created: {new Date(flow.createdAt).toLocaleString()}
          </div>
          <div className="text-xs text-gray-500">
            Updated: {new Date(flow.updatedAt).toLocaleString()}
          </div>
        </div>
      </div>
    </div>
  );
}