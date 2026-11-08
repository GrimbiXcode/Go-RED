import { useState, type FormEvent } from 'react';
import { useTranslation } from 'react-i18next';
import { KeyRound, LogIn } from 'lucide-react';
import { isSignedOut, setToken, startSignIn, useAuthStore } from '../lib/auth';

interface TokenPromptProps {
  /** Runs after a token was stored; defaults to reloading the page so every request and the socket use it. */
  onDone?: () => void;
}

/**
 * Shown after a request came back unauthorized: a single sign-on button when
 * the server has OIDC sign-in, the access token field when it takes a token
 * (or before GET /api/auth/info has answered), or both.
 */
export function TokenPrompt({ onDone = () => window.location.reload() }: TokenPromptProps) {
  const { t } = useTranslation();
  const required = useAuthStore((state) => state.required);
  const info = useAuthStore((state) => state.info);
  const setRequired = useAuthStore((state) => state.setRequired);
  const [value, setValue] = useState('');

  if (!required) return null;

  const sso = info?.methods.includes('oidc') ?? false;
  const token = !info || info.methods.includes('token');

  const submit = (event: FormEvent) => {
    event.preventDefault();
    const entered = value.trim();
    if (!entered) return;
    setToken(entered);
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
          {sso && info && (
            <>
              {isSignedOut() && (
                <p className="text-sm font-medium text-fg" data-testid="signed-out">
                  {t('auth.signedOut')}
                </p>
              )}
              <p className="text-sm text-muted">{t('auth.ssoText')}</p>
              <button
                type="button"
                onClick={() => startSignIn(info)}
                className="w-full flex items-center justify-center gap-2 px-3 py-1.5 bg-accent text-accent-fg rounded hover:bg-accent-strong text-sm font-medium"
                data-testid="sso-button"
              >
                <LogIn className="w-4 h-4" aria-hidden="true" />
                {t('auth.ssoButton')}
              </button>
            </>
          )}
          {sso && token && <p className="text-xs text-faint text-center">{t('auth.or')}</p>}
          {token && (
            <>
              <p className="text-sm text-muted">{t('auth.text')}</p>
              <input
                type="password"
                autoFocus={!sso}
                autoComplete="current-password"
                value={value}
                onChange={(event) => setValue(event.target.value)}
                placeholder={t('auth.placeholder')}
                aria-label={t('auth.placeholder')}
                className="w-full px-2 py-1.5 text-sm bg-sunken text-fg border border-line rounded focus:outline-none focus:border-accent"
                data-testid="token-input"
              />
            </>
          )}
        </div>
        {token && (
          <div className="flex items-center justify-end gap-2 px-4 py-3 border-t border-line">
            <button
              type="submit"
              className="px-3 py-1.5 bg-accent text-accent-fg rounded hover:bg-accent-strong text-xs font-medium disabled:opacity-50 disabled:cursor-not-allowed"
              disabled={!value.trim()}
            >
              {t('auth.submit')}
            </button>
          </div>
        )}
      </form>
    </div>
  );
}

export default TokenPrompt;
