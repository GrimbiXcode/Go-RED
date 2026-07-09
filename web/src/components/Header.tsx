import { useCallback, useEffect, useRef, useState } from 'react';
import { WebSocketStatus } from './WebSocketStatus';

export interface HeaderProps {
  hasSelectedFlow: boolean;
  isDirty: boolean;
  canDeploy: boolean;
  canUndeploy: boolean;
  onDeploy: () => void;
  onUndeploy: () => void;
  onSave: () => void;
  onExport: () => void;
  onImport: () => void;
}

function MenuItem({
  onClick,
  disabled,
  children,
}: {
  onClick: () => void;
  disabled?: boolean;
  children: React.ReactNode;
}) {
  return (
    <button
      className="w-full text-left px-3 py-2 text-sm text-gray-700 hover:bg-gr-blue-50 disabled:opacity-40 disabled:cursor-not-allowed disabled:hover:bg-transparent"
      onClick={onClick}
      disabled={disabled}
    >
      {children}
    </button>
  );
}

export function Header({
  hasSelectedFlow,
  isDirty,
  canDeploy,
  canUndeploy,
  onDeploy,
  onUndeploy,
  onSave,
  onExport,
  onImport,
}: HeaderProps) {
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

  return (
    <header className="h-11 flex items-center justify-between gap-3 px-3 bg-gr-blue-500 text-white shrink-0">
      <div className="flex items-center gap-2 min-w-0">
        <span className="font-semibold text-sm tracking-wide whitespace-nowrap">Go-RED</span>
      </div>

      <div className="flex items-center gap-3">
        <WebSocketStatus variant="dot" />

        <button
          className={`px-3 py-1.5 rounded text-xs font-medium ${
            canUndeploy
              ? 'bg-gr-fuchsia-500 text-white hover:bg-gr-fuchsia-600'
              : 'bg-white/10 text-white/40 cursor-not-allowed'
          }`}
          onClick={onUndeploy}
          disabled={!canUndeploy}
          title={canUndeploy ? 'Flow stoppen' : 'Flow läuft nicht'}
        >
          Stop
        </button>

        <button
          className={`px-3 py-1.5 rounded text-xs font-semibold ${
            canDeploy
              ? 'bg-white text-gr-blue-700 hover:bg-gr-blue-50'
              : 'bg-white/10 text-white/40 cursor-not-allowed'
          }`}
          onClick={onDeploy}
          disabled={!canDeploy}
          title={
            !hasSelectedFlow
              ? 'Kein Flow ausgewählt'
              : isDirty
                ? 'Ungespeicherte Änderungen deployen'
                : canDeploy
                  ? 'Deploy'
                  : 'Keine Änderungen zum Deployen'
          }
        >
          {isDirty ? '● Deploy' : 'Deploy'}
        </button>

        <div className="relative" ref={menuRef}>
          <button
            className="w-8 h-8 flex items-center justify-center rounded hover:bg-white/15 text-lg leading-none"
            onClick={() => setMenuOpen((v) => !v)}
            aria-label="Hauptmenü"
            aria-expanded={menuOpen}
          >
            ☰
          </button>

          {menuOpen && (
            <div className="absolute right-0 mt-1 w-48 bg-white text-gray-800 rounded shadow-lg border border-gray-200 py-1 z-20">
              <MenuItem onClick={runAndClose(onSave)} disabled={!hasSelectedFlow}>
                Speichern
              </MenuItem>
              <MenuItem onClick={runAndClose(onExport)} disabled={!hasSelectedFlow}>
                Exportieren…
              </MenuItem>
              <MenuItem onClick={runAndClose(onImport)}>Importieren…</MenuItem>
            </div>
          )}
        </div>
      </div>
    </header>
  );
}
