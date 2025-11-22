# GoReleaser: Detailed Research

**Research Date**: 2025-11-17
**Latest Version**: v2.12.7 (October 2025)
**GitHub Action**: v6.4.0

## Executive Summary

**Decision**: Use goreleaser for multi-platform Go binary builds + webapp bundle packaging.

**Key Benefits**:
- Native cross-compilation for Linux/Darwin, amd64/arm64
- Flexible artifact handling (binaries + custom archives)
- Automatic GitHub Release uploads
- Reproducible builds with commit-based timestamps

## Latest Versions

- **GoReleaser**: v2.12.7 (October 2025)
- **GitHub Action**: v6.4.0 (August 14, 2025)
- **Action Reference**: `goreleaser/goreleaser-action@v6`
- **Recommended Version Constraint**: `~> v2`

## Complete .goreleaser.yaml Configuration

```yaml
# .goreleaser.yaml
version: 2

before:
  hooks:
    # Ensure dependencies are up to date
    - go mod tidy
    # Optional: Run tests before building
    # - go test ./...

builds:
  # gridapi - Linux only (server binary)
  - id: gridapi
    main: ./cmd/gridapi
    binary: gridapi
    env:
      - CGO_ENABLED=0
    goos:
      - linux
    goarch:
      - amd64
      - arm64
    # Reproducible builds (2025 best practice)
    mod_timestamp: "{{ .CommitTimestamp }}"
    flags:
      - -trimpath
    ldflags:
      - -s -w
      - -X main.version={{ .Version }}
      - -X main.commit={{ .Commit }}
      - -X main.date={{ .CommitDate }}
      - -X main.builtBy=goreleaser

  # gridctl - Multi-platform CLI
  - id: gridctl
    main: ./cmd/gridctl
    binary: gridctl
    env:
      - CGO_ENABLED=0
    goos:
      - linux
      - darwin
    goarch:
      - amd64
      - arm64
    # Reproducible builds
    mod_timestamp: "{{ .CommitTimestamp }}"
    flags:
      - -trimpath
    ldflags:
      - -s -w
      - -X main.version={{ .Version }}
      - -X main.commit={{ .Commit }}
      - -X main.date={{ .CommitDate }}
      - -X main.builtBy=goreleaser

archives:
  # Archive for gridapi (Linux only)
  - id: gridapi-archive
    builds:
      - gridapi
    name_template: "grid-gridapi_{{ .Version }}_{{ .Os }}_{{ .Arch }}"
    formats:
      - tar.gz
    files:
      - LICENSE
      - README.md
      - CHANGELOG.md

  # Archive for gridctl (multi-platform)
  - id: gridctl-archive
    builds:
      - gridctl
    name_template: "grid-gridctl_{{ .Version }}_{{ .Os }}_{{ .Arch }}"
    formats:
      - tar.gz
      - zip
    files:
      - LICENSE
      - README.md
      - CHANGELOG.md
    format_overrides:
      - goos: darwin
        format: zip

checksum:
  name_template: "checksums.txt"
  algorithm: sha256

changelog:
  # Use github-native for best integration with release-please
  use: github-native
  sort: asc
  filters:
    exclude:
      - "^docs:"
      - "^test:"
      - "^ci:"
      - "^chore:"
      - "^build\\(deps\\):"

release:
  github:
    owner: TerraConstructs
    name: grid

  # Don't replace existing releases (important for release-please workflow)
  replace_existing_draft: false
  replace_existing_artifacts: false

  # Allow modifying release after creation
  draft: false
  prerelease: auto

  # Add webapp bundle as extra file
  extra_files:
    - glob: ./dist/grid-webapp_{{ .Version }}.tar.gz

  # Release notes customization
  header: |
    ## Grid {{ .Tag }} Release

    Welcome to this new release!

  footer: |
    **Full Changelog**: https://github.com/TerraConstructs/grid/compare/{{ .PreviousTag }}...{{ .Tag }}

# Metadata for the release
metadata:
  mod_timestamp: "{{ .CommitTimestamp }}"

# Performance optimization
parallelism: -1  # Use all available CPUs
```

## GitHub Actions Workflow

### Complete Workflow with Webapp Build

