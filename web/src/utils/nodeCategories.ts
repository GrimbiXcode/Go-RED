/**
 * Node categories: order and colors, shared by the palette, the canvas,
 * the tray and the style guide (docs/NEXT_LEVEL_PLAN.md, Phase 4).
 *
 * Every category has one fill color; the node label is white on it, so
 * every value keeps at least 4.5:1 against white (src/test/tokens.test.ts
 * checks this and that src/styles/tailwind.css carries the same values).
 * Soft backgrounds and text tints for panels are derived per theme in CSS
 * (--cat-<name>-soft / --cat-<name>-text).
 */

export const CATEGORY_ORDER = [
  'input',
  'output',
  'function',
  'flow-control',
  'social',
  'storage',
  'network',
  'protocol',
  'parser',
  'config',
  'dashboard',
  'custom',
] as const;

export type KnownCategory = (typeof CATEGORY_ORDER)[number];

/** Fill color per category; the values behind the --cat-* variables. */
export const CATEGORY_FILLS: Record<KnownCategory, string> = {
  'input': '#0c7f9c',
  'output': '#2e8157',
  'function': '#5b4fcf',
  'flow-control': '#9d681b',
  'social': '#b83280',
  'storage': '#4a5568',
  'network': '#7c3aed',
  'protocol': '#0f766e',
  'parser': '#c05621',
  'config': '#6b7280',
  'dashboard': '#0369a1',
  'custom': '#64748b',
};

export interface CategoryColor {
  /** Solid fill, e.g. for a small color chip. */
  swatch: string;
  /** Light tint for a category header row. */
  softBg: string;
  /** Text color to pair with `softBg`. */
  softText: string;
  /** CSS expression of the fill, for inline styles (`--node-color`). */
  fill: string;
}

// Class names are spelled out so Tailwind's scanner generates them.
export const CATEGORY_COLORS: Record<string, CategoryColor> = {
  'input': { swatch: 'bg-cat-input', softBg: 'bg-cat-input-soft', softText: 'text-cat-input-text', fill: 'var(--cat-input)' },
  'output': { swatch: 'bg-cat-output', softBg: 'bg-cat-output-soft', softText: 'text-cat-output-text', fill: 'var(--cat-output)' },
  'function': { swatch: 'bg-cat-function', softBg: 'bg-cat-function-soft', softText: 'text-cat-function-text', fill: 'var(--cat-function)' },
  'flow-control': { swatch: 'bg-cat-flow-control', softBg: 'bg-cat-flow-control-soft', softText: 'text-cat-flow-control-text', fill: 'var(--cat-flow-control)' },
  'social': { swatch: 'bg-cat-social', softBg: 'bg-cat-social-soft', softText: 'text-cat-social-text', fill: 'var(--cat-social)' },
  'storage': { swatch: 'bg-cat-storage', softBg: 'bg-cat-storage-soft', softText: 'text-cat-storage-text', fill: 'var(--cat-storage)' },
  'network': { swatch: 'bg-cat-network', softBg: 'bg-cat-network-soft', softText: 'text-cat-network-text', fill: 'var(--cat-network)' },
  'protocol': { swatch: 'bg-cat-protocol', softBg: 'bg-cat-protocol-soft', softText: 'text-cat-protocol-text', fill: 'var(--cat-protocol)' },
  'parser': { swatch: 'bg-cat-parser', softBg: 'bg-cat-parser-soft', softText: 'text-cat-parser-text', fill: 'var(--cat-parser)' },
  'config': { swatch: 'bg-cat-config', softBg: 'bg-cat-config-soft', softText: 'text-cat-config-text', fill: 'var(--cat-config)' },
  'dashboard': { swatch: 'bg-cat-dashboard', softBg: 'bg-cat-dashboard-soft', softText: 'text-cat-dashboard-text', fill: 'var(--cat-dashboard)' },
  'custom': { swatch: 'bg-cat-custom', softBg: 'bg-cat-custom-soft', softText: 'text-cat-custom-text', fill: 'var(--cat-custom)' },
};

export const DEFAULT_CATEGORY_COLOR: CategoryColor = CATEGORY_COLORS.custom;

export function getCategoryColor(category: string): CategoryColor {
  return CATEGORY_COLORS[category] || DEFAULT_CATEGORY_COLOR;
}

function channel(value: number): number {
  const c = value / 255;
  return c <= 0.03928 ? c / 12.92 : Math.pow((c + 0.055) / 1.055, 2.4);
}

/** Relative luminance of a #rrggbb color (WCAG 2). */
export function luminance(hex: string): number {
  const clean = hex.replace('#', '');
  const full = clean.length === 3 ? clean.split('').map((c) => c + c).join('') : clean;
  const r = parseInt(full.slice(0, 2), 16);
  const g = parseInt(full.slice(2, 4), 16);
  const b = parseInt(full.slice(4, 6), 16);
  return 0.2126 * channel(r) + 0.7152 * channel(g) + 0.0722 * channel(b);
}

/** WCAG contrast ratio between two #rrggbb colors. */
export function contrastRatio(a: string, b: string): number {
  const la = luminance(a);
  const lb = luminance(b);
  const [hi, lo] = la > lb ? [la, lb] : [lb, la];
  return (hi + 0.05) / (lo + 0.05);
}

/** White or near-black, whichever reads better on the given fill (for NodeMetadata.color overrides). */
export function readableTextColor(hex: string): string {
  if (!/^#([0-9a-f]{3}|[0-9a-f]{6})$/i.test(hex)) return '#ffffff';
  return contrastRatio('#ffffff', hex) >= contrastRatio('#1d2433', hex) ? '#ffffff' : '#1d2433';
}

/** Sorts categories by CATEGORY_ORDER, unknown categories alphabetically before 'custom'. */
export function sortCategories(categories: string[]): string[] {
  const known = new Set<string>(CATEGORY_ORDER);
  const ranked = categories.filter((c) => known.has(c) && c !== 'custom');
  const unranked = categories.filter((c) => !known.has(c)).sort();
  const hasCustom = categories.includes('custom');

  ranked.sort((a, b) => CATEGORY_ORDER.indexOf(a as KnownCategory) - CATEGORY_ORDER.indexOf(b as KnownCategory));

  return [...ranked, ...unranked, ...(hasCustom ? ['custom'] : [])];
}
