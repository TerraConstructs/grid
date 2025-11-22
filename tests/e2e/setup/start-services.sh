#!/usr/bin/env bash
#
# Start all services required for E2E tests:
# - PostgreSQL (docker-compose)
# - Keycloak (docker-compose)
# - gridapi server
# - webapp dev server (Vite)
#
# This script is invoked by Playwright's webServer configuration.
# It must be idempotent and handle already-running services gracefully.

set -euo pipefail

# Color output for better visibility
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

log_info() {
    echo -e "${GREEN}[E2E Setup]${NC} $*"
}

log_warn() {
    echo -e "${YELLOW}[E2E Setup]${NC} $*"
}

log_error() {
    echo -e "${RED}[E2E Setup]${NC} $*"
}

# Determine project root (where docker-compose.yml lives)
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/../../.." && pwd)"

cd "${PROJECT_ROOT}"

# PID files for tracking started processes
GRIDAPI_PID_FILE="/tmp/grid-e2e-gridapi.pid"
WEBAPP_PID_FILE="/tmp/grid-e2e-webapp.pid"

# Cleanup function to kill background processes on exit
cleanup() {
    log_info "Cleaning up E2E test services..."

    if [[ -f "${GRIDAPI_PID_FILE}" ]]; then
        GRIDAPI_PID=$(cat "${GRIDAPI_PID_FILE}")
        if kill -0 "${GRIDAPI_PID}" 2>/dev/null; then
            log_info "Stopping gridapi (PID: ${GRIDAPI_PID})"
            kill "${GRIDAPI_PID}" || true
        fi
        rm -f "${GRIDAPI_PID_FILE}"
    fi

    if [[ -f "${WEBAPP_PID_FILE}" ]]; then
        WEBAPP_PID=$(cat "${WEBAPP_PID_FILE}")
        if kill -0 "${WEBAPP_PID}" 2>/dev/null; then
            log_info "Stopping webapp (PID: ${WEBAPP_PID})"
            kill "${WEBAPP_PID}" || true
        fi
        rm -f "${WEBAPP_PID_FILE}"
    fi
}

# Register cleanup on script exit
trap cleanup EXIT INT TERM

#
# Step 1: Start docker-compose (PostgreSQL + Keycloak)
#
log_info "Starting docker-compose services (postgres + keycloak)..."
docker-compose up -d

# Wait for postgres to be healthy
log_info "Waiting for PostgreSQL to be ready..."
for i in {1..30}; do
    if docker-compose exec -T postgres pg_isready -U grid >/dev/null 2>&1; then
        log_info "PostgreSQL is ready"
        break
    fi
    if [[ $i -eq 30 ]]; then
        log_error "PostgreSQL failed to become ready"
        exit 1
    fi
    sleep 1
done

# Wait for Keycloak to be healthy
log_info "Waiting for Keycloak to be ready..."
for i in {1..60}; do
    if curl -f http://localhost:8443/health/ready >/dev/null 2>&1; then
        log_info "Keycloak is ready"
        break
    fi
    if [[ $i -eq 60 ]]; then
        log_error "Keycloak failed to become ready"
        exit 1
    fi
    sleep 2
done

#
# Step 2: Seed test data (create test users in Keycloak)
#
log_info "Seeding test data..."
bash "${SCRIPT_DIR}/seed-test-data.sh"

#
# Step 3: Build gridapi if needed
#
if [[ ! -f "${PROJECT_ROOT}/bin/gridapi" ]]; then
    log_info "Building gridapi..."
    cd "${PROJECT_ROOT}/cmd/gridapi"
    go build -o "${PROJECT_ROOT}/bin/gridapi" .
    cd "${PROJECT_ROOT}"
else
    log_info "gridapi binary already exists"
fi

#
# Step 4: Run database migrations
#
log_info "Running database migrations..."
"${PROJECT_ROOT}/bin/gridapi" db init \
    --db-url="postgres://grid:gridpass@localhost:5432/grid?sslmode=disable" || true

"${PROJECT_ROOT}/bin/gridapi" db migrate \
    --db-url="postgres://grid:gridpass@localhost:5432/grid?sslmode=disable"

