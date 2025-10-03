# Quickstart: State Dependency Management

**Feature**: State Dependency Management
**Date**: 2025-10-02
**Purpose**: Validate acceptance scenarios via automated integration tests

## Prerequisites

- Grid API server running (`./bin/gridapi serve`)
- Grid CLI built (`./bin/gridctl`)
- PostgreSQL database initialized with migrations
- Two test states created: `landing-zone` and `cluster`

## Scenario 1: Basic Dependency Declaration (FR-001, FR-002, AS-1)

**Given**: Two states exist (landing-zone produces `vpc_id`, cluster consumes it)

**Steps**:
```bash
# 1. Create producer state
./bin/gridctl state create --logic-id landing-zone
# Output: Created state landing-zone (GUID: <uuid>)

# 2. Create consumer state
./bin/gridctl state create --logic-id cluster
# Output: Created state cluster (GUID: <uuid>)

# 3. Declare dependency
./bin/gridctl deps add \
  --from landing-zone \
  --output vpc_id \
  --to cluster
# Output: Dependency added: landing-zone.vpc_id -> cluster (edge ID: 123)

# 4. Verify dependency exists
./bin/gridctl deps list --state cluster
# Output:
# EDGE_ID  FROM_STATE      FROM_OUTPUT  STATUS   LAST_UPDATED
# 123      landing-zone    vpc_id       pending  2025-10-02T10:00:00Z
```

**Expected**:
- Dependency edge created successfully
- Edge status initially `pending` (no observation yet)
- Edge appears in `deps list` output for consumer state

---

## Scenario 2: Cycle Prevention (FR-003, FR-004, FR-045, AS-2)

**Given**: States A→B and B→C exist

**Steps**:
```bash
# 1. Create states
./bin/gridctl state create --logic-id state-a
./bin/gridctl state create --logic-id state-b
./bin/gridctl state create --logic-id state-c

# 2. Create A→B dependency
./bin/gridctl deps add --from state-a --output out1 --to state-b
# Output: Dependency added: state-a.out1 -> state-b (edge ID: 1)

# 3. Create B→C dependency
./bin/gridctl deps add --from state-b --output out2 --to state-c
# Output: Dependency added: state-b.out2 -> state-c (edge ID: 2)

# 4. Attempt to create C→A (would create cycle)
./bin/gridctl deps add --from state-c --output out3 --to state-a
# Output (ERROR): Cycle detected: state-c.out3 -> state-a would create cycle [state-a -> state-b -> state-c -> state-a]
# Exit code: 1
```

**Expected**:
- First two edges created successfully
- Third edge rejected with cycle error
- Exit code non-zero for failed add

---

## Scenario 3: Edge Status Tracking (FR-017 to FR-027, AS-3)

**Given**: Dependency from landing-zone.subnet_id to cluster

**Steps**:
```bash
# 1. Declare dependency (before output exists)
./bin/gridctl deps add --from landing-zone --output subnet_id --to cluster
# Edge status: pending

# 2. Apply landing-zone state (creates subnet_id output)
cd /tmp/landing-zone-state
terraform init  # Uses Grid backend
terraform apply -auto-approve
# Terraform writes state to Grid API via PUT /tfstate/{guid}
# Grid UpdateEdges job runs: detects new output, computes fingerprint, updates edge

# 3. Check edge status
./bin/gridctl deps list --state cluster
# Output:
# EDGE_ID  FROM_STATE      FROM_OUTPUT  STATUS  IN_DIGEST        OUT_DIGEST  LAST_IN_AT
# 456      landing-zone    subnet_id    dirty   <sha256-base58>  -           2025-10-02T10:05:00Z

# 4. Run deps sync to update cluster locals
cd /tmp/cluster-state
./bin/gridctl deps sync --state cluster
# Output: Generated grid_dependencies.tf with 1 dependency

# 5. Apply cluster state (observes landing-zone.subnet_id)
terraform init
terraform apply -auto-approve
# Terraform writes cluster state to Grid API
# Grid UpdateEdges job: updates edge.out_digest to match in_digest, status → clean

# 6. Verify edge status is clean
./bin/gridctl deps list --state cluster
# Output:
# EDGE_ID  FROM_STATE      FROM_OUTPUT  STATUS  IN_DIGEST        OUT_DIGEST       LAST_OUT_AT
# 456      landing-zone    subnet_id    clean   <sha256-base58>  <sha256-base58>  2025-10-02T10:10:00Z
```

**Expected**:
- Edge transitions: pending → dirty (when output appears) → clean (when consumer observes)
- `in_digest` and `out_digest` match when status is clean
- Timestamps update on each transition

