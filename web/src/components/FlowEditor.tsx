import React, { useState, useCallback } from 'react';
import { useFlowContext } from './FlowProvider';
import { FlowCanvas } from './FlowCanvas';
import { NodePalette } from './NodePalette';
import { Sidebar } from './Sidebar';
import { Toolbar } from './Toolbar';
import { NodeConfigModal } from './NodeConfigModal';
import type { FlowNode } from '../types/flow';

export function FlowEditor() {
  const {
    flows,
    loading,
    error,
    selectedFlow,
    nodeTypes,
    nodeTypesLoading,
    createNewFlow,
    selectFlow,
    addNode,
    removeNode,
    updateNode,
    addConnection,
    removeConnection,
    deployCurrentFlow,
    undeployCurrentFlow,
  } = useFlowContext();

  const [selectedNode, setSelectedNode] = useState<FlowNode | null>(null);
  const [showConfigModal, setShowConfigModal] = useState(false);

  const handleCreateNewFlow = useCallback(async () => {
    const newFlow = await createNewFlow('New Flow', 'A new flow');
    selectFlow(newFlow.id);
  }, [createNewFlow, selectFlow]);

  const handleSelectFlow = useCallback((flowId: string) => {
    selectFlow(flowId);
    setSelectedNode(null);
  }, [selectFlow]);

  const handleNodeSelect = useCallback((node: FlowNode) => {
    setSelectedNode(node);
  }, []);

  const handleNodeDeselect = useCallback(() => {
    setSelectedNode(null);
  }, []);

  const handleConfigureNode = useCallback(() => {
    if (selectedNode) {
      setShowConfigModal(true);
    }
  }, [selectedNode]);

  const handleCloseConfigModal = useCallback(() => {
    setShowConfigModal(false);
  }, []);

  const handleSaveNodeConfig = useCallback((config: Record<string, any>) => {
    if (selectedNode) {
      updateNode(selectedNode.id, { config });
      setShowConfigModal(false);
      setSelectedNode(null);
    }
  }, [selectedNode, updateNode]);

  const handleDeploy = useCallback(async () => {
    if (selectedFlow) {
      await deployCurrentFlow();
    }
  }, [selectedFlow, deployCurrentFlow]);

  const handleUndeploy = useCallback(async () => {
    if (selectedFlow) {
      await undeployCurrentFlow();
    }
  }, [selectedFlow, undeployCurrentFlow]);

  if (loading && flows.length === 0) {
    return (
      <div className="flex h-full w-full items-center justify-center">
        <div className="text-gray-500">Loading flows...</div>
      </div>
    );
  }

  if (error) {
    return (
      <div className="flex h-full w-full items-center justify-center">
        <div className="text-red-500">Error: {error.message}</div>
      </div>
    );
  }

  return (
    <div className="flex h-full w-full overflow-hidden">
      <div className="w-64 bg-white border-r border-gray-200 overflow-y-auto">
        <NodePalette
          nodeTypes={nodeTypes}
          loading={nodeTypesLoading}
          onAddNode={addNode}
        />
      </div>

      <div className="flex-1 flex flex-col overflow-hidden">
        <Toolbar
          flows={flows}
          selectedFlow={selectedFlow}
          onCreateNewFlow={handleCreateNewFlow}
          onSelectFlow={handleSelectFlow}
          onDeploy={handleDeploy}
          onUndeploy={handleUndeploy}
        />

        <div className="flex-1 overflow-hidden">
          <FlowCanvas
            flow={selectedFlow}
            onNodeSelect={handleNodeSelect}
            onNodeDeselect={handleNodeDeselect}
            onAddNode={addNode}
            onRemoveNode={removeNode}
            onAddConnection={addConnection}
            onRemoveConnection={removeConnection}
          />
        </div>
      </div>

      <div className="w-80 bg-white border-l border-gray-200 overflow-y-auto">
        <Sidebar
          flow={selectedFlow}
          selectedNode={selectedNode}
          onConfigureNode={handleConfigureNode}
        />
      </div>

      {showConfigModal && selectedNode && (
        <NodeConfigModal
          node={selectedNode}
          nodeTypes={nodeTypes}
          onClose={handleCloseConfigModal}
          onSave={handleSaveNodeConfig}
        />
      )}
    </div>
  );
}