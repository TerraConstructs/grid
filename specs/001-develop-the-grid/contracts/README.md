# API Contracts

This directory contains API contract definitions for the Terraform State Management feature.

## Files

### state_service.proto
Connect RPC service definition for state management operations. This contract is:
- Used by Go SDK and future TypeScript/JavaScript SDK
- Generates code into `api/state/v1` module
- Consumed by CLI and webapp clients
- Follows constitutional SDK-first pattern

**Operations**:
- `CreateState`: Create new state with **client-generated UUIDv7** + logic_id, returns backend config
- `ListStates`: List all states with metadata
- `GetStateConfig`: Retrieve backend config for existing state

**Note**: Client (CLI) generates UUIDv7 per specification to save API roundtrip and optimize Postgres index performance.

### terraform-backend-rest.yaml
OpenAPI 3.0 specification for Terraform HTTP Backend REST API. This contract is:
- **Constitutional exception**: Not reflected in protobuf or SDKs
- Consumed directly by Terraform/OpenTofu CLI binaries
- Documents the standard Terraform HTTP Backend protocol
- Must be implemented at `/tfstate/{guid}` endpoints on API server

**Caution - Custom HTTP Methods**:
Terraform uses **LOCK** and **UNLOCK** as custom HTTP methods by default (not PUT):
- Source: `github.com/opentofu/opentofu/internal/backend/remote-state/http/backend.go`
- Default `lock_method`: "LOCK"
- Default `unlock_method`: "UNLOCK"
- OpenAPI 3.0 cannot document custom methods (limitation)

**Endpoints** (actual HTTP methods shown):
- `GET /tfstate/{guid}`: Retrieve state content
- `POST /tfstate/{guid}`: Update state content (configurable via `update_method`)
- **`LOCK /tfstate/{guid}/lock`**: Acquire lock (Terraform default)
- **`UNLOCK /tfstate/{guid}/unlock`**: Release lock (Terraform default)

**Implementation implications**:
Server should support both default methods (LOCK/UNLOCK) and alternatives (PUT/POST/etc) following more common REST practices.

## Contract Testing

Both contracts MUST have failing contract tests generated before implementation:

### Connect RPC Contract Tests
Location: `tests/contract/state_service_test.go`
- Table-driven tests for each RPC method
- Success cases and error cases
- Follow https://kmcd.dev/posts/connectrpc-unittests patterns

### Terraform Backend REST Tests
Location: `tests/contract/terraform_backend_test.go`
- HTTP request/response validation against OpenAPI spec
- Lock conflict scenarios (423 responses)
- State size warning header validation

## Usage in Implementation

1. **Phase 1 (Current)**: Contracts defined, tests written (failing)
2. **Phase 2**: Tasks generated from contracts
3. **Phase 3**: Implementation makes tests pass
4. **Phase 4**: Integration tests with real Terraform CLI

## Constitutional Compliance

- ✅ Connect RPC follows Constitution IV (Cross-Language Parity)
- ✅ REST API exception documented per Constitution IV (Terraform HTTP Backend)
- ✅ Both contracts have TDD tests per Constitution V (Test Strategy)