---

## Scenario 4: Mock Dependencies (FR-006, FR-014, FR-015, AS-4, AS-9)

**Given**: User wants to wire states before outputs exist

**Steps**:
```bash
# 1. Declare dependency with mock value
./bin/gridctl deps add \
  --from landing-zone \
  --output vpc_id \
  --to cluster \
  --mock '{"value": "vpc-mock-12345"}'
# Output: Dependency added with mock value: landing-zone.vpc_id -> cluster (edge ID: 789)

# 2. Verify edge status is "mock"
./bin/gridctl deps list --state cluster
# Output:
# EDGE_ID  FROM_STATE      FROM_OUTPUT  STATUS  MOCK_VALUE
# 789      landing-zone    vpc_id       mock    {"value":"vpc-mock-12345"}

# 3. Generate HCL locals (includes mock)
./bin/gridctl deps sync --state cluster
# Output: Generated grid_dependencies.tf with 1 dependency (using mock value)

# 4. Apply landing-zone (real vpc_id output appears)
cd /tmp/landing-zone-state
terraform apply -auto-approve
# Grid UpdateEdges job: detects real output, replaces mock, edge.status → dirty

# 5. Check edge transitioned to dirty
./bin/gridctl deps list --state cluster
# Output:
# EDGE_ID  FROM_STATE      FROM_OUTPUT  STATUS  IN_DIGEST        OUT_DIGEST  MOCK_VALUE
# 789      landing-zone    vpc_id       dirty   <real-digest>    -           null

# 6. Re-apply cluster to observe real value
./bin/gridctl deps sync --state cluster  # Regenerate HCL with real value
cd /tmp/cluster-state
terraform apply -auto-approve
# Edge transitions to clean
```

**Expected**:
- Mock edge created successfully
- HCL generation includes mock value
- Edge transitions to dirty when real output appears
- Mock value cleared when real output observed

---

## Scenario 5: Topological Ordering (FR-032, FR-033, AS-5)

**Given**: Complex graph with multiple layers

**Steps**:
```bash
# 1. Create dependency chain: foundation → network → compute → app
./bin/gridctl state create --logic-id foundation
./bin/gridctl state create --logic-id network
./bin/gridctl state create --logic-id compute
./bin/gridctl state create --logic-id app

./bin/gridctl deps add --from foundation --output region --to network
./bin/gridctl deps add --from network --output vpc_id --to compute
./bin/gridctl deps add --from network --output subnet_ids --to compute
./bin/gridctl deps add --from compute --output cluster_endpoint --to app

# 2. Request topological ordering from app perspective
./bin/gridctl deps topo --state app --direction upstream
# Output:
# Layer 0: foundation
# Layer 1: network
# Layer 2: compute
# Layer 3: app
#
# Recommended reconciliation order: foundation → network → compute → app

# 3. Request downstream ordering from foundation
./bin/gridctl deps topo --state foundation --direction downstream
# Output:
# Layer 0: foundation
# Layer 1: network
# Layer 2: compute
# Layer 3: app
#
# Recommended reconciliation order: app → compute → network → foundation (reverse)
```

**Expected**:
- Layered view shows correct dependency levels
- Upstream ordering: dependencies before consumers
- Downstream ordering: consumers before producers (for propagation analysis)

---

## Scenario 6: Managed Locals Block Generation (FR-037 to FR-044, AS-6)

**Given**: Cluster state has 3 dependencies

