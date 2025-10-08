import { createContext, useContext, ReactNode } from 'react';
import { GridApiAdapter } from '@tcons/grid';
import type { Transport } from '@connectrpc/connect';

interface GridContextValue {
  api: GridApiAdapter;
  transport: Transport;
}

const GridContext = createContext<GridContextValue | null>(null);

interface GridProviderProps {
  api: GridApiAdapter;
  transport: Transport;
  children: ReactNode;
}

export function GridProvider({ api, transport, children }: GridProviderProps) {
  return (
    <GridContext.Provider value={{ api, transport }}>
      {children}
    </GridContext.Provider>
  );
}

export function useGrid(): GridContextValue {
  const context = useContext(GridContext);
  if (!context) {
    throw new Error('useGrid must be used within a GridProvider');
  }
  return context;
}
