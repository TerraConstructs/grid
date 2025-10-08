# Research: Connect RPC TypeScript/Web Integration

**Feature**: Live Dashboard Integration (004-wire-up-grid)
**Date**: 2025-10-06
**Status**: Complete

## Overview

This document contains research findings for integrating the Grid React dashboard with the live Grid API using Connect RPC, focusing on TypeScript SDK patterns, build configuration, testing strategies, and minimal adapter interface design.

## 1. Connect RPC Web Testing Patterns

### Decision
Use `createRouterTransport()` from `@connectrpc/connect` for unit and integration testing with React Context API for transport injection.

### Rationale
- Creates in-memory server running nearly identical code to production
- Enables testing without network requests or external dependencies
- Supports full service implementation with error handling and state
- Provides schema-based serialization ensuring requests match production flow
- Type-safe mocking using generated protobuf types
- Context API provides centralized transport management across component tree

### Implementation Pattern

**In-Memory Mock Transport**:
```typescript
import { createRouterTransport } from "@connectrpc/connect";
import { StateService } from "@tcons/grid";
import { Code, ConnectError } from "@connectrpc/connect";

const mockTransport = createRouterTransport(({ service }) => {
  service(StateService, {
    listStates: () => ({
      states: [/* mock data */]
    }),
    getStateInfo: (request) => {
      if (request.state.case === 'logicId' && request.state.value === 'not-found') {
        throw new ConnectError("State not found", Code.NotFound);
      }
      return {/* mock response */};
    }
  });
});
```

**React Context Provider**:
```typescript
// src/context/GridContext.tsx
import React, { createContext, useContext, useMemo } from 'react';
import { Transport } from '@connectrpc/connect';

interface GridContextValue {
  transport: Transport;
}

const GridContext = createContext<GridContextValue | null>(null);

export function GridProvider({ transport, children }: GridProviderProps) {
  return (
    <GridContext.Provider value={{ transport }}>
      {children}
    </GridContext.Provider>
  );
}

export function useGridTransport() {
  const context = useContext(GridContext);
  if (!context) throw new Error('useGridTransport must be used within GridProvider');
  return context.transport;
}
```

### References
- https://connectrpc.com/docs/web/testing/
- https://github.com/connectrpc/examples-es

---

## 2. TypeScript SDK Build Configuration

### Decision
Use separate `tsc` builds for ESM and CJS with shared type definitions, outputting to `lib/{esm,cjs,types}/`.

### Rationale
- TypeScript doesn't natively support dual ESM/CJS emit from single compilation
- Separate builds allow proper `package.json` module markers in each directory
- Explicit control over module resolution and type definitions
- Compatible with pnpm `workspace:*` protocol for local development
- Declaration files reference `lib/types/` not `src/` for publishable SDK

### Build Configuration

**TypeScript Configs**:
```json
// tsconfig.build.json (base)
{
  "extends": "./tsconfig.json",
  "compilerOptions": {
    "outDir": "lib/types",
    "declaration": true,
    "declarationMap": true,
    "composite": true
  }
}

// tsconfig.esm.json
{
  "extends": "./tsconfig.build.json",
  "compilerOptions": {
    "outDir": "lib/esm",
    "module": "ES2022",
    "moduleResolution": "bundler",
    "declaration": false
  }
}

// tsconfig.cjs.json
{
  "extends": "./tsconfig.build.json",
  "compilerOptions": {
    "outDir": "lib/cjs",
    "module": "CommonJS",
    "moduleResolution": "node",
    "declaration": false
  }
}
```

**package.json Configuration**:
```json
{
  "name": "@tcons/grid",
  "type": "module",
  "exports": {
    ".": {
      "import": {
        "types": "./lib/types/src/index.d.ts",
        "default": "./lib/esm/src/index.js"
      },
      "require": {
        "types": "./lib/types/src/index.d.ts",
        "default": "./lib/cjs/src/index.js"
      }
    }
  },
  "files": ["lib/"],
  "scripts": {
    "build": "npm run clean && npm run build:types && npm run build:esm && npm run build:cjs && npm run add-package-markers",
    "build:types": "tsc -p tsconfig.build.json --emitDeclarationOnly --declarationMap",
    "build:esm": "tsc -p tsconfig.esm.json",
    "build:cjs": "tsc -p tsconfig.cjs.json",
    "add-package-markers": "echo '{\"type\":\"commonjs\"}' > lib/cjs/package.json && echo '{\"type\":\"module\"}' > lib/esm/package.json"
  }
}
```

