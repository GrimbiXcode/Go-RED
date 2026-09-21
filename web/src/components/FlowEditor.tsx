import { useCallback, useEffect, useMemo } from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import { ReactFlowProvider } from '@xyflow/react';
import { useTranslation } from 'react-i18next';
import { FlowCanvas } from './FlowCanvas';
import { NodePalette } from './NodePalette';
import { Sidebar } from './Sidebar';
import { Header } from './Header';
import { FlowTabs } from './FlowTabs';
import { NodeEditTray } from './NodeEditTray';
import { DebugPanel } from './DebugPanel';
import { SidebarTabs, InfoTabIcon, DebugTabIcon } from './SidebarTabs';
import { StatusBar } from './StatusBar';
import { ExportModal } from './ExportModal';
import { ImportModal } from './ImportModal';
import { useFlowStore, selectCanDeploy, selectCanRedo, selectCanUndeploy, selectCanUndo, hasUndeployedChanges, type NodePatch } from '../store/flowStore';
import { connectionsOnMissingPorts, outputPortIds } from '../schema/ports';
import { useEditorStore, type SidebarTab } from '../store/editorStore';
import { notify } from '../store/notificationStore';
import { deployFlow } from '../utils/api';
import { NoFlowsState } from './EmptyStates';
import { ShortcutHelp } from './ShortcutHelp';
import { copySelection, duplicateSelection, pasteClipboard } from '../store/editorActions';
import { useEditorShortcuts } from '../hooks/useEditorShortcuts';

function errorMessage(error: unknown): string {
  return error instanceof Error ? error.message : String(error);
}

