import { useWebSocket } from '../hooks/useWebSocket';

export interface WebSocketStatusProps {
  /** 'pill' (default) is the original colored-badge look; 'dot' is a
   * compact light-on-dark variant for use on the (Gopher Blue) header;
   * 'dot-light' is the same but for light backgrounds (the bottom status
   * bar, see StatusBar.tsx). */
  variant?: 'pill' | 'dot' | 'dot-light';
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

export function WebSocketStatus({ variant = 'pill' }: WebSocketStatusProps) {
  const { state } = useWebSocket();
  const { connected, connecting, error } = state;

  const label = connected ? 'Connected' : connecting ? 'Connecting...' : error ? 'Disconnected' : 'Unknown';
  const dotKey = connected ? 'connected' : connecting ? 'connecting' : error ? 'error' : 'unknown';

  if (variant === 'dot' || variant === 'dot-light') {
    const onDark = variant === 'dot';
    return (
      <span
        className={`flex items-center gap-1.5 text-xs ${onDark ? 'text-white/90' : 'text-gray-500'}`}
        title={label}
      >
        <span className={`w-2 h-2 rounded-full ${(onDark ? dotColorsOnDark : dotColorsOnLight)[dotKey]}`} />
        <span className="hidden sm:inline">{label}</span>
      </span>
    );
  }

  if (connected) {
    return (
      <div className="flex items-center gap-1 px-2 py-1 bg-green-100 text-green-700 rounded text-xs font-medium">
        <span className="w-2 h-2 bg-green-500 rounded-full animate-pulse" />
        <span>Connected</span>
      </div>
    );
  }

  if (connecting) {
    return (
      <div className="flex items-center gap-1 px-2 py-1 bg-yellow-100 text-yellow-700 rounded text-xs font-medium">
        <span className="w-2 h-2 bg-yellow-500 rounded-full animate-pulse" />
        <span>Connecting...</span>
      </div>
    );
  }

  if (error) {
    return (
      <div className="flex items-center gap-1 px-2 py-1 bg-red-100 text-red-700 rounded text-xs font-medium cursor-pointer hover:bg-red-200">
        <span className="w-2 h-2 bg-red-500 rounded-full" />
        <span>Disconnected</span>
      </div>
    );
  }

  return (
    <div className="flex items-center gap-1 px-2 py-1 bg-gray-100 text-gray-700 rounded text-xs font-medium">
      <span className="w-2 h-2 bg-gray-500 rounded-full" />
      <span>Unknown</span>
    </div>
  );
}
