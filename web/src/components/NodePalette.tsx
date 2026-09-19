import React, { useCallback, useState, useMemo } from 'react';
import { useTranslation } from 'react-i18next';
import type { NodeMetadata } from '../types/node';
import { CategoryIcon, NodeIcon } from './CategoryIcon';
import { getCategoryColor, sortCategories } from '../utils/nodeCategories';
import { useFlowStore } from '../store/flowStore';

export function getCategories(nodeTypes: NodeMetadata[]): string[] {
  const categories = new Set<string>();
  nodeTypes.forEach((node) => categories.add(node.category));
  return sortCategories(Array.from(categories));
}

export function groupByCategory(nodeTypes: NodeMetadata[]): Record<string, NodeMetadata[]> {
  const grouped: Record<string, NodeMetadata[]> = {};
  getCategories(nodeTypes).forEach((category) => {
    grouped[category] = [];
  });
  nodeTypes.forEach((node) => {
    if (grouped[node.category]) grouped[node.category].push(node);
  });
  return grouped;
}

/** Filters node types by name, type, description or tag. */
export function filterNodeTypes(nodeTypes: NodeMetadata[], query: string): NodeMetadata[] {
  const needle = query.trim().toLowerCase();
  if (!needle) return nodeTypes;
  return nodeTypes.filter(
    (node) =>
      node.name.toLowerCase().includes(needle) ||
      node.description?.toLowerCase().includes(needle) ||
      node.type.toLowerCase().includes(needle) ||
      node.tags?.some((tag) => tag.toLowerCase().includes(needle))
  );
}

function NodePaletteItem({ node }: { node: NodeMetadata }) {
  const handleDragStart = useCallback(
    (event: React.DragEvent<HTMLDivElement>) => {
      event.dataTransfer.setData('application/reactflow', JSON.stringify({ nodeType: node.type }));
      event.dataTransfer.effectAllowed = 'move';
    },
    [node.type]
  );

  const color = getCategoryColor(node.category);

  return (
    <div
      className="flex items-center gap-1.5 px-2 py-1 rounded hover:bg-gray-50 cursor-grab active:cursor-grabbing"
      draggable
      onDragStart={handleDragStart}
      title={node.description || node.name}
      data-testid={`palette-node-${node.type}`}
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
  isExpanded: boolean;
  onToggle: () => void;
}

function CategorySection({ category, nodes, isExpanded, onToggle }: CategorySectionProps) {
  const color = getCategoryColor(category);

  return (
    <div className="mb-1">
      <button
        className={`flex items-center justify-between w-full px-2 py-1.5 rounded ${color.softBg} ${color.softText} font-medium`}
        onClick={onToggle}
        aria-expanded={isExpanded}
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
            <NodePaletteItem key={node.id} node={node} />
          ))}
        </div>
      )}
    </div>
  );
}

export interface NodePaletteProps {
  /** Overrides the store (for tests and the style guide). */
  nodeTypes?: NodeMetadata[];
  loading?: boolean;
}

export function NodePalette(props: NodePaletteProps) {
  const { t } = useTranslation();
  const storeNodeTypes = useFlowStore((state) => state.nodeTypes);
  const storeLoading = useFlowStore((state) => state.nodeTypesLoading);
  const nodeTypes = props.nodeTypes ?? storeNodeTypes;
  const loading = props.loading ?? storeLoading;

  const [expandedCategories, setExpandedCategories] = useState<Set<string>>(new Set());
  const [searchQuery, setSearchQuery] = useState('');

  const filteredNodeTypes = useMemo(() => filterNodeTypes(nodeTypes, searchQuery), [nodeTypes, searchQuery]);
  const filteredGroupedNodes = useMemo(() => groupByCategory(filteredNodeTypes), [filteredNodeTypes]);
  const filteredCategories = useMemo(() => getCategories(filteredNodeTypes), [filteredNodeTypes]);
  const searching = searchQuery.trim().length > 0;

  React.useEffect(() => {
    if (filteredCategories.length > 0 && expandedCategories.size === 0) {
      setExpandedCategories(new Set([filteredCategories[0]]));
    }
  }, [filteredCategories, expandedCategories.size]);

  const toggleCategory = useCallback((category: string) => {
    setExpandedCategories((prev) => {
      const next = new Set(prev);
      if (next.has(category)) next.delete(category);
      else next.add(category);
      return next;
    });
  }, []);

  if (loading && nodeTypes.length === 0) {
    return (
      <div className="p-4">
        <div className="text-sm text-gray-500">{t('palette.loading')}</div>
      </div>
    );
  }

  return (
    <div className="p-2 h-full text-xs" data-testid="node-palette">
      <div className="mb-2">
        <input
          type="search"
          className="w-full px-2 py-1.5 text-xs border border-gray-300 rounded bg-white focus:outline-none focus:ring-2 focus:ring-gr-blue-500 focus:border-transparent"
          placeholder={t('palette.search')}
          aria-label={t('palette.search')}
          value={searchQuery}
          onChange={(event) => setSearchQuery(event.target.value)}
        />
      </div>

      {filteredCategories.length === 0 ? (
        <div className="text-xs text-gray-500 p-2">{searching ? t('palette.noMatch') : t('palette.noNodes')}</div>
      ) : (
        <div className="space-y-0.5">
          {filteredCategories.map((category) => (
            <CategorySection
              key={category}
              category={category}
              nodes={filteredGroupedNodes[category] || []}
              isExpanded={searching || expandedCategories.has(category)}
              onToggle={() => toggleCategory(category)}
            />
          ))}
        </div>
      )}

      <div className="mt-4 pt-2 border-t border-gray-200">
        <div className="text-[10px] text-gray-400">{t('palette.hint')}</div>
      </div>
    </div>
  );
}
