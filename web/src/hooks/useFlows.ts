import { useState, useEffect, useCallback, useRef } from 'react';
import type { Flow, FlowNode, NodeConnection, FlowStatus, FlowConfig } from '../types/flow';
import type { NodeMetadata } from '../types/node';
import { useWebSocket } from './useWebSocket';
import {
  fetchFlows,
  fetchFlow,
  createFlow,
  updateFlow,
  deleteFlow,
  deployFlow,
  undeployFlow,
  getNodes,
  generateId,
} from '../utils/api';

import type { FlowSummary } from '../types/api';

export interface FlowState {
  flows: FlowSummary[];
  loading: boolean;
  error: Error | null;
  selectedFlowId: string | null;
  selectedFlow: Flow | null;
  nodeTypes: NodeMetadata[];
  nodeTypesLoading: boolean;
  nodeTypesError: Error | null;
  // Set once a position update sent via updateNode() is confirmed
  // persisted by the backend's node:update broadcast - lets the UI show a
  // brief "position saved" indicator instead of leaving drag-and-drop
  // saves completely silent.
  positionSavedAt: { nodeId: string; at: number } | null;
}

export interface FlowActions {
  loadFlows: () => Promise<void>;
  loadFlow: (flowId: string) => Promise<void>;
  createNewFlow: (name: string, description?: string) => Promise<Flow>;
  updateCurrentFlow: (updates: Partial<Flow>) => Promise<void>;
  deleteCurrentFlow: () => Promise<void>;
  deployCurrentFlow: (force?: boolean) => Promise<void>;
  undeployCurrentFlow: () => Promise<void>;
  selectFlow: (flowId: string) => void;
  deselectFlow: () => void;
  addNode: (nodeType: string, position: { x: number; y: number }) => void;
  removeNode: (nodeId: string) => void;
  updateNode: (nodeId: string, updates: Partial<FlowNode>) => void;
  addConnection: (connection: Omit<NodeConnection, 'id'>) => void;
  removeConnection: (connectionId: string) => void;
  setFlowConfig: (config: FlowConfig) => void;
  resetFlow: () => void;
}

export interface UseFlowsReturn extends FlowState, FlowActions {}

