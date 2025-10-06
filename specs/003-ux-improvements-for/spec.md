# Feature Specification: CLI Context-Aware State Management

**Feature Branch**: `003-ux-improvements-for`
**Created**: 2025-10-03
**Status**: Draft
**Input**: User description: "UX improvements for the grid SDK/CLI: when running state and deps commands, the current directory should provide \"context\". When a state is created the information of this new state should be stored in the current directory (if the directory already has state information ... it should error out unless a --force flag is provided). All the gridctl commands should default to using the state information in the current directory (for example default for the logic_id or for deps management default it should default the `--to-state` flag.) currently the deps command requires the --output flag to define which output of the from-state is being used to create the dependency edge. This is a bit cumbersome, especially when there are many outputs. A better UX would be to allow the user to specify just the from-state and have the CLI prompt them to select which output they want to use (if there are multiple outputs). This would make it easier to create dependencies without needing to remember or look up the exact output names. similarly, when getting state information (see grictl state get), it should include information about dependencies and dependents as well as all outputs the state has."

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

### Session 2025-10-03
- Q: How should corrupted/invalid directory state information be handled? → A: Warning logged, information ignored, user can proceed
- Q: How should write permission issues be handled when creating state context? → A: Detect early, warn state context won't be stored, allow state creation without storing context
- Q: How should concurrent writes to the same directory be handled? → A: First write wins, second write errors with notice to retry with --force
- Q: How should invalid state (deleted from server) in directory context be handled? → A: Error and prompt to run create command
- Q: Should parent directories be checked for state information? → A: No, only current directory (well-known Grid context file)
- Q: What should be included in directory state information besides GUID and logic-id? → A: GUID (required), flag if logic-id changed
- Q: Should interactive output selection allow selecting one or multiple outputs? → A: Allow selecting multiple outputs (creates multiple dependency edges in one operation)
- Q: When displaying state information, should output values be shown or just output keys? → A: Display only output keys (names), not values
- Q: Should there be a --non-interactive flag for CI/automation scenarios? → A: Yes, add --non-interactive flag that errors when prompting would occur

---

## User Scenarios & Testing *(mandatory)*

### Primary User Story
As a Grid CLI user working with Terraform/OpenTofu states, I want my current working directory to automatically provide context to all state and dependency commands, so I don't need to repeatedly specify which state I'm working with. When I create a state in a directory, that directory should remember the state association. When I work with dependencies, I want interactive prompts to help me select outputs instead of memorizing exact output names. When I view state information, I want to see the full picture including all dependencies and outputs.

### Acceptance Scenarios

#### Scenario 1: Creating State with Directory Context
1. **Given** I am in an empty directory `/project/frontend`
   **When** I run the command to create a new state
   **Then** state information is saved in the current directory
   **And** subsequent commands in this directory use this state by default

2. **Given** I am in a directory `/project/frontend` that already has state information
   **When** I attempt to create another state without a force flag
   **Then** the system prevents the operation with an error message
   **And** suggests using the force flag if I intend to replace the existing state

3. **Given** I am in a directory with existing state information
   **When** I run the command to create a new state with the force flag
   **Then** the old state information is replaced with the new state information

#### Scenario 2: Commands Using Directory Context
1. **Given** I am in directory `/project/frontend` with saved state information
   **When** I run any state or dependency command without specifying state identifiers
   **Then** the command automatically uses the state associated with this directory

2. **Given** I am in a directory without state information
   **When** I run a command that requires state context without providing identifiers
   **Then** the system returns an error indicating no state context found

#### Scenario 3: Interactive Output Selection for Dependencies
1. **Given** I want to create a dependency from `state-a` which has multiple outputs (e.g., "vpc_id", "subnet_id", "security_group_id")
   **When** I specify only the from-state without the output flag
   **Then** the system fetches all available outputs from the from-state
   **And** presents an interactive selection menu showing all outputs
   **And** I can select one or more outputs to use for the dependencies

2. **Given** I want to create a dependency from `state-b` which has a single output
   **When** I specify only the from-state without the output flag
   **Then** the system automatically uses the single available output without prompting

3. **Given** I want to create a dependency from `state-c` which has no outputs
   **When** I specify only the from-state without the output flag
   **Then** the system allows me to proceed with mock dependency creation

4. **Given** I am running in a CI/automation environment with `--non-interactive` flag
   **When** I attempt to create a dependency from a state with multiple outputs without specifying the output flag
   **Then** the system returns an error immediately without prompting

#### Scenario 4: Enhanced State Information Display
1. **Given** I have a state with dependencies (consuming outputs from other states)
   **When** I request state information
   **Then** the display shows all dependencies with their source states and output names

2. **Given** I have a state with dependents (other states consuming its outputs)
   **When** I request state information
   **Then** the display shows all dependent states and which outputs they consume

3. **Given** I have a state with outputs defined
   **When** I request state information
   **Then** the display lists all output names (keys only, not values)

### Edge Cases
- When directory state information file is corrupted or has invalid format, system logs warning and ignores the information, allowing user to proceed as if no context exists
- When user lacks write permissions in directory, system detects early, warns that state context won't be stored, and allows state creation without storing context
- When multiple processes attempt to write state information to same directory simultaneously, first write wins and second write returns error instructing user to retry with --force
- When from-state specified in dependency command doesn't exist, system returns error indicating state not found
- When API is unavailable while fetching outputs for interactive selection, system returns error with network/connectivity details
- When state referenced in directory context no longer exists on server (was deleted), system returns error and prompts user to run create command
- Parent directories are NOT checked for state information; only current directory's well-known Grid context file is used
- If state information in directory exists and logic-id is the same as provided by the user, no changes are made (force flag not required).

