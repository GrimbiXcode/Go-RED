import { useEffect } from 'react';
import { Routes, Route, Navigate } from 'react-router-dom';
import { FlowEditor } from './components/FlowEditor';
import { ToastHost } from './components/ToastNotification';
import { TokenPrompt } from './components/TokenPrompt';
import { StyleGuide } from './pages/StyleGuide';
import { bindServerEvents } from './store/bindServerEvents';
import { signedIn } from './lib/auth';
import { fetchAuthUser } from './utils/api';

function App() {
  useEffect(() => bindServerEvents(), []);
  useEffect(() => {
    // A 401 here opens the sign-in through the API layer; nothing else to do.
    fetchAuthUser()
      .then(signedIn)
      .catch(() => undefined);
  }, []);

  return (
    <div className="h-screen w-full bg-app text-fg">
      <Routes>
        <Route path="/flow/:flowId" element={<FlowEditor />} />
        <Route path="/" element={<FlowEditor />} />
        <Route path="/styleguide" element={<StyleGuide />} />
        <Route path="*" element={<Navigate to="/" replace />} />
      </Routes>
      <ToastHost />
      <TokenPrompt />
    </div>
  );
}

export default App;
