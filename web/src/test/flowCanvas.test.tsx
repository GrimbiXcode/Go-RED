import { describe, it, expect } from 'vitest';
import { FlowNode } from '../types/flow';

describe('FlowCanvas Position Fallback Logic', () => {
  describe('Node Position Handling', () => {
    it('should provide fallback position when position is undefined', () => {
      const nodeWithoutPosition: any = {
        id: 'node-1',
        type: 'function',
        config: {},
      };

      // Simulate the fallback logic from FlowCanvas.tsx line 78
      const position = nodeWithoutPosition.position || { x: 0, y: 0 };
      
      expect(position.x).toBe(0);
      expect(position.y).toBe(0);
    });

    it('should provide fallback position when position is null', () => {
      const nodeWithNullPosition: any = {
        id: 'node-1',
        type: 'function',
        position: null,
        config: {},
      };

      const position = nodeWithNullPosition.position || { x: 0, y: 0 };
      
      expect(position.x).toBe(0);
      expect(position.y).toBe(0);
    });

    it('should use existing position when available', () => {
      const nodeWithPosition: FlowNode = {
        id: 'node-1',
        type: 'function',
        position: { x: 100, y: 200 },
        config: {},
      };

      const position = nodeWithPosition.position || { x: 0, y: 0 };
      
      expect(position.x).toBe(100);
      expect(position.y).toBe(200);
    });

    it('should handle partial position with fallback for missing coordinates', () => {
      const nodeWithPartialPosition: any = {
        id: 'node-1',
        type: 'function',
        position: { x: 100 },
        config: {},
      };

      // More robust fallback for each coordinate
      const x = nodeWithPartialPosition.position?.x || 0;
      const y = nodeWithPartialPosition.position?.y || 0;
      
      expect(x).toBe(100);
      expect(y).toBe(0);
    });

    it('should handle position with both coordinates missing', () => {
      const nodeWithEmptyPosition: any = {
        id: 'node-1',
        type: 'function',
        position: {},
        config: {},
      };

      const x = nodeWithEmptyPosition.position?.x || 0;
      const y = nodeWithEmptyPosition.position?.y || 0;
      
      expect(x).toBe(0);
      expect(y).toBe(0);
    });

    it('should handle position with NaN values', () => {
      const nodeWithNaNPosition: any = {
        id: 'node-1',
        type: 'function',
        position: { x: NaN, y: NaN },
        config: {},
      };

      // NaN is falsy, so fallback should work
      const x = nodeWithNaNPosition.position?.x || 0;
      const y = nodeWithNaNPosition.position?.y || 0;
      
      expect(x).toBe(0);
      expect(y).toBe(0);
    });
  });

  describe('FlowCanvas Node Array Conversion', () => {
    it('should convert nodes object to array for ReactFlow', () => {
      const nodes: Record<string, FlowNode> = {
        'node-1': { id: 'node-1', type: 'function', position: { x: 100, y: 200 }, config: {} },
        'node-2': { id: 'node-2', type: 'debug', position: { x: 300, y: 400 }, config: {} },
      };

      const nodeArray = Object.values(nodes);
      
      expect(nodeArray).toHaveLength(2);
      expect(nodeArray[0].id).toBe('node-1');
      expect(nodeArray[1].id).toBe('node-2');
    });

    it('should handle empty nodes object', () => {
      const nodes: Record<string, FlowNode> = {};
      const nodeArray = Object.values(nodes);
      
      expect(nodeArray).toHaveLength(0);
    });

    it('should preserve node order when converting to array', () => {
      const nodes: Record<string, FlowNode> = {
        'node-1': { id: 'node-1', type: 'function', position: { x: 100, y: 200 }, config: {} },
        'node-2': { id: 'node-2', type: 'debug', position: { x: 300, y: 400 }, config: {} },
        'node-3': { id: 'node-3', type: 'inject', position: { x: 500, y: 600 }, config: {} },
      };

      const nodeArray = Object.values(nodes);
      
      expect(nodeArray.length).toBe(3);
      expect(nodeArray.map(n => n.id)).toContain('node-1');
      expect(nodeArray.map(n => n.id)).toContain('node-2');
      expect(nodeArray.map(n => n.id)).toContain('node-3');
    });
  });

  describe('ReactFlow Node Data Structure', () => {
    it('should create valid ReactFlow node structure', () => {
      const flowNode: FlowNode = {
        id: 'node-1',
        type: 'function',
        name: 'Function Node',
        position: { x: 100, y: 200 },
        config: { key: 'value' },
        disabled: false,
      };

      // Simulate the node structure used in FlowCanvas.tsx
      const reactFlowNode = {
        id: flowNode.id,
        type: flowNode.type,
        position: flowNode.position,
        data: {
          label: flowNode.name || flowNode.type,
          node: flowNode,
        },
        draggable: true,
        selectable: true,
        connectable: true,
      };

      expect(reactFlowNode.id).toBe('node-1');
      expect(reactFlowNode.type).toBe('function');
      expect(reactFlowNode.position.x).toBe(100);
      expect(reactFlowNode.position.y).toBe(200);
      expect(reactFlowNode.data.label).toBe('Function Node');
      expect(reactFlowNode.data.node).toBe(flowNode);
    });

    it('should use type as label when name is undefined', () => {
      const flowNode: FlowNode = {
        id: 'node-1',
        type: 'function',
        position: { x: 100, y: 200 },
        config: {},
      };

      const reactFlowNode = {
        id: flowNode.id,
        type: flowNode.type,
        position: flowNode.position,
        data: {
          label: flowNode.name || flowNode.type,
          node: flowNode,
        },
      };

      expect(reactFlowNode.data.label).toBe('function');
    });

    it('should handle disabled nodes', () => {
      const flowNode: FlowNode = {
        id: 'node-1',
        type: 'function',
        position: { x: 100, y: 200 },
        config: {},
        disabled: true,
      };

      const reactFlowNode = {
        id: flowNode.id,
        type: flowNode.type,
        position: flowNode.position,
        data: {
          label: flowNode.name || flowNode.type,
          node: flowNode,
        },
        draggable: !flowNode.disabled,
        selectable: !flowNode.disabled,
        connectable: !flowNode.disabled,
        style: flowNode.disabled ? { opacity: 0.5 } : {},
      };

      expect(reactFlowNode.draggable).toBe(false);
      expect(reactFlowNode.selectable).toBe(false);
      expect(reactFlowNode.connectable).toBe(false);
      expect(reactFlowNode.style.opacity).toBe(0.5);
    });
  });

  describe('Connection Error Prevention', () => {
    it('should not create connections with undefined positions', () => {
      const nodeWithoutPosition: any = {
        id: 'node-1',
        type: 'function',
        config: {},
      };

      // This would cause the "Cannot read properties of undefined (reading 'x')" error
      // if we try to access nodeWithoutPosition.position.x directly
      
      // Safe access pattern
      const x = nodeWithoutPosition.position?.x;
      const y = nodeWithoutPosition.position?.y;
      
      // This should not throw
      expect(x).toBeUndefined();
      expect(y).toBeUndefined();
    });

    it('should safely handle nodes in array with missing positions', () => {
      const nodes: any[] = [
        { id: 'node-1', type: 'function', position: { x: 100, y: 200 }, config: {} },
        { id: 'node-2', type: 'debug', config: {} }, // Missing position
        { id: 'node-3', type: 'inject', position: { x: 300, y: 400 }, config: {} },
      ];

      // Safely process all nodes without throwing
      const processedNodes = nodes.map(node => ({
        ...node,
        position: node.position || { x: 0, y: 0 },
      }));

      expect(processedNodes).toHaveLength(3);
      expect(processedNodes[0].position.x).toBe(100);
      expect(processedNodes[1].position.x).toBe(0); // Fallback
      expect(processedNodes[2].position.x).toBe(300);
    });
  });
});
