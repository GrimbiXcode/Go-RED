/**
 * Shared node-category metadata for the palette (Phase 2 of
 * docs/FRONTEND_NODE_RED_REDESIGN.md). Single source of truth so the
 * category order/colors don't drift between the palette and, later, the
 * canvas node redesign (Phase 3) that will consume the same data.
 *
 * Colors are tints/shades of the three verified Go brand colors
 * (gr-blue/gr-skyblue/gr-fuchsia, see tailwind.config.js) plus neutral
 * gray — Go's brand book doesn't define a wide category palette like
 * Node-RED's, so categories are distinguished by tint/shade instead of by
 * hue (see "Farbpalette: Go statt Node-RED-Rot" in the plan doc).
 */

// Node-RED's own reference order is input/output/function/social/storage/
// analysis/advanced; extended here with Go-RED's actual current categories
// (network/protocol/parser/dashboard), 'custom' always last as the catch-all
// for anything unrecognized.
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

export interface CategoryColor {
  /** Solid swatch, e.g. for a small color chip. */
  swatch: string;
  /** Light background for a category header row. */
  softBg: string;
  /** Text color to pair with `softBg`. */
  softText: string;
}

export const CATEGORY_COLORS: Record<string, CategoryColor> = {
  input: { swatch: 'bg-gr-blue-500', softBg: 'bg-gr-blue-50', softText: 'text-gr-blue-700' },
  output: { swatch: 'bg-gr-skyblue-500', softBg: 'bg-gr-skyblue-50', softText: 'text-gr-skyblue-700' },
  function: { swatch: 'bg-gr-blue-700', softBg: 'bg-gr-blue-100', softText: 'text-gr-blue-800' },
  'flow-control': { swatch: 'bg-gr-fuchsia-600', softBg: 'bg-gr-fuchsia-50', softText: 'text-gr-fuchsia-800' },
  storage: { swatch: 'bg-slate-500', softBg: 'bg-slate-100', softText: 'text-slate-700' },
  network: { swatch: 'bg-gr-skyblue-700', softBg: 'bg-gr-skyblue-100', softText: 'text-gr-skyblue-800' },
  protocol: { swatch: 'bg-gr-blue-300', softBg: 'bg-gr-blue-50', softText: 'text-gr-blue-600' },
  parser: { swatch: 'bg-gr-skyblue-300', softBg: 'bg-gr-skyblue-50', softText: 'text-gr-skyblue-600' },
  social: { swatch: 'bg-gr-fuchsia-400', softBg: 'bg-gr-fuchsia-50', softText: 'text-gr-fuchsia-700' },
  config: { swatch: 'bg-slate-600', softBg: 'bg-slate-100', softText: 'text-slate-800' },
  dashboard: { swatch: 'bg-gr-blue-400', softBg: 'bg-gr-blue-50', softText: 'text-gr-blue-700' },
  custom: { swatch: 'bg-slate-400', softBg: 'bg-slate-100', softText: 'text-slate-600' },
};

export const DEFAULT_CATEGORY_COLOR: CategoryColor = CATEGORY_COLORS.custom;

export function getCategoryColor(category: string): CategoryColor {
  return CATEGORY_COLORS[category] || DEFAULT_CATEGORY_COLOR;
}

/** Sorts categories by CATEGORY_ORDER, unknown categories alphabetically before 'custom'. */
export function sortCategories(categories: string[]): string[] {
  const known = new Set<string>(CATEGORY_ORDER);
  const ranked = categories.filter((c) => known.has(c) && c !== 'custom');
  const unranked = categories.filter((c) => !known.has(c)).sort();
  const hasCustom = categories.includes('custom');

  ranked.sort((a, b) => CATEGORY_ORDER.indexOf(a as any) - CATEGORY_ORDER.indexOf(b as any));

  return [...ranked, ...unranked, ...(hasCustom ? ['custom'] : [])];
}
