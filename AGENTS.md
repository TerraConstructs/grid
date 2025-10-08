# Repository Guidelines

## Project Structure & Module Organization
Grid uses a Go workspace with module roots under `cmd`, `pkg`, `api`, and `tests`. `cmd/gridapi` hosts the API server with internal subpackages for config, db, repository, server, and state logic, while `cmd/gridctl` contains the CLI command tree. Shared Go libraries live in `pkg/sdk`, generated stubs in `api`, and TypeScript bindings under `js/sdk/gen`. Example Terraform projects sit in `examples`, demo assets in `demo`, and specs or UX notes in `specs/`. Keep generated code out of reviews; source changes belong in the proto or non-gen directories.

## Build, Test, and Development Commands
Use `make build` to compile `gridapi` and `gridctl` into `bin/`. Start the local Postgres dependency with `make db-up`, or reset everything via `make db-reset`. `./bin/gridapi serve --db-url "postgres://grid:gridpass@localhost:5432/grid?sslmode=disable"` runs the API; pair it with `./bin/gridctl state list --server http://localhost:8080` to exercise endpoints. Run `buf generate` whenever you touch files beneath `proto/`, and commit the updated Go and TypeScript artifacts.

## Coding Style & Naming Conventions
Go code follows standard `gofmt` output, tabs over spaces, and exports only when cross-package reuse is required. Keep package paths rooted under `cmd/.../internal` for private logic and `pkg/sdk` for reusable APIs. Name migrations `YYYYMMDDHHMMSS_description.go`, Terraform samples using hyphenated directories, and flags or config keys in kebab-case. TypeScript modules in `js/sdk` should remain ESLint-friendly with camelCase identifiers and default export avoidance.

## Testing Guidelines
Prefer `make test-unit` for fast feedback; it runs race-detected Go tests that avoid external services. Repository tests that hit Postgres live under `cmd/gridapi/internal/repository` and require `make test-unit-db` (which auto-starts Docker). End-to-end coverage is in `tests/integration`; run it with `make test-integration` after building binaries. Name new Go tests with the `TestXxx` pattern in `*_test.go`, and clean up transient fixtures using `t.Cleanup` or `make test-clean`. Contract tests in `tests/contract` are placeholders—note TODOs before adding new cases.

## Commit & Pull Request Guidelines
Follow the existing conventional prefixes (`feat:`, `fix:`, `chore:`, `docs:`) and keep messages imperative and under 72 characters. Reference related issues in the body and mention migrations, proto changes, or new binaries explicitly. PRs should include a concise summary, testing evidence (`make test-unit`, `make test-integration`, etc.), and screenshots or CLI transcripts when behavior changes. Request reviews from domain owners (`gridapi`, `gridctl`, SDK) and ensure generated artifacts and docs stay in sync with code changes.
