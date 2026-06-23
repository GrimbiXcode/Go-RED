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
      const flowWithoutDescription: Flow = {
        id: 'flow-2',
        name: 'Flow without description',
        nodes: {},
        connections: [],
        status: 'draft' as FlowStatus,
        createdAt: new Date().toISOString(),
        updatedAt: new Date().toISOString(),
      };
      expect(flowWithoutDescription.description).toBeUndefined();
    });

    it('should have optional config', () => {
      const flowWithoutConfig: Flow = {
        id: 'flow-3',
        name: 'Flow without config',
        nodes: {},
        connections: [],
        status: 'draft' as FlowStatus,
        createdAt: new Date().toISOString(),
        updatedAt: new Date().toISOString(),
      };
      expect(flowWithoutConfig.config).toBeUndefined();
    });
  });

  describe('FlowNode', () => {
    it('should have all required fields', () => {
      const node: FlowNode = {
        id: 'node-1',
        type: 'function',
        position: { x: 100, y: 200 },
        config: { key: 'value' },
      };
      expect(node.id).toBe('node-1');
      expect(node.type).toBe('function');
      expect(node.position.x).toBe(100);
      expect(node.position.y).toBe(200);
      expect(node.config).toEqual({ key: 'value' });
    });

    it('should have optional name', () => {
      const nodeWithoutName: FlowNode = {
        id: 'node-2',
        type: 'debug',
        position: { x: 300, y: 400 },
        config: {},
      };
      expect(nodeWithoutName.name).toBeUndefined();
    });

    it('should have optional status', () => {
      const nodeWithoutStatus: FlowNode = {
        id: 'node-3',
        type: 'inject',
        position: { x: 500, y: 600 },
        config: {},
      };
      expect(nodeWithoutStatus.status).toBeUndefined();
    });

    it('should have optional disabled', () => {
      const nodeWithoutDisabled: FlowNode = {
        id: 'node-4',
        type: 'function',
        position: { x: 700, y: 800 },
        config: {},
      };
      expect(nodeWithoutDisabled.disabled).toBeUndefined();
    });

    it('should handle negative positions', () => {
      const nodeNegative: FlowNode = {
        id: 'node-5',
        type: 'function',
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

    it('should accept connections with all ports defined', () => {
      const connWithPorts: NodeConnection = {
        id: 'conn-2',
        sourceNode: 'node-1',
        sourcePort: 'output',
        targetNode: 'node-2',
        targetPort: 'input',
      };
      expect(connWithPorts.sourcePort).toBe('output');
      expect(connWithPorts.targetPort).toBe('input');
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

    it('should not accept invalid status values', () => {
      // This would be a compile-time error if uncommented
      // const invalidFlow: Flow = {
      //   ...mockFlow,
      //   status: 'invalid' as FlowStatus,
      // };
    });
  });

  describe('FlowConfig', () => {
    it('should have all optional fields', () => {
      const config: FlowConfig = {
        autoDeploy: true,
        timeout: 60,
        maxMessages: 1000,
        environment: { NODE_ENV: 'production' },
      };
      expect(config.autoDeploy).toBe(true);
      expect(config.timeout).toBe(60);
      expect(config.maxMessages).toBe(1000);
      expect(config.environment).toEqual({ NODE_ENV: 'production' });
    });

    it('should accept empty config', () => {
      const emptyConfig: FlowConfig = {};
      expect(emptyConfig.autoDeploy).toBeUndefined();
      expect(emptyConfig.timeout).toBeUndefined();
      expect(emptyConfig.maxMessages).toBeUndefined();
      expect(emptyConfig.environment).toBeUndefined();
    });
  });

  describe('NodeStatus', () => {
    it('should have all required fields', () => {
      const status: NodeStatus = {
        state: 'idle',
      };
      expect(status.state).toBe('idle');
    });

    it('should have optional fields', () => {
      const statusWithOptional: NodeStatus = {
        state: 'processing',
        message: 'Processing message',
        timestamp: new Date().toISOString(),
        processingCount: 10,
        errorCount: 0,
      };
      expect(statusWithOptional.message).toBe('Processing message');
      expect(statusWithOptional.timestamp).toBeDefined();
      expect(statusWithOptional.processingCount).toBe(10);
      expect(statusWithOptional.errorCount).toBe(0);
    });

    it('should accept all valid states', () => {
      const validStates: NodeStatus['state'][] = ['idle', 'processing', 'error', 'completed'];
      validStates.forEach(state => {
        const status: NodeStatus = { state };
        expect(status.state).toBe(state);
      });
    });
  });

  describe('Flow Validation', () => {
    it('should validate a valid flow with connections', () => {
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

    it('should handle empty flows', () => {
      const emptyFlow: Flow = {
        id: 'empty-flow',
        name: 'Empty Flow',
        nodes: {},
        connections: [],
        status: 'draft' as FlowStatus,
        createdAt: new Date().toISOString(),
        updatedAt: new Date().toISOString(),
      };

      expect(Object.keys(emptyFlow.nodes)).toHaveLength(0);
      expect(emptyFlow.connections).toHaveLength(0);
    });

    it('should handle flows with multiple connections', () => {
      const multiConnFlow: Flow = {
        id: 'multi-conn-flow',
        name: 'Multi Connection Flow',
        nodes: {
          'node-1': { id: 'node-1', type: 'function', position: { x: 100, y: 100 }, config: {} },
          'node-2': { id: 'node-2', type: 'function', position: { x: 300, y: 100 }, config: {} },
          'node-3': { id: 'node-3', type: 'debug', position: { x: 500, y: 100 }, config: {} },
        },
        connections: [
          { id: 'conn-1', sourceNode: 'node-1', targetNode: 'node-2' },
          { id: 'conn-2', sourceNode: 'node-2', targetNode: 'node-3' },
        ],
        status: 'deployed' as FlowStatus,
        createdAt: new Date().toISOString(),
        updatedAt: new Date().toISOString(),
      };

      expect(multiConnFlow.connections).toHaveLength(2);
      expect(multiConnFlow.connections[0].sourceNode).toBe('node-1');
      expect(multiConnFlow.connections[1].targetNode).toBe('node-3');
    });
  });

  describe('Type Compatibility', () => {
    it('should allow FlowNode config to accept any structure', () => {
      const nodeWithComplexConfig: FlowNode = {
        id: 'node-complex',
        type: 'function',
        position: { x: 100, y: 200 },
        config: {
          nested: {
            value: 42,
            array: [1, 2, 3],
            boolean: true,
          },
        },
      };
      expect(nodeWithComplexConfig.config.nested.value).toBe(42);
    });

    it('should handle different node types', () => {
      const nodeTypes = ['function', 'inject', 'debug', 'switch', 'delay', 'http'];
      nodeTypes.forEach(type => {
        const node: FlowNode = {
          id: `node-${type}`,
          type,
          position: { x: 100, y: 200 },
          config: {},
        };
        expect(node.type).toBe(type);
      });
    });
  });
});
