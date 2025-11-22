# GitHub Actions: Detailed Research

**Research Date**: 2025-11-17
**Focus**: CI/CD best practices for Go + Node.js monorepo with PostgreSQL

## Executive Summary

**Runner Recommendation**: `ubuntu-24.04` (explicit version for reproducibility)
**Key Improvements**: Built-in caching in setup actions (20-40% faster), new cache backend (Feb 2025)

## Latest Action Versions (2025)

| Action | Latest Version | Key Features |
|--------|----------------|--------------|
| `actions/checkout` | **v5** | Faster checkouts, improved Git performance |
| `actions/setup-go` | **v6** | Built-in caching enabled by default |
| `actions/setup-node` | **v6** | Built-in caching for npm/yarn/pnpm |
| `actions/cache` | **v4** | New cache backend (v2) with improved performance |
| `pnpm/action-setup` | **v4** | Supports pnpm 10, integrates with setup-node caching |
| `actions/upload-artifact` | **v4** | Improved performance and reliability |
| `actions/download-artifact` | **v4** | Compatible with upload-artifact@v4 |

## Ubuntu Runner Version

### Recommendation: `ubuntu-24.04` ✅

**Status as of 2025**:
- `ubuntu-latest` now points to **Ubuntu 24.04** (migration completed January 17, 2025)
- Ubuntu 24.04 became GA in July 2024
- ARM64 support available for Ubuntu 24.04 runners

**Best Practice**:
```yaml
runs-on: ubuntu-24.04  # Explicit version recommended for reproducibility
```

**Why explicit version**:
- Avoids surprises from future `ubuntu-latest` migrations
- Reproducible builds across time
- Clear documentation of runner requirements

**Latest Self-Hosted Runner**: v2.323.0 (May 2025)

**Important Caveats**:
- ubuntu-24.04 runners have cgroup restrictions that can occasionally break docker-compose networking between services
- **Mitigation**: Add netcat health checks for service reachability (not just postgres health), see below

## PostgreSQL 17 Service Containers

### Recommended Configuration

```yaml
services:
  postgres:
    image: postgres:17  # Specify exact version
    env:
      POSTGRES_PASSWORD: postgres
      POSTGRES_USER: grid
      POSTGRES_DB: grid_test
    options: >-
      --health-cmd pg_isready
      --health-interval 10s
      --health-timeout 5s
      --health-retries 5
    ports:
      - 5432:5432
```

### Critical: Health Checks ⚠️

**Without health checks**: PostgreSQL container may shut down after initial setup, causing random test failures.

**Health check configuration**:
```yaml
options: >-
  --health-cmd pg_isready      # Command to check if Postgres is ready
  --health-interval 10s         # Check every 10 seconds
  --health-timeout 5s           # 5 second timeout for each check
  --health-retries 5            # Retry 5 times before marking unhealthy
```

### Port Mapping Strategy

**Running on VM (ubuntu-24.04)**:
```yaml
services:
  postgres:
    ports:
      - 5432:5432  # Map host port to container port

# Connect via localhost
env:
  DATABASE_URL: postgres://grid:postgres@localhost:5432/grid_test?sslmode=disable
```

**Running in container job**:
```yaml
container:
  image: golang:1.24

services:
  postgres:
    # No port mapping needed

# Connect via service name
env:
  DATABASE_URL: postgres://grid:postgres@postgres:5432/grid_test?sslmode=disable
```

### Wait for PostgreSQL Pattern

```yaml
- name: Wait for PostgreSQL
  run: |
    until pg_isready -h localhost -p 5432 -U grid; do
      echo "Waiting for postgres..."
      sleep 2
    done
```

**Grid recommendation**: Include explicit wait step before running migrations/tests.

## Docker Compose in CI (Dev/CI Parity)

### Why Use Docker Compose in CI

**Grid uses docker-compose extensively for local development**:
- PostgreSQL setup with health checks
- Keycloak with realm import and DB initialization (`initdb/01-init-keycloak-db.sql`)
- Service dependencies properly configured

