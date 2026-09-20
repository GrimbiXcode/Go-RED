import { useEffect, useMemo, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Search } from 'lucide-react';
import type { NodeMetadata } from '../types/node';
import { filterNodeTypes } from './NodePalette';
import { NodeIcon } from './CategoryIcon';
import { getCategoryColor } from '../utils/nodeCategories';

export interface QuickAddProps {
  x: number;
  y: number;
  nodeTypes: NodeMetadata[];
  onPick: (type: string) => void;
  onClose: () => void;
}

/** Popover opened by double-clicking the canvas: type to search, Enter or click to add a node there. */
export function QuickAdd({ x, y, nodeTypes, onPick, onClose }: QuickAddProps) {
  const { t } = useTranslation();
  const [query, setQuery] = useState('');
  const [active, setActive] = useState(0);
  const ref = useRef<HTMLDivElement>(null);
  const input = useRef<HTMLInputElement>(null);

  const matches = useMemo(() => filterNodeTypes(nodeTypes, query).slice(0, 8), [nodeTypes, query]);

  useEffect(() => {
    input.current?.focus();
    const onDown = (event: MouseEvent) => {
      if (ref.current && !ref.current.contains(event.target as Node)) onClose();
    };
    document.addEventListener('mousedown', onDown);
    return () => document.removeEventListener('mousedown', onDown);
  }, [onClose]);

  useEffect(() => setActive(0), [query]);

  const width = 288;
  const left = Math.min(x, (typeof window !== 'undefined' ? window.innerWidth : x + width) - width - 8);
  const top = Math.min(y, (typeof window !== 'undefined' ? window.innerHeight : y + 320) - 320);

  return (
    <div
      ref={ref}
      className="fixed z-40 w-72 bg-panel text-fg rounded-md shadow-float border border-line animate-fade-in"
      style={{ left: Math.max(4, left), top: Math.max(4, top) }}
      data-testid="quick-add"
      role="dialog"
      aria-label={t('quickAdd.title')}
    >
      <div className="relative border-b border-line">
        <Search className="absolute left-2 top-1/2 -translate-y-1/2 w-3.5 h-3.5 text-faint" aria-hidden="true" />
        <input
          ref={input}
          type="text"
          className="w-full pl-7 pr-2 py-2 text-xs bg-transparent text-fg placeholder:text-faint focus:outline-none"
          placeholder={t('quickAdd.placeholder')}
          aria-label={t('quickAdd.placeholder')}
          value={query}
          onChange={(event) => setQuery(event.target.value)}
          onKeyDown={(event) => {
            if (event.key === 'Escape') onClose();
            else if (event.key === 'ArrowDown') {
              event.preventDefault();
              setActive((a) => Math.min(a + 1, matches.length - 1));
            } else if (event.key === 'ArrowUp') {
              event.preventDefault();
              setActive((a) => Math.max(a - 1, 0));
            } else if (event.key === 'Enter' && matches[active]) {
              event.preventDefault();
              onPick(matches[active].type);
            }
          }}
        />
      </div>
      <div className="max-h-64 overflow-y-auto py-1" role="listbox">
        {matches.length === 0 && <div className="px-3 py-2 text-xs text-muted">{t('palette.noMatch')}</div>}
        {matches.map((node, index) => {
          const color = getCategoryColor(node.category);
          return (
            <button
              key={node.type}
              role="option"
              aria-selected={index === active}
              className={`w-full flex items-center gap-2 px-2 py-1.5 text-left text-xs ${index === active ? 'bg-accent-soft' : 'hover:bg-surface'}`}
              onMouseEnter={() => setActive(index)}
              onClick={() => onPick(node.type)}
              data-testid={`quick-add-${node.type}`}
            >
              <span className="flex items-center justify-center w-5 h-5 rounded text-white shrink-0" style={{ background: node.color || color.fill }}>
                <NodeIcon icon={node.icon} category={node.category} className="w-3 h-3" />
              </span>
              <span className="flex-1 truncate text-fg">{node.name}</span>
              <span className="text-2xs text-faint capitalize">{node.category}</span>
            </button>
          );
        })}
      </div>
    </div>
  );
}
