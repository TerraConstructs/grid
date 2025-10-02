# Feature Specification: Terraform State Management Framework

**Feature Branch**: `001-develop-the-grid`
**Created**: 2025-09-30
**Status**: Draft
**Input**: User description: "Develop the grid terraform state management framework with initial functionality to define and use a state. For this feature, mutable user-provided state logic-id (is required from the user) as the only state metadata. The client CLI generates immutable GUID for the newly created state which is persisted by the API server. The server response should return the necessary endpoints for terraform HTTP Backend configuration such that the newly allocated state can be used (endpoints are mounted on our apiserver baseURL but /tfstate/*). Users may run the client CLI in current directory to initialise the state, the CLI will generate a simple HCL file on disk with the terraform backend configuration received from the server. Once defined in a directory, running terraform init or tofu init should work, a sample terraform configuration with null resources, should correctly store the terraform state JSON via the apiserver in a database for persistence and changes to the null resources should plan and apply correctly. It should be possible to list all the states (showing GUID to logic_id tab delimited in the terminal)"

## Execution Flow (main)
```
1. Parse user description from Input
   → If empty: ERROR "No feature description provided"
2. Extract key concepts from description
   → Identify: actors, actions, data, constraints
3. For each unclear aspect:
   → Mark with [NEEDS CLARIFICATION: specific question]
4. Fill User Scenarios & Testing section
   → If no clear user flow: ERROR "Cannot determine user scenarios"
5. Generate Functional Requirements
   → Each requirement must be testable
   → Mark ambiguous requirements
6. Identify Key Entities (if data involved)
7. Run Review Checklist
   → If any [NEEDS CLARIFICATION]: WARN "Spec has uncertainties"
   → If implementation details found: ERROR "Remove tech details"
8. Return: SUCCESS (spec ready for planning)
```

---

## ⚡ Quick Guidelines
- ✅ Focus on WHAT users need and WHY
- ❌ Avoid HOW to implement (no tech stack, APIs, code structure)
- 👥 Written for business stakeholders, not developers

---

## Clarifications

### Session 2025-09-30
- Q: Logic-id uniqueness scope - what uniqueness constraint should apply? → A: Globally unique - No two states can have the same logic-id anywhere in the system
- Q: Backend configuration file collision - what should happen if a file already exists? → A: Prompt user - Ask for confirmation before overwriting
- Q: State file size limits - should the system enforce a maximum size limit? → A: Warn at threshold - Accept all sizes but warn users when exceeding reasonable threshold (e.g., 10MB)
- Q: Non-existent state access - what should happen when Terraform attempts to access a state that doesn't exist? → A: Return 404 error - Reject request, require user to create state first via CLI
- Q: Concurrent lock conflicts - what should happen when attempting to lock an already-locked state? → A: Return lock error (immediately, let Terraform handle retry)

---

## User Scenarios & Testing

### Primary User Story
As a DevOps engineer managing Terraform/OpenTofu infrastructure, I want to store my Terraform state files remotely in Grid so that my team can collaborate on infrastructure changes, track state history, and avoid state file conflicts when multiple people work on the same infrastructure.

### Acceptance Scenarios

1. **Given** I have Grid CLI installed, **When** I create a new state with logic-id "production-us-east", **Then** the system generates a unique identifier and provides Terraform HTTP backend configuration that I can use in my project

2. **Given** I have backend configuration in my Terraform project, **When** I run `terraform init` or `tofu init`, **Then** Terraform successfully connects to Grid's state backend and initializes

3. **Given** my Terraform is initialized with Grid backend, **When** I run `terraform apply` on a configuration with null resources, **Then** the state is stored remotely in Grid and I can see state updates after applying changes

4. **Given** I have multiple states stored in Grid, **When** I run the CLI list command, **Then** I see all my states displayed with their unique identifiers and logic-ids

5. **Given** I create a state in one directory, **When** I modify null resources and run `terraform plan`, **Then** Terraform correctly retrieves the current state, calculates changes, and can apply them

### Edge Cases
- System MUST reject state creation with a duplicate logic-id and provide clear error message
- CLI MUST prompt for user confirmation when backend configuration file already exists, allowing user to abort or overwrite
- System MUST return HTTP 404 error when accessing non-existent state (invalid GUID), requiring explicit state creation via CLI
- System MUST return lock error immediately when concurrent Terraform operations attempt to lock the same state (Terraform handles retry logic)
- How does the system handle state file corruption or invalid state data?