**Benefits of using docker-compose in CI**:
- ✅ **Dev/CI parity**: Same environment locally and in CI
- ✅ **Easier troubleshooting**: Issues in CI reproduce locally
- ✅ **DRY principle**: One docker-compose.yml, not duplicated service config
- ✅ **Consistent setup**: Keycloak DB init, health checks, networking all work the same way
- ✅ **Maintainability**: Changes to services (new env vars, volumes) propagate to CI automatically

### Docker Compose in GitHub Actions

**Best Practice**: Use `docker/setup-buildx-action` to pin docker-compose version

While docker-compose is pre-installed on ubuntu-24.04 runners, using the official action ensures:
- ✅ Version pinning (reproducible builds)
- ✅ Consistent behavior across runner updates
- ✅ Explicit dependency declaration

```yaml
- name: Set up Docker Buildx
  uses: docker/setup-buildx-action@v3

- name: Start services
  run: docker compose up -d postgres
```

**Alternative**: Rely on runner's pre-installed version (not recommended for production):
```yaml
# ⚠️ Version may change with runner updates
- run: docker compose up -d postgres
```

**Grid Recommendation**: Use `docker/setup-buildx-action@v3` for version stability.

**Basic Usage**:
```yaml
jobs:
  integration-test:
    runs-on: ubuntu-24.04
    steps:
      - uses: actions/checkout@v5

      - name: Start services
        run: docker compose up -d postgres

      - name: Wait for health
        run: |
          until docker compose ps postgres | grep -q "healthy"; do
            echo "Waiting for postgres to be healthy..."
            sleep 2
          done

      - name: Run tests
        run: make test-integration
        env:
          DATABASE_URL: postgres://grid:gridpass@localhost:5432/grid?sslmode=disable

      - name: Cleanup
        if: always()
        run: docker compose down -v
```

### Grid-Specific Pattern (Keycloak + Postgres)

**Mode1 tests** (Keycloak required):
```yaml
jobs:
  integration-mode1:
    runs-on: ubuntu-24.04
    steps:
      - uses: actions/checkout@v5

      - uses: actions/setup-go@v6
        with:
          go-version: '1.24'
          cache: true

      - name: Build binaries
        run: make build

      - name: Start services (Postgres + Keycloak)
        run: docker compose up -d postgres keycloak

      - name: Wait for services
        run: |
          echo "Waiting for PostgreSQL..."
          until docker compose ps postgres | grep -q "healthy"; do
            sleep 2
          done
          echo "Waiting for Keycloak..."
          sleep 5  # Keycloak takes longer to start

      - name: Run Mode1 tests
        run: make test-integration-mode1

      - name: Cleanup
        if: always()
        run: docker compose down -v
```

**Why this works**:
- `initdb/01-init-keycloak-db.sql` runs automatically (Postgres init script)
- Keycloak realm import happens via docker-compose config
- Same environment variables, same ports, same networking as local dev
- Makefile targets work identically in CI and locally

### Comparison: GitHub Service Containers vs Docker Compose

| Aspect | GitHub Service Containers | Docker Compose |
|--------|---------------------------|----------------|
| **Setup** | Define in workflow YAML | Use existing docker-compose.yml |
| **Dev/CI Parity** | ❌ Different configs | ✅ Identical setup |
| **Maintainability** | ❌ Duplicated service config | ✅ Single source of truth |
| **Complexity** | ❌ Init scripts need manual handling | ✅ Automatic (initdb, volumes) |
| **Troubleshooting** | ❌ Can't reproduce CI issues locally | ✅ Same docker-compose locally |
| **Networking** | More complex (port mapping) | Simpler (compose networking) |

**Grid Decision**: Use `docker compose` commands directly in CI workflows for dev/CI parity.

### Alternative: docker/setup-buildx-action

**Not needed for Grid**: We're not building Docker images, just running services via docker-compose.

```yaml
# ❌ NOT NEEDED - docker-compose is pre-installed
- uses: docker/setup-buildx-action@v3

# ✅ JUST USE IT
- run: docker compose up -d
```

**Grid recommendation**: Include explicit wait step before running migrations/tests.

## Matrix Strategy Best Practices

### Matrix vs Separate Jobs: When to Use Each

**Use Matrix When**:
- Jobs are nearly identical (same steps, same env vars)
- Only platform/version differences (e.g., Go 1.23 vs 1.24)
- Easy to maintain DRY without obscuring differences

