import { describe, it, expect, vi } from "vitest";
import { createRouterTransport, ConnectError, Code } from "@connectrpc/connect";
import { create } from "@bufbuild/protobuf";
import { timestampFromDate } from "@bufbuild/protobuf/wkt";

import {
  GridApiAdapter,
  createGridClient,
  normalizeConnectError,
} from "./index.js";
import type { DependencyEdge, StateInfo } from "./types.js";
import {
  StateService,
  ListStatesResponseSchema,
  ListAllEdgesResponseSchema,
  StateInfoSchema,
  GetStateInfoResponseSchema,
  BackendConfigSchema,
  DependencyEdgeSchema,
  OutputKeySchema,
} from "../gen/state/v1/state_pb.js";

describe("createGridClient", () => {
  it("calls listStates via Connect transport", async () => {
    const createdAt = timestampFromDate(new Date("2024-01-01T00:00:00Z"));
    const updatedAt = timestampFromDate(new Date("2024-01-01T01:00:00Z"));

    const transport = createRouterTransport(({ service }) => {
      service(StateService, {
        async listStates() {
          return create(ListStatesResponseSchema, {
            states: [
              create(StateInfoSchema, {
                guid: "guid-1",
                logicId: "logic-1",
                createdAt,
                updatedAt,
                sizeBytes: 0n,
                dependencyLogicIds: [],
              }),
            ],
          });
        },
      });
    });

    const client = createGridClient(transport);
    const response = await client.listStates({});

    expect(response.states).toHaveLength(1);
    expect(response.states[0].logicId).toBe("logic-1");
  });
});

