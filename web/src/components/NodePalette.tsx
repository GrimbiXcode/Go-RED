import React, { useCallback, useState, useMemo } from 'react';
import type { NodeMetadata, NodeCategory } from '../types/node';
import DOMPurify from 'dompurify';

interface NodePaletteProps {
  nodeTypes: NodeMetadata[];
  loading: boolean;
}

const categoryIcons: Record<NodeCategory, string> = {
  input: '📥',
  output: '📤',
  function: '🔄',
  storage: '💾',
  network: '🌐',
  protocol: '🔌',
  parser: '📋',
  social: '💬',
  dashboard: '📊',
  custom: '⚙️',
};

const categoryColors: Record<NodeCategory, string> = {
  input: 'bg-blue-100 text-blue-600',
  output: 'bg-green-100 text-green-600',
  function: 'bg-purple-100 text-purple-600',
  storage: 'bg-orange-100 text-orange-600',
  network: 'bg-cyan-100 text-cyan-600',
  protocol: 'bg-indigo-100 text-indigo-600',
  parser: 'bg-pink-100 text-pink-600',
  social: 'bg-rose-100 text-rose-600',
  dashboard: 'bg-teal-100 text-teal-600',
  custom: 'bg-gray-100 text-gray-600',
};

function getCategories(nodeTypes: NodeMetadata[]): NodeCategory[] {
  const categories = new Set<NodeCategory>();
  nodeTypes.forEach((node) => {
    categories.add(node.category);
  });
  return Array.from(categories).sort();
}

function groupByCategory(nodeTypes: NodeMetadata[]): Record<NodeCategory, NodeMetadata[]> {
  const grouped: Record<NodeCategory, NodeMetadata[]> = {} as Record<NodeCategory, NodeMetadata[]>;
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

  return (
    <div
      className="flex items-center gap-2 p-2 rounded hover:bg-gray-50 cursor-grab active:cursor-grabbing"
      draggable
      onDragStart={handleDragStart}
      title={node.description || node.name}
    >
      {node.icon && node.icon.startsWith('<svg') ? (
        <span 
          className="text-lg"
          dangerouslySetInnerHTML={{ __html: DOMPurify.sanitize(node.icon) }}
        />
      ) : (
        <span className="text-lg">{node.icon || '⚙️'}</span>
      )}
      <span className="text-sm flex-1">{node.name}</span>
    </div>
  );
}

interface CategorySectionProps {
  category: NodeCategory;
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
  const colorClass = categoryColors[category] || 'bg-gray-100 text-gray-600';
  const icon = categoryIcons[category] || '⚙️';

  return (
    <div className="mb-2">
      <button
        className={`flex items-center justify-between w-full p-2 rounded ${colorClass} font-medium`}
        onClick={onToggle}
      >
        <div className="flex items-center gap-2">
          {icon.startsWith('<svg') ? (
            <span 
              dangerouslySetInnerHTML={{ __html: DOMPurify.sanitize(icon) }}
            />
          ) : (
            <span>{icon}</span>
          )}
          <span className="capitalize text-sm">{category}</span>
          <span className="text-xs text-gray-500">{nodes.length}</span>
        </div>
        <span className="text-sm">{isExpanded ? '▼' : '▶'}</span>
      </button>
      
      {isExpanded && (
        <div className="mt-1 ml-4">
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
  const [expandedCategories, setExpandedCategories] = useState<Set<NodeCategory>>(new Set());
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

  const toggleCategory = useCallback((category: NodeCategory) => {
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
    <div className="p-2 h-full">
      <div className="text-sm font-semibold text-gray-700 mb-2">Node Palette</div>
      
      <div className="mb-2">
        <input
          type="text"
          className="w-full p-2 text-sm border border-gray-300 rounded bg-white focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-transparent"
          placeholder="Search nodes..."
          value={searchQuery}
          onChange={(e) => setSearchQuery(e.target.value)}
        />
      </div>
      
      {filteredCategories.length === 0 ? (
        <div className="text-sm text-gray-500 p-2">
          {searchQuery ? 'No nodes match your search' : 'No node types available'}
        </div>
      ) : (
        <div className="space-y-1">
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
        <div className="text-xs text-gray-400">
          Drag nodes to the canvas
        </div>
      </div>
    </div>
  );
}