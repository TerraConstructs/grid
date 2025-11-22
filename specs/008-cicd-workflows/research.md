# Research: CI/CD Workflows

**Feature**: 008-cicd-workflows
**Date**: 2025-11-17
**Research Phase**: Phase 0

## Executive Summary

Comprehensive research completed for implementing CI/CD workflows using GitHub Actions, release-please, and goreleaser. All tools are production-ready with 2025 best practices. Key decisions prioritize simplicity, dev/CI parity, and transparency over premature optimization.

## High-Level Conclusions

### 1. Version Management: release-please

**Decision**: Unified versioning (single v0.x.y for all artifacts)

**Rationale**:
- Dramatically simpler than independent module versioning
- Single Release PR, single changelog, single version number
- Aligns with Principle VII (Simplicity & Pragmatism)
- Violates Constitution Principle VI but justified in Complexity Tracking
- Migration path exists when pain is demonstrated

**Latest Version**: `googleapis/release-please-action@v4`

**See**: [research/release-please.md](research/release-please.md) for detailed configuration and workflow examples

---

### 2. Multi-Platform Builds: goreleaser

**Decision**: goreleaser for Go binaries + webapp bundle

**Rationale**:
- Industry standard for Go releases
- Native cross-compilation (linux/darwin, amd64/arm64)
- Flexible artifact handling (binaries + custom archives)
- Automatic GitHub Release uploads

**Build Targets**:
- gridapi: linux/amd64, linux/arm64 (2 artifacts)
- gridctl: linux + darwin for amd64/arm64 (4 artifacts)
- webapp: .tar.gz bundle (1 artifact)

**Latest Version**: `goreleaser/goreleaser-action@v6` (goreleaser v2.12.7)

**See**: [research/goreleaser.md](research/goreleaser.md) for complete .goreleaser.yaml configuration

---

### 3. CI/CD Platform: GitHub Actions

**Decision**: GitHub Actions with docker-compose for dev/CI parity

**Rationale**:
- **Dev/CI Parity**: Use existing docker-compose.yml (same as local dev)
- **Maintainability**: Single source of truth for service configuration
- **Troubleshooting**: CI issues reproduce locally with same docker-compose commands
- **Simplicity**: No duplicated service config in workflow YAML

**Runner**: `ubuntu-24.04` (explicit version for reproducibility)

**Latest Action Versions**:
- actions/checkout@v5
- actions/setup-go@v6 (built-in caching)
- actions/setup-node@v6 (built-in pnpm caching)
- pnpm/action-setup@v4
- actions/cache@v4 (new backend, Feb 2025)

**See**: [research/github-actions.md](research/github-actions.md) for workflow patterns and best practices

---

## Key Considerations

### 1. Matrix vs Separate Jobs

**Decision**: Use separate jobs for Mode1 vs Mode2 integration tests

**Rationale**:
- Mode1 and Mode2 have significantly different setup (Keycloak vs internal IdP)
- Different environment variables required (see Makefile)
- Copy-pasted jobs are clearer than matrix with conditional logic
- Easier to maintain when tests diverge over time
- Matches Makefile structure (separate targets)

**Benefits**:
- Self-contained, readable jobs
- Easy to add mode-specific setup
- Clearer error messages (job name shows which mode failed)

---

### 2. Docker Compose in CI

**Decision**: Use `docker compose` commands directly in workflows

**Rationale**:
- Grid already uses docker-compose extensively for local dev
- initdb/01-init-keycloak-db.sql runs automatically (Postgres init script)
- Same environment variables, ports, networking as local dev
- Makefile targets work identically in CI and locally

**Pattern**:
```yaml
- name: Start services
  run: docker compose up -d postgres keycloak

- name: Wait for health
  run: |
    until docker compose ps postgres | grep -q "healthy"; do
      sleep 2
    done

- name: Run tests
  run: make test-integration-mode1
```

