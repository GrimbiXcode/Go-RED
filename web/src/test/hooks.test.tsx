import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { renderHook, act } from '@testing-library/react';
import { useFlows } from '../hooks/useFlows';
import type { Flow, FlowNode, NodeConnection, FlowStatus } from '../types/flow';

// Mock fetch
const mockFetch = vi.fn();
globalThis.fetch = mockFetch;

// Mock WebSocket
class MockWebSocket {
  url: string;
  readyState: number;
  onopen: ((this: MockWebSocket, ev: Event) => any) | null;
  onclose: ((this: MockWebSocket, ev: CloseEvent) => any) | null;
  onerror: ((this: MockWebSocket, ev: Event) => any) | null;
  onmessage: ((this: MockWebSocket, ev: MessageEvent) => any) | null;
  send: vi.fn();
  close: vi.fn();

  constructor(url: string | URL) {
    this.url = String(url);
    this.readyState = 1;
    this.onopen = null;
    this.onclose = null;
    this.onerror = null;
    this.onmessage = null;
  }
}

Object.defineProperty(globalThis, 'WebSocket', { value: MockWebSocket });

describe('useFlows Hook', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    
    // Default mock for fetch
    mockFetch.mockResolvedValue({
      ok: true,
      json: async () => ({
        flows: [],
        nodeTypes: [],
      }),
    });
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  describe('Initial State', () => {
    it('should initialize with default state', async () => {
      mockFetch
        .mockResolvedValueOnce({
          ok: true,
          json: async () => ({ flows: [] }),
        })
        .mockResolvedValueOnce({
          ok: true,
          json: async () => ({ nodeTypes: [] }),
        });

      const { result } = renderHook(() => useFlows());

      // Wait for initial fetch to complete
      await act(async () => {
        await new Promise(resolve => setTimeout(resolve, 100));
      });

      expect(result.current.loading).toBe(true);
      expect(result.current.error).toBe(null);
      expect(result.current.selectedFlowId).toBe(null);
      expect(result.current.selectedFlow).toBe(null);
    });
  });

  describe('loadFlows', () => {
    it('should load flows successfully', async () => {
      const mockFlows = [
        {
          id: 'flow-1',
          name: 'Flow 1',
          description: 'First flow',
          nodeCount: 2,
          connectionCount: 1,
          status: 'draft' as FlowStatus,
          createdAt: new Date().toISOString(),
          updatedAt: new Date().toISOString(),
        },
        {
          id: 'flow-2',
          name: 'Flow 2',
          description: 'Second flow',
          nodeCount: 5,
          connectionCount: 3,
          status: 'deployed' as FlowStatus,
          createdAt: new Date().toISOString(),
          updatedAt: new Date().toISOString(),
        },
      ];

      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: async () => ({ flows: mockFlows }),
      });

      const { result } = renderHook(() => useFlows());

      await act(async () => {
        await result.current.loadFlows();
      });

      expect(result.current.loading).toBe(false);
      expect(result.current.error).toBe(null);
      expect(result.current.flows).toEqual(mockFlows);
    });

    it('should handle error when loading flows', async () => {
      mockFetch.mockResolvedValueOnce({
        ok: false,
        status: 500,
        json: async () => ({ message: 'Internal server error' }),
      });

      const { result } = renderHook(() => useFlows());

      await act(async () => {
        await result.current.loadFlows();
      });

      expect(result.current.loading).toBe(false);
      expect(result.current.error).toBeDefined();
    });
  });

  describe('selectFlow', () => {
    it('should select a flow and load its details', async () => {
      const mockFlow: Flow = {
        id: 'flow-1',
        name: 'Test Flow',
        description: 'A test flow',
        nodes: {
          'node-1': {
            id: 'node-1',
            type: 'function',
            position: { x: 100, y: 200 },
            config: {},
          },
        },
        connections: [],
        status: 'draft' as FlowStatus,
        createdAt: new Date().toISOString(),
        updatedAt: new Date().toISOString(),
      };

      mockFetch
        .mockResolvedValueOnce({
          ok: true,
          json: async () => ({ flows: [] }),
        })
        .mockResolvedValueOnce({
          ok: true,
          json: async () => ({ nodeTypes: [] }),
        })
        .mockResolvedValueOnce({
          ok: true,
          json: async () => (mockFlow),
        });

      const { result } = renderHook(() => useFlows());

      // Wait for initial load
      await act(async () => {
        await new Promise(resolve => setTimeout(resolve, 100));
      });

      await act(async () => {
        await result.current.selectFlow('flow-1');
      });

      expect(result.current.selectedFlowId).toBe('flow-1');
      expect(result.current.selectedFlow).toEqual(mockFlow);
    });

    it('should handle error when selecting flow', async () => {
      mockFetch
        .mockResolvedValueOnce({
          ok: true,
          json: async () => ({ flows: [] }),
        })
        .mockResolvedValueOnce({
          ok: true,
          json: async () => ({ nodeTypes: [] }),
        })
        .mockResolvedValueOnce({
          ok: false,
          status: 404,
          json: async () => ({ message: 'Flow not found' }),
        });

      const { result } = renderHook(() => useFlows());

      // Wait for initial load
      await act(async () => {
        await new Promise(resolve => setTimeout(resolve, 100));
      });

      // This should not throw, but the flow won't be selected
      await act(async () => {
        await result.current.selectFlow('non-existent');
      });

      expect(result.current.selectedFlowId).toBe(null);
      expect(result.current.selectedFlow).toBe(null);
    });
  });

  describe('deselectFlow', () => {
    it('should deselect current flow', async () => {
      const mockFlow: Flow = {
        id: 'flow-1',
        name: 'Test Flow',
        nodes: {},
        connections: [],
        status: 'draft' as FlowStatus,
        createdAt: new Date().toISOString(),
        updatedAt: new Date().toISOString(),
      };

      mockFetch
        .mockResolvedValueOnce({
          ok: true,
          json: async () => ({ flows: [] }),
        })
        .mockResolvedValueOnce({
          ok: true,
          json: async () => ({ nodeTypes: [] }),
        })
        .mockResolvedValueOnce({
          ok: true,
          json: async () => (mockFlow),
        });

      const { result } = renderHook(() => useFlows());

      // Wait for initial load
      await act(async () => {
        await new Promise(resolve => setTimeout(resolve, 100));
      });

      // Select a flow
      await act(async () => {
        await result.current.selectFlow('flow-1');
      });

      expect(result.current.selectedFlowId).toBe('flow-1');

      // Deselect
      act(() => {
        result.current.deselectFlow();
      });

      expect(result.current.selectedFlowId).toBe(null);
      expect(result.current.selectedFlow).toBe(null);
    });
  });

  describe('addNode', () => {
    it('should add node to flow', async () => {
      const initialFlow: Flow = {
        id: 'flow-1',
        name: 'Test Flow',
        nodes: {},
        connections: [],
        status: 'draft' as FlowStatus,
        createdAt: new Date().toISOString(),
        updatedAt: new Date().toISOString(),
      };

      mockFetch
        .mockResolvedValueOnce({
          ok: true,
          json: async () => ({ flows: [] }),
        })
        .mockResolvedValueOnce({
          ok: true,
          json: async () => ({ nodeTypes: [] }),
        })
        .mockResolvedValueOnce({
          ok: true,
          json: async () => (initialFlow),
        });

      const { result } = renderHook(() => useFlows());

      // Wait for initial load
      await act(async () => {
        await new Promise(resolve => setTimeout(resolve, 100));
      });

      // Select the flow
      await act(async () => {
        await result.current.selectFlow('flow-1');
      });

      const position = { x: 100, y: 200 };

      act(() => {
        result.current.addNode('function', position);
      });

      // Check that node was added to local state
      expect(result.current.selectedFlow?.nodes).toBeDefined();
      const nodeIds = Object.keys(result.current.selectedFlow?.nodes || {});
      expect(nodeIds.length).toBe(1);
      
      const addedNode = result.current.selectedFlow?.nodes?.[nodeIds[0]];
      expect(addedNode?.type).toBe('function');
      expect(addedNode?.position).toEqual(position);
    });

    it('should fail to add node without selected flow', () => {
      mockFetch
        .mockResolvedValueOnce({
          ok: true,
          json: async () => ({ flows: [] }),
        })
        .mockResolvedValueOnce({
          ok: true,
          json: async () => ({ nodeTypes: [] }),
        });

      const { result } = renderHook(() => useFlows());

      expect(() => {
        result.current.addNode('function', { x: 100, y: 200 });
      }).toThrow('No flow selected');
    });
  });

  describe('removeNode', () => {
    it('should remove node from flow', async () => {
      const initialFlow: Flow = {
        id: 'flow-1',
        name: 'Test Flow',
        nodes: {
          'node-1': {
            id: 'node-1',
            type: 'function',
            position: { x: 100, y: 200 },
            config: {},
          },
          'node-2': {
            id: 'node-2',
            type: 'debug',
            position: { x: 300, y: 400 },
            config: {},
          },
        },
        connections: [],
        status: 'draft' as FlowStatus,
        createdAt: new Date().toISOString(),
        updatedAt: new Date().toISOString(),
      };

      mockFetch
        .mockResolvedValueOnce({
          ok: true,
          json: async () => ({ flows: [] }),
        })
        .mockResolvedValueOnce({
          ok: true,
          json: async () => ({ nodeTypes: [] }),
        })
        .mockResolvedValueOnce({
          ok: true,
          json: async () => (initialFlow),
        });

      const { result } = renderHook(() => useFlows());

      // Wait for initial load
      await act(async () => {
        await new Promise(resolve => setTimeout(resolve, 100));
      });

      // Select the flow
      await act(async () => {
        await result.current.selectFlow('flow-1');
      });

      act(() => {
        result.current.removeNode('node-1');
      });

      // Check that node was removed
      expect(result.current.selectedFlow?.nodes).toBeDefined();
      expect(result.current.selectedFlow?.nodes?.['node-1']).toBeUndefined();
      expect(result.current.selectedFlow?.nodes?.['node-2']).toBeDefined();
    });

    it('should fail to remove node without selected flow', () => {
      mockFetch
        .mockResolvedValueOnce({
          ok: true,
          json: async () => ({ flows: [] }),
        })
        .mockResolvedValueOnce({
          ok: true,
          json: async () => ({ nodeTypes: [] }),
        });

      const { result } = renderHook(() => useFlows());

      expect(() => {
        result.current.removeNode('node-1');
      }).toThrow('No flow selected');
    });
  });

  describe('addConnection', () => {
    it('should add connection to flow', async () => {
      const initialFlow: Flow = {
        id: 'flow-1',
        name: 'Test Flow',
        nodes: {
          'node-1': {
            id: 'node-1',
            type: 'function',
            position: { x: 100, y: 200 },
            config: {},
          },
          'node-2': {
            id: 'node-2',
            type: 'debug',
            position: { x: 300, y: 400 },
            config: {},
          },
        },
        connections: [],
        status: 'draft' as FlowStatus,
        createdAt: new Date().toISOString(),
        updatedAt: new Date().toISOString(),
      };

      mockFetch
        .mockResolvedValueOnce({
          ok: true,
          json: async () => ({ flows: [] }),
        })
        .mockResolvedValueOnce({
          ok: true,
          json: async () => ({ nodeTypes: [] }),
        })
        .mockResolvedValueOnce({
          ok: true,
          json: async () => (initialFlow),
        });

      const { result } = renderHook(() => useFlows());

      // Wait for initial load
      await act(async () => {
        await new Promise(resolve => setTimeout(resolve, 100));
      });

      // Select the flow
      await act(async () => {
        await result.current.selectFlow('flow-1');
      });

      const connection: Omit<NodeConnection, 'id'> = {
        sourceNode: 'node-1',
        sourcePort: 'output',
        targetNode: 'node-2',
        targetPort: 'input',
      };

      act(() => {
        result.current.addConnection(connection);
      });

      // Check that connection was added
      expect(result.current.selectedFlow?.connections).toBeDefined();
      expect(result.current.selectedFlow?.connections?.length).toBe(1);
      expect(result.current.selectedFlow?.connections?.[0].sourceNode).toBe('node-1');
      expect(result.current.selectedFlow?.connections?.[0].targetNode).toBe('node-2');
    });

    it('should fail to add connection without selected flow', () => {
      mockFetch
        .mockResolvedValueOnce({
          ok: true,
          json: async () => ({ flows: [] }),
        })
        .mockResolvedValueOnce({
          ok: true,
          json: async () => ({ nodeTypes: [] }),
        });

      const { result } = renderHook(() => useFlows());

      expect(() => {
        result.current.addConnection({
          sourceNode: 'node-1',
          targetNode: 'node-2',
        });
      }).toThrow('No flow selected');
    });
  });

  describe('removeConnection', () => {
    it('should remove connection from flow', async () => {
      const initialFlow: Flow = {
        id: 'flow-1',
        name: 'Test Flow',
        nodes: {
          'node-1': {
            id: 'node-1',
            type: 'function',
            position: { x: 100, y: 200 },
            config: {},
          },
          'node-2': {
            id: 'node-2',
            type: 'debug',
            position: { x: 300, y: 400 },
            config: {},
          },
        },
        connections: [
          {
            id: 'conn-1',
            sourceNode: 'node-1',
            targetNode: 'node-2',
          },
        ],
        status: 'draft' as FlowStatus,
        createdAt: new Date().toISOString(),
        updatedAt: new Date().toISOString(),
      };

      mockFetch
        .mockResolvedValueOnce({
          ok: true,
          json: async () => ({ flows: [] }),
        })
        .mockResolvedValueOnce({
          ok: true,
          json: async () => ({ nodeTypes: [] }),
        })
        .mockResolvedValueOnce({
          ok: true,
          json: async () => (initialFlow),
        });

      const { result } = renderHook(() => useFlows());

      // Wait for initial load
      await act(async () => {
        await new Promise(resolve => setTimeout(resolve, 100));
      });

      // Select the flow
      await act(async () => {
        await result.current.selectFlow('flow-1');
      });

      act(() => {
        result.current.removeConnection('conn-1');
      });

      // Check that connection was removed
      expect(result.current.selectedFlow?.connections).toBeDefined();
      expect(result.current.selectedFlow?.connections?.length).toBe(0);
    });

    it('should fail to remove connection without selected flow', () => {
      mockFetch
        .mockResolvedValueOnce({
          ok: true,
          json: async () => ({ flows: [] }),
        })
        .mockResolvedValueOnce({
          ok: true,
          json: async () => ({ nodeTypes: [] }),
        });

      const { result } = renderHook(() => useFlows());

      expect(() => {
        result.current.removeConnection('conn-1');
      }).toThrow('No flow selected');
    });
  });

  describe('Flow Node Position Handling', () => {
    it('should handle nodes with position', async () => {
      const flowWithNodes: Flow = {
        id: 'flow-1',
        name: 'Test Flow',
        nodes: {
          'node-1': {
            id: 'node-1',
            type: 'function',
            position: { x: 100, y: 200 },
            config: {},
          },
        },
        connections: [],
        status: 'draft' as FlowStatus,
        createdAt: new Date().toISOString(),
        updatedAt: new Date().toISOString(),
      };

      mockFetch
        .mockResolvedValueOnce({
          ok: true,
          json: async () => ({ flows: [] }),
        })
        .mockResolvedValueOnce({
          ok: true,
          json: async () => ({ nodeTypes: [] }),
        })
        .mockResolvedValueOnce({
          ok: true,
          json: async () => (flowWithNodes),
        });

      const { result } = renderHook(() => useFlows());

      // Wait for initial load
      await act(async () => {
        await new Promise(resolve => setTimeout(resolve, 100));
      });

      // Select the flow
      await act(async () => {
        await result.current.selectFlow('flow-1');
      });

      // Add a new node with position
      act(() => {
        result.current.addNode('function', { x: 300, y: 400 });
      });

      // Check that the new node has the correct position
      const nodeIds = Object.keys(result.current.selectedFlow?.nodes || {});
      expect(nodeIds.length).toBe(2);
      
      const newNode = result.current.selectedFlow?.nodes?.[nodeIds.find(id => id !== 'node-1') || ''];
      expect(newNode?.position.x).toBe(300);
      expect(newNode?.position.y).toBe(400);
    });
  });

  describe('Flow Update', () => {
    it('should update flow name and description', async () => {
      const initialFlow: Flow = {
        id: 'flow-1',
        name: 'Initial Name',
        description: 'Initial Description',
        nodes: {},
        connections: [],
        status: 'draft' as FlowStatus,
        createdAt: new Date().toISOString(),
        updatedAt: new Date().toISOString(),
      };

      const updatedFlow: Flow = {
        ...initialFlow,
        name: 'Updated Name',
        description: 'Updated Description',
      };

      mockFetch
        .mockResolvedValueOnce({
          ok: true,
          json: async () => ({ flows: [] }),
        })
        .mockResolvedValueOnce({
          ok: true,
          json: async () => ({ nodeTypes: [] }),
        })
        .mockResolvedValueOnce({
          ok: true,
          json: async () => (initialFlow),
        })
        .mockResolvedValueOnce({
          ok: true,
          json: async () => (updatedFlow),
        });

      const { result } = renderHook(() => useFlows());

      // Wait for initial load
      await act(async () => {
        await new Promise(resolve => setTimeout(resolve, 100));
      });

      // Select the flow
      await act(async () => {
        await result.current.selectFlow('flow-1');
      });

      // Update the flow
      await act(async () => {
        await result.current.updateCurrentFlow({
          name: 'Updated Name',
          description: 'Updated Description',
        });
      });

      expect(result.current.selectedFlow?.name).toBe('Updated Name');
      expect(result.current.selectedFlow?.description).toBe('Updated Description');
    });
  });
});
