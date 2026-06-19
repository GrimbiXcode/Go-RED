
import { FlowEditor } from './components/FlowEditor';
import { FlowProvider } from './components/FlowProvider';
import { ToastProvider } from './components/ToastNotification';

function App() {
  return (
    <div className="h-screen w-full bg-gray-50">
      <FlowProvider>
        <ToastProvider>
          <FlowEditor />
        </ToastProvider>
      </FlowProvider>
    </div>
  );
}

export default App;