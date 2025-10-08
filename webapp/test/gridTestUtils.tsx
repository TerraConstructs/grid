import React, { ReactElement } from 'react';
import { render, RenderOptions } from '@testing-library/react';
import { GridApiAdapter } from '@tcons/grid';
import { GridProvider } from '../src/context/GridContext';

interface RenderWithGridOptions extends Omit<RenderOptions, 'wrapper'> {
  api?: GridApiAdapter;
  transport?: unknown;
}

/**
 * Render a component with Grid API context for testing.
 *
 * @param ui - Component to render
 * @param options - Render options including mock transport or handlers
 *
 * @example
 * ```typescript
 * const { getByText } = renderWithGrid(<MyComponent />, {
 *   handlers: {
 *     listStates: () => ({ states: [] })
 *   }
 * });
 * ```
 */
export function renderWithGrid(
  ui: ReactElement,
  {
    transport,
    api,
    ...renderOptions
  }: RenderWithGridOptions = {}
) {
  if (!api) {
    throw new Error('renderWithGrid requires an api instance for testing');
  }

  const mockTransport = transport ?? {};
  const gridApi = api;

  const Wrapper = ({ children }: { children: React.ReactNode }) => (
    <GridProvider api={gridApi} transport={mockTransport as any}>
      {children}
    </GridProvider>
  );

  return render(ui, { wrapper: Wrapper, ...renderOptions });
}
