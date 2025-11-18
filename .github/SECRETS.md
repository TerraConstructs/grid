# GitHub Secrets Configuration

This document describes the GitHub repository secrets required for CI/CD workflows to function properly.

## Overview

GitHub Secrets are encrypted environment variables used in GitHub Actions workflows. They provide secure access to external services without exposing sensitive credentials in code.

## Configuration Location

Navigate to: **Repository Settings → Secrets and variables → Actions → Repository secrets**

## Required Secrets

### RELEASE_PLEASE_TOKEN

**Purpose**: Allow release-please to create branches/PRs that trigger PR checks (GITHUB_TOKEN-created PRs do not run workflows)

**Used by**: `.github/workflows/release-please.yml`

**How to create (Personal Access Token)**:

1. **Create a PAT** at <https://github.com/settings/tokens/new> (or org SSO equivalent)
2. **Token type**: "Fine-grained personal access token"
3. **Repository access**: Restrict to this repository
4. **Permissions**:
   - **Contents**: Read and write (branch pushes)
   - **Pull requests**: Read and write (open/update PR)
   - **Workflows**: Read and write (trigger CI on the PR)
5. **Expiration**: Choose a rotation interval that matches your org policy
6. **Add to GitHub Secrets**:
   - Name: `RELEASE_PLEASE_TOKEN`
   - Value: Paste the token

**Alternative**: A GitHub App installation token with equivalent `contents`, `pull_requests`, and `workflows` permissions also works.

### NPM_TOKEN

**Purpose**: Publish `@tcons/grid` JavaScript SDK to npm registry

**Used by**: `.github/workflows/release-npm.yml`

**How to create**:

