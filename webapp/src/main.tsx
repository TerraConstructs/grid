import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
import App from './App.tsx';
import { GridProvider } from './context/GridContext';
import { gridApi, gridTransport } from './services/gridApi';
import './index.css';

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <GridProvider api={gridApi} transport={gridTransport}>
      <App />
    </GridProvider>
  </StrictMode>
);
