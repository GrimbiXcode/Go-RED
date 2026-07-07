import { describe, it, expect } from 'vitest';
import type { Flow, FlowNode, NodeConnection, FlowStatus, FlowConfig, NodeStatus } from '../types/flow';

const defaultConfig: FlowConfig = {
  timeout: 30,
  maxConcurrency: 100,
  retryPolicy: {
    maxRetries: 3,
    backoff: 1,
    maxBackoff: 30,
    retryOn: [],
  },
  environment: {},
};

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
  config: defaultConfig,
  createdAt: new Date().toISOString(),
  updatedAt: new Date().toISOString(),
  version: '1.0',
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
      expect(mockFlow.config).toBeDefined();
      expect(mockFlow.version).toBeDefined();
      expect(mockFlow.createdAt).toBeDefined();
      expect(mockFlow.updatedAt).toBeDefined();
    });

    it('description is always a string (never omitted on the wire)', () => {
      const flowWithoutDescription: Flow = {
        id: 'flow-2',
        name: 'Flow without description',
        description: '',
        nodes: {},
        connections: [],
        status: 'draft' as FlowStatus,
        config: defaultConfig,
        createdAt: new Date().toISOString(),
        updatedAt: new Date().toISOString(),
        version: '1.0',
      };
      expect(flowWithoutDescription.description).toBe('');
    });

    it('config is always present (never omitted on the wire)', () => {
      const flowWithConfig: Flow = {
        id: 'flow-3',
        name: 'Flow with config',
        description: '',
        nodes: {},
        connections: [],
        status: 'draft' as FlowStatus,
        config: defaultConfig,
        createdAt: new Date().toISOString(),
        updatedAt: new Date().toISOString(),
        version: '1.0',
      };
      expect(flowWithConfig.config).toEqual(defaultConfig);
    });
  });

  describe('FlowNode', () => {
    it('should have all required fields', () => {
      const node: FlowNode = {
        id: 'node-1',
        type: 'function',
        position: { x: 100, y: 200 },
        config: { key: 'value' },
        status: { state: 'idle' },
        disabled: false,
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
        status: { state: 'idle' },
        disabled: false,
      };
      expect(nodeWithoutName.name).toBeUndefined();
    });

    it('status and disabled are always present (never omitted on the wire)', () => {
      const node: FlowNode = {
        id: 'node-3',
        type: 'inject',
        position: { x: 500, y: 600 },
        config: {},
        status: { state: 'idle' },
        disabled: false,
      };
      expect(node.status?.state).toBe('idle');
      expect(node.disabled).toBe(false);
    });

    it('should handle negative positions', () => {
      const nodeNegative: FlowNode = {
        id: 'node-5',
        type: 'function',
        position: { x: -100, y: -200 },
        config: {},
        status: { state: 'idle' },
        disabled: false,
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
      // These are exactly the values internal/dto.FlowStatusFromEngine can
      // produce; the backend never sends anything else.
      const validStatuses: FlowStatus[] = ['draft', 'running', 'error', 'deploying', 'undeploying'];
      validStatuses.forEach(status => {
        const flow: Flow = {
          id: 'flow-status',
          name: 'Status Flow',
          description: '',
          nodes: {},
          connections: [],
          status,
          config: defaultConfig,
          createdAt: new Date().toISOString(),
          updatedAt: new Date().toISOString(),
          version: '1.0',
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
    it('should have all required fields, including a nested retryPolicy', () => {
      const config: FlowConfig = {
        timeout: 60,
        maxConcurrency: 50,
        retryPolicy: {
          maxRetries: 5,
          backoff: 2,
          maxBackoff: 60,
          retryOn: ['timeout'],
        },
        environment: { NODE_ENV: 'production' },
      };
      expect(config.timeout).toBe(60);
      expect(config.maxConcurrency).toBe(50);
      expect(config.retryPolicy.maxRetries).toBe(5);
      expect(config.environment).toEqual({ NODE_ENV: 'production' });
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
        description: '',
        nodes: {
          'node-1': { id: 'node-1', type: 'function', position: { x: 100, y: 200 }, config: {}, status: { state: 'idle' }, disabled: false },
          'node-2': { id: 'node-2', type: 'debug', position: { x: 300, y: 400 }, config: {}, status: { state: 'idle' }, disabled: false },
        },
        connections: [
          { id: 'conn-1', sourceNode: 'node-1', targetNode: 'node-2' },
        ],
        status: 'draft' as FlowStatus,
        config: defaultConfig,
        createdAt: new Date().toISOString(),
        updatedAt: new Date().toISOString(),
        version: '1.0',
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
        description: '',
        nodes: {
          'node-1': { id: 'node-1', type: 'function', position: { x: 100, y: 200 }, config: {}, status: { state: 'idle' }, disabled: false },
        },
        connections: [
          { id: 'conn-1', sourceNode: 'node-1', targetNode: 'non-existent' },
        ],
        status: 'draft' as FlowStatus,
        config: defaultConfig,
        createdAt: new Date().toISOString(),
        updatedAt: new Date().toISOString(),
        version: '1.0',
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
        description: '',
        nodes: {},
        connections: [],
        status: 'draft' as FlowStatus,
        config: defaultConfig,
        createdAt: new Date().toISOString(),
        updatedAt: new Date().toISOString(),
        version: '1.0',
      };

      expect(Object.keys(emptyFlow.nodes)).toHaveLength(0);
      expect(emptyFlow.connections).toHaveLength(0);
    });

    it('should handle flows with multiple connections', () => {
      const multiConnFlow: Flow = {
        id: 'multi-conn-flow',
        name: 'Multi Connection Flow',
        description: '',
        nodes: {
          'node-1': { id: 'node-1', type: 'function', position: { x: 100, y: 100 }, config: {}, status: { state: 'idle' }, disabled: false },
          'node-2': { id: 'node-2', type: 'function', position: { x: 300, y: 100 }, config: {}, status: { state: 'idle' }, disabled: false },
          'node-3': { id: 'node-3', type: 'debug', position: { x: 500, y: 100 }, config: {}, status: { state: 'idle' }, disabled: false },
        },
        connections: [
          { id: 'conn-1', sourceNode: 'node-1', targetNode: 'node-2' },
          { id: 'conn-2', sourceNode: 'node-2', targetNode: 'node-3' },
        ],
        status: 'running' as FlowStatus,
        config: defaultConfig,
        createdAt: new Date().toISOString(),
        updatedAt: new Date().toISOString(),
        version: '1.0',
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
        status: { state: 'idle' },
        disabled: false,
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
          status: { state: 'idle' },
          disabled: false,
        };
        expect(node.type).toBe(type);
      });
    });
  });
});
