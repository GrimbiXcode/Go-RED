import { useState, useCallback, useEffect } from 'react';
import { useFlowContext } from './FlowProvider';
import { FlowCanvas } from './FlowCanvas';
import { NodePalette } from './NodePalette';
import { Sidebar } from './Sidebar';
import { Header } from './Header';
import { FlowTabs } from './FlowTabs';
import { NodeConfigModal } from './NodeConfigModal';
import { MessageLogPanel } from './MessageLogPanel';
import { ExportModal } from './ExportModal';
import { ImportModal } from './ImportModal';
import { useToast } from './ToastNotification';
import { ReactFlowProvider } from 'reactflow';
import type { FlowNode, NodeConnection } from '../types/flow';

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
  const [showMessageLog, setShowMessageLog] = useState(false);
  const [showExportModal, setShowExportModal] = useState(false);
  const [showImportModal, setShowImportModal] = useState(false);
  // Local-only "has this flow changed since the last deploy" flag driving
  // the Header's Deploy button (see docs/FRONTEND_NODE_RED_REDESIGN.md
  // Phase 1) — deliberately not persisted, no backend field for this.
  const [isDirty, setIsDirty] = useState(false);

  useEffect(() => {
    setIsDirty(false);
  }, [selectedFlow?.id]);

  const handleCreateNewFlow = useCallback(async () => {
    await createNewFlow('New Flow', 'A new flow');
  }, [createNewFlow]);

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
      setIsDirty(true);
    },
    [removeNode]
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
      showToast('success', 'Flow gelöscht');
    } catch (err) {
      showToast('error', `Flow konnte nicht gelöscht werden: ${err instanceof Error ? err.message : String(err)}`);
    }
  }, [selectedFlow, deleteCurrentFlow, showToast]);

  const handleSelectFlow = useCallback((flowId: string) => {
    selectFlow(flowId);
    setSelectedNode(null);
  }, [selectFlow]);

  const handleNodeSelect = useCallback((node: FlowNode) => {
    // Validate the selected node before setting state
    if (!node) {
      console.error('Cannot select node: node is null or undefined');
      return;
    }
    
    const nodeId = node.id;
    const nodeType = node.type;
    
    if (!nodeId || typeof nodeId !== 'string' || nodeId.trim() === '') {
      console.error('Cannot select node: invalid node ID', { nodeId, node });
      return;
    }
    
    if (!nodeType || typeof nodeType !== 'string' || nodeType.trim() === '') {
      console.error('Cannot select node: invalid node type', { nodeType, node });
      return;
    }
    
    // Ensure node has valid position
    const validatedNode: FlowNode = {
      ...node,
      id: nodeId.trim(),
      type: nodeType.trim(),
      position: node.position || { x: 0, y: 0 },
      config: node.config || {},
    };
    
    setSelectedNode(validatedNode);
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
    if (selectedNode && selectedNode.id && selectedNode.id.trim() !== '') {
      updateNode(selectedNode.id, { config });
      setIsDirty(true);
      setShowConfigModal(false);
      setSelectedNode(null);
    } else {
      console.error('Cannot save node config: invalid selected node');
      setShowConfigModal(false);
      setSelectedNode(null);
    }
  }, [selectedNode, updateNode]);

  const handleSave = useCallback(async () => {
    if (!selectedFlow) {
      showToast('error', 'No flow selected to save');
      return;
    }

    try {
      // Sanitize nodes and connections before saving
      const sanitizedNodes: Record<string, FlowNode> = {};
      Object.entries(selectedFlow.nodes || {}).forEach(([id, node]) => {
        if (node && 
            id && typeof id === 'string' && id.trim() !== '' &&
            node.id && typeof node.id === 'string' && node.id.trim() !== '' &&
            node.type && typeof node.type === 'string' && node.type.trim() !== '') {
          sanitizedNodes[id.trim()] = {
            ...node,
            id: node.id.trim(),
            type: node.type.trim(),
            position: node.position || { x: 0, y: 0 },
            config: node.config || {},
          };
        } else {
          console.warn('Skipping invalid node during save:', { id, node });
        }
      });
      
      const sanitizedConnections = (selectedFlow.connections || []).filter(conn => {
        const isValid = conn &&
          conn.id && typeof conn.id === 'string' && conn.id.trim() !== '' &&
          conn.sourceNode && typeof conn.sourceNode === 'string' && conn.sourceNode.trim() !== '' &&
          conn.targetNode && typeof conn.targetNode === 'string' && conn.targetNode.trim() !== '';
        if (!isValid) {
          console.warn('Skipping invalid connection during save:', { conn });
        }
        return isValid;
      }).map(conn => ({
        ...conn,
        id: conn.id.trim(),
        sourceNode: conn.sourceNode.trim(),
        targetNode: conn.targetNode.trim(),
        sourcePort: (conn.sourcePort || 'output').trim(),
        targetPort: (conn.targetPort || 'input').trim(),
      }));

      await updateCurrentFlow({
        nodes: sanitizedNodes,
        connections: sanitizedConnections,
        config: selectedFlow.config,
      });
      showToast('success', 'Flow saved successfully!');
      // Reset selected node to prevent stale data in sidebar
      setSelectedNode(null);
    } catch (error) {
      showToast('error', `Failed to save flow: ${error instanceof Error ? error.message : String(error)}`);
    }
  }, [selectedFlow, updateCurrentFlow, showToast]);

  const handleToggleMessageLog = useCallback(() => {
    setShowMessageLog((prev) => !prev);
  }, []);

  const handleDeploy = useCallback(async () => {
    if (selectedFlow) {
      await deployCurrentFlow();
      setIsDirty(false);
    }
  }, [selectedFlow, deployCurrentFlow]);

  const handleUndeploy = useCallback(async () => {
    if (selectedFlow) {
      await undeployCurrentFlow();
    }
  }, [selectedFlow, undeployCurrentFlow]);

  // 'deployed' isn't in the generated FlowStatus union (see
  // types/generated.ts) but deployCurrentFlow sets it anyway — checking
  // both here so Undeploy actually becomes clickable after a deploy.
  const canDeploy = !!selectedFlow && (['draft', 'error'].includes(selectedFlow.status) || isDirty);
  const canUndeploy = !!selectedFlow && ['running', 'deployed'].includes(selectedFlow.status);

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

  const handleFlowImported = useCallback((flowId: string) => {
    selectFlow(flowId);
    showToast('success', 'Flow imported and selected');
  }, [selectFlow, showToast]);

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
        onToggleMessageLog={handleToggleMessageLog}
      />

      <div className="flex flex-1 overflow-hidden">
        <div className="w-64 bg-white border-r border-gray-200 overflow-y-auto">
          <NodePalette
            nodeTypes={nodeTypes}
            loading={nodeTypesLoading}
          />
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
                onAddConnection={handleAddConnection}
                onRemoveConnection={handleRemoveConnection}
              />
            </ReactFlowProvider>
          </div>
        </div>

        <div className="w-80 bg-white border-l border-gray-200 overflow-y-auto">
          <Sidebar
            flow={selectedFlow}
            selectedNode={selectedNode}
            onConfigureNode={handleConfigureNode}
          />
        </div>
      </div>

      {showConfigModal && selectedNode && (
        <NodeConfigModal
          node={selectedNode}
          nodeTypes={nodeTypes}
          onClose={handleCloseConfigModal}
          onSave={handleSaveNodeConfig}
        />
      )}
      
      <MessageLogPanel
        selectedFlowId={selectedFlow?.id}
        isOpen={showMessageLog}
        onClose={() => setShowMessageLog(false)}
      />

      {selectedFlow && (
        <ExportModal
          flowId={selectedFlow.id}
          flowName={selectedFlow.name}
          isOpen={showExportModal}
          onClose={handleCloseExportModal}
        />
      )}

      <ImportModal
        isOpen={showImportModal}
        onClose={handleCloseImportModal}
        onFlowImported={handleFlowImported}
      />
    </div>
  );
}