# Quickstart: Output Schema Support - Phase 2

**Feature Branch**: `010-output-schema-support`
**Date**: 2025-11-25
**Prerequisites**: Phase 1 complete (schema declaration & storage working)

This guide covers Phase 2 functionality: Schema Inference (2A) and Schema Validation (2B).

---

## Phase 2A: Schema Inference

### Scenario 1: Automatic Schema Inference on First Upload

When Terraform uploads state to a new state (or an output without a schema), the system automatically infers a JSON Schema.

```bash
# 1. Create a new state (no schemas yet)
./bin/gridctl state create vpc-prod --label env=prod

# 2. Initialize Terraform with the state backend
cd terraform/vpc
./bin/gridctl state init vpc-prod

# 3. Apply Terraform (uploads state with outputs)
terraform apply -auto-approve

# Output in state:
# vpc_id = "vpc-0abc123def456"
# availability_zones = ["us-east-1a", "us-east-1b"]
# config = { "region": "us-east-1", "enable_nat": true }

# 4. Check outputs - schemas were auto-inferred
./bin/gridctl state get vpc-prod

# Output:
# Outputs (3):
#   vpc_id: schema=inferred, validation=valid
#   availability_zones: schema=inferred, validation=valid
#   config: schema=inferred, validation=valid

# 5. View inferred schema for vpc_id
./bin/gridctl state get-output-schema --key vpc_id vpc-prod

# Output:
# Schema (inferred):
# {
#   "type": "string"
# }

# 6. View inferred schema for config (complex object)
./bin/gridctl state get-output-schema --key config vpc-prod

# Output:
# Schema (inferred):
# {
#   "type": "object",
#   "properties": {
#     "region": { "type": "string" },
#     "enable_nat": { "type": "boolean" }
#   },
#   "required": ["region", "enable_nat"]
# }
```

### Scenario 2: Manual Schema Overwrites Inferred

```bash
# 1. Set a more specific schema (overwrites inferred)
cat > vpc_schema.json <<EOF
{
  "type": "string",
  "pattern": "^vpc-[a-f0-9]{8,17}$",
  "description": "AWS VPC ID"
}
EOF

./bin/gridctl state set-output-schema --key vpc_id --schema-file vpc_schema.json vpc-prod

# 2. Verify schema source changed to manual
./bin/gridctl state get vpc-prod

# Output:
# Outputs (3):
#   vpc_id: schema=manual, validation=valid
#   availability_zones: schema=inferred, validation=valid
#   config: schema=inferred, validation=valid
```

### Scenario 3: Pre-declared Schema Prevents Inference

```bash
# 1. Create state and set schema BEFORE Terraform runs
./bin/gridctl state create db-prod --label env=prod

cat > connection_schema.json <<EOF
{
  "type": "object",
  "properties": {
    "host": { "type": "string", "format": "hostname" },
    "port": { "type": "integer", "minimum": 1, "maximum": 65535 },
    "database": { "type": "string", "minLength": 1 }
  },
  "required": ["host", "port", "database"]
}
EOF

./bin/gridctl state set-output-schema --key connection_string --schema-file connection_schema.json db-prod

# 2. Run Terraform - inference is SKIPPED (schema already exists)
cd terraform/rds
./bin/gridctl state init db-prod
terraform apply -auto-approve

# 3. Verify manual schema preserved (not overwritten by inference)
./bin/gridctl state get-output-schema --key connection_string db-prod
# Still shows the detailed schema with format, minimum, required fields
```

---

## Phase 2B: Schema Validation

### Scenario 4: Validation Pass

```bash
# 1. Set a pattern-matching schema
cat > vpc_schema.json <<EOF
{
  "type": "string",
  "pattern": "^vpc-[a-f0-9]{8,17}$"
}
EOF

./bin/gridctl state set-output-schema --key vpc_id --schema-file vpc_schema.json vpc-prod

# 2. Upload state with valid VPC ID
terraform apply -auto-approve
# Output: vpc_id = "vpc-abc123456789"

# 3. Check validation status
./bin/gridctl state get vpc-prod

# Output:
# Outputs (1):
#   vpc_id: schema=manual, validation=valid
#     validated at: 2025-11-25 10:30:00
```

### Scenario 5: Validation Failure

```bash
# 1. Set a strict schema for subnet_ids
cat > subnet_schema.json <<EOF
{
  "type": "array",
  "items": {
    "type": "string",
    "pattern": "^subnet-[a-f0-9]{8,17}$"
  }
}
EOF

./bin/gridctl state set-output-schema --key subnet_ids --schema-file subnet_schema.json vpc-prod

# 2. Terraform outputs invalid data (hypothetically)
# Output: subnet_ids = ["invalid-format", "subnet-abc123"]

# 3. Check validation status - FAILED
./bin/gridctl state get vpc-prod

# Output:
# Outputs (2):
#   vpc_id: schema=manual, validation=valid
#   subnet_ids: schema=manual, validation=invalid
#     error: value at index 0 does not match pattern "^subnet-[a-f0-9]{8,17}$"
#     validated at: 2025-11-25 10:35:00
```

