/**
 * Grid TypeScript SDK for Terraform state management
 *
 * This SDK provides a simple re-export of the generated protobuf types.
 * Consumers should use Connect RPC client libraries (@connectrpc/connect) to interact with the API.
 *
 * @example
 * ```typescript
 * import { createPromiseClient } from "@connectrpc/connect";
 * import { createConnectTransport } from "@connectrpc/connect-node";
 * import { StateService } from "@tcons/grid";
 *
 * const transport = createConnectTransport({ baseUrl: "http://localhost:8080" });
 * const client = createPromiseClient(StateService, transport);
 * ```
 */

import { createClient, Client, Transport } from "@connectrpc/connect";
import { StateService } from "../gen/state/v1/state_connect.js";
import type {
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
} from "../gen/state/v1/state_pb.js";

// Re-export the Connect service definition
export { StateService } from "../gen/state/v1/state_connect.js";

// Re-export all protobuf message classes and types
export {
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
  StateInfo,
  BackendConfig,
  StateLock,
  LockInfo,
} from "../gen/state/v1/state_pb.js";

/**
 * GridClient provides a convenient wrapper around the StateService Connect client.
 *
 * @example
 * ```typescript
 * import { GridClient } from "@tcons/grid";
 * import { createConnectTransport } from "@connectrpc/connect-node";
 *
 * const transport = createConnectTransport({ baseUrl: "http://localhost:8080" });
 * const client = new GridClient("http://localhost:8080", transport);
 *
 * const response = await client.createState({
 *   guid: "018e8c5e-7890-7000-8000-123456789abc",
 *   logicId: "production-us-east"
 * });
 * ```
 */
export class GridClient {
  private client: Client<typeof StateService>;

  constructor(baseUrl: string, transport: Transport) {
    this.client = createClient(StateService, transport);
  }

  async createState(request: CreateStateRequest): Promise<CreateStateResponse> {
    return this.client.createState(request);
  }

  async listStates(request: ListStatesRequest): Promise<ListStatesResponse> {
    return this.client.listStates(request);
  }

  async getStateConfig(request: GetStateConfigRequest): Promise<GetStateConfigResponse> {
    return this.client.getStateConfig(request);
  }

  async getStateLock(request: GetStateLockRequest): Promise<GetStateLockResponse> {
    return this.client.getStateLock(request);
  }

  async unlockState(request: UnlockStateRequest): Promise<UnlockStateResponse> {
    return this.client.unlockState(request);
  }
}
