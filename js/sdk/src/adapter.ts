import { Transport, Code, ConnectError } from '@connectrpc/connect';
import type { Timestamp } from '@bufbuild/protobuf/wkt';
import type {
  DependencyEdge as ProtoDependencyEdge,
  GetStateInfoResponse,
  OutputKey as ProtoOutputKey,
  BackendConfig as ProtoBackendConfig,
  LabelValue as ProtoLabelValue,
  StateInfo as ProtoStateInfo,
} from '../gen/state/v1/state_pb.js';
import type {
  StateSummary,
  StateInfo,
  DependencyEdge,
  OutputKey,
  BackendConfig,
  LabelScalar,
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

function convertProtoLabelValue(label: ProtoLabelValue): LabelScalar | undefined {
  switch (label.value.case) {
    case 'stringValue':
      return label.value.value;
    case 'numberValue':
      return label.value.value;
    case 'boolValue':
      return label.value.value;
    default:
      return undefined;
  }
}

function convertProtoLabels(
  labels: Record<string, ProtoLabelValue> | undefined
): Record<string, LabelScalar> | undefined {
  if (!labels) {
    return undefined;
  }

  const entries = Object.entries(labels);
  if (entries.length === 0) {
    return undefined;
  }

  const result: Record<string, LabelScalar> = {};
  for (const [key, value] of entries) {
    const scalar = convertProtoLabelValue(value);
    if (scalar !== undefined) {
      result[key] = scalar;
    }
  }

  return Object.keys(result).length > 0 ? result : undefined;
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

  const labels = convertProtoLabels(response.labels);

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
    size_bytes: Number(response.sizeBytes),
    ...(labels ? { labels } : {}),
  };
}

/**
 * Convert protobuf StateInfo (from ListStatesResponse) to plain StateSummary type.
 * Uses efficient count fields from backend instead of fetching full relationship data.
 */
function convertProtoStateSummary(protoState: ProtoStateInfo): StateSummary {
  const labels = convertProtoLabels(protoState.labels);

  return {
    guid: protoState.guid,
    logic_id: protoState.logicId,
    locked: protoState.locked,
    created_at: timestampToISO(protoState.createdAt),
    updated_at: timestampToISO(protoState.updatedAt),
    size_bytes: Number(protoState.sizeBytes),
    computed_status: protoState.computedStatus as StateSummary['computed_status'],
    dependency_logic_ids: protoState.dependencyLogicIds || [],
    dependencies_count: protoState.dependenciesCount,
    dependents_count: protoState.dependentsCount,
    outputs_count: protoState.outputsCount,
    ...(labels ? { labels } : {}),
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
   * Create a new state with client-generated GUID and logic ID.
   *
   * @param request - State creation parameters (guid, logicId, labels)
   * @returns CreateStateResponse with backend config
   */
  async createState(request: {
    guid: string;
    logicId: string;
    labels?: Record<string, string>;
  }): Promise<{
    guid: string;
    logicId: string;
    backendConfig: BackendConfig;
  } | null> {
    try {
    const response = await this.client.createState(request);
    return {
      guid: response.guid,
      logicId: response.logicId,
      backendConfig: convertProtoBackendConfig(response.backendConfig),
    };
    } catch (error) {
      if (error instanceof ConnectError) {
        const errorMessage = error.message || 'Failed to create state';
        // Check for permission denied (code 7 = PERMISSION_DENIED)
        if (error.code === 7 || errorMessage.toLowerCase().includes('permission')) {
          throw new Error('Permission denied: You do not have access to create this state.');
        }
        throw new Error(errorMessage);
      }
      throw error;
    }
  }

  /**
   * List all states with summary information (optimized for list rendering).
   * Returns lightweight StateSummary objects with efficient count fields.
   * Use getStateInfo() to fetch full StateInfo with relationships when needed.
   *
   * @param options - Optional filter/includeLabels/includeStatus flags
   * @returns Array of StateSummary objects
   */
  async listStates(options?: {
    filter?: string;
    includeLabels?: boolean;
    includeStatus?: boolean;
  }): Promise<StateSummary[]> {
    const { filter, includeLabels = true, includeStatus = true } = options ?? {};

    const request: Record<string, unknown> = {
      includeLabels,
      includeStatus,
    };
    if (filter && filter.trim() !== '') {
      request.filter = filter;
    }

    const listResponse = await this.client.listStates(request);

    // Convert proto StateInfo to StateSummary (uses count fields, not full relationships)
    return listResponse.states.map((protoState) =>
      convertProtoStateSummary(protoState as ProtoStateInfo)
    );
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

  /**
   * Get the current label validation policy.
   *
   * @returns Label policy with version and JSON definition
   * @throws ConnectError if policy not found
   */
  async getLabelPolicy(): Promise<{
    version: number;
    policyJson: string;
    createdAt: Date;
    updatedAt: Date;
  } | null> {
    try {
      const response = await this.client.getLabelPolicy({});
      return {
        version: response.version,
        policyJson: response.policyJson,
        createdAt: response.createdAt ? new Date(timestampToISO(response.createdAt)) : new Date(),
        updatedAt: response.updatedAt ? new Date(timestampToISO(response.updatedAt)) : new Date(),
      };
    } catch (error) {
      if (error instanceof ConnectError && error.code === Code.NotFound) {
        return null;
      }
      throw error;
    }
  }

  /**
   * Set or update the label validation policy.
   *
   * @param policyJson - JSON string of policy definition
   * @returns Confirmation with new version and timestamp
   */
  async setLabelPolicy(policyJson: string): Promise<{
    version: number;
    updatedAt: Date;
  }> {
    const response = await this.client.setLabelPolicy({ policyJson });
    return {
      version: response.version,
      updatedAt: response.updatedAt ? new Date(timestampToISO(response.updatedAt)) : new Date(),
    };
  }
}