### Output Display Format

**Default behavior:**
- All outputs show schema and validation status inline (no flag required)
- Format: `key: schema=SOURCE, validation=STATUS`
- Timestamps displayed in human-readable format: `validated at: YYYY-MM-DD HH:MM:SS`
- Validation errors shown indented (4 spaces) below affected output

**Outputs without schema:**
```bash
# Example output when some outputs lack schemas:
# Outputs (3):
#   vpc_id: schema=inferred, validation=valid
#   subnet_ids: schema=manual, validation=invalid
#     error: pattern mismatch at /items/0
#     validated at: 2025-11-25 10:35:00
#   unvalidated_output (no schema set)
```

**Schema badge in get-output-schema:**
- Always shows schema source: `Schema (manual):` or `Schema (inferred):`
- Prepended before JSON output for context
```bash
./bin/gridctl state get-output-schema --key vpc_id vpc-prod

# Output:
# Schema (manual):
# {
#   "type": "string",
#   "pattern": "^vpc-[a-f0-9]{8,17}$"
# }
```

---

### Scenario 6: Dependency Edge Status (schema-invalid)

```bash
# Setup: vpc-prod produces vpc_id, app-prod consumes it

# 1. Create app state with dependency
./bin/gridctl state create app-prod --label env=prod
./bin/gridctl state depend --to app-prod --from vpc-prod --output vpc_id

# 2. When vpc_id fails schema validation, edge status changes
./bin/gridctl state get app-prod

# Output:
# Dependencies (1):
#   vpc-prod.vpc_id → app-prod
#     status: schema-invalid
#     error: Producer output "vpc_id" failed schema validation

# 3. Fix the issue (re-apply with valid data)
cd terraform/vpc
terraform apply -auto-approve

# 4. Edge status clears automatically
./bin/gridctl state get app-prod

# Output:
# Dependencies (1):
#   vpc-prod.vpc_id → app-prod
#     status: clean
```

---

## API Usage (Go SDK)

### Check Validation Status

```go
import "github.com/terraconstructs/grid/pkg/sdk"

client, _ := sdk.NewClient(sdk.WithBaseURL("http://localhost:8080"))

// List outputs with validation info
outputs, _ := client.ListStateOutputs(ctx, sdk.StateRef{LogicID: "vpc-prod"})

for _, out := range outputs {
    fmt.Printf("Output: %s\n", out.Key)
    fmt.Printf("  Schema Source: %s\n", *out.SchemaSource) // "manual" or "inferred"

    if out.ValidationStatus != nil {
        fmt.Printf("  Validation: %s\n", *out.ValidationStatus)
        if out.ValidationError != nil {
            fmt.Printf("  Error: %s\n", *out.ValidationError)
        }
        fmt.Printf("  Validated At: %s\n", out.ValidatedAt.Format(time.RFC3339))
    }
}
```

### Check Edge Schema Status

```go
// Get state with dependencies
state, _ := client.GetStateInfo(ctx, sdk.StateRef{LogicID: "app-prod"})

for _, edge := range state.IncomingEdges {
    if edge.Status == "schema-invalid" {
        fmt.Printf("WARNING: Dependency %s.%s has schema violation\n",
            edge.FromStateLogicID, edge.FromOutput)
    }
}
```

---

## Webapp UI

### Outputs Tab (DetailView)

When viewing a state's detail modal:

1. Click the **"Outputs"** tab
2. Each output shows:
   - **Output key** (code font)
   - **Schema badge**: "manual" (blue) or "inferred" (gray)
   - **Validation status**: ✓ Valid (green), ⚠ Invalid (orange), ✗ Error (red)
   - **Error details** (if validation failed)
   - **"View Schema"** button to expand full JSON Schema

### Dependency Graph (GraphView)

- **Red dashed edge**: `schema-invalid` status
- Hover shows: "Schema validation failed for output X"

---

## Troubleshooting

### Q: Why didn't my schema get inferred?

**A**: Schema inference only runs when:
1. Output has no existing schema (manual or inferred)
2. State upload succeeds
3. Output value is non-null

Check: `./bin/gridctl state get-output-schema --key <key> <state>`

If schema exists, inference is skipped.

### Q: Validation says "error" not "invalid" - what's wrong?

**A**: `error` means the validation system failed (not the data):
- Schema JSON is malformed
- Schema compilation failed
- Internal timeout

Check the `validation_error` message for details.

### Q: Edge shows "schema-invalid" but I fixed the data - why?

**A**: Edge status updates after the **next state upload**. Run:
```bash
terraform apply  # Or terraform refresh
```

The edge update job runs asynchronously after upload.

### Q: Can I disable inference for a specific output?

**A**: Yes, set an empty permissive schema:
```bash
echo '{}' > permissive.json
./bin/gridctl state set-output-schema --key <key> --schema-file permissive.json <state>
```

An empty schema `{}` accepts any value but prevents inference.
