# GitHub Actions Workflows - Grid Project

This document describes the GitHub Actions workflows used in the Grid project for continuous integration and deployment.

## Workflow Files

### 1. PR Testing Workflow (`.github/workflows/pr-tests.yml`)

**Trigger**: Pull requests to any branch

**Purpose**: Validate all code changes before merge with comprehensive testing

**Jobs**:
- **unit-tests**: Run Go unit tests (no external dependencies)
- **integration-tests-mode2**: Integration tests with PostgreSQL (internal IdP mode)
- **frontend-tests**: Web application tests (React + TypeScript)
- **js-sdk-tests**: JavaScript SDK tests (TypeScript compilation, linting, build)
- **go-lint**: Go code linting and formatting checks
- **buf-lint**: Protobuf schema linting
- **buf-breaking**: Protobuf backward compatibility checks
- **generated-code-check**: Verify protobuf-generated code is up-to-date

**Performance Target**: <10 minutes with caching

### 2. PR Title Validation (`.github/workflows/pr-title-check.yml`)

**Trigger**: Pull request creation/update

**Purpose**: Enforce conventional commit format for PR titles

**Validation**: Checks PR title matches pattern:
```
(feat|fix|chore|docs|refactor|test|ci)[(scope)][!]: description
```

Examples:
- `feat: add user authentication`
- `fix(grid-a1b2): resolve state locking issue`
- `chore!: breaking change in API`

### 3. Release Please Workflow (`.github/workflows/release-please.yml`)

**Trigger**: Push to `main` branch

**Purpose**: Automated version management and changelog generation

**Actions**:
1. Analyzes conventional commit messages since last release
2. Opens/updates Release PR with:
   - Version bump (semver: major/minor/patch)
   - CHANGELOG.md updates
   - Version file updates (version.go, package.json)
3. On Release PR merge:
   - Creates Git tag (e.g., `v0.2.0`)
   - Creates GitHub Release with changelog

**Configuration**:
- Uses unified versioning (single version for all components)
- Version stored in `.release-please-manifest.json`
- Config in `.github/release-please-config.json`

### 4. Release Build Workflow (`.github/workflows/release-build.yml`)

**Trigger**: Git tag push (created by release-please)

**Purpose**: Build multi-platform binaries and webapp bundle

**Artifacts Produced**:
- gridapi: `linux/amd64`, `linux/arm64` (2 binaries)
- gridctl: `linux` + `darwin` for `amd64` + `arm64` (4 binaries)
- webapp: Single `.tar.gz` bundle

**Process**:
1. Builds webapp bundle (pnpm build + tar)
2. Runs goreleaser to build Go binaries for all platforms
3. Uploads all artifacts to GitHub Release
4. Generates checksums

**Performance Target**: <20 minutes

### 5. npm Publish Workflow (`.github/workflows/release-npm.yml`)

**Trigger**: Git tag push (created by release-please)

**Purpose**: Publish `@tcons/grid` package to npm registry

**Process**:
1. Build JavaScript SDK (`js/sdk`)
2. Publish to npm with provenance (supply chain attestation)
3. Version synchronized with Git tag via release-please

**Required Secret**: `NPM_TOKEN` (automation token)

## Common Patterns

### Caching Strategy

All workflows use built-in caching for performance:

**Go modules**:
```yaml
- uses: actions/setup-go@v6
  with:
    go-version-file: go.mod
    cache: true  # Automatic go.sum-based caching
```

**pnpm dependencies**:
```yaml
- uses: pnpm/action-setup@v4
  with:
    version: 10

- uses: actions/setup-node@v6
  with:
    node-version: 20
    cache: 'pnpm'
    cache-dependency-path: 'js/sdk/pnpm-lock.yaml'
```

**Expected cache hit rate**: 60-80% during normal development

### Docker Compose for Services

Integration tests use docker-compose for dev/CI parity:

```yaml
- name: Start PostgreSQL
  run: docker compose up -d postgres

- name: Wait for health
  run: |
    until docker compose ps postgres | grep -q "healthy"; do
      sleep 2
    done
```