**Steps**:
```bash
# 1. Declare multiple dependencies
./bin/gridctl deps add --from landing-zone --output vpc_id --to cluster
./bin/gridctl deps add --from landing-zone --output subnet_ids --to cluster
./bin/gridctl deps add --from iam-setup --output cluster_role_arn --to cluster

# 2. Generate managed locals block
cd /tmp/cluster-state
./bin/gridctl deps sync --state cluster
# Output: Generated grid_dependencies.tf

# 3. Inspect generated file
cat grid_dependencies.tf
# Expected output:
# # BEGIN GRID MANAGED BLOCK - DO NOT EDIT
# # Generated by Grid at 2025-10-02T10:15:00Z
# # Dependencies for state: cluster
#
# data "terraform_remote_state" "landing_zone" {
#   backend = "http"
#   config = {
#     address        = "http://localhost:8080/tfstate/<landing-zone-guid>"
#     lock_address   = "http://localhost:8080/tfstate/<landing-zone-guid>/lock"
#     unlock_address = "http://localhost:8080/tfstate/<landing-zone-guid>/unlock"
#   }
# }
#
# data "terraform_remote_state" "iam_setup" {
#   backend = "http"
#   config = {
#     address        = "http://localhost:8080/tfstate/<iam-setup-guid>"
#     lock_address   = "http://localhost:8080/tfstate/<iam-setup-guid>/lock"
#     unlock_address = "http://localhost:8080/tfstate/<iam-setup-guid>/unlock"
#   }
# }
#
# locals {
#   landing_zone_vpc_id        = data.terraform_remote_state.landing_zone.outputs.vpc_id
#   landing_zone_subnet_ids    = data.terraform_remote_state.landing_zone.outputs.subnet_ids
#   iam_setup_cluster_role_arn = data.terraform_remote_state.iam_setup.outputs.cluster_role_arn
# }
#
# # END GRID MANAGED BLOCK

# 4. Use locals in Terraform config
cat main.tf
# resource "aws_eks_cluster" "main" {
#   vpc_config {
#     subnet_ids = local.landing_zone_subnet_ids
#   }
#   role_arn = local.iam_setup_cluster_role_arn
# }

# 5. Run terraform init and validate
terraform init
terraform validate
# Output: Success! The configuration is valid.

# 6. Re-run sync (idempotent overwrite)
./bin/gridctl deps sync --state cluster
# Output: Generated grid_dependencies.tf (overwrites previous version)
```

**Expected**:
- `grid_dependencies.tf` created with managed block markers
- One `terraform_remote_state` data source per unique producer
- Locals named `<state_slug>_<output_slug>` (e.g., `landing_zone_vpc_id`)
- Idempotent: re-running sync overwrites file deterministically

---

## Scenario 7: Dependency Listing and Status (FR-028, FR-029, FR-034, FR-035, AS-7)

**Given**: States with multiple dependencies

**Steps**:
```bash
# 1. List incoming dependencies (what cluster consumes)
./bin/gridctl deps list --state cluster
# Output:
# EDGE_ID  FROM_STATE      FROM_OUTPUT        STATUS  LAST_UPDATED
# 123      landing-zone    vpc_id             clean   2025-10-02T10:00:00Z
# 124      landing-zone    subnet_ids         clean   2025-10-02T10:01:00Z
# 125      iam-setup       cluster_role_arn   dirty   2025-10-02T10:02:00Z

# 2. List outgoing dependents (who consumes landing-zone)
./bin/gridctl deps list --from landing-zone
# Output:
# EDGE_ID  TO_STATE   TO_INPUT        FROM_OUTPUT  STATUS  LAST_UPDATED
# 123      cluster    -               vpc_id       clean   2025-10-02T10:00:00Z
# 124      cluster    -               subnet_ids   clean   2025-10-02T10:01:00Z
# 126      app        network_config  vpc_id       dirty   2025-10-02T10:03:00Z

# 3. Check state-level status
./bin/gridctl state list
# Output:
# LOGIC_ID        GUID           STATUS            DEPENDENCIES
# landing-zone    <uuid>         clean             -
# iam-setup       <uuid>         clean             -
# cluster         <uuid>         stale             landing-zone, iam-setup
# app             <uuid>         potentially-stale landing-zone, cluster

# 4. Get detailed status for cluster
./bin/gridctl deps status --state cluster
# Output:
# State: cluster
# Status: stale (1 dirty incoming edge)
#
# Incoming Dependencies:
#   [clean]   landing-zone.vpc_id (edge 123)
#   [clean]   landing-zone.subnet_ids (edge 124)
#   [dirty]   iam-setup.cluster_role_arn (edge 125)
#
# Summary: 2 clean, 1 dirty, 0 pending, 0 unknown
# Recommendation: Reconcile upstream change from iam-setup, then re-apply cluster
```

**Expected**:
- Incoming deps show what state consumes
- Outgoing deps show who consumes state's outputs
- State list shows quick status indicators (clean/stale/potentially-stale)
- Detailed status shows per-edge breakdown

---

## Scenario 8: Search by Output Key (FR-030, FR-031, AS-8)

**Given**: Multiple states produce/consume `vpc_id` output

**Steps**:
```bash
# 1. Search for all edges consuming vpc_id
./bin/gridctl deps search --output vpc_id
# Output:
# EDGE_ID  FROM_STATE      TO_STATE    STATUS  LAST_UPDATED
# 123      landing-zone    cluster     clean   2025-10-02T10:00:00Z
# 126      landing-zone    app         dirty   2025-10-02T10:03:00Z
# 201      network-dev     test-env    clean   2025-10-02T09:00:00Z

# 2. Search with --show-producers to find who provides vpc_id
./bin/gridctl deps search --output vpc_id --show-producers
# Output:
# Producer States:
#   landing-zone (2 consumers)
#   network-dev (1 consumer)
```

