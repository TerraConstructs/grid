/**
 * StateSummary represents lightweight state metadata for list operations.
 * Optimized for rendering state lists without N+1 queries.
 * Use getStateInfo() to fetch full StateInfo with relationships when needed.
 */
export interface StateSummary {
  /** Immutable client-generated UUIDv7 identifier */
  guid: string;

  /** User-friendly mutable identifier */
  logic_id: string;

  /** Whether the state is currently locked */
  locked?: boolean;

  /** State creation timestamp (ISO 8601) */
  created_at: string;

  /** State last update timestamp (ISO 8601) */
  updated_at: string;

  /** State JSON size in bytes */
  size_bytes?: number;

  /** Aggregate status computed from dependency edges */
  computed_status?: 'clean' | 'stale' | 'potentially-stale';

  /** Logic IDs of states this state depends on */
  dependency_logic_ids: string[];

  /** Label metadata (typed values) */
  labels?: Record<string, LabelScalar>;

  /** Number of incoming dependency edges (efficient count from backend) */
  dependencies_count: number;

  /** Number of outgoing dependency edges (efficient count from backend) */
  dependents_count: number;

  /** Number of outputs available (efficient count from backend) */
  outputs_count: number;
}

/**
 * StateInfo represents comprehensive metadata for a Terraform remote state
 * including dependencies, outputs, and backend configuration.
 */
export interface StateInfo {
  /** Immutable client-generated UUIDv7 identifier */
  guid: string;

  /** User-friendly mutable identifier */
  logic_id: string;

  /** Whether the state is currently locked (future feature) */
  locked?: boolean;

  /** State creation timestamp (ISO 8601) */
  created_at: string;

  /** State last update timestamp (ISO 8601) */
  updated_at: string;

  /** State JSON size in bytes (future feature) */
  size_bytes?: number;

  /** Aggregate status computed from dependency edges */
  computed_status?: 'clean' | 'stale' | 'potentially-stale';

  /** Logic IDs of states this state depends on */
  dependency_logic_ids: string[];

  /** Terraform HTTP backend configuration */
  backend_config: BackendConfig;

  /** Incoming dependency edges (this state consumes outputs) */
  dependencies: DependencyEdge[];

  /** Outgoing dependency edges (this state produces outputs) */
  dependents: DependencyEdge[];

  /** Available Terraform outputs from this state */
  outputs: OutputKey[];

  /** Label metadata (typed values) */
  labels?: Record<string, LabelScalar>;
}

/** Permitted label value types */
export type LabelScalar = string | number | boolean;

/** Terraform HTTP backend configuration URLs */
export interface BackendConfig {
  /** Main state endpoint */
  address: string;

  /** Lock endpoint */
  lock_address: string;

  /** Unlock endpoint */
  unlock_address: string;
}

/** Dependency edge between producer and consumer states */
export interface DependencyEdge {
  /** Unique edge identifier */
  id: number;

  /** Producer state GUID */
  from_guid: string;

  /** Producer state logic ID */
  from_logic_id: string;

  /** Producer output key name */
  from_output: string;

  /** Consumer state GUID */
  to_guid: string;

  /** Consumer state logic ID */
  to_logic_id: string;

  /** HCL variable name override (optional) */
  to_input_name?: string;

  /** Synchronization status */
  status: EdgeStatus;

  /** Consumer's last observed value hash (SHA256) */
  in_digest?: string;

  /** Producer's current value hash (SHA256) */
  out_digest?: string;

  /** Placeholder value for missing outputs (JSON string) */
  mock_value_json?: string;

  /** Last time consumer updated (ISO 8601) */
  last_in_at?: string;

  /** Last time producer updated (ISO 8601) */
  last_out_at?: string;

  /** Edge creation timestamp (ISO 8601) */
  created_at: string;

  /** Edge last modification timestamp (ISO 8601) */
  updated_at: string;
}

/**
 * Edge synchronization status using a composite model.
 * Combines two orthogonal dimensions:
 * 1. Drift: clean (in_digest === out_digest) vs dirty (in_digest !== out_digest)
 * 2. Validation: valid (passes schema) vs invalid (fails schema)
 */
export type EdgeStatus =
  | 'pending'           // Edge created, no digest values yet
  | 'clean'             // in_digest === out_digest AND valid (synchronized AND valid)
  | 'clean-invalid'     // in_digest === out_digest AND invalid (synchronized but fails schema)
  | 'dirty'             // in_digest !== out_digest AND valid (out of sync but valid)
  | 'dirty-invalid'     // in_digest !== out_digest AND invalid (out of sync AND fails schema)
  | 'potentially-stale' // Producer updated, consumer not re-evaluated
  | 'mock'              // Using mock_value_json
  | 'missing-output';   // Producer doesn't have required output

/** Metadata about a Terraform output */
export interface OutputKey {
  /** Output name from Terraform state */
  key: string;

  /** Whether output is marked sensitive in Terraform */
  sensitive: boolean;

  /** JSON Schema for output validation (optional) */
  schema_json?: string;

  /** How the schema was created: 'manual' (user-provided) or 'inferred' (auto-generated) */
  schema_source?: 'manual' | 'inferred';

  /** Validation status: 'valid', 'invalid', 'error', or 'not_validated' */
  validation_status?: 'valid' | 'invalid' | 'error' | 'not_validated';

  /** Validation error message (only present if validation_status is 'invalid' or 'error') */
  validation_error?: string;

  /** Timestamp when validation last ran (ISO 8601) */
  validated_at?: string;
}
