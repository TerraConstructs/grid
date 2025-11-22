# End-to-End Release Flow Testing

This document provides a comprehensive test plan for validating the complete CI/CD release pipeline before production use.

## Overview

The Grid release automation involves multiple workflows working together:
1. **PR Testing** → Validates code changes
2. **Release Please** → Creates Release PR and Git tag
3. **Release Build** → Builds binaries via goreleaser
4. **npm Publish** → Publishes JavaScript SDK

This test plan validates each component and the full end-to-end flow.

## Prerequisites

Before testing, ensure:
- ✅ All workflows are committed to `main` branch
- ✅ GitHub Secrets are configured (see `.github/SECRETS.md`)
- ✅ Branch protection rules are applied (see `.github/BRANCH_PROTECTION.md`)
- ✅ Repository has at least one commit with conventional commit format

## Test Plan

### Phase 1: PR Testing Validation

**Goal**: Verify all PR test jobs execute correctly

**Steps**:

1. **Create a test branch**:
   ```bash
   git checkout -b test/cicd-validation
   ```

2. **Make a trivial change**:
   ```bash
   echo "# Test" >> .github/TEST.md
   git add .github/TEST.md
   git commit -m "test: validate CI/CD workflows"
   git push origin test/cicd-validation
   ```

3. **Open a Pull Request**:
   - Navigate to GitHub → Pull Requests → New Pull Request
   - **Base**: `main`
   - **Compare**: `test/cicd-validation`
   - **Title**: `test: validate CI/CD workflows` (must follow conventional commits)
   - Click "Create Pull Request"

4. **Monitor PR checks**:
   - Navigate to PR → "Checks" tab
   - Wait for all jobs to complete (expected ~5-10 minutes)

5. **Expected results**:
   - ✅ `unit-tests` passes
   - ✅ `integration-tests-mode2` passes
   - ✅ `frontend-tests` passes
   - ✅ `js-sdk-tests` passes
   - ✅ `go-lint` passes
   - ✅ `buf-lint` passes
   - ✅ `buf-breaking` passes
   - ✅ `generated-code-check` passes
   - ✅ `pr-title-check / validate-title` passes

6. **Troubleshooting**:
   - If any check fails, click into the job logs
   - Common issues:
     - **Cache misses**: Expected on first run, should pass on retry
     - **Docker timeout**: Increase healthcheck wait time in workflow
     - **Breaking changes**: Expected if protobuf changed (acknowledge or fix)

