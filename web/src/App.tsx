
import { Routes, Route } from 'react-router-dom';
import { FlowEditor } from './components/FlowEditor';
import { FlowProvider } from './components/FlowProvider';
import { ToastProvider } from './components/ToastNotification';
import { StyleGuide } from './pages/StyleGuide';

function Editor() {
  return (
    <FlowProvider>
      <ToastProvider>
        <FlowEditor />
      </ToastProvider>
    </FlowProvider>
  );
}

function App() {
  return (
    <div className="h-screen w-full bg-gray-50">
      {import.meta.env.DEV ? (
        <Routes>
          <Route path="/styleguide" element={<StyleGuide />} />
          <Route path="*" element={<Editor />} />
        </Routes>
      ) : (
        <Editor />
      )}
    </div>
  );
}

export default App;