---

## Requirements *(mandatory)*

### Functional Requirements

#### Directory Context Management
- **FR-001**: System MUST persist state association information in the current working directory when a state is created
- **FR-002**: System MUST prevent creating a new state in a directory that already has state information, unless a force flag is explicitly provided or the logic-id matches the existing state
- **FR-003**: System MUST provide a force flag option that allows overwriting existing state information in a directory (even if logic-id matches, new GUID is generated)
- **FR-004**: System MUST read state information from the current directory to provide default values for state identifier parameters
- **FR-005**: System MUST display clear error messages when commands requiring state context are run in directories without state information
- **FR-006**: State information stored in directories MUST include the state GUID (required)
- **FR-006a**: System MUST search only the current working directory for state context information, not parent directories
- **FR-006b**: When directory state information is corrupted or invalid, system MUST log a warning and ignore the information
- **FR-006c**: When user lacks write permissions in directory, system MUST detect early, warn that context won't be stored, and allow state creation to proceed
- **FR-006d**: When multiple processes attempt concurrent writes to same directory, first write MUST succeed and subsequent writes MUST fail with instruction to retry with --force
- **FR-006e**: When state referenced in directory context no longer exists on server, system MUST return error and prompt user to run create command

#### Default Parameter Behavior
- **FR-007**: All state commands MUST use the current directory's state information as the default for the state guid or logic_id parameter when not explicitly provided
- **FR-008**: All dependency commands MUST use the current directory's state as the default for the to-state parameter when not explicitly provided
- **FR-009**: System MUST allow explicit parameter values to override directory context defaults

#### Interactive Output Selection
- **FR-010**: Dependency creation commands MUST support creating dependencies without requiring the output flag
- **FR-011**: When output flag is not provided and from-state has multiple outputs, system MUST fetch all available outputs from the from-state
- **FR-012**: System MUST present an interactive selection menu showing all available outputs from the from-state
- **FR-013**: Interactive selection menu MUST allow users to choose one or more outputs, creating multiple dependency edges in a single operation
- **FR-014**: When from-state has exactly one output, system MUST automatically use that output without prompting
- **FR-015**: When from-state has zero outputs, system MUST allow mock dependency creation
- **FR-016**: System MUST handle cases where output fetching fails with appropriate error messages
- **FR-017**: System MUST provide a --non-interactive flag that causes commands to error immediately when interactive prompting would occur, enabling use in CI/automation scenarios

#### Enhanced State Information Display
- **FR-018**: State information retrieval MUST include all dependencies showing which states and outputs are being consumed
- **FR-019**: State information retrieval MUST include all dependents showing which states are consuming this state's outputs
- **FR-020**: State information retrieval MUST include a list of all output names (keys) that the state provides, without displaying output values
- **FR-021**: Dependency information MUST show the output name and the source state identifier
- **FR-022**: Dependent information MUST show the dependent state identifier and which output they consume

#### Server-Side Output Caching (identified during planning)
- **FR-023**: Server MUST cache Terraform output keys in database (state_outputs table) for performance optimization
- **FR-024**: Server MUST invalidate output cache atomically when Terraform state serial number changes
- **FR-025**: Server MUST support cross-state search by output key name (e.g., "find all states with 'vpc_id' output")
- **FR-026**: ListStateOutputs RPC MUST read from cached state_outputs table, not parse Terraform state JSON on every call
- **FR-027**: Terraform HTTP backend PUT handler MUST update state_outputs table in same transaction as state data update
- **FR-028**: Output cache invalidation MUST be idempotent (delete old outputs with different serial + insert new outputs)
- **FR-029**: State deletion MUST cascade delete all cached outputs for that state (enforced via foreign key constraint)

### Key Entities

- **Directory State Context**: Information stored in a user's working directory that associates that directory with a specific Grid state. Contains state identifiers (GUID and logic-id) that enable directory-aware command execution.

- **State Output**: A named value exported by a Terraform/OpenTofu state that can be consumed by other states. Has a key (output name) and represents data that dependent states can reference.

- **Dependency**: A relationship between two states where one state (dependent/to-state) consumes an output from another state (source/from-state). The relationship is defined by specifying which specific output is being consumed.

- **Interactive Selection Session**: A user interaction where the system presents multiple options (outputs) and waits for the user to make a selection before proceeding with the command.

---

## Review & Acceptance Checklist
*GATE: Automated checks run during main() execution*

### Content Quality
- [x] No implementation details (languages, frameworks, APIs)
- [x] Focused on user value and business needs
- [x] Written for non-technical stakeholders
- [x] All mandatory sections completed

### Requirement Completeness
- [x] No [NEEDS CLARIFICATION] markers remain
- [x] Requirements are testable and unambiguous
- [x] Success criteria are measurable
- [x] Scope is clearly bounded
- [x] Dependencies and assumptions identified

---

## Execution Status
*Updated by main() during processing*

- [x] User description parsed
- [x] Key concepts extracted
- [x] Ambiguities marked
- [x] User scenarios defined
- [x] Requirements generated
- [x] Entities identified
- [x] Review checklist passed

---
