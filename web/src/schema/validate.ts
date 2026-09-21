import type { Schema } from '../types/generated';
import { isVisible, optionsOf, resolveProperties, type ResolvedProperty } from './properties';

/** Field errors keyed by property; list items use "rules.0.value". */
export type ValidationErrors = Record<string, string>;

export type Translate = (key: string, options?: Record<string, unknown>) => string;

/** True when a value counts as "not entered" for the given control. */
export function isEmptyValue(value: unknown, widget?: string): boolean {
  if (value === undefined || value === null) return true;
  if (typeof value === 'string') return value.trim() === '';
  if (Array.isArray(value)) return value.length === 0;
  if (typeof value === 'object') {
    if (widget === 'typedInput') {
      const typed = value as { type?: string; value?: unknown; path?: unknown };
      if (!typed.type) return true;
      const inner = typed.path !== undefined ? typed.path : typed.value;
      return inner === undefined || inner === null || String(inner).trim() === '';
    }
    return Object.keys(value as object).length === 0;
  }
  return false;
}

function validateValue(prop: ResolvedProperty, value: unknown, t: Translate): string | undefined {
  const { schema, widget } = prop;
  switch (widget) {
    case 'number':
    case 'duration': {
      if (typeof value !== 'number' || Number.isNaN(value)) return t('validation.number');
      if (schema.min !== undefined && schema.min !== null && value < schema.min) return t('validation.min', { min: schema.min });
      if (schema.max !== undefined && schema.max !== null && value > schema.max) return t('validation.max', { max: schema.max });
      return undefined;
    }
    case 'select': {
      const options = optionsOf(schema);
      if (options.length > 0 && !options.some((o) => o.value === String(value))) return t('validation.oneOf');
      return undefined;
    }
    case 'typedInput': {
      const typed = value as { type?: string; value?: unknown };
      if (typed.type === 'num' && Number.isNaN(Number(typed.value))) return t('validation.number');
      if (typed.type === 'json') {
        try {
          JSON.parse(String(typed.value));
        } catch {
          return t('validation.json');
        }
      }
      return undefined;
    }
    case 'keyValue': {
      if (typeof value === 'object' && Object.keys(value as object).some((key) => key.trim() === '')) return t('validation.keyRequired');
      return undefined;
    }
    default: {
      if (typeof value === 'string' && schema.pattern) {
        try {
          if (!new RegExp(schema.pattern).test(value)) return t('validation.pattern', { pattern: schema.pattern });
        } catch {
          // An invalid pattern in a schema never blocks the user.
        }
      }
      return undefined;
    }
  }
}

/**
 * Checks a config against its schema the way the tray does: required
 * properties present (hidden ones are skipped), numbers within min/max,
 * select values known, typed inputs parseable, list items valid.
 */
export function validateConfig(schema: Schema | null | undefined, config: Record<string, unknown>, t: Translate): ValidationErrors {
  const errors: ValidationErrors = {};
  if (!schema) return errors;
  const required = new Set(schema.required || []);
  for (const prop of resolveProperties(schema)) {
    if (!isVisible(prop.schema, config, schema)) continue;
    const value = config[prop.key];
    const empty = isEmptyValue(value, prop.widget);
    if (required.has(prop.key) && empty) {
      errors[prop.key] = t('validation.required');
      continue;
    }
    if (empty) continue;
    const message = validateValue(prop, value, t);
    if (message) errors[prop.key] = message;
    if (prop.widget === 'list' && prop.schema.items && Array.isArray(value)) {
      value.forEach((item, index) => {
        const nested = validateConfig(prop.schema.items, (item || {}) as Record<string, unknown>, t);
        for (const [key, msg] of Object.entries(nested)) errors[`${prop.key}.${index}.${key}`] = msg;
      });
    }
  }
  return errors;
}
