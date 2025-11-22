# Implementation Plan: CI/CD Workflows

**Branch**: `008-cicd-workflows` | **Date**: 2025-11-17 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/008-cicd-workflows/spec.md`

**Note**: This template is filled in by the `/speckit.plan` command. See `.specify/templates/commands/plan.md` for the execution workflow.

## Summary

Implement automated CI/CD workflows using GitHub Actions, release-please for version management, and goreleaser for multi-platform builds. The system will automatically test all PRs (unit, integration, database migrations, protobuf validation), enforce conventional commit standards, and publish release artifacts (binaries, webapp bundle, npm package) when Release PRs are merged. This enables zero-manual-step releases while maintaining code quality gates.

## Technical Context

**Language/Version**: YAML (GitHub Actions workflows), Go 1.24+ (existing project), Node.js 20+ (pnpm workspaces)
**Primary Dependencies**:
  - release-please (version management, changelog generation)
  - goreleaser (multi-platform binary builds)
  - GitHub Actions (CI/CD platform)
  - buf (protobuf linting and breaking change detection)
  - PostgreSQL 17 service containers (integration tests)
**Storage**: N/A (CI/CD infrastructure only)
**Testing**: Existing test suites via make targets (test-integration, test-integration-mode2, test-unit, test-unit-db)
**Target Platform**: GitHub-hosted runners (ubuntu-24.04 primary, matrix for integration test modes)
**Project Type**: Monorepo (Go workspace + pnpm workspaces) with CI/CD infrastructure
**Performance Goals**:
  - PR test feedback within 10 minutes
  - Release builds complete within 20 minutes
  - Cache hit rate >60% for Go modules and pnpm dependencies
**Constraints**:
  - No Docker image publishing (out of scope for initial implementation)
  - No test coverage metrics initially (may add later)
  - Keycloak mode1 tests deferred to manual/nightly runs
  - Linux-only for gridapi (linux/amd64, linux/arm64)
  - Multi-platform for gridctl (linux + darwin for amd64/arm64)
**Scale/Scope**:
  - workflows: PR tests, release-please (release pr + merge), goreleaser + npm publish, conventional commit validation
  - release artifacts (gridapi binaries (2), gridctl binaries(4), webapp bundle (1), npm package (1) @tcons/grid)
  - integration test modes in jobs (plain, mode2)

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

### Applicable Principles

**Principle III: Dependency Flow Discipline** ✅ PASS
- CI/CD workflows do not create new code dependencies
- Workflows consume existing build targets (make commands, pnpm scripts)
- No circular dependencies introduced

**Principle V: Test Strategy** ✅ PASS
- Workflows execute existing test suites (make test-integration, test-integration-mode2, test-unit, test-unit-db)
- Integration tests validate cross-module interactions as required
- Protobuf schema validation via buf lint/breaking preserved

**Principle VI: Versioning & Releases** ⚠️ EXCEPTION REQUIRED
- Constitution requires: "Modules MUST version independently"
- Proposal uses: Single unified version for entire repo (gridapi, gridctl, pkg/sdk, js/sdk, webapp all share v0.x.y)
- Violation: Single release-please manifest at root vs. independent module versioning
- Justification: Simplicity for initial implementation (see Complexity Tracking below)

**Principle VII: Simplicity & Pragmatism** ✅ PASS
- Using GitHub Actions (boring, standard technology)
- Using proven tools (release-please, goreleaser) vs custom scripts
- No premature optimization (defer Docker images, coverage metrics to future iterations)
- Starting minimal (2 test modes, skipping slow Keycloak mode1 initially)

**Development Workflow Compliance** ✅ PASS
- CI enforcement added: dependency graph analysis, proto regeneration checks, test execution
- Branch protection rules documented for requiring PR tests to pass
- Conventional commit validation workflow enforces PR title format

### Gate Result: ⚠️ ONE EXCEPTION REQUIRES JUSTIFICATION

**Single violation**: Principle VI (Versioning & Releases) - unified versioning instead of independent module versions.

This exception is documented in Complexity Tracking below and aligns with Principle VII (Simplicity & Pragmatism). The violation is temporary and can be migrated to independent versioning in a future iteration once the need for independent releases is demonstrated.

## Project Structure

### Documentation (this feature)

```
specs/008-cicd-workflows/
├── plan.md              # This file (/speckit.plan command output)
├── research.md          # Phase 0 output (completed)
├── research/            # Detailed research (completed)
│   ├── release-please.md
│   ├── goreleaser.md
│   └── github-actions.md
├── proposal.md          # User-provided initial design (reference)
└── tasks.md             # Phase 2 output (/speckit.tasks command - NOT created by /speckit.plan)
```

**Phase 1 Deliverables (data-model.md, contracts/, quickstart.md): N/A**

Rationale for skipping Phase 1 standard deliverables:
- **data-model.md**: CI/CD workflows don't have data models - they're infrastructure configuration
- **contracts/**: No API contracts - workflows are declarative YAML, self-documenting
- **quickstart.md**: Workflows are automatically triggered - no user-facing "usage" guide needed

Configuration examples are comprehensively documented in research/ files.

### Source Code (repository root)

```
.github/
├── workflows/
│   ├── pr-tests.yml              # NEW: PR testing workflow (unit, integration, buf, linting)
│   ├── release-please.yml        # NEW: Version management and changelog
│   ├── release-build.yml         # NEW: goreleaser + webapp bundle on tag
│   ├── release-npm.yml           # NEW: Publish @tcons/grid to npm
│   └── pr-title-check.yml        # NEW: Conventional commit validation
├── dependabot.yml                # NEW: Grouped dependency updates
└── release-please-config.json    # NEW: release-please configuration

