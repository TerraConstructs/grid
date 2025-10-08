/**
 * Grid TypeScript SDK for Terraform state management
 *
 * This SDK provides both low-level Connect RPC access and a high-level
 * adapter interface for dashboard and UI integration.
 *
 * @example
 * ```typescript
 * // High-level adapter (recommended for dashboards)
 * import { GridApiAdapter, createGridTransport } from "@tcons/grid";
 *
 * const transport = createGridTransport("http://localhost:8080");
 * const api = new GridApiAdapter(transport);
 * const states = await api.listStates();
 *
 * // Low-level Connect RPC client
 * import { createPromiseClient } from "@connectrpc/connect";
 * import { createConnectTransport } from "@connectrpc/connect-node";
 * import { StateService } from "@tcons/grid";
 *
 * const transport = createConnectTransport({ baseUrl: "http://localhost:8080" });
 * const client = createPromiseClient(StateService, transport);
 * ```
 */

// ===== High-level API (Dashboard/UI) =====

/**
 * GridApiAdapter - Main interface for dashboard integration
 * Provides mockApi-compatible methods with plain TypeScript types
 */
export { GridApiAdapter } from './adapter.js';

/**
 * Transport and client factories
 */
export { createGridTransport, createGridClient } from './client.js';
export type { StateServiceClient } from './client.js';

/**
 * Plain TypeScript model types (not protobuf)
 */
export type {
  StateInfo,
  BackendConfig,
  DependencyEdge,
  EdgeStatus,
  OutputKey,
} from './types.js';

/**
 * Error handling utilities
 */
export { normalizeConnectError } from './errors.js';
export type { UserFriendlyError } from './errors.js';

// ===== Low-level Connect RPC API =====

/**
 * Connect service definition
 */
export { StateService } from "../gen/state/v1/state_pb.js";

/**
 * Generated protobuf message types
 */
export type {
  CreateStateRequest,
  CreateStateResponse,
  ListStatesRequest,
  ListStatesResponse,
  GetStateConfigRequest,
  GetStateConfigResponse,
  GetStateLockRequest,
  GetStateLockResponse,
  UnlockStateRequest,
  UnlockStateResponse,
  GetStateInfoRequest,
  GetStateInfoResponse,
  DependencyEdge as ProtoDependencyEdge,
  OutputKey as ProtoOutputKey,
  BackendConfig as ProtoBackendConfig,
} from "../gen/state/v1/state_pb.js";
