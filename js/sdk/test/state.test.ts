/**
 * Tests for Grid TypeScript SDK
 *
 * These tests use Vitest with mocked Connect transport to verify SDK behavior
 * without requiring a running server.
 */

import { describe, it, expect } from "vitest";
import { createRouterTransport } from "@connectrpc/connect";
import { GridClient } from "../src/index.js";
import { StateService } from "../gen/state/v1/state_connect.js";
import {
  CreateStateResponse,
  ListStatesResponse,
  GetStateConfigResponse,
  GetStateLockResponse,
  UnlockStateResponse,
  StateInfo,
  BackendConfig,
  StateLock,
  LockInfo,
} from "../gen/state/v1/state_pb.js";
import { Timestamp } from "@bufbuild/protobuf";
import { ConnectError, Code } from "@connectrpc/connect";

describe("GridClient", () => {
  describe("createState", () => {
    it("should create a state successfully", async () => {
      const mockGuid = "018e8c5e-7890-7000-8000-123456789abc";
      const mockLogicId = "production-us-east";

      const transport = createRouterTransport(({ service }) => {
        service(StateService, {
          createState: async (req) => {
            return new CreateStateResponse({
              guid: req.guid,
              logicId: req.logicId,
              backendConfig: new BackendConfig({
                address: `http://localhost:8080/tfstate/${req.guid}`,
                lockAddress: `http://localhost:8080/tfstate/${req.guid}/lock`,
                unlockAddress: `http://localhost:8080/tfstate/${req.guid}/unlock`,
              }),
            });
          },
        });
      });

      const client = new GridClient("http://localhost:8080", transport);
      const response = await client.createState({
        guid: mockGuid,
        logicId: mockLogicId,
      });

      expect(response.guid).toBe(mockGuid);
      expect(response.logicId).toBe(mockLogicId);
      expect(response.backendConfig).toBeDefined();
      expect(response.backendConfig?.address).toContain(mockGuid);
    });

    it("should handle duplicate logic_id error", async () => {
      const transport = createRouterTransport(({ service }) => {
        service(StateService, {
          createState: async () => {
            throw new ConnectError("State already exists", Code.AlreadyExists);
          },
        });
      });

      const client = new GridClient("http://localhost:8080", transport);

      await expect(
        client.createState({
          guid: "018e8c5e-7890-7000-8000-123456789abc",
          logicId: "production-us-east",
        })
      ).rejects.toThrow();
    });

    it("should handle invalid GUID error", async () => {
      const transport = createRouterTransport(({ service }) => {
        service(StateService, {
          createState: async () => {
            throw new ConnectError("Invalid GUID format", Code.InvalidArgument);
          },
        });
      });

      const client = new GridClient("http://localhost:8080", transport);

      await expect(
        client.createState({
          guid: "invalid-uuid",
          logicId: "production-us-east",
        })
      ).rejects.toThrow();
    });
  });

  describe("listStates", () => {
    it("should list states successfully", async () => {
      const transport = createRouterTransport(({ service }) => {
        service(StateService, {
          listStates: async () => {
            return new ListStatesResponse({
              states: [
                new StateInfo({
                  guid: "018e8c5e-7890-7000-8000-123456789abc",
                  logicId: "production-us-east",
                  locked: false,
                  sizeBytes: BigInt(1024),
                  createdAt: Timestamp.now(),
                  updatedAt: Timestamp.now(),
                }),
                new StateInfo({
                  guid: "018e8c5e-7890-7000-8000-123456789def",
                  logicId: "staging-us-west",
                  locked: true,
                  sizeBytes: BigInt(2048),
                  createdAt: Timestamp.now(),
                  updatedAt: Timestamp.now(),
                }),
              ],
            });
          },
        });
      });

      const client = new GridClient("http://localhost:8080", transport);
      const response = await client.listStates({});

      expect(response.states).toHaveLength(2);
      expect(response.states[0].guid).toBe("018e8c5e-7890-7000-8000-123456789abc");
      expect(response.states[1].locked).toBe(true);
    });

    it("should return empty list when no states exist", async () => {
      const transport = createRouterTransport(({ service }) => {
        service(StateService, {
          listStates: async () => {
            return new ListStatesResponse({ states: [] });
          },
        });
      });

      const client = new GridClient("http://localhost:8080", transport);
      const response = await client.listStates({});

      expect(response.states).toHaveLength(0);
    });
  });

  describe("getStateConfig", () => {
    it("should get state config successfully", async () => {
      const mockGuid = "018e8c5e-7890-7000-8000-123456789abc";
      const mockLogicId = "production-us-east";

      const transport = createRouterTransport(({ service }) => {
        service(StateService, {
          getStateConfig: async (req) => {
            return new GetStateConfigResponse({
              guid: mockGuid,
              backendConfig: new BackendConfig({
                address: `http://localhost:8080/tfstate/${mockGuid}`,
                lockAddress: `http://localhost:8080/tfstate/${mockGuid}/lock`,
                unlockAddress: `http://localhost:8080/tfstate/${mockGuid}/unlock`,
              }),
            });
          },
        });
      });

      const client = new GridClient("http://localhost:8080", transport);
      const response = await client.getStateConfig({ logicId: mockLogicId });

      expect(response.guid).toBe(mockGuid);
      expect(response.backendConfig).toBeDefined();
      expect(response.backendConfig?.address).toContain(mockGuid);
    });

    it("should handle not found error", async () => {
      const transport = createRouterTransport(({ service }) => {
        service(StateService, {
          getStateConfig: async () => {
            throw new ConnectError("State not found", Code.NotFound);
          },
        });
      });

      const client = new GridClient("http://localhost:8080", transport);

      await expect(
        client.getStateConfig({ logicId: "nonexistent" })
      ).rejects.toThrow();
    });
  });

  describe("getStateLock", () => {
    it("should get unlocked state", async () => {
      const transport = createRouterTransport(({ service }) => {
        service(StateService, {
          getStateLock: async () => {
            return new GetStateLockResponse({
              lock: new StateLock({ locked: false }),
            });
          },
        });
      });

      const client = new GridClient("http://localhost:8080", transport);
      const response = await client.getStateLock({
        guid: "018e8c5e-7890-7000-8000-123456789abc",
      });

      expect(response.lock?.locked).toBe(false);
      expect(response.lock?.info).toBeUndefined();
    });

    it("should get locked state with lock info", async () => {
      const transport = createRouterTransport(({ service }) => {
        service(StateService, {
          getStateLock: async () => {
            return new GetStateLockResponse({
              lock: new StateLock({
                locked: true,
                info: new LockInfo({
                  id: "lock-123",
                  operation: "apply",
                  who: "user@example.com",
                  version: "1.0.0",
                  created: Timestamp.now(),
                  path: "/path/to/terraform",
                }),
              }),
            });
          },
        });
      });

      const client = new GridClient("http://localhost:8080", transport);
      const response = await client.getStateLock({
        guid: "018e8c5e-7890-7000-8000-123456789abc",
      });

      expect(response.lock?.locked).toBe(true);
      expect(response.lock?.info?.id).toBe("lock-123");
      expect(response.lock?.info?.operation).toBe("apply");
    });

    it("should handle state not found", async () => {
      const transport = createRouterTransport(({ service }) => {
        service(StateService, {
          getStateLock: async () => {
            throw new ConnectError("State not found", Code.NotFound);
          },
        });
      });

      const client = new GridClient("http://localhost:8080", transport);

      await expect(
        client.getStateLock({ guid: "nonexistent" })
      ).rejects.toThrow();
    });
  });

  describe("unlockState", () => {
    it("should unlock state successfully", async () => {
      const transport = createRouterTransport(({ service }) => {
        service(StateService, {
          unlockState: async () => {
            return new UnlockStateResponse({
              lock: new StateLock({ locked: false }),
            });
          },
        });
      });

      const client = new GridClient("http://localhost:8080", transport);
      const response = await client.unlockState({
        guid: "018e8c5e-7890-7000-8000-123456789abc",
        lockId: "lock-123",
      });

      expect(response.lock?.locked).toBe(false);
    });

    it("should handle lock ID mismatch", async () => {
      const transport = createRouterTransport(({ service }) => {
        service(StateService, {
          unlockState: async () => {
            throw new ConnectError("Lock ID mismatch", Code.InvalidArgument);
          },
        });
      });

      const client = new GridClient("http://localhost:8080", transport);

      await expect(
        client.unlockState({
          guid: "018e8c5e-7890-7000-8000-123456789abc",
          lockId: "wrong-lock-id",
        })
      ).rejects.toThrow();
    });

    it("should handle state not found", async () => {
      const transport = createRouterTransport(({ service }) => {
        service(StateService, {
          unlockState: async () => {
            throw new ConnectError("State not found", Code.NotFound);
          },
        });
      });

      const client = new GridClient("http://localhost:8080", transport);

      await expect(
        client.unlockState({
          guid: "nonexistent",
          lockId: "lock-123",
        })
      ).rejects.toThrow();
    });
  });
});
