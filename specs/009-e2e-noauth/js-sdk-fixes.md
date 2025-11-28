# Changes Needed to js/sdk/src/client.ts

## Current State

The SDK currently exposes only 6 RPC methods out of 30+ available in the generated protobuf code:

Currently Exposed:

1. `createState`
2. `listStates`
3. `getStateInfo`
4. `listAllEdges`
5. `getLabelPolicy`
6. `setLabelPolicy`

## Missing Methods Needed for E2E Tests

### Critical for E2E (Priority 1)

```typescript
// Dependency Management
addDependency(request: AddDependencyRequest): Promise<AddDependencyResponse>;
removeDependency(request: RemoveDependencyRequest): Promise<RemoveDependencyResponse>;
listDependencies(request: ListDependenciesRequest): Promise<ListDependenciesResponse>;
listDependents(request: ListDependentsRequest): Promise<ListDependentsResponse>;

// State Management  
updateStateLabels(request: UpdateStateLabelsRequest): Promise<UpdateStateLabelsResponse>;

// Status & Outputs
getStateStatus(request: GetStateStatusRequest): Promise<GetStateStatusResponse>;
listStateOutputs(request: ListStateOutputsRequest): Promise<ListStateOutputsResponse>;
```

### Useful for E2E (Priority 2)

```typescript
// Topology
getTopologicalOrder(request: GetTopologicalOrderRequest): Promise<GetTopologicalOrderResponse>;
getDependencyGraph(request: GetDependencyGraphRequest): Promise<GetDependencyGraphResponse>;
searchByOutput(request: SearchByOutputRequest): Promise<SearchByOutputResponse>;

// Configuration
getStateConfig(request: GetStateConfigRequest): Promise<GetStateConfigResponse>;
```

## Required Changes to js/sdk/src/client.ts

### Step 1: Import Missing Types

```typescript
// Add to existing imports from '../gen/state/v1/state_pb.js'
import type {
// ... existing imports ...

// Dependency Management
AddDependencyRequest,
AddDependencyResponse,
RemoveDependencyRequest,
RemoveDependencyResponse,
ListDependenciesRequest,
ListDependenciesResponse,
ListDependentsRequest,
ListDependentsResponse,

// State Management
UpdateStateLabelsRequest,
UpdateStateLabelsResponse,

// Status & Outputs
GetStateStatusRequest,
GetStateStatusResponse,
ListStateOutputsRequest,
ListStateOutputsResponse,

// Topology
GetTopologicalOrderRequest,
GetTopologicalOrderResponse,
GetDependencyGraphRequest,
GetDependencyGraphResponse,
SearchByOutputRequest,
SearchByOutputResponse,

// Configuration
GetStateConfigRequest,
GetStateConfigResponse,
} from '../gen/state/v1/state_pb.js';
```

### Step 2: Extend StateServiceClient Interface

```typescript
export interface StateServiceClient {
// --- Existing methods ---
createState(request: CreateStateRequest | Record<string, unknown>): Promise<CreateStateResponse>;
listStates(request?: ListStatesRequest | Record<string, unknown>): Promise<ListStatesResponse>;
getStateInfo(request: GetStateInfoRequest | Record<string, unknown>): Promise<GetStateInfoResponse>;
listAllEdges(request?: ListAllEdgesRequest | Record<string, unknown>): Promise<ListAllEdgesResponse>;
getLabelPolicy(request?: GetLabelPolicyRequest | Record<string, unknown>): Promise<GetLabelPolicyResponse>;
setLabelPolicy(request: SetLabelPolicyRequest | Record<string, unknown>): Promise<SetLabelPolicyResponse>;

// --- NEW: Dependency Management ---
addDependency(request: AddDependencyRequest | Record<string, unknown>): Promise<AddDependencyResponse>;
removeDependency(request: RemoveDependencyRequest | Record<string, unknown>): Promise<RemoveDependencyResponse>;
listDependencies(request: ListDependenciesRequest | Record<string, unknown>): Promise<ListDependenciesResponse>;
listDependents(request: ListDependentsRequest | Record<string, unknown>): Promise<ListDependentsResponse>;

// --- NEW: State Management ---
updateStateLabels(request: UpdateStateLabelsRequest | Record<string, unknown>): Promise<UpdateStateLabelsResponse>;

// --- NEW: Status & Outputs ---
getStateStatus(request: GetStateStatusRequest | Record<string, unknown>): Promise<GetStateStatusResponse>;
listStateOutputs(request: ListStateOutputsRequest | Record<string, unknown>): Promise<ListStateOutputsResponse>;

// --- NEW: Topology ---
getTopologicalOrder(request: GetTopologicalOrderRequest | Record<string, unknown>): Promise<GetTopologicalOrderResponse>;
getDependencyGraph(request: GetDependencyGraphRequest | Record<string, unknown>): Promise<GetDependencyGraphResponse>;
searchByOutput(request: SearchByOutputRequest | Record<string, unknown>): Promise<SearchByOutputResponse>;

// --- NEW: Configuration ---
getStateConfig(request: GetStateConfigRequest | Record<string, unknown>): Promise<GetStateConfigResponse>;
}
```

> Note: The createGridClient function at line 89 doesn't need changes—it uses type casting from the generated StateService which already includes all methods.

## Verification

After making these changes, the E2E tests can use the SDK like this:

```typescript
import { createGridTransport, createGridClient } from '@tcons/grid/sdk';

const transport = createGridTransport('http://localhost:8080');
const client = createGridClient(transport);

// ✅ Now available for E2E tests
const dep = await client.addDependency({
from: { logicId: 'producer' },
fromOutput: 'vpc_id',
to: { logicId: 'consumer' },
});

const labels = await client.updateStateLabels({
guid: 'xxx',
labels: { env: 'prod', team: 'platform' },
});

const status = await client.getStateStatus({
stateReference: { logicId: 'my-state' },
});
```

### Summary of Required Work

1. Add type imports (~20 new types from state_pb.js)
2. Extend interface (~11 new method signatures)
3. No runtime changes (implementation already exists in generated code)
4. TypeScript only (100% type safety, zero new runtime code)