.goreleaser.yml                   # NEW: Multi-platform build configuration

.release-please-manifest.json     # NEW: Version tracking for release-please

CHANGELOG.md                      # NEW: Auto-generated release notes (created by release-please)

# Existing structure (no changes to source code)
cmd/gridapi/                      # Existing: API server
cmd/gridctl/                      # Existing: CLI client
pkg/sdk/                          # Existing: Go SDK
js/sdk/                           # Existing: Node.js SDK (@tcons/grid)
webapp/                           # Existing: Web application
proto/                            # Existing: Protobuf definitions
api/                              # Existing: Generated Connect RPC code
tests/                            # Existing: Integration tests
```

**Structure Decision**: CI/CD infrastructure only. All workflows live in `.github/workflows/` per GitHub Actions convention. Configuration files (`.goreleaser.yml`, release-please configs) live at repository root. No changes to existing source code directories - workflows consume existing build targets and test suites.

## Complexity Tracking

*Fill ONLY if Constitution Check has violations that must be justified*

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| **Principle VI**: Unified versioning (single v0.x.y for all modules) instead of independent module versions | Dramatically simplifies initial CI/CD setup: single release-please workflow, single changelog, single Release PR. All artifacts (binaries, npm package, webapp) share the same version tag. Eliminates coordination complexity for determining which modules changed and need releases. | Independent versioning requires: (1) Multiple release-please configs tracking each module separately, (2) Complex changelog aggregation across modules, (3) Coordination logic to determine which modules have changes, (4) Multiple Release PRs or complex monorepo plugin configuration, (5) Users tracking different version numbers for cli vs sdk vs webapp. The pain of unified versioning (unnecessary bumps when only one module changes) has not yet been demonstrated, per Principle VII's "add complexity only when pain is demonstrated" rule. |

**Migration Path**: Once independent releases become necessary (e.g., CLI needs v2.0.0 breaking change but SDK stays v1.x), migrate to release-please's monorepo mode with per-package manifests. The infrastructure built here (workflows, goreleaser config) will remain 90% reusable.

**Acceptance Criteria for Exception**: This exception aligns with Principle VII (Simplicity & Pragmatism) which explicitly states "start minimal; add complexity only when pain is demonstrated." We will revisit this decision after 6 months or when first evidence emerges that unified versioning is blocking legitimate independent releases.
