# Quickstart: Terraform State Management with Grid

**Feature**: 001-develop-the-grid
**Target Audience**: DevOps engineers using Terraform/OpenTofu
**Time**: 10 minutes
**Prerequisites**: Docker, Terraform or OpenTofu CLI

## Overview

This quickstart demonstrates Grid's Terraform state management capabilities:
1. Starting the Grid API server
2. Creating a new state via CLI
3. Configuring Terraform to use Grid backend
4. Running Terraform operations with remote state

## Step 1: Start Grid API Server

```bash
# Start PostgreSQL database (docker-compose.yml in repo root)
docker compose up -d postgres

# Wait for healthy status
docker compose ps postgres

# Run database migrations
gridapi db migrate --db-url="postgres://grid:gridpass@localhost:5432/grid?sslmode=disable"

# Start API server
gridapi serve --port=8080 --db-url="postgres://grid:gridpass@localhost:5432/grid?sslmode=disable"
```

**Expected Output**:
```
2025-09-30T10:00:00Z INFO Starting Grid API server port=8080
2025-09-30T10:00:00Z INFO Database connected
2025-09-30T10:00:00Z INFO Connect RPC handlers mounted
2025-09-30T10:00:00Z INFO Terraform HTTP Backend handlers mounted path=/tfstate
2025-09-30T10:00:00Z INFO Server listening addr=:8080
```

**Verify**: `curl http://localhost:8080/health` returns `{"status":"ok"}`

## Step 2: Create State via CLI

```bash
# Create new state with logic-id "demo-project"
# CLI generates UUIDv7, sends to server with logic-id
gridctl state create demo-project --server=http://localhost:8080

# Alternative: List all states (empty initially)
gridctl state list --server=http://localhost:8080
```

**Expected Output**:
```
Generating UUID: 018e8c5e-7890-7000-8000-123456789abc
Created state: demo-project
GUID: 018e8c5e-7890-7000-8000-123456789abc

Backend configuration:
  address:        http://localhost:8080/tfstate/018e8c5e-7890-7000-8000-123456789abc
  lock_address:   http://localhost:8080/tfstate/018e8c5e-7890-7000-8000-123456789abc/lock
  unlock_address: http://localhost:8080/tfstate/018e8c5e-7890-7000-8000-123456789abc/unlock

Run 'gridctl state init demo-project' in your Terraform directory to generate backend config.
```

**Validation**:
- GUID is a valid UUIDv7 format (timestamp-ordered, starts with hex timestamp)
- CLI generates GUID locally (no server roundtrip for GUID allocation)
- Backend URLs include the GUID
- Endpoints are mounted on API server base URL

## Step 3: Initialize Terraform Backend

```bash
# Create sample Terraform project
mkdir terraform-demo && cd terraform-demo

# Generate backend configuration
gridctl state init demo-project --server=http://localhost:8080

# Verify generated file
cat backend.tf
```

**Expected Output** (backend.tf):
```hcl
terraform {
  backend "http" {
    address        = "http://localhost:8080/tfstate/018e8c5e-7890-7000-8000-123456789abc"
    lock_address   = "http://localhost:8080/tfstate/018e8c5e-7890-7000-8000-123456789abc/lock"
    unlock_address = "http://localhost:8080/tfstate/018e8c5e-7890-7000-8000-123456789abc/unlock"
  }
}
```

**Validation**:
- File created in current directory
- Backend type is "http"
- All three endpoints present with correct GUID

## Step 4: Create Sample Terraform Configuration

```bash
# Create main.tf with null resources
cat > main.tf <<'EOF'
resource "null_resource" "example" {
  triggers = {
    timestamp = "2025-09-30T10:00:00Z"
  }
}

resource "null_resource" "another" {
  triggers = {
    value = "initial"
  }
}
EOF
```

## Step 5: Run Terraform Init

```bash
terraform init
```

**Expected Output**:
```
Initializing the backend...

Successfully configured the backend "http"! Terraform will automatically
use this backend unless the backend configuration changes.

Initializing provider plugins...
- Finding latest version of hashicorp/null...
- Installing hashicorp/null v3.2.2...
- Installed hashicorp/null v3.2.2 (signed by HashiCorp)

Terraform has been successfully initialized!
```

