import { useCallback, useEffect, useRef, useState, type ReactNode } from 'react';
import { useTranslation } from 'react-i18next';
import { ChevronDown, Menu, Monitor, Moon, Redo2, Sun, Undo2 } from 'lucide-react';
import { SUPPORTED_LANGUAGES, setLanguage, type Language } from '../i18n';
import { THEME_PREFERENCES, useThemeStore, type ThemePreference } from '../lib/theme';
import { WebSocketStatus } from './WebSocketStatus';

export interface HeaderProps {
  hasFlow: boolean;
  /** The draft differs from what is running (or the flow never ran). */
  hasChanges: boolean;
  canDeploy: boolean;
  canUndeploy: boolean;
  canUndo: boolean;
  canRedo: boolean;
  /** Other flows whose draft differs from their running instance. */
  modifiedFlows?: number;
  onDeploy: () => void;
  onDeployAll?: () => void;
  onUndeploy: () => void;
  onUndo: () => void;
  onRedo: () => void;
  onExport: () => void;
  onImport: () => void;
}

function MenuItem({ onClick, disabled, active, children }: { onClick: () => void; disabled?: boolean; active?: boolean; children: ReactNode }) {
  return (
    <button
      role="menuitem"
      className={`w-full text-left px-3 py-1.5 text-sm flex items-center gap-2 hover:bg-accent-soft disabled:opacity-40 disabled:cursor-not-allowed disabled:hover:bg-transparent ${
        active ? 'font-semibold text-accent-text' : 'text-fg'
      }`}
      onClick={onClick}
      disabled={disabled}
    >
      {children}
    </button>
  );
}

function MenuSection({ label }: { label: string }) {
  return <div className="px-3 pt-2 pb-1 text-2xs uppercase tracking-wide text-faint">{label}</div>;
}

/** Small mark next to the wordmark: three nodes wired together. */
export function GoRedMark({ className = 'w-6 h-6' }: { className?: string }) {
  return (
    <svg viewBox="0 0 24 24" className={className} aria-hidden="true">
      <rect x="1" y="1" width="22" height="22" rx="6" fill="var(--accent)" />
      <path d="M9 12c3 0 3-4.5 6-4.5M9 12c3 0 3 4.5 6 4.5" stroke="#0b1116" strokeWidth="1.7" fill="none" strokeLinecap="round" />
      <circle cx="7" cy="12" r="2.3" fill="#0b1116" />
      <circle cx="17" cy="7.5" r="2.3" fill="#0b1116" />
      <circle cx="17" cy="16.5" r="2.3" fill="#0b1116" />
    </svg>
  );
}

const themeIcons: Record<ThemePreference, ReactNode> = {
  system: <Monitor className="w-3.5 h-3.5" aria-hidden="true" />,
  light: <Sun className="w-3.5 h-3.5" aria-hidden="true" />,
  dark: <Moon className="w-3.5 h-3.5" aria-hidden="true" />,
};

const iconButton = 'w-8 h-8 flex items-center justify-center rounded-md text-white/85 hover:bg-white/10 hover:text-white disabled:opacity-30 disabled:hover:bg-transparent';

/**
 * Dark app bar: wordmark and connection state on the left; undo/redo,
 * Stop and the Deploy split button (this flow / every modified flow) on
 * the right, followed by the main menu (export, import, language, theme).
 */
