# Tasks: E2E No-Authentication Tests

**Feature**: `e2e-no-auth`
**Version**: `1.0.0`
**Status**: `DRAFT`

This document lists the ordered tasks required to implement, and validate the E2E test suite for the `gridapi` no-authentication mode.

## Phase 1: Test Scaffolding & Setup

- [ ] **T001**: Create new directory `specs/009-e2e-noauth` for design documents.
- [ ] **T002**: Create `specs/009-e2e-noauth/spec.md` defining the functional requirements.
- [ ] **T003**: Create `specs/009-e2e-noauth/plan.md` detailing the implementation plan.
- [ ] **T004**: Create this `specs/009-e2e-noauth/tasks.md` file.
- [ ] **T005**: Create `specs/009-e2e-noauth/quickstart.md` for running the tests.

## Phase 2: Implementation

- [ ] **T006**: Create the `tests/e2e/setup/start-no-auth-services.sh` script to orchestrate services for the no-auth test environment.
- [ ] **T007**: Make the `start-no-auth-services.sh` script executable (`chmod +x`).
- [ ] **T008**: Create the `tests/e2e/no-auth-flow.spec.ts` test file containing the Playwright test logic.
- [ ] **T009**: Update the Playwright configuration (`playwright.config.ts`) to include a new project for the "no-auth" flow, using the new setup script.

## Phase 3: Integration & Validation

- [ ] **T010**: Add a `test:e2e:no-auth` script to the root `package.json` file.
- [ ] **T011**: Add a `test-e2e-no-auth` target to the root `Makefile`.
- [ ] **T012**: Run `make test-e2e-no-auth` and verify that all tests pass successfully.
- [ ] **T013**: Review and merge all new and modified files.
