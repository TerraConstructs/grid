
Review Summary: WebApp Data Model vs. Bun ORM Models

Alignment Analysis

✅ Well-Aligned Entities

1. User Entity
- Frontend Model (data-model.md:24-32): Defines User with id, username, email, authType, roles[], groups[]
- Backend Model (auth.go:15-27): Defines User with ID, Subject, Email, Name, PasswordHash, timestamps
- Gap: The frontend User.username maps to backend User.Name, which is good
- Gap: The frontend User.authType is derived (not stored in DB) - determined by presence of Subject (external) vs PasswordHash (internal)
- Gap: The frontend User.roles[] and User.groups[] are not stored directly in the User table - they need to be joined/aggregated

2. Session Entity
- Frontend Model (data-model.md:64-71): Session with user, expiresAt, isLoading, error
- Backend Model (auth.go:130-145): Session table with UserID, ServiceAccountID, TokenHash, ExpiresAt, etc.
- Alignment: The backend session is server-side (DB-persisted), frontend session is client-side representation
- Gap: Frontend Session embeds User object - this requires a join or separate fetch
- Gap: Frontend isLoading and error are UI state only (not persisted)

3. Role/Group Relationships
- Frontend Expectation: User has roles: string[] and groups: string[] arrays
- Backend Reality:
  - Roles come from UserRole join table (auth.go:106-116) or GroupRole for SSO groups (auth.go:119-127)
  - Groups are not stored in DB - they come from JWT claims in external IdP mode
- Implementation Required: Whoami endpoint must:
  a. Join users → user_roles → roles to get role names
  b. Join users → sessions to get id_token, then decode JWT to extract groups claim (external IdP only)

---
🔴 Critical Gaps

1. Missing Backend-to-Frontend Mapping

The data-model.md assumes a /api/auth/whoami endpoint (line 58) that does not exist yet. This endpoint needs to:

// Expected response shape
interface WhoamiResponse {
  user: {
    id: string;        // From users.id
    username: string;  // From users.name
    email: string;     // From users.email
    authType: 'internal' | 'external'; // Derived: Subject != null ? 'external' : 'internal'
    roles: string[];   // Aggregated from user_roles → roles.name OR group_roles → roles.name
    groups?: string[]; // Extracted from sessions.id_token JWT 'groups' claim (external IdP only)
  };
  expiresAt: number;   // From sessions.expires_at (Unix timestamp)
}

Implementation needs:
- Repository method to fetch session with joined user data
- Logic to decode JWT id_token and extract groups claim
- Aggregation of roles from both user_roles (direct) and group_roles (via JWT groups)

2. AuthConfig Endpoint Missing

The frontend expects /auth/config endpoint (data-model.md:102) to return:

interface AuthConfig {
  mode: 'internal-idp' | 'external-idp' | 'disabled';
  issuer?: string;
  clientId?: string;
  audience?: string;
  supportsDeviceFlow: boolean;
}

Backend needs:
- This requires reading gridapi server configuration (OIDC settings)
- Currently, auth config is in cmd/gridapi/internal/config/config.go but not exposed as an API endpoint
- Need to add Connect RPC method or HTTP handler for /auth/config

3. Role Aggregation Logic

For external IdP users, roles come from TWO sources:
1. Direct user-role mappings (user_roles table)
2. Group-based role mappings (group_roles table, matched against JWT groups claim)

The data-model.md (line 40) states:
Derived from direct user-role mappings (internal IdP) or group-role mappings (external IdP).

This is incomplete. The backend should support:
- Internal IdP users: roles from user_roles only
- External IdP users: roles from both user_roles AND group_roles (union)

Backend needs:
- Service method to resolve effective roles by:
  a. Fetching direct user_roles for the user
  b. Decoding session JWT to get groups claim
  c. Querying group_roles for matching group names
  d. Returning union of both role sets

---
📋 Consistency Issues

1. Username vs Name Terminology

- Frontend: Uses username (data-model.md:26)
- Backend: Uses name field (auth.go:21)
- Resolution: Acceptable - just document that frontend User.username maps to backend User.Name

2. Session Expiry Format

- Frontend: Expects expiresAt: number (Unix timestamp in milliseconds, data-model.md:67)
- Backend: Stores expires_at as time.Time (auth.go:139)
- Resolution: Backend API should convert time.Time to Unix milliseconds: session.ExpiresAt.UnixMilli()

3. AuthType Derivation

- Frontend: Expects authType: 'internal' | 'external' as a field (data-model.md:28)
- Backend: Doesn't store this explicitly - must derive from Subject != nil (external) vs PasswordHash != nil (internal)
- Resolution: Backend whoami handler should compute this:
authType := "internal"
if user.Subject != nil && *user.Subject != "" {
    authType = "external"
}

---
✅ Recommendations

1. Create whoami endpoint in gridapi:
  - Path: /api/auth/whoami (or Connect RPC method GetCurrentUser)
  - Returns user + session info with joined data
  - Handles role aggregation and JWT group extraction
2. Create config endpoint:
  - Path: /auth/config
  - Returns server auth mode and OIDC parameters (if external IdP enabled)
3. Add repository methods:
  - GetUserWithRoles(ctx, userID) - joins user_roles → roles
  - GetGroupRoles(ctx, groupNames) - fetches roles for JWT groups
  - GetSessionWithUser(ctx, tokenHash) - joins session → user
4. Update data-model.md to clarify:
  - authType is computed, not stored
  - roles is aggregated from multiple sources (user_roles + group_roles for external IdP)
  - groups comes from JWT claims, not DB
5. Document backend constraints:
  - Session table foreign key constraints (UserID → users.id)
  - Role aggregation logic for mixed direct + group-based roles

---
🎯 Action Items

From highest to lowest priority:

1. Implement /api/auth/whoami endpoint - Critical for frontend session restoration
2. Implement /auth/config endpoint - Needed to determine login UI
3. Add role aggregation service method - Required for accurate permissions display
4. Update data-model.md - Clarify derivation vs storage for authType, roles, groups
5. Write integration tests - Verify whoami returns correct shape for both internal/external IdP modes

The core data model is well-designed, but the mapping between backend DB schema and frontend expectations needs explicit implementation in the gridapi handlers/services layer. The biggest missing piece is the
whoami endpoint that bridges the two.
