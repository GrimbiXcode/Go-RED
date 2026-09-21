import { useState, type FormEvent } from 'react';
import { useTranslation } from 'react-i18next';
import { KeyRound } from 'lucide-react';
import { setToken, useAuthStore } from '../lib/auth';

interface TokenPromptProps {
  /** Runs after a token was stored; defaults to reloading the page so every request and the socket use it. */
  onDone?: () => void;
}

/** Asks for the server's access token after a request came back unauthorized. */
export function TokenPrompt({ onDone = () => window.location.reload() }: TokenPromptProps) {
  const { t } = useTranslation();
  const required = useAuthStore((state) => state.required);
  const setRequired = useAuthStore((state) => state.setRequired);
  const [value, setValue] = useState('');

  if (!required) return null;

  const submit = (event: FormEvent) => {
    event.preventDefault();
    const token = value.trim();
    if (!token) return;
    setToken(token);
    setRequired(false);
    onDone();
  };

  return (
    <div className="fixed inset-0 bg-black/40 flex items-center justify-center z-50 p-4" role="dialog" aria-label={t('auth.title')}>
      <form className="bg-panel rounded-md shadow-float w-full max-w-sm" onSubmit={submit} data-testid="token-prompt">
        <div className="flex items-center gap-2 px-4 py-3 border-b border-line">
          <KeyRound className="w-4 h-4 text-accent-text" aria-hidden="true" />
          <h3 className="font-semibold text-sm text-fg">{t('auth.title')}</h3>
        </div>
        <div className="p-4 space-y-3">
          <p className="text-sm text-muted">{t('auth.text')}</p>
          <input
            type="password"
            autoFocus
            autoComplete="current-password"
            value={value}
            onChange={(event) => setValue(event.target.value)}
            placeholder={t('auth.placeholder')}
            aria-label={t('auth.placeholder')}
            className="w-full px-2 py-1.5 text-sm bg-sunken text-fg border border-line rounded focus:outline-none focus:border-accent"
            data-testid="token-input"
          />
        </div>
        <div className="flex items-center justify-end gap-2 px-4 py-3 border-t border-line">
          <button
            type="submit"
            className="px-3 py-1.5 bg-accent text-accent-fg rounded hover:bg-accent-strong text-xs font-medium disabled:opacity-50 disabled:cursor-not-allowed"
            disabled={!value.trim()}
          >
            {t('auth.submit')}
          </button>
        </div>
      </form>
    </div>
  );
}

export default TokenPrompt;