**Validation**:
- Backend initialized successfully
- No state file in local directory (.terraform/ exists but no terraform.tfstate)
- Server logs show GET request to /tfstate/{guid} returning 404 (state not yet created)

## Step 6: Apply Configuration

```bash
terraform apply -auto-approve
```

**Expected Output**:
```
Terraform will perform the following actions:

  # null_resource.another will be created
  + resource "null_resource" "another" {
      + id       = (known after apply)
      + triggers = {
          + "value" = "initial"
        }
    }

  # null_resource.example will be created
  + resource "null_resource" "example" {
      + id       = (known after apply)
      + triggers = {
          + "timestamp" = "2025-09-30T10:00:00Z"
        }
    }

Plan: 2 to add, 0 to change, 0 to destroy.

null_resource.example: Creating...
null_resource.another: Creating...
null_resource.example: Creation complete after 0s [id=123...]
null_resource.another: Creation complete after 0s [id=456...]

Apply complete! Resources: 2 added, 0 changed, 0 destroyed.
```

**Validation**:
- Resources created successfully
- Server logs show:
  - `LOCK /tfstate/{guid}/lock` - 200 OK
  - `POST /tfstate/{guid}` - 200 OK (state stored)
  - `UNLOCK /tfstate/{guid}/unlock` - 200 OK
- Still no local terraform.tfstate file

## Step 7: Verify Remote State

```bash
# List states via CLI
gridctl state list --server=http://localhost:8080

# Query state directly via API (dev/debug only)
curl http://localhost:8080/tfstate/018e8c5e-7890-7000-8000-123456789abc | jq .version
```

**Expected Output** (list):
```
GUID                                    LOGIC_ID        LOCKED  CREATED                 UPDATED
018e8c5e-7890-7000-8000-123456789abc   demo-project    false   2025-09-30T10:00:00Z   2025-09-30T10:05:00Z
```

**Expected Output** (curl):
```json
4
```

**Validation**:
- State exists in Grid
- State content is valid Terraform JSON
- State not locked (apply completed)
- Updated timestamp reflects recent apply

## Step 8: Modify and Plan

```bash
# Update main.tf to change trigger value
cat > main.tf <<'EOF'
resource "null_resource" "example" {
  triggers = {
    timestamp = "2025-09-30T11:00:00Z"  # Changed
  }
}

resource "null_resource" "another" {
  triggers = {
    value = "updated"  # Changed
  }
}
EOF

# Run plan
terraform plan
```

**Expected Output**:
```
null_resource.another: Refreshing state... [id=456...]
null_resource.example: Refreshing state... [id=123...]

Terraform will perform the following actions:

  # null_resource.another will be replaced
  ~ resource "null_resource" "another" {
      ~ id       = "456..." -> (known after apply)
      ~ triggers = {
          ~ "value" = "initial" -> "updated"
        }
    }

  # null_resource.example will be replaced
  ~ resource "null_resource" "example" {
      ~ id       = "123..." -> (known after apply)
      ~ triggers = {
          ~ "timestamp" = "2025-09-30T10:00:00Z" -> "2025-09-30T11:00:00Z"
        }
    }

Plan: 2 to add, 0 to change, 2 to destroy.
```

