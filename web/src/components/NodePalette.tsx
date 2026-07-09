import React, { useCallback, useState, useMemo } from 'react';
import type { NodeMetadata } from '../types/node';
import { CategoryIcon, NodeIcon } from './CategoryIcon';
import { getCategoryColor, sortCategories } from '../utils/nodeCategories';

interface NodePaletteProps {
  nodeTypes: NodeMetadata[];
  loading: boolean;
}

function getCategories(nodeTypes: NodeMetadata[]): string[] {
  const categories = new Set<string>();
  nodeTypes.forEach((node) => {
    categories.add(node.category);
  });
  return sortCategories(Array.from(categories));
}

function groupByCategory(nodeTypes: NodeMetadata[]): Record<string, NodeMetadata[]> {
  const grouped: Record<string, NodeMetadata[]> = {} as Record<string, NodeMetadata[]>;
  const categories = getCategories(nodeTypes);
  categories.forEach((category) => {
    grouped[category] = [];
  });
  nodeTypes.forEach((node) => {
    if (grouped[node.category]) {
      grouped[node.category].push(node);
    }
  });
  return grouped;
}

interface NodePaletteItemProps {
  node: NodeMetadata;
  onDragStart: (event: React.DragEvent<HTMLDivElement>, nodeType: string) => void;
}

function NodePaletteItem({ node, onDragStart }: NodePaletteItemProps) {
  const handleDragStart = useCallback(
    (event: React.DragEvent<HTMLDivElement>) => {
      event.dataTransfer.setData('application/reactflow', JSON.stringify({ nodeType: node.type }));
      event.dataTransfer.effectAllowed = 'move';
      onDragStart(event, node.type);
    },
    [node.type, onDragStart]
  );

  const color = getCategoryColor(node.category);

  return (
    <div
      className="flex items-center gap-1.5 px-2 py-1 rounded hover:bg-gray-50 cursor-grab active:cursor-grabbing"
      draggable
      onDragStart={handleDragStart}
      title={node.description || node.name}
    >
      <span className={`w-2 h-2 rounded-sm shrink-0 ${color.swatch}`} aria-hidden="true" />
      <NodeIcon icon={node.icon} category={node.category} className={`w-3.5 h-3.5 shrink-0 ${color.softText}`} />
      <span className="text-xs flex-1 truncate">{node.name}</span>
    </div>
  );
}

interface CategorySectionProps {
  category: string;
  nodes: NodeMetadata[];
  onDragStart: (event: React.DragEvent<HTMLDivElement>, nodeType: string) => void;
  isExpanded: boolean;
  onToggle: () => void;
}

function CategorySection({ 
  category, 
  nodes, 
  onDragStart,
  isExpanded,
  onToggle 
}: CategorySectionProps) {
  const color = getCategoryColor(category);

  return (
    <div className="mb-1">
      <button
        className={`flex items-center justify-between w-full px-2 py-1.5 rounded ${color.softBg} ${color.softText} font-medium`}
        onClick={onToggle}
      >
        <div className="flex items-center gap-1.5">
          <CategoryIcon category={category} className="w-3.5 h-3.5" />
          <span className="capitalize text-xs">{category}</span>
          <span className="text-[10px] text-gray-500">{nodes.length}</span>
        </div>
        <span className="text-[10px]">{isExpanded ? '▼' : '▶'}</span>
      </button>

      {isExpanded && (
        <div className="mt-0.5 ml-3">
          {nodes.map((node) => (
            <NodePaletteItem
              key={node.id}
              node={node}
              onDragStart={onDragStart}
            />
          ))}
        </div>
      )}
    </div>
  );
}

export function NodePalette({ nodeTypes, loading }: NodePaletteProps) {
  const [expandedCategories, setExpandedCategories] = useState<Set<string>>(new Set());
  const [searchQuery, setSearchQuery] = useState('');

  const filteredNodeTypes = useMemo(() => {
    if (!searchQuery.trim()) return nodeTypes;
    const query = searchQuery.toLowerCase();
    return nodeTypes.filter((node) => 
      node.name.toLowerCase().includes(query) ||
      node.description?.toLowerCase().includes(query) ||
      node.type.toLowerCase().includes(query) ||
      node.tags?.some((tag) => tag.toLowerCase().includes(query))
    );
  }, [nodeTypes, searchQuery]);

  const filteredGroupedNodes = groupByCategory(filteredNodeTypes);
  const filteredCategories = getCategories(filteredNodeTypes);

  React.useEffect(() => {
    if (filteredCategories.length > 0 && expandedCategories.size === 0) {
      setExpandedCategories(new Set([filteredCategories[0]]));
    }
  }, [filteredCategories.length, expandedCategories.size]);

  const handleDragStart = useCallback(
    (event: React.DragEvent<HTMLDivElement>, nodeType: string) => {
      event.dataTransfer.setData('application/reactflow', JSON.stringify({ nodeType }));
    },
    []
  );

  const toggleCategory = useCallback((category: string) => {
    setExpandedCategories((prev) => {
      const newSet = new Set(prev);
      if (newSet.has(category)) {
        newSet.delete(category);
      } else {
        newSet.add(category);
      }
      return newSet;
    });
  }, []);

  if (loading) {
    return (
      <div className="p-4">
        <div className="text-sm text-gray-500">Loading node types...</div>
      </div>
    );
  }

  return (
    <div className="p-2 h-full text-xs">
      <div className="mb-2">
        <input
          type="text"
          className="w-full px-2 py-1.5 text-xs border border-gray-300 rounded bg-white focus:outline-none focus:ring-2 focus:ring-gr-blue-500 focus:border-transparent"
          placeholder="Search nodes..."
          value={searchQuery}
          onChange={(e) => setSearchQuery(e.target.value)}
        />
      </div>

      {filteredCategories.length === 0 ? (
        <div className="text-xs text-gray-500 p-2">
          {searchQuery ? 'No nodes match your search' : 'No node types available'}
        </div>
      ) : (
        <div className="space-y-0.5">
          {filteredCategories.map((category) => (
            <CategorySection
              key={category}
              category={category}
              nodes={filteredGroupedNodes[category] || []}
              onDragStart={handleDragStart}
              isExpanded={expandedCategories.has(category)}
              onToggle={() => toggleCategory(category)}
            />
          ))}
        </div>
      )}

      <div className="mt-4 pt-2 border-t border-gray-200">
        <div className="text-[10px] text-gray-400">
          Drag nodes to the canvas
        </div>
      </div>
    </div>
  );
}