export function useFlows(): UseFlowsReturn {
  const ws = useWebSocket();
  const [state, setState] = useState<FlowState>({
    flows: [],
    loading: true,
    error: null,
    selectedFlowId: null,
    selectedFlow: null,
    nodeTypes: [],
    nodeTypesLoading: true,
    nodeTypesError: null,
    positionSavedAt: null,
  });
  // Node IDs whose most recent update was position-only, awaiting the
  // node:update broadcast that confirms the backend actually persisted it.
  const pendingPositionSaves = useRef<Set<string>>(new Set());
  const loadFlows = useCallback(async () => {
    try {
      setState((prev) => ({ ...prev, loading: true, error: null }));
      const flows = await fetchFlows();
      setState((prev) => ({ ...prev, flows, loading: false }));
    } catch (error) {
      setState((prev) => ({
        ...prev,
        loading: false,
        error: error as Error,
      }));
    }
  }, []);
  const loadFlow = useCallback(async (flowId: string) => {
    try {
      setState((prev) => ({ ...prev, loading: true, error: null }));
      const flow = await fetchFlow(flowId);
      setState((prev) => ({
        ...prev,
        selectedFlowId: flowId,
        selectedFlow: flow,
        loading: false,
      }));
    } catch (error) {
      setState((prev) => ({
        ...prev,
        loading: false,
        error: error as Error,
      }));
    }
  }, []);
  const createNewFlow = useCallback(async (name: string, description?: string) => {
    const newFlow = await createFlow({ name, description });
    await loadFlows();
    return newFlow;
  }, [loadFlows]);
  const updateCurrentFlow = useCallback(async (updates: Partial<Flow>) => {
    console.log('[FRONTEND] updateCurrentFlow - Starting update with:', updates);
    if (!state.selectedFlowId) {
      console.error('[FRONTEND] updateCurrentFlow - No flow selected');
      throw new Error('No flow selected');
    }
    console.log('[FRONTEND] updateCurrentFlow - Sending to backend:', { flowId: state.selectedFlowId, updates });
    const updatedFlow = await updateFlow(state.selectedFlowId, updates);
    console.log('[FRONTEND] updateCurrentFlow - Received updated flow:', updatedFlow);
    setState((prev) => ({
      ...prev,
      selectedFlow: updatedFlow,
    }));
  }, [state.selectedFlowId]);
  const deleteCurrentFlow = useCallback(async () => {
    if (!state.selectedFlowId) {
      throw new Error('No flow selected');
    }
    await deleteFlow(state.selectedFlowId);
    setState((prev) => ({
      ...prev,
      flows: prev.flows.filter((flow) => flow.id !== state.selectedFlowId),
      selectedFlowId: null,
      selectedFlow: null,
    }));
  }, [state.selectedFlowId, state.flows]);
  const deployCurrentFlow = useCallback(async (force = false) => {
    console.log('[FRONTEND] deployCurrentFlow - Starting deploy:', { flowId: state.selectedFlowId, force });
    if (!state.selectedFlowId) {
      console.error('[FRONTEND] deployCurrentFlow - No flow selected');
      throw new Error('No flow selected');
    }
    console.log('[FRONTEND] deployCurrentFlow - Sending to backend:', { flowId: state.selectedFlowId, force });
    const response = await deployFlow(state.selectedFlowId, force);
    console.log('[FRONTEND] deployCurrentFlow - Deploy successful:', response);
    // Trust the backend's reported status rather than assuming success
    // means "running" - deploy previously always looked successful in the
    // UI even when it silently no-op'd server-side, which is exactly what
    // made that bug invisible.
    const status = response.status as FlowStatus;
    setState((prev) => ({
      ...prev,
      selectedFlow: prev.selectedFlow ? {
        ...prev.selectedFlow,
        status,
      } : null,
      flows: prev.flows.map((flow) =>
        flow.id === state.selectedFlowId
          ? { ...flow, status }
          : flow
      ),
    }));
  }, [state.selectedFlowId, state.flows]);
  const undeployCurrentFlow = useCallback(async () => {
    console.log('[FRONTEND] undeployCurrentFlow - Starting undeploy:', { flowId: state.selectedFlowId });
    if (!state.selectedFlowId) {
      console.error('[FRONTEND] undeployCurrentFlow - No flow selected');
      throw new Error('No flow selected');
    }
    console.log('[FRONTEND] undeployCurrentFlow - Sending to backend:', { flowId: state.selectedFlowId });
    const response = await undeployFlow(state.selectedFlowId);
    console.log('[FRONTEND] undeployCurrentFlow - Undeploy successful:', response);
    const status = response.status as FlowStatus;
    setState((prev) => ({
      ...prev,
      selectedFlow: prev.selectedFlow ? {
        ...prev.selectedFlow,
        status,
      } : null,
      flows: prev.flows.map((flow) =>
        flow.id === state.selectedFlowId
          ? { ...flow, status }
          : flow
      ),
    }));
  }, [state.selectedFlowId, state.flows]);
  const selectFlow = useCallback(async (flowId: string) => {
    try {
      const flow = await fetchFlow(flowId);
      setState((prev) => ({
        ...prev,
        selectedFlowId: flowId,
        selectedFlow: flow,
      }));
    } catch (error) {
      console.error('Failed to load flow:', error);
    }
  }, []);
  const deselectFlow = useCallback(() => {
    setState((prev) => ({
      ...prev,
      selectedFlowId: null,
      selectedFlow: null,
    }));
  }, []);
  const loadNodeTypes = useCallback(async () => {
    try {
      setState((prev) => ({ ...prev, nodeTypesLoading: true, nodeTypesError: null }));
      const nodeTypes = await getNodes();
      setState((prev) => ({ ...prev, nodeTypes, nodeTypesLoading: false }));
    } catch (error) {
      setState((prev) => ({
        ...prev,
        nodeTypesLoading: false,
        nodeTypesError: error as Error,
      }));
    }
  }, []);
  const addNode = useCallback((nodeType: string, position: { x: number; y: number }) => {
    if (!state.selectedFlow) {
      throw new Error('No flow selected');
    }
    const newNode: FlowNode = {
      id: generateId(),
      type: nodeType,
      position,
      config: {},
      status: { state: 'idle' },
      disabled: false,
    };
    console.log('[FRONTEND] addNode - Adding node to local state:', {
      nodeId: newNode.id,
      nodeType: newNode.type,
      flowId: state.selectedFlow.id
    });
    setState((prev) => ({
      ...prev,
      selectedFlow: prev.selectedFlow ? {
        ...prev.selectedFlow,
        nodes: {
          ...prev.selectedFlow.nodes,
          [newNode.id]: newNode,
        },
      } : null,
    }));
    console.log('[FRONTEND] addNode - Sending to backend:', {
      nodeId: newNode.id,
      flowId: state.selectedFlow.id
    });
    ws.sendMessage('node:add', {
      node: newNode,
      flowId: state.selectedFlow.id,
    });
  }, [state.selectedFlow, ws]);
  const removeNode = useCallback((nodeId: string) => {
    if (!state.selectedFlow) {
      throw new Error('No flow selected');
    }
    const { [nodeId]: _, ...remainingNodes } = state.selectedFlow.nodes;
    const remainingConnections = state.selectedFlow.connections.filter(
      (conn) => conn.sourceNode !== nodeId && conn.targetNode !== nodeId
    );
    setState((prev) => ({
      ...prev,
      selectedFlow: prev.selectedFlow ? {
        ...prev.selectedFlow,
        nodes: remainingNodes,
        connections: remainingConnections,
      } : null,
    }));
    ws.sendMessage('node:remove', {
      nodeId,
      flowId: state.selectedFlow.id,
    });
  }, [state.selectedFlow, ws]);
  const updateNode = useCallback((nodeId: string, updates: Partial<FlowNode>) => {
    if (!state.selectedFlow || !state.selectedFlow.nodes[nodeId]) {
      throw new Error('Node not found');
    }
    const isPositionOnly = Object.keys(updates).length === 1 && 'position' in updates;
    if (isPositionOnly) {
      pendingPositionSaves.current.add(nodeId);
    }
    setState((prev) => ({
      ...prev,
      selectedFlow: prev.selectedFlow ? {
        ...prev.selectedFlow,
        nodes: {
          ...prev.selectedFlow.nodes,
          [nodeId]: {
            ...prev.selectedFlow.nodes[nodeId],
            ...updates,
          },
        },
      } : null,
    }));
    ws.sendMessage('node:update', {
      node: {
        ...state.selectedFlow.nodes[nodeId],
        ...updates,
      },
      flowId: state.selectedFlow.id,
    });
  }, [state.selectedFlow, ws]);
  const addConnection = useCallback((connection: Omit<NodeConnection, 'id'>) => {
    if (!state.selectedFlow) {
      throw new Error('No flow selected');
    }
    const newConnection: NodeConnection = {
      ...connection,
      id: generateId(),
    };
    console.log('[FRONTEND] addConnection - Attempting to add connection:', {
      connectionId: newConnection.id,
      sourceNode: newConnection.sourceNode,
      targetNode: newConnection.targetNode,
      flowId: state.selectedFlow.id,
      availableNodes: Object.keys(state.selectedFlow.nodes || {})
    });
    setState((prev) => ({
      ...prev,
      selectedFlow: prev.selectedFlow ? {
        ...prev.selectedFlow,
        connections: [...prev.selectedFlow.connections, newConnection],
      } : null,
    }));
    console.log('[FRONTEND] addConnection - Sending to backend:', {
      connection: newConnection,
      flowId: state.selectedFlow.id
    });
    ws.sendMessage('connection:add', {
      connection: newConnection,
      flowId: state.selectedFlow.id,
    });
  }, [state.selectedFlow, ws]);
  const removeConnection = useCallback((connectionId: string) => {
    if (!state.selectedFlow) {
      throw new Error('No flow selected');
    }
    setState((prev) => ({
      ...prev,
      selectedFlow: prev.selectedFlow ? {
        ...prev.selectedFlow,
        connections: prev.selectedFlow.connections.filter(
          (conn) => conn.id !== connectionId
        ),
      } : null,
    }));
    ws.sendMessage('connection:remove', {
      connectionId,
      flowId: state.selectedFlow.id,
    });
  }, [state.selectedFlow, ws]);
  const setFlowConfig = useCallback((config: FlowConfig) => {
    if (!state.selectedFlow) {
      throw new Error('No flow selected');
    }
    setState((prev) => ({
      ...prev,
      selectedFlow: prev.selectedFlow ? {
        ...prev.selectedFlow,
        config,
      } : null,
    }));
  }, [state.selectedFlow]);
  const resetFlow = useCallback(() => {
    if (!state.selectedFlow) {
      throw new Error('No flow selected');
    }
    setState((prev) => ({
      ...prev,
      selectedFlow: prev.selectedFlow ? {
        ...prev.selectedFlow,
        nodes: {},
        connections: [],
      } : null,
    }));
  }, [state.selectedFlow]);
  useEffect(() => {
    loadFlows();
    loadNodeTypes();
  }, [loadFlows, loadNodeTypes]);
  useEffect(() => {
    const flowUpdateSub = ws.subscribe('flow:status', (data) => {
      if (data.flowId) {
        setState((prev) => ({
          ...prev,
          flows: prev.flows.map((flow) =>
            flow.id === data.flowId ? { ...flow, status: data.status } : flow
          ),
          selectedFlow: prev.selectedFlow?.id === data.flowId && prev.selectedFlow
            ? { ...prev.selectedFlow, status: data.status }
            : prev.selectedFlow,
        }));
      }
    });
    const nodeUpdateSub = ws.subscribe('node:status', (data) => {
      if (data.nodeId && data.flowId) {
        setState((prev) => ({
          ...prev,
          selectedFlow: prev.selectedFlow?.id === data.flowId && prev.selectedFlow
            ? {
                ...prev.selectedFlow,
                nodes: {
                  ...prev.selectedFlow.nodes,
                  [data.nodeId]: {
                    ...prev.selectedFlow.nodes[data.nodeId],
                    status: data.status,
                  },
                },
              }
            : prev.selectedFlow,
        }));
      }
    });
    const flowListSub = ws.subscribe('flow:list', (data) => {
      if (data.flows) {
        setState((prev) => ({ ...prev, flows: data.flows }));
      }
    });
    // Confirms a node:update the client itself sent was actually persisted
    // (the backend broadcasts this to every client, including the sender,
    // after SaveFlow succeeds). Only used here to surface a "position
    // saved" indicator for drag-and-drop moves - other node:update causes
    // (config edits) already have their own save feedback.
    const nodePersistedSub = ws.subscribe('node:update', (data) => {
      if (data.nodeId && pendingPositionSaves.current.has(data.nodeId)) {
        pendingPositionSaves.current.delete(data.nodeId);
        setState((prev) => ({
          ...prev,
          positionSavedAt: { nodeId: data.nodeId, at: Date.now() },
        }));
      }
    });
    const errorSub = ws.subscribe('error', (data) => {
      console.error('WebSocket error:', data);
      setState((prev) => ({ ...prev, error: new Error(data.error || 'Unknown error') }));
    });
    return () => {
      flowUpdateSub();
      nodeUpdateSub();
      flowListSub();
      nodePersistedSub();
      errorSub();
    };
  }, [ws]);
  return {
    ...state,
    loadFlows,
    loadFlow,
    createNewFlow,
    updateCurrentFlow,
    deleteCurrentFlow,
    deployCurrentFlow,
    undeployCurrentFlow,
    selectFlow,
    deselectFlow,
    addNode,
    removeNode,
    updateNode,
    addConnection,
    removeConnection,
    setFlowConfig,
    resetFlow,
  };
}

export default useFlows;