# Branch Protection Rules

This document describes the recommended branch protection rules for the Grid repository.

## Overview

Branch protection rules ensure code quality and prevent accidental or malicious changes to critical branches. These settings must be configured manually in GitHub repository settings.

## Configuration Location

Navigate to: **Repository Settings → Branches → Branch protection rules → Add rule**

## Main Branch Protection

### Branch Name Pattern
```
main
```

### Required Settings

#### 1. Require Pull Request Before Merging
- ✅ **Enable**: Require a pull request before merging
- **Required approvals**: 1 (minimum)
- ✅ **Dismiss stale pull request approvals when new commits are pushed**
- ✅ **Require review from Code Owners** (if CODEOWNERS file exists)

**Rationale**: Ensures all changes are reviewed before merging, maintaining code quality.

#### 2. Require Status Checks to Pass Before Merging
- ✅ **Enable**: Require status checks to pass before merging
- ✅ **Require branches to be up to date before merging**

**Required status checks** (add these exact names):
- `unit-tests`
- `integration-tests-mode2`
- `frontend-tests`
- `js-sdk-tests`
- `go-lint`
- `buf-lint`
- `buf-breaking`
- `generated-code-check`
- `pr-title-check / validate-title`

**Rationale**: Prevents broken code from merging and ensures consistent commit history.

#### 3. Require Conversation Resolution Before Merging
- ✅ **Enable**: Require conversation resolution before merging

**Rationale**: Ensures all review comments are addressed.

#### 4. Require Linear History
- ⚠️ **Optional**: Require linear history

**Rationale**: Enforces squash merging or rebase, preventing merge commits. **Decision**: Optional based on team preference.

#### 5. Prevent Force Pushes
- ✅ **Enable**: Do not allow force pushes

**Rationale**: Protects commit history integrity.

#### 6. Prevent Deletions
- ✅ **Enable**: Do not allow deletions

**Rationale**: Prevents accidental branch deletion.

### Optional Settings

#### Restrict Who Can Push to Matching Branches
- ⚠️ **Optional**: Restrict pushes to specific users/teams

**Rationale**: Useful for larger teams to limit direct push access. Most teams can leave this disabled and rely on pull request requirements.

#### Require Deployments to Succeed Before Merging
- ❌ **Disable**: Not applicable (no deployment gates currently)

**Rationale**: Grid doesn't use GitHub deployment environments for branch protection.

#### Lock Branch
- ❌ **Disable**: Branch should remain active

**Rationale**: Main branch is actively developed.

## Release Branches

If using release branches (e.g., `release/*`, `v*`), apply similar protection:

### Branch Name Pattern
```
release/*
```

### Settings
- Same as main branch except:
  - **Require pull request**: Optional (release-please creates PRs)
  - **Status checks**: Optional (already tested on main)

## Development Branches

Personal development branches (e.g., `feature/*`, `fix/*`) should **not** have protection rules to allow flexible iteration.

## Verification

After applying these rules, verify by:

1. **Attempting direct push to main**:
   ```bash
   git checkout main
   git commit --allow-empty -m "test"
   git push origin main
   ```
   **Expected**: Push rejected with error message

2. **Opening a test PR without reviews**:
   - Create a PR from a feature branch
   - Attempt to merge without approval
   **Expected**: Merge button disabled

3. **Opening a PR with failing tests**:
   - Create a PR with intentionally broken code
   - Wait for CI to fail
   **Expected**: Merge button shows "Required checks failing"

4. **Opening a PR with invalid title**:
   - Create a PR with title "my changes"
   **Expected**: `pr-title-check` status check fails

## Troubleshooting

### Status Check Not Appearing

**Problem**: Required status check listed but not enforced.

**Solution**:
1. Status checks must run at least once to be selectable
2. Create a test PR to trigger all workflows
3. Refresh branch protection settings page
4. Status checks should now appear in autocomplete list

### Unable to Merge After Approval

**Problem**: "Required statuses must pass" but all checks are green.

**Solution**:
1. Verify "Require branches to be up to date" is enabled
2. Update branch with latest main: `git merge origin/main` or use GitHub UI "Update branch" button
3. Wait for CI to re-run
4. Merge should now be enabled

### Release Please PR Cannot Merge

**Problem**: Release PR created by release-please bot but stuck due to required approvals.

**Solution**:
1. **Option A**: Manually approve the Release PR (recommended for control)
2. **Option B**: Add `release-please[bot]` to bypass list (⚠️ use with caution)

**Recommended**: Keep approval requirement and manually review release notes before approving.

## GitHub Actions Permissions

To allow release-please and other workflows to function:

### Required Repository Settings

Navigate to: **Repository Settings → Actions → General → Workflow permissions**

- ✅ **Read and write permissions** (for release-please to create PRs and tags)
- ✅ **Allow GitHub Actions to create and approve pull requests**

**Rationale**: release-please needs write access to create Release PRs and tags. Without this, the release workflow will fail.

## Exemptions and Overrides

### Administrator Override

Administrators can bypass branch protection rules. This should be used **very sparingly**:

**Valid use cases**:
- Emergency hotfixes (with post-incident review)
- Repository initialization
- Fixing broken CI (when CI itself is preventing merges)

**Invalid use cases**:
- Skipping code review "to save time"
- Avoiding test failures
- Urgency without actual emergency

### Bot Exemptions

Consider allowing these bots to bypass certain rules:

- `dependabot[bot]`: Auto-merge minor dependency updates (with passing CI)
- `release-please[bot]`: Auto-create release PRs (but keep approval requirement)

**Configuration**: Settings → Branches → Edit rule → "Allow specified actors to bypass required pull requests"

## Monitoring and Auditing

### Regular Audits

Periodically review:
1. **Audit log**: Settings → Security → Audit log
   - Check for bypassed protections
   - Review who modified branch protection rules

2. **Required checks**: Ensure list stays up-to-date with workflow changes
   - New workflows may add required checks
   - Deprecated checks should be removed

### Alerts

Enable notifications for:
- Branch protection rule changes (Settings → Notifications)
- Failed required status checks (automatically notified on PRs)
- PR review requests (individual user settings)

## References

- **GitHub Documentation**: [About protected branches](https://docs.github.com/en/repositories/configuring-branches-and-merges-in-your-repository/managing-protected-branches/about-protected-branches)
- **Workflow definitions**: `.github/workflows/`
- **CI/CD documentation**: `.github/WORKFLOWS.md`

## Updates

This document should be updated when:
- New required workflows are added
- CI job names change
- Branch protection policy changes

**Last Updated**: 2025-11-18
