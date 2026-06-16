import React, { createContext, useContext, ReactNode } from 'react';
import { useFlows } from '../hooks/useFlows';

type FlowContextType = ReturnType<typeof useFlows>;

const FlowContext = createContext<FlowContextType | undefined>(undefined);

interface FlowProviderProps {
  children: ReactNode;
}

export function FlowProvider({ children }: FlowProviderProps) {
  const flowState = useFlows();

  return (
    <FlowContext.Provider value={flowState}>
      {children}
    </FlowContext.Provider>
  );
}

export function useFlowContext() {
  const context = useContext(FlowContext);
  if (context === undefined) {
    throw new Error('useFlowContext must be used within a FlowProvider');
  }
  return context;
}