# Grid Webapp

React-based web dashboard for Grid Terraform state management.

## Development Setup

### Prerequisites

- Node.js 18+ and pnpm
- Grid API server (gridapi) running on localhost:8080

### Quick Start

```bash
# Install dependencies
pnpm install

# Start development server
pnpm dev
# Webapp runs on http://localhost:5173
```

### ⚠️ IMPORTANT: Vite Proxy Configuration

**DO NOT REMOVE** the proxy configuration in `vite.config.ts`. It is **CRITICAL** for development.

```typescript
server: {
  proxy: {
    '/auth': 'http://localhost:8080',
    '/api': 'http://localhost:8080',
    '/state.v1.StateService': 'http://localhost:8080',
    '/tfstate': 'http://localhost:8080',
  }
}
```

**Why this exists**:

1. **httpOnly Session Cookies**: Grid uses httpOnly cookies for session management (security best practice)
2. **Same-Origin Requirement**: httpOnly cookies are only sent when the request is same-origin as the cookie domain
3. **Dev Mode Problem**: Webapp runs on `localhost:5173`, API on `localhost:8080` (different origins)
4. **Vite Proxy Solution**: Proxies API requests to `localhost:8080` while browser thinks everything is `localhost:5173`

**Without the proxy**:
- ❌ Session cookies won't be sent with API requests (401 Unauthorized)
- ❌ Authentication won't work
- ❌ SSO login will fail
- ❌ Connect RPC calls will fail with "unauthenticated"

**Related Issues**: See Beads issue `grid-202d` for SSO callback redirect fix

---

## Architecture

### SDK Configuration

The Grid SDK (`@tcons/grid`) uses `window.location.origin` as the API base URL:

```typescript
// js/sdk/src/auth.ts
let API_BASE_URL = typeof window !== 'undefined'
  ? window.location.origin  // Uses webapp's origin (localhost:5173 in dev)
  : 'http://localhost:8080';
```

**Why `window.location.origin`**:
- In dev: Requests go to `localhost:5173`, Vite proxies to `localhost:8080`
- In prod: Requests go to actual deployment URL (same-origin by design)
- Enables session cookies to work correctly in both environments

**DO NOT** change this to hardcode `http://localhost:8080` - it breaks the proxy setup.

---

## Deployment Architecture

### Production Deployment Options

Grid webapp and API must be deployed **same-origin** for session cookies to work.

#### Option 1: Reverse Proxy (Recommended)

Use nginx/caddy/traefik to:
- Serve static webapp files at `/`
- Proxy API requests to gridapi backend

**Example nginx config**:

```nginx
server {
    listen 80;
    server_name grid.example.com;

    # Serve webapp static files
    root /var/www/grid/webapp;
    index index.html;

    # Proxy API requests to gridapi
    location /auth/ {
        proxy_pass http://localhost:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
    }

    location /api/ {
        proxy_pass http://localhost:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
    }

    location /state.v1.StateService/ {
        proxy_pass http://localhost:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
    }

    location /tfstate/ {
        proxy_pass http://localhost:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
    }

    # SPA fallback for client-side routing
    location / {
        try_files $uri $uri/ /index.html;
    }
}
```

#### Option 2: Embedded Static Files

Embed webapp build in gridapi binary and serve from `/`:

```go
// Serve static files at root
http.Handle("/", http.FileServer(http.FS(embeddedWebapp)))
```

**Pros**: Single binary deployment
**Cons**: Requires rebuild for frontend changes

#### Option 3: CDN + CORS (Not Recommended)

Serving webapp from CDN with CORS to API server:

**Why this doesn't work**:
- httpOnly cookies cannot be accessed cross-origin
- Even with `credentials: 'include'`, browser security prevents cookie sharing
- Would require switching to non-httpOnly cookies (security risk)

---

## Build & Test

```bash
# Build for production
pnpm build
# Output: dist/ directory with optimized static files

# Run tests
pnpm test

# Type check
pnpm type-check

# Lint
pnpm lint
```

---

## Known Issues

### SSO Callback Redirect (grid-202d)

**Problem**: SSO callback currently redirects to `localhost:8080` instead of `localhost:5173`

**Symptom**: After Keycloak login, user sees "404 page not found" at `localhost:8080/`

**Root Cause**: Backend `HandleSSOCallback` hardcodes redirect to "/" (relative path), which browser resolves as `localhost:8080/`

**Status**: Tracked in Beads issue `grid-202d`

**Workaround**: Manually navigate back to `http://localhost:5173/` - session cookie will work via proxy

**Fix**: Backend needs to implement `redirect_uri` parameter per contract specification (specs/007-webapp-auth/contracts/README.md:138)

---

## Authentication Modes

Grid supports three authentication modes (configured via gridapi environment variables):

### 1. Disabled (No Auth)
```bash
# gridapi without auth env vars
./bin/gridapi serve
```
- Webapp shows dashboard immediately
- No login required
- All states visible

### 2. Internal IdP (Username/Password)
```bash
export OIDC_ISSUER="http://localhost:8080"
export OIDC_CLIENT_ID="gridapi"
./bin/gridapi serve
```
- Webapp shows username/password login form
- Users managed by gridapi (`gridapi users create`)
- Sessions stored in database

### 3. External IdP (SSO)
```bash
export EXTERNAL_IDP_ISSUER="http://localhost:8443/realms/grid"
export EXTERNAL_IDP_CLIENT_ID="grid-api"
export EXTERNAL_IDP_CLIENT_SECRET="<secret>"
./bin/gridapi serve
```
- Webapp shows "Sign In with SSO" button
- Users authenticate via Keycloak/Azure/Okta
- Group-based role assignment
- **Note**: See grid-202d for callback redirect issue

---

## Tech Stack

- **Framework**: React 18 + TypeScript
- **Build Tool**: Vite
- **RPC Client**: @connectrpc/connect-web (Connect RPC)
- **Styling**: Tailwind CSS
- **Icons**: Lucide React
- **State**: React Context API

---

## Contributing

When making changes:

1. **Never remove the Vite proxy config** - it's required for development
2. **Never hardcode API URLs** - use `window.location.origin` via SDK
3. **Test authentication flows** - verify cookies work in dev mode
4. **Check Beads issues** - link code changes to relevant issues

For questions, see:
- specs/007-webapp-auth/ - Authentication implementation details
- Beads issue grid-202d - SSO callback fix tracking
- CLAUDE.md - Project development guidelines
