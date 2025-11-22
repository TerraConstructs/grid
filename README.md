# Grid

[![PR Tests](https://github.com/TerraConstructs/grid/actions/workflows/pr-tests.yml/badge.svg)](https://github.com/TerraConstructs/grid/actions/workflows/pr-tests.yml)
[![Release](https://github.com/TerraConstructs/grid/actions/workflows/release-please.yml/badge.svg)](https://github.com/TerraConstructs/grid/actions/workflows/release-please.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/TerraConstructs/grid/cmd/gridapi)](https://goreportcard.com/report/github.com/TerraConstructs/grid/cmd/gridapi)

Grid is a remote Terraform/OpenTofu state service paired with a friendly CLI that automates backend configuration, dependency wiring, and collaborative workflows.

## Highlights
- Remote state API (`gridapi`) that speaks the Terraform HTTP backend protocol and persists state in PostgreSQL
- Directory-aware CLI (`gridctl`) that creates states, generates backend config, and manages cross-state dependencies
- Dashboard to visualize states and their dependencies (`webapp`)
- Go and TypeScript SDKs for programmatic access and automation

## Demo
![Grid CLI demo showing dependency sync](demo/demo.gif)

The animated demo captures the full workflow: creating two Terraform states (`network` and `cluster`), wiring a dependency, syncing managed HCL, and listing states/dependencies. Regenerate it any time with:

```bash
./record.sh # from repo root
```

> [!IMPORTANT]
> Requires `vhs` installed (see demo/README.md).

## Quickstart

```bash
# Build CLI and API binaries
make build

# Start dependencies (PostgreSQL via Docker Compose)
make db-up

# Run database migrations
make db-migrate

# Run the API server without AuthN/AuthZ (defaults: :8080, DATABASE_URL env var)
./bin/gridapi serve --db-url "postgres://grid:gridpass@localhost:5432/grid?sslmode=disable" &

# Use gridctl against the running server
./bin/gridctl state list --server http://localhost:8080

# Dashboard (webapp)
cd webapp
pnpm install
pnpm dev
```

Grid CLI commands read `.grid/` context in your working directory so repeated calls automatically target the same state. Override with explicit flags whenever needed.

## Installation

### Pre-built Binaries

Download the latest release for your platform from the [Releases page](https://github.com/TerraConstructs/grid/releases):

- **gridapi**: `linux/amd64`, `linux/arm64`
- **gridctl**: `linux/amd64`, `linux/arm64`, `darwin/amd64`, `darwin/arm64`
- **webapp**: Single `.tar.gz` bundle

### npm Package

Install the JavaScript SDK:

```bash
npm install @tcons/grid
# or
pnpm add @tcons/grid
```

### Build from Source

```bash
# Build CLI and API binaries
make build

# Binaries will be in ./bin/
./bin/gridapi --help
./bin/gridctl --help
```

## Repository Layout

```
.
├── cmd/gridapi/   # API server entrypoint
├── cmd/gridctl/   # CLI commands
├── pkg/sdk/       # Go SDK
├── js/sdk/        # Generated TypeScript SDK (WIP)
├── examples/      # Terraform sample projects
├── demo/          # VHS scripts and rendered assets
├── webapp/        # Web application for the dashboard
└── specs/         # Product specifications and UX docs
```

## Beads Usage

Beads is a issue tracking tool for Humans and AI that integrates with git. See [steveyegge/beads](https://github.com/steveyegge/beads).

### Git Hooks

Git Hooks for beads handle SQLite to JSONL flushing before commits to avoid dirty working trees. See [git-hooks/README.md](git-hooks/README.md) for details.
