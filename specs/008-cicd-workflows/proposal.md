## 1. Versioning & changelog model

* **Single version for the whole repo**

  * One semantic version (e.g. `v0.4.0`) applies to:

    * `gridapi` CLI
    * `gridctl` CLI
    * `pkg/sdk` (Go)
    * `js/sdk` (`@tcons/grid`)
    * `webapp` bundle

* **Single changelog at repo root**

  * `CHANGELOG.md` lives at root.
  * Release notes describe changes across all surfaces.

* **Conventional commits + clean main**

  * PR titles follow Conventional Commit format (`feat:`, `fix:`, etc.).
  * PRs are **squash-merged** so `main` history is a clean sequence of meaningful commits.
  * A CI check enforces PR title format.

* **release-please as the release brain**

  * Watches `main` for new changes.
  * Opens a **single “Release PR”** when there are releasable changes.
  * On merging the Release PR:

    * Bumps version (at root).
    * Updates root `CHANGELOG.md`.
    * Creates a **Git tag** and a **GitHub Release**.

---

## 2. goreleaser build & attach to GitHub Release

* **Trigger**

  * GitHub Action triggered on:

    * `on: push: tags: [ 'v*' ]`
    * Or `on: release: types: [published]`
  * The tag / release was created by `release-please`.

* **goreleaser responsibilities**

  * Builds and uploads binaries + webapp bundle as **release assets**:

  1. `gridapi`:

     * Targets: `linux/amd64`, `linux/arm64`.
  2. `gridctl`:

     * Targets: `linux/amd64`, `linux/arm64`, `darwin/amd64`, `darwin/arm64`.
  3. `webapp`:

     * Runs `pnpm install`, `pnpm -C webapp build`.
     * Archives `webapp/dist` → e.g. `grid-webapp-vX.Y.Z.tgz`.
     * Attaches `.tgz` to the same GitHub Release.

* **Result**

  * One GitHub Release page per version with:

    * Release notes from `release-please`.
    * All CLI binaries.
    * Webapp static bundle.

---

## 3. JS SDK (`@tcons/grid`) publishing

* **Separate npm publish workflow**

  * Triggered by the same tag (e.g. `v*`) or by `release: published`.
  * Steps:

    1. `pnpm install` (with pnpm workspaces).
    2. `pnpm -C js/sdk build` (if there’s a build step).
    3. `pnpm -C js/sdk pack` (optional, to inspect the tarball).
    4. `pnpm -C js/sdk publish --access public` (using `NODE_AUTH_TOKEN` secret).

* **Changelog in the published package**

  * Ensure the JS package’s dist tarball includes changelog info:

    * Either:

      * Copy root `CHANGELOG.md` (or a generated per-package subset) into `js/sdk/` before publishing.
      * Or maintain a small `js/sdk/CHANGELOG.md` that `release-please` updates (symlink or copy step).
    * Add `files` or `include` config in `package.json` so the changelog file ends up in the published tarball.

---

## 4. Tests

You already have:

* `make test-integration` (no auth, only PG)
* `make test-integration-mode2` (internal IdP, PG only)
* `make test-integration-mode1` (Keycloak + PG, slower)

**Options for CI layout:**

* **Option CI1 – Matrix jobs per test mode**

  ```yaml
  jobs:
    integration:
      strategy:
        matrix:
          mode: [plain, mode2]
      services:
        postgres:
          image: postgres:16
          ports: ["5432:5432"]
          env:
            POSTGRES_PASSWORD: postgres
          # ...
      steps:
        - uses: actions/checkout@v4
        - uses: actions/setup-go@v5
        - run: make test-integration${{ matrix.mode == 'plain' && '' || '-mode2' }}
  ```

  * Each mode gets a fresh Postgres service.
  * Run on PRs and on pushes to main.
  * Optionally add Keycloak-based mode1 in a **separate workflow** or a scheduled job to avoid slowing every PR.

* **Option – Keep Keycloak only for “full e2e” / nightly**

> Note: Keycloak setup is slow, playwright e2e are not implemented yet.

  * For now, focus on:

    * `test-integration`
    * `test-integration-mode2`

  * Later full e2e with Playwright + Keycloak can be:

    * `workflow_dispatch`
    * `schedule: nightly`
    * Or only on main before cutting a release.

Highlevel:

   * PRs to main:

     * Job 1: `make test-integration`
     * Job 2: `make test-integration-mode2`
   * Possibly nightly / manual job that also runs Keycloak mode1 and, later, Playwright e2e.

## 5. Dependency updates

Your constraints:

* Monorepo (Go `go.work`, pnpm workspaces)
* Want grouped PRs
* Want auto-merge once **build + integration tests** pass

**Dependabot (with grouping)**

* Dependabot now supports grouped updates via `groups` config.
* You can:

  * Group Go module bumps
  * Group npm/pnpm bumps
* Combine with:

  * Required status checks: build + integration tests
  * `github-actions` bot or `automerge` rules after checks pass.

For example:
```yaml
version: 2
updates:
  - package-ecosystem: gomod
    directory: "/"
    schedule:
      interval: weekly
    open-pull-requests-limit: 10
    labels:
      - bot/merge
    commit-message:
      prefix: "chore: "
    groups:
      hashicorp:
        patterns:
          - "github.com/hashicorp/*"
      gomod:
        patterns:
          - "*"
        exclude-patterns:
          - "github.com/hashicorp/*"
  - package-ecosystem: "github-actions"
    directory: "/"
    schedule:
      interval: "weekly"
    labels:
      - bot/merge
    commit-message:
      prefix: "chore: "
    groups:
      github-actions:
        patterns:
          - "*"
```

## 4. Big-picture flow

1. You merge feature PRs into `main` (squash, conventional titles).
2. `release-please` opens a Release PR when there are changes.
3. You merge the Release PR:

   * Version bump + root changelog update.
   * Tag + GitHub Release created.
4. Tag / Release triggers:

   * **goreleaser** build → binaries + webapp `.tgz` attached to the Release.
   * **npm publish** workflow → `@tcons/grid` pushed to npm with matching version and included changelog.

Example PR title validation (enforces conventional commits, allows optional JIRA style issue keys, but should be converted to match beads grid-<sha> style keys):

```yaml
# Validates PR title follows conventional commits
on:
  pull_request:
    types:
      - edited
      - opened
      - synchronize
      - reopened

jobs:
  conventional_commit_title:
    runs-on: ubuntu-24.04
    steps:
      # source https://github.com/chanzuckerberg/github-actions/blob/cac0ba177b109becac01bc340a3a1547feb40fe5/.github/actions/conventional-commits/action.yml
      - uses: actions/github-script@v8
        with:
          script: |
            const validator = /^(chore|feat|fix|revert|docs|style|ci)(\((((PETI|HSENG|SAENG)-[0-9]+)|([a-z-]+))\))?(!)?: (.)+$/
            const title = context.payload.pull_request.title
            const is_valid = validator.test(title)

            if (!is_valid) {
              const details = JSON.stringify({
                title: title,
                valid_syntax: validator.toString(),
              })
              core.setFailed(`Your pr title doesn't adhere to conventional commits syntax. See more details: ${details}`)
            }
```
