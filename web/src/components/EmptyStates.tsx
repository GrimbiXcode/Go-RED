import { useTranslation } from 'react-i18next';
import { FolderPlus, Upload } from 'lucide-react';

/** Three nodes and two wires: the editor's idea of a flow, as an illustration. */
function FlowIllustration() {
  return (
    <svg viewBox="0 0 240 96" className="w-60 h-24" aria-hidden="true">
      <path d="M72 48h32M136 48h32" stroke="var(--wire)" strokeWidth="2" fill="none" />
      <rect x="8" y="30" width="64" height="36" rx="6" fill="var(--cat-input)" />
      <rect x="8" y="30" width="22" height="36" rx="6" fill="rgba(0,0,0,0.14)" />
      <rect x="104" y="30" width="64" height="36" rx="6" fill="var(--cat-function)" />
      <rect x="104" y="30" width="22" height="36" rx="6" fill="rgba(0,0,0,0.14)" />
      <rect x="168" y="30" width="64" height="36" rx="6" fill="var(--cat-output)" />
      <rect x="168" y="30" width="22" height="36" rx="6" fill="rgba(0,0,0,0.14)" />
      <circle cx="72" cy="48" r="5" fill="var(--bg-panel)" stroke="var(--cat-input)" strokeWidth="2" />
      <circle cx="104" cy="48" r="5" fill="var(--bg-panel)" stroke="var(--cat-function)" strokeWidth="2" />
      <circle cx="168" cy="48" r="5" fill="var(--bg-panel)" stroke="var(--cat-function)" strokeWidth="2" />
    </svg>
  );
}

export interface NoFlowsStateProps {
  onCreate: () => void;
  onImport: () => void;
}

/** Shown in place of the canvas while the server knows no flow at all. */
export function NoFlowsState({ onCreate, onImport }: NoFlowsStateProps) {
  const { t } = useTranslation();
  return (
    <div className="flex h-full w-full items-center justify-center bg-canvas" data-testid="empty-flows">
      <div className="flex flex-col items-center text-center max-w-md px-6">
        <FlowIllustration />
        <h2 className="mt-4 text-lg font-semibold text-fg">{t('empty.title')}</h2>
        <p className="mt-2 text-sm text-muted">{t('empty.text')}</p>
        <div className="mt-5 flex gap-2">
          <button className="flex items-center gap-1.5 px-3 h-8 rounded-md bg-accent text-accent-fg text-xs font-semibold hover:bg-accent-strong" onClick={onCreate}>
            <FolderPlus className="w-4 h-4" aria-hidden="true" />
            {t('empty.create')}
          </button>
          <button className="flex items-center gap-1.5 px-3 h-8 rounded-md border border-line-strong text-xs font-medium text-fg hover:bg-surface" onClick={onImport}>
            <Upload className="w-4 h-4" aria-hidden="true" />
            {t('empty.import')}
          </button>
        </div>
      </div>
    </div>
  );
}