#
# Step 5: Bootstrap group-to-role mappings
#
log_info "Bootstrapping group-to-role mappings..."
# Create test-admins -> admin role
"${PROJECT_ROOT}/bin/gridapi" role create \
    --name "admin" \
    --description "Full admin access" || log_warn "admin role may already exist"

"${PROJECT_ROOT}/bin/gridapi" role map-group \
    --role-name "admin" \
    --group "test-admins" \
    --db-url="postgres://grid:gridpass@localhost:5432/grid?sslmode=disable" || true

# Create product-engineers -> product-engineer role
"${PROJECT_ROOT}/bin/gridapi" role create \
    --name "product-engineer" \
    --description "Product engineer with env=dev access" || log_warn "product-engineer role may already exist"

"${PROJECT_ROOT}/bin/gridapi" role map-group \
    --role-name "product-engineer" \
    --group "product-engineers" \
    --db-url="postgres://grid:gridpass@localhost:5432/grid?sslmode=disable" || true

# Create platform-engineers -> platform-engineer role
"${PROJECT_ROOT}/bin/gridapi" role create \
    --name "platform-engineer" \
    --description "Platform engineer with env=prod access" || log_warn "platform-engineer role may already exist"

"${PROJECT_ROOT}/bin/gridapi" role map-group \
    --role-name "platform-engineer" \
    --group "platform-engineers" \
    --db-url="postgres://grid:gridpass@localhost:5432/grid?sslmode=disable" || true

#
# Step 6: Start gridapi server (Mode 1 - External IdP)
#
log_info "Starting gridapi server..."

# Export environment variables for Mode 1 (External IdP with Keycloak)
export EXTERNAL_IDP_ISSUER="http://localhost:8443/realms/grid"
export EXTERNAL_IDP_CLIENT_ID="grid-api"
export EXTERNAL_IDP_CLIENT_SECRET="tsREgTe21npWljPNsYf6qzenc2AWF9e9"
export EXTERNAL_IDP_REDIRECT_URI="http://localhost:8080/auth/sso/callback"

# Start gridapi in background
"${PROJECT_ROOT}/bin/gridapi" serve \
    --server-addr ":8080" \
    --db-url "postgres://grid:gridpass@localhost:5432/grid?sslmode=disable" \
    > /tmp/grid-e2e-gridapi.log 2>&1 &

GRIDAPI_PID=$!
echo "${GRIDAPI_PID}" > "${GRIDAPI_PID_FILE}"
log_info "gridapi started (PID: ${GRIDAPI_PID})"

# Wait for gridapi to be ready
log_info "Waiting for gridapi to be ready..."
for i in {1..30}; do
    if curl -f http://localhost:8080/health >/dev/null 2>&1; then
        log_info "gridapi is ready"
        break
    fi
    if [[ $i -eq 30 ]]; then
        log_error "gridapi failed to become ready"
        log_error "Check logs at /tmp/grid-e2e-gridapi.log"
        exit 1
    fi
    sleep 1
done

#
# Step 7: Start webapp dev server (Vite)
#
log_info "Starting webapp dev server..."

cd "${PROJECT_ROOT}/webapp"
pnpm install --silent

# Start webapp in background
pnpm dev > /tmp/grid-e2e-webapp.log 2>&1 &
WEBAPP_PID=$!
echo "${WEBAPP_PID}" > "${WEBAPP_PID_FILE}"
log_info "webapp started (PID: ${WEBAPP_PID})"

# Wait for webapp to be ready
log_info "Waiting for webapp to be ready..."
for i in {1..30}; do
    if curl -f http://localhost:5173 >/dev/null 2>&1; then
        log_info "webapp is ready"
        break
    fi
    if [[ $i -eq 30 ]]; then
        log_error "webapp failed to become ready"
        log_error "Check logs at /tmp/grid-e2e-webapp.log"
        exit 1
    fi
    sleep 1
done

log_info "All services started successfully!"
log_info "  - PostgreSQL: localhost:5432"
log_info "  - Keycloak:   http://localhost:8443"
log_info "  - gridapi:    http://localhost:8080"
log_info "  - webapp:     http://localhost:5173"

# Keep the script running so Playwright can detect when services are ready
# The script will be terminated by Playwright when tests complete
wait
