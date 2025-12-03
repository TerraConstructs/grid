#!/usr/bin/env bash
#
# Start services required for No-Auth E2E tests:
# - PostgreSQL (docker compose)
# - gridapi server (in no-auth mode)
# - webapp dev server (Vite)
#
# This script is invoked by Playwright's webServer configuration.

set -euo pipefail

# Color output for better visibility
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

log_info() {
    echo -e "${GREEN}[E2E No-Auth Setup]${NC} $*"
}

log_warn() {
    echo -e "${YELLOW}[E2E No-Auth Setup]${NC} $*"
}

log_error() {
    echo -e "${RED}[E2E No-Auth Setup]${NC} $*"
}

# Determine project root (where docker-compose.yml lives)
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/../../.." && pwd)"

cd "${PROJECT_ROOT}"

# PID files for tracking started processes
GRIDAPI_PID_FILE="/tmp/grid-e2e-no-auth-gridapi.pid"
WEBAPP_PID_FILE="/tmp/grid-e2e-no-auth-webapp.pid"

DB_URL="postgres://grid:gridpass@localhost:5432/grid?sslmode=disable"
SERVER_URL="http://localhost:8080"

export GRID_DATABASE_URL="${DB_URL}"
export GRID_SERVER_URL="${SERVER_URL}"

# Cleanup function to kill background processes on exit
cleanup() {
    log_info "Cleaning up E2E no-auth test services..."

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
# Step 1: Start docker compose (PostgreSQL)
#
log_info "Starting docker compose services (postgres)..."
docker compose up -d postgres

# Wait for postgres to be healthy
log_info "Waiting for PostgreSQL to be ready..."
for i in {1..30}; do
    if docker compose exec -T postgres pg_isready -U grid >/dev/null 2>&1; then
        log_info "PostgreSQL is ready"
        break
    fi
    if [[ $i -eq 30 ]]; then
        log_error "PostgreSQL failed to become ready"
        exit 1
    fi
    sleep 1
done

#
# Step 2: Build gridapi and gridctl if needed
#
if [[ ! -f "${PROJECT_ROOT}/bin/gridapi" ]] || [[ ! -f "${PROJECT_ROOT}/bin/gridctl" ]]; then
    log_info "Building gridapi and gridctl..."
    make build
else
    log_info "gridapi and gridctl binaries already exist"
fi

#
# Step 3: Run database migrations
# TODO: Reset db?
#
log_info "and running migrations..."
"${PROJECT_ROOT}/bin/gridapi" db init \
    --db-url="${DB_URL}" \
    --server-url="${SERVER_URL}" || true
"${PROJECT_ROOT}/bin/gridapi" db migrate \
    --db-url="${DB_URL}" \
    --server-url="${SERVER_URL}"

#
# Step 4: Start gridapi server (No-Auth Mode)
#
log_info "Starting gridapi server in no-auth mode..."

# Unset any potential auth-related env vars
unset GRID_OIDC_ISSUER GRID_OIDC_CLIENT_ID GRID_OIDC_SIGNING_KEY_PATH
unset GRID_OIDC_EXTERNAL_IDP_ISSUER GRID_OIDC_EXTERNAL_IDP_CLIENT_ID GRID_OIDC_EXTERNAL_IDP_CLIENT_SECRET GRID_OIDC_EXTERNAL_IDP_REDIRECT_URI

# Start gridapi in background
"${PROJECT_ROOT}/bin/gridapi" serve \
    --server-addr ":8080" \
    --db-url "${DB_URL}" \
    --server-url "${SERVER_URL}" \
    > /tmp/grid-e2e-no-auth-gridapi.log 2>&1 &

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
        log_error "Check logs at /tmp/grid-e2e-no-auth-gridapi.log"
        exit 1
    fi
    sleep 1
done

#
# Step 5: Seed test data using gridctl
#
log_info "Seeding test data with gridctl..."
export GRID_SERVER_URL="http://localhost:8080"

# Create states (clean up .grid after each to avoid interference)
"${PROJECT_ROOT}/bin/gridctl" state create "test-producer-no-auth" && rm .grid
"${PROJECT_ROOT}/bin/gridctl" state create "test-consumer-no-auth" && rm .grid

# Create dependency
"${PROJECT_ROOT}/bin/gridctl" deps add \
    --from "test-producer-no-auth" \
    --output "vpc_id" \
    --to "test-consumer-no-auth"

log_info "✓ Test data seeded"


#
# Step 6: Start webapp dev server (Vite)
#
log_info "Starting webapp dev server..."

cd "${PROJECT_ROOT}/webapp"
pnpm install --silent

# Start webapp in background
pnpm dev > /tmp/grid-e2e-no-auth-webapp.log 2>&1 &
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
        log_error "Check logs at /tmp/grid-e2e-no-auth-webapp.log"
        exit 1
    fi
    sleep 1
done

log_info "All no-auth services started successfully!"
log_info "  - PostgreSQL: localhost:5432"
log_info "  - gridapi:    http://localhost:8080 (no-auth mode)"
log_info "  - webapp:     http://localhost:5173"

# Keep the script running so Playwright can detect when services are ready
# The script will be terminated by Playwright when tests complete
wait
