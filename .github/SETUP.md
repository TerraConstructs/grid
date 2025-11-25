# Release Please Setup

This document outlines the steps to bootstrap `release-please` for this repository.

## Prerequisites

1.  Install `release-please` globally:
    ```bash
    npm i release-please -g
    ```

2.  Log in to GitHub with the `gh` CLI and ensure you have `repo` scopes. You can check this with `gh auth status`. Then, export your token as an environment variable:
    ```bash
    export GITHUB_TOKEN=$(gh auth token)
    ```

## Bootstrapping

Run the following command to bootstrap `release-please`. This will create the initial `release-please-config.json` and `.release-please-manifest.json` files and open a pull request.

```bash
release-please bootstrap \
  --token=$GITHUB_TOKEN \
  --repo-url="TerraConstructs/grid" \
  --release-type=simple
```

After the bootstrap PR is merged, `release-please` will be configured to run on every push to `main`.

```