**Benefits Over GitHub Service Containers**:
- ✅ Dev/CI parity (identical setup)
- ✅ Single source of truth (docker-compose.yml)
- ✅ Easier troubleshooting (reproduce CI issues locally)
- ✅ Automatic init scripts and volume handling

---

### 3. PR Title Validation

**Decision**: Use `actions/github-script@v8` (official action)

**Rationale**:
- ✅ Official GitHub action (no supply chain risk)
- ✅ Transparent (regex visible in workflow file)
- ✅ Simple (no configuration files needed)
- ✅ Flexible (easy to customize regex or error messages)
- ❌ Avoid third-party actions (`amannn/action-semantic-pull-request`)

**Pattern**:
```yaml
- uses: actions/github-script@v8
  with:
    script: |
      const validator = /^(chore|feat|fix|revert|docs|style|ci|refactor|test)(\((grid-[a-f0-9]+|[a-z-]+)\))?(!)?: (.)+$/
      const title = context.payload.pull_request.title
      const is_valid = validator.test(title)
      if (!is_valid) {
        core.setFailed(`Your PR title doesn't adhere to conventional commits syntax.`)
      }
```

---

### 4. Version Injection

**Setup Required**:
1. Create `cmd/gridapi/version.go` and `cmd/gridctl/version.go` with annotations
2. release-please updates version const via `extra-files` config
3. goreleaser injects commit/date via `-X` ldflags
4. **Implementation Detail**: Wire version variables to Cobra `version` commands (deferred to task phase)

**File Structure**:
```go
// cmd/gridapi/version.go
package main

// x-release-please-start-version
const Version = "0.1.0"
// x-release-please-end

var (
    version   = Version  // Overridden by goreleaser
    commit    = "none"
    date      = "unknown"
    builtBy   = "unknown"
)
```

---

## Performance Expectations

### PR Test Feedback Time

**Goal**: <10 minutes

**Breakdown** (with caching):
- Unit tests: ~1m 20s
- Integration (mode2): ~2m 45s
- Integration (mode1): ~3m 30s (Keycloak slower)
- Frontend tests: ~1m 30s
- Buf lint: ~20s
- Build verification: ~1m

**Total** (parallel jobs): ~4-5 minutes (cached) ✅

### Release Build Time

**Goal**: <20 minutes

**Breakdown**:
- Webapp build: ~2m
- goreleaser (6 platform builds): ~3-5m
- npm publish: ~1m

**Total**: ~6-8 minutes ✅

### Cache Hit Rate

**Goal**: >60%

**Strategy**:
- Go modules: Cached by `setup-go@v6` (keyed on go.sum)
- pnpm store: Cached by `setup-node@v6` (keyed on pnpm-lock.yaml)
- Docker layers: Handled by docker-compose (not explicitly cached)

**Expected**: 70-80% cache hit rate in normal development ✅

---

## Workflow Overview

### 1. PR Testing Workflow

**Trigger**: `on: pull_request`

**Jobs** (parallel):
- Unit tests (no DB)
- Integration tests - Mode2 (docker-compose: postgres)
- Integration tests - Mode1 (docker-compose: postgres + keycloak)
- Frontend tests (webapp - pnpm)
- **JS SDK tests** (js/sdk - pnpm)
- Buf lint (protobuf validation)
- PR title validation (conventional commits)

**Outcome**: Green checkmarks = PR ready to merge

#### JS SDK Test Job Details

```yaml
js-sdk-tests:
  runs-on: ubuntu-24.04
  defaults:
    run:
      working-directory: ./js/sdk

  steps:
    - uses: actions/checkout@v5

    - uses: pnpm/action-setup@v4
      with:
        version: 10

    - uses: actions/setup-node@v6
      with:
        node-version: 20
        cache: 'pnpm'
        cache-dependency-path: 'js/sdk/pnpm-lock.yaml'

    - run: pnpm install --frozen-lockfile
    - run: pnpm run type-check  # TypeScript compilation
    - run: pnpm run lint         # ESLint
    - run: pnpm run test         # Vitest (if tests exist)
    - run: pnpm run build        # Build dist/