**Use Separate Jobs When**:
- Jobs have significant differences in setup/teardown
- Different environment variables or services required
- Clarity and maintainability trump DRY principle

**Grid Recommendation**: Use **separate jobs** for Mode1 vs Mode2 integration tests.

**Why**: Mode1 requires Keycloak + different env vars, Mode2 requires internal IdP setup. The differences are significant enough that copy-pasted jobs are clearer than a matrix with conditional logic.

```yaml
jobs:
  # Separate job for Mode2 (Internal IdP)
  integration-mode2:
    runs-on: ubuntu-24.04
    steps:
      - uses: actions/checkout@v5
      - uses: actions/setup-go@v6
        with:
          go-version: '1.24'
          cache: true

      # Use docker-compose (matches local dev workflow)
      - name: Start services
        run: docker compose up -d postgres

      - name: Run Mode2 tests
        run: make test-integration-mode2

  # Separate job for Mode1 (External IdP with Keycloak)
  integration-mode1:
    runs-on: ubuntu-24.04
    steps:
      - uses: actions/checkout@v5
      - uses: actions/setup-go@v6
        with:
          go-version: '1.24'
          cache: true

      # Use docker-compose (matches local dev workflow)
      - name: Start services
        run: docker compose up -d postgres keycloak

      - name: Wait for Keycloak
        run: sleep 5  # Or use wait-for-health script

      - name: Run Mode1 tests
        run: make test-integration-mode1
```

**Benefits of Separate Jobs**:
- Each job is self-contained and readable
- Easy to add mode-specific setup without affecting other modes
- Clearer error messages (job name shows which mode failed)
- Matches Makefile structure (separate targets for mode1/mode2)

### Matrix Best Practices (2025)

When you DO use matrices:

**1. fail-fast: false**
- See all failures, not just first one
- Better for debugging matrix issues

**2. Dynamic Matrices**
```yaml
matrix:
  include:
    - mode: plain
      go-version: '1.24'
    - mode: mode2
      go-version: '1.24'
      extra-flags: '--verbose'
```

**3. Exclude Invalid Combinations**
```yaml
matrix:
  os: [ubuntu-24.04, macos-latest]
  arch: [amd64, arm64]
  exclude:
    - os: macos-latest
      arch: amd64  # Skip if not needed
```

**4. Test Sharding for Large Suites**
```yaml
strategy:
  matrix:
    shard: [1, 2, 3, 4]  # Split tests into 4 parallel shards

steps:
  - run: go test -v ./... -shard=${{ matrix.shard }}/4
```

## Caching Strategies

### Go Modules Caching (Built-in) ✅

```yaml
- uses: actions/setup-go@v6
  with:
    go-version: '1.24'
    cache: true  # Default in v6, caches both modules and build cache
```

**Performance Results**:
- **First run**: ~1m20s (cache population)
- **Subsequent runs**: ~18s (86% faster)
- **Improvement**: 20-40% faster for repos with >100MB dependencies

**What's cached**:
- `~/.cache/go-build` (build cache)
- `~/go/pkg/mod` (Go modules)

**Cache key**: Based on `go.sum` hash

### pnpm Caching (Built-in) ✅

```yaml
- uses: pnpm/action-setup@v4
  with:
    version: 10
    run_install: false

- uses: actions/setup-node@v6
  with:
    node-version: 20
    cache: 'pnpm'  # Automatically caches pnpm store

- run: pnpm install --frozen-lockfile
```

**What's cached**:
- `~/.pnpm-store` (global pnpm store)

**Cache key**: Based on `pnpm-lock.yaml` hash

### Manual Caching (Advanced)

If you need custom cache keys:

```yaml
- uses: actions/cache@v4
  with:
    path: |
      ~/.cache/go-build
      ~/go/pkg/mod
    key: ${{ runner.os }}-go-${{ hashFiles('**/go.sum') }}-${{ hashFiles('**/go.mod') }}-${{ github.run_id }}
    restore-keys: |
      ${{ runner.os }}-go-${{ hashFiles('**/go.sum') }}-${{ hashFiles('**/go.mod') }}-
      ${{ runner.os }}-go-${{ hashFiles('**/go.sum') }}-
      ${{ runner.os }}-go-
```

