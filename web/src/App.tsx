import { useEffect } from 'react';
import { Routes, Route, Navigate } from 'react-router-dom';
import { FlowEditor } from './components/FlowEditor';
import { ToastHost } from './components/ToastNotification';
import { StyleGuide } from './pages/StyleGuide';
import { bindServerEvents } from './store/bindServerEvents';

function App() {
  useEffect(() => bindServerEvents(), []);

  return (
    <div className="h-screen w-full bg-gray-50">
      <Routes>
        <Route path="/flow/:flowId" element={<FlowEditor />} />
        <Route path="/" element={<FlowEditor />} />
        {import.meta.env.DEV && <Route path="/styleguide" element={<StyleGuide />} />}
        <Route path="*" element={<Navigate to="/" replace />} />
      </Routes>
      <ToastHost />
    </div>
  );
}

export default App;
