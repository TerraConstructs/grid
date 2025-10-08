# Bolt v2 prompt

Build the "Grid" Dashboard

A readonly (no AuthN/AuthZ) dashboard for a Terraform State management API with the following protobuf RPC functions and messages.

mock API calls with demo data (example states: prod/network (provides vpc_id, app_subnets, ingress_nlb_sg, ingress_nlb_id), prod/cluster (consumes network > vpc_id, domain_zone_id, app_subnets, data_subnets, ingress_nlb_sg (for whitelisting ingress on nodes, exposes cluster_node_sg), prod/db (consumes network data_subnets, exposes db_sg_id), prod/app1_deploy (consumes prod/db db_sg_id, prod/clusster: cluster_node_sg, network domain_zone_id and ingress_nlb_id for DNS Zone App Aliasing on ingress host)

Capabilities
- [ ] Graph view of states and dependencies and state of edges. render topo layers; color dirty edges; click to drill.
- [ ] List view of states, dependency edges and their states
- [ ] Detail view of state (json view of tfstate, list of dependencies, list of dependents, list of outputs)

Consider angled connectors for the graph and grid-like design

The dashboard features a purple accent color (`#8B5CF6`) with clean light/dark themes, interactive dependency graph visualization with topological layers, colored edges indicating state (green for clean, orange for dirty, blue for pending), list view of all states and edges, and detailed state view with JSON inspection, outputs management, and dependency tracking. The mock API simulates the protobuf RPC functions with realistic production infrastructure data including network, cluster, database, and application deployment states.

<protobuf>
// StateService provides remote state management for Terraform/OpenTofu clients.
service StateService {
  // CreateState creates a new state with client-generated GUID and logic ID.
  rpc CreateState(CreateStateRequest) returns (CreateStateResponse);

  // ListStates returns all states with summary information.
  rpc ListStates(ListStatesRequest) returns (ListStatesResponse);

  // GetStateConfig retrieves backend configuration for an existing state by logic ID.
  rpc GetStateConfig(GetStateConfigRequest) returns (GetStateConfigResponse);

  // GetStateLock inspects the current lock metadata for a state by GUID.
  rpc GetStateLock(GetStateLockRequest) returns (GetStateLockResponse);

  // UnlockState releases a lock using the lock ID provided by Terraform/OpenTofu.
  rpc UnlockState(UnlockStateRequest) returns (UnlockStateResponse);

  // --- Dependency Management RPCs ---

  // AddDependency declares a dependency edge from producer output to consumer state.
  // Returns existing edge if duplicate (idempotent). Rejects if would create cycle.
  rpc AddDependency(AddDependencyRequest) returns (AddDependencyResponse);

  // RemoveDependency deletes a dependency edge by ID.
  rpc RemoveDependency(RemoveDependencyRequest) returns (RemoveDependencyResponse);

  // ListDependencies returns all edges where the given state is the consumer (incoming deps).
  rpc ListDependencies(ListDependenciesRequest) returns (ListDependenciesResponse);

  // ListDependents returns all edges where the given state is the producer (outgoing deps).
  rpc ListDependents(ListDependentsRequest) returns (ListDependentsResponse);

  // SearchByOutput finds all edges that reference a specific output key (by name).
  rpc SearchByOutput(SearchByOutputRequest) returns (SearchByOutputResponse);

  // GetTopologicalOrder computes layered ordering of states rooted at given state.
  rpc GetTopologicalOrder(GetTopologicalOrderRequest) returns (GetTopologicalOrderResponse);

  // GetStateStatus computes on-demand status for a state based on its incoming edges.
  rpc GetStateStatus(GetStateStatusRequest) returns (GetStateStatusResponse);

  // GetDependencyGraph returns full dependency graph data for client-side HCL generation.
  rpc GetDependencyGraph(GetDependencyGraphRequest) returns (GetDependencyGraphResponse);

  // ListStateOutputs returns available output keys from a state's Terraform/OpenTofu JSON.
  // Output values are NOT returned (security/size concerns); only keys and sensitive flags.
  // Used by CLI for interactive output selection when creating dependencies.
  rpc ListStateOutputs(ListStateOutputsRequest) returns (ListStateOutputsResponse);

  // GetStateInfo retrieves comprehensive state information including:
  // - Basic metadata (GUID, logic-id, timestamps)
  // - Backend configuration (Terraform HTTP backend URLs)
  // - Dependencies (incoming edges: states this state depends on)
  // - Dependents (outgoing edges: states that depend on this state)
  // - Outputs (available Terraform output keys from state JSON)
  //
  // This consolidates information previously requiring multiple RPC calls.
  rpc GetStateInfo(GetStateInfoRequest) returns (GetStateInfoResponse);
}

// CreateStateRequest creates a new state using a client-generated GUID.
message CreateStateRequest {
  string guid = 1;
  string logic_id = 2;
}

// CreateStateResponse confirms creation and returns backend config.
message CreateStateResponse {
  string guid = 1;
  string logic_id = 2;
  BackendConfig backend_config = 3;
}

// ListStatesRequest requests all states.
message ListStatesRequest {}

// ListStatesResponse returns all states with basic info.
message ListStatesResponse {
  repeated StateInfo states = 1;
}

// StateInfo is summary information for a state.
message StateInfo {
  string guid = 1;
  string logic_id = 2;
  bool locked = 3;
  google.protobuf.Timestamp created_at = 4;
  google.protobuf.Timestamp updated_at = 5;
  int64 size_bytes = 6;
  // Derived status fields for quick indicators
  optional string computed_status = 7; // "clean", "stale", "potentially-stale"
  repeated string dependency_logic_ids = 8; // Unique set of producer logic_ids (incoming edges)
}

// BackendConfig contains Terraform backend configuration URLs.
message BackendConfig {
  string address = 1;
  string lock_address = 2;
  string unlock_address = 3;
}

// GetStateConfigRequest retrieves backend config for existing state.
message GetStateConfigRequest {
  string logic_id = 1;
}

// GetStateConfigResponse returns backend config.
message GetStateConfigResponse {
  string guid = 1;
  BackendConfig backend_config = 2;
}

// GetStateLockRequest fetches current lock metadata by GUID.
message GetStateLockRequest {
  string guid = 1;
}

// LockInfo mirrors Terraform's lock payload.
message LockInfo {
  string id = 1;
  string operation = 2;
  string info = 3;
  string who = 4;
  string version = 5;
  google.protobuf.Timestamp created = 6;
  string path = 7;
}

// StateLock response wrapper indicating lock state plus metadata when present.
message StateLock {
  bool locked = 1;
  LockInfo info = 2;
}

// GetStateLockResponse returns current lock status.
message GetStateLockResponse {
  StateLock lock = 1;
}

// UnlockStateRequest releases a lock given the current lock ID.
message UnlockStateRequest {
  string guid = 1;
  string lock_id = 2;
}

// UnlockStateResponse mirrors GetStateLockResponse after unlock attempt.
message UnlockStateResponse {
  StateLock lock = 1;
}

// --- Dependency Management Messages ---

// AddDependencyRequest creates a new dependency edge.
message AddDependencyRequest {
  // Producer state reference (logic_id or GUID, prefer logic_id for UX)
  oneof from_state {
    string from_logic_id = 1;
    string from_guid = 2;
  }

  // Output key from producer state
  string from_output = 3;

  // Consumer state reference (logic_id or GUID)
  oneof to_state {
    string to_logic_id = 4;
    string to_guid = 5;
  }

  // Optional override for generated HCL local variable name
  optional string to_input_name = 6;

  // Optional mock value for ahead-of-time dependency declaration
  // (JSON-encoded value, used when producer output doesn't exist yet)
  optional string mock_value_json = 7;
}

// AddDependencyResponse returns the created or existing edge.
message AddDependencyResponse {
  DependencyEdge edge = 1;
  bool already_exists = 2; // True if edge already existed (idempotent)
}

// RemoveDependencyRequest deletes an edge by ID.
message RemoveDependencyRequest {
  int64 edge_id = 1;
}

// RemoveDependencyResponse confirms deletion.
message RemoveDependencyResponse {
  bool success = 1;
}

// ListDependenciesRequest fetches incoming edges for a consumer state.
message ListDependenciesRequest {
  // Consumer state reference (logic_id or GUID)
  oneof state {
    string logic_id = 1;
    string guid = 2;
  }
}

// ListDependenciesResponse returns all incoming edges.
message ListDependenciesResponse {
  repeated DependencyEdge edges = 1;
}

// ListDependentsRequest fetches outgoing edges for a producer state.
message ListDependentsRequest {
  // Producer state reference (logic_id or GUID)
  oneof state {
    string logic_id = 1;
    string guid = 2;
  }
}

// ListDependentsResponse returns all outgoing edges.
message ListDependentsResponse {
  repeated DependencyEdge edges = 1;
}

// SearchByOutputRequest finds edges by output key name.
message SearchByOutputRequest {
  string output_key = 1;
}

// SearchByOutputResponse returns matching edges.
message SearchByOutputResponse {
  repeated DependencyEdge edges = 1;
}

// GetTopologicalOrderRequest computes layered ordering rooted at a state.
message GetTopologicalOrderRequest {
  // Root state reference (logic_id or GUID)
  oneof state {
    string logic_id = 1;
    string guid = 2;
  }

  // Direction: "downstream" (default) or "upstream"
  optional string direction = 3;
}

// GetTopologicalOrderResponse returns layered state ordering.
message GetTopologicalOrderResponse {
  repeated Layer layers = 1;
}

// Layer represents a level in the topological ordering.
message Layer {
  int32 level = 1;
  repeated StateRef states = 2;
}

// StateRef is a minimal state reference.
message StateRef {
  string guid = 1;
  string logic_id = 2;
}

// GetStateStatusRequest computes on-demand status for a state.
message GetStateStatusRequest {
  // State reference (logic_id or GUID)
  oneof state {
    string logic_id = 1;
    string guid = 2;
  }
}

// GetStateStatusResponse returns computed status with incoming edges.
message GetStateStatusResponse {
  string guid = 1;
  string logic_id = 2;
  string status = 3; // "clean", "stale", "potentially-stale"
  repeated IncomingEdgeView incoming = 4;
  StatusSummary summary = 5;
}

// IncomingEdgeView shows incoming edge details for status computation.
message IncomingEdgeView {
  int64 edge_id = 1;
  string from_guid = 2;
  string from_logic_id = 3;
  string from_output = 4;
  string status = 5; // Edge status: "clean", "dirty", "pending", etc.
  optional string in_digest = 6;
  optional string out_digest = 7;
  optional google.protobuf.Timestamp last_in_at = 8;
  optional google.protobuf.Timestamp last_out_at = 9;
}

// StatusSummary aggregates incoming edge counts.
message StatusSummary {
  int32 incoming_clean = 1;
  int32 incoming_dirty = 2;
  int32 incoming_pending = 3;
  int32 incoming_unknown = 4;
}

// GetDependencyGraphRequest fetches graph data for consumer state HCL generation.
message GetDependencyGraphRequest {
  // Consumer state reference (logic_id or GUID)
  oneof state {
    string logic_id = 1;
    string guid = 2;
  }
}

// GetDependencyGraphResponse returns data needed for grid_dependencies.tf generation.
message GetDependencyGraphResponse {
  string consumer_guid = 1;
  string consumer_logic_id = 2;
  repeated ProducerState producers = 3;
  repeated DependencyEdge edges = 4;
}

// ProducerState represents a unique producer state in the graph.
message ProducerState {
  string guid = 1;
  string logic_id = 2;
  BackendConfig backend_config = 3;
}

// DependencyEdge represents a directed dependency edge.
message DependencyEdge {
  int64 id = 1;
  string from_guid = 2;
  string from_logic_id = 3;
  string from_output = 4;
  string to_guid = 5;
  string to_logic_id = 6;
  optional string to_input_name = 7;
  string status = 8; // "pending", "clean", "dirty", "potentially-stale", "mock", "missing-output"
  optional string in_digest = 9;
  optional string out_digest = 10;
  optional string mock_value_json = 11;
  optional google.protobuf.Timestamp last_in_at = 12;
  optional google.protobuf.Timestamp last_out_at = 13;
  google.protobuf.Timestamp created_at = 14;
  google.protobuf.Timestamp updated_at = 15;
}

// --- Outputs Caching Messages ---

// OutputKey represents a single Terraform/OpenTofu output name and metadata.
message OutputKey {
  // Output name from Terraform state JSON (e.g., "vpc_id", "db_password")
  string key = 1;

  // Whether output is marked sensitive in Terraform state metadata
  // Used by CLI to display warning: "⚠️  sensitive" next to output name
  bool sensitive = 2;
}

// ListStateOutputsRequest fetches output keys for a state.
message ListStateOutputsRequest {
  // State identifier (prefer logic_id for UX, guid for precision)
  oneof state {
    string logic_id = 1;  // User-friendly state identifier
    string guid = 2;      // Immutable UUIDv7 identifier
  }
}

// ListStateOutputsResponse returns output keys parsed from Terraform state JSON.
message ListStateOutputsResponse {
  // State identifiers for confirmation
  string state_guid = 1;
  string state_logic_id = 2;

  // List of output keys available in this state's Terraform JSON
  // Empty array if state has no outputs (not an error)
  repeated OutputKey outputs = 3;
}

// GetStateInfoRequest fetches full state information.
message GetStateInfoRequest {
  // State identifier (prefer logic_id for UX, guid for precision)
  oneof state {
    string logic_id = 1;  // User-friendly state identifier
    string guid = 2;      // Immutable UUIDv7 identifier
  }
}

// GetStateInfoResponse returns comprehensive state view.
message GetStateInfoResponse {
  // State identifiers
  string guid = 1;
  string logic_id = 2;

  // Terraform HTTP backend configuration
  BackendConfig backend_config = 3;

  // Incoming dependency edges (this state consumes outputs from these states)
  // Equivalent to: SELECT * FROM edges WHERE to_guid = this.guid
  repeated DependencyEdge dependencies = 4;

  // Outgoing dependency edges (other states consume this state's outputs)
  // Equivalent to: SELECT * FROM edges WHERE from_guid = this.guid
  repeated DependencyEdge dependents = 5;

  // Available outputs from this state's Terraform JSON (keys only, no values)
  // Empty array if state has no Terraform state JSON uploaded yet
  repeated OutputKey outputs = 6;

  // State lifecycle timestamps
  google.protobuf.Timestamp created_at = 7;
  google.protobuf.Timestamp updated_at = 8;
}
</protobuf>
