import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen } from '@testing-library/react';
import { FlowCanvas } from '../components/FlowCanvas';
import type { Flow } from '../types/flow';

// Mock ReactFlow
vi.mock('reactflow', async () => {
  const actual = await vi.importActual<typeof import('reactflow')>('reactflow');
  return {
    ...actual,
    ReactFlowProvider: ({ children }: { children: React.ReactNode }) => children,
    useNodesState: () => [
      [],
      vi.fn(),
      vi.fn()
    ],
    useEdgesState: () => [
      [],
      vi.fn(),
      vi.fn()
    ],
    useReactFlow: () => ({
      screenToFlowPosition: (pos: { x: number; y: number }) => pos,
    }),
  };
});

// Test data
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
      config: {},
    },
    'node-2': {
      id: 'node-2',
      type: 'inject',
      position: { x: 300, y: 400 },
      config: {},
    },
  },
  connections: [
    {
      id: 'conn-1',
      sourceNode: 'node-1',
      sourcePort: 'output',
      targetNode: 'node-2',
      targetPort: 'input',
    },
  ],
  status: 'draft',
  createdAt: new Date().toISOString(),
  updatedAt: new Date().toISOString(),
};

const emptyFlow: Flow = {
  id: 'empty-flow',
  name: 'Empty Flow',
  nodes: {},
  connections: [],
  status: 'draft',
  createdAt: new Date().toISOString(),
  updatedAt: new Date().toISOString(),
};

const onNodeSelect = vi.fn();
const onNodeDeselect = vi.fn();
const onAddNode = vi.fn();
const onRemoveNode = vi.fn();
const onAddConnection = vi.fn();
const onRemoveConnection = vi.fn();