7. **Cleanup** (optional):
   - Close the test PR (don't merge)
   - Delete the test branch

**Success Criteria**: All 9 required checks pass ✅

---

### Phase 2: Release Please Dry Run

**Goal**: Verify release-please creates a Release PR

**Steps**:

1. **Ensure main has releasable commits**:
   ```bash
   git checkout main
   git pull origin main
   git log --oneline -10
   ```
   - Check for commits with `feat:`, `fix:`, `chore:`, etc.
   - If none exist, merge the test PR from Phase 1 or create one

2. **Trigger release-please**:
   - **Option A** (automatic): Push to main (triggers on push)
   - **Option B** (manual): Actions → Release Please → Run workflow

3. **Monitor workflow run**:
   - Navigate to Actions → Release Please → Latest run
   - Wait for completion (~30 seconds)

4. **Expected results**:
   - ✅ Workflow completes successfully
   - ✅ Release PR is created OR updated
   - Navigate to Pull Requests → Find PR titled `chore: release 0.x.y`

5. **Inspect Release PR**:
   - **Title**: `chore: release 0.x.y` (version should increment)
   - **Changes**:
     - `CHANGELOG.md` updated with new entries
     - `cmd/gridapi/version.go` updated with new version
     - `cmd/gridctl/version.go` updated with new version
     - `js/sdk/package.json` updated with new version
     - `.release-please-manifest.json` updated with new version
   - **Labels**: `autorelease: pending`

6. **Troubleshooting**:
   - **No Release PR created**: No releasable commits since last release
   - **Version wrong**: Check `.release-please-manifest.json` for current version
   - **Missing file updates**: Check `extra-files` in `.github/release-please-config.json`

**Success Criteria**: Release PR created with correct version bumps ✅

---

### Phase 3: Release Build Test

**Goal**: Verify goreleaser builds all artifacts

**Prerequisites**: Release PR from Phase 2 exists

**Steps**:

1. **Merge the Release PR** (⚠️ triggers actual release):
   - Navigate to the Release PR
   - **Review changes** carefully (versions, changelog)
   - Click "Merge pull request"
   - **Confirm merge**

2. **Monitor release-please output**:
   - Workflow creates a Git tag (e.g., `v0.2.0`)
   - Workflow creates a GitHub Release (draft or published)

3. **Monitor release-build workflow**:
   - Navigate to Actions → Release Build → Latest run (triggered by tag)
   - Wait for completion (~5-10 minutes)

4. **Expected results**:
   - ✅ Workflow completes successfully
   - ✅ GitHub Release created with artifacts

5. **Inspect GitHub Release**:
   - Navigate to Releases → Latest release
   - **Tag**: `v0.x.y` (matches Release PR version)
   - **Release notes**: Auto-generated from CHANGELOG.md
   - **Artifacts** (7 files expected):
     - `grid_0.x.y_linux_amd64.tar.gz` (gridapi + gridctl for Linux amd64)
     - `grid_0.x.y_linux_arm64.tar.gz` (gridapi + gridctl for Linux arm64)
     - `grid_0.x.y_darwin_amd64.tar.gz` (gridctl for macOS Intel)
     - `grid_0.x.y_darwin_arm64.tar.gz` (gridctl for macOS Apple Silicon)
     - `webapp_0.x.y.tar.gz` (webapp bundle)
     - `checksums.txt` (SHA256 checksums)
     - Source code (auto-generated by GitHub)

6. **Verify artifact contents**:
   ```bash
   # Download a binary archive
   wget https://github.com/TerraConstructs/grid/releases/download/v0.x.y/grid_0.x.y_linux_amd64.tar.gz

   # Extract
   tar -xzf grid_0.x.y_linux_amd64.tar.gz

   # Verify binaries
   ./gridapi version
   ./gridctl version
   ```

   **Expected output**:
   ```
   gridapi version 0.x.y
     commit: <git_sha>
     built: <timestamp>
     by: goreleaser
   ```

7. **Troubleshooting**:
   - **No artifacts**: Check goreleaser logs for build errors
   - **Missing platforms**: Verify `.goreleaser.yml` build matrix
   - **Webapp bundle missing**: Check pnpm build logs in "before hooks"

**Success Criteria**: All 7 artifacts present and binaries run ✅

---

### Phase 4: npm Publish Test

**Goal**: Verify JavaScript SDK publishes to npm

**Prerequisites**: Release from Phase 3 completed

**Steps**:

1. **Monitor release-npm workflow**:
   - Navigate to Actions → Publish npm Package → Latest run
   - Triggered by same tag as release-build
   - Wait for completion (~1-2 minutes)

2. **Expected results**:
   - ✅ Workflow completes successfully
   - ✅ Package published to npm

3. **Verify npm package**:
   ```bash
   # Check latest version
   npm view @tcons/grid version

   # Should output: 0.x.y (matching the Git tag)

   # Install and test
   npm install @tcons/grid
   node -e "const grid = require('@tcons/grid'); console.log(grid);"
   ```

4. **Verify package metadata**:
   - Visit https://www.npmjs.com/package/@tcons/grid
   - **Version**: Should match Git tag `v0.x.y`
   - **Published**: Timestamp should be recent
   - **Provenance**: Should show GitHub Actions attestation

5. **Troubleshooting**:
   - **401 Unauthorized**: Check `NPM_TOKEN` secret (see `.github/SECRETS.md`)
   - **404 Not Found**: Package name may be wrong or not yet created
   - **Version conflict**: Package version already exists (happens if re-running release)

**Success Criteria**: Package published with correct version ✅

---

### Phase 5: Full End-to-End Validation

**Goal**: Validate complete workflow from PR to release

**Test Scenario**: Simulate a real feature release

**Steps**:

1. **Create a feature branch**:
   ```bash
   git checkout main
   git pull origin main
   git checkout -b feat/test-release-flow
   ```

2. **Make a meaningful change**:
   ```bash
   # Example: Add a new CLI flag
   echo "// Test feature flag" >> cmd/gridctl/cmd/root.go
   git add cmd/gridctl/cmd/root.go
   git commit -m "feat: add test feature flag for release validation"
   git push origin feat/test-release-flow
   ```

3. **Open PR and wait for checks**:
   - Create PR with title `feat: add test feature flag for release validation`
   - Wait for all 9 PR checks to pass
   - Get review approval (if branch protection requires)

4. **Merge PR to main**:
   - Merge the PR
   - Observe release-please creates/updates Release PR

5. **Merge Release PR**:
   - Review Release PR changes
   - Merge Release PR
   - Observe tag creation and release workflows trigger

6. **Wait for all workflows**:
   - release-please (tag creation)
   - release-build (artifacts)
   - release-npm (package)

7. **Verify final outputs**:
   - ✅ GitHub Release with all artifacts
   - ✅ npm package published
   - ✅ CHANGELOG.md contains new feature entry
   - ✅ Version bumped correctly (minor for `feat:`)

**Success Criteria**: Complete flow from PR to published release ✅

---

## Test Matrix

| Test | Status | Notes |
|------|--------|-------|
| PR Tests - Unit | ⬜ | |
| PR Tests - Integration (mode2) | ⬜ | |
| PR Tests - Frontend | ⬜ | |
| PR Tests - JS SDK | ⬜ | |
| PR Tests - Go Lint | ⬜ | |
| PR Tests - Buf Lint | ⬜ | |
| PR Tests - Buf Breaking | ⬜ | |
| PR Tests - Generated Code | ⬜ | |
| PR Title Validation | ⬜ | |
| Release Please - PR Creation | ⬜ | |
| Release Please - Version Bump | ⬜ | |
| Release Please - CHANGELOG | ⬜ | |
| Release Build - gridapi (linux/amd64) | ⬜ | |
| Release Build - gridapi (linux/arm64) | ⬜ | |
| Release Build - gridctl (all platforms) | ⬜ | |
| Release Build - webapp bundle | ⬜ | |
| Release Build - Checksums | ⬜ | |
| npm Publish - Package | ⬜ | |
| npm Publish - Provenance | ⬜ | |

## Rollback Procedures

### Undo a Bad Release

If a release is published with critical issues:

1. **Delete the GitHub Release**:
   - Releases → Click release → Delete release
   - **Keep the tag** for audit trail (or delete if needed)

2. **Unpublish npm package** (within 72 hours):
   ```bash
   npm unpublish @tcons/grid@0.x.y
   ```
   **Warning**: This is permanent and may break users

3. **Deprecate instead** (preferred):
   ```bash
   npm deprecate @tcons/grid@0.x.y "This version has critical bugs, please upgrade"
   ```

4. **Create hotfix release**:
   - Fix the issue on main
   - Release-please will create a patch version (0.x.y+1)
   - Publish the fixed version

### Revert a Merged PR

If a merged PR breaks main:

1. **Revert the commit**:
   ```bash
   git revert <commit_sha>
   git push origin main
   ```

2. **Update Release PR** (if exists):
   - release-please will update the existing Release PR
   - Revert will appear in CHANGELOG

3. **Do not delete tags**: Deleting tags confuses release-please state

## Performance Benchmarks

Track these metrics over time:

| Metric | Target | Current | Notes |
|--------|--------|---------|-------|
| PR Test Feedback Time | <10 min | ___ | From PR creation to all checks green |
| Release Build Time | <20 min | ___ | From tag push to artifacts uploaded |
| npm Publish Time | <2 min | ___ | From workflow start to package visible |
| Cache Hit Rate | >60% | ___ | Go modules + pnpm dependencies |

## Next Steps

After completing this test plan:

1. **Document results**: Fill in the Test Matrix checkboxes
2. **Apply branch protection**: See `.github/BRANCH_PROTECTION.md`
3. **Enable Dependabot**: Automatic dependency updates
4. **Monitor first real release**: Watch for issues in production

## References

- **Workflow documentation**: `.github/WORKFLOWS.md`
- **Branch protection**: `.github/BRANCH_PROTECTION.md`
- **Secrets configuration**: `.github/SECRETS.md`
- **Release please**: `specs/008-cicd-workflows/research/release-please.md`
- **goreleaser**: `specs/008-cicd-workflows/research/goreleaser.md`

**Last Updated**: 2025-11-18