```

**What's Tested**:
- TypeScript type correctness (generated from protobuf)
- ESLint code quality
- Unit tests (if any) via Vitest
- Build succeeds (dist/ generation)

**Performance**: ~1-2 minutes (cached)

---

### 2. Release Workflow

**Trigger**: `on: push: branches: [main]`

**Steps**:
1. release-please watches commits
2. Opens Release PR when releasable changes detected
3. Maintainer reviews and merges Release PR
4. release-please creates Git tag (v0.x.y) and GitHub Release
5. Tag push triggers goreleaser workflow
6. goreleaser builds binaries + uploads to GitHub Release
7. **Separate workflow publishes @tcons/grid to npm**

**Outcome**: Fully automated release with zero manual steps

#### npm Publish Workflow Details

**Trigger**: `on: push: tags: ['v*']` (triggered by release-please tag creation)

```yaml
name: Publish npm Package

on:
  push:
    tags:
      - 'v*'

permissions:
  contents: read
  id-token: write  # Required for provenance

jobs:
  publish-npm:
    runs-on: ubuntu-24.04
    defaults:
      run:
        working-directory: ./js/sdk

    steps:
      - uses: actions/checkout@v5

      - uses: pnpm/action-setup@v4
        with:
          version: 10

      - uses: actions/setup-node@v6
        with:
          node-version: 20
          cache: 'pnpm'
          cache-dependency-path: 'js/sdk/pnpm-lock.yaml'
          registry-url: 'https://registry.npmjs.org'

      - run: pnpm install --frozen-lockfile

      - run: pnpm run build

      # Optional: Verify package contents
      - run: pnpm pack --dry-run

      - name: Publish to npm
        run: pnpm publish --access public --provenance --no-git-checks
        env:
          NODE_AUTH_TOKEN: ${{ secrets.NPM_TOKEN }}
```

**Key Configuration**:
- `--access public`: Package is publicly accessible
- `--provenance`: Adds supply chain attestation (GitHub signatures)
- `--no-git-checks`: Skip git cleanliness check (tag already created)
- `NODE_AUTH_TOKEN`: npm authentication (stored in repository secrets)

**Version Synchronization**:
- `js/sdk/package.json` version updated by release-please via `extra-files` config
- Same version as Git tag (e.g., v0.4.0 → package.json: "0.4.0")

**Changelog in Package**:
Option 1: Copy root CHANGELOG.md before publishing:
```yaml
- name: Copy changelog
  run: cp ../../CHANGELOG.md ./CHANGELOG.md

- name: Publish
  run: pnpm publish --access public