describe('FlowCanvas Component', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  describe('Rendering', () => {
    it('should render select flow message when no flow is selected', () => {
      render(
        <FlowCanvas
          flow={null}
          onNodeSelect={onNodeSelect}
          onNodeDeselect={onNodeDeselect}
          onAddNode={onAddNode}
          onRemoveNode={onRemoveNode}
          onAddConnection={onAddConnection}
          onRemoveConnection={onRemoveConnection}
        />
      );

      expect(screen.getByText(/Select a flow to edit/i)).toBeInTheDocument();
    });

    it('should render flow with nodes and connections', () => {
      render(
        <FlowCanvas
          flow={mockFlow}
          onNodeSelect={onNodeSelect}
          onNodeDeselect={onNodeDeselect}
          onAddNode={onAddNode}
          onRemoveNode={onRemoveNode}
          onAddConnection={onAddConnection}
          onRemoveConnection={onRemoveConnection}
        />
      );

      expect(screen.queryByText(/Select a flow to edit/i)).not.toBeInTheDocument();
    });

    it('should render empty flow', () => {
      render(
        <FlowCanvas
          flow={emptyFlow}
          onNodeSelect={onNodeSelect}
          onNodeDeselect={onNodeDeselect}
          onAddNode={onAddNode}
          onRemoveNode={onRemoveNode}
          onAddConnection={onAddConnection}
          onRemoveConnection={onRemoveConnection}
        />
      );

      expect(screen.queryByText(/Select a flow to edit/i)).not.toBeInTheDocument();
    });
  });

  describe('Node Position Handling', () => {
    it('should handle nodes with valid positions', () => {
      const flowWithPositions: Flow = {
        ...emptyFlow,
        nodes: {
          'node-1': {
            id: 'node-1',
            type: 'function',
            position: { x: 100, y: 200 },
            config: {},
          },
          'node-2': {
            id: 'node-2',
            type: 'inject',
            position: { x: 0, y: 0 },
            config: {},
          },
          'node-3': {
            id: 'node-3',
            type: 'debug',
            position: { x: -100, y: -200 },
            config: {},
          },
        },
      };

      render(
        <FlowCanvas
          flow={flowWithPositions}
          onNodeSelect={onNodeSelect}
          onNodeDeselect={onNodeDeselect}
          onAddNode={onAddNode}
          onRemoveNode={onRemoveNode}
          onAddConnection={onAddConnection}
          onRemoveConnection={onRemoveConnection}
        />
      );

      expect(screen.queryByText(/Select a flow to edit/i)).not.toBeInTheDocument();
    });

    it('should handle nodes without position (fallback)', () => {
      const flowWithoutPosition = {
        ...emptyFlow,
        nodes: {
          'node-1': {
            id: 'node-1',
            type: 'function',
            config: {},
          } as any,
        },
      } as Flow;

      render(
        <FlowCanvas
          flow={flowWithoutPosition}
          onNodeSelect={onNodeSelect}
          onNodeDeselect={onNodeDeselect}
          onAddNode={onAddNode}
          onRemoveNode={onRemoveNode}
          onAddConnection={onAddConnection}
          onRemoveConnection={onRemoveConnection}
        />
      );

      expect(screen.queryByText(/Select a flow to edit/i)).not.toBeInTheDocument();
    });
  });

  describe('Different Node Types', () => {
    it('should handle different node types', () => {
      const flowWithNodeTypes: Flow = {
        ...emptyFlow,
        nodes: {
          'inject-1': {
            id: 'inject-1',
            type: 'inject',
            position: { x: 100, y: 100 },
            config: {},
          },
          'function-1': {
            id: 'function-1',
            type: 'function',
            position: { x: 200, y: 200 },
            config: {},
          },
          'debug-1': {
            id: 'debug-1',
            type: 'debug',
            position: { x: 300, y: 300 },
            config: {},
          },
        },
      };

      render(
        <FlowCanvas
          flow={flowWithNodeTypes}
          onNodeSelect={onNodeSelect}
          onNodeDeselect={onNodeDeselect}
          onAddNode={onAddNode}
          onRemoveNode={onRemoveNode}
          onAddConnection={onAddConnection}
          onRemoveConnection={onRemoveConnection}
        />
      );

      expect(screen.queryByText(/Select a flow to edit/i)).not.toBeInTheDocument();
    });
  });

  describe('Edge Cases', () => {
    it('should handle very large flows', () => {
      const largeFlow: Flow = {
        ...emptyFlow,
        nodes: {},
        connections: [],
      };

      for (let i = 0; i < 50; i++) {
        largeFlow.nodes[`node-${i}`] = {
          id: `node-${i}`,
          type: 'function',
          position: { x: i * 20, y: i * 20 },
          config: {},
        };
      }

      for (let i = 0; i < 49; i++) {
        largeFlow.connections.push({
          id: `conn-${i}`,
          sourceNode: `node-${i}`,
          targetNode: `node-${i + 1}`,
        });
      }

      render(
        <FlowCanvas
          flow={largeFlow}
          onNodeSelect={onNodeSelect}
          onNodeDeselect={onNodeDeselect}
          onAddNode={onAddNode}
          onRemoveNode={onRemoveNode}
          onAddConnection={onAddConnection}
          onRemoveConnection={onRemoveConnection}
        />
      );

      expect(screen.queryByText(/Select a flow to edit/i)).not.toBeInTheDocument();
    });

    it('should handle nodes at extreme positions', () => {
      const flowWithExtremePositions: Flow = {
        ...emptyFlow,
        nodes: {
          'node-negative': {
            id: 'node-negative',
            type: 'function',
            position: { x: -10000, y: -10000 },
            config: {},
          },
          'node-positive': {
            id: 'node-positive',
            type: 'function',
            position: { x: 10000, y: 10000 },
            config: {},
          },
        },
      };

      render(
        <FlowCanvas
          flow={flowWithExtremePositions}
          onNodeSelect={onNodeSelect}
          onNodeDeselect={onNodeDeselect}
          onAddNode={onAddNode}
          onRemoveNode={onRemoveNode}
          onAddConnection={onAddConnection}
          onRemoveConnection={onRemoveConnection}
        />
      );

      expect(screen.queryByText(/Select a flow to edit/i)).not.toBeInTheDocument();
    });
  });
});
