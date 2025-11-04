read @specs/006-authz-authn-rbac/tasks.md - specifically the refactoring work for T009A to not use opaque tokens for service accounts (going forward we issue JWT tokens for Service Accounts in internal IdP mode as well and for revocation we track jti (together with the token expiry time so we can clean up) in a database table to handle JWT Revocations (JWT are stateless, original plan was to have opaque tokens (session GUID) and use session table to track active sessions (See T009), this was stateful and caused issues due to the way the op.Provider was encrypting and storing the token vs what was sent back to the CLI to use.. )


Why JWT for service accounts (with zitadel/oidc OP)

zitadel/oidc’s OP supports client credentials if your storage implements the right interface; in that case the access token should be a JWT (not opaque). That’s exactly what you want for stateless verification and Chi middleware simplicity.

Also, ZITADEL docs emphasize using Web Keys (JWKS) to sign & verify tokens; you expose your jwks_uri so GridAPI and clients can validate without server affinity.
https://zitadel.com/docs/guides/integrate/login/oidc/webkeys#web-key-management

1) External IdP (Keycloak/Entra/Okta)

- RP only: do OIDC discovery, validate JWTs via IdP jwks_uri, verify iss/aud/exp/iat.
- Map claims to your principal (configurable paths for sub, email, groups).
- No sessions required for service accounts; they’ll just present IdP-issued JWTs.

2) Internal IdP (your lightweight OP)

Use zitadel/oidc OP to host:

- Authorization Server endpoints (only those you truly need).
- Token endpoint supporting Client Credentials for service accounts.
- JWKS endpoint (public keys) so your own API can validate JWTs statelessly.

Key point from the package docs: if your Storage implements the client-credentials storage, the access token returned by `CreateAccessToken` is a JWT (not opaque).

We worked to refactor oidc.go following T009A, but oidc.go had invalid identifiers before proceeding to T010A and T017A

Please review the logic on how to handle service accounts for "Internal IdP mode of the gridapi server"

Read more about the `op.Provider` struct and its fields/methods:
- grep local go modules dir: `grep -A23 "type Provider struct {" /Users/vincentdesmet/go/pkg/mod/github.com/zitadel/oidc/v3@v3.45.0/pkg/op/op.go`
or
- web fetch https://raw.githubusercontent.com/zitadel/oidc/refs/tags/v3.45.0/pkg/op/op.go (use grep to parse through sections) to stop stabbing around)

Here are observations takeaways after actually reading the zitadel/oidc framework interfaces - please confirm my understanding and proceed to fix the code in oidc.go accordingly:
   1. `op.Provider` does not have `Signer()` or `Issuer()` methods. The issuer is an unexported function, and signing is handled by the unexported crypto field.
   2. `op.Sign` is a package-level function, not a method. Its signature is op.Sign(ctx, claims, signer).
   3. The `op.Signer` interface is the key. I need an object that implements this interface to pass to op.Sign. The Signer interface is likely implemented by the provider's internal Crypto object.
   4. `oidc.IDTokenClaims` does not have a `TokenHeader` field. The claims should be set directly on the struct (e.g., Issuer, Subject, JWTID).
   5. `request.GetClientID()` does not exist on the `op.TokenRequest` interface. I need to get the client ID through other means, likely by type-asserting the request to a more specific type like op.AuthRequest.


The previous approach was incorrect because it was trying to use fields and functions (`Issuer` on `IDTokenClaims`, `op.Sign`).

Review if `go-jose` library should be used directly for JWT creation.
Suggested plan to fix `cmd/gridapi/internal/auth/oidc.go`:
 1.  **Correct JWT Claims Creation**: I will properly initialize the `oidc.IDTokenClaims` struct, embedding the standard `jwt.Claims` from the `go-jose` library for fields like `Issuer`, `Subject`, and `ID` (for the JTI).
 2.  **Use `go-jose` for Signing**: I will replace the incorrect call to `op.Sign` with the correct method for creating and signing a JWT using the `go-jose/v4` library directly, which is already a dependency. This will use the `signingKey` already present in the `providerStorage`.
 3.  **Implement Revocation Logic**: I will add the missing `revokeSessionByJTI` function to handle JWT revocation by looking up the session in the database using a hash of the token's JTI.
 4.  **Fix Dependent Functions**: I will implement the `TODO`s in `SetUserinfoFromToken` and `SetIntrospectionFromToken` to correctly fetch session information using the JTI from the database, making them compatible with stateless JWTs.
 5.  **Type Correction**: I will change the `signer` field in `providerStorage` from the non-existent `op.Signer` to the correct `op.Crypto` type.

After the file is modified, use IDE tools to detect problems or verify compile errors with `go build` on it.


## New Revocation model that makes sense (and keeps JWTs stateless)

- Include a jti claim on every token. Keep a DB table revoked_jti(jti, exp) (KISS/YAGNI, no in-memory LRU, only Db table and migrations for now)
- On each request: validate signature/standard claims → check jti against the denylist → proceed.
- Admin “revoke session/token” actions just insert jti into this table. (You already have a session table; simply store the jti there and mark it revoked.)
- Emergency “nuke all” = rotate signing key (will invalidate all future verifications until clients refresh keys), which is consistent with JWKS behavior. 

This gives you immediate revocation without making every request stateful.

---

## How to wire `zitadel/oidc` OP for JWT signing (service accounts)

Conceptually (leaving out low-level type names that may shift across minor versions):

NOTE: Only lazy load JWKS when self hosting OIDC OP; if using external IdP, just fetch their JWKS.

1. **Key / JWKS setup**

   * Provide a key storage that returns your active signing key(s).
   * Expose **`.well-known/openid-configuration`** and **`/jwks`** (the OP helpers do this) so validators fetch keys.

2. **Implement the OP storage interfaces**

   * Implement the required storage for clients (service accounts), scopes, and **client-credentials** support.
   * Ensure your storage returns the service account client, allowed scopes, and signals that **client_credentials** is enabled. The OP will then call your **CreateAccessToken** path, and **that token should be a JWT**. ([Go Packages][1])

3. **Create JWT access tokens**

   * In your access-token creation hook, mint a JWT with:

     * `iss` = your OP issuer URL
     * `sub` = `sa:<clientID>`
     * `aud` = GridAPI audience (and additional audiences if needed)
     * `exp/iat/nbf`
     * `jti` = random ID (store in your session/issuance table)
     * optional: `scope`, `groups`, or a compact `roles`/`permissions` claim if you want the token to carry anything beyond scopes
   * Sign with your current private key (alg per your key).

4. **Expose the token endpoint for service accounts**

   * Support **Client Credentials** grant: `client_id` + `client_secret` (or private key JWT if you want stronger auth); the ZITADEL docs cover “private key JWT” if you prefer mTLS/PKJWT. ([zitadel.com][5])

