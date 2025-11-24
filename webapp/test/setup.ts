import '@testing-library/jest-dom/vitest';
import { vi } from 'vitest';

// Complete React Flow mocking implementation for Jest/Vitest
// Based on: https://reactflow.dev/learn/advanced-use/testing
class ResizeObserver {
  callback: globalThis.ResizeObserverCallback;

  constructor(callback: globalThis.ResizeObserverCallback) {
    this.callback = callback;
  }

  observe(target: Element) {
    this.callback([{ target } as globalThis.ResizeObserverEntry], this);
  }

  unobserve() {}

  disconnect() {}
}

class DOMMatrixReadOnly {
  m22: number;
  constructor(transform: string) {
    const scale = transform?.match(/scale\(([1-9.])\)/)?.[1];
    this.m22 = scale !== undefined ? +scale : 1;
  }
}

// Only run the shim once when requested
let init = false;

export const mockReactFlow = () => {
  if (init) return;
  init = true;

  global.ResizeObserver = ResizeObserver;

  // eslint-disable-next-line @typescript-eslint/ban-ts-comment
  // @ts-ignore
  global.DOMMatrixReadOnly = DOMMatrixReadOnly;

  Object.defineProperties(global.HTMLElement.prototype, {
    offsetHeight: {
      get() {
        return parseFloat(this.style.height) || 1;
      },
    },
    offsetWidth: {
      get() {
        return parseFloat(this.style.width) || 1;
      },
    },
  });

  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  (global.SVGElement as any).prototype.getBBox = () => ({
    x: 0,
    y: 0,
    width: 0,
    height: 0,
  });
};

// Initialize React Flow mocks
mockReactFlow();

// Suppress React Flow NaN warnings in tests (these are harmless - caused by jsdom limitations)
const originalConsoleError = console.error;
console.error = (...args) => {
  // Filter out React Flow NaN attribute warnings
  const message = args[0]?.toString() || '';
  if (message.includes('Received NaN for the') && message.includes('attribute')) {
    return;
  }
  originalConsoleError(...args);
};

// Suppress unhandled d3-drag/d3-zoom errors in React Flow during tests
// These occur because jsdom doesn't fully implement browser event handling
process.on('uncaughtException', (error: Error) => {
  // Suppress d3-drag/d3-zoom errors that occur in React Flow tests
  if (error.message && error.message.includes("Cannot read properties of null (reading 'document')")) {
    // Silently ignore - this is a known jsdom limitation with d3-zoom
    return;
  }
  // Re-throw other errors
  throw error;
});

process.on('unhandledRejection', (reason: any) => {
  // Suppress d3-related promise rejections
  if (reason && reason.message && reason.message.includes("Cannot read properties of null (reading 'document')")) {
    return;
  }
  // Re-throw other rejections
  throw reason;
});

// Mock fetch to return disabled auth config
const originalFetch = global.fetch;
global.fetch = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
  const url = typeof input === 'string' ? input : input instanceof URL ? input.toString() : input.url;

  // Mock auth config endpoint
  if (url.includes('/auth/config')) {
    return new Response(JSON.stringify({ mode: 'disabled' }), {
      status: 200,
      headers: { 'Content-Type': 'application/json' },
    });
  }

  // Pass through other requests
  return originalFetch(input, init);
});
