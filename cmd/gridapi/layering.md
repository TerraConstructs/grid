# Grid API Layering and Internal Package Structure

This document defines clear, strict layering rules for the Grid API server. The aim is to keep responsibilities crisp, enable safe refactors, and ensure tests verify behavior at the right seam.

## Principles

- Handlers do transport; Services do business logic; Repositories do persistence.
- Handlers and middleware should depend on Services, not on Repositories.
- Services are the only place that compose multiple repositories and enforce domain invariants.
- Authn/Authz concerns remain in middleware, but repository access for identity/session/role workflows should be encapsulated behind an Auth/IAM service where feasible.
- Tests should validate each layer in isolation and via the public seams (RPC/HTTP, services) for end-to-end confidence.

## Layers and Allowed Dependencies

1) Data Models

- Location: `cmd/gridapi/internal/db/models`
- Purpose: DB entities, value objects, constants. No business logic.
- Allowed deps: standard library only.

2) DB Providers & Migrations

- Location: `cmd/gridapi/internal/db/bunx`, `cmd/gridapi/internal/migrations`
- Purpose: DB connections, dialect specifics, migrations. No domain logic.
- Allowed deps: Models, ORM libs.

3) Repositories (DAL)

- Location: `cmd/gridapi/internal/repository`
- Purpose: CRUD and query composition; hide dialect quirks.
- Allowed deps: Models, DB providers.
- Not allowed: HTTP/Connect, server, middleware, services importing back into repos.

4) Services (Domain Logic)

- Location: `cmd/gridapi/internal/state`, `cmd/gridapi/internal/dependency`, `cmd/gridapi/internal/tfstate`, `cmd/gridapi/internal/graph`
- Purpose: Business rules, validation, orchestration across repositories and pure utils.
- Allowed deps: Repositories, Models, other services (cohesive), pure helpers.
- Not allowed: HTTP/Connect specifics.

5) Auth

- Location: `cmd/gridapi/internal/auth`
- Purpose: OIDC/JWT verification, claims, casbin setup, identifiers, actions.
- Recommendation: Add `iam` or `auth` service to encapsulate user/session/role/service-account workflows so handlers and CLI do not talk to repos directly.

6) Middleware

- Location: `cmd/gridapi/internal/middleware`
- Purpose: Request-scoped concerns (authn/authz). Extract resource attributes via Services. Avoid direct repository access for business state.
- Allowed deps: Auth (verifier/enforcer), Services, Config.

7) Server (Handlers)

- Location: `cmd/gridapi/internal/server`
- Purpose: HTTP/Connect handlers, request validation, mapping between proto/JSON and domain types. No repository usage.
- Allowed deps: Services, Auth (identities), Middleware types for wiring.

8) Commands (CLI)

- Location: `cmd/gridapi/cmd/*`
- Purpose: Wire config, DB, repositories, services, middleware, and start server. For admin commands, reuse Services rather than talking to repositories directly.
- Allowed deps: Config, DB provider, Repositories (wiring), Services, Server router. Avoid domain logic in commands.

## Package Mapping (Current)

- Models: `internal/db/models`
- DB provider: `internal/db/bunx`
- Migrations: `internal/migrations`
- Repositories: `internal/repository`
- Services: `internal/state`, `internal/dependency`, `internal/tfstate`, `internal/graph`
- Auth: `internal/auth`
- Middleware: `internal/middleware`
- Handlers/Router: `internal/server`
- CLI: `cmd/gridapi/cmd`

## Do/Don’t Examples

- Handlers must not import `internal/repository`. Use services.
- Middleware should query resource attributes via services, not repositories.
- CLI admin flows must not implement business flows or talk to repos directly; call a service.
- Service is the façade for business operations: label policy validation, state content writes, edge graph operations, auth/IAM workflows.

## Known Gaps (Targets for Refactor)

- Edge update job lives in `internal/server` and directly updates repos. Move to a service under `internal/dependency` (or a new `edgeupdater` service) and inject an interface into handlers.
- Connect auth handlers and SSO HTTP handlers manipulate repositories and casbin directly. Introduce an `iam` service and route handlers and CLI through it.