5. **Validation on GridAPI routes**

   * For Control/Data Plane, your Chi middleware does:

     1. Discover (or cache) OP config & JWKS.
     2. Verify signature + `iss/aud/exp` (+ `kid` presence).
     3. Extract claims via your **configurable mapping** (JSONPath-like or dot-path).
     4. Check **denylist by `jti`** for revocation.
     5. Build principal (user/service account) → pass to RBAC layer.

---

## About “sessions” now

* **Web UI**: keep cookie sessions (scs) if you want UX comfort. That’s independent from service-account JWTs.
* **Service accounts**: **do not** use opaque tokens or session lookups on each request. Just keep an issuance log (with `jti`) for auditing and revocation.

---

## Verify we have removed:

* All the “exceptions” in the middleware to let opaque tokens through.
* The token→session lookup path on every request.
* The “fake public client” device flow for SAs.

Replace with:

* Pure JWT validation for every service-account request.
* Optional **one** DB/cache check for `jti` revocation after signature verification.

## Quick reference

* `zitadel/oidc` OP storage note: **client-credentials ⇒ access token should be JWT** (not opaque). 
* Web Keys / JWKS behavior and key rotation.
* Service user auth with private key JWT (optional stronger client auth).

* **Don’t drop sessions entirely.**
  Drop *opaque access tokens* and any per-request **session lookup** for service-account/API traffic.
  **Keep a sessions table** for:

  1. **Web (human) logins** via external IdP (cookie session UX, logout, “view my sessions”, admin revoke), and
  2. **Revocation/indexing of JWTs** by `jti` (denylist), plus auditing.

* **Session RPCs stay**, but get scoped to:

  * Human web sessions (admin list/revoke by user).
  * Optional: service-account token issuance listing/revocation by `jti` (not per-request auth).

## How this fits the web (external IdP) story

### Recommended split by traffic type

1. **API/CLI/Tofu/Terraform (machine & CLI):**

   * **AuthN**: Bearer **JWT** (from external IdP in “external mode”, or from your internal OP in “internal mode”).
   * **AuthZ**: Your RBAC using context data about principal.
   * **Revocation**: Check `jti` against a **denylist** (DB + memory cache). No session lookup on the request path.

2. **Web UI (humans) with external IdP:**

   * Use **OIDC Authorization Code + PKCE** with the external IdP.
   * On callback, **verify tokens**, then create a **server-side session** (cookie via SCS or equivalent).
   * Store minimal identity in the session (user id `sub`, email, groups snapshot, issued_at, CSRF).
   * API routes called from the browser can authenticate via this **cookie session** (no bearer token needed from the browser).
   * **Logout/revoke** = delete the session row(s) and cookie — immediate effect for web traffic.

### Why you still want the sessions table

* **User experience**: “Where am I logged in?”, “Log out of other devices”, admin lockout, etc.
* **Security control**: Fast web logout/revocation without waiting for IdP token expiry.
* **Auditability**: When/where sessions were created, last seen, user agent/IP.
* **JWT revocation**: Keep an **IssuedTokens / Sessions** (or just `revoked_jti`) table to allow immediate deny of specific JWTs by `jti` while keeping JWT verification stateless.

### What changes vs your earlier attempt

* **No opaque service tokens**. All service-account access uses **JWT access tokens**.
* **No per-request DB hop** for service-account traffic. Only one quick **`jti` denylist** check after signature/claims verification.
* **Sessions are not an AuthN backbone for APIs** — they are **only** for:

  * **Web** cookie auth, and
  * **Revocation/audit** of issued JWTs.

## Concretely: what to keep/add vs drop

### Keep/Add

* **sessions table** (rename/reshape if helpful)
* **issued_tokens / revoked_tokens** (or fold into sessions):
  * `jti`, `subject` (user/sa), `aud`, `exp`, `issued_at`, `revoked_at`, `revoked_by`.
  * In-memory **LRU** of revoked `jti` for fast checks (if done simple in golang thread safe LRU cache, else mark for future according to KISS/YAGNI).
* **Session RPCs**:
  * Admin versions to list/revoke by user.
  * `ListMySessions`, `RevokeMySession(session_id)` (web users) (tag for future task if not in plan yet)
  * Optionally: `ListIssuedTokens(principal)` and `RevokeTokenByJTI(jti)` (service accounts).

### Drop (or refactor away)

* Opaque access tokens + middleware “exceptions” to let them pass.
* Request-path **session lookups for service-account traffic**.
* “Fake public client + device flow for SAs.” (already removed?)

## Middleware chain (how it runs)

**API routes (Control/Data Plane):**

```
Bearer JWT → Verify (iss/aud/exp/nbf + JWKS) → Check jti denylist → Build Principal → RBAC
```

**Web UI routes:**

```
Cookie → SCS session load → Principal from session → RBAC
```

(web app can call the same backend; cookie auth covers it. No need to expose IdP tokens to the browser or stuff them in localStorage.)

## Using zitadel/oidc OP for JWT creation (internal mode)

* Implement OP storage for **Client Credentials** (service accounts).
* Mint **JWT access tokens** with `iss`, `sub = "sa:<client_id>"`, `aud`, `exp`, **`jti`**, optional `scope/roles`.
* Publish **JWKS**; validators verify signature and claims.
* On admin revoke: insert `jti` into the denylist table; subsequent requests fail immediately.

## Notes

* review plan for IdP refresh tokens for web users, mark as future task if not present
  Optional. If you want long, smooth sessions in the UI, store refresh tokens server-side (encrypted) and rotate per OIDC best practices. If you don’t need it yet, keep sessions short and re-login when expired.

* if the IdP disables a user:
  - existing **web** sessions continue until they expire or they are revoked. 
  - existing **JWTs** continue until their `exp` unless their `jti` is denylisted (can also force-logout by nuking web sessions).

* If needed to unify web session “logout everywhere” and token revocation (mark as future task if not present): revoke all web `session_id`s for the user and insert all active JWT `jti`s for that subject into the denylist.

---

### Bottom line

* **Keep sessions**, but **only** for web UX + admin control + auditing.
* **Use JWTs everywhere else** (service accounts & CLI/API), with a **`jti` denylist** for revocation.
* Your AuthN/AuthZ path becomes simpler, faster, and aligns with your spec’s stateless goals without sacrificing control for the UI.
Sly here — let’s lock a clean, **one-webapp / two-backend-modes** story that doesn’t fork your UI code.

# The simple rule

**The browser never needs a JWT.**
Regardless of backend mode, the web app authenticates via a **server-side cookie session** set by GridAPI. JWTs are for CLI/CI and `/tfstate/*`, not for the browser.

# Two backend modes, one browser experience

## Mode A — Internal IdP (no external provider)

* **Humans (web):** Local username/password → GridAPI creates a **cookie session** (SCS) → browser calls Control Plane APIs with that cookie.
* **Machines (CLI/CI):** Service Account **client-credentials** → GridAPI OP issues **short-lived JWT** → used for Control/Data Plane.
* **Web never handles JWT.** It just sends the session cookie.

### Web flow

1. User hits `/app` → 401 → redirect to `/auth/login` (local form).
2. POST credentials → GridAPI creates DB session, sets `Set-Cookie: sessionid=…; HttpOnly; Secure; SameSite=Lax`.
3. Browser calls APIs with the cookie; server middleware resolves the principal from session.
4. Logout = delete session row + clear cookie (instant).

---

## Mode B — External IdP (Keycloak/Entra/Okta)

* **GridAPI acts as RP** using **Auth Code + PKCE**.
* On callback, GridAPI validates IdP tokens **server-side** and then **creates a cookie session** (just like internal mode).
  You can optionally store/refresh the IdP refresh token server-side, but the browser still uses only the cookie.
* **Machines (CLI/CI):** Either use the external IdP’s tokens directly or (simpler) use GridAPI-issued SA JWTs if you keep SAs in Grid.

### Web flow

1. User hits `/app` → 401 → redirect to `/auth/login` → GridAPI redirects to IdP.
2. IdP auth completes → callback to `/auth/callback` on GridAPI.
3. GridAPI validates tokens, builds principal, **creates cookie session** (`sessionid`), optionally stores IdP refresh token encrypted server-side.
4. Browser continues with cookie; no JWT in JS.

---

# Why this unifies your UI

* **Identical browser behavior** in both modes: the web app just checks “am I authenticated?” and calls APIs; the cookie/session does the rest.
* **No SPA token plumbing** (no localStorage, no `Authorization` header from the browser).
* **CSRF is solvable**: only needed for cookie-auth’d **mutating** Control Plane routes; send a CSRF token as a cookie/header pair. `/tfstate/*` remains **JWT-only** (no CSRF).

# What about `/tfstate/*` from the web?

Don’t. Keep `/tfstate/*` **JWT-only** and accessed by CLI/CI.
If you must display state blobs in the UI, have the **server** (cookie-auth’d) **proxy/serve** the needed data from your datastore — do not make the browser fetch `/tfstate/*` directly.

# Web app implementation checklist (mode-agnostic)

* [ ] On 401 from API, redirect to `/auth/login` (GridAPI decides: local form vs IdP redirect).
* [ ] After login, rely on the **session cookie** (HttpOnly, Secure, SameSite=Lax).
* [ ] Show user identity from `/me` (name/email/roles) returned via the session principal.
* [ ] CSRF token endpoint (e.g., `GET /auth/csrf`) for cookie-auth’d writes; attach header `X-CSRF-Token`.
* [ ] `POST /auth/logout` clears cookie + kills session server-side.
* [ ] Error handling: 401 → login; 403 → show “not authorized”; 404 for out-of-scope items.

# GridAPI middleware shape

* **Control Plane routes (used by web & CLI):**
  `authn: cookie-session **OR** bearer-jwt → build Principal → RBAC`
* **Data Plane `/tfstate/*`:**
  `authn: bearer-jwt only → RBAC (+ lock rules)`
* **CSRF:** only on cookie-auth’d mutating Control Plane routes.

# Config toggles you’ll expose

* For internal: enable local login endpoints.
* For external: set OIDC discovery URL, client_id, client_secret, allowed redirect URIs.
* Common: session cookie name, domain, idle/absolute timeouts; whether to store IdP refresh tokens server-side.

# FAQ

* **Do users “get a JWT” in external mode?**
  They do, but **GridAPI consumes it server-side** on callback and then issues a **session cookie** to the browser. The browser never needs the JWT.
* **Can we share code between modes?**
  Yes — the web app uses the same auth guard logic (cookie presence), the same `/me`, `/logout`, and the same API calls.

# Acceptance snippets

* **Internal mode login**: entering correct local credentials results in session cookie; API calls succeed without `Authorization` header.
* **External mode login**: redirect to IdP; after callback a session cookie exists; API calls succeed; no JWT in front-end memory.
* **CLI path**: `gridctl login --service-account` obtains JWT; `gridctl tf plan` works; web unaffected.
* **/tfstate/***: browser requests without JWT → 401; CLI with JWT → OK.

Bottom line: one webapp, one cookie-session UX; GridAPI abstracts whether the identity came from local users (internal) or an external IdP. JWTs stay where they shine: automation and `/tfstate/*`.
Great questions. Here’s the crisp, sanity-checked model that keeps things simple and consistent with your spec.

## 1) Are we “re-adding users” to internal mode?

Future tasks for webapp adoption with internal IdP

**Yes, but very minimal — not a full IdP.**
In **internal mode** you add a **local human user store** only to support the **web UI** with cookie sessions. You do **not** need to run a full OIDC OP for humans.

* **Humans (web):** local users → username/password (later optional TOTP/magic-link) → server-side **cookie session**.
* **Machines (CLI/CI & /tfstate):** **service accounts** → **client credentials** → short-lived **JWT** access tokens signed by your internal OP.

So: internal OP signs **service-account JWTs** only. Humans do **not** go through the internal OP; they use classic web login (cookie).

* If you later want **human CLI login** with no browser, that’s when you’d add device flow — optional.

## 3) How does the web app auth flow work (internal mode, local users)?

A standard cookie session flow — simple and robust.

### Minimal endpoints

* `GET /auth/login` – serve login page (or JSON if you prefer API-first).
* `POST /auth/login` – verify credentials, create server-side session, set `HttpOnly; Secure; SameSite=Lax` cookie.
* `POST /auth/logout` – delete session row, clear cookie.
* (Optional) `POST /auth/register` and `POST /auth/forgot-password` for tiny teams without external IdP.
* (Optional) `POST /auth/magic-link` if you prefer passwordless.

### Data you need

* **users**: `id`, `email`, `password_hash`, `created_at`, `disabled_at` (and optional `totp_secret`).
* **sessions** (web): `session_id`, `user_id`, `created_at`, `last_seen_at`, `ip`, `ua`, `revoked_at`.
* **revoked_jti** (JWT denylist): `jti`, `subject`, `exp`, `revoked_at`, `revoked_by`.

### Middleware chain

* **Web UI routes (browser):**
  `scs.LoadAndSave → cookie-session-auth → build Principal(user) → RBAC`
* **Control Plane API (shared):**
  `cookie-session-auth **OR** bearer-jwt-auth → Principal(user|service) → RBAC`
* **Data Plane /tfstate:**
  `bearer-jwt-auth → Principal(service|user) → RBAC (+ lock rules)`

  * Recommend requiring **JWT only** here to keep it stateless.

### CSRF

* Apply CSRF protection to **cookie-auth’d mutating** Control Plane routes.
* Not needed for `/tfstate/*` (bearer only) or read-only GETs.

### RBAC + label scope

* Same enforcement after principal build.
* For web users, roles resolve via your DB (direct assignments and/or group mappings if you later add them).
* For service accounts, roles resolve by `sa:<client_id>`.

## 4) Why this is simpler (and matches your goals)

* The browser talks directly to GridAPI using cookies; CLI/CI talks with JWT.
* **Stateless where it matters.** `/tfstate/*` stays stateless (JWT + `jti` denylist).
* **Immediate revocation paths.**
  * Web: delete session row(s) → instantly logged out.
  * JWT: insert `jti` into denylist → instantly rejected on next request.
* **No secrets in the browser.** Service account client secrets never touch front-end code.

## 5) Quick acceptance checklist (to wire against FRs)

* Web unauthenticated → redirect to `/auth/login` (FR-071).
* Login creates cookie session; subsequent UI/API calls succeed with **cookie only** (FR-072–077).
* CLI uses `gridctl login --service-account` to fetch **JWT**; `gridctl tf` injects it (FR-097a-l + earlier FRs).
* `/tfstate/*` rejects cookie-only requests with 401; requires JWT (FR-057–059).
* Admin “revoke session” removes web cookie session; admin “revoke token” adds `jti` to denylist (FR-007, FR-102a).

## 6) Optional future niceties (when you care)

* Add **TOTP** for local users.
* Add **magic links** for passwordless local login.
* Add **refresh tokens** server-side for longer web UX (still cookie-based; no tokens in JS).
* If you later want a single consistent protocol for humans too, you can promote the internal OP to issue **OIDC auth-code** tokens for the web and convert them to a cookie session on callback — but you don’t need that now.

---

### Bottom line

* **Yes**, add a **minimal local user** store in internal mode (for web cookies).
* The **web app** authenticates via **username/password → cookie session**; CLI/CI use **SA JWTs**.
* Same GridAPI, two auth shapes, one RBAC. Clean and future-proof.


Awesome—here’s a tight, copy-pasteable summary you can hand to the team. It locks the model, fixes the opaque-token mess, and spells out flows for every persona across both GridAPI IdP modes.

# 0) Goals (what we’re standardizing)

* **Service accounts (SA):** use **short-lived JWT access tokens** (stateless) — never opaque tokens.
* **Web UI (humans):** use **server-side cookie sessions** — browser never handles JWT.
* **/tfstate/***: **JWT-only** (no cookies).
* **Revocation:**

  * **Web:** delete session row → instant logout.
  * **JWT:** short TTL + **`jti` denylist** (DB + in-memory cache) → instant rejection.

---

# 1) Final Model Overview

## Modes

* **External IdP mode:** GridAPI is an **OIDC Relying Party** (RP).

  * Web users: OIDC Auth-Code(+PKCE) at IdP → GridAPI validates → creates **cookie session**.
  * SA & CLI/CI: either use **external IdP-issued JWTs**, or (recommended for simplicity) **Grid-managed SAs** with **JWTs** minted by Grid’s internal signer.
* **Internal IdP mode:** No external IdP.

  * Web users: **local username/password** → **cookie session**.
  * SA & CLI/CI: **client-credentials** against Grid’s lightweight OP → **JWTs**.

---

# 2) Migration: Opaque SA Tokens → JWT (step-by-step)

1. **Stop issuing opaque access tokens** for SAs.
2. **Introduce JWT minting** for SAs (client-credentials grant):

   * Claims: `iss`, `sub = "sa:<client_id>"`, `aud = "gridapi"`, `exp` (5–15 min), `iat`, **`jti`** (rand).
   * Sign with your private key; expose **JWKS** at `/.well-known/jwks.json`.
3. **Validation middleware** (Chi):

   * Verify signature via JWKS.
   * Verify `iss/aud/exp/nbf`.
   * **Check `jti` against denylist** (LRU cache + DB).
   * Build `Principal` (service account) → **RBAC**.
4. **Revocation path**:

   * Admin “revoke token” → insert `jti` into `revoked_jti(jti, exp, revoked_at, revoked_by)`.
   * Optional: “revoke all for SA” → mark all active `jti` for that `sub`.
5. **Delete the opaque-token exceptions** and **remove per-request session lookups** for SA traffic.

---

# 3) Server-Side Sessions (for Web UI)

* **Create** on successful web login (local in internal mode, OIDC callback in external mode).
* Store **`session_id`, `user_id`, timestamps, ip/ua, revoked_at`**.
* Set **cookie**: `sessionid=<rand>`; `HttpOnly; Secure; SameSite=Lax; Path=/`.
* **Middleware (web/Control Plane)**:
  Try cookie → load session → principal(user) → RBAC.
  Else try Bearer JWT (for CLI) → principal → RBAC.
  Else 401.
* **Logout/revoke:** delete session row(s) → browser immediately unauthorized.
* **CSRF:** required for **cookie-auth’d mutating** Control Plane routes.

---

# 4) Revocation Model (concise)

| Thing             | Storage                     | Check on request                      | Immediate revoke   |
| ----------------- | --------------------------- | ------------------------------------- | ------------------ |
| **Web session**   | `sessions` table            | Load by `session_id`                  | Delete row(s)      |
| **SA/CLI JWT**    | `revoked_jti` table (+ LRU) | After signature & claims, check `jti` | Insert `jti`       |
| **Nuke all JWTs** | Key rotation                | Signature fails on old `kid`          | Rotate signing key |

---

# 5) Mode Matrix (who uses what)

| Actor                    | Internal IdP mode                 | External IdP mode                                                |
| ------------------------ | --------------------------------- | ---------------------------------------------------------------- |
| **Web user**             | Local login → cookie session      | OIDC Auth-Code(+PKCE) at IdP → server validates → cookie session |
| **CLI human** (optional) | Prefer SA; device-code not needed | Prefer SA; device-code optional via IdP                          |
| **Service account**      | Grid OP client-credentials → JWT  | External IdP JWT **or** Grid-managed SA JWT                      |
| **/tfstate/***           | **JWT only**                      | **JWT only**                                                     |

---

# 6) User Stories & Auth Flows

## A) CLI (External IdP user — optional)

* **Story:** As a human with external IdP, I can use `gridctl login --device` to obtain a JWT for short-lived CLI sessions.
* **Flow:** `gridctl` starts device flow at IdP → user approves in browser → CLI receives **JWT** → stores securely → sends `Authorization: Bearer <JWT>` on API & `/tfstate/*`.
* **Caveats:** Only do this if you want human CLI with IdP. Prefer SA for CI repeatability.

## B) CLI (External IdP SA)

* **Story:** As a CI job, I authenticate using **client credentials** (external IdP) to get a **JWT**.
* **Flow:** CI exchanges `client_id/secret` at IdP token endpoint → gets **JWT** → `Authorization: Bearer <JWT>`.
* **Caveats:** You must trust/manage the IdP app registration and rotate secrets there.

## C) CLI (Internal IdP SA)

* **Story:** As a CI job with no external IdP, I authenticate to Grid’s lightweight OP and get a **JWT**.
* **Flow:** `client_id/secret` → Grid token endpoint → **JWT** → `Authorization: Bearer <JWT>`.
* **Caveats:** Keep TTL short; rely on `jti` denylist for revocation.

## D) Terraform Wrapper (`gridctl tf`)

* **Story:** As a developer/CI, I run `gridctl tf plan/apply` and the wrapper injects the **JWT** automatically.
* **Flow:**

  1. Wrapper ensures a valid JWT (refresh if needed).
  2. Exports env/headers so Terraform’s HTTP backend calls include `Authorization: Bearer <JWT>`.
  3. Streams stdio; preserves exit codes.
* **Caveats:** Do not write tokens to disk; ephemeral config only; one retry on 401 (then fail).

## E) Web UI (Internal mode, local user → cookie session)

* **Story:** As a human, I log in with email/password and use the UI; I don’t manage tokens.
* **Flow:**

  1. GET `/app` → 401 → redirect to `/auth/login`.
  2. POST credentials → **create session** (DB) → `Set-Cookie: sessionid=…`.
  3. Browser calls APIs with cookie; server resolves session → RBAC.
  4. POST `/auth/logout` → session row deleted → instant logout.
* **Caveats:** Add CSRF for mutating API calls.

## F) Web UI (External IdP → cookie session)

* **Story:** As a human, I SSO via IdP and then use the UI with a cookie session.
* **Flow:**

  1. GET `/app` → redirect to IdP (Auth-Code + PKCE).
  2. Callback to GridAPI → server validates tokens (JWKS) → **create session** → `Set-Cookie`.
  3. Browser uses cookie; no JWT in front-end.
* **Caveats:** Optionally store IdP refresh token server-side (encrypted) for silent renewal; still keep app cookie as the primary browser auth.

---

# 7) Route-level Auth Rules (final)

* **Control Plane (RPC/HTTP used by web & CLI):**
  AuthN middleware tries **cookie session first**, then **Bearer JWT**. If neither → 401. Then **RBAC**.
* **Data Plane `/tfstate/*`:**
  **Bearer JWT only** (reject cookies). Then **RBAC** + lock rules.
* **CSRF:** apply to **cookie-auth’d mutating** Control Plane routes only.

---

# 8) Operational Caveats & Best Practices

* **Short TTLs for JWTs** (5–15 minutes). Let `gridctl`/CI refresh automatically.
* **`jti` denylist** with in-memory LRU + DB table for instant revocation.
* **Key rotation**: publish JWKS, version keys by `kid`; rotate periodically and on incident.
* **Never expose SA secrets or JWTs to the browser.**
* **Logging:** never log tokens; redacted command previews only.
* **RBAC consistency:** same enforcement no matter if principal came from cookie or JWT.

---

# 9) Minimal Tables (schema sketch)

* `users(id, email, password_hash, disabled_at, created_at)` — internal mode only.
* `sessions(session_id, user_id, created_at, last_seen_at, ip, ua, revoked_at)` — web cookies.
* `service_accounts(client_id, secret_hash, name, created_at, disabled_at)`.
* `revoked_jti(jti, subject, exp, revoked_at, revoked_by)` — JWT denylist.
* RBAC tables you already have (roles, assignments, policies).

---

This gives you:

* A **stateless** and fast path for CLI/CI & `/tfstate/*` (JWT).
* A **stateful** but simple and revocable path for the web (cookie sessions).
* Clean parity across **internal** and **external** IdP modes without forking the web app.


You’re right: if Terraform grabs a single static header at startup and then churns for an hour, a 10–15 minute JWT will time out and you can’t “refresh mid-run.” You have two viable patterns:

## Option A (simple, but weaker): long-lived SA access tokens

* Make SA JWT TTL ≥ worst-case run (e.g., 90–120 min).
* Works immediately, but increases blast radius if the token leaks.

## Option B (recommended): **Run Token (scoped, long-TTL, one-run JWT)**

Keep normal SA tokens short, but **exchange** them before the run for a **narrowly-scoped, longer-lived JWT** that Terraform uses for the entire run.

### How it works

1. `gridctl tf` obtains a **short-lived SA JWT** (10–15 min).
2. `gridctl tf` calls a new endpoint, e.g. `POST /tfstate/{guid}/run-token` with the SA JWT.
3. GridAPI issues a **Run Token (JWT)** with:

   * `sub`: `sa:<client_id>`
   * `scope`: `tfstate:{guid}:read,write,lock,unlock` (only data-plane ops)
   * `aud`: `gridapi`
   * `exp`: long enough for the run (e.g., 2 hours)
   * `jti`: random
4. `gridctl tf` sets the HTTP backend header to **this Run Token** for the entire Terraform process.
5. Terraform makes all calls with the Run Token; no refresh needed.

### Why it’s better than just “make SA JWT longer”

* **Narrow blast radius**: token can only hit one state’s data-plane ops.
* **Short normal SA tokens**: your general API/Control-Plane exposure stays minimal.
* **Revocation still simple**: denylist by `jti`, plus an admin “cancel run” can revoke the Run Token immediately.
* **No mid-run header swap**: Terraform keeps using the same header it started with.

---

## Concrete changes to make

### 1) Add a Run Token endpoint (token exchange)

* **Request**: bearer = **short SA JWT** (or user JWT if you ever allow human-run).
* **Body**: `{ "state_guid": "...", "duration": "7200s" }` (duration bounded by server max).
* **Response**: `{ "run_token": "<JWT>", "exp": 1731700000 }`.

### 2) Run Token claims (JWT)

* `iss`: GridAPI issuer
* `aud`: `gridapi`
* `sub`: `sa:<client_id>`
* `jti`: random
* `exp`, `iat`
* `scope`: `["tfstate:read","tfstate:write","tfstate:lock","tfstate:unlock"]`
* `state`: `<guid>` (hard scope to one state)
* (optional) `lock_ttl_max`: server hint
* Sign with your normal key; publish via JWKS as usual.

### 3) Data-plane auth rule

Accept **either**:

* A normal short SA JWT with full RBAC (good for quick reads), **or**
* A **Run Token** whose `scope` includes the operation and whose `state` equals `{guid}`.

In practice for long runs, `gridctl tf` should always use the **Run Token**.

### 4) Revocation

* Keep your **DB-backed JTI denylist** (no Redis needed).
* Add admin actions:

  * **RevokeRunToken(jti)** (immediate)
  * **CancelRunsForState(state_guid)** (revoke all current run-token JTIs for that state)
* Cleanup job: delete denylist rows after `exp + skew`.

### 5) `gridctl tf` wrapper behavior

* Before spawning Terraform:

  * Get short SA JWT (or reuse if still valid).
  * Call **/run-token** for the target state, request desired max duration (within server cap).
  * Set header env to the **Run Token**.
  * Run Terraform; no refresh logic required.
* On 401 (should be rare): fail fast; advise to retry. The aim is that Run Token simply doesn’t expire mid-run.

### 6) Policy/limits

* Server-side **max Run Token TTL** (e.g., 3h) to prevent “forever” tokens.
* Only issue Run Tokens when caller has RBAC for the state’s data plane.
* Optionally record a **run record**: `{jti, state_guid, issued_by, exp, ci_job_id?}` for audit.

---

## Where this leaves each mode & actor

### Internal IdP mode

* **SA/CLI**: gets short SA JWT → exchanges for **Run Token** → Terraform uses Run Token.
* **Web UI (local users)**: cookie session; never touches Run Tokens or SA secrets.

### External IdP mode

* **SA/CLI**:

  * Either use external IdP client-credentials to get a JWT and exchange for **Run Token**, or
  * Keep SAs in Grid and use the same internal flow above.
* **Web UI (SSO)**: Authorization Code → server validates → cookie session; no JWT in browser.

---

## If you absolutely cannot add a Run Token endpoint

Then yes, you must make the **SA JWT TTL ≥ max run time** (90–120 min) for those workloads. Keep:

* Separate SA app/role for Terraform with restricted RBAC.
* JTI denylist for immediate revocation.
* Clear operational guidance: shorter TTL elsewhere; only Terraform SAs get long TTLs.

But if you can add **Run Tokens**, you get long-run stability **and** tight scoping, without making all SA tokens long-lived.

---

### TL;DR

* Terraform can’t change headers mid-run, so you either:

  * make the working token live long enough, or
  * **issue a long-TTL but **narrowly-scoped** **Run Token** just for that state’s data-plane ops.
* The **Run Token exchange** is the clean, secure path: short SA JWTs for general use, long but scoped **Run Tokens** for plan/apply.

You’re not wrong—keep it simple.

Given Terraform grabs its auth header once and may not touch the backend again for an hour+, you have two *practical* paths. Here’s the straight answer, plus what’s actually feasible with popular IdPs.

# The workable answer

## 1) Standardize on **Grid-issued SA JWTs** for Terraform runs

Even when an **external IdP** is configured for humans/SSO, use **Grid service accounts** for CLI/CI and the Terraform HTTP backend. That way **you** control token TTL and don’t depend on an IdP’s limits or refresh mechanics that Terraform can’t use mid-run.

* **Set a longer TTL just for Terraform tokens** (e.g., 90–120 minutes; configurable).
* Keep your normal SA tokens short for other API calls if you want; but the token you hand to Terraform must comfortably exceed worst-case plan/apply.
* Still use the **`jti` denylist** for immediate revocation.
* Security guardrails:

  * Scope SA roles to exactly the state(s) and operations needed (label scope + `tfstate:*` only).
  * Rotate SA secrets regularly.
  * Log issuance + decision audits.

This avoids the whole “token refresh during apply” problem and keeps the system simple/stable.

## 2) If a customer insists on using **external IdP tokens** for Terraform

Only do it if their IdP can mint long-enough access tokens for your API audience. Many can; some can’t.

* **Auth0**: access token lifetime for a custom API is configurable (default 24h). You can set ≥ 2 hours. ([Auth0][1])
* **Okta**: access token lifetime configurable via policies; **max 24h** (min 5m). ([Okta Developer][2])
* **Microsoft Entra (Azure AD)**: access token lifetime is typically **~60–90 minutes** and is not easily extended nowadays (older lifetime policies are deprecated). This can be too short for long applies. ([Office 365 for IT Pros][3])

When an IdP **cannot** issue a long-enough token (notably Entra ID), you’ll need to fall back to **Grid-issued SA JWTs** for Terraform. That keeps IdP SSO for the web, but decouples machine runtimes from IdP limits.

# Why not try to “refresh mid-run”?

Terraform’s HTTP backend reads env-provided headers once and then may compute for a long time. If the token expires before the next backend request, the write/unlock fails and you’re stuck. Community threads and HashiCorp notes show similar pain with other expiring credentials during long applies. ([Stack Overflow][4])

# What to implement (minimal, KISS)

* **Token issuing (Grid SAs)**

  * Add a config `TFSTATE_TOKEN_TTL` (default 120m).
  * Mint **JWT** with `iss/aud/sub="sa:<id>"/exp/jti`.
  * Publish JWKS; verify statelessly on the API.
  * Use your **DB-only `revoked_jti` table** + hourly purge where `exp < now() - skew`. (You already settled on this—great.)

* **Wrapper (`gridctl tf`)**

  * At start, fetch the **long-TTL Terraform token** once and export to Terraform’s HTTP backend env.
  * No refresh logic needed.

* **RBAC**

  * Scope SA to data-plane operations only (`tfstate:*`) and label scope for the target states.
  * Keep web/UI on cookie sessions (SSO via external IdP or local users in internal mode).

# Mode matrix (final)

| Scenario                                    | Recommended token used by Terraform                                                                                |
| ------------------------------------------- | ------------------------------------------------------------------------------------------------------------------ |
| **External IdP for humans (SSO)**           | **Grid-issued SA JWT (90–120m)** for Terraform. Web stays on cookie session.                                       |
| **Internal mode (no external IdP)**         | **Grid-issued SA JWT (90–120m)** for Terraform. Web uses local-user cookie session.                                |
| **Customer mandates using their IdP token** | Only if IdP can issue ≥ run length (Auth0/Okta OK; Entra is often too short). Otherwise: Grid SA JWT. ([Auth0][1]) |

# TL;DR

* Don’t fight Terraform’s one-shot header behavior.
* Issue a **single, long-enough JWT** for the run (90–120m) — **from Grid SAs** — and keep SSO/JWT complexities away from the browser.
* If an external IdP can mint long tokens for your API audience, fine; but **don’t rely on it**, especially with Entra. You’ll ship a more reliable product by standardizing on Grid-issued SA JWTs for Terraform.

[1]: https://auth0.com/docs/secure/tokens/access-tokens?utm_source=chatgpt.com "Access Tokens"
[2]: https://developer.okta.com/docs/api/openapi/okta-oauth/guides/overview/?utm_source=chatgpt.com "OpenID Connect & OAuth 2.0"
[3]: https://office365itpros.com/2023/05/29/azure-ad-access-token-lifetime/?utm_source=chatgpt.com "Azure AD Access Token Lifetimes and PowerShell Scripts"
[4]: https://stackoverflow.com/questions/68111651/aws-session-token-expired-while-running-terraform-apply?utm_source=chatgpt.com "AWS Session token expired while running terraform apply"



```bash
# ============================================
# 1. FRESH LOGIN
# ============================================
./bin/gridctl auth login \
--client-id="0199ea6c-f0cc-7cf8-9ba4-fc24e3936ac1" \
--client-secret="c9913d8d810ab150ba704b1a29b341e62383aa422702772425eec1ceab1492f3"

# ============================================
# 2. CHECK STORED CREDENTIALS
# ============================================
# View full credentials file
cat ~/.grid/credentials.json | jq '.'

# Extract just the access token
jq -r '.access_token' ~/.grid/credentials.json

# Show first 30 characters of token
jq -r '.access_token' ~/.grid/credentials.json | head -c 30

# Show expiration time
jq -r '.expires_at' ~/.grid/credentials.json

# Show principal ID (if stored)
jq -r '.principal_id' ~/.grid/credentials.json

# ============================================
# 3. COMPUTE TOKEN HASH (CLIENT-SIDE)
# ============================================
# Full SHA256 hash of stored token
printf '%s' "$(jq -r '.access_token' ~/.grid/credentials.json)" | sha256sum

# First 20 characters of hash (for comparison)
printf '%s' "$(jq -r '.access_token' ~/.grid/credentials.json)" | sha256sum | awk '{print substr($1,1,20)}'

# ============================================
# 4. CHECK SESSION IN DATABASE
# ============================================
# View latest session
psql "postgres://grid:gridpass@localhost:5432/grid?sslmode=disable" -c "
SELECT 
    id,
    user_id,
    service_account_id,
    LEFT(token_hash, 20) as hash_prefix,
    created_at,
    expires_at,
    revoked
FROM sessions 
ORDER BY created_at DESC 
LIMIT 1;"

# View all sessions for our service account
psql "postgres://grid:gridpass@localhost:5432/grid?sslmode=disable" -c "
SELECT 
    id,
    LEFT(token_hash, 20) as hash_prefix,
    created_at,
    expires_at,
    revoked
FROM sessions 
WHERE service_account_id = '578383c2-a138-4ff8-b092-fcd19c617add'
ORDER BY created_at DESC 
LIMIT 5;"

# Get full token hash from latest session
psql "postgres://grid:gridpass@localhost:5432/grid?sslmode=disable" -c "
SELECT token_hash 
FROM sessions 
ORDER BY created_at DESC 
LIMIT 1;"

# ============================================
# 5. VERIFY HASHES MATCH
# ============================================
# Compare client token hash vs database hash
echo "=== Client Token Hash ===" && \
printf '%s' "$(jq -r '.access_token' ~/.grid/credentials.json)" | sha256sum | awk '{print substr($1,1,20)}' && \
echo "=== Database Hash ===" && \
psql "postgres://grid:gridpass@localhost:5432/grid?sslmode=disable" -t -c "
SELECT LEFT(token_hash, 20) 
FROM sessions 
ORDER BY created_at DESC 
LIMIT 1;" | tr -d ' '

# ============================================
# 6. CHECK CASBIN ROLES
# ============================================
# View service account's role assignments
psql "postgres://grid:gridpass@localhost:5432/grid?sslmode=disable" -c "
SELECT * FROM casbin_rules 
WHERE ptype = 'g' 
AND v0 = 'sa:0199ea6c-f0cc-7cf8-9ba4-fc24e3936ac1';"

# View all policies for platform-engineer role
psql "postgres://grid:gridpass@localhost:5432/grid?sslmode=disable" -c "
SELECT * FROM casbin_rules 
WHERE ptype = 'p' 
AND v0 = 'role:platform-engineer';"

# ============================================
# 7. TEST AUTHENTICATED REQUEST (CURL)
# ============================================
# Test with curl (verbose)
TOKEN=$(jq -r '.access_token' ~/.grid/credentials.json) && \
curl -v \
-H "Authorization: Bearer $TOKEN" \
-H "Content-Type: application/json" \
-d '{"principal_id":"sa:0199ea6c-f0cc-7cf8-9ba4-fc24e3936ac1"}' \
http://localhost:8080/state.v1.StateService/GetEffectivePermissions

# Test with curl (just status code)
TOKEN=$(jq -r '.access_token' ~/.grid/credentials.json) && \
curl -s -o /dev/null -w "HTTP Status: %{http_code}\n" \
-H "Authorization: Bearer $TOKEN" \
-H "Content-Type: application/json" \
-d '{"principal_id":"sa:0199ea6c-f0cc-7cf8-9ba4-fc24e3936ac1"}' \
http://localhost:8080/state.v1.StateService/GetEffectivePermissions

# ============================================
# 8. TEST WITH GRIDCTL
# ============================================
# Test auth status command
./bin/gridctl auth status

# ============================================
# 9. CHECK SERVICE ACCOUNT EXISTS
# ============================================
psql "postgres://grid:gridpass@localhost:5432/grid?sslmode=disable" -c "
SELECT 
    id,
    client_id,
    name,
    disabled,
    created_at
FROM service_accounts 
WHERE client_id = '0199ea6c-f0cc-7cf8-9ba4-fc24e3936ac1';"

# ============================================
# 10. CHECK SERVER LOGS
# ============================================
# If server is running in background bash shell with ID 98829b:
# (Replace 98829b with actual shell ID from background bash)
# Use BashOutput tool to check logs

# ============================================
# 11. CLEANUP (OPTIONAL)
# ============================================
# Revoke all sessions for the service account
psql "postgres://grid:gridpass@localhost:5432/grid?sslmode=disable" -c "
UPDATE sessions 
SET revoked = true 
WHERE service_account_id = '578383c2-a138-4ff8-b092-fcd19c617add';"

# Delete old sessions
psql "postgres://grid:gridpass@localhost:5432/grid?sslmode=disable" -c "
DELETE FROM sessions 
WHERE expires_at < NOW();"

# ============================================
# 12. DEBUGGING - CHECK IF TOKEN IS JWT
# ============================================
# Try to decode token as JWT (will fail for opaque tokens)
TOKEN=$(jq -r '.access_token' ~/.grid/credentials.json) && \
echo $TOKEN | cut -d. -f1-3 | wc -w

# If result is 1 (one word), it's opaque
# If result is 3 (three words separated by dots), it's JWT

# Try to base64 decode the token (won't work for opaque)
TOKEN=$(jq -r '.access_token' ~/.grid/credentials.json) && \
echo $TOKEN | base64 -d 2>&1 || echo "Not base64 encoded (opaque token)"
```

Quick verification script:

```bash
#!/bin/bash
# Save as verify-auth.sh

echo "=== Token from credentials file ==="
TOKEN=$(jq -r '.access_token' ~/.grid/credentials.json)
echo "First 40 chars: ${TOKEN:0:40}"
echo "Full hash: $(printf '%s' "$TOKEN" | sha256sum | awk '{print $1}')"
echo ""

echo "=== Latest session in database ==="
psql "postgres://grid:gridpass@localhost:5432/grid?sslmode=disable" -t -c "
SELECT 
    'Created: ' || created_at::text || E'\n' ||
    'Hash: ' || token_hash || E'\n' ||
    'Expired: ' || (expires_at < NOW())::text
FROM sessions 
ORDER BY created_at DESC 
LIMIT 1;"
echo ""

echo "=== Hash Comparison ==="
CLIENT_HASH=$(printf '%s' "$TOKEN" | sha256sum | awk '{print $1}')
DB_HASH=$(psql "postgres://grid:gridpass@localhost:5432/grid?sslmode=disable" -t -A -c "SELECT token_hash FROM sessions ORDER BY created_at 
DESC LIMIT 1;")

if [ "$CLIENT_HASH" == "$DB_HASH" ]; then
    echo "✅ MATCH - Hashes are identical"
else
    echo "❌ MISMATCH"
    echo "Client: $CLIENT_HASH"
    echo "DB:     $DB_HASH"
fi
```

Run with: chmod +x verify-auth.sh && ./verify-auth.sh

---

## CRITICAL: JWT Creation Architecture Clarification

**Date**: 2025-10-18

### Discovery

When investigating why JWT tokens were not being issued correctly, we discovered that the zitadel/oidc library **handles JWT creation internally** when `AccessTokenType() = JWT`.

### What This Means

**Incorrect Understanding** (from T009A):
- ❌ We need to implement custom `CreateAccessToken()` method that manually creates JWTs
- ❌ We need to manually create JWTs using go-jose library
- ❌ We need to call `jwt.Signed(signer).Claims(claims).Serialize()`
- ❌ We need to manually generate and store JTI

**Correct Understanding**:
- ✅ Library creates JWTs automatically when `AccessTokenType() = JWT`
- ✅ We only need to implement `GetAudience()`, `GetSubject()`, `GetScopes()` trait methods
- ✅ Library uses our signing key from `provider.Crypto()`
- ✅ Library generates its own `jti` internally
- ✅ Library exposes JWKS at `/keys` endpoint automatically
- ✅ Library handles all JWT signing logic internally

### Architecture Flow

When `AccessTokenType() = JWT`:
1. zitadel/oidc library creates the JWT using `provider.Crypto()` (our RSA key)
2. Library calls our `GetSubject()`, `GetAudience()`, `GetScopes()` to get claims
3. Library generates `jti` internally
4. Library signs the JWT with our RSA private key
5. Library exposes JWKS via `/keys` endpoint with our public key

**What IS being used**:
- The `rsaSigningKey` struct (lines 640-656 in oidc.go) - passed to the library via `provider.Crypto()`
- The library's internal JWT creation logic
- Our `GetAudience()`, `GetSubject()`, `GetScopes()` implementations

**What IS NOT being used**:
- ❌ Our custom `createJWT()` method (if present) - dead code
- ❌ Our custom go-jose signing setup - NOT USED
- ❌ The JTI we generate manually - library generates its own

### Required Code Changes

**File**: `cmd/gridapi/internal/auth/oidc.go`

1. **Line ~722**: Set access token type
   ```go
   func (c *serviceAccountClient) AccessTokenType() op.AccessTokenType {
       return op.AccessTokenTypeJWT  // NOT op.AccessTokenTypeBearer
   }
   ```

2. **Line ~507**: Fix audience for client credentials
   ```go
   func (r *clientCredentialsTokenRequest) GetAudience() []string {
       return []string{"gridapi"}  // NOT []string{} or client_id
   }
   ```

### Signing Key Persistence

**STILL CRITICAL**: The random key generation on line 120 remains a production bug:

```go
privateKey, err := rsa.GenerateKey(rand.Reader, 2048)  // ❌ NEW KEY ON EVERY RESTART
```

Even though the library creates JWTs, it uses **OUR** key. Random keys invalidate all tokens on restart.

**Solution**: Load persistent keys from disk (T003A still required).

### What Changed in Our Understanding

**Previous (Incorrect) Plan**:
- Implement `CreateAccessToken()` to manually create JWTs
- Use go-jose to sign tokens manually
- Generate and store JTI ourselves
- Persist JTI in sessions table or revoked_jti table

**Current (Correct) Plan**:
- Set `AccessTokenType()` to return `op.AccessTokenTypeJWT`
- Fix `GetAudience()` to return correct audience
- Load persistent signing key from disk (T003A)
- Let the library handle **all** JWT creation, signing, and JTI generation

### Impact on Task T009A

T009A should be **SIMPLIFIED**:

**Old Task (Incorrect)**:
- Implement custom JWT creation logic
- Use go-jose for signing
- Generate JTI manually
- Store JTI in database

**New Task (Correct)**:
- Change `AccessTokenType()` to return `op.AccessTokenTypeJWT`
- Fix `GetAudience()` to return `[]string{"gridapi"}`
- Verify tokens have 3 parts (header.payload.signature)
- Verify claims include: iss, sub, aud, exp, iat, jti

**Total lines changed**: ~2 lines (not ~150 lines of manual JWT creation)

### Verification Steps

1. **Confirm JWT structure**:
   ```bash
   # After gridapi runs, check token from CLI
   TOKEN=$(jq -r '.access_token' ~/.grid/credentials.json)
   echo $TOKEN | awk -F. '{print NF}'  # Should output: 3 (header.payload.signature)
   ```

2. **Decode JWT claims**:
   ```bash
   # Decode payload (middle part)
   echo $TOKEN | awk -F. '{print $2}' | base64 -d 2>/dev/null | jq .
   # Should show: iss, sub, aud, exp, iat, jti
   ```

3. **Verify audience is correct**:
   ```bash
   echo $TOKEN | awk -F. '{print $2}' | base64 -d 2>/dev/null | jq -r '.aud'
   # Should output: ["gridapi"] or "gridapi"
   ```

### References

- zitadel/oidc library documentation on JWT access tokens
- `cmd/gridapi/internal/auth/oidc.go` (implementation file)
- JWT-MIGRATION-ACTION-PATH.md Phase 2 "Dead Code Note"
- tasks.md T009A (updated task description)
