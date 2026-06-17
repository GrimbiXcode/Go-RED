
import { FlowEditor } from './components/FlowEditor';
import { FlowProvider } from './components/FlowProvider';

function App() {
  return (
    <div className="h-screen w-full bg-gray-50">
      <FlowProvider>
        <FlowEditor />
      </FlowProvider>
    </div>
  );
}

export default App;