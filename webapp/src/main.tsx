import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
import App from './App.tsx';
import { GridProvider } from './context/GridContext';
import { AuthProvider } from './context/AuthContext';
import { gridApi, gridTransport } from './services/gridApi';
import './index.css';

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <AuthProvider>
      <GridProvider api={gridApi} transport={gridTransport}>
        <App />
      </GridProvider>
    </AuthProvider>
  </StrictMode>
);