**Expected**:
- All edges referencing output key returned
- Ability to find producers and consumers of specific outputs

---

## Scenario 9: to_input_name Default and Override (FR-008 to FR-011, AS-10, AS-11)

**Given**: User can optionally override generated local variable names

**Steps**:
```bash
# 1. Declare dependency WITHOUT to_input_name (service generates default)
./bin/gridctl deps add \
  --from landing-zone \
  --output vpc_id \
  --to cluster
# Output: Dependency added: landing-zone.vpc_id -> cluster as 'landing_zone_vpc_id' (edge ID: 300)
# Note: Default to_input_name generated: slugify("landing-zone") + "_" + slugify("vpc_id") = "landing_zone_vpc_id"

# 2. Declare dependency WITH to_input_name override
./bin/gridctl deps add \
  --from landing-zone \
  --output subnet_ids \
  --to cluster \
  --to-input network_subnets
# Output: Dependency added: landing-zone.subnet_ids -> cluster as 'network_subnets' (edge ID: 301)

# 3. Generate HCL locals (both default and override)
./bin/gridctl deps sync --state cluster
cat grid_dependencies.tf
# Expected locals block:
# locals {
#   landing_zone_vpc_id = data.terraform_remote_state.landing_zone.outputs.vpc_id       # Default
#   network_subnets     = data.terraform_remote_state.landing_zone.outputs.subnet_ids   # Override
# }

# 4. Attempt to create conflicting to_input_name
./bin/gridctl deps add \
  --from iam-setup \
  --output vpc_config \
  --to cluster \
  --to-input network_subnets
# Output (ERROR): Conflict: to_input_name 'network_subnets' already used by edge 301 for state cluster
# Exit code: 1

# 5. Verify uniqueness constraint
./bin/gridctl deps list --state cluster
# Output:
# EDGE_ID  FROM_STATE      FROM_OUTPUT  TO_INPUT_NAME         STATUS
# 300      landing-zone    vpc_id       landing_zone_vpc_id   clean    (default)
# 301      landing-zone    subnet_ids   network_subnets       clean    (override)
```

**Expected**:
- `to_input_name` is **optional in CLI** (service generates default if not provided)
- Default naming: `slugify(from_logic_id) + "_" + slugify(from_output)`
- Custom `to_input_name` used when explicitly provided
- Uniqueness constraint enforced per consumer state (always non-null in database)
- Conflict error when duplicate `to_input_name` attempted

---

## Scenario 10: Dependency Removal (FR-005, Edge Case)

**Given**: Active dependency exists

**Steps**:
```bash
# 1. Remove dependency
./bin/gridctl deps remove --edge-id 123
# Output: Dependency removed (edge ID: 123)

# 2. Verify removal
./bin/gridctl deps list --state cluster
# Output: (edge 123 no longer appears)

# 3. Regenerate HCL (should exclude removed edge)
./bin/gridctl deps sync --state cluster
cat grid_dependencies.tf
# Expected: grid_dependencies.tf no longer includes landing-zone.vpc_id

# 4. Check state status recomputed
./bin/gridctl deps status --state cluster
# Output: Status recomputed without removed edge
```

**Expected**:
- Edge deleted successfully
- HCL regeneration excludes removed edge
- State status recomputed on demand

---

## Integration Test Execution

**Location**: `tests/integration/dependency_test.go`

**Pattern**:
```go
func TestDependencyWorkflow(t *testing.T) {
  // Setup: Start gridapi server, create test database
  // Run each scenario as subtest
  t.Run("BasicDependencyDeclaration", func(t *testing.T) { /* Scenario 1 */ })
  t.Run("CyclePrevention", func(t *testing.T) { /* Scenario 2 */ })
  t.Run("EdgeStatusTracking", func(t *testing.T) { /* Scenario 3 */ })
  // ... etc
}
```

**Validation Criteria**:
- All scenarios pass automated tests
- No manual intervention required (fully automated)
- Tests run against real PostgreSQL (not mocks)
- Tests use real gridapi server (TestMain pattern)

---

## Success Criteria

- ✅ All 10 scenarios execute successfully via `go test`
- ✅ Cycle prevention works at both DB and application layer
- ✅ Edge statuses transition correctly (pending → dirty → clean)
- ✅ Mock dependencies supported and transition to real outputs
- ✅ Managed HCL blocks generated idempotently
- ✅ Topological ordering computed correctly
- ✅ State status derived accurately from edges
- ✅ to_input_name overrides work with uniqueness enforcement

**Next**: Use this quickstart as basis for integration test implementation in Phase 4.
