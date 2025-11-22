# Release-Please: Detailed Research

**Research Date**: 2025-11-17
**Latest Version**: googleapis/release-please-action@v4 (core lib v17.1.3)

## Executive Summary

**Decision**: Use release-please with unified versioning (single version for entire repo).

**Key Benefits**:
- Zero-manual-step releases
- Automated changelog generation
- Conventional commit enforcement via PR workflow
- Native GitHub integration (tags, releases)

**Potential future issues**:
* Release Noise and Version Inflation: A minor change in one component (e.g., a typo fix in the webapp) will trigger a version bump for all components, including the Go SDK and CLI. This can confuse users of the SDKs, who will see frequent, yet irrelevant, version updates.
* Dilution of Semantic Versioning: If a breaking change is made to the CLI (e.g., v1.x -> v2.0), all other artifacts are also forced to jump to v2.0. This falsely signals a breaking change for the SDKs and webapp, undermining the reliability of semantic versioning for consumers.
* Obscured Changelogs: The single, aggregated changelog will contain entries from all parts of the monorepo. Users of a specific SDK will have to manually filter out noise from other components to understand what changes are relevant to them.
* Coupled Release Cadences: This approach prevents independent release schedules. The webapp team might want to release updates multiple times a day, while the Go SDK team needs to maintain a much more stable and predictable release cadence. Unified versioning forces them into lockstep.
* Future Migration Complexity: While simpler now, the decision to move to independent versioning later will be a significant effort. It will require re-configuring the release pipeline, potentially creating complex branching strategies, and clearly communicating the new versioning scheme to your user base.

## Latest Stable Version

**GitHub Action**: `googleapis/release-please-action@v4`
**Core Library**: v17.1.3 (October 2025)

**Critical Migration Note**: Action repository moved from `google-github-actions/release-please-action` (archived) → `googleapis/release-please-action`

## Configuration Files

### `.github/release-please-config.json`

```json
{
  "$schema": "https://raw.githubusercontent.com/googleapis/release-please/main/schemas/config.json",
  "packages": {
    ".": {
      "release-type": "go",
      "package-name": "grid",
      "changelog-path": "CHANGELOG.md",
      "extra-files": [
        "cmd/gridapi/version.go",
        "cmd/gridctl/version.go",
        "js/webapp/package.json"
      ]
    }
  },
  "plugins": [
    {
      "type": "node-workspace",
      "updateAllPackages": true,
      "updatePeerDependencies": true
    }
  ],
  "separate-pull-requests": false,
  "bump-minor-pre-major": true,
  "bump-patch-for-minor-pre-major": true,
  "pull-request-title-pattern": "chore: release ${version}",
  "changelog-sections": [
    {"type": "feat", "section": "Features"},
    {"type": "fix", "section": "Bug Fixes"},
    {"type": "perf", "section": "Performance Improvements"},
    {"type": "deps", "section": "Dependencies"},
    {"type": "chore", "section": "Miscellaneous", "hidden": false}
  ]
}
```

### `.release-please-manifest.json`

```json
{
  ".": "0.1.0"
}
```

## GitHub Actions Workflow

```yaml
name: release-please

on:
  push:
    branches:
      - main

permissions:
  contents: write
  pull-requests: write

jobs:
  release-please:
    runs-on: ubuntu-24.04
    outputs:
      release_created: ${{ steps.release.outputs.release_created }}
      tag_name: ${{ steps.release.outputs.tag_name }}
      version: ${{ steps.release.outputs.version }}
    steps:
      - uses: googleapis/release-please-action@v4
        id: release
```

## Key Configuration Options

| Option | Purpose | Recommendation |
|--------|---------|----------------|
| `release-type: go` | Sets Go-specific version handling | Use for root package |
| `extra-files` | Update version in additional files | Add version.go files, package.json |
| `separate-pull-requests` | Combined vs individual PRs | `false` for unified releases |
| `bump-minor-pre-major` | Pre-1.0 versioning behavior | `true` (breaking changes bump minor) |
| `pull-request-title-pattern` | Customize PR titles | Match your commit style |
| `changelog-sections` | Group commits in CHANGELOG | Customize for your needs |
| `node-workspace` plugin | Sync npm workspace deps | Required for pnpm workspaces |

## Version File Annotations

For Go version constants:

```go
// cmd/gridapi/version.go
package main

// x-release-please-start-version
const Version = "0.1.0"
// x-release-please-end

// Also include variables for goreleaser injection
var (
    version   = Version  // Defaults to const, overridden by goreleaser
    commit    = "none"
    date      = "unknown"
    builtBy   = "unknown"
)
```

**Note for Implementation**: The `version` variable will need to be wired up to Cobra commands' `version` subcommand in both `gridapi` and `gridctl`. This is an implementation detail that will be handled during task execution (not part of this research/planning phase).

For structured JSON files:

```json
{
  "extra-files": [
    {
      "type": "json",
      "path": "js/webapp/package.json",
      "jsonpath": "$.version"
    }
  ]
}
```

## Breaking Changes (v3 → v4)

### 1. Action Repository Changed

```yaml
# ❌ OLD (archived)
uses: google-github-actions/release-please-action@v3

# ✅ NEW
uses: googleapis/release-please-action@v4
```

