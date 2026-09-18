import { useState, useEffect, useCallback } from 'react';
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

/** A transient error reported by the server over the WebSocket. */
export interface ServerError {
  message: string;
  at: number;
}

export interface FlowState {
  flows: FlowSummary[];
  loading: boolean;
  /** Fatal error while loading the flow list; the editor cannot work without it. */
  error: Error | null;
  /** Last error the server pushed over the WebSocket (shown as a toast). */
  serverError: ServerError | null;
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
  deployCurrentFlow: () => Promise<void>;
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
  const { subscribe, sendMessage } = useWebSocket();
  const [state, setState] = useState<FlowState>({
    flows: [],
    loading: true,
    error: null,
    serverError: null,
    selectedFlowId: null,
    selectedFlow: null,
    nodeTypes: [],
    nodeTypesLoading: true,
    nodeTypesError: null,
  });

  const applyStatus = useCallback((flowId: string, status: FlowStatus) => {
    setState((prev) => ({
      ...prev,
      flows: prev.flows.map((flow) => (flow.id === flowId ? { ...flow, status } : flow)),
      selectedFlow:
        prev.selectedFlow && prev.selectedFlow.id === flowId ? { ...prev.selectedFlow, status } : prev.selectedFlow,
    }));
  }, []);

  const loadFlows = useCallback(async () => {
    try {
      setState((prev) => ({ ...prev, loading: true, error: null }));
      const flows = await fetchFlows();
      setState((prev) => ({ ...prev, flows, loading: false }));
    } catch (error) {
      setState((prev) => ({ ...prev, loading: false, error: error as Error }));
    }
  }, []);

  const loadFlow = useCallback(async (flowId: string) => {
    try {
      setState((prev) => ({ ...prev, loading: true, error: null }));
      const flow = await fetchFlow(flowId);
      setState((prev) => ({ ...prev, selectedFlowId: flowId, selectedFlow: flow, loading: false }));
    } catch (error) {
      setState((prev) => ({ ...prev, loading: false, error: error as Error }));
    }
  }, []);

  const createNewFlow = useCallback(
    async (name: string, description?: string) => {
      const newFlow = await createFlow({ name, description });
      await loadFlows();
      return newFlow;
    },
    [loadFlows]
  );

  const updateCurrentFlow = useCallback(
    async (updates: Partial<Flow>) => {
      if (!state.selectedFlowId) {
        throw new Error('No flow selected');
      }
      const updatedFlow = await updateFlow(state.selectedFlowId, updates);
      setState((prev) => ({ ...prev, selectedFlow: updatedFlow }));
    },
    [state.selectedFlowId]
  );

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
  }, [state.selectedFlowId]);

  const deployCurrentFlow = useCallback(async () => {
    if (!state.selectedFlowId) {
      throw new Error('No flow selected');
    }
    const response = await deployFlow(state.selectedFlowId);
    applyStatus(state.selectedFlowId, response.status);
  }, [state.selectedFlowId, applyStatus]);

  const undeployCurrentFlow = useCallback(async () => {
    if (!state.selectedFlowId) {
      throw new Error('No flow selected');
    }
    const response = await undeployFlow(state.selectedFlowId);
    applyStatus(state.selectedFlowId, response.status);
  }, [state.selectedFlowId, applyStatus]);

  const selectFlow = useCallback(async (flowId: string) => {
    try {
      const flow = await fetchFlow(flowId);
      setState((prev) => ({ ...prev, selectedFlowId: flowId, selectedFlow: flow }));
    } catch (error) {
      console.error('Failed to load flow:', error);
    }
  }, []);

  const deselectFlow = useCallback(() => {
    setState((prev) => ({ ...prev, selectedFlowId: null, selectedFlow: null }));
  }, []);

  const loadNodeTypes = useCallback(async () => {
    try {
      setState((prev) => ({ ...prev, nodeTypesLoading: true, nodeTypesError: null }));
      const nodeTypes = await getNodes();
      setState((prev) => ({ ...prev, nodeTypes, nodeTypesLoading: false }));
    } catch (error) {
      setState((prev) => ({ ...prev, nodeTypesLoading: false, nodeTypesError: error as Error }));
    }
  }, []);

  const addNode = useCallback(
    (nodeType: string, position: { x: number; y: number }) => {
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
      setState((prev) => ({
        ...prev,
        selectedFlow: prev.selectedFlow
          ? { ...prev.selectedFlow, nodes: { ...prev.selectedFlow.nodes, [newNode.id]: newNode } }
          : null,
      }));
      sendMessage('node:add', { node: newNode, flowId: state.selectedFlow.id });
    },
    [state.selectedFlow, sendMessage]
  );

  const removeNode = useCallback(
    (nodeId: string) => {
      if (!state.selectedFlow) {
        throw new Error('No flow selected');
      }
      const { [nodeId]: _removed, ...remainingNodes } = state.selectedFlow.nodes;
      const remainingConnections = state.selectedFlow.connections.filter(
        (conn) => conn.sourceNode !== nodeId && conn.targetNode !== nodeId
      );
      setState((prev) => ({
        ...prev,
        selectedFlow: prev.selectedFlow
          ? { ...prev.selectedFlow, nodes: remainingNodes, connections: remainingConnections }
          : null,
      }));
      sendMessage('node:remove', { nodeId, flowId: state.selectedFlow.id });
    },
    [state.selectedFlow, sendMessage]
  );

  const updateNode = useCallback(
    (nodeId: string, updates: Partial<FlowNode>) => {
      if (!state.selectedFlow || !state.selectedFlow.nodes[nodeId]) {
        throw new Error('Node not found');
      }
      const merged: FlowNode = { ...state.selectedFlow.nodes[nodeId], ...updates };
      setState((prev) => ({
        ...prev,
        selectedFlow: prev.selectedFlow
          ? {
              ...prev.selectedFlow,
              nodes: {
                ...prev.selectedFlow.nodes,
                [nodeId]: { ...prev.selectedFlow.nodes[nodeId], ...updates },
              },
            }
          : null,
      }));
      sendMessage('node:update', { node: merged, flowId: state.selectedFlow.id });
    },
    [state.selectedFlow, sendMessage]
  );

  const addConnection = useCallback(
    (connection: Omit<NodeConnection, 'id'>) => {
      if (!state.selectedFlow) {
        throw new Error('No flow selected');
      }
      const newConnection: NodeConnection = { ...connection, id: generateId() };
      setState((prev) => ({
        ...prev,
        selectedFlow: prev.selectedFlow
          ? { ...prev.selectedFlow, connections: [...prev.selectedFlow.connections, newConnection] }
          : null,
      }));
      sendMessage('connection:add', { connection: newConnection, flowId: state.selectedFlow.id });
    },
    [state.selectedFlow, sendMessage]
  );

  const removeConnection = useCallback(
    (connectionId: string) => {
      if (!state.selectedFlow) {
        throw new Error('No flow selected');
      }
      setState((prev) => ({
        ...prev,
        selectedFlow: prev.selectedFlow
          ? {
              ...prev.selectedFlow,
              connections: prev.selectedFlow.connections.filter((conn) => conn.id !== connectionId),
            }
          : null,
      }));
      sendMessage('connection:remove', { connectionId, flowId: state.selectedFlow.id });
    },
    [state.selectedFlow, sendMessage]
  );

  const setFlowConfig = useCallback(
    (config: FlowConfig) => {
      if (!state.selectedFlow) {
        throw new Error('No flow selected');
      }
      setState((prev) => ({
        ...prev,
        selectedFlow: prev.selectedFlow ? { ...prev.selectedFlow, config } : null,
      }));
    },
    [state.selectedFlow]
  );

  const resetFlow = useCallback(() => {
    if (!state.selectedFlow) {
      throw new Error('No flow selected');
    }
    setState((prev) => ({
      ...prev,
      selectedFlow: prev.selectedFlow ? { ...prev.selectedFlow, nodes: {}, connections: [] } : null,
    }));
  }, [state.selectedFlow]);

  useEffect(() => {
    loadFlows();
    loadNodeTypes();
  }, [loadFlows, loadNodeTypes]);

  useEffect(() => {
    const unsubscribeStatus = subscribe('flow:status', (data) => {
      if (data && data.flowId && data.status) {
        applyStatus(data.flowId, data.status as FlowStatus);
      }
    });
    const unsubscribeNodeStatus = subscribe('node:status', (data) => {
      if (data && data.nodeId && data.flowId) {
        setState((prev) => {
          if (!prev.selectedFlow || prev.selectedFlow.id !== data.flowId || !prev.selectedFlow.nodes[data.nodeId]) {
            return prev;
          }
          return {
            ...prev,
            selectedFlow: {
              ...prev.selectedFlow,
              nodes: {
                ...prev.selectedFlow.nodes,
                [data.nodeId]: { ...prev.selectedFlow.nodes[data.nodeId], status: data.status },
              },
            },
          };
        });
      }
    });
    const unsubscribeList = subscribe('flow:list', (data) => {
      if (data && Array.isArray(data.flows)) {
        setState((prev) => ({ ...prev, flows: data.flows }));
      }
    });
    const unsubscribeError = subscribe('error', (data) => {
      const message = (data && (data.message || data.error)) || 'Unknown server error';
      setState((prev) => ({ ...prev, serverError: { message: String(message), at: Date.now() } }));
    });
    return () => {
      unsubscribeStatus();
      unsubscribeNodeStatus();
      unsubscribeList();
      unsubscribeError();
    };
  }, [subscribe, applyStatus]);

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