```yaml
name: Release Build

on:
  push:
    tags:
      - 'v*'

permissions:
  contents: write
  packages: write

jobs:
  # Build webapp first
  build-webapp:
    runs-on: ubuntu-24.04
    steps:
      - name: Checkout
        uses: actions/checkout@v5
        with:
          fetch-depth: 0

      - name: Setup pnpm
        uses: pnpm/action-setup@v4
        with:
          version: 10

      - name: Setup Node.js
        uses: actions/setup-node@v6
        with:
          node-version: '20'
          cache: 'pnpm'
          cache-dependency-path: 'webapp/pnpm-lock.yaml'
      # omitted: Build the JS SDK first
      - name: Build webapp
        run: |
          cd webapp
          pnpm install --frozen-lockfile
          pnpm run build

      - name: Create webapp tarball
        run: |
          cd webapp/dist
          tar -czf ../../dist/grid-webapp_${{ github.ref_name }}.tar.gz .

      - name: Upload webapp artifact
        uses: actions/upload-artifact@v4
        with:
          name: webapp-dist
          path: dist/grid-webapp_*.tar.gz
          retention-days: 1

  # Release with GoReleaser
  goreleaser:
    needs: build-webapp
    runs-on: ubuntu-24.04
    steps:
      - name: Checkout
        uses: actions/checkout@v5
        with:
          fetch-depth: 0

      - name: Set up Go
        uses: actions/setup-go@v6
        with:
          go-version: '1.24'
          cache: true

      - name: Download webapp artifact
        uses: actions/download-artifact@v4
        with:
          name: webapp-dist
          path: dist/

      - name: Run GoReleaser
        uses: goreleaser/goreleaser-action@v6
        with:
          distribution: goreleaser
          version: '~> v2'
          args: release --clean
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```

## Build Configuration Best Practices

### Multi-Platform Matrix

```yaml
builds:
  - env:
      - CGO_ENABLED=0  # Static binaries (no libc dependency)
    goos:
      - linux
      - darwin
    goarch:
      - amd64
      - arm64
    # Ignore invalid combinations
    ignore:
      - goos: darwin
        goarch: 386  # macOS doesn't support 32-bit
```

### Reproducible Builds

**Critical for security and verification**:

```yaml
builds:
  - mod_timestamp: "{{ .CommitTimestamp }}"  # Use commit time, not build time
    flags:
      - -trimpath  # Remove file system paths from binary
    ldflags:
      - -X main.date={{ .CommitDate }}  # Commit date, not current date
```

**Why this matters**:
- Same source = same binary (byte-for-byte reproducible)
- Security audits can verify binaries match source code
- Supply chain attack detection

### Build Flags Explained

```yaml
ldflags:
  - -s -w                          # Strip symbol table and debug info (smaller binaries)
  - -X main.version={{ .Version }} # Inject version from Git tag
  - -X main.commit={{ .Commit }}   # Inject commit SHA
  - -X main.date={{ .CommitDate }} # Inject commit date
  - -X main.builtBy=goreleaser     # Identify build tool
```

**Binary size reduction**: `-s -w` typically reduces binary size by 20-30%

## Archive Naming Conventions

### Standard Convention (Recommended)

```yaml
name_template: "{{ .ProjectName }}_{{ .Version }}_{{ .Os }}_{{ .Arch }}"
```

Produces:
- `grid-gridapi_1.0.0_linux_amd64.tar.gz`
- `grid-gridctl_1.0.0_darwin_arm64.zip`

### Component-Specific Naming

```yaml
name_template: "{{ .ProjectName }}-{{ .Binary }}_{{ .Version }}_{{ .Os }}_{{ .Arch }}"
```

Produces:
- `grid-gridapi_1.0.0_linux_amd64.tar.gz`
- `grid-gridctl_1.0.0_darwin_arm64.tar.gz`

### With Architecture Variants

```yaml
name_template: >-
  {{ .ProjectName }}_{{ .Version }}_{{ .Os }}_{{ .Arch }}
  {{- with .Arm }}v{{ . }}{{ end }}
  {{- with .Mips }}_{{ . }}{{ end }}
  {{- if not (eq .Amd64 "v1") }}{{ .Amd64 }}{{ end }}
```

## Custom Artifacts (Webapp Tarball)