1. **Log in to npmjs.com** with your npm account
2. **Navigate to** [Access Tokens](https://www.npmjs.com/settings/<username>/tokens)
3. **Click** "Generate New Token" → "Classic Token"
4. **Select token type**: "Automation" (for CI/CD)
5. **Configure access**:
   - **Package scope**: `@tcons/grid` (or all packages if org-wide)
   - **Permissions**: "Read and write" (required for publishing)
   - **IP allowlist**: Leave empty (GitHub Actions runners use dynamic IPs)
6. **Copy the token** (you won't see it again!)
7. **Add to GitHub Secrets**:
   - Name: `NPM_TOKEN`
   - Value: Paste the token

**Token format**: `npm_<random_string>` (classic) or `npm_v2_<random_string>` (granular)

**Expiration**: Set to "No expiration" for CI/CD (or renew before expiry)

**Rotation**: Regenerate annually or when compromised

## Optional Secrets

### CODECOV_TOKEN (Future)

**Purpose**: Upload test coverage reports to Codecov

**Status**: Not yet implemented (see `specs/008-cicd-workflows/research.md` for future work)

**Used by**: Would be used in `.github/workflows/pr-tests.yml` if coverage is added

## Automatic Secrets (No Action Required)

### GITHUB_TOKEN

**Purpose**: Authenticate GitHub API requests within workflows

**Used by**: All workflows (automatically injected by GitHub). Note: PRs created with `GITHUB_TOKEN` do **not** trigger other workflows; use `RELEASE_PLEASE_TOKEN` for release-please to ensure PR checks run.

**Capabilities**:
- Create PRs and releases (release-please)
- Upload release artifacts (goreleaser)
- Read repository contents
- Write comments and statuses

**Permissions**: Configured per-workflow via `permissions:` block

**Token lifetime**: Automatically rotated per workflow run

**Note**: This token is **automatically provided** by GitHub Actions. No manual configuration needed.

## Verification

### Test NPM_TOKEN

After adding `NPM_TOKEN`, verify it works:

1. **Manually trigger** the `release-npm` workflow:
   - Go to **Actions → Release npm Package → Run workflow**
   - Requires a git tag to exist (or create a test tag)

2. **Check workflow run**:
   - Should succeed and publish to npm (or dry-run if using `--dry-run`)

3. **Expected errors if token is invalid**:
   ```
   error: ENEEDAUTH This command requires you to be logged in
   error: EAUTH Unable to authenticate
   ```

### Test GITHUB_TOKEN (Automatic)

No manual testing needed. If workflows run at all, `GITHUB_TOKEN` is working.

**Common permission errors**:
```
Error: Resource not accessible by integration
```
**Solution**: Check workflow `permissions:` block grants required scopes

## Security Best Practices

### Secret Storage
- ✅ **Use GitHub Secrets** (encrypted at rest)
- ❌ **Never commit secrets** to git (use `.gitignore` for `.env` files)
- ❌ **Never log secrets** in workflow output (GitHub auto-redacts, but avoid explicit echo)

### Access Control
- **Limit scope**: Create tokens with minimum required permissions
- **Use automation tokens**: Separate tokens for CI/CD vs personal use
- **Enable SSO**: If using GitHub Enterprise with SAML SSO, authorize tokens

### Monitoring
- **Audit logs**: Settings → Security → Audit log
  - Check for secret access events
  - Review token usage patterns

- **npm activity**: npmjs.com → Account → Activity
  - Monitor package publish events
  - Verify publish IPs match GitHub Actions (if suspicious)

### Incident Response

**If a secret is compromised**:

1. **Immediately revoke** the token at its source (npm, GitHub, etc.)
2. **Delete the secret** from GitHub Settings
3. **Regenerate** a new token
4. **Add new token** to GitHub Secrets with the same name
5. **Re-run workflows** to verify
6. **Review audit logs** to assess impact
7. **Rotate any downstream secrets** if affected

## Troubleshooting

### Workflow Fails with "Secret not found"

**Problem**: Workflow references `${{ secrets.SECRET_NAME }}` but fails.

**Solutions**:
1. **Verify secret name** matches exactly (case-sensitive)
2. **Check secret scope**:
   - Repository secrets work for all workflows
   - Environment secrets require `environment:` in job
3. **Confirm secret is saved**: Settings → Secrets → verify entry exists

### npm Publish Fails with "401 Unauthorized"

**Problem**: `release-npm` workflow fails during `pnpm publish` step.

**Solutions**:
1. **Regenerate NPM_TOKEN** (may have expired or been revoked)
2. **Verify token permissions**: Must have "Read and write" access
3. **Check package name**: Ensure `@tcons/grid` exists and you have publish rights
4. **Test token locally**:
   ```bash
   echo "//registry.npmjs.org/:_authToken=\${NPM_TOKEN}" > .npmrc
   export NPM_TOKEN=<your_token>
   pnpm publish --dry-run
   ```

### GITHUB_TOKEN Permission Denied

**Problem**: Workflow fails with "Resource not accessible by integration".

**Solutions**:
1. **Check workflow permissions**: Verify `permissions:` block grants needed scopes
2. **Repository settings**: Settings → Actions → General → Workflow permissions
   - Must be "Read and write permissions"
   - "Allow GitHub Actions to create and approve pull requests" enabled
3. **Branch protection**: Admin override may be required for protected branches

## Adding New Secrets

When adding new workflows that require secrets:

1. **Document the secret** in this file
2. **Add creation instructions** (where to generate, what permissions)
3. **Update verification section** (how to test it works)
4. **Add to PR template** (if using PR templates for releases)

## References

- **GitHub Documentation**: [Encrypted secrets](https://docs.github.com/en/actions/security-guides/encrypted-secrets)
- **npm Documentation**: [Creating and viewing access tokens](https://docs.npmjs.com/creating-and-viewing-access-tokens)
- **Workflow files**: `.github/workflows/`
- **Branch protection**: `.github/BRANCH_PROTECTION.md`

## Updates

This document should be updated when:
- New secrets are required for workflows
- Secret names change
- Token creation procedures change
- Security best practices evolve

**Last Updated**: 2025-11-18
