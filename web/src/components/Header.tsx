import { useCallback, useEffect, useRef, useState, type ReactNode } from 'react';
import { useTranslation } from 'react-i18next';
import { SUPPORTED_LANGUAGES, setLanguage, type Language } from '../i18n';

export interface HeaderProps {
  hasFlow: boolean;
  /** The draft differs from what is running (or the flow never ran). */
  hasChanges: boolean;
  canDeploy: boolean;
  canUndeploy: boolean;
  canUndo: boolean;
  canRedo: boolean;
  onDeploy: () => void;
  onUndeploy: () => void;
  onUndo: () => void;
  onRedo: () => void;
  onExport: () => void;
  onImport: () => void;
}

function MenuItem({ onClick, disabled, children }: { onClick: () => void; disabled?: boolean; children: ReactNode }) {
  return (
    <button
      role="menuitem"
      className="w-full text-left px-3 py-2 text-sm text-gray-700 hover:bg-gr-blue-50 disabled:opacity-40 disabled:cursor-not-allowed disabled:hover:bg-transparent"
      onClick={onClick}
      disabled={disabled}
    >
      {children}
    </button>
  );
}

const iconProps = {
  viewBox: '0 0 24 24',
  fill: 'none' as const,
  stroke: 'currentColor',
  strokeWidth: 2,
  strokeLinecap: 'round' as const,
  strokeLinejoin: 'round' as const,
  className: 'w-4 h-4',
};

function UndoIcon() {
  return (
    <svg {...iconProps} aria-hidden="true">
      <polyline points="9 14 4 9 9 4" />
      <path d="M20 20v-7a4 4 0 0 0-4-4H4" />
    </svg>
  );
}

function RedoIcon() {
  return (
    <svg {...iconProps} aria-hidden="true">
      <polyline points="15 14 20 9 15 4" />
      <path d="M4 20v-7a4 4 0 0 1 4-4h12" />
    </svg>
  );
}

export function Header({
  hasFlow,
  hasChanges,
  canDeploy,
  canUndeploy,
  canUndo,
  canRedo,
  onDeploy,
  onUndeploy,
  onUndo,
  onRedo,
  onExport,
  onImport,
}: HeaderProps) {
  const { t, i18n } = useTranslation();
  const [menuOpen, setMenuOpen] = useState(false);
  const menuRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!menuOpen) return;
    const handleClickOutside = (event: MouseEvent) => {
      if (menuRef.current && !menuRef.current.contains(event.target as Node)) {
        setMenuOpen(false);
      }
    };
    document.addEventListener('mousedown', handleClickOutside);
    return () => document.removeEventListener('mousedown', handleClickOutside);
  }, [menuOpen]);

  const runAndClose = useCallback(
    (action: () => void) => () => {
      action();
      setMenuOpen(false);
    },
    []
  );

  const deployTitle = !hasFlow
    ? t('header.noFlow')
    : hasChanges
      ? t('header.deployChanged')
      : canDeploy
        ? t('header.deploy')
        : t('header.nothingToDeploy');

  return (
    <header className="h-11 flex items-center justify-between gap-3 px-3 bg-gr-blue-500 text-white shrink-0">
      <div className="flex items-center gap-2 min-w-0">
        <span className="font-semibold text-sm tracking-wide whitespace-nowrap">{t('app.title')}</span>
      </div>

      <div className="flex items-center gap-2">
        <button
          className="w-8 h-8 flex items-center justify-center rounded hover:bg-white/15 disabled:opacity-30 disabled:hover:bg-transparent"
          onClick={onUndo}
          disabled={!canUndo}
          title={`${t('header.undo')} (Ctrl+Z)`}
          aria-label={t('header.undo')}
        >
          <UndoIcon />
        </button>
        <button
          className="w-8 h-8 flex items-center justify-center rounded hover:bg-white/15 disabled:opacity-30 disabled:hover:bg-transparent mr-2"
          onClick={onRedo}
          disabled={!canRedo}
          title={`${t('header.redo')} (Ctrl+Y)`}
          aria-label={t('header.redo')}
        >
          <RedoIcon />
        </button>

        <button
          className={`px-3 py-1.5 rounded text-xs font-medium ${
            canUndeploy ? 'bg-gr-fuchsia-500 text-white hover:bg-gr-fuchsia-600' : 'bg-white/10 text-white/40 cursor-not-allowed'
          }`}
          onClick={onUndeploy}
          disabled={!canUndeploy}
          title={canUndeploy ? t('header.stopFlow') : t('header.flowNotRunning')}
        >
          {t('header.stop')}
        </button>

        <button
          className={`px-3 py-1.5 rounded text-xs font-semibold ${
            canDeploy ? 'bg-white text-gr-blue-700 hover:bg-gr-blue-50' : 'bg-white/10 text-white/40 cursor-not-allowed'
          }`}
          onClick={onDeploy}
          disabled={!canDeploy}
          title={deployTitle}
          data-testid="deploy-button"
        >
          {hasChanges && hasFlow ? `● ${t('header.deploy')}` : t('header.deploy')}
        </button>

        <div className="relative" ref={menuRef}>
          <button
            className="w-8 h-8 flex items-center justify-center rounded hover:bg-white/15 text-lg leading-none"
            onClick={() => setMenuOpen((open) => !open)}
            aria-label={t('header.menu')}
            aria-expanded={menuOpen}
            aria-haspopup="menu"
          >
            ☰
          </button>

          {menuOpen && (
            <div role="menu" className="absolute right-0 mt-1 w-52 bg-white text-gray-800 rounded shadow-lg border border-gray-200 py-1 z-20">
              <MenuItem onClick={runAndClose(onExport)} disabled={!hasFlow}>
                {t('header.export')}
              </MenuItem>
              <MenuItem onClick={runAndClose(onImport)}>{t('header.import')}</MenuItem>
              <div className="my-1 border-t border-gray-200" />
              <div className="px-3 py-1 text-[10px] uppercase tracking-wide text-gray-400">{t('language.label')}</div>
              {SUPPORTED_LANGUAGES.map((language: Language) => (
                <MenuItem key={language} onClick={runAndClose(() => setLanguage(language))}>
                  <span className={i18n.language.startsWith(language) ? 'font-semibold' : ''}>{t(`language.${language}`)}</span>
                </MenuItem>
              ))}
            </div>
          )}
        </div>
      </div>
    </header>
  );
}
