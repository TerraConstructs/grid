| ID | Category | Severity | Location(s) | Summary | Recommendation |
  |----|----------|----------|-------------|---------|----------------|
  | D1 | Duplication | HIGH | specs/006-authz-authn-rbac/spec.md:341, specs/006-authz-authn-rbac/spec.md:343, specs/006-authz-authn-rbac/spec.md:443 | FR-011/FR-012 restate the same authorization gate already  captured by FR-037, creating redundant checkpoints. | Collapse FR-011/FR-012 into cross-references to FR-037 (or vice versa) so implementation and testing target one canonical requirement. |
  | I1 | Inconsistency | CRITICAL | specs/006-authz-authn-rbac/spec.md:674, specs/006-authz-authn-rbac/plan.md:410 | Spec mandates TLS for credential transport (FR-100) but the plan’s Keycloak bootstrap enables plain HTTP (KC_HTTP_ENABLED: "true", KC_HOSTNAME_STRICT_HTTPS: "false"). | Update the plan (and any related tasks/scripts) to honour HTTPS-only flows—e.g., terminate TLS in dev or document a secure proxy—before implementation proceeds. |
  | G1 | Coverage | HIGH | specs/006-authz-authn-rbac/spec.md:365, specs/006-authz-authn-rbac/tasks.md:200-237 | FR-023 (clear, non-leaking authz errors) lacks any explicit plan or task coverage; current tasks focus on middleware plumbing without response messaging rules. | Add concrete tasks to design and verify denial messaging (API + CLI + webapp), referencing FR-023 so developers know how to shape responses and logs. |
  | A1 | Ambiguity | MEDIUM | specs/006-authz-authn-rbac/spec.md:526 | FR-053 requires “fast validation” without defining acceptable latency despite prescribing cache refresh timing. | Specify an upper-bound metric for validation latency (e.g., <5 ms per request) or remove the adjective to avoid subjective acceptance debates. |
  | U1 | Underspecification | MEDIUM | specs/006-authz-authn-rbac/spec.md | Spec omits a Non-Functional Requirements section altogether, so performance, availability, and compliance expectations are scattered or absent. | Add an explicit Non-Functional section (even if some items reference existing FRs) to anchor operational/service-level expectations per template guidance. |

  Problem: FR-011/FR-012 restate the same authorization gate already captured by FR-037, creating redundant checkpoints.
  Action:  Collapse FR-011/FR-012 into cross-references to FR-037 (or vice versa) so implementation and testing target one canonical requirement.

  Problem: Spec mandates TLS for credential transport (FR-100) but the plan’s Keycloak bootstrap enables plain HTTP (KC_HTTP_ENABLED: "true", KC_HOSTNAME_STRICT_HTTPS: "false"). 
  Action:  Add spec note allowing local dev without TLS, Update the plan (and any related tasks/scripts)

  Problem: Spec omits a Non-Functional Requirements section altogether, so performance, availability, and compliance expectations are scattered or absent.
  Action:  Add an explicit Non-Functional section (even if some items reference existing FRs) to anchor operational/service-level expectations per template guidance.

  Problem: FR-053 requires “fast validation” without defining acceptable latency despite prescribing cache refresh timing.
  Action:  Specify an upper-bound metric for validation latency (e.g., <5 ms per request) or remove the adjective to avoid subjective acceptance debates.

  Problem: FR-023 (clear, non-leaking authz errors) lacks any explicit plan or task coverage; current tasks focus on middleware plumbing without response messaging rules.	
  Action:  Add concrete tasks to design and verify denial messaging (API + CLI + webapp), referencing FR-023 so developers know how to shape responses and logs.


  - 66 of 97 tasks lack any FR/requirement reference; high-impact examples include T001 (schema migration), T010 (JWT verifier), and T020–T025 (OIDC HTTP handlers) in specs/006-authz-authn-rbac/tasks.md:91-236. Add requirement tags to maintain traceability.

# TODOS:

- [ ] Add Authz verifications on all connect_handlers (including new ones added for service account management)

- [ ] 1a. Add logic to the RevokeServiceAccount RPC handler to remove all Casbin role assignments for the service account, ensuring its permissions are fully purged from the authorization system.

- [ ] T009B mentions that AuthN middleware automatically creates session in authn middleware (T017), but this is not the case for Service Accounts (as highlighted in Plan .. need to double check that)

# Review notes

## Internal IdP key management

Based on the current implementation in cmd/gridapi/internal/auth/oidc.go, a new RSA signing key is generated in memory every time the apiserver starts.

When the server restarts, the old signing key is lost, and a new one is created. Any tokens that were signed with the old key can no longer be verified because the corresponding public key
is no longer available from the server's JWKS endpoint.

Therefore, all previously minted tokens become invalid upon an apiserver restart. For a production environment, these signing keys should be persisted and loaded on startup rather than
being regenerated each time.

```golang
func newProviderStorage(deps ProviderDependencies) (*providerStorage, error) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048) // in memory key
	if err != nil {
		return nil, fmt.Errorf("generate signing key: %w", err)
	}
  ...
```

## External IdP SSO login

When a user clicks "Login," the following happens:

1. The Grid (as a Relying Party) needs to send the user to the external IdP.
2. To prevent CSRF attacks, it generates a random state value.
3. To use the secure PKCE flow, it generates a code_verifier.
4. The Grid needs to remember the state and code_verifier when the user is redirected back from the IdP a few seconds later.
5. The zitadel/oidc library stores these values in a browser cookie.

These keys are used to sign and encrypt that cookie.

- hashKey (HMAC): Creates a signature to ensure the cookie's contents haven't been tampered with in the browser.
- cryptoKey (AES): Encrypts the cookie's contents so the user (or a browser extension) cannot read sensitive values like the code_verifier.

This is a transient, short-lived security mechanism to protect the integrity of a single login attempt.

> [!IMPORTANT]
> Production-Ready Recommendation
>
> Generating keys on startup is fine for local development. However, in a production environment with multiple server instances or restarts, this will cause problems (in-flight logins will fail if a server restarts with a new key).
> The best practice is to generate these keys once and store them as persistent application secrets.

Production hash keys:

```bash
# Generate keys once and store them in your .env or Kubernetes secret
echo "COOKIE_HASH_KEY=$(openssl rand -base64 32)"
echo "COOKIE_CRYPTO_KEY=$(openssl rand -base64 32)"
```
## Session Management

Deferred tasks T042 (ListSessions) and T043 (RevokeSession) in specs/006-authz-authn-rbac/tasks.md currently lack proper authorization checks in the authz_interceptor.go file. The current implementation of the authz_interceptor handles ListSessions and RevokeSession with a static permission check:

```golang
  1 case statev1connect.StateServiceListSessionsProcedure, statev1connect.StateServiceRevokeSessionProcedure:
  2     objType = auth.ObjectTypeAdmin
  3     action = auth.AdminSessionRevoke
```

This requires the auth.AdminSessionRevoke permission for all session management, which is incorrect. It fails to account for the requirement in T086 that a user should be able to revoke their own sessions
without being an admin.

Here is how the interceptor should handle these tasks correctly:

For RevokeSession (T043)

The interceptor needs to perform a dynamic check:

  1. It will inspect the incoming RevokeSessionRequest to get the ID of the session being revoked.
  2. It will use the SessionRepository to fetch that session from the database and identify its owner (user_id).
  3. It will compare the session's owner ID to the ID of the currently authenticated user (principal.InternalID).
      * If they match, the user is revoking their own session. The interceptor will allow the request to proceed without checking for admin permissions.
      * If they do not match, the user is attempting to revoke someone else's session. The interceptor will then proceed to check if the user has the auth.AdminSessionRevoke permission using the Casbin
        enforcer. If they don't, the request is denied.

For ListSessions (T042)

The logic is similar. The interceptor will inspect the ListSessionsRequest:
  1. If the request asks for the sessions belonging to the currently authenticated user, it should be allowed.
  2. If the request asks for another user's sessions, it should only be allowed if the caller has the auth.AdminSessionRevoke permission.

My previous implementation was too simplistic. I will correct the authz_interceptor.go file to implement this more detailed, resource-aware authorization logic. This will require adding the
SessionRepository to the AuthzDependencies so the interceptor can perform these lookups.

## Logic on permissions

```
message EffectivePermissions {
  repeated string roles = 1; // Role names
  repeated string actions = 2; // Aggregated actions (e.g., ["state:*", "tfstate:read"])
  repeated string label_scope_exprs = 3; // List of go-bexpr expressions from all roles (union/OR semantics - access granted if ANY expression matches)
  optional CreateConstraints effective_create_constraints = 4;
  repeated string effective_immutable_keys = 5; // Union of all immutable keys
}
```

This seems to decouple the role -> action -> labelScopeExprs into 3 sets.
This means that if a user has 2 roles, one with `state:read` and labelScopeExpr `env=prod` and another role with `tfstate:read` and labelScopeExpr `env=dev`, you can't tell really what the user can do.
(can they read state in prod and tfstate in dev? or can they read state and tfstate in both envs?)

Is that the correct behavior as for how Casbin works with a model matcher:

```
[matchers]
m = g(r.sub, p.role) && r.objType == p.objType && r.act == p.act && bexprMatch(p.scopeExpr, r.labels)
```

Doesn't this mean access is only allowed if the user has any role that allows the specific action on the specific object type with the specific label scope expression (but they must be in a tuple...).

## Persistence issues

Fix: Persist auth requests, tokens, and device codes in the database (they’re in-memory today, so restart loses state).

note, sample oidc sample storage implementation:
https://github.com/zitadel/oidc/blob/v3.45.0/example/server/storage/storage.go

Violation:

- The specs explicitly call for durable storage: FR-003a mandates using zitadel/oidc “with Grid supplying only deployment-specific configuration and session persistence” (specs/006-authz-authn-rbac/spec.md:321-323). FR-005/FR-007 then rely on those persisted sessions so issued tokens remain valid across restarts and can later be revoked (specs/006-authz-authn-rbac/spec.md:326-333).
- The implementation plan reinforces this by requiring op.Storage adapters backed by our repositories “(clients, users, sessions, device codes)” and by wiring handlers to persist Grid session rows (specs/006-authz-authn-rbac/tasks.md:114-118, specs/006-authz-authn-rbac/research.md:381-394).
- Leaving auth requests, device codes, or issued tokens in memory would violate those requirements—restart would orphan in-flight device flows and make revocation/state tracking impossible—so yes, persistence in the database is a functional requirement.

Original tasks only mentioned Bun Models for
- users
- service_accounts
- roles
- user_roles
- group_roles
- sessions
- casbin_rules

Missing:
- auth requests
- tokens
- device codes

See oidc.go:

```golang
		authRequests:    make(map[string]*authRequest),
		authCodes:       make(map[string]string),
		tokens:          make(map[string]*token),
		refreshTokens:   make(map[string]*refreshToken),
		deviceCodes:     make(map[string]deviceAuthorizationEntry),
		userCodes:       make(map[string]string),
```

More violations:
  1. [x] Wire the session repository into the storage callbacks to satisfy FR‑005/FR‑007/FR‑098/FR‑099.
  2. [ ] Wire the audit logging hooks into the storage callbacks to satisfy FR‑005/FR‑007/FR‑098/FR‑099.
  3. [ ] Expand client modelling beyond service accounts (interactive/web clients still need proper redirect URIs and login flows).

## ORM adoption

Potential candidates for code improvements during SQLite epic.

1. Bun relationships: Why are FK constraints not created through Bun relationships
Ref: https://bun.uptrace.dev/guide/golang-orm.html#table-relationships

2. Bun Audit Trail pattern: For AuthN / AuthZ activities
ref: https://bun.uptrace.dev/guide/models.html#audit-trail-pattern

3. Bun Validation and Constraints: Could this improve code maintainability and cross driver compatibility?
ref: https://bun.uptrace.dev/guide/models.html#validation-and-constraints

4. Detecting Database engine features: For SQLite feature spec
ref: https://bun.uptrace.dev/guide/drivers.html#writing-database-specific-code

