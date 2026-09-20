import React, { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { ChevronDown, ChevronRight, Search } from 'lucide-react';
import type { NodeMetadata } from '../types/node';
import { NodeIcon } from './CategoryIcon';
import { getCategoryColor, sortCategories } from '../utils/nodeCategories';
import { useFlowStore } from '../store/flowStore';

const COLLAPSED_KEY = 'go-red.palette.collapsed';

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

function readCollapsed(): Set<string> {
  try {
    const raw = window.localStorage.getItem(COLLAPSED_KEY);
    const parsed: unknown = raw ? JSON.parse(raw) : [];
    return new Set(Array.isArray(parsed) ? parsed.filter((item): item is string => typeof item === 'string') : []);
  } catch {
    return new Set();
  }
}

function writeCollapsed(collapsed: Set<string>): void {
  try {
    window.localStorage.setItem(COLLAPSED_KEY, JSON.stringify(Array.from(collapsed)));
  } catch {
    // Not remembered, still applied for this session.
  }
}

function isEditableTarget(target: EventTarget | null): boolean {
  if (!(target instanceof HTMLElement)) return false;
  const tag = target.tagName;
  return tag === 'INPUT' || tag === 'TEXTAREA' || tag === 'SELECT' || target.isContentEditable;
}

/** A palette row is a miniature of the node: icon well in the category color plus the label. */
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
      className="flex items-center h-7 rounded-md border border-line bg-panel shadow-panel hover:border-line-strong hover:shadow-float cursor-grab active:cursor-grabbing overflow-hidden"
      draggable
      onDragStart={handleDragStart}
      title={node.description || node.name}
      data-testid={`palette-node-${node.type}`}
    >
      <span className="flex items-center justify-center w-7 h-full text-white shrink-0" style={{ background: node.color || color.fill }}>
        <NodeIcon icon={node.icon} category={node.category} className="w-3.5 h-3.5" />
      </span>
      <span className="px-2 text-xs text-fg truncate flex-1">{node.name}</span>
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
  const { t } = useTranslation();
  const color = getCategoryColor(category);
  const Chevron = isExpanded ? ChevronDown : ChevronRight;

  return (
    <div className="mb-2">
      <button
        className="flex items-center w-full gap-2 px-1 py-1 rounded text-xs font-medium text-fg hover:bg-surface"
        onClick={onToggle}
        aria-expanded={isExpanded}
        title={isExpanded ? t('palette.collapse') : t('palette.expand')}
      >
        <span className="w-2.5 h-2.5 rounded-sm shrink-0" style={{ background: color.fill }} aria-hidden="true" />
        <span className="capitalize">{category}</span>
        <span className="text-2xs text-faint ml-auto">{nodes.length}</span>
        <Chevron className="w-3.5 h-3.5 text-faint" aria-hidden="true" />
      </button>

      {isExpanded && (
        <div className="mt-1 space-y-1 pl-1">
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

/**
 * Node palette: categories with a color chip, all open unless the user
 * collapsed them (remembered per browser), rows as mini nodes, and a
 * search box that "/" focuses from anywhere in the editor.
 */
export function NodePalette(props: NodePaletteProps) {
  const { t } = useTranslation();
  const storeNodeTypes = useFlowStore((state) => state.nodeTypes);
  const storeLoading = useFlowStore((state) => state.nodeTypesLoading);
  const nodeTypes = props.nodeTypes ?? storeNodeTypes;
  const loading = props.loading ?? storeLoading;

  const [collapsed, setCollapsed] = useState<Set<string>>(() => readCollapsed());
  const [searchQuery, setSearchQuery] = useState('');
  const searchRef = useRef<HTMLInputElement>(null);

  const filteredNodeTypes = useMemo(() => filterNodeTypes(nodeTypes, searchQuery), [nodeTypes, searchQuery]);
  const filteredGroupedNodes = useMemo(() => groupByCategory(filteredNodeTypes), [filteredNodeTypes]);
  const filteredCategories = useMemo(() => getCategories(filteredNodeTypes), [filteredNodeTypes]);
  const searching = searchQuery.trim().length > 0;

  useEffect(() => {
    const onKeyDown = (event: KeyboardEvent) => {
      const mod = event.ctrlKey || event.metaKey;
      const slash = event.key === '/' && !mod && !event.altKey && !isEditableTarget(event.target);
      const find = mod && !event.shiftKey && !event.altKey && event.key.toLowerCase() === 'f';
      if (!slash && !find) return;
      event.preventDefault();
      searchRef.current?.focus();
      searchRef.current?.select();
    };
    window.addEventListener('keydown', onKeyDown);
    return () => window.removeEventListener('keydown', onKeyDown);
  }, []);

  const toggleCategory = useCallback((category: string) => {
    setCollapsed((prev) => {
      const next = new Set(prev);
      if (next.has(category)) next.delete(category);
      else next.add(category);
      writeCollapsed(next);
      return next;
    });
  }, []);

  if (loading && nodeTypes.length === 0) {
    return (
      <div className="p-4">
        <div className="text-sm text-muted">{t('palette.loading')}</div>
      </div>
    );
  }

  return (
    <div className="p-2 h-full text-xs flex flex-col" data-testid="node-palette">
      <div className="relative mb-3">
        <Search className="absolute left-2 top-1/2 -translate-y-1/2 w-3.5 h-3.5 text-faint pointer-events-none" aria-hidden="true" />
        <input
          ref={searchRef}
          type="search"
          className="w-full pl-7 pr-8 py-1.5 text-xs border border-line rounded-md bg-panel text-fg placeholder:text-faint focus:outline-none focus:ring-2 focus:ring-accent focus:border-transparent"
          placeholder={t('palette.search')}
          aria-label={t('palette.search')}
          value={searchQuery}
          onChange={(event) => setSearchQuery(event.target.value)}
          onKeyDown={(event) => {
            if (event.key === 'Escape') {
              setSearchQuery('');
              (event.target as HTMLInputElement).blur();
            }
          }}
        />
        {!searching && (
          <kbd className="absolute right-2 top-1/2 -translate-y-1/2 px-1 rounded border border-line text-2xs text-faint font-mono" title={t('palette.shortcut')}>
            /
          </kbd>
        )}
      </div>

      {filteredCategories.length === 0 ? (
        <div className="text-xs text-muted p-2">{searching ? t('palette.noMatch') : t('palette.noNodes')}</div>
      ) : (
        <div className="flex-1 overflow-y-auto">
          {filteredCategories.map((category) => (
            <CategorySection
              key={category}
              category={category}
              nodes={filteredGroupedNodes[category] || []}
              isExpanded={searching || !collapsed.has(category)}
              onToggle={() => toggleCategory(category)}
            />
          ))}
        </div>
      )}

      <div className="mt-2 pt-2 border-t border-line">
        <div className="text-2xs text-faint">{t('palette.hint')}</div>
      </div>
    </div>
  );
}