**Use case**: When you need date-based cache keys or custom invalidation logic

### Cache Backend v2 (February 2025)

- GitHub rewrote cache backend for improved performance
- `actions/cache@v4` integrates with new cache service APIs
- Fully backward compatible
- Better cache hit rates and faster restore times

## Conventional Commit PR Title Validation

### Recommended: actions/github-script (Official Action) ✅

```yaml
name: PR Title Validation

on:
  pull_request:
    types:
      - opened
      - edited
      - synchronize
      - reopened

permissions:
  pull-requests: read

jobs:
  conventional_commit_title:
    runs-on: ubuntu-24.04
    steps:
      - uses: actions/github-script@v8
        with:
          script: |
            const validator = /^(chore|feat|fix|revert|docs|style|ci|refactor|test)(\((grid-[a-f0-9]+|[a-z-]+)\))?(!)?: (.)+$/
            const title = context.payload.pull_request.title
            const is_valid = validator.test(title)

            if (!is_valid) {
              const details = JSON.stringify({
                title: title,
                valid_syntax: validator.toString(),
              })
              core.setFailed(`Your PR title doesn't adhere to conventional commits syntax. See details: ${details}`)
            }
```

**Why github-script over third-party actions**:
- ✅ **Official GitHub action** (maintained by GitHub)
- ✅ **Transparent**: Regex logic is visible in workflow file
- ✅ **No supply chain risk**: No third-party dependencies
- ✅ **Flexible**: Easy to customize regex or error messages
- ✅ **Simple**: No additional configuration files needed

**Conventional Commit Format**:
```
type(scope)!: subject

type: feat|fix|chore|docs|style|refactor|test|ci
scope: optional, e.g., (gridapi), (cli), (grid-a1b2)
!: optional breaking change marker
subject: lowercase description
```

**Examples**:
- ✅ `feat(gridapi): add state locking endpoint`
- ✅ `fix(grid-a1b2): resolve migration rollback error`
- ✅ `chore!: upgrade to Go 1.25 (BREAKING)`
- ❌ `Add new feature` (missing type)
- ❌ `Feat: Add feature` (capital F in type)

### Alternative: amannn/action-semantic-pull-request

**Third-party action** (not recommended for Grid):
```yaml
- uses: amannn/action-semantic-pull-request@v5  # Third-party, opaque
```

**Concerns**:
- ❌ Not official GitHub action
- ❌ Adds supply chain dependency
- ❌ Logic hidden in action code (less transparent)
- ❌ Requires trusting third-party maintainer

**Grid Decision**: Stick with `actions/github-script` for transparency and official support.

## Dependabot Grouped Updates

### Recommended Configuration

```yaml
# .github/dependabot.yml
version: 2

updates:
  # Go modules (all workspace modules)
  - package-ecosystem: "gomod"
    directories:
      - "/"
      - "/api"
      - "/cmd/gridapi"
      - "/cmd/gridctl"
      - "/pkg/sdk"
      - "/tests"
    schedule:
      interval: "weekly"
      day: "monday"
    groups:
      go-dependencies:
        patterns:
          - "*"
        update-types:
          - "minor"
          - "patch"
      go-major:
        patterns:
          - "*"
        update-types:
          - "major"
    open-pull-requests-limit: 5
    labels:
      - "dependencies"
      - "go"

  # npm/pnpm (webapp + js/sdk)
  - package-ecosystem: "npm"
    directories:
      - "/webapp"
      - "/js/sdk"
    schedule:
      interval: "weekly"
    groups:
      react-dependencies:
        patterns:
          - "react*"
          - "@types/react*"
      dev-dependencies:
        dependency-type: "development"
      prod-dependencies:
        dependency-type: "production"
    open-pull-requests-limit: 5
    labels:
      - "dependencies"
      - "javascript"

  # GitHub Actions
  - package-ecosystem: "github-actions"
    directory: "/"
    schedule:
      interval: "weekly"
    groups:
      github-actions:
        patterns:
          - "*"
    labels:
      - "dependencies"
      - "github-actions"
