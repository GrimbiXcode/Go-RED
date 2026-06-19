import { useWebSocket } from '../hooks/useWebSocket';

export function WebSocketStatus() {
  const { state } = useWebSocket();
  const { connected, connecting, error } = state;

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