### Approach 1: Extra Release Files (Recommended)

```yaml
release:
  extra_files:
    - glob: ./dist/grid-webapp_{{ .Version }}.tar.gz
```

**Pros**:
- Clean separation: webapp is standalone artifact
- Easy to identify in release assets
- goreleaser doesn't try to extract/process it

**Cons**:
- Requires pre-building webapp in separate job
- Need artifact upload/download in workflow

### Approach 2: Include in Binary Archives

```yaml
archives:
  - id: gridapi-archive
    builds:
      - gridapi
    files:
      - LICENSE
      - README.md
      - src: "dist/webapp.tar.gz"
        dst: webapp.tar.gz
        strip_parent: true
```

**Pros**:
- Single archive per platform includes everything
- Users download one file

**Cons**:
- Larger archive size
- Webapp duplicated across all gridapi platform archives

**Grid Recommendation**: Use Approach 1 (extra_files) for cleaner separation.

## Changelog Generation

### GitHub-Native (Best for 2025)

```yaml
changelog:
  use: github-native  # Uses GitHub's release notes API
```

**Pros**:
- Integrates with GitHub's release notes
- Automatic contributor attribution
- No conflicts with release-please

**Cons**:
- Requires GitHub API access
- Limited customization

### Git-Based Generation

```yaml
changelog:
  use: git
  sort: asc
  filters:
    exclude:
      - "^docs:"
      - "^test:"
      - "^ci:"
      - Merge pull request
      - Merge branch
  groups:
    - title: Features
      regexp: '^.*?feat(\([[:word:]]+\))??!?:.+$'
      order: 0
    - title: Bug Fixes
      regexp: '^.*?fix(\([[:word:]]+\))??!?:.+$'
      order: 1
    - title: Others
      order: 999
```

**Grid Recommendation**: Use `github-native` to avoid conflicts with release-please changelog.

## GitHub Release Integration

### Important Settings for release-please Workflow

```yaml
release:
  # Don't replace release-please's release
  replace_existing_draft: false
  replace_existing_artifacts: false

  # Auto-detect prerelease (v1.0.0-rc1)
  prerelease: auto

  # Mark as latest release
  make_latest: true
```

**Why this matters**:
- release-please creates the GitHub Release first
- goreleaser adds artifacts to **existing** release
- `replace_existing_artifacts: false` prevents overwriting release-please's metadata

### Custom Release Notes

```yaml
release:
  header: |
    ## Grid {{ .Tag }}

    Download the appropriate binary for your platform below.

  footer: |
    **Full Changelog**: https://github.com/TerraConstructs/grid/compare/{{ .PreviousTag }}...{{ .Tag }}

    ---

    ### Installation

    ```bash
    # Linux (amd64)
    wget https://github.com/TerraConstructs/grid/releases/download/{{ .Tag }}/grid-gridctl_{{ .Version }}_linux_amd64.tar.gz
    tar -xzf grid-gridctl_{{ .Version }}_linux_amd64.tar.gz
    sudo mv gridctl /usr/local/bin/
    ```
```

## Performance Optimization

### Parallelism

```yaml
# Use all available CPUs
parallelism: -1

# OR set specific number
parallelism: 4
```

**Impact**: 4-8 platform builds complete in <2 minutes with parallelism: -1

### Caching in GitHub Actions

```yaml
- uses: actions/setup-go@v6
  with:
    go-version: '1.24'
    cache: true  # Automatically cache Go modules
```

**Performance gain**: 40-60% faster builds with warm cache

### Local Testing (Development)

```bash
# Build without releasing (fast iteration)
goreleaser build --snapshot --clean --single-target

# Build for current platform only
goreleaser build --single-target

# Test full release locally
goreleaser release --snapshot --clean --skip=publish
```

**Use case**: Test .goreleaser.yaml changes without creating releases

## Platform-Specific Considerations

### Linux Binaries

```yaml
goos:
  - linux
goarch:
  - amd64
  - arm64
formats:
  - tar.gz  # Standard for Linux
```

**Static binaries**: `CGO_ENABLED=0` creates fully static binaries (no libc dependency)

### Darwin (macOS) Binaries