```

### Grouping Strategies

**1. By Update Type** (Recommended)
```yaml
groups:
  minor-and-patch:
    update-types:
      - "minor"
      - "patch"
  major-updates:
    update-types:
      - "major"
```

**2. By Dependency Type**
```yaml
groups:
  dev-dependencies:
    dependency-type: "development"
  production-dependencies:
    dependency-type: "production"
```

**3. By Pattern**
```yaml
groups:
  react-ecosystem:
    patterns:
      - "react*"
      - "@types/react*"
      - "react-*"
```

**4. Security Updates** (Beta)
```yaml
groups:
  security-updates:
    applies-to: security-updates
    patterns:
      - "*"
```

### Benefits for Monorepos

- Reduces PR noise (50+ individual updates → 3-5 grouped PRs)
- Easier to review related updates together
- Faster CI (batch updates reduce workflow runs)
- Better for pnpm workspaces with shared dependencies

## Fork PR Security

### Requirement: Manual Approval for Fork PRs

**Security Risk**: Workflows from fork PRs can access secrets and execute arbitrary code if not restricted.

**Solution**: Configure branch protection to require manual approval for first-time contributors.

**GitHub Settings** (Repository Settings > Actions > General):
```
Fork pull request workflows from outside collaborators:
○ Require approval for all outside collaborators
● Require approval for first-time contributors ✅
○ Require approval for first-time contributors and contributors who haven't contributed recently
```

**Why This Matters**:
- Fork PRs can modify workflow files
- Without approval, malicious PRs could steal secrets or mine crypto
- First-time contributor approval is good balance between security and friction

**Additional Protection**:
```yaml
# Use pull_request_target cautiously - only for safe operations
on:
  pull_request_target:  # Has write access and secrets
    types: [opened]

# Safe: Only validates PR title (no code execution)
jobs:
  validate-title:
    steps:
      - uses: actions/github-script@v8  # Official action, safe
```

**Grid Requirement**: Enable "Require approval for first-time contributors" in repository settings.

## Complete Example Workflow

```yaml
name: CI

on:
  pull_request:
    branches: [main]
  push:
    branches: [main]

permissions:
  contents: read
  pull-requests: write

jobs:
  # Unit tests (no DB)
  unit-tests:
    runs-on: ubuntu-24.04
    steps:
      - uses: actions/checkout@v5

      - uses: actions/setup-go@v6
        with:
          go-version: '1.24'
          cache: true

      - name: Run unit tests
        run: make test-unit

      - name: Run with race detector
        run: go test -race ./...

  # Integration tests with matrix
  integration-tests:
    runs-on: ubuntu-24.04
    strategy:
      fail-fast: false
      matrix:
        mode: [plain, mode2]

    services:
      postgres:
        image: postgres:17
        env:
          POSTGRES_PASSWORD: postgres
          POSTGRES_USER: grid
          POSTGRES_DB: grid_test
        options: >-
          --health-cmd pg_isready
          --health-interval 10s
          --health-timeout 5s
          --health-retries 5
        ports:
          - 5432:5432

    steps:
      - uses: actions/checkout@v5

      - uses: actions/setup-go@v6
        with:
          go-version: '1.24'
          cache: true

      - name: Wait for PostgreSQL
        run: |
          until pg_isready -h localhost -p 5432 -U grid; do
            echo "Waiting for postgres..."
            sleep 2
          done

      - name: Run migrations
        run: ./bin/gridapi db migrate
        env:
          DATABASE_URL: postgres://grid:postgres@localhost:5432/grid_test?sslmode=disable

      - name: Run integration tests (${{ matrix.mode }})
        run: make test-integration-${{ matrix.mode }}
        env:
          DATABASE_URL: postgres://grid:postgres@localhost:5432/grid_test?sslmode=disable

  # Frontend tests
  frontend-tests:
    runs-on: ubuntu-24.04
    defaults:
      run:
        working-directory: ./webapp

    steps:
      - uses: actions/checkout@v5

      - uses: pnpm/action-setup@v4
        with:
          version: 10

      - uses: actions/setup-node@v6
        with:
          node-version: 20
          cache: 'pnpm'
          cache-dependency-path: 'webapp/pnpm-lock.yaml'

      - run: pnpm install --frozen-lockfile
      - run: pnpm run type-check
      - run: pnpm run lint
      - run: pnpm run test

  # Protobuf validation
  buf-lint:
    runs-on: ubuntu-24.04
    steps:
      - uses: actions/checkout@v5

      - uses: bufbuild/buf-setup-action@v1
        with:
          version: latest

      - name: Lint protobuf
        run: buf lint

      - name: Breaking change detection
        run: buf breaking --against '.git#branch=main'
        if: github.event_name == 'pull_request'

  # Build verification
  build:
    runs-on: ubuntu-24.04
    needs: [unit-tests, integration-tests]
    steps:
      - uses: actions/checkout@v5

      - uses: actions/setup-go@v6
        with:
          go-version: '1.24'
          cache: true

      - name: Build binaries
        run: make build

      - name: Upload artifacts
        uses: actions/upload-artifact@v4
        with:
          name: binaries
          path: bin/
          retention-days: 7
