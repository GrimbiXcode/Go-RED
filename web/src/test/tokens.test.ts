import { describe, it, expect } from 'vitest';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { CATEGORY_FILLS, CATEGORY_ORDER, contrastRatio, readableTextColor, sortCategories } from '../utils/nodeCategories';

const css = readFileSync(resolve(__dirname, '../styles/tailwind.css'), 'utf8');

/** Reads the custom properties of one selector block into a map, resolving var() references. */
function tokensOf(selector: string): Record<string, string> {
  const start = css.indexOf(`${selector} {`);
  expect(start, `block ${selector}`).toBeGreaterThanOrEqual(0);
  const end = css.indexOf('\n}', start);
  const block = css.slice(start, end);
  const raw: Record<string, string> = {};
  for (const match of block.matchAll(/--([\w-]+):\s*([^;]+);/g)) raw[match[1]] = match[2].trim();
  const resolveValue = (value: string, depth = 0): string => {
    const ref = /^var\(--([\w-]+)\)$/.exec(value);
    if (!ref || depth > 5) return value;
    return resolveValue(raw[ref[1]] ?? value, depth + 1);
  };
  return Object.fromEntries(Object.entries(raw).map(([key, value]) => [key, resolveValue(value)]));
}

const light = tokensOf("[data-theme='light']");
const dark = tokensOf("[data-theme='dark']");

/** Text on background pairs that must read at body size (WCAG AA, 4.5:1). */
const PAIRS: [string, string][] = [
  ['fg', 'bg-app'],
  ['fg', 'bg-panel'],
  ['fg', 'bg-surface'],
  ['fg', 'bg-sunken'],
  ['fg-muted', 'bg-app'],
  ['fg-muted', 'bg-panel'],
  ['fg-muted', 'bg-surface'],
  ['fg-muted', 'bg-sunken'],
  ['fg-on-header', 'bg-header'],
  ['accent-fg', 'accent'],
  ['accent-fg', 'accent-strong'],
  ['accent-text', 'bg-panel'],
  ['accent-text', 'accent-soft'],
  ['danger-fg', 'danger'],
  ['danger-text', 'bg-panel'],
  ['danger-text', 'danger-soft'],
  ['warn-text', 'warn-soft'],
  ['warn-text', 'bg-panel'],
  ['ok-text', 'ok-soft'],
];

describe('design tokens', () => {
  it.each([
    ['light', light],
    ['dark', dark],
  ])('%s theme text/background pairs reach 4.5:1', (_name, tokens) => {
    for (const [fgToken, bgToken] of PAIRS) {
      expect(tokens[fgToken], fgToken).toMatch(/^#[0-9a-f]{6}$/i);
      expect(tokens[bgToken], bgToken).toMatch(/^#[0-9a-f]{6}$/i);
      const ratio = contrastRatio(tokens[fgToken], tokens[bgToken]);
      expect(ratio, `${fgToken} on ${bgToken}: ${ratio.toFixed(2)}`).toBeGreaterThanOrEqual(4.5);
    }
  });

  it('defines every token for both themes', () => {
    const lightKeys = Object.keys(light).filter((key) => !key.startsWith('gr-'));
    for (const key of lightKeys) {
      if (key.startsWith('cat-') && !key.endsWith('-soft') && !key.endsWith('-text')) continue; // fills are shared
      expect(dark[key], `dark value of --${key}`).toBeDefined();
    }
  });

  it('category fills keep white label text readable and match the CSS', () => {
    for (const category of CATEGORY_ORDER) {
      const fill = CATEGORY_FILLS[category];
      expect(light[`cat-${category}`], category).toBe(fill);
      const ratio = contrastRatio('#ffffff', fill);
      expect(ratio, `${category} ${fill}: ${ratio.toFixed(2)}`).toBeGreaterThanOrEqual(4.5);
      expect(readableTextColor(fill)).toBe('#ffffff');
    }
    expect(readableTextColor('#fef3c7')).toBe('#1d2433');
  });

  it('orders categories with custom last', () => {
    expect(sortCategories(['custom', 'parser', 'zzz', 'input'])).toEqual(['input', 'parser', 'zzz', 'custom']);
  });
});
