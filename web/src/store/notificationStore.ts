import { create } from 'zustand';

export type ToastType = 'success' | 'error' | 'info' | 'warning';

export interface Toast {
  id: string;
  type: ToastType;
  message: string;
  duration: number;
}

interface NotificationState {
  toasts: Toast[];
  notify: (type: ToastType, message: string, duration?: number) => void;
  dismiss: (id: string) => void;
}

let nextToastId = 0;

const DEFAULT_DURATION: Record<ToastType, number> = {
  success: 3000,
  info: 4000,
  warning: 5000,
  error: 6000,
};

/**
 * Toasts as a store rather than a React context, so stores and the
 * WebSocket event binding can raise them without a component in between.
 */
export const useNotificationStore = create<NotificationState>((set) => ({
  toasts: [],
  notify: (type, message, duration) => {
    const id = String(++nextToastId);
    set((state) => ({
      toasts: [...state.toasts, { id, type, message, duration: duration ?? DEFAULT_DURATION[type] }],
    }));
  },
  dismiss: (id) => set((state) => ({ toasts: state.toasts.filter((toast) => toast.id !== id) })),
}));

export const notify = (type: ToastType, message: string, duration?: number) =>
  useNotificationStore.getState().notify(type, message, duration);