**Validation**:
- Terraform retrieved current state from Grid
- Calculated diff correctly (2 resources to replace)
- Server logs show:
  - `LOCK /tfstate/{guid}/lock` - 200 OK
  - `GET /tfstate/{guid}` - 200 OK
  - `UNLOCK /tfstate/{guid}/unlock` - 200 OK (plan doesn't modify state)

## Step 9: Apply Changes

```bash
terraform apply -auto-approve
```

**Expected Output**:
```
null_resource.another: Refreshing state... [id=456...]
null_resource.example: Refreshing state... [id=123...]

Plan: 2 to add, 0 to change, 2 to destroy.

null_resource.another: Destroying... [id=456...]
null_resource.example: Destroying... [id=123...]
null_resource.another: Destruction complete after 0s
null_resource.example: Destruction complete after 0s
null_resource.example: Creating...
null_resource.another: Creating...
null_resource.example: Creation complete after 0s [id=789...]
null_resource.another: Creation complete after 0s [id=abc...]

Apply complete! Resources: 2 added, 0 changed, 2 destroyed.
```

**Validation**:
- Resources replaced successfully
- State updated in Grid
- Server logs show POST with updated state content
- State version incremented (query via curl shows version 5+)

## Step 10: Test Lock Behavior (Optional)

```bash
# In terminal 1: Start long-running apply (simulate with sleep in null_resource)
terraform apply -auto-approve

# In terminal 2 (while apply is running): Try concurrent plan
terraform plan
```

**Expected Output** (terminal 2):
```
Error: Error acquiring the state lock

Error message: state is locked
Lock Info:
  ID:        <lock-uuid>
  Path:      demo-project
  Operation: apply
  Who:       user@hostname
  Version:   1.5.0
  Created:   2025-09-30T10:10:00Z

Terraform will automatically retry to acquire the lock. If lock
acquisition continues to fail, you may need to manually unlock the state.
```

**Validation**:
- Lock conflict detected
- Server returned 423 Locked with lock info
- Terraform displays lock details
- Terraform retries automatically (or fails after timeout)

## Success Criteria

✅ API server starts and connects to database
✅ CLI creates state and generates backend config
✅ Terraform init succeeds with Grid backend
✅ Terraform apply stores state remotely
✅ Terraform plan retrieves and uses remote state
✅ State locking prevents concurrent operations
✅ State listing shows all states with metadata
✅ No local terraform.tfstate file created

## Cleanup

```bash
# From Terraform directory
terraform destroy -auto-approve

# Stop API server (Ctrl+C)

# Stop and remove PostgreSQL (with volumes)
docker compose down -v

# Or stop without removing volumes
docker compose down
```

## Troubleshooting

### Connection Refused
- Verify API server is running: `curl http://localhost:8080/health`
- Check --server flag matches API server address

### Lock Timeout
- Check server logs for lock info
- Manually unlock if needed (future feature)
- Ensure previous operation completed

### State Not Found (404)
- Verify GUID in backend.tf matches created state
- Confirm state exists: `gridctl state list`
- Check for typos in logic-id

### Database Connection Error
- Verify PostgreSQL is running: `docker compose ps postgres`
- Check health status: `docker compose ps` (should show "healthy")
- Check connection string format
- Ensure migrations ran successfully
- View logs: `docker compose logs -f postgres`

## Advanced: Configuring Alternative HTTP Methods

Grid supports flexible HTTP method configuration for compatibility:

```hcl
# backend-custom-methods.tf
terraform {
  backend "http" {
    address        = "http://localhost:8080/tfstate/018e8c5e-7890-7000-8000-123456789abc"
    lock_address   = "http://localhost:8080/tfstate/018e8c5e-7890-7000-8000-123456789abc/lock"
    unlock_address = "http://localhost:8080/tfstate/018e8c5e-7890-7000-8000-123456789abc/unlock"

    # Override default methods
    update_method  = "PUT"     # Default: POST
    lock_method    = "POST"    # Default: LOCK
    unlock_method  = "DELETE"  # Default: UNLOCK
  }
}
```

**Supported Methods**:
- Update: POST, PUT, PATCH
- Lock: LOCK (default), PUT, POST
- Unlock: UNLOCK (default), PUT, DELETE, POST

**Environment Variables**:
```bash
export TF_HTTP_LOCK_METHOD=PUT
export TF_HTTP_UNLOCK_METHOD=DELETE
terraform init  # Uses environment variable overrides
```

## Advanced: Disabling Locking

For single-user scenarios, locking can be disabled:

```hcl
# backend-no-lock.tf
terraform {
  backend "http" {
    address = "http://localhost:8080/tfstate/018e8c5e-7890-7000-8000-123456789abc"
    # Omit lock_address and unlock_address to disable locking
  }
}
```

**Warning**: Without locking, concurrent Terraform operations may corrupt state!

## Next Steps

- Add authentication/authorization (future feature)
- Implement state versioning and history (future feature)
- Add state deletion (future feature)
- Explore multi-user collaboration scenarios
- Configure custom HTTP methods for compatibility
- Implement request tracing with custom headers

## Related Documentation

- Terraform HTTP Backend: https://developer.hashicorp.com/terraform/language/settings/backends/http
- OpenTofu Compatibility: https://opentofu.org/docs/language/settings/backends/http
- Grid API Documentation: docs/terraform-backend.md