```yaml
goos:
  - darwin
goarch:
  - amd64  # Intel Macs
  - arm64  # Apple Silicon
formats:
  - zip    # macOS users expect .zip
```

**Code signing** (future): goreleaser supports macOS code signing with `signs` configuration

### Windows Binaries (Future)

```yaml
goos:
  - windows
goarch:
  - amd64
formats:
  - zip  # Windows users expect .zip
```

**Grid Decision**: Windows support deferred (not in initial scope)

## Version Injection

### Setting up version.go

```go
// cmd/gridapi/main.go
package main

var (
    version   = "dev"
    commit    = "none"
    date      = "unknown"
    builtBy   = "unknown"
)

func main() {
    fmt.Printf("gridapi %s (commit: %s, built: %s by %s)\n",
        version, commit, date, builtBy)
    // ...
}
```

### goreleaser injects at build time

```yaml
ldflags:
  - -X main.version={{ .Version }}
  - -X main.commit={{ .Commit }}
  - -X main.date={{ .CommitDate }}
  - -X main.builtBy=goreleaser
```

**Result**:
```bash
$ gridctl version
gridctl 1.0.0 (commit: abc123f, built: 2025-11-17T10:30:00Z by goreleaser)
```

## New Features in v2.x (2025)

1. **github-native changelog**: Better GitHub integration
2. **Improved reproducible builds**: `mod_timestamp` support
3. **Better ARM64 support**: Native builds on Apple Silicon
4. **Improved parallelism**: Better CPU utilization with `-1` flag
5. **Format improvements**: `formats` (plural) replaces `format`

## Breaking Changes (v1 → v2)

- `archives.format` → `archives.formats` (plural)
- Default version constraint changed from `~> v1` to `~> v2`
- Some deprecated fields removed (check migration guide)

## Common Gotchas

### 1. Archive Format per Platform

```yaml
format_overrides:
  - goos: darwin
    format: zip  # macOS users prefer .zip
  - goos: linux
    format: tar.gz  # Linux users prefer .tar.gz
```

### 2. Files in Archives

```yaml
files:
  - LICENSE
  - README.md
  - CHANGELOG.md
  # Glob patterns supported
  - docs/**/*.md
```

**Default**: If not specified, only the binary is included

### 3. Checksum File

Always generated automatically:
```
checksums.txt
```

Contains SHA256 hashes of all artifacts (important for verification)

### 4. Git Tag Requirement

goreleaser requires a Git tag to run:
```bash
# Local testing uses snapshot mode
goreleaser release --snapshot  # No tag required

# Actual release requires tag
git tag -a v1.0.0 -m "Release v1.0.0"
goreleaser release  # Uses tag v1.0.0
```

## Official Documentation

- **Main Documentation**: https://goreleaser.com/
- **Customization Reference**: https://goreleaser.com/customization/
- **GitHub Actions Integration**: https://goreleaser.com/ci/actions/
- **Action Repository**: https://github.com/goreleaser/goreleaser-action
- **Release Notes**: https://github.com/goreleaser/goreleaser/releases
- **Build Configuration**: https://goreleaser.com/customization/build/
- **Archive Configuration**: https://goreleaser.com/customization/archive/

## Expected Build Artifacts (Grid)

After goreleaser completes, the GitHub Release will contain:

### Binaries (6 archives)
1. `grid-gridapi_1.0.0_linux_amd64.tar.gz`
2. `grid-gridapi_1.0.0_linux_arm64.tar.gz`
3. `grid-gridctl_1.0.0_linux_amd64.tar.gz`
4. `grid-gridctl_1.0.0_linux_arm64.tar.gz`
5. `grid-gridctl_1.0.0_darwin_amd64.zip`
6. `grid-gridctl_1.0.0_darwin_arm64.zip`

### Webapp
7. `grid-webapp_1.0.0.tar.gz` (extra_file)

### Metadata
8. `checksums.txt` (SHA256 hashes for verification)

**Total**: 8 files per release

## Performance Benchmarks

Based on typical Go projects:
- **Build time**: 1-2 minutes for 6 platform combinations (with parallelism: -1)
- **Artifact upload**: 30-60 seconds (depends on binary size)
- **Total goreleaser job**: ~3-5 minutes

**Grid estimation**: With gridapi + gridctl + webapp, expect <5 minutes total.