import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { withDefaults } from '../../schema/properties';
import { PropertyFields, type FieldErrors } from './PropertyFields';
import { smallButtonClass, type WidgetProps } from './types';

/**
 * Ordered list of objects, each edited with the property's item schema
 * (Switch rules, Change rules). Items can be added, removed, moved with
 * the arrow buttons or dragged into place.
 */
export function ListWidget({ id, property, value, onChange, error, context, setInvalid }: WidgetProps & { errors?: FieldErrors }) {
  const { t } = useTranslation();
  const items = Array.isArray(value) ? (value as Record<string, unknown>[]) : [];
  const [dragIndex, setDragIndex] = useState<number | null>(null);
  const itemSchema = property.items;

  const update = (next: Record<string, unknown>[]) => onChange(next);
  const move = (from: number, to: number) => {
    if (to < 0 || to >= items.length || from === to) return;
    const next = [...items];
    const [moved] = next.splice(from, 1);
    next.splice(to, 0, moved);
    update(next);
  };

  return (
    <div className="space-y-2" data-testid={`list-${id}`}>
      {items.map((item, index) => (
        <div
          key={index}
          className={`rounded border border-line bg-surface p-2 ${dragIndex === index ? 'opacity-50' : ''}`}
          draggable
          onDragStart={() => setDragIndex(index)}
          onDragOver={(event) => event.preventDefault()}
          onDrop={() => {
            if (dragIndex !== null) move(dragIndex, index);
            setDragIndex(null);
          }}
          onDragEnd={() => setDragIndex(null)}
          data-testid={`list-item-${id}-${index}`}
        >
          <div className="flex items-center gap-1 mb-1 text-2xs text-muted">
            <span className="cursor-grab select-none" title={t('widgets.drag')}>
              ⋮⋮
            </span>
            <span className="flex-1">{t('widgets.itemN', { n: index + 1 })}</span>
            <button type="button" className={smallButtonClass} onClick={() => move(index, index - 1)} disabled={index === 0} aria-label={t('widgets.moveUp')}>
              ↑
            </button>
            <button type="button" className={smallButtonClass} onClick={() => move(index, index + 1)} disabled={index === items.length - 1} aria-label={t('widgets.moveDown')}>
              ↓
            </button>
            <button type="button" className={smallButtonClass} onClick={() => update(items.filter((_, i) => i !== index))} aria-label={t('widgets.remove')}>
              ✕
            </button>
          </div>
          {itemSchema ? (
            <PropertyFields
              schema={itemSchema}
              config={item || {}}
              onChange={(key, nextValue) => update(items.map((it, i) => (i === index ? { ...it, [key]: nextValue } : it)))}
              errors={(context as { errors?: FieldErrors }).errors}
              prefix={`${id}.${index}`}
              context={context}
              setInvalid={setInvalid}
              compact
            />
          ) : (
            <pre className="text-2xs font-mono">{JSON.stringify(item)}</pre>
          )}
        </div>
      ))}
      {error && items.length === 0 && <div className="text-2xs text-danger-text">{error}</div>}
      <button
        type="button"
        className="text-xs text-accent-text hover:underline"
        onClick={() => update([...items, itemSchema ? withDefaults(itemSchema, {}) : {}])}
        data-testid={`list-add-${id}`}
      >
        + {t('widgets.add')}
      </button>
    </div>
  );
}
