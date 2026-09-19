import { useEffect, useMemo, useState, type ComponentType } from 'react';
import { useTranslation } from 'react-i18next';
import { optionsOf } from '../../schema/properties';
import type { Widget } from '../../schema/properties';
import { CodeEditor, type CodeLanguage } from './CodeEditor';
import { TypedInputWidget } from './TypedInputWidget';
import { ListWidget } from './ListWidget';
import { fieldClass, smallButtonClass, type WidgetProps } from './types';

function TextWidget({ id, property, value, onChange, error }: WidgetProps) {
  return (
    <input
      id={id}
      type="text"
      className={fieldClass(error)}
      value={value === undefined || value === null ? '' : String(value)}
      onChange={(event) => onChange(event.target.value)}
      placeholder={property.placeholder || (property.default != null && property.default !== '' ? String(property.default) : '')}
    />
  );
}

function TextareaWidget({ id, property, value, onChange, error }: WidgetProps) {
  return (
    <textarea
      id={id}
      className={fieldClass(error)}
      rows={4}
      value={value === undefined || value === null ? '' : String(value)}
      onChange={(event) => onChange(event.target.value)}
      placeholder={property.placeholder || ''}
    />
  );
}

function NumberWidget({ id, property, value, onChange, error }: WidgetProps) {
  return (
    <input
      id={id}
      type="number"
      className={fieldClass(error)}
      value={typeof value === 'number' && !Number.isNaN(value) ? value : ''}
      onChange={(event) => onChange(event.target.value === '' ? undefined : Number(event.target.value))}
      min={property.min ?? undefined}
      max={property.max ?? undefined}
      placeholder={property.placeholder || (property.default != null ? String(property.default) : '')}
    />
  );
}

function BooleanWidget({ id, value, onChange }: WidgetProps) {
  return <input id={id} type="checkbox" className="h-4 w-4 accent-gr-blue-500" checked={!!value} onChange={(event) => onChange(event.target.checked)} />;
}

function SelectWidget({ id, property, value, onChange, error }: WidgetProps) {
  const options = useMemo(() => optionsOf(property), [property]);
  const current = value === undefined || value === null ? '' : String(value);
  const known = options.some((option) => option.value === current);
  return (
    <select id={id} className={fieldClass(error)} value={current} onChange={(event) => onChange(event.target.value)}>
      {!known && <option value={current}>{current}</option>}
      {options.map((option) => (
        <option key={option.value} value={option.value}>
          {option.label}
        </option>
      ))}
    </select>
  );
}

function CredentialWidget({ id, property, value, onChange, error }: WidgetProps) {
  const { t } = useTranslation();
  const [show, setShow] = useState(false);
  return (
    <div className="flex gap-1">
      <input
        id={id}
        type={show ? 'text' : 'password'}
        className={fieldClass(error)}
        value={value === undefined || value === null ? '' : String(value)}
        onChange={(event) => onChange(event.target.value)}
        placeholder={property.placeholder || ''}
        autoComplete="off"
      />
      <button type="button" className={smallButtonClass} onClick={() => setShow((s) => !s)} aria-label={show ? t('widgets.hide') : t('widgets.show')}>
        {show ? '🙈' : '👁'}
      </button>
    </div>
  );
}

const UNIT_FACTORS: Record<string, number> = { ms: 1, s: 1000, min: 60_000, h: 3_600_000 };
const UNITS = ['ms', 's', 'min', 'h'];

function bestUnit(ms: number, stored: string): string {
  if (!ms) return stored;
  for (const unit of [...UNITS].reverse()) {
    if (ms % UNIT_FACTORS[unit] === 0) return unit;
  }
  return 'ms';
}