**Benefits**:
- Single source of truth (`docker-compose.yml`)
- Identical setup locally and in CI
- Automatic init scripts and volume handling
- Easy troubleshooting (reproduce CI issues locally)

### Version Injection

Binaries receive version information via two mechanisms:

1. **release-please** updates `const Version` in `version.go` files
2. **goreleaser** injects runtime metadata via `-ldflags`:
   ```
   -X main.version={{.Version}}
   -X main.commit={{.Commit}}
   -X main.date={{.Date}}
   -X main.builtBy=goreleaser
   ```

### Parallel Job Execution

Jobs run in parallel when possible for performance:
- All PR test jobs are independent (parallel)
- Release build and npm publish are sequential (tag → build → npm)

### Runner Configuration

**Standard runner**: `ubuntu-24.04` (explicit version for reproducibility)

**Benefits over `ubuntu-latest`**:
- Pinned OS version prevents surprise breakages
- Easier debugging (know exact environment)
- Can upgrade deliberately when ready

## Dependency Updates

**Dependabot** (`..github/dependabot.yml`) provides automated dependency updates:

**Grouping strategy**:
- Go modules grouped by type (direct, indirect)
- npm packages grouped by scope
- GitHub Actions grouped together

**Schedule**: Weekly

**Benefits**:
- Fewer PRs (grouped updates)
- Reduced noise
- Security patches applied promptly

## Branch Protection Rules

**Recommended settings for `main` branch**:

1. **Require pull request reviews**: 1 approval minimum
2. **Require status checks**: All PR test jobs must pass
   - `unit-tests`
   - `integration-tests-mode2`
   - `frontend-tests`
   - `js-sdk-tests`
   - `go-lint`
   - `buf-lint`
   - `buf-breaking`
   - `generated-code-check`
   - `pr-title-check`
3. **Require branches up-to-date**: Yes (prevents race conditions)
4. **No force pushes**: Disabled
5. **Require linear history**: Optional (consider for cleaner history)

**Repository Settings**:
- Allow GitHub Actions to create PRs (for release-please)
- Automatic head branch deletion after merge

## Required Secrets

Add these secrets in repository settings (`Settings → Secrets and variables → Actions`):

| Secret | Purpose | How to Create |
|--------|---------|---------------|
| `NPM_TOKEN` | Publish to npm | Create "Automation" token at npmjs.com |

**Note**: `GITHUB_TOKEN` is automatically provided by GitHub Actions.

## Troubleshooting

### PR Tests Failing

1. **Check job logs**: Click failed job in GitHub Actions UI
2. **Reproduce locally**:
   ```bash
   # For unit tests
   make test-unit

   # For integration tests
   docker compose up -d postgres
   make test-integration-mode2

   # For frontend
   cd webapp && pnpm install && pnpm run test
   ```
3. **Common issues**:
   - Stale cache: Re-run workflow
   - Docker service not ready: Check healthcheck timing
   - Dependency mismatch: Verify lockfiles committed

### Release Please Not Creating PR

**Possible causes**:
1. No releasable commits since last release (check commit messages)
2. Previous Release PR still open (close or merge it)
3. Configuration error (validate JSON syntax)

**Debug**:
- Check workflow run logs
- Verify conventional commit format
- Ensure commits on `main` branch

### goreleaser Build Failing

**Common issues**:
1. Version variables not found: Check `version.go` file exists
2. Webapp build failing: Test `cd webapp && pnpm run build` locally
3. Cross-compilation errors: Check `CGO_ENABLED=0` set

## Performance Monitoring

Track these metrics over time:

- **PR feedback time**: Goal <10 minutes
- **Release build time**: Goal <20 minutes
- **Cache hit rate**: Goal >60%

**How to check**:
1. Go to Actions tab
2. Select workflow run
3. View job timing in summary

## Migration Notes

**Unified Versioning**: Currently using single version for all components (gridapi, gridctl, sdk, webapp). This is documented in `specs/008-cicd-workflows/plan.md` Complexity Tracking.

**Future migration** to independent per-module versioning would require:
- Multi-package release-please config
- Multiple goreleaser configs (one per binary)
- Tag-based workflow triggering (e.g., `gridctl/v*`)

Migration path is documented in `specs/008-cicd-workflows/research.md`.