export function FlowEditor() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const { flowId } = useParams<{ flowId: string }>();

  const flows = useFlowStore((state) => state.flows);
  const flowsLoading = useFlowStore((state) => state.flowsLoading);
  const flowsError = useFlowStore((state) => state.flowsError);
  const flow = useFlowStore((state) => state.flow);
  const flowError = useFlowStore((state) => state.flowError);
  const saveState = useFlowStore((state) => state.saveState);
  const saveError = useFlowStore((state) => state.saveError);
  const canDeploy = useFlowStore(selectCanDeploy);
  const canUndeploy = useFlowStore(selectCanUndeploy);
  const canUndo = useFlowStore(selectCanUndo);
  const canRedo = useFlowStore(selectCanRedo);
  const hasChanges = useFlowStore(hasUndeployedChanges);

  const loadFlows = useFlowStore((state) => state.loadFlows);
  const loadNodeTypes = useFlowStore((state) => state.loadNodeTypes);
  const selectFlow = useFlowStore((state) => state.selectFlow);
  const createFlow = useFlowStore((state) => state.createFlow);
  const deleteFlow = useFlowStore((state) => state.deleteFlow);
  const deploy = useFlowStore((state) => state.deploy);
  const undeploy = useFlowStore((state) => state.undeploy);
  const undo = useFlowStore((state) => state.undo);
  const redo = useFlowStore((state) => state.redo);
  const flushSave = useFlowStore((state) => state.flushSave);
  const updateNode = useFlowStore((state) => state.updateNode);
  const renameFlowById = useFlowStore((state) => state.renameFlowById);
  const duplicateFlow = useFlowStore((state) => state.duplicateFlow);
  const reorderFlows = useFlowStore((state) => state.reorderFlows);
  const nodeTypes = useFlowStore((state) => state.nodeTypes);
  const removeNodes = useFlowStore((state) => state.removeNodes);

  const sidebarTab = useEditorStore((state) => state.sidebarTab);
  const toggleSidebarTab = useEditorStore((state) => state.toggleSidebarTab);
  const configNodeId = useEditorStore((state) => state.configNodeId);
  const closeConfig = useEditorStore((state) => state.closeConfig);
  const showExport = useEditorStore((state) => state.showExport);
  const showImport = useEditorStore((state) => state.showImport);
  const setShowExport = useEditorStore((state) => state.setShowExport);
  const setShowImport = useEditorStore((state) => state.setShowImport);
  const showShortcuts = useEditorStore((state) => state.showShortcuts);
  const setShowShortcuts = useEditorStore((state) => state.setShowShortcuts);
  const resetForFlow = useEditorStore((state) => state.resetForFlow);

  // Initial data.
  useEffect(() => {
    void loadFlows();
    void loadNodeTypes();
  }, [loadFlows, loadNodeTypes]);

  // The URL is the source of truth for which flow is open.
  useEffect(() => {
    resetForFlow();
    void selectFlow(flowId ?? null);
  }, [flowId, selectFlow, resetForFlow]);

  // With no flow in the URL, open the first one (Node-RED opens its first tab).
  useEffect(() => {
    if (!flowId && flows.length > 0) {
      navigate(`/flow/${flows[0].id}`, { replace: true });
    }
  }, [flowId, flows, navigate]);

  // Never lose a pending autosave to a reload or tab close.
  useEffect(() => {
    const onPageHide = () => {
      void flushSave({ keepalive: true });
    };
    const onBeforeUnload = (event: BeforeUnloadEvent) => {
      if (useFlowStore.getState().saveState === 'idle') return;
      void flushSave({ keepalive: true });
      event.preventDefault();
      event.returnValue = '';
    };
    window.addEventListener('pagehide', onPageHide);
    window.addEventListener('beforeunload', onBeforeUnload);
    return () => {
      window.removeEventListener('pagehide', onPageHide);
      window.removeEventListener('beforeunload', onBeforeUnload);
    };
  }, [flushSave]);

  const handleDuplicateFlow = useCallback(
    async (id: string) => {
      try {
        const copy = await duplicateFlow(id);
        notify('success', t('toast.flowDuplicated', { name: copy.name }));
        navigate(`/flow/${copy.id}`);
      } catch (error) {
        notify('error', t('toast.flowDuplicateFailed', { message: errorMessage(error) }));
      }
    },
    [duplicateFlow, navigate, t]
  );

  const handleDeploy = useCallback(async () => {
    try {
      await deploy();
      notify('success', t('toast.deployed'));
    } catch (error) {
      notify('error', t('toast.deployFailed', { message: errorMessage(error) }));
    }
  }, [deploy, t]);

  const shortcutDeploy = useCallback(() => {
    if (canDeploy) void handleDeploy();
  }, [canDeploy, handleDeploy]);
  const shortcutExport = useCallback(() => {
    if (flow) setShowExport(true);
  }, [flow, setShowExport]);
  const shortcutHelp = useCallback(() => setShowShortcuts(true), [setShowShortcuts]);
  const shortcutPaste = useCallback(() => {
    pasteClipboard();
  }, []);
  const shortcutCopy = useCallback(() => {
    copySelection();
  }, []);
  const shortcutDuplicate = useCallback(() => {
    duplicateSelection();
  }, []);

  useEditorShortcuts({
    undo,
    redo,
    deploy: shortcutDeploy,
    exportFlow: shortcutExport,
    help: shortcutHelp,
    copy: shortcutCopy,
    paste: shortcutPaste,
    duplicate: shortcutDuplicate,
  });

  const handleSelectFlow = useCallback(
    (id: string) => {
      if (id !== flowId) navigate(`/flow/${id}`);
    },
    [flowId, navigate]
  );

  const handleCreateFlow = useCallback(async () => {
    try {
      const created = await createFlow(t('tabs.defaultName'));
      navigate(`/flow/${created.id}`);
    } catch (error) {
      notify('error', t('toast.flowCreateFailed', { message: errorMessage(error) }));
    }
  }, [createFlow, navigate, t]);

  const handleDeleteFlow = useCallback(
    async (id: string) => {
      const target = flows.find((f) => f.id === id);
      if (!target) return;
      if (!window.confirm(t('tabs.confirmDelete', { name: target.name }))) return;
      try {
        await deleteFlow(id);
        notify('success', t('toast.flowDeleted'));
        if (id === flowId) navigate('/', { replace: true });
      } catch (error) {
        notify('error', t('toast.flowDeleteFailed', { message: errorMessage(error) }));
      }
    },
    [flows, flowId, deleteFlow, navigate, t]
  );

  const modifiedFlows = useMemo(
    () => flows.filter((f) => f.id !== flowId && f.status === 'running' && !!f.deployedAt && f.updatedAt > f.deployedAt),
    [flows, flowId]
  );

  const handleDeployAll = useCallback(async () => {
    let count = 0;
    try {
      if (canDeploy) {
        await deploy();
        count++;
      }
      for (const target of modifiedFlows) {
        await deployFlow(target.id);
        count++;
      }
      notify('success', t('toast.deployedAll', { count }));
    } catch (error) {
      notify('error', t('toast.deployFailed', { message: errorMessage(error) }));
    }
  }, [canDeploy, deploy, modifiedFlows, t]);

  const handleUndeploy = useCallback(async () => {
    try {
      await undeploy();
      notify('success', t('toast.stopped'));
    } catch (error) {
      notify('error', t('toast.stopFailed', { message: errorMessage(error) }));
    }
  }, [undeploy, t]);

  const handleFlowImported = useCallback(
    (id: string) => {
      notify('success', t('toast.imported'));
      navigate(`/flow/${id}`);
    },
    [navigate, t]
  );

  const configNode = configNodeId && flow ? flow.nodes[configNodeId] : null;

  const handleSaveNodeConfig = useCallback(
    (patch: NodePatch) => {
      if (configNodeId && flow) {
        const metadata = nodeTypes.find((nt) => nt.type === flow.nodes[configNodeId]?.type);
        // Outputs can depend on the config (Switch rules): connections on
        // ports that no longer exist go away in the same undo step.
        const dangling = metadata ? connectionsOnMissingPorts(flow.connections, configNodeId, outputPortIds(metadata, patch.config)) : [];
        updateNode(configNodeId, patch, { removeConnections: dangling.map((c) => c.id) });
        if (dangling.length > 0) notify('info', t('toast.connectionsRemoved', { count: dangling.length }));
      }
      closeConfig();
    },
    [configNodeId, flow, nodeTypes, updateNode, closeConfig, t]
  );

  const handleDeleteConfiguredNode = useCallback(() => {
    if (configNodeId) removeNodes([configNodeId]);
    closeConfig();
  }, [configNodeId, removeNodes, closeConfig]);

  const sidebarTabs = useMemo(
    () => [
      { id: 'info' as SidebarTab, label: t('sidebar.info'), icon: <InfoTabIcon />, content: <Sidebar /> },
      { id: 'debug' as SidebarTab, label: t('sidebar.debug'), icon: <DebugTabIcon />, content: <DebugPanel flowId={flow?.id} /> },
    ],
    [t, flow?.id]
  );

  if (flowsLoading && flows.length === 0) {
    return (
      <div className="flex h-full w-full items-center justify-center">
        <div className="text-muted">{t('app.loadingFlows')}</div>
      </div>
    );
  }

  if (flowsError && flows.length === 0) {
    return (
      <div className="flex h-full w-full flex-col items-center justify-center gap-3">
        <div className="text-danger-text">{t('app.loadError', { message: flowsError })}</div>
        <button className="px-3 py-1.5 text-xs bg-accent text-accent-fg rounded" onClick={() => void loadFlows()}>
          {t('app.retry')}
        </button>
      </div>
    );
  }

  return (
    <div className="flex flex-col h-full w-full overflow-hidden">
      <Header
        hasFlow={!!flow}
        hasChanges={hasChanges}
        canDeploy={canDeploy}
        canUndeploy={canUndeploy}
        canUndo={canUndo}
        canRedo={canRedo}
        modifiedFlows={modifiedFlows.length}
        onDeploy={handleDeploy}
        onDeployAll={handleDeployAll}
        onUndeploy={handleUndeploy}
        onUndo={undo}
        onRedo={redo}
        onExport={() => (flow ? setShowExport(true) : notify('error', t('toast.noFlowToExport')))}
        onImport={() => setShowImport(true)}
      />

      <div className="flex flex-1 overflow-hidden">
        <div className="w-64 bg-panel border-r border-line overflow-y-auto shrink-0">
          <NodePalette />
        </div>

        <div className="flex-1 flex flex-col overflow-hidden">
          <FlowTabs
            flows={flows}
            selectedFlowId={flowId ?? null}
            onSelectFlow={handleSelectFlow}
            onCreateFlow={handleCreateFlow}
            onDeleteFlow={handleDeleteFlow}
            onRenameFlow={(id, name) => void renameFlowById(id, name)}
            onDuplicateFlow={(id) => void handleDuplicateFlow(id)}
            onReorderFlows={(ids) => void reorderFlows(ids)}
          />

          <div className="flex-1 overflow-hidden">
            {flows.length === 0 ? (
              <NoFlowsState onCreate={() => void handleCreateFlow()} onImport={() => setShowImport(true)} />
            ) : flowError ? (
              <div className="flex h-full w-full items-center justify-center bg-canvas">
                <div className="text-danger-text text-sm">{flowError}</div>
              </div>
            ) : (
              <ReactFlowProvider>
                <FlowCanvas flow={flow} />
              </ReactFlowProvider>
            )}
          </div>
        </div>

        <SidebarTabs activeTabId={sidebarTab} onSelectTab={(id) => toggleSidebarTab(id as SidebarTab)} tabs={sidebarTabs} />
      </div>

      <StatusBar flow={flow} saveState={saveState} saveError={saveError} />

      {configNode && (
        <NodeEditTray
          key={configNode.id}
          node={configNode}
          onClose={closeConfig}
          onSave={handleSaveNodeConfig}
          onDelete={handleDeleteConfiguredNode}
        />
      )}

      {flow && <ExportModal flowId={flow.id} flowName={flow.name} isOpen={showExport} onClose={() => setShowExport(false)} />}

      <ImportModal isOpen={showImport} onClose={() => setShowImport(false)} onFlowImported={handleFlowImported} />

      <ShortcutHelp isOpen={showShortcuts} onClose={() => setShowShortcuts(false)} />
    </div>
  );
}