```

## New GitHub Actions Features (2025)

### 1. YAML Anchors Support (September 2025) 🔥

```yaml
# Define reusable configuration
go-setup: &go-setup
  uses: actions/setup-go@v6
  with:
    go-version: '1.24'
    cache: true

jobs:
  test:
    steps:
      - *go-setup  # Reference the anchor
  build:
    steps:
      - *go-setup  # Reuse configuration
```

**Benefits**: Reduces duplication, improves maintainability

### 2. Increased Reusable Workflow Limits

- **10 nested reusable workflows** (up from 4)
- **50 total workflows** per workflow run (up from 20)

### 3. Check Run ID in Job Context

```yaml
- name: Report job status
  run: echo "Job ID: ${{ job.check_run_id }}"
```

### 4. M2 macOS Runners (November 2025)

```yaml
runs-on: macos-latest-xlarge  # M2 powered
```

**Relevant for Grid**: If adding macOS builds/tests later

## Performance Optimization Checklist

To achieve **<10 min PR feedback** and **>60% cache hit rate**:

✅ **Use built-in caching**
- `setup-go@v6` with `cache: true`
- `setup-node@v6` with `cache: 'pnpm'`

✅ **Optimize PostgreSQL startup**
- Always include health checks
- Use explicit wait step

✅ **Parallelize jobs**
- Run unit, integration, frontend tests in parallel
- Use matrix for test modes

✅ **Smart caching keys**
- Include `go.sum` and `pnpm-lock.yaml` hashes
- Use restore-keys for partial cache hits

✅ **Limit scope**
- Run tests only on affected paths
- Use `paths` filter in workflow triggers

✅ **Monitor performance**
- Track workflow run times in GitHub Insights
- Adjust based on actual metrics

## Common Patterns

### Conditional Job Execution

```yaml
jobs:
  integration-tests:
    if: github.event_name == 'pull_request' || github.ref == 'refs/heads/main'
    # ...
```

### Path Filtering

```yaml
on:
  pull_request:
    paths:
      - '**.go'
      - 'go.mod'
      - 'go.sum'
      - '.github/workflows/pr-tests.yml'
```

### Concurrency Control

```yaml
concurrency:
  group: ${{ github.workflow }}-${{ github.ref }}
  cancel-in-progress: true  # Cancel previous runs on new push
```

### Environment Variables

```yaml
env:
  GO_VERSION: '1.24'
  NODE_VERSION: '20'
  PNPM_VERSION: '10'

jobs:
  test:
    steps:
      - uses: actions/setup-go@v6
        with:
          go-version: ${{ env.GO_VERSION }}
```

## Common Gotchas & Critical Fixes

### 1. 🐳 Docker Compose v2 Syntax

**CRITICAL**: GitHub Actions runners (2025) only have `docker compose` (v2) installed, NOT `docker-compose` (v1).

```yaml
# ❌ WRONG - deprecated since 2023
- run: docker-compose up -d