## Requirements

### Functional Requirements

**State Creation**
- **FR-001**: Users MUST be able to create a new remote state by providing a logic-id (human-readable identifier)
- **FR-002**: System MUST generate a globally unique immutable identifier (GUID) for each new state
- **FR-003**: System MUST store the mapping between GUID and user-provided logic-id
- **FR-003a**: System MUST enforce global uniqueness of logic-ids and reject creation attempts with duplicate logic-ids
- **FR-004**: System MUST return Terraform HTTP Backend configuration endpoints (address, lock_address, unlock_address) upon state creation
- **FR-005**: Backend configuration endpoints MUST use the pattern `/tfstate/{GUID}` appended to the API server base URL

**Backend Configuration Generation**
- **FR-006**: CLI MUST generate a valid HCL file with Terraform HTTP backend configuration in the current directory
- **FR-006a**: CLI MUST prompt user for confirmation if backend configuration file already exists before overwriting
- **FR-007**: Generated HCL configuration MUST include address, lock_address, and unlock_address fields pointing to Grid endpoints
- **FR-008**: Generated configuration MUST be compatible with both Terraform and OpenTofu CLI tools

**Terraform/OpenTofu Integration**
- **FR-009**: System MUST implement the Terraform HTTP Backend protocol specification
- **FR-010**: System MUST accept state storage requests from `terraform init` and `tofu init` commands
- **FR-011**: System MUST persist Terraform state data in a database for durability
- **FR-012**: System MUST support state retrieval for `terraform plan` operations
- **FR-013**: System MUST support state updates for `terraform apply` operations
- **FR-014**: System MUST handle state locking to prevent concurrent modification conflicts
- **FR-014a**: System MUST return lock error immediately when attempting to lock an already-locked state (no queuing or waiting)
- **FR-015**: System MUST handle state unlocking after operations complete
- **FR-016**: System MUST return HTTP 404 error when Terraform attempts to access a non-existent state (invalid GUID)

**State Listing**
- **FR-017**: Users MUST be able to list all their states
- **FR-018**: State list output MUST display GUID and logic-id in tab-delimited format for each state
- **FR-019**: State list MUST show all states the user has access to

**Data Persistence**
- **FR-020**: System MUST store state metadata (GUID, logic-id) persistently
- **FR-021**: System MUST store Terraform state file contents (JSON) persistently
- **FR-022**: System MUST maintain data integrity across server restarts
- **FR-023**: System MUST accept state files of any size but SHOULD warn users when state file size exceeds a reasonable threshold (e.g., 10MB)

### Key Entities

- **State**: Represents a Terraform/OpenTofu state file managed by Grid
  - Unique identifier (GUID, immutable, client-generated)
  - Logic ID (mutable, user-provided, human-readable label, globally unique across all states)
  - State file content (Terraform state JSON)
  - Lock status (to prevent concurrent modifications)
  - Created timestamp
  - Last modified timestamp
  - Note: No ownership/permissions model in initial version (no authentication/authorization)

- **Backend Configuration**: The HCL code generated for Terraform projects
  - Address endpoint (for state retrieval/storage)
  - Lock endpoint (for acquiring lock)
  - Unlock endpoint (for releasing lock)
  - Associated state GUID

---

## Review & Acceptance Checklist

### Content Quality
- [x] No implementation details (languages, frameworks, APIs)
- [x] Focused on user value and business needs
- [x] Written for non-technical stakeholders
- [x] All mandatory sections completed

### Requirement Completeness
- [ ] No [NEEDS CLARIFICATION] markers remain
- [x] Requirements are testable and unambiguous
- [x] Success criteria are measurable
- [x] Scope is clearly bounded
- [x] Dependencies and assumptions identified

**Outstanding Clarifications**:
- State retention/deletion policies

---

## Execution Status

- [x] User description parsed
- [x] Key concepts extracted
- [x] Ambiguities marked
- [x] User scenarios defined
- [x] Requirements generated
- [x] Entities identified
- [ ] Review checklist passed (pending clarifications)

---