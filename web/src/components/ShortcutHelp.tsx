import { useTranslation } from 'react-i18next';
import { X } from 'lucide-react';

export interface ShortcutHelpProps {
  isOpen: boolean;
  onClose: () => void;
}

const GROUPS: { title: string; items: [string, string][] }[] = [
  {
    title: 'shortcuts.editing',
    items: [
      ['Ctrl+Z', 'shortcuts.undo'],
      ['Ctrl+Y', 'shortcuts.redo'],
      ['Ctrl+C', 'shortcuts.copy'],
      ['Ctrl+V', 'shortcuts.paste'],
      ['Ctrl+D', 'shortcuts.duplicate'],
      ['Ctrl+A', 'shortcuts.selectAll'],
      ['Del', 'shortcuts.delete'],
      ['←↑→↓', 'shortcuts.nudge'],
      ['Shift+←↑→↓', 'shortcuts.nudgeGrid'],
    ],
  },
  {
    title: 'shortcuts.canvas',
    items: [
      ['Drag', 'shortcuts.select'],
      ['Space+Drag', 'shortcuts.pan'],
      ['+ / −', 'shortcuts.zoom'],
      ['Double-click', 'shortcuts.quickAdd'],
      ['Right-click', 'shortcuts.contextMenu'],
    ],
  },
  {
    title: 'shortcuts.flow',
    items: [
      ['Ctrl+S', 'shortcuts.deploy'],
      ['Ctrl+E', 'shortcuts.export'],
      ['Ctrl+F or /', 'shortcuts.find'],
      ['?', 'shortcuts.help'],
    ],
  },
];

/** Keyboard reference, opened with "?". */
export function ShortcutHelp({ isOpen, onClose }: ShortcutHelpProps) {
  const { t } = useTranslation();
  if (!isOpen) return null;
  return (
    <div className="fixed inset-0 bg-black/40 flex items-center justify-center z-50 p-4" role="dialog" aria-label={t('shortcuts.title')} onClick={onClose}>
      <div className="bg-panel rounded-md shadow-float w-full max-w-lg" onClick={(event) => event.stopPropagation()} data-testid="shortcut-help">
        <div className="flex items-center justify-between px-4 py-3 border-b border-line">
          <h3 className="font-semibold text-sm text-fg">{t('shortcuts.title')}</h3>
          <button className="text-faint hover:text-muted" onClick={onClose} aria-label={t('common.close')}>
            <X className="w-4 h-4" aria-hidden="true" />
          </button>
        </div>
        <div className="p-4 grid grid-cols-1 sm:grid-cols-3 gap-4">
          {GROUPS.map((group) => (
            <div key={group.title}>
              <div className="text-2xs uppercase tracking-wide text-faint mb-2">{t(group.title)}</div>
              <dl className="space-y-1.5">
                {group.items.map(([keys, label]) => (
                  <div key={keys} className="flex items-center gap-2 text-xs">
                    <dt>
                      <kbd className="px-1.5 py-0.5 rounded border border-line bg-surface font-mono text-2xs text-fg whitespace-nowrap">{keys}</kbd>
                    </dt>
                    <dd className="text-muted">{t(label)}</dd>
                  </div>
                ))}
              </dl>
            </div>
          ))}
        </div>
      </div>
    </div>
  );
}