export function Header({
  hasFlow,
  hasChanges,
  canDeploy,
  canUndeploy,
  canUndo,
  canRedo,
  modifiedFlows = 0,
  onDeploy,
  onDeployAll,
  onUndeploy,
  onUndo,
  onRedo,
  onExport,
  onImport,
}: HeaderProps) {
  const { t, i18n } = useTranslation();
  const [menuOpen, setMenuOpen] = useState(false);
  const [deployMenuOpen, setDeployMenuOpen] = useState(false);
  const menuRef = useRef<HTMLDivElement>(null);
  const deployRef = useRef<HTMLDivElement>(null);
  const themePreference = useThemeStore((state) => state.preference);
  const setThemePreference = useThemeStore((state) => state.setPreference);

  useEffect(() => {
    if (!menuOpen && !deployMenuOpen) return;
    const handleClickOutside = (event: MouseEvent) => {
      const target = event.target as Node;
      if (menuRef.current && !menuRef.current.contains(target)) setMenuOpen(false);
      if (deployRef.current && !deployRef.current.contains(target)) setDeployMenuOpen(false);
    };
    document.addEventListener('mousedown', handleClickOutside);
    return () => document.removeEventListener('mousedown', handleClickOutside);
  }, [menuOpen, deployMenuOpen]);

  const runAndClose = useCallback(
    (action: () => void) => () => {
      action();
      setMenuOpen(false);
      setDeployMenuOpen(false);
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
  const canDeployAll = canDeploy || modifiedFlows > 0;

  return (
    <header className="h-11 flex items-center justify-between gap-3 px-3 bg-header text-white shrink-0 shadow-panel" data-testid="app-header">
      <div className="flex items-center gap-3 min-w-0">
        <div className="flex items-center gap-2">
          <GoRedMark />
          <span className="font-semibold text-sm tracking-wide whitespace-nowrap">
            <span className="text-accent">Go</span>-RED
          </span>
        </div>
        <WebSocketStatus variant="dot" />
      </div>

      <div className="flex items-center gap-1.5">
        <button className={iconButton} onClick={onUndo} disabled={!canUndo} title={`${t('header.undo')} (Ctrl+Z)`} aria-label={t('header.undo')}>
          <Undo2 className="w-4 h-4" aria-hidden="true" />
        </button>
        <button className={`${iconButton} mr-2`} onClick={onRedo} disabled={!canRedo} title={`${t('header.redo')} (Ctrl+Y)`} aria-label={t('header.redo')}>
          <Redo2 className="w-4 h-4" aria-hidden="true" />
        </button>

        <button
          className={`px-3 h-8 rounded-md text-xs font-medium border ${
            canUndeploy ? 'border-danger/70 text-white hover:bg-danger hover:text-danger-fg' : 'border-white/15 text-white/35 cursor-not-allowed'
          }`}
          onClick={onUndeploy}
          disabled={!canUndeploy}
          title={canUndeploy ? t('header.stopFlow') : t('header.flowNotRunning')}
        >
          {t('header.stop')}
        </button>

        <div className="relative flex items-stretch" ref={deployRef}>
          <button
            className={`pl-3 pr-2.5 h-8 rounded-l-md text-xs font-semibold ${
              canDeploy ? 'bg-accent text-accent-fg hover:bg-accent-strong' : 'bg-white/10 text-white/35 cursor-not-allowed'
            }`}
            onClick={onDeploy}
            disabled={!canDeploy}
            title={deployTitle}
            data-testid="deploy-button"
          >
            {hasChanges && hasFlow ? `● ${t('header.deploy')}` : t('header.deploy')}
          </button>
          <button
            className={`w-6 h-8 rounded-r-md flex items-center justify-center border-l ${
              canDeployAll ? 'bg-accent text-accent-fg border-black/15 hover:bg-accent-strong' : 'bg-white/10 text-white/35 border-white/10 cursor-not-allowed'
            }`}
            onClick={() => setDeployMenuOpen((open) => !open)}
            disabled={!canDeployAll}
            aria-label={t('header.deployOptions')}
            aria-expanded={deployMenuOpen}
            aria-haspopup="menu"
            data-testid="deploy-options"
          >
            <ChevronDown className="w-3.5 h-3.5" aria-hidden="true" />
          </button>
          {deployMenuOpen && (
            <div role="menu" className="absolute right-0 top-9 w-64 bg-panel text-fg rounded-md shadow-float border border-line py-1 z-40 animate-fade-in">
              <MenuItem onClick={runAndClose(onDeploy)} disabled={!canDeploy}>
                {t('header.deployThis')}
              </MenuItem>
              <MenuItem onClick={runAndClose(() => onDeployAll?.())} disabled={!onDeployAll || (!canDeploy && modifiedFlows === 0)}>
                {t('header.deployAll', { count: modifiedFlows + (canDeploy && hasFlow ? 1 : 0) })}
              </MenuItem>
            </div>
          )}
        </div>

        <div className="relative" ref={menuRef}>
          <button className={iconButton} onClick={() => setMenuOpen((open) => !open)} aria-label={t('header.menu')} aria-expanded={menuOpen} aria-haspopup="menu">
            <Menu className="w-4 h-4" aria-hidden="true" />
          </button>

          {menuOpen && (
            <div role="menu" className="absolute right-0 mt-1 w-56 bg-panel text-fg rounded-md shadow-float border border-line py-1 z-40 animate-fade-in">
              <MenuItem onClick={runAndClose(onExport)} disabled={!hasFlow}>
                {t('header.export')}
              </MenuItem>
              <MenuItem onClick={runAndClose(onImport)}>{t('header.import')}</MenuItem>
              <div className="my-1 border-t border-line" />
              <MenuSection label={t('language.label')} />
              {SUPPORTED_LANGUAGES.map((language: Language) => (
                <MenuItem key={language} onClick={runAndClose(() => setLanguage(language))} active={i18n.language.startsWith(language)}>
                  {t(`language.${language}`)}
                </MenuItem>
              ))}
              <div className="my-1 border-t border-line" />
              <MenuSection label={t('theme.label')} />
              {THEME_PREFERENCES.map((preference) => (
                <MenuItem key={preference} onClick={runAndClose(() => setThemePreference(preference))} active={themePreference === preference}>
                  {themeIcons[preference]}
                  {t(`theme.${preference}`)}
                </MenuItem>
              ))}
            </div>
          )}
        </div>
      </div>
    </header>
  );
}