describe("GridApiAdapter", () => {
  it("transforms protobuf responses into plain state info", async () => {
    const stateCreatedAt = timestampFromDate(new Date("2024-02-01T00:00:00Z"));
    const stateUpdatedAt = timestampFromDate(new Date("2024-02-01T02:30:00Z"));
    const edgeCreatedAt = timestampFromDate(new Date("2024-02-01T03:00:00Z"));
    const edgeUpdatedAt = timestampFromDate(new Date("2024-02-01T04:15:00Z"));

    const backendConfig = create(BackendConfigSchema, {
      address: "https://grid/states/consumer-guid",
      lockAddress: "https://grid/states/consumer-guid/lock",
      unlockAddress: "https://grid/states/consumer-guid/unlock",
    });

    const dependencyEdge = create(DependencyEdgeSchema, {
      id: 1n,
      fromGuid: "producer-guid",
      fromLogicId: "network/prod",
      fromOutput: "vpc_id",
      toGuid: "consumer-guid",
      toLogicId: "app/prod",
      status: "clean",
      createdAt: edgeCreatedAt,
      updatedAt: edgeUpdatedAt,
    });

    const stateInfoResponse = create(GetStateInfoResponseSchema, {
      guid: "consumer-guid",
      logicId: "app/prod",
      backendConfig,
      dependencies: [dependencyEdge],
      dependents: [],
      outputs: [create(OutputKeySchema, { key: "vpc_id", sensitive: false })],
      createdAt: stateCreatedAt,
      updatedAt: stateUpdatedAt,
      computedStatus: "clean",
    });

    const listStatesResponse = create(ListStatesResponseSchema, {
      states: [
        create(StateInfoSchema, {
          guid: "consumer-guid",
          logicId: "app/prod",
          createdAt: stateCreatedAt,
          updatedAt: stateUpdatedAt,
          sizeBytes: 0n,
          dependencyLogicIds: ["network/prod"],
          computedStatus: "clean",
        }),
      ],
    });

    const getStateInfoMock = vi.fn(async () => stateInfoResponse);

    const transport = createRouterTransport(({ service }) => {
      service(StateService, {
        async listStates() {
          return listStatesResponse;
        },
        getStateInfo: getStateInfoMock,
      });
    });

    const adapter = new GridApiAdapter(transport);

    const states = await adapter.listStates();

    expect(getStateInfoMock).toHaveBeenCalledTimes(1);
    expect(states).toHaveLength(1);

    const state = states[0];
    expect(state.guid).toBe("consumer-guid");
    expect(state.logic_id).toBe("app/prod");
    expect(state.backend_config).toEqual({
      address: "https://grid/states/consumer-guid",
      lock_address: "https://grid/states/consumer-guid/lock",
      unlock_address: "https://grid/states/consumer-guid/unlock",
    });
    expect(state.dependency_logic_ids).toEqual(["network/prod"]);
    expect(state.dependencies[0]).toMatchObject({
      id: 1,
      from_logic_id: "network/prod",
      to_logic_id: "app/prod",
      status: "clean",
    });
    expect(state.outputs[0]).toEqual({ key: "vpc_id", sensitive: false });
  });

  it("converts dependency edges returned by getAllEdges", async () => {
    const now = new Date("2024-03-01T01:02:03.456Z");
    const later = new Date("2024-03-02T04:05:06.789Z");

    const dependencyEdge = create(DependencyEdgeSchema, {
      id: 42n,
      fromGuid: "from-guid",
      fromLogicId: "prod/network",
      fromOutput: "db_endpoint",
      toGuid: "to-guid",
      toLogicId: "prod/app",
      toInputName: "database_endpoint",
      status: "dirty",
      inDigest: "123",
      outDigest: "456",
      mockValueJson: '{"value":"mock"}',
      lastInAt: timestampFromDate(now),
      lastOutAt: timestampFromDate(later),
      createdAt: timestampFromDate(now),
      updatedAt: timestampFromDate(later),
    });

    const transport = createRouterTransport(({ service }) => {
      service(StateService, {
        async listAllEdges() {
          return create(ListAllEdgesResponseSchema, {
            edges: [dependencyEdge],
          });
        },
      });
    });

    const adapter = new GridApiAdapter(transport);
    const edges = await adapter.getAllEdges();

    expect(edges).toHaveLength(1);
    expect(edges[0]).toEqual({
      id: 42,
      from_guid: "from-guid",
      from_logic_id: "prod/network",
      from_output: "db_endpoint",
      to_guid: "to-guid",
      to_logic_id: "prod/app",
      to_input_name: "database_endpoint",
      status: "dirty",
      in_digest: "123",
      out_digest: "456",
      mock_value_json: '{"value":"mock"}',
      last_in_at: now.toISOString(),
      last_out_at: later.toISOString(),
      created_at: now.toISOString(),
      updated_at: later.toISOString(),
    });
  });

  it("returns null when getStateInfo reports not found", async () => {
    const transport = createRouterTransport(({ service }) => {
      service(StateService, {
        async getStateInfo() {
          throw new ConnectError("missing", Code.NotFound);
        },
      });
    });

    const adapter = new GridApiAdapter(transport);
    const result = await adapter.getStateInfo("does-not-exist");

    expect(result).toBeNull();
  });

  it("lists dependencies for a logic ID", async () => {
    const transport = createRouterTransport(() => {});
    const adapter = new GridApiAdapter(transport);

    const dependency: DependencyEdge = {
      id: 55,
      from_guid: "network-guid",
      from_logic_id: "network/prod",
      from_output: "vpc_id",
      to_guid: "app-guid",
      to_logic_id: "app/prod",
      to_input_name: "network_vpc_id",
      status: "clean",
      created_at: "2024-04-01T00:00:00.000Z",
      updated_at: "2024-04-01T01:00:00.000Z",
    };

    const stateInfo: StateInfo = {
      guid: "app-guid",
      logic_id: "app/prod",
      created_at: "2024-04-01T00:00:00.000Z",
      updated_at: "2024-04-01T01:00:00.000Z",
      dependency_logic_ids: ["network/prod"],
      backend_config: {
        address: "https://grid/states/app-guid",
        lock_address: "https://grid/states/app-guid/lock",
        unlock_address: "https://grid/states/app-guid/unlock",
      },
      dependencies: [dependency],
      dependents: [],
      outputs: [],
    };

    vi.spyOn(adapter, "getStateInfo").mockResolvedValue(stateInfo);

    const dependencies = await adapter.listDependencies("app/prod");

    expect(dependencies).toEqual([dependency]);
  });

  it("lists dependents for a logic ID", async () => {
    const transport = createRouterTransport(() => {});
    const adapter = new GridApiAdapter(transport);

    const dependent: DependencyEdge = {
      id: 99,
      from_guid: "app-guid",
      from_logic_id: "app/prod",
      from_output: "endpoint",
      to_guid: "consumer-guid",
      to_logic_id: "consumer/prod",
      status: "dirty",
      created_at: "2024-05-01T00:00:00.000Z",
      updated_at: "2024-05-01T01:00:00.000Z",
    };

    const stateInfo: StateInfo = {
      guid: "app-guid",
      logic_id: "app/prod",
      created_at: "2024-05-01T00:00:00.000Z",
      updated_at: "2024-05-01T01:00:00.000Z",
      dependency_logic_ids: [],
      backend_config: {
        address: "https://grid/states/app-guid",
        lock_address: "https://grid/states/app-guid/lock",
        unlock_address: "https://grid/states/app-guid/unlock",
      },
      dependencies: [],
      dependents: [dependent],
      outputs: [],
    };

    vi.spyOn(adapter, "getStateInfo").mockResolvedValue(stateInfo);

    const dependents = await adapter.listDependents("app/prod");

    expect(dependents).toEqual([dependent]);
  });
});

describe("normalizeConnectError", () => {
  it("maps ConnectError instances to user-friendly errors", () => {
    const error = new ConnectError("State not found", Code.NotFound);
    const friendly = normalizeConnectError(error, "Fetching state");

    expect(friendly).toEqual({
      title: "Not Found",
      message: "Fetching state: The requested state could not be found.",
      code: Code.NotFound,
      canRetry: false,
    });
  });

  it("falls back to a generic error for unknown inputs", () => {
    const friendly = normalizeConnectError(new Error("boom"));

    expect(friendly.title).toBe("Unexpected Error");
    expect(friendly.code).toBe(Code.Unknown);
    expect(friendly.canRetry).toBe(true);
  });
});
