import { useTranslation } from 'react-i18next';
import { useWebSocket } from '../hooks/useWebSocket';

export interface WebSocketStatusProps {
  /** 'dot' is a compact light-on-dark variant for the (Gopher Blue)
   * header; 'dot-light' is the same for light backgrounds (the bottom
   * status bar). */
  variant?: 'dot' | 'dot-light';
}

const dotColorsOnDark = {
  connected: 'bg-white',
  connecting: 'bg-white/70 animate-pulse',
  error: 'bg-gr-fuchsia-300',
  unknown: 'bg-white/40',
};

const dotColorsOnLight = {
  connected: 'bg-gr-blue-500',
  connecting: 'bg-gr-skyblue-500 animate-pulse',
  error: 'bg-gr-fuchsia-500',
  unknown: 'bg-gray-400',
};

export function WebSocketStatus({ variant = 'dot-light' }: WebSocketStatusProps) {
  const { t } = useTranslation();
  const { state } = useWebSocket();
  const { connected, connecting, error } = state;

  const key = connected ? 'connected' : connecting ? 'connecting' : error ? 'disconnected' : 'unknown';
  const dotKey = key === 'disconnected' ? 'error' : key;
  const label = t(`status.${key}`);
  const onDark = variant === 'dot';

  return (
    <span
      className={`flex items-center gap-1.5 text-xs ${onDark ? 'text-white/90' : 'text-gray-500'}`}
      title={label}
      data-testid="ws-status"
      data-state={key}
    >
      <span className={`w-2 h-2 rounded-full ${(onDark ? dotColorsOnDark : dotColorsOnLight)[dotKey]}`} />
      <span className="hidden sm:inline">{label}</span>
    </span>
  );
}
