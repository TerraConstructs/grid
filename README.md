# Grid

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

# Run the API server (defaults: :8080, DATABASE_URL env var)
./bin/gridapi serve --db-url "postgres://grid:gridpass@localhost:5432/grid?sslmode=disable" &

# Use gridctl against the running server
./bin/gridctl state list --server http://localhost:8080
```

Grid CLI commands read `.grid/` context in your working directory so repeated calls automatically target the same state. Override with explicit flags whenever needed.

## Repository Layout

```
.
├── cmd/gridapi/   # API server entrypoint
├── cmd/gridctl/   # CLI commands
├── pkg/sdk/       # Go SDK
├── js/sdk/        # Generated TypeScript SDK (WIP)
├── examples/      # Terraform sample projects
├── demo/          # VHS scripts and rendered assets
└── specs/         # Product specifications and UX docs
```
