import { useEffect } from 'react';
import { useTranslation } from 'react-i18next';
import { useNotificationStore, type Toast, type ToastType } from '../store/notificationStore';

export type { ToastType };

const toastStyles: Record<ToastType, { border: string; icon: string }> = {
  success: { border: 'border-gr-blue-500', icon: 'text-gr-blue-500' },
  info: { border: 'border-slate-400', icon: 'text-slate-500' },
  warning: { border: 'border-gr-fuchsia-300', icon: 'text-gr-fuchsia-400' },
  error: { border: 'border-gr-fuchsia-600', icon: 'text-gr-fuchsia-600' },
};

function ToastIcon({ type }: { type: ToastType }) {
  const props = {
    viewBox: '0 0 24 24',
    fill: 'none' as const,
    stroke: 'currentColor',
    strokeWidth: 2,
    strokeLinecap: 'round' as const,
    strokeLinejoin: 'round' as const,
    className: 'w-4 h-4 shrink-0',
  };
  switch (type) {
    case 'success':
      return (
        <svg {...props} aria-hidden="true">
          <circle cx="12" cy="12" r="9" />
          <polyline points="8 12 11 15 16 9" />
        </svg>
      );
    case 'error':
      return (
        <svg {...props} aria-hidden="true">
          <circle cx="12" cy="12" r="9" />
          <line x1="9" y1="9" x2="15" y2="15" />
          <line x1="15" y1="9" x2="9" y2="15" />
        </svg>
      );
    case 'warning':
      return (
        <svg {...props} aria-hidden="true">
          <path d="M12 3 2 20h20L12 3z" />
          <line x1="12" y1="10" x2="12" y2="15" />
          <circle cx="12" cy="17.5" r="0.75" fill="currentColor" stroke="none" />
        </svg>
      );
    case 'info':
    default:
      return (
        <svg {...props} aria-hidden="true">
          <circle cx="12" cy="12" r="9" />
          <line x1="12" y1="11" x2="12" y2="16" />
          <circle cx="12" cy="7.5" r="0.75" fill="currentColor" stroke="none" />
        </svg>
      );
  }
}

function ToastItem({ toast, onDismiss }: { toast: Toast; onDismiss: (id: string) => void }) {
  const { t } = useTranslation();

  useEffect(() => {
    const timer = setTimeout(() => onDismiss(toast.id), toast.duration);
    return () => clearTimeout(timer);
  }, [toast.id, toast.duration, onDismiss]);

  const style = toastStyles[toast.type];

  return (
    <div
      role={toast.type === 'error' ? 'alert' : 'status'}
      className={`w-80 bg-white rounded shadow-lg border-l-4 ${style.border} px-3 py-2 flex items-start gap-2 animate-slide-in-right`}
    >
      <span className={`mt-0.5 ${style.icon}`}>
        <ToastIcon type={toast.type} />
      </span>
      <span className="text-xs text-gray-700 flex-1">{toast.message}</span>
      <button
        className="text-gray-400 hover:text-gray-600 shrink-0"
        onClick={() => onDismiss(toast.id)}
        aria-label={t('common.close')}
      >
        ✕
      </button>
    </div>
  );
}

/** Renders the toasts held by the notification store. Mount once, near the root. */
export function ToastHost() {
  const toasts = useNotificationStore((state) => state.toasts);
  const dismiss = useNotificationStore((state) => state.dismiss);

  return (
    // Bottom-right (above the status bar), not top-right: the header's
    // menu also opens top-right and a toast there would cover it.
    <div className="fixed bottom-8 right-4 z-50 flex flex-col-reverse gap-2 items-end">
      {toasts.map((toast) => (
        <ToastItem key={toast.id} toast={toast} onDismiss={dismiss} />
      ))}
    </div>
  );
}

/** Convenience hook: `showToast(type, message, duration?)`. */
export function useToast() {
  const showToast = useNotificationStore((state) => state.notify);
  return { showToast };
}