```

Option 2: Reference root changelog in package.json:
```json
{
  "repository": {
    "url": "https://github.com/TerraConstructs/grid",
    "directory": "js/sdk"
  },
  "bugs": "https://github.com/TerraConstructs/grid/issues",
  "homepage": "https://github.com/TerraConstructs/grid/tree/main/js/sdk#readme"
}
```

**Grid Decision**: Use Option 2 (repository metadata) to avoid changelog duplication. Users can view changelog on GitHub.

**Secrets Required**:
- `NPM_TOKEN`: npm publish token (created at https://www.npmjs.com/settings/<user>/tokens)
  - Token type: "Automation" (for CI/CD)
  - Scope: "Read and write" for @tcons/grid package

**Performance**: ~1-2 minutes

---

### 3. Dependency Updates Workflow

**Trigger**: Dependabot weekly schedule

**Strategy**:
- Grouped updates (Go modules, npm packages, GitHub Actions)
- Separate PRs for major vs minor/patch
- Auto-merge when tests pass (future enhancement)

**Outcome**: Automated security updates and dependency maintenance

---

## Technology Stack Summary

| Component | Tool | Version | Purpose |
|-----------|------|---------|---------|
| Version Management | release-please | v4 | Automated versioning and changelog |
| Build Tool | goreleaser | v2.12.7 | Multi-platform Go binary builds |
| CI Platform | GitHub Actions | N/A | Workflow orchestration |
| Service Orchestration | docker-compose | pre-installed | Dev/CI parity for services |
| Dep Updates | Dependabot | Native | Grouped dependency PRs |

**No new runtime dependencies** - all tools are CI/CD infrastructure only.

---

## Prior Work

**Beads Query Results**: No existing CI/CD infrastructure found in issue tracker. This is greenfield implementation.

**Related Features**: Feature 007-webapp-auth established integration test patterns (mode1, mode2) that CI must support.

---

## Open Questions / Future Work

1. **Test coverage metrics**: Deferred. Would require coverage collection, storage, and diff reporting.
2. **Docker image publishing**: Out of scope initially. goreleaser supports Docker builds if needed later.
3. **Keycloak mode1 in CI**: Currently deferred to manual/nightly workflow to avoid slowing every PR.
4. **Migration to independent versioning**: Documented migration path exists when pain is demonstrated.
5. **Playwright E2E tests**: Not yet implemented. When added, run in separate workflow (slow, potentially flaky).

---

## Detailed Research

For complete configuration examples, workflow patterns, and troubleshooting guides:

1. **[release-please.md](research/release-please.md)** - Version management, changelog generation, configuration
2. **[goreleaser.md](research/goreleaser.md)** - Multi-platform builds, artifact packaging, GitHub Release integration
3. **[github-actions.md](research/github-actions.md)** - Workflow patterns, caching, docker-compose usage, PR validation

---

## Future Migration to Independent Versioning

While the initial unified versioning strategy is simpler, migrating to independent, per-module versioning is a foreseeable future task. The overall effort is **Medium**, with the complexity concentrated in refactoring the release workflows.

The migration would involve the following key changes:

### 1. **`release-please` Configuration (Low Effort)**

- **Configuration (`.github/release-please-config.json`)**: The config would be restructured to define each module (`pkg/sdk`, `js/sdk`, `cmd/gridctl`, etc.) as a separate package with its own `release-type` and `changelog-path`.
- **Manifest (`.release-please-manifest.json`)**: The manifest would change from a single root version to a map of component paths to their individual versions.
- **Pull Requests**: `separate-pull-requests` would be set to `true` to generate a distinct Release PR for each component that has changes.

### 2. **`goreleaser` Configuration (Medium Effort)**

- **Multiple Configurations**: The single `.goreleaser.yml` would likely be split into multiple files (e.g., `.goreleaser.gridctl.yml`, `.goreleaser.gridapi.yml`), one for each Go binary. This is because a single configuration assumes a single version.
- **Targeted Builds**: The release workflow would need to call `goreleaser` with the appropriate `--config` flag to ensure only the correct component is built and released.

### 3. **GitHub Actions Workflows (Medium to High Effort)**

- **Triggering Logic**: The `release-build.yml` and `release-npm.yml` workflows would need significant changes. Instead of triggering on a single tag format (`v*`), they would need to handle multiple, component-specific tag formats (e.g., `gridctl/v*`, `js/sdk/v*`).
- **Workflow Dispatching**: The workflows would need to parse the incoming tag to identify which component to build. This would likely involve a dispatcher job that calls the appropriate build job (e.g., a tag `gridctl/v1.2.0` would trigger the `goreleaser` job with the `gridctl` config).
- **Complexity Increase**: The simplicity of a single, linear release pipeline would be replaced by a more complex, branching logic that requires careful management.

---

## Next Steps

This research phase is complete. Proceed to:
- **Phase 1**: Generate data-model.md, contracts/, quickstart.md
- **Phase 2**: Generate tasks.md with dependency-ordered implementation tasks

**Note**: Constitution violation (unified versioning) is documented in plan.md Complexity Tracking and accepted as temporary exception per Principle VII.
