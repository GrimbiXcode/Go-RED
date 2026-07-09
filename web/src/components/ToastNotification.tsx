import React, { useState, useEffect, useCallback } from 'react';

export type ToastType = 'success' | 'error' | 'info' | 'warning';

interface ToastMessage {
  id: string;
  type: ToastType;
  message: string;
  duration?: number;
}

interface ToastNotificationProps {
  message: ToastMessage;
  onDismiss: (id: string) => void;
}

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

function ToastNotification({ message, onDismiss }: ToastNotificationProps) {
  useEffect(() => {
    const timer = setTimeout(() => {
      onDismiss(message.id);
    }, message.duration || 3000);

    return () => clearTimeout(timer);
  }, [message.id, message.duration, onDismiss]);

  const style = toastStyles[message.type];

  return (
    <div
      className={`w-80 bg-white rounded shadow-lg border-l-4 ${style.border} px-3 py-2 flex items-start gap-2 animate-slide-in-right`}
    >
      <span className={`mt-0.5 ${style.icon}`}>
        <ToastIcon type={message.type} />
      </span>
      <span className="text-xs text-gray-700 flex-1">{message.message}</span>
      <button
        className="text-gray-400 hover:text-gray-600 shrink-0"
        onClick={() => onDismiss(message.id)}
        aria-label="Schließen"
      >
        ✕
      </button>
    </div>
  );
}

interface ToastProviderProps {
  children: React.ReactNode;
}

export interface ToastContextType {
  showToast: (type: ToastType, message: string, duration?: number) => void;
}

let toastId = 0;

const ToastContext = React.createContext<ToastContextType | undefined>(undefined);

export function ToastProvider({ children }: ToastProviderProps) {
  const [toasts, setToasts] = useState<ToastMessage[]>([]);

  const showToast = useCallback((type: ToastType, message: string, duration?: number) => {
    const id = String(++toastId);
    setToasts((prev) => [...prev, { id, type, message, duration }]);
  }, []);

  const dismissToast = useCallback((id: string) => {
    setToasts((prev) => prev.filter((toast) => toast.id !== id));
  }, []);

  return (
    <ToastContext.Provider value={{ showToast }}>
      {children}
      {/* Bottom-right (above the status bar), not top-right: the header's
          hamburger menu also opens top-right, and a toast sitting on top
          of it would block clicks on the menu items underneath. */}
      <div className="fixed bottom-8 right-4 z-50 flex flex-col-reverse gap-2 items-end">
        {toasts.map((toast) => (
          <ToastNotification
            key={toast.id}
            message={toast}
            onDismiss={dismissToast}
          />
        ))}
      </div>
    </ToastContext.Provider>
  );
}

export function useToast() {
  const context = React.useContext(ToastContext);
  if (context === undefined) {
    throw new Error('useToast must be used within a ToastProvider');
  }
  return context;
}