### 2. Output Name Change (CRITICAL)

```yaml
# ❌ NEVER USE - has bugs!
if: steps.release.outputs.releases_created

# ✅ USE INSTEAD
if: steps.release.outputs.release_created == 'true'
```

**Warning**: `releases_created` has a known bug where it returns `true` even when no release was created! This can cause unexpected production deployments.

### 3. Configuration Moved to Files

```yaml
# ❌ v3 approach
with:
  release-type: node
  package-name: my-package
  path: packages/my-package

# ✅ v4 approach
with:
  # No inputs - use config files
  config-file: .github/release-please-config.json
  manifest-file: .github/.release-please-manifest.json
```

## Gotchas & Common Mistakes

### 1. Squash-Merge Strategy (Important!)

- Release-please **strongly recommends** squash-merge for PRs
- Why: Linear history, cleaner changelogs, easier to parse
- Grid setup: Already using squash-merge ✅

### 2. Conventional Commits Required

Only these prefixes trigger releases:
- `feat:` - New feature (minor bump)
- `fix:` - Bug fix (patch bump)
- `deps:` - Dependency update (patch bump)
- `perf:` - Performance improvement (patch bump)

Breaking changes:
- Use `!` marker: `feat!:` or `fix!:` (major/minor bump depending on pre-1.0)

Non-releasable types:
- `chore:`, `build:`, `ci:`, `docs:`, `style:`, `test:` - No version bump

### 3. Token Permissions

- Default `GITHUB_TOKEN` works but **won't trigger other workflows**
- Use a PAT (Personal Access Token) if you need downstream workflows to run
- For Grid: Default token is fine (goreleaser triggered by tag, not release event)

### 4. Initial Setup

- Manually set initial version in `.release-please-manifest.json`
- First run creates the baseline; subsequent runs use git history
- Recommended: Start with `0.1.0` for pre-1.0 projects

### 5. Go-Specific Considerations

- Release-type `go` expects a `CHANGELOG.md` in the root
- No automatic version file updates (use `extra-files` for version.go)
- Tags follow Go module conventions: `v1.2.3` (v-prefix included)

### 6. Extra-Files Paths

- Paths must be **within** the package path
- Cannot use `../` to reference parent directories
- For multi-location updates, use root package (`.`) with all files listed

## Unified vs Independent Versioning

### Unified Versioning (Grid Approach)

**Configuration**:
```json
{
  "packages": {
    ".": {}  // Single root package
  },
  "separate-pull-requests": false
}
```

**Pros**:
- Single version number (v0.4.0) for all artifacts
- One CHANGELOG.md at repo root
- One Release PR to review
- Simpler to understand for users

**Cons**:
- All modules bump version together (even if unchanged)
- Cannot release CLI independently from SDK

### Independent Versioning (Constitution-Compliant)

**Configuration**:
```json
{
  "packages": {
    "cmd/gridapi": {},
    "cmd/gridctl": {},
    "pkg/sdk": {},
    "js/sdk": {}
  },
  "separate-pull-requests": true  // or use linked-versions plugin
}
```

**Pros**:
- True independence for each module
- Only changed modules get version bumps
- Constitution-compliant

**Cons**:
- Multiple Release PRs or complex linked-versions config
- Users track different version numbers (gridctl v2.0, sdk v1.5)
- More complex changelog management

**Grid Decision**: Start with unified versioning per Principle VII (Simplicity & Pragmatism). Migrate to independent versioning when pain is demonstrated (documented in Complexity Tracking).

## Integration with goreleaser

Release-please creates Git tag → goreleaser listens for tag push:

```yaml
# release-please.yml (creates tag)
on:
  push:
    branches: [main]

# release-build.yml (builds on tag)
on:
  push:
    tags: ['v*']
```

**Workflow**:
1. Developer merges PR with conventional commit title
2. release-please opens Release PR
3. Maintainer merges Release PR
4. release-please creates tag `v0.4.0` and GitHub Release
5. Tag push triggers goreleaser workflow
6. goreleaser builds binaries and uploads to GitHub Release

## Official Documentation

- **Action Repository**: https://github.com/googleapis/release-please-action
- **Core Library**: https://github.com/googleapis/release-please
- **Manifest Configuration**: https://github.com/googleapis/release-please/blob/main/docs/manifest-releaser.md
- **Customization Guide**: https://github.com/googleapis/release-please/blob/main/docs/customizing.md
- **Config Schema**: https://raw.githubusercontent.com/googleapis/release-please/main/schemas/config.json
- **Supported Release Types**: https://github.com/googleapis/release-please#release-types-supported

## Recommended Setup Workflow

1. Create `.github/release-please-config.json` (see configuration above)
2. Create `.release-please-manifest.json` with initial version
3. Create `.github/workflows/release-please.yml` (see workflow above)
4. Add version file annotations to `cmd/gridapi/version.go`, `cmd/gridctl/version.go`
5. Test workflow by merging a PR with `feat: test release-please` title
6. Verify Release PR is created automatically
7. Merge Release PR to create tag and GitHub Release

## Performance Considerations

- Release-please execution time: <30 seconds
- Runs on every push to main (cheap, no builds)
- Release PR creation is idempotent (updates existing PR if present)
- No resource-intensive operations (changelog is git log analysis)