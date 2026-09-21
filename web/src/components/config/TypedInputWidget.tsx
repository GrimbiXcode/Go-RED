import { useTranslation } from 'react-i18next';
import { isRefType } from '../../schema/properties';
import { fieldClass, type WidgetProps } from './types';

interface TypedValue {
  type: string;
  value?: string;
  path?: string;
}

const ALL_TYPES = ['msg', 'flow', 'global', 'str', 'num', 'bool', 'json', 'env'];

function normalize(raw: unknown, fallbackType: string): TypedValue {
  const typed = (raw && typeof raw === 'object' ? raw : {}) as Partial<TypedValue>;
  const type = typed.type || fallbackType;
  const text = typed.path !== undefined ? String(typed.path) : typed.value !== undefined ? String(typed.value) : '';
  return isRefType(type) ? { type, path: text } : { type, value: text };
}

/**
 * Node-RED's typed input: a type picker (msg., flow., global., string,
 * number, boolean, JSON, env) next to the value. Reference types are
 * stored as {type, path}, literal types as {type, value}.
 */
export function TypedInputWidget({ id, property, value, onChange, error }: WidgetProps) {
  const { t } = useTranslation();
  const types = (property.typedInput?.types || ALL_TYPES).filter((type) => ALL_TYPES.includes(type));
  const current = normalize(value, property.typedInput?.default || types[0] || 'str');
  const text = current.path ?? current.value ?? '';

  const emit = (type: string, nextText: string) => {
    onChange(isRefType(type) ? { type, path: nextText } : { type, value: nextText });
  };

  return (
    <div className="flex gap-1" data-testid={`typed-input-${id}`}>
      <select
        className={`${fieldClass(error)} !w-auto shrink-0 font-mono`}
        value={current.type}
        onChange={(event) => emit(event.target.value, text)}
        aria-label={t('widgets.typedType')}
      >
        {types.map((type) => (
          <option key={type} value={type}>
            {t(`widgets.typed.${type}`)}
          </option>
        ))}
      </select>
      {current.type === 'bool' ? (
        <select id={id} className={fieldClass(error)} value={text === 'true' ? 'true' : 'false'} onChange={(event) => emit(current.type, event.target.value)}>
          <option value="true">{t('widgets.true')}</option>
          <option value="false">{t('widgets.false')}</option>
        </select>
      ) : current.type === 'json' ? (
        <textarea id={id} className={`${fieldClass(error)} font-mono`} rows={2} value={text} onChange={(event) => emit(current.type, event.target.value)} spellCheck={false} />
      ) : (
        <input
          id={id}
          type="text"
          className={`${fieldClass(error)} ${isRefType(current.type) ? 'font-mono' : ''}`}
          value={text}
          placeholder={isRefType(current.type) ? 'payload' : property.placeholder || ''}
          onChange={(event) => emit(current.type, event.target.value)}
        />
      )}
    </div>
  );
}