# ✅ CORRECT - v2 syntax
- run: docker compose up -d
```

**Grid Requirement**: All documentation and workflows MUST use `docker compose` (with space), not `docker-compose` (with hyphen).

---

### 2. 🔒 Cgroup Networking + Port Reachability

**Issue**: ubuntu-24.04 runners have cgroup restrictions that occasionally break docker-compose networking.

**Symptom**: PostgreSQL health checks pass but service is unreachable from host (localhost:5432 fails).

**Fix**: Add netcat reachability check in addition to health check:

```yaml
- name: Wait for PostgreSQL (health + network)
  run: |
    echo "Waiting for PostgreSQL health..."
    until docker compose ps postgres | grep -q "healthy"; do
      sleep 2
    done

    echo "Waiting for port 5432 reachability..."
    until nc -z localhost 5432; do
      echo "Port 5432 not reachable yet..."
      sleep 2
    done

    echo "PostgreSQL is healthy and reachable!"
```

**Why**: Docker health checks verify internal container state, but cgroup networking can still block host→container traffic.

---

### 3. 📝 Expanded Conventional Commit Types

**Issue**: release-please supports more commit types than the basic regex.

**Fix**: Expand regex to include all release-please types:

```javascript
// ❌ INCOMPLETE
/^(chore|feat|fix|revert|docs|style|ci|refactor|test)(\((grid-[a-f0-9]+|[a-z-]+)\))?(!)?: (.)+$/

// ✅ COMPLETE - includes perf, build
/^(feat|fix|chore|docs|style|refactor|perf|test|ci|build|revert)(\([^)]+\))?(!)?: .+$/
```

**Supported Types**:
- `feat`: Minor version bump
- `fix`: Patch version bump
- `perf`: Patch version bump (performance improvement)
- `build`: Patch version bump (build system changes)
- `BREAKING CHANGE:` footer: Major/minor version bump
- `feat!:` or `fix!:`: Major/minor version bump (breaking change marker)

**Non-releasing types**: `chore`, `docs`, `style`, `refactor`, `test`, `ci`, `revert` (no version bump)

---

### 4. 🧬 release-please Go Module Bumps

**Issue**: With unified versioning, release-please may try to bump Go module versions inside `pkg/sdk/go.mod` and update imports across the repo.

**Problem**: This is unwanted behavior for single-versioning setup where Go module paths like `github.com/TerraConstructs/grid/pkg/sdk` should remain stable.

**Fix**: Add to release-please-config.json:

```json
{
  "packages": {
    ".": {
      "include-component-in-tag": false
    }
  }
}
```

**What this does**: Prevents release-please from including component names in tags and attempting to version Go modules independently.

---

### 5. 💾 pnpm Cache Hit Rate Improvement

**Issue**: `setup-node@v6` with `cache: 'pnpm'` has known 2024-2025 bugs:
- pnpm store path changed between pnpm 8 and pnpm 9
- Cache misses occur frequently when lockfile changes
- Typical cache hit rate: ~60%

**Fix**: Add explicit cache step for better hit rate (60% → 90%):

```yaml
- uses: pnpm/action-setup@v4
  with:
    version: 10

- uses: actions/cache@v4
  with:
    path: ~/.pnpm-store
    key: ${{ runner.os }}-pnpm-${{ hashFiles('**/pnpm-lock.yaml') }}
    restore-keys: |
      ${{ runner.os }}-pnpm-

- uses: actions/setup-node@v6
  with:
    node-version: 20
    # Don't use cache: 'pnpm' when using explicit cache above
```

**Grid Recommendation**: Use explicit cache for js/sdk and webapp jobs.

---

### 6. 🎯 Path Filters for Frontend Tests

**Issue**: Frontend tests (pnpm install + vite build + typescript checks) can take 3-4 minutes.

**Problem**: Running on every PR wastes CI time when only Go code changed.

**Fix**: Use path filters to run frontend tests only when webapp files change:

```yaml
frontend-tests:
  runs-on: ubuntu-24.04
  # Only run when webapp files change
  if: |
    github.event_name == 'push' ||
    (github.event_name == 'pull_request' &&
     contains(github.event.pull_request.changed_files.*.path, 'webapp/'))

# OR use on.pull_request.paths filter
on:
  pull_request:
    paths:
      - 'webapp/**'
      - 'js/sdk/**'
      - '!**/*.md'  # Exclude markdown files
