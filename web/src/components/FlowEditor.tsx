import { useState, useCallback, useEffect } from 'react';
import { useFlowContext } from './FlowProvider';
import { FlowCanvas } from './FlowCanvas';
import { NodePalette } from './NodePalette';
import { Sidebar } from './Sidebar';
import { Header } from './Header';
import { FlowTabs } from './FlowTabs';
import { NodeConfigModal } from './NodeConfigModal';
import { MessageLogPanel } from './MessageLogPanel';
import { SidebarTabs, InfoTabIcon, DebugTabIcon } from './SidebarTabs';
import { StatusBar } from './StatusBar';
import { ExportModal } from './ExportModal';
import { ImportModal } from './ImportModal';
import { useToast } from './ToastNotification';
import { ReactFlowProvider } from 'reactflow';
import type { FlowNode, NodeConnection } from '../types/flow';

function errorMessage(error: unknown): string {
  return error instanceof Error ? error.message : String(error);
}

export function FlowEditor() {
  const {
    flows,
    loading,
    error,
    serverError,
    selectedFlow,
    nodeTypes,
    nodeTypesLoading,
    createNewFlow,
    selectFlow,
    addNode,
    removeNode,
    updateNode,
    updateCurrentFlow,
    addConnection,
    removeConnection,
    deployCurrentFlow,
    undeployCurrentFlow,
    deleteCurrentFlow,
  } = useFlowContext();

  const { showToast } = useToast();

  const [selectedNode, setSelectedNode] = useState<FlowNode | null>(null);
  const [showConfigModal, setShowConfigModal] = useState(false);
  // Which right-hand sidebar tab is open ('info' | 'debug' | null for collapsed).
  const [activeSidebarTab, setActiveSidebarTab] = useState<string | null>('info');
  const [showExportModal, setShowExportModal] = useState(false);
  const [showImportModal, setShowImportModal] = useState(false);
  // Local-only "has this flow changed since the last deploy" flag driving
  // the Header's Deploy button. Deliberately not persisted.
  const [isDirty, setIsDirty] = useState(false);

  useEffect(() => {
    setIsDirty(false);
  }, [selectedFlow?.id]);

  // Errors the server pushes over the WebSocket (e.g. an inject into a
  // flow that is not running) are transient: show them, don't replace
  // the whole editor with an error screen.
  useEffect(() => {
    if (serverError) {
      showToast('error', serverError.message);
    }
  }, [serverError, showToast]);

  const handleCreateNewFlow = useCallback(async () => {
    try {
      await createNewFlow('New Flow', 'A new flow');
    } catch (err) {
      showToast('error', `Flow konnte nicht angelegt werden: ${errorMessage(err)}`);
    }
  }, [createNewFlow, showToast]);

  const handleAddNode = useCallback(
    (nodeType: string, position: { x: number; y: number }) => {
      addNode(nodeType, position);
      setIsDirty(true);
    },
    [addNode]
  );

  const handleRemoveNode = useCallback(
    (nodeId: string) => {
      removeNode(nodeId);
      setSelectedNode((current) => (current && current.id === nodeId ? null : current));
      setIsDirty(true);
    },
    [removeNode]
  );

  const handleNodeMove = useCallback(
    (nodeId: string, position: { x: number; y: number }) => {
      updateNode(nodeId, { position });
      setSelectedNode((current) => (current && current.id === nodeId ? { ...current, position } : current));
      setIsDirty(true);
    },
    [updateNode]
  );

  const handleAddConnection = useCallback(
    (connection: Omit<NodeConnection, 'id'>) => {
      addConnection(connection);
      setIsDirty(true);
    },
    [addConnection]
  );

  const handleRemoveConnection = useCallback(
    (connectionId: string) => {
      removeConnection(connectionId);
      setIsDirty(true);
    },
    [removeConnection]
  );

  const handleDeleteSelectedFlow = useCallback(async () => {
    if (!selectedFlow) return;
    if (!window.confirm(`Flow "${selectedFlow.name}" wirklich löschen?`)) return;
    try {
      await deleteCurrentFlow();
      setSelectedNode(null);
      showToast('success', 'Flow gelöscht');
    } catch (err) {
      showToast('error', `Flow konnte nicht gelöscht werden: ${errorMessage(err)}`);
    }
  }, [selectedFlow, deleteCurrentFlow, showToast]);

  const handleSelectFlow = useCallback(
    (flowId: string) => {
      selectFlow(flowId);
      setSelectedNode(null);
    },
    [selectFlow]
  );

  const handleNodeSelect = useCallback((node: FlowNode) => {
    if (!node || !node.id || !node.type) return;
    setSelectedNode({
      ...node,
      position: node.position || { x: 0, y: 0 },
      config: node.config || {},
    });
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

  const handleSaveNodeConfig = useCallback(
    (config: Record<string, any>) => {
      if (selectedNode) {
        updateNode(selectedNode.id, { config });
        setIsDirty(true);
      }
      setShowConfigModal(false);
      setSelectedNode(null);
    },
    [selectedNode, updateNode]
  );

  const handleDeleteConfiguredNode = useCallback(() => {
    if (!selectedNode) return;
    handleRemoveNode(selectedNode.id);
    setShowConfigModal(false);
    setSelectedNode(null);
  }, [selectedNode, handleRemoveNode]);

  const handleSave = useCallback(async () => {
    if (!selectedFlow) {
      showToast('error', 'No flow selected to save');
      return;
    }
    try {
      await updateCurrentFlow({
        nodes: selectedFlow.nodes,
        connections: selectedFlow.connections,
        config: selectedFlow.config,
      });
      showToast('success', 'Flow saved');
      setSelectedNode(null);
    } catch (err) {
      showToast('error', `Failed to save flow: ${errorMessage(err)}`);
    }
  }, [selectedFlow, updateCurrentFlow, showToast]);

  const handleDeploy = useCallback(async () => {
    if (!selectedFlow) return;
    try {
      await deployCurrentFlow();
      setIsDirty(false);
      showToast('success', 'Flow deployed');
    } catch (err) {
      showToast('error', `Deploy failed: ${errorMessage(err)}`, 6000);
    }
  }, [selectedFlow, deployCurrentFlow, showToast]);

  const handleUndeploy = useCallback(async () => {
    if (!selectedFlow) return;
    try {
      await undeployCurrentFlow();
      showToast('success', 'Flow stopped');
    } catch (err) {
      showToast('error', `Stop failed: ${errorMessage(err)}`);
    }
  }, [selectedFlow, undeployCurrentFlow, showToast]);

  const isRunning = selectedFlow?.status === 'running';
  const canDeploy = !!selectedFlow && (!isRunning || isDirty);
  const canUndeploy = !!selectedFlow && isRunning;

  const handleExportFlow = useCallback(() => {
    if (selectedFlow) {
      setShowExportModal(true);
    } else {
      showToast('error', 'No flow selected to export');
    }
  }, [selectedFlow, showToast]);

  const handleImportFlow = useCallback(() => {
    setShowImportModal(true);
  }, []);

  const handleFlowImported = useCallback(
    (flowId: string) => {
      selectFlow(flowId);
      showToast('success', 'Flow imported and selected');
    },
    [selectFlow, showToast]
  );

  const handleCloseExportModal = useCallback(() => {
    setShowExportModal(false);
  }, []);

  const handleCloseImportModal = useCallback(() => {
    setShowImportModal(false);
  }, []);

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
    <div className="flex flex-col h-full w-full overflow-hidden">
      <Header
        hasSelectedFlow={!!selectedFlow}
        isDirty={isDirty}
        canDeploy={canDeploy}
        canUndeploy={canUndeploy}
        onDeploy={handleDeploy}
        onUndeploy={handleUndeploy}
        onSave={handleSave}
        onExport={handleExportFlow}
        onImport={handleImportFlow}
      />

      <div className="flex flex-1 overflow-hidden">
        <div className="w-64 bg-white border-r border-gray-200 overflow-y-auto">
          <NodePalette nodeTypes={nodeTypes} loading={nodeTypesLoading} />
        </div>

        <div className="flex-1 flex flex-col overflow-hidden">
          <FlowTabs
            flows={flows}
            selectedFlowId={selectedFlow?.id ?? null}
            onSelectFlow={handleSelectFlow}
            onCreateNewFlow={handleCreateNewFlow}
            onDeleteSelectedFlow={handleDeleteSelectedFlow}
          />

          <div className="flex-1 overflow-hidden">
            <ReactFlowProvider>
              <FlowCanvas
                flow={selectedFlow}
                availableNodeTypes={nodeTypes}
                onNodeSelect={handleNodeSelect}
                onNodeDeselect={handleNodeDeselect}
                onAddNode={handleAddNode}
                onRemoveNode={handleRemoveNode}
                onNodeMove={handleNodeMove}
                onAddConnection={handleAddConnection}
                onRemoveConnection={handleRemoveConnection}
              />
            </ReactFlowProvider>
          </div>
        </div>

        <SidebarTabs
          activeTabId={activeSidebarTab}
          onSelectTab={setActiveSidebarTab}
          tabs={[
            {
              id: 'info',
              label: 'Info',
              icon: <InfoTabIcon />,
              content: <Sidebar flow={selectedFlow} selectedNode={selectedNode} onConfigureNode={handleConfigureNode} />,
            },
            {
              id: 'debug',
              label: 'Debug',
              icon: <DebugTabIcon />,
              content: <MessageLogPanel selectedFlowId={selectedFlow?.id} />,
            },
          ]}
        />
      </div>

      <StatusBar flow={selectedFlow} />

      {showConfigModal && selectedNode && (
        <NodeConfigModal
          node={selectedNode}
          nodeTypes={nodeTypes}
          onClose={handleCloseConfigModal}
          onSave={handleSaveNodeConfig}
          onDelete={handleDeleteConfiguredNode}
        />
      )}

      {selectedFlow && (
        <ExportModal
          flowId={selectedFlow.id}
          flowName={selectedFlow.name}
          isOpen={showExportModal}
          onClose={handleCloseExportModal}
        />
      )}

      <ImportModal isOpen={showImportModal} onClose={handleCloseImportModal} onFlowImported={handleFlowImported} />
    </div>
  );
}
