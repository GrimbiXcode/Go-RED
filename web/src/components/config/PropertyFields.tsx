import { useTranslation } from 'react-i18next';
import type { Schema } from '../../types/generated';
import { groupProperties, isVisible, resolveProperties, type ResolvedProperty } from '../../schema/properties';
import { widgetRegistry } from './widgets';
import type { WidgetContext } from './types';

export type FieldErrors = Record<string, string>;

export interface PropertyFieldsProps {
  schema: Schema;
  config: Record<string, unknown>;
  onChange: (key: string, value: unknown) => void;
  errors?: FieldErrors;
  /** Error key prefix for nested schemas ("rules.0"). */
  prefix?: string;
  context: WidgetContext;
  setInvalid: (key: string, message: string | null) => void;
  compact?: boolean;
}

function Field({ prop, id, value, onChange, error, required, context, setInvalid, compact }: {
  prop: ResolvedProperty;
  id: string;
  value: unknown;
  onChange: (value: unknown) => void;
  error?: string;
  required: boolean;
  context: WidgetContext;
  setInvalid: (message: string | null) => void;
  compact?: boolean;
}) {
  const Widget = widgetRegistry[prop.widget];
  const label = (
    <label htmlFor={id} className="text-xs font-medium text-gray-600">
      {prop.label}
      {required && <span className="text-gr-fuchsia-500 ml-0.5">*</span>}
    </label>
  );
  const help = !compact && prop.schema.description ? <div className="mt-1 text-[10px] text-gray-400 leading-snug">{prop.schema.description}</div> : null;
  const errorLine = error ? (
    <div className="mt-1 text-[11px] text-gr-fuchsia-600" data-testid="field-error" data-field={id}>
      {error}
    </div>
  ) : null;

  if (prop.widget === 'boolean') {
    return (
      <div data-testid={`field-${id}`} data-widget={prop.widget}>
        <div className="flex items-center gap-2">
          <Widget id={id} property={prop.schema} value={value} onChange={onChange} setInvalid={setInvalid} error={error} context={context} />
          {label}
        </div>
        {help}
        {errorLine}
      </div>
    );
  }

  return (
    <div data-testid={`field-${id}`} data-widget={prop.widget}>
      <div className="mb-1">{label}</div>
      <Widget id={id} property={prop.schema} value={value} onChange={onChange} setInvalid={setInvalid} error={error} context={context} />
      {help}
      {errorLine}
    </div>
  );
}

/** Renders a schema's properties in groups, honoring order and visibility rules. */
export function PropertyFields({ schema, config, onChange, errors, prefix, context, setInvalid, compact }: PropertyFieldsProps) {
  const { t } = useTranslation();
  const required = new Set(schema.required || []);
  const groups = groupProperties(resolveProperties(schema).filter((prop) => isVisible(prop.schema, config, schema)));
  const listContext = errors ? ({ ...context, errors } as WidgetContext) : context;

  return (
    <div className={compact ? 'space-y-2' : 'space-y-4'}>
      {groups.map((group) => (
        <div key={group.group || '__default'} className={compact ? 'space-y-2' : 'space-y-3'}>
          {group.group && <div className="text-[10px] font-semibold uppercase tracking-wide text-gray-400 border-b border-gray-200 pb-1">{group.group}</div>}
          {group.properties.map((prop) => {
            const id = prefix ? `${prefix}.${prop.key}` : prop.key;
            return (
              <Field
                key={id}
                prop={prop}
                id={id}
                value={config[prop.key]}
                onChange={(value) => onChange(prop.key, value)}
                error={errors?.[id]}
                required={required.has(prop.key)}
                context={listContext}
                setInvalid={(message) => setInvalid(id, message)}
                compact={compact}
              />
            );
          })}
        </div>
      ))}
      {groups.length === 0 && <div className="text-xs text-gray-500">{t('config.noProperties')}</div>}
    </div>
  );
}
