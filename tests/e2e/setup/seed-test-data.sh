#!/usr/bin/env bash
#
# Seed test data for E2E tests:
# - Create test user alice@example.com in Keycloak
# - Add alice to product-engineers group
# - Create additional test users as needed
#
# This script is idempotent - it can be run multiple times safely.

set -euo pipefail

# Color output
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

log_info() {
    echo -e "${GREEN}[E2E Seed]${NC} $*"
}

log_warn() {
    echo -e "${YELLOW}[E2E Seed]${NC} $*"
}

# Keycloak admin credentials (from docker-compose.yml)
KEYCLOAK_ADMIN="admin"
KEYCLOAK_ADMIN_PASSWORD="admin"
KEYCLOAK_URL="http://localhost:8443"
REALM="grid"

#
# Function: Execute kcadm command inside Keycloak container
#
kcadm() {
    docker-compose exec -T keycloak /opt/keycloak/bin/kcadm.sh "$@"
}

#
# Step 1: Authenticate with Keycloak admin API
#
log_info "Authenticating with Keycloak admin..."
kcadm config credentials \
    --server "${KEYCLOAK_URL}" \
    --realm master \
    --user "${KEYCLOAK_ADMIN}" \
    --password "${KEYCLOAK_ADMIN_PASSWORD}" \
    --config /tmp/kcadm.config

#
# Step 2: Create test user alice@example.com
#
log_info "Creating test user alice@example.com..."

# Check if user exists
USER_ID=$(kcadm get users -r "${REALM}" -q username=alice@example.com --fields id --format csv --noquotes --config /tmp/kcadm.config 2>/dev/null | tail -n1 || echo "")

if [[ -z "${USER_ID}" ]]; then
    # Create new user
    kcadm create users -r "${REALM}" \
        -s username=alice@example.com \
        -s email=alice@example.com \
        -s emailVerified=true \
        -s enabled=true \
        --config /tmp/kcadm.config

    log_info "User alice@example.com created"

    # Get the user ID
    USER_ID=$(kcadm get users -r "${REALM}" -q username=alice@example.com --fields id --format csv --noquotes --config /tmp/kcadm.config | tail -n1)
else
    log_info "User alice@example.com already exists"
fi

# Set password
log_info "Setting password for alice@example.com..."
kcadm set-password -r "${REALM}" \
    --username alice@example.com \
    --new-password "test123" \
    --temporary=false \
    --config /tmp/kcadm.config || log_warn "Password may already be set"

#
# Step 3: Add alice to product-engineers group
#
log_info "Adding alice@example.com to product-engineers group..."

# Get product-engineers group ID
GROUP_ID=$(kcadm get groups -r "${REALM}" -q search=product-engineers --fields id --format csv --noquotes --config /tmp/kcadm.config 2>/dev/null | tail -n1 || echo "")

if [[ -n "${GROUP_ID}" ]]; then
    # Add user to group
    kcadm update users/"${USER_ID}"/groups/"${GROUP_ID}" -r "${REALM}" \
        --config /tmp/kcadm.config 2>/dev/null || log_warn "User may already be in group"

    log_info "alice@example.com added to product-engineers group"
else
    log_warn "product-engineers group not found - skipping group assignment"
fi

#
# Step 4: Create additional test user for JIT provisioning test
#
log_info "Creating test user newuser@example.com (for JIT provisioning test)..."

NEW_USER_ID=$(kcadm get users -r "${REALM}" -q username=newuser@example.com --fields id --format csv --noquotes --config /tmp/kcadm.config 2>/dev/null | tail -n1 || echo "")

if [[ -z "${NEW_USER_ID}" ]]; then
    kcadm create users -r "${REALM}" \
        -s username=newuser@example.com \
        -s email=newuser@example.com \
        -s emailVerified=true \
        -s enabled=true \
        --config /tmp/kcadm.config

    log_info "User newuser@example.com created"

    # Set password
    kcadm set-password -r "${REALM}" \
        --username newuser@example.com \
        --new-password "test123" \
        --temporary=false \
        --config /tmp/kcadm.config
else
    log_info "User newuser@example.com already exists"
fi

#
# Step 5: Create platform engineer test user (for permission testing)
#
log_info "Creating test user platform@example.com..."

PLATFORM_USER_ID=$(kcadm get users -r "${REALM}" -q username=platform@example.com --fields id --format csv --noquotes --config /tmp/kcadm.config 2>/dev/null | tail -n1 || echo "")

if [[ -z "${PLATFORM_USER_ID}" ]]; then
    kcadm create users -r "${REALM}" \
        -s username=platform@example.com \
        -s email=platform@example.com \
        -s emailVerified=true \
        -s enabled=true \
        --config /tmp/kcadm.config

    log_info "User platform@example.com created"

    # Get the user ID
    PLATFORM_USER_ID=$(kcadm get users -r "${REALM}" -q username=platform@example.com --fields id --format csv --noquotes --config /tmp/kcadm.config | tail -n1)
else
    log_info "User platform@example.com already exists"
fi

# Set password
kcadm set-password -r "${REALM}" \
    --username platform@example.com \
    --new-password "test123" \
    --temporary=false \
    --config /tmp/kcadm.config || log_warn "Password may already be set"

# Add to platform-engineers group
PLATFORM_GROUP_ID=$(kcadm get groups -r "${REALM}" -q search=platform-engineers --fields id --format csv --noquotes --config /tmp/kcadm.config 2>/dev/null | tail -n1 || echo "")

if [[ -n "${PLATFORM_GROUP_ID}" ]]; then
    kcadm update users/"${PLATFORM_USER_ID}"/groups/"${PLATFORM_GROUP_ID}" -r "${REALM}" \
        --config /tmp/kcadm.config 2>/dev/null || log_warn "User may already be in group"

    log_info "platform@example.com added to platform-engineers group"
fi

log_info "Test data seeding complete!"
log_info "  - alice@example.com (password: test123) - member of product-engineers"
log_info "  - platform@example.com (password: test123) - member of platform-engineers"
log_info "  - newuser@example.com (password: test123) - no groups (for JIT test)"