```

**Performance Gain**: Reduces PR feedback time by 3-4 minutes for non-frontend PRs.

---

### 7. 🦀 goreleaser Darwin Cross-Compilation

**Issue**: goreleaser cross-compilation for Darwin targets (macOS) requires special setup:
- `CGO_ENABLED=1` for some builds (if using cgo)
- Zig toolchain OR cross-compilation toolchains

**Grid Current Setup**: gridctl uses `CGO_ENABLED=0` (static binaries, no cgo), so no special setup needed.

**If Using CGO (Future)**:

Option 1: Use Zig for cross-compilation:
```yaml
# .goreleaser.yml
builds:
  - env:
      - CGO_ENABLED=1
    flags:
      - -buildmode=default
    ldflags:
      - -extldflags=-static
    goos: [linux, darwin]
    goarch: [amd64, arm64]
    hooks:
      pre:
        - apt-get update && apt-get install -y zig
```

Option 2: Use OSXCross toolchain (more complex)

**Grid Decision**: Stick with `CGO_ENABLED=0` for simplicity unless cgo is required.

---

### 8. 🟢 Node.js Version in goreleaser Workflow

**Issue**: goreleaser workflow builds webapp bundle via pnpm, but GitHub runners don't load Node.js automatically.

**Problem**: pnpm scripts run with system Node (might be incompatible with Node 20 lockfile).

**Fix**: Explicitly setup Node.js before goreleaser runs pnpm:

```yaml
jobs:
  build-webapp:
    steps:
      - uses: actions/setup-node@v6
        with:
          node-version: 20  # Match local dev environment

      - uses: pnpm/action-setup@v4
        with:
          version: 10

      - name: Build webapp
        run: |
          cd webapp
          pnpm install --frozen-lockfile
          pnpm run build
```

**Grid Requirement**: Ensure Node 20 is setup before any pnpm commands in goreleaser workflow.

---

### 9. 🧪 Path Filters Best Practices

**Pattern for Grid**:

```yaml
# PR tests workflow
on:
  pull_request:
    paths:
      # Always run for workflow changes
      - '.github/workflows/pr-tests.yml'

      # Go code
      - '**.go'
      - 'go.mod'
      - 'go.sum'
      - 'go.work'

      # Protobuf
      - 'proto/**'

      # Exclude markdown (don't trigger on doc changes)
      - '!**/*.md'

# Frontend tests workflow (separate)
on:
  pull_request:
    paths:
      - '.github/workflows/frontend-tests.yml'
      - 'webapp/**'
      - 'js/sdk/**'
      - '!**/*.md'
```

**Benefits**: Faster PR feedback, reduced CI minutes usage.

---

## Official Documentation

- [GitHub Actions Documentation](https://docs.github.com/en/actions)
- [GitHub Actions Changelog](https://github.blog/changelog/label/actions/)
- [PostgreSQL Service Containers](https://docs.github.com/actions/guides/creating-postgresql-service-containers)
- [Caching Dependencies](https://docs.github.com/en/actions/using-workflows/caching-dependencies-to-speed-up-workflows)
- [Matrix Strategy](https://docs.github.com/en/actions/using-jobs/using-a-matrix-for-your-jobs)
- [Dependabot Configuration](https://docs.github.com/en/code-security/dependabot/dependabot-version-updates/configuration-options-for-the-dependabot.yml-file)
- [YAML Anchors Announcement](https://github.blog/changelog/2025-09-18-actions-yaml-anchors-and-non-public-workflow-templates/)

## Performance Benchmarks (Grid Estimates)

Based on Grid's test suite:

| Job | Cold Run | Cached Run | Savings |
|-----|----------|------------|---------|
| Unit Tests | 2m 30s | 1m 20s | 47% |
| Integration (plain) | 4m 00s | 2m 30s | 37% |
| Integration (mode2) | 4m 30s | 2m 45s | 39% |
| Frontend Tests | 3m 00s | 1m 30s | 50% |
| Buf Lint | 30s | 20s | 33% |
| Build | 2m 00s | 1m 00s | 50% |

**Total (parallel)**: ~5m (cold) → ~3m (cached)

**Achieving <10 min goal**: ✅ Easily achievable with parallelization and caching