/** Number plus a unit picker; the value is stored in the schema's unit (ms or s). */
function DurationWidget({ id, property, value, onChange, error }: WidgetProps) {
  const { t } = useTranslation();
  const storedUnit = property.unit === 's' ? 's' : 'ms';
  const storedMs = typeof value === 'number' && !Number.isNaN(value) ? value * UNIT_FACTORS[storedUnit] : undefined;
  const [unit, setUnit] = useState(() => bestUnit(storedMs ?? 0, storedUnit));
  const shown = storedMs === undefined ? '' : storedMs / UNIT_FACTORS[unit];

  const emit = (amount: string, nextUnit: string) => {
    if (amount === '') {
      onChange(undefined);
      return;
    }
    const ms = Number(amount) * UNIT_FACTORS[nextUnit];
    onChange(ms / UNIT_FACTORS[storedUnit]);
  };

  return (
    <div className="flex gap-1" data-testid={`duration-${id}`}>
      <input id={id} type="number" className={fieldClass(error)} value={shown} min={0} onChange={(event) => emit(event.target.value, unit)} placeholder={property.placeholder || ''} />
      <select
        className={`${fieldClass(error)} !w-auto shrink-0`}
        value={unit}
        aria-label={t('widgets.unit')}
        onChange={(event) => {
          setUnit(event.target.value);
          if (shown !== '') emit(String(shown), event.target.value);
        }}
      >
        {UNITS.map((u) => (
          <option key={u} value={u}>
            {t(`widgets.units.${u}`)}
          </option>
        ))}
      </select>
    </div>
  );
}

function StringListWidget({ id, property, value, onChange, error }: WidgetProps) {
  const { t } = useTranslation();
  const items = Array.isArray(value) ? value.map((item) => String(item)) : [];
  const update = (next: string[]) => onChange(next);
  return (
    <div className="space-y-1" data-testid={`string-list-${id}`}>
      {items.map((item, index) => (
        <div key={index} className="flex gap-1">
          <input
            id={index === 0 ? id : undefined}
            type="text"
            className={fieldClass(error)}
            value={item}
            placeholder={property.placeholder || ''}
            onChange={(event) => update(items.map((it, i) => (i === index ? event.target.value : it)))}
          />
          <button type="button" className={smallButtonClass} onClick={() => update(items.filter((_, i) => i !== index))} aria-label={t('widgets.remove')}>
            ✕
          </button>
        </div>
      ))}
      <button type="button" className="text-xs text-gr-blue-600 hover:underline" onClick={() => update([...items, ''])}>
        + {t('widgets.add')}
      </button>
    </div>
  );
}

interface Pair {
  key: string;
  value: string;
}

/** Editable map[string]string; rows with an empty key are kept locally but not stored. */
function KeyValueWidget({ id, value, onChange, error }: WidgetProps) {
  const { t } = useTranslation();
  const [rows, setRows] = useState<Pair[]>(() =>
    value && typeof value === 'object' ? Object.entries(value as Record<string, unknown>).map(([key, v]) => ({ key, value: String(v ?? '') })) : []
  );

  const update = (next: Pair[]) => {
    setRows(next);
    const object: Record<string, string> = {};
    for (const row of next) if (row.key.trim() !== '') object[row.key] = row.value;
    onChange(object);
  };

  return (
    <div className="space-y-1" data-testid={`key-value-${id}`}>
      {rows.map((row, index) => (
        <div key={index} className="flex gap-1">
          <input
            id={index === 0 ? id : undefined}
            type="text"
            className={fieldClass(error)}
            value={row.key}
            placeholder={t('widgets.key')}
            onChange={(event) => update(rows.map((r, i) => (i === index ? { ...r, key: event.target.value } : r)))}
          />
          <input
            type="text"
            className={fieldClass(error)}
            value={row.value}
            placeholder={t('widgets.value')}
            onChange={(event) => update(rows.map((r, i) => (i === index ? { ...r, value: event.target.value } : r)))}
          />
          <button type="button" className={smallButtonClass} onClick={() => update(rows.filter((_, i) => i !== index))} aria-label={t('widgets.remove')}>
            ✕
          </button>
        </div>
      ))}
      <button type="button" className="text-xs text-gr-blue-600 hover:underline" onClick={() => update([...rows, { key: '', value: '' }])}>
        + {t('widgets.add')}
      </button>
    </div>
  );
}

/** Picks another node of the open flow (config nodes, link targets, scopes). */
function NodeSelectWidget({ id, property, value, onChange, error, context }: WidgetProps) {
  const { t } = useTranslation();
  const candidates = useMemo(() => {
    const nodes = Object.values(context.flow?.nodes || {}).filter((node) => node.id !== context.nodeId);
    const types = property.nodeTypes || [];
    const filtered = types.length > 0 ? nodes.filter((node) => types.includes(node.type)) : nodes;
    return filtered
      .map((node) => {
        const typeName = context.nodeTypes.find((nt) => nt.type === node.type)?.name || node.type;
        return { id: node.id, label: node.name ? `${node.name} (${typeName})` : `${typeName} · ${node.id.slice(0, 8)}` };
      })
      .sort((a, b) => a.label.localeCompare(b.label));
  }, [context.flow, context.nodeId, context.nodeTypes, property.nodeTypes]);

  if (property.multiple) {
    const selected = new Set(Array.isArray(value) ? value.map(String) : []);
    return (
      <div className="space-y-1 max-h-40 overflow-y-auto border border-gray-200 rounded p-2" data-testid={`node-select-${id}`}>
        {candidates.length === 0 && <div className="text-xs text-gray-400">{t('widgets.noNodes')}</div>}
        {candidates.map((candidate) => (
          <label key={candidate.id} className="flex items-center gap-2 text-xs text-gray-700">
            <input
              type="checkbox"
              className="h-3.5 w-3.5 accent-gr-blue-500"
              checked={selected.has(candidate.id)}
              onChange={(event) => {
                const next = new Set(selected);
                if (event.target.checked) next.add(candidate.id);
                else next.delete(candidate.id);
                onChange(candidates.filter((c) => next.has(c.id)).map((c) => c.id));
              }}
            />
            <span className="truncate">{candidate.label}</span>
          </label>
        ))}
      </div>
    );
  }

  const current = value === undefined || value === null ? '' : String(value);
  const known = current === '' || candidates.some((c) => c.id === current);
  return (
    <select id={id} className={fieldClass(error)} value={current} onChange={(event) => onChange(event.target.value)} data-testid={`node-select-${id}`}>
      <option value="">{t('widgets.none')}</option>
      {!known && <option value={current}>{current}</option>}
      {candidates.map((candidate) => (
        <option key={candidate.id} value={candidate.id}>
          {candidate.label}
        </option>
      ))}
    </select>
  );
}

function CodeWidget({ id, property, value, onChange }: WidgetProps) {
  const language = (property.language || 'text') as CodeLanguage;
  return <CodeEditor id={id} language={language} value={value === undefined || value === null ? '' : String(value)} onChange={onChange} placeholder={property.placeholder} aria-label={property.label || id} />;
}

/** JSON editor for a whole object or array; the parsed value is stored, parse errors block Done. */
function JsonWidget({ id, property, value, onChange, setInvalid }: WidgetProps) {
  const { t } = useTranslation();
  const [text, setText] = useState(() => (value === undefined ? '' : JSON.stringify(value, null, 2)));

  useEffect(() => () => setInvalid(null), [setInvalid]);

  const handleChange = (next: string) => {
    setText(next);
    if (next.trim() === '') {
      setInvalid(null);
      onChange(undefined);
      return;
    }
    try {
      onChange(JSON.parse(next));
      setInvalid(null);
    } catch {
      setInvalid(t('validation.json'));
    }
  };

  return <CodeEditor id={id} language="json" value={text} onChange={handleChange} placeholder={property.placeholder || (property.type === 'array' ? '[]' : '{}')} minHeight="5rem" aria-label={property.label || id} />;
}

export const widgetRegistry: Record<Widget, ComponentType<WidgetProps>> = {
  text: TextWidget,
  textarea: TextareaWidget,
  number: NumberWidget,
  boolean: BooleanWidget,
  select: SelectWidget,
  typedInput: TypedInputWidget,
  code: CodeWidget,
  list: ListWidget,
  keyValue: KeyValueWidget,
  credential: CredentialWidget,
  duration: DurationWidget,
  json: JsonWidget,
  stringList: StringListWidget,
  nodeSelect: NodeSelectWidget,
};
