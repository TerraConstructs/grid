# Specification: E2E Tests for No-Authentication Mode

**Feature**: `e2e-no-auth`
**Version**: `1.0.0`
**Status**: `DRAFT`

## 1. Overview

This document specifies the requirements for an end-to-end (E2E) test suite that validates the Grid web application's functionality when the backing `gridapi` server is running without any authentication enabled.

This mode is critical for local development, rapid UI prototyping, and simple testing scenarios where the overhead of setting up an Identity Provider (IdP) is undesirable.

## 2. User Stories

- **As a Developer**, I want to run the Grid webapp against a `gridapi` instance with authentication disabled, so that I can quickly inspect the UI and test graph rendering with seeded data without configuring Keycloak or another IdP.
- **As a Release Manager**, I want an automated E2E test that guarantees the webapp's "no-auth" mode is not broken by changes to authentication or other features, ensuring backward compatibility and development flexibility.

## 3. Functional Requirements (FR)

| ID      | Requirement                                                                                                                              |
|---------|------------------------------------------------------------------------------------------------------------------------------------------|
| **FR-001**  | The `gridapi` server MUST be able to run in a "no-auth" mode when no OIDC or External IdP environment variables are provided.             |
| **FR-002**  | The webapp MUST detect the "no-auth" mode from the `gridapi`'s `/auth/config` endpoint and bypass all login UI and authentication flows. |
| **FR-003**  | A dedicated E2E test suite MUST exist to validate the "no-auth" scenario from the browser to the backend.                                |
| **FR-004**  | The E2E test setup MUST programmatically seed test data, including states and dependencies, using the `gridctl` CLI tool.               |
| **FR-005**  | The E2E test MUST verify that the seeded states and dependencies are correctly rendered in the webapp's graph view.                    |

## 4. Out of Scope

- Testing authenticated flows (covered by `auth-flow.spec.ts`).
- Testing `gridctl` functionality beyond what is needed to seed data for the webapp test.
