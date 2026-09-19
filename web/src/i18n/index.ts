import i18n from 'i18next';
import { initReactI18next } from 'react-i18next';
import en from './en';
import de from './de';

export const SUPPORTED_LANGUAGES = ['en', 'de'] as const;
export type Language = (typeof SUPPORTED_LANGUAGES)[number];

const STORAGE_KEY = 'go-red.language';

function isLanguage(value: unknown): value is Language {
  return typeof value === 'string' && (SUPPORTED_LANGUAGES as readonly string[]).includes(value);
}

/** The language to start with: a stored choice, else the browser's, else English. */
export function detectLanguage(): Language {
  try {
    const stored = window.localStorage.getItem(STORAGE_KEY);
    if (isLanguage(stored)) return stored;
  } catch {
    // Storage may be unavailable (private mode, disabled); fall through.
  }
  const browser = typeof navigator !== 'undefined' ? navigator.language : '';
  return browser.toLowerCase().startsWith('de') ? 'de' : 'en';
}

export function setLanguage(language: Language): void {
  try {
    window.localStorage.setItem(STORAGE_KEY, language);
  } catch {
    // Not persisted, but still switched for this session.
  }
  void i18n.changeLanguage(language);
}

if (!i18n.isInitialized) {
  void i18n.use(initReactI18next).init({
    resources: {
      en: { translation: en },
      de: { translation: de },
    },
    lng: typeof window === 'undefined' ? 'en' : detectLanguage(),
    fallbackLng: 'en',
    interpolation: { escapeValue: false },
    returnNull: false,
    // Keep the browser console clean in production.
    showSupportNotice: false,
  });
}

export default i18n;
