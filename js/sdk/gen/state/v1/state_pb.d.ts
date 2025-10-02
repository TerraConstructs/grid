import type { GenFile, GenMessage, GenService } from "@bufbuild/protobuf/codegenv1";
import type { Timestamp } from "@bufbuild/protobuf/wkt";
import type { Message } from "@bufbuild/protobuf";
/**
 * Describes the file state/v1/state.proto.
 */
export declare const file_state_v1_state: GenFile;
/**
 * CreateStateRequest creates a new state using a client-generated GUID.
 *
 * @generated from message state.v1.CreateStateRequest
 */
export type CreateStateRequest = Message<"state.v1.CreateStateRequest"> & {
    /**
     * @generated from field: string guid = 1;
     */
    guid: string;
    /**
     * @generated from field: string logic_id = 2;
     */
    logicId: string;
};
/**
 * Describes the message state.v1.CreateStateRequest.
 * Use `create(CreateStateRequestSchema)` to create a new message.
 */
export declare const CreateStateRequestSchema: GenMessage<CreateStateRequest>;
/**
 * CreateStateResponse confirms creation and returns backend config.
 *
 * @generated from message state.v1.CreateStateResponse
 */
export type CreateStateResponse = Message<"state.v1.CreateStateResponse"> & {
    /**
     * @generated from field: string guid = 1;
     */
    guid: string;
    /**
     * @generated from field: string logic_id = 2;
     */
    logicId: string;
    /**
     * @generated from field: state.v1.BackendConfig backend_config = 3;
     */
    backendConfig?: BackendConfig;
};
/**
 * Describes the message state.v1.CreateStateResponse.
 * Use `create(CreateStateResponseSchema)` to create a new message.
 */
export declare const CreateStateResponseSchema: GenMessage<CreateStateResponse>;
/**
 * ListStatesRequest requests all states.
 *
 * @generated from message state.v1.ListStatesRequest
 */
export type ListStatesRequest = Message<"state.v1.ListStatesRequest"> & {};
/**
 * Describes the message state.v1.ListStatesRequest.
 * Use `create(ListStatesRequestSchema)` to create a new message.
 */
export declare const ListStatesRequestSchema: GenMessage<ListStatesRequest>;
/**
 * ListStatesResponse returns all states with basic info.
 *
 * @generated from message state.v1.ListStatesResponse
 */
export type ListStatesResponse = Message<"state.v1.ListStatesResponse"> & {
    /**
     * @generated from field: repeated state.v1.StateInfo states = 1;
     */
    states: StateInfo[];
};
/**
 * Describes the message state.v1.ListStatesResponse.
 * Use `create(ListStatesResponseSchema)` to create a new message.
 */
export declare const ListStatesResponseSchema: GenMessage<ListStatesResponse>;
/**
 * StateInfo is summary information for a state.
 *
 * @generated from message state.v1.StateInfo
 */
export type StateInfo = Message<"state.v1.StateInfo"> & {
    /**
     * @generated from field: string guid = 1;
     */
    guid: string;
    /**
     * @generated from field: string logic_id = 2;
     */
    logicId: string;
    /**
     * @generated from field: bool locked = 3;
     */
    locked: boolean;
    /**
     * @generated from field: google.protobuf.Timestamp created_at = 4;
     */
    createdAt?: Timestamp;
    /**
     * @generated from field: google.protobuf.Timestamp updated_at = 5;
     */
    updatedAt?: Timestamp;
    /**
     * @generated from field: int64 size_bytes = 6;
     */
    sizeBytes: bigint;
};
/**
 * Describes the message state.v1.StateInfo.
 * Use `create(StateInfoSchema)` to create a new message.
 */
export declare const StateInfoSchema: GenMessage<StateInfo>;
/**
 * BackendConfig contains Terraform backend configuration URLs.
 *
 * @generated from message state.v1.BackendConfig
 */
export type BackendConfig = Message<"state.v1.BackendConfig"> & {
    /**
     * @generated from field: string address = 1;
     */
    address: string;
    /**
     * @generated from field: string lock_address = 2;
     */
    lockAddress: string;
    /**
     * @generated from field: string unlock_address = 3;
     */
    unlockAddress: string;
};
/**
 * Describes the message state.v1.BackendConfig.
 * Use `create(BackendConfigSchema)` to create a new message.
 */
export declare const BackendConfigSchema: GenMessage<BackendConfig>;
/**
 * GetStateConfigRequest retrieves backend config for existing state.
 *
 * @generated from message state.v1.GetStateConfigRequest
 */
export type GetStateConfigRequest = Message<"state.v1.GetStateConfigRequest"> & {
    /**
     * @generated from field: string logic_id = 1;
     */
    logicId: string;
};
/**
 * Describes the message state.v1.GetStateConfigRequest.
 * Use `create(GetStateConfigRequestSchema)` to create a new message.
 */
export declare const GetStateConfigRequestSchema: GenMessage<GetStateConfigRequest>;
/**
 * GetStateConfigResponse returns backend config.
 *
 * @generated from message state.v1.GetStateConfigResponse
 */
export type GetStateConfigResponse = Message<"state.v1.GetStateConfigResponse"> & {
    /**
     * @generated from field: string guid = 1;
     */
    guid: string;
    /**
     * @generated from field: state.v1.BackendConfig backend_config = 2;
     */
    backendConfig?: BackendConfig;
};
/**
 * Describes the message state.v1.GetStateConfigResponse.
 * Use `create(GetStateConfigResponseSchema)` to create a new message.
 */
export declare const GetStateConfigResponseSchema: GenMessage<GetStateConfigResponse>;
/**
 * GetStateLockRequest fetches current lock metadata by GUID.
 *
 * @generated from message state.v1.GetStateLockRequest
 */
export type GetStateLockRequest = Message<"state.v1.GetStateLockRequest"> & {
    /**
     * @generated from field: string guid = 1;
     */
    guid: string;
};
/**
 * Describes the message state.v1.GetStateLockRequest.
 * Use `create(GetStateLockRequestSchema)` to create a new message.
 */
export declare const GetStateLockRequestSchema: GenMessage<GetStateLockRequest>;
/**
 * LockInfo mirrors Terraform's lock payload.
 *
 * @generated from message state.v1.LockInfo
 */
export type LockInfo = Message<"state.v1.LockInfo"> & {
    /**
     * @generated from field: string id = 1;
     */
    id: string;
    /**
     * @generated from field: string operation = 2;
     */
    operation: string;
    /**
     * @generated from field: string info = 3;
     */
    info: string;
    /**
     * @generated from field: string who = 4;
     */
    who: string;
    /**
     * @generated from field: string version = 5;
     */
    version: string;
    /**
     * @generated from field: google.protobuf.Timestamp created = 6;
     */
    created?: Timestamp;
    /**
     * @generated from field: string path = 7;
     */
    path: string;
};
/**
 * Describes the message state.v1.LockInfo.
 * Use `create(LockInfoSchema)` to create a new message.
 */
export declare const LockInfoSchema: GenMessage<LockInfo>;
/**
 * StateLock response wrapper indicating lock state plus metadata when present.
 *
 * @generated from message state.v1.StateLock
 */
export type StateLock = Message<"state.v1.StateLock"> & {
    /**
     * @generated from field: bool locked = 1;
     */
    locked: boolean;
    /**
     * @generated from field: state.v1.LockInfo info = 2;
     */
    info?: LockInfo;
};
/**
 * Describes the message state.v1.StateLock.
 * Use `create(StateLockSchema)` to create a new message.
 */
export declare const StateLockSchema: GenMessage<StateLock>;
/**
 * GetStateLockResponse returns current lock status.
 *
 * @generated from message state.v1.GetStateLockResponse
 */
export type GetStateLockResponse = Message<"state.v1.GetStateLockResponse"> & {
    /**
     * @generated from field: state.v1.StateLock lock = 1;
     */
    lock?: StateLock;
};
/**
 * Describes the message state.v1.GetStateLockResponse.
 * Use `create(GetStateLockResponseSchema)` to create a new message.
 */
export declare const GetStateLockResponseSchema: GenMessage<GetStateLockResponse>;
/**
 * UnlockStateRequest releases a lock given the current lock ID.
 *
 * @generated from message state.v1.UnlockStateRequest
 */
export type UnlockStateRequest = Message<"state.v1.UnlockStateRequest"> & {
    /**
     * @generated from field: string guid = 1;
     */
    guid: string;
    /**
     * @generated from field: string lock_id = 2;
     */
    lockId: string;
};
/**
 * Describes the message state.v1.UnlockStateRequest.
 * Use `create(UnlockStateRequestSchema)` to create a new message.
 */
export declare const UnlockStateRequestSchema: GenMessage<UnlockStateRequest>;
/**
 * UnlockStateResponse mirrors GetStateLockResponse after unlock attempt.
 *
 * @generated from message state.v1.UnlockStateResponse
 */
export type UnlockStateResponse = Message<"state.v1.UnlockStateResponse"> & {
    /**
     * @generated from field: state.v1.StateLock lock = 1;
     */
    lock?: StateLock;
};
/**
 * Describes the message state.v1.UnlockStateResponse.
 * Use `create(UnlockStateResponseSchema)` to create a new message.
 */
export declare const UnlockStateResponseSchema: GenMessage<UnlockStateResponse>;
/**
 * StateService provides remote state management for Terraform/OpenTofu clients.
 *
 * @generated from service state.v1.StateService
 */
export declare const StateService: GenService<{
    /**
     * CreateState creates a new state with client-generated GUID and logic ID.
     *
     * @generated from rpc state.v1.StateService.CreateState
     */
    createState: {
        methodKind: "unary";
        input: typeof CreateStateRequestSchema;
        output: typeof CreateStateResponseSchema;
    };
    /**
     * ListStates returns all states with summary information.
     *
     * @generated from rpc state.v1.StateService.ListStates
     */
    listStates: {
        methodKind: "unary";
        input: typeof ListStatesRequestSchema;
        output: typeof ListStatesResponseSchema;
    };
    /**
     * GetStateConfig retrieves backend configuration for an existing state by logic ID.
     *
     * @generated from rpc state.v1.StateService.GetStateConfig
     */
    getStateConfig: {
        methodKind: "unary";
        input: typeof GetStateConfigRequestSchema;
        output: typeof GetStateConfigResponseSchema;
    };
    /**
     * GetStateLock inspects the current lock metadata for a state by GUID.
     *
     * @generated from rpc state.v1.StateService.GetStateLock
     */
    getStateLock: {
        methodKind: "unary";
        input: typeof GetStateLockRequestSchema;
        output: typeof GetStateLockResponseSchema;
    };
    /**
     * UnlockState releases a lock using the lock ID provided by Terraform/OpenTofu.
     *
     * @generated from rpc state.v1.StateService.UnlockState
     */
    unlockState: {
        methodKind: "unary";
        input: typeof UnlockStateRequestSchema;
        output: typeof UnlockStateResponseSchema;
    };
}>;
