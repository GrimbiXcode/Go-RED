import { useEffect } from 'react';
import { useTranslation } from 'react-i18next';
import { CircleCheck, CircleX, Info, TriangleAlert, X } from 'lucide-react';
import { useNotificationStore, type Toast, type ToastType } from '../store/notificationStore';

export type { ToastType };

const toastStyles: Record<ToastType, { border: string; icon: string }> = {
  success: { border: 'border-accent', icon: 'text-accent' },
  info: { border: 'border-line-strong', icon: 'text-muted' },
  warning: { border: 'border-warn', icon: 'text-warn' },
  error: { border: 'border-danger', icon: 'text-danger-text' },
};

function ToastIcon({ type }: { type: ToastType }) {
  const className = 'w-4 h-4 shrink-0';
  switch (type) {
    case 'success':
      return <CircleCheck className={className} aria-hidden="true" />;
    case 'error':
      return <CircleX className={className} aria-hidden="true" />;
    case 'warning':
      return <TriangleAlert className={className} aria-hidden="true" />;
    default:
      return <Info className={className} aria-hidden="true" />;
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
      className={`w-80 bg-panel rounded-md shadow-float border border-line border-l-4 ${style.border} px-3 py-2 flex items-start gap-2 animate-slide-in-right`}
    >
      <span className={`mt-0.5 ${style.icon}`}>
        <ToastIcon type={toast.type} />
      </span>
      <span className="text-xs text-fg flex-1">{toast.message}</span>
      <button
        className="text-faint hover:text-muted shrink-0"
        onClick={() => onDismiss(toast.id)}
        aria-label={t('common.close')}
      >
        <X className="w-3.5 h-3.5" aria-hidden="true" />
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
