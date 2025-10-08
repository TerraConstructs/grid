import { Transport, Code, ConnectError } from '@connectrpc/connect';
import type { Timestamp } from '@bufbuild/protobuf/wkt';
import type {
  DependencyEdge as ProtoDependencyEdge,
  GetStateInfoResponse,
  OutputKey as ProtoOutputKey,
  BackendConfig as ProtoBackendConfig,
} from '../gen/state/v1/state_pb.js';
import type {
  StateInfo,
  DependencyEdge,
  OutputKey,
  BackendConfig,
} from './models/state-info.js';
import {
  createGridClient,
  type StateServiceClient,
} from './client.js';

/**
 * Convert protobuf Timestamp to ISO 8601 string.
 */
function timestampToISO(ts: Timestamp | undefined): string {
  if (!ts) return new Date().toISOString();
  return new Date(Number(ts.seconds) * 1000 + ts.nanos / 1000000).toISOString();
}

/**
 * Convert protobuf DependencyEdge to plain TypeScript type.
 */
function convertProtoDependencyEdge(edge: ProtoDependencyEdge): DependencyEdge {
  return {
    id: Number(edge.id),
    from_guid: edge.fromGuid,
    from_logic_id: edge.fromLogicId,
    from_output: edge.fromOutput,
    to_guid: edge.toGuid,
    to_logic_id: edge.toLogicId,
    to_input_name: edge.toInputName,
    status: edge.status as DependencyEdge['status'],
    in_digest: edge.inDigest,
    out_digest: edge.outDigest,
    mock_value_json: edge.mockValueJson,
    last_in_at: edge.lastInAt ? timestampToISO(edge.lastInAt) : undefined,
    last_out_at: edge.lastOutAt ? timestampToISO(edge.lastOutAt) : undefined,
    created_at: timestampToISO(edge.createdAt),
    updated_at: timestampToISO(edge.updatedAt),
  };
}

/**
 * Convert protobuf OutputKey to plain TypeScript type.
 */
function convertProtoOutputKey(output: ProtoOutputKey): OutputKey {
  return {
    key: output.key,
    sensitive: output.sensitive,
  };
}

/**
 * Convert protobuf BackendConfig to plain TypeScript type.
 */
function convertProtoBackendConfig(
  config: ProtoBackendConfig | undefined
): BackendConfig {
  if (!config) {
    throw new Error('BackendConfig is required but was undefined');
  }
  return {
    address: config.address,
    lock_address: config.lockAddress,
    unlock_address: config.unlockAddress,
  };
}

/**
 * Convert protobuf GetStateInfoResponse to plain StateInfo type.
 */
function convertProtoStateInfo(response: GetStateInfoResponse): StateInfo {
  const dependencyLogicIds = Array.from(
    new Set(response.dependencies.map((e: ProtoDependencyEdge) => e.fromLogicId))
  );

  return {
    guid: response.guid,
    logic_id: response.logicId,
    created_at: timestampToISO(response.createdAt),
    updated_at: timestampToISO(response.updatedAt),
    computed_status: response.computedStatus as StateInfo['computed_status'],
    dependency_logic_ids: dependencyLogicIds,
    backend_config: convertProtoBackendConfig(response.backendConfig),
    dependencies: response.dependencies.map(convertProtoDependencyEdge),
    dependents: response.dependents.map(convertProtoDependencyEdge),
    outputs: response.outputs.map(convertProtoOutputKey),
  };
}

/**
 * Grid API Adapter providing a mockApi-compatible interface.
 *
 * Wraps Connect RPC client calls and converts protobuf types to plain
 * TypeScript types suitable for React components.
 *
 * @example
 * ```typescript
 * import { createConnectTransport } from '@connectrpc/connect-web';
 * import { GridApiAdapter } from '@tcons/grid';
 *
 * const transport = createConnectTransport({
 *   baseUrl: 'http://localhost:8080'
 * });
 * const api = new GridApiAdapter(transport);
 *
 * const states = await api.listStates();
 * const stateInfo = await api.getStateInfo('prod/network');
 * ```
 */
export class GridApiAdapter {
  private client: StateServiceClient;

  constructor(transport: Transport) {
    this.client = createGridClient(transport);
  }

  /**
   * List all states with comprehensive information.
   * Performs N queries to fetch full state info for each state.
   *
   * @returns Array of StateInfo objects
   */
  async listStates(): Promise<StateInfo[]> {
    const response = await this.client.listStates({});

    // Fetch full info for each state
    const stateInfoPromises = response.states.map((state: { logicId: string }) =>
      this.getStateInfo(state.logicId)
    );

    const stateInfos = await Promise.all(stateInfoPromises);

    // Filter out nulls (states that were deleted between list and get)
    return stateInfos.filter((info: StateInfo | null): info is StateInfo => info !== null);
  }

  /**
   * Get comprehensive information about a specific state.
   *
   * @param logicId - The state's logic ID
   * @returns StateInfo or null if not found
   */
  async getStateInfo(logicId: string): Promise<StateInfo | null> {
    try {
      const response = await this.client.getStateInfo({
        state: { case: 'logicId', value: logicId },
      });
      return convertProtoStateInfo(response);
    } catch (error) {
      if (error instanceof ConnectError && error.code === Code.NotFound) {
        return null;
      }
      throw error;
    }
  }

  /**
   * List incoming dependency edges for a specific state.
   *
   * @param logicId - The consumer state's logic ID
   * @returns Array of incoming dependency edges
   */
  async listDependencies(logicId: string): Promise<DependencyEdge[]> {
    const stateInfo = await this.getStateInfo(logicId);
    return stateInfo?.dependencies || [];
  }

  /**
   * List outgoing dependency edges for a specific state.
   *
   * @param logicId - The producer state's logic ID
   * @returns Array of outgoing dependency edges
   */
  async listDependents(logicId: string): Promise<DependencyEdge[]> {
    const stateInfo = await this.getStateInfo(logicId);
    return stateInfo?.dependents || [];
  }

  /**
   * Get all dependency edges in the system.
   *
   * @returns Array of all dependency edges
   */
  async getAllEdges(): Promise<DependencyEdge[]> {
    const response = await this.client.listAllEdges({});
    return response.edges.map(convertProtoDependencyEdge);
  }
}
