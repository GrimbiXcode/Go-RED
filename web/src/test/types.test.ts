import { describe, it, expect } from 'vitest';
import type { Flow, FlowNode, NodeConnection, FlowStatus, FlowConfig, NodeStatus } from '../types/flow';

// Test Flow Type
const mockFlow: Flow = {
  id: 'flow-1',
  name: 'Test Flow',
  description: 'A test flow',
  nodes: {
    'node-1': {
      id: 'node-1',
      type: 'function',
      name: 'Function Node',
      position: { x: 100, y: 200 },
      config: { key: 'value' },
      status: { state: 'idle' },
      disabled: false,
    },
  },
  connections: [
    {
      id: 'conn-1',
      sourceNode: 'node-1',
      sourcePort: 'output',
      targetNode: 'node-1',
      targetPort: 'input',
    },
  ],
  status: 'draft' as FlowStatus,
  config: {
    autoDeploy: false,
    timeout: 30,
    maxMessages: 100,
    environment: {},
  },
  createdAt: new Date().toISOString(),
  updatedAt: new Date().toISOString(),
};

describe('Flow Types', () => {
  describe('Flow', () => {
    it('should have all required fields', () => {
      expect(mockFlow.id).toBe('flow-1');
      expect(mockFlow.name).toBe('Test Flow');
      expect(mockFlow.description).toBe('A test flow');
      expect(mockFlow.nodes).toBeDefined();
      expect(mockFlow.connections).toBeDefined();
      expect(mockFlow.status).toBe('draft');
      expect(mockFlow.createdAt).toBeDefined();
      expect(mockFlow.updatedAt).toBeDefined();
    });

    it('should have optional description', () => {
      const flowWithoutDesc: Flow = {
        ...mockFlow,
        description: undefined,
      };
      expect(flowWithoutDesc.description).toBeUndefined();
    });

    it('should have optional config', () => {
      const flowWithoutConfig: Flow = {
        ...mockFlow,
        config: undefined,
      };
      expect(flowWithoutConfig.config).toBeUndefined();
    });

    it('should allow empty nodes and connections', () => {
      const emptyFlow: Flow = {
        id: 'empty-flow',
        name: 'Empty Flow',
        nodes: {},
        connections: [],
        status: 'draft' as FlowStatus,
        createdAt: new Date().toISOString(),
        updatedAt: new Date().toISOString(),
      };
      expect(emptyFlow.nodes).toEqual({});
      expect(emptyFlow.connections).toEqual([]);
    });
  });

  describe('FlowNode', () => {
    it('should have all required fields', () => {
      const node: FlowNode = {
        id: 'node-1',
        type: 'function',
        position: { x: 100, y: 200 },
        config: {},
      };
      expect(node.id).toBe('node-1');
      expect(node.type).toBe('function');
      expect(node.position).toBeDefined();
      expect(node.position.x).toBe(100);
      expect(node.position.y).toBe(200);
      expect(node.config).toBeDefined();
    });

    it('should have optional name', () => {
      const nodeWithoutName: FlowNode = {
        id: 'node-1',
        type: 'function',
        position: { x: 100, y: 200 },
        config: {},
        name: undefined,
      };
      expect(nodeWithoutName.name).toBeUndefined();
    });

    it('should have optional status', () => {
      const nodeWithoutStatus: FlowNode = {
        id: 'node-1',
        type: 'function',
        position: { x: 100, y: 200 },
        config: {},
        status: undefined,
      };
      expect(nodeWithoutStatus.status).toBeUndefined();
    });

    it('should have optional disabled', () => {
      const nodeWithoutDisabled: FlowNode = {
        id: 'node-1',
        type: 'function',
        position: { x: 100, y: 200 },
        config: {},
        disabled: undefined,
      };
      expect(nodeWithoutDisabled.disabled).toBeUndefined();
    });

    it('should handle position with zero coordinates', () => {
      const nodeAtOrigin: FlowNode = {
        id: 'node-origin',
        type: 'inject',
        position: { x: 0, y: 0 },
        config: {},
      };
      expect(nodeAtOrigin.position.x).toBe(0);
      expect(nodeAtOrigin.position.y).toBe(0);
    });

    it('should handle position with negative coordinates', () => {
      const nodeNegative: FlowNode = {
        id: 'node-negative',
        type: 'debug',
        position: { x: -100, y: -200 },
        config: {},
      };
      expect(nodeNegative.position.x).toBe(-100);
      expect(nodeNegative.position.y).toBe(-200);
    });
  });

  describe('NodeConnection', () => {
    it('should have all required fields', () => {
      const conn: NodeConnection = {
        id: 'conn-1',
        sourceNode: 'node-1',
        targetNode: 'node-2',
      };
      expect(conn.id).toBe('conn-1');
      expect(conn.sourceNode).toBe('node-1');
      expect(conn.targetNode).toBe('node-2');
    });

    it('should have optional sourcePort', () => {
      const connWithoutSourcePort: NodeConnection = {
        id: 'conn-1',
        sourceNode: 'node-1',
        targetNode: 'node-2',
        sourcePort: undefined,
      };
      expect(connWithoutSourcePort.sourcePort).toBeUndefined();
    });

    it('should have optional targetPort', () => {
      const connWithoutTargetPort: NodeConnection = {
        id: 'conn-1',
        sourceNode: 'node-1',
        targetNode: 'node-2',
        targetPort: undefined,
      };
      expect(connWithoutTargetPort.targetPort).toBeUndefined();
    });
  });

  describe('FlowStatus', () => {
    it('should accept all valid status values', () => {
      const validStatuses: FlowStatus[] = ['draft', 'deployed', 'running', 'stopped', 'error', 'paused'];
      validStatuses.forEach(status => {
        const flow: Flow = {
          id: 'flow-status',
          name: 'Status Flow',
          nodes: {},
          connections: [],
          status,
          createdAt: new Date().toISOString(),
          updatedAt: new Date().toISOString(),
        };
        expect(flow.status).toBe(status);
      });
    });
  });

  describe('FlowConfig', () => {
    it('should handle various config values', () => {
      const configs: FlowConfig[] = [
        {},
        { autoDeploy: true },
        { timeout: 60 },
        { maxMessages: 500 },
        { environment: { VAR1: 'value1', VAR2: 'value2' } },
        { autoDeploy: true, timeout: 30, maxMessages: 100, environment: {} },
      ];

      configs.forEach((config, index) => {
        const flow: Flow = {
          id: `flow-${index}`,
          name: `Config Flow ${index}`,
          nodes: {},
          connections: [],
          status: 'draft' as FlowStatus,
          config,
          createdAt: new Date().toISOString(),
          updatedAt: new Date().toISOString(),
        };
        expect(flow.config).toEqual(config);
      });
    });
  });

  describe('NodeStatus', () => {
    it('should handle all NodeStatus states', () => {
      const states: NodeStatus['state'][] = ['idle', 'processing', 'error', 'completed'];

      states.forEach(state => {
        const node: FlowNode = {
          id: `node-${state}`,
          type: 'function',
          position: { x: 100, y: 200 },
          config: {},
          status: { state },
        };
        expect(node.status?.state).toBe(state);
      });
    });

    it('should handle status with additional fields', () => {
      const node: FlowNode = {
        id: 'node-with-status',
        type: 'function',
        position: { x: 100, y: 200 },
        config: {},
        status: {
          state: 'processing',
          message: 'Processing message',
          timestamp: new Date().toISOString(),
          processingCount: 5,
          errorCount: 0,
        },
      };

      expect(node.status?.state).toBe('processing');
      expect(node.status?.message).toBe('Processing message');
      expect(node.status?.timestamp).toBeDefined();
      expect(node.status?.processingCount).toBe(5);
      expect(node.status?.errorCount).toBe(0);
    });
  });

  describe('Flow Serialization', () => {
    it('should serialize and deserialize flow correctly', () => {
      const originalFlow: Flow = mockFlow;

      // Serialize to JSON
      const jsonString = JSON.stringify(originalFlow);
      expect(jsonString).toBeDefined();

      // Deserialize from JSON
      const parsedFlow: Flow = JSON.parse(jsonString);

      // Check that all fields are preserved
      expect(parsedFlow.id).toBe(originalFlow.id);
      expect(parsedFlow.name).toBe(originalFlow.name);
      expect(parsedFlow.description).toBe(originalFlow.description);
      expect(parsedFlow.status).toBe(originalFlow.status);

      // Check nodes
      expect(Object.keys(parsedFlow.nodes).length).toBe(Object.keys(originalFlow.nodes).length);
      Object.keys(originalFlow.nodes).forEach(nodeId => {
        expect(parsedFlow.nodes[nodeId]).toBeDefined();
        expect(parsedFlow.nodes[nodeId].id).toBe(originalFlow.nodes[nodeId].id);
        expect(parsedFlow.nodes[nodeId].type).toBe(originalFlow.nodes[nodeId].type);
        expect(parsedFlow.nodes[nodeId].position.x).toBe(originalFlow.nodes[nodeId].position.x);
        expect(parsedFlow.nodes[nodeId].position.y).toBe(originalFlow.nodes[nodeId].position.y);
      });

      // Check connections
      expect(parsedFlow.connections.length).toBe(originalFlow.connections.length);
      originalFlow.connections.forEach((conn, index) => {
        expect(parsedFlow.connections[index].id).toBe(conn.id);
        expect(parsedFlow.connections[index].sourceNode).toBe(conn.sourceNode);
        expect(parsedFlow.connections[index].targetNode).toBe(conn.targetNode);
      });
    });

    it('should handle serialization of nodes with position', () => {
      const node: FlowNode = {
        id: 'node-1',
        type: 'function',
        name: 'Function Node',
        position: { x: 100, y: 200 },
        config: { key: 'value' },
        status: { state: 'idle' },
        disabled: false,
      };
      const jsonString = JSON.stringify(node);
      const parsedNode: FlowNode = JSON.parse(jsonString);

      expect(parsedNode.position.x).toBe(node.position.x);
      expect(parsedNode.position.y).toBe(node.position.y);
    });
  });

  describe('Flow Validation', () => {
    it('should validate flow with nodes and connections', () => {
      const validFlow: Flow = {
        id: 'valid-flow',
        name: 'Valid Flow',
        nodes: {
          'node-1': { id: 'node-1', type: 'function', position: { x: 100, y: 200 }, config: {} },
          'node-2': { id: 'node-2', type: 'debug', position: { x: 300, y: 400 }, config: {} },
        },
        connections: [
          { id: 'conn-1', sourceNode: 'node-1', targetNode: 'node-2' },
        ],
        status: 'draft' as FlowStatus,
        createdAt: new Date().toISOString(),
        updatedAt: new Date().toISOString(),
      };

      // Check that all connection references exist
      validFlow.connections.forEach(conn => {
        expect(validFlow.nodes[conn.sourceNode]).toBeDefined();
        expect(validFlow.nodes[conn.targetNode]).toBeDefined();
      });
    });

    it('should detect invalid connections', () => {
      const invalidFlow: Flow = {
        id: 'invalid-flow',
        name: 'Invalid Flow',
        nodes: {
          'node-1': { id: 'node-1', type: 'function', position: { x: 100, y: 200 }, config: {} },
        },
        connections: [
          { id: 'conn-1', sourceNode: 'node-1', targetNode: 'non-existent' },
        ],
        status: 'draft' as FlowStatus,
        createdAt: new Date().toISOString(),
        updatedAt: new Date().toISOString(),
      };

      // Check for invalid references
      const hasInvalidRefs = invalidFlow.connections.some(conn => {
        return !invalidFlow.nodes[conn.sourceNode] || !invalidFlow.nodes[conn.targetNode];
      });

      expect(hasInvalidRefs).toBe(true);
    });
  });

  describe('Position Fallback', () => {
    it('should handle position fallback in FlowCanvas', () => {
      // Simulate the position fallback logic from FlowCanvas.tsx
      const nodeWithoutPosition: any = {
        id: 'node-no-pos',
        type: 'function',
        // position is missing
      };

      const position = nodeWithoutPosition.position || { x: 0, y: 0 };
      expect(position.x).toBe(0);
      expect(position.y).toBe(0);
    });
  });
});
