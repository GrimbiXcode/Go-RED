import { create } from 'zustand';

export type ThemePreference = 'light' | 'dark' | 'system';
export type ResolvedTheme = 'light' | 'dark';

export const THEME_PREFERENCES: ThemePreference[] = ['system', 'light', 'dark'];
const STORAGE_KEY = 'go-red.theme';
const DARK_QUERY = '(prefers-color-scheme: dark)';

function isPreference(value: unknown): value is ThemePreference {
  return value === 'light' || value === 'dark' || value === 'system';
}

/** The stored choice, else "system". */
export function readThemePreference(): ThemePreference {
  try {
    const stored = window.localStorage.getItem(STORAGE_KEY);
    if (isPreference(stored)) return stored;
  } catch {
    // Storage unavailable: behave like a first visit.
  }
  return 'system';
}

export function systemTheme(): ResolvedTheme {
  if (typeof window === 'undefined' || typeof window.matchMedia !== 'function') return 'light';
  return window.matchMedia(DARK_QUERY).matches ? 'dark' : 'light';
}

export function resolveTheme(preference: ThemePreference): ResolvedTheme {
  return preference === 'system' ? systemTheme() : preference;
}

/** Puts the theme on <html>, where the CSS tokens read it (index.html does the same before React loads). */
export function applyTheme(theme: ResolvedTheme): void {
  if (typeof document !== 'undefined') document.documentElement.dataset.theme = theme;
}

interface ThemeState {
  preference: ThemePreference;
  resolved: ResolvedTheme;
  setPreference: (preference: ThemePreference) => void;
}

export const useThemeStore = create<ThemeState>((set) => ({
  preference: 'system',
  resolved: 'light',
  setPreference: (preference) => {
    try {
      window.localStorage.setItem(STORAGE_KEY, preference);
    } catch {
      // Not persisted, still applied for this session.
    }
    const resolved = resolveTheme(preference);
    applyTheme(resolved);
    set({ preference, resolved });
  },
}));

/**
 * Applies the stored preference and follows the system setting while the
 * preference is "system". Returns a cleanup function.
 */
export function initTheme(): () => void {
  const preference = readThemePreference();
  const resolved = resolveTheme(preference);
  applyTheme(resolved);
  useThemeStore.setState({ preference, resolved });

  if (typeof window === 'undefined' || typeof window.matchMedia !== 'function') return () => {};
  const query = window.matchMedia(DARK_QUERY);
  const onChange = () => {
    if (useThemeStore.getState().preference !== 'system') return;
    const next = systemTheme();
    applyTheme(next);
    useThemeStore.setState({ resolved: next });
  };
  query.addEventListener('change', onChange);
  return () => query.removeEventListener('change', onChange);
}
