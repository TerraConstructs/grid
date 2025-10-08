import React, { ReactElement } from 'react';
import { render, RenderOptions } from '@testing-library/react';
import { createRouterTransport } from '@connectrpc/connect';
import { StateService } from '@tcons/grid/gen/state/v1/state_pb';

/**
 * Grid test utilities for creating mock transports and rendering components
 * with Grid API context.
 */

/**
 * Create a mock Connect router transport for testing.
 * This allows components to make RPC calls without hitting a real server.
 *
 * @example
 * ```typescript
 * const mockTransport = createMockGridTransport({
 *   listStates: () => ({ states: [/* mock data *\/] }),
 *   getStateInfo: () => ({ /* mock state info *\/ })
 * });
 * ```
 */
export function createMockGridTransport(handlers: Partial<typeof StateService.methods>) {
  return createRouterTransport(({ service }) => {
    service(StateService, handlers as typeof StateService.methods);
  });
}

interface GridProviderWrapperProps {
  children: React.ReactNode;
  transport: ReturnType<typeof createRouterTransport>;
}

/**
 * Wrapper component that provides Grid context to test components.
 * This will be updated once GridContext is implemented in T034.
 */
function GridProviderWrapper({ children, transport }: GridProviderWrapperProps) {
  // TODO: Replace with actual GridProvider once implemented in T034
  // For now, just render children directly
  return <>{children}</>;
}

interface RenderWithGridOptions extends Omit<RenderOptions, 'wrapper'> {
  transport?: ReturnType<typeof createRouterTransport>;
  handlers?: Partial<typeof StateService.methods>;
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
    handlers = {},
    ...renderOptions
  }: RenderWithGridOptions = {}
) {
  const mockTransport = transport || createMockGridTransport(handlers);

  const Wrapper = ({ children }: { children: React.ReactNode }) => (
    <GridProviderWrapper transport={mockTransport}>
      {children}
    </GridProviderWrapper>
  );

  return render(ui, { wrapper: Wrapper, ...renderOptions });
}