**Webapp Dependency** (using pnpm workspace protocol):
```json
// webapp/package.json
{
  "dependencies": {
    "@tcons/grid": "workspace:*"
  }
}
```

### References
- https://medium.com/ekino-france/supporting-dual-package-for-cjs-and-esm-in-typescript-library-b5feabac1357
- https://pnpm.io/workspaces

---

## 3. Vite Tree-Shaking Requirements

### Decision
Current SDK structure already optimal: use named exports with ESM format.

### Rationale
- Named exports are statically analyzable by Vite/Rollup
- Enables dead code elimination at method level
- ESM format required for tree-shaking (CJS doesn't support it)
- Vite automatically eliminates unused named exports in production builds

### Requirements Met
✅ ESM format (`"type": "module"` in package.json)
✅ Named exports (not default exports)
✅ Flat export structure (no nested namespaces)
✅ Pure annotations preserved (Vite handles automatically)

**Correct Pattern** (already in use):
```typescript
// js/sdk/src/index.ts
export { StateService } from "../gen/state/v1/state_connect.js";
export { StateInfo, DependencyEdge, OutputKey } from "../gen/state/v1/state_pb.js";
```

**Verification**:
```bash
cd webapp && npm run build
npx vite-bundle-visualizer  # Optional: analyze bundle
```

### References
- https://vite.dev/guide/features (tree-shaking section)
- https://web.dev/articles/reduce-javascript-payloads-with-tree-shaking

---

## 4. Error Normalization Pattern

### Decision
Implement adapter layer with complete Connect RPC `Code` enum mapping to human-readable messages.

### Rationale
- Centralizes error handling logic
- Provides consistent UX across all API calls
- Separates technical gRPC codes from user-facing messages
- Allows context-specific error messages
- Easier to internationalize (i18n) in future

### Complete Code Enum Mapping

| Code | Value | Use Case | User Message | Can Retry |
|------|-------|----------|--------------|-----------|
| `Canceled` | 1 | User cancels operation | "The operation was canceled." | No |
| `Unknown` | 2 | Unknown server error | "An unexpected error occurred." | Yes |
| `InvalidArgument` | 3 | Client sent invalid data | "The provided input is invalid." | No |
| `DeadlineExceeded` | 4 | Request timeout | "The request took too long." | Yes |
| `NotFound` | 5 | Resource doesn't exist | "The requested state could not be found." | No |
| `AlreadyExists` | 6 | Resource conflict | "A state with this identifier already exists." | No |
| `PermissionDenied` | 7 | Authorization failure | "You don't have permission to perform this action." | No |
| `ResourceExhausted` | 8 | Rate limit/quota exceeded | "Too many requests. Please wait." | Yes |
| `FailedPrecondition` | 9 | System not in required state | "The operation cannot be performed in the current state." | No |
| `Aborted` | 10 | Concurrent modification conflict | "The operation was aborted due to a conflict." | Yes |
| `OutOfRange` | 11 | Parameter out of valid range | "The provided value is out of range." | No |
| `Unimplemented` | 12 | Feature not supported | "This feature is not currently supported." | No |
| `Internal` | 13 | Internal server error | "A server error occurred." | Yes |
| `Unavailable` | 14 | Service temporarily down | "The service is temporarily unavailable." | Yes |
| `DataLoss` | 15 | Unrecoverable data corruption | "A server error occurred." | Yes |
| `Unauthenticated` | 16 | Missing/invalid credentials | "Please sign in to continue." | No |

### Implementation

**Error Handler**:
```typescript
// src/errors.ts
import { ConnectError, Code } from '@connectrpc/connect';

export interface UserFriendlyError {
  title: string;
  message: string;
  code: Code;
  canRetry: boolean;
}

export function normalizeConnectError(error: unknown, context?: string): UserFriendlyError {
  if (!(error instanceof ConnectError)) {
    return {
      title: 'Unexpected Error',
      message: 'An unexpected error occurred. Please try again.',
      code: Code.Unknown,
      canRetry: true
    };
  }

  const baseContext = context ? `${context}: ` : '';

  switch (error.code) {
    case Code.NotFound:
      return {
        title: 'Not Found',
        message: `${baseContext}The requested state could not be found.`,
        code: error.code,
        canRetry: false
      };
    // ... other cases
    default:
      return {
        title: 'Error',
        message: error.rawMessage || 'An error occurred.',
        code: error.code,
        canRetry: true
      };
  }
}
```

### References
- https://connectrpc.com/docs/web/errors/
- https://grpc.io/docs/guides/status-codes/

---

## 5. Minimal API Surface Design

### Decision
Create thin adapter interface matching existing `mockApi` methods, mapping to Connect RPC calls.

### Rationale
- Maintains existing component contracts (no refactoring)
- Provides abstraction layer for easier testing
- Maps cleanly to proto RPCs
- Allows gradual migration from mock to real API
- Simplifies type conversions between protobuf and plain TypeScript

### mockApi Interface Analysis

**Current Interface**:
```typescript
interface MockApi {
  listStates(): Promise<StateInfo[]>;
  getStateInfo(logicId: string): Promise<StateInfo | null>;
  listDependencies(logicId: string): Promise<DependencyEdge[]>;
  listDependents(logicId: string): Promise<DependencyEdge[]>;
  getAllEdges(): Promise<DependencyEdge[]>;
}
```

**RPC Mapping**:
| mockApi Method | Proto RPC | Implementation Notes |
|----------------|-----------|---------------------|
| `listStates()` | `ListStates` | Direct 1:1 mapping |
| `getStateInfo(logicId)` | `GetStateInfo` | Returns comprehensive view |
| `listDependencies(logicId)` | Part of `GetStateInfo` | From `dependencies` field |
| `listDependents(logicId)` | Part of `GetStateInfo` | From `dependents` field |
| `getAllEdges()` | **`ListAllEdges` (NEW)** | New RPC needed for performance |

### Adapter Implementation Pattern

```typescript
// js/sdk/src/adapter.ts
export class GridApiAdapter {
  private client: ReturnType<typeof createClient<typeof StateService>>;

  constructor(transport: Transport) {
    this.client = createClient(StateService, transport);
  }

  async listStates(): Promise<StateInfo[]> {
    const response = await this.client.listStates({});
    return Promise.all(
      response.states.map(state => this.getStateInfo(state.logicId))
    ).then(states => states.filter(Boolean) as StateInfo[]);
  }

  async getStateInfo(logicId: string): Promise<StateInfo | null> {
    try {
      const response = await this.client.getStateInfo({
        state: { case: 'logicId', value: logicId }
      });
      return convertProtoStateInfo(response);
    } catch (error) {
      if (error instanceof ConnectError && error.code === Code.NotFound) {
        return null;
      }
      throw error;
    }
  }

  async getAllEdges(): Promise<DependencyEdge[]> {
    // Option 1: Aggregate from all states (current)
    const states = await this.listStates();
    const edges = new Map<number, DependencyEdge>();
    states.forEach(state => {
      state.dependencies.forEach(e => edges.set(e.id, e));
      state.dependents.forEach(e => edges.set(e.id, e));
    });
    return Array.from(edges.values());

    // Option 2: Use new ListAllEdges RPC (recommended)
    // const response = await this.client.listAllEdges({});
    // return response.edges.map(convertProtoDependencyEdge);
  }
}
```

### Type Conversion Helpers

```typescript
function timestampToISO(ts: Timestamp | undefined): string {
  if (!ts) return new Date().toISOString();
  return new Date(Number(ts.seconds) * 1000 + ts.nanos / 1000000).toISOString();
}

function convertProtoDependencyEdge(edge: ProtoDependencyEdge): DependencyEdge {
  return {
    id: Number(edge.id),
    from_guid: edge.fromGuid,
    from_logic_id: edge.fromLogicId,
    from_output: edge.fromOutput,
    to_guid: edge.toGuid,
    to_logic_id: edge.toLogicId,
    to_input_name: edge.toInputName,
    status: edge.status,
    in_digest: edge.inDigest,
    out_digest: edge.outDigest,
    mock_value_json: edge.mockValueJson,
    last_in_at: edge.lastInAt ? timestampToISO(edge.lastInAt) : undefined,
    last_out_at: edge.lastOutAt ? timestampToISO(edge.lastOutAt) : undefined,
    created_at: timestampToISO(edge.createdAt),
    updated_at: timestampToISO(edge.updatedAt)
  };
}
```

---

## 6. Required Proto Enhancements

### New RPC: ListAllEdges

**Justification**: Current `mockApi.getAllEdges()` requires N+1 queries (ListStates + GetStateInfo for each). A dedicated RPC improves performance.

**Proposed Proto Definition**:
```protobuf
// proto/state/v1/state.proto
rpc ListAllEdges(ListAllEdgesRequest) returns (ListAllEdgesResponse);

message ListAllEdgesRequest {
  // No parameters for PoC (return all edges)
  // Future: Add optional status_filter, pagination
}

message ListAllEdgesResponse {
  repeated DependencyEdge edges = 1;
}
```

**Implementation Impact**:
- Repository: Add `ListAllEdges() ([]*models.DependencyEdge, error)`
- Service: Add `ListAllEdges(ctx, req) (*statev1.ListAllEdgesResponse, error)`
- Handler: Wire up Connect RPC handler
- Adapter: Replace aggregation logic with direct RPC call

---

## Summary

### Decisions Made

1. **Testing**: Use `createRouterTransport()` with React Context API for transport injection
2. **Build**: Dual ESM/CJS compilation with separate `lib/{esm,cjs,types}/` outputs
3. **Tree-Shaking**: Current named exports pattern already optimal for Vite
4. **Errors**: Implement comprehensive Code enum → user message mapping in adapter layer
5. **API Surface**: GridApiAdapter wrapping 3 RPCs: `ListStates`, `GetStateInfo`, `ListAllEdges` (new)

### Required Work

**Proto Changes**:
- Add `ListAllEdges` RPC + messages

**Go Backend**:
- Implement repository method for listing all edges
- Add service method
- Wire up Connect RPC handler

**TypeScript SDK** (`js/sdk`):
- Update build scripts for dual compilation
- Create adapter layer (`src/adapter.ts`)
- Implement error normalization (`src/errors.ts`)
- Add type conversion helpers (`src/types.ts`)
- Configure exports in package.json

**Webapp**:
- Create GridContext provider
- Replace mockApi with GridApiAdapter
- Update component imports to use adapter types

### Performance Notes

- **getAllEdges**: N+1 query problem solved by new `ListAllEdges` RPC
- **listStates**: Still requires N queries for full details (acceptable for PoC, 10-50 states)
- Future: Consider batching or enhanced `ListStates` returning full state info

---

## 7. UI Testing Utilities & Refresh Scope

### Testing Library Choice
- `@testing-library/jest-dom`
  - Pros: gives semantic matchers (`toBeInTheDocument`, `toHaveAccessibleName`, `toHaveAttribute`) that map directly to the dashboard acceptance criteria; installs via `import '@testing-library/jest-dom/vitest'` with no extra configuration; proven compatibility with Vitest + React 18 when using the default `jsdom` environment.
  - Cons: adds ~3.5 KB gzip to the Vitest runtime, but only in tests.
- `happy-dom`
  - Pros: lighter-weight DOM implementation than `jsdom`; can speed up low-level rendering tests when the component tree is simple.
  - Cons: does not provide assertion helpers, so we still fall back to basic `expect` checks; lacks some browser APIs (`ResizeObserver`, scroll metrics) used by charting libs and toast animations, which our scenario specs rely on; diverges from the `jsdom` environment expected by upstream `@testing-library/user-event`, leading to subtle pointer-event gaps.

**Decision**: Stick with `@testing-library/jest-dom` on Vitest's `jsdom` environment. The richer matchers reduce bespoke helpers for accessibility assertions, and we avoid the API coverage gaps we would need to polyfill in `happy-dom`.

```
The same is true for the fantastic jest-dom library. Luckily, although the library’s name suggests otherwise, its makers made it compatible with Vitest.

npm install --save-dev @testing-library/jest-dom
After installing the jest-dom dependency, we can activate it in our Vitest setup file. If you don’t have one yet, create it.

// tests/setup.ts
import "@testing-library/jest-dom/vitest";

// ...
// In vitest.config.js add (if you haven't already)
export default defineConfig({
  test: {
    // ...
    setupFiles: ["./tests/setup.js"];
    // ...
  },
})
And just like that, we can use jest-dom matchers with Vitest!

// Examples
expect(getByTestId("button")).toBeDisabled();
expect(getByTestId("button")).toBeInDocument();
expect(getByTestId("invalid-form")).toBeInvalid();
expect(getByText("Visible Example")).toBeVisible();
```

References:
- https://markus.oberlehner.net/blog/using-testing-library-jest-dom-with-vitest
- https://testing-library.com/docs/react-testing-library/setup
- https://github.com/testing-library/jest-dom?tab=readme-ov-file#with-vitest

### Refresh Strategy
- Constrain the MVP to manual refresh only (no polling) per FR-022 and preserve the selected state across refreshes so operators don't lose context when reloading live data.

---

**Research Status**: ✅ **COMPLETE**
**Next Phase**: Design & Contracts (Phase 1)
