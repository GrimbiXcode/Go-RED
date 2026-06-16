import { useState, useEffect, useCallback } from 'react';
import type { Flow, FlowNode, NodeConnection, FlowStatus, FlowConfig } from '../types/flow';
import type { NodeMetadata } from '../types/node';
import type { WebSocketMessageType } from '../types/message';
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

export interface FlowState {
  flows: Flow[];
  loading: boolean;
  error: Error | null;
  selectedFlowId: string | null;
  selectedFlow: Flow | null;
  nodeTypes: NodeMetadata[];
  nodeTypesLoading: boolean;
  nodeTypesError: Error | null;
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
  });
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
    const newFlow = await createFlow({
      name,
      description,
      nodes: {},
      connections: [],
      config: {},
    });
    await loadFlows();
    return newFlow;
  }, [loadFlows]);
  const updateCurrentFlow = useCallback(async (updates: Partial<Flow>) => {
    if (!state.selectedFlowId) {
      throw new Error('No flow selected');
    }
    const updatedFlow = await updateFlow(state.selectedFlowId, updates);
    setState((prev) => ({
      ...prev,
      selectedFlow: updatedFlow,
      flows: prev.flows.map((flow) =>
        flow.id === updatedFlow.id ? updatedFlow : flow
      ),
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
    if (!state.selectedFlowId) {
      throw new Error('No flow selected');
    }
    await deployFlow(state.selectedFlowId, force);
    setState((prev) => ({
      ...prev,
      selectedFlow: prev.selectedFlow ? {
        ...prev.selectedFlow,
        status: 'deployed' as FlowStatus,
      } : null,
      flows: prev.flows.map((flow) =>
        flow.id === state.selectedFlowId
          ? { ...flow, status: 'deployed' as FlowStatus }
          : flow
      ),
    }));
  }, [state.selectedFlowId, state.flows]);
  const undeployCurrentFlow = useCallback(async () => {
    if (!state.selectedFlowId) {
      throw new Error('No flow selected');
    }
    await undeployFlow(state.selectedFlowId);
    setState((prev) => ({
      ...prev,
      selectedFlow: prev.selectedFlow ? {
        ...prev.selectedFlow,
        status: 'stopped' as FlowStatus,
      } : null,
      flows: prev.flows.map((flow) =>
        flow.id === state.selectedFlowId
          ? { ...flow, status: 'stopped' as FlowStatus }
          : flow
      ),
    }));
  }, [state.selectedFlowId, state.flows]);
  const selectFlow = useCallback((flowId: string) => {
    const flow = state.flows.find((f) => f.id === flowId);
    setState((prev) => ({
      ...prev,
      selectedFlowId: flowId,
      selectedFlow: flow || null,
    }));
  }, [state.flows]);
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
    };
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
    setState((prev) => ({
      ...prev,
      selectedFlow: prev.selectedFlow ? {
        ...prev.selectedFlow,
        connections: [...prev.selectedFlow.connections, newConnection],
      } : null,
    }));
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
          selectedFlow: prev.selectedFlow?.id === data.flowId
            ? { ...prev.selectedFlow, status: data.status }
            : prev.selectedFlow,
        }));
      }
    });
    const nodeUpdateSub = ws.subscribe('node:status', (data) => {
      if (data.nodeId && data.flowId) {
        setState((prev) => ({
          ...prev,
          selectedFlow: prev.selectedFlow?.id === data.flowId
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
    const errorSub = ws.subscribe('error', (data) => {
      console.error('WebSocket error:', data);
      setState((prev) => ({ ...prev, error: new Error(data.error || 'Unknown error') }));
    });
    return () => {
      flowUpdateSub();
      nodeUpdateSub();
      flowListSub();
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