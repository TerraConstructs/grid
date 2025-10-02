/**
 * StateService provides remote state management for Terraform/OpenTofu clients.
 *
 * @generated from service state.v1.StateService
 */
export declare const StateService: {
    readonly typeName: "state.v1.StateService";
    readonly methods: {
        /**
         * CreateState creates a new state with client-generated GUID and logic ID.
         *
         * @generated from rpc state.v1.StateService.CreateState
         */
        readonly createState: {
            readonly name: "CreateState";
            readonly I: any;
            readonly O: any;
            readonly kind: any;
        };
        /**
         * ListStates returns all states with summary information.
         *
         * @generated from rpc state.v1.StateService.ListStates
         */
        readonly listStates: {
            readonly name: "ListStates";
            readonly I: any;
            readonly O: any;
            readonly kind: any;
        };
        /**
         * GetStateConfig retrieves backend configuration for an existing state by logic ID.
         *
         * @generated from rpc state.v1.StateService.GetStateConfig
         */
        readonly getStateConfig: {
            readonly name: "GetStateConfig";
            readonly I: any;
            readonly O: any;
            readonly kind: any;
        };
        /**
         * GetStateLock inspects the current lock metadata for a state by GUID.
         *
         * @generated from rpc state.v1.StateService.GetStateLock
         */
        readonly getStateLock: {
            readonly name: "GetStateLock";
            readonly I: any;
            readonly O: any;
            readonly kind: any;
        };
        /**
         * UnlockState releases a lock using the lock ID provided by Terraform/OpenTofu.
         *
         * @generated from rpc state.v1.StateService.UnlockState
         */
        readonly unlockState: {
            readonly name: "UnlockState";
            readonly I: any;
            readonly O: any;
            readonly kind: any;
        };
    };
};
