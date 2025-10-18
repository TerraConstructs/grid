#!/usr/bin/env bash
# Reset Keycloak environment (stop, prune volumes, restart) (FR-112)

set -euo pipefail

echo "⚠️  Keycloak Reset - This will DELETE all Keycloak data"
echo ""
echo "This script will:"
echo "  1. Stop Keycloak and PostgreSQL services"
echo "  2. Remove all volumes (including Keycloak database)"
echo "  3. Restart services with fresh state"
echo ""

# Ask for confirmation
read -p "Continue? (y/N) " -n 1 -r
echo
if [[ ! $REPLY =~ ^[Yy]$ ]]; then
    echo "Aborted."
    exit 0
fi

echo ""
echo "Stopping services..."
docker compose stop keycloak postgres

echo "Removing volumes..."
docker compose down -v

echo "Starting fresh services..."
docker compose up -d postgres keycloak

echo "Waiting for services to be healthy..."
sleep 5

echo ""
echo "✓ Keycloak environment reset complete!"
echo ""
echo "Next steps:"
echo "  1. Access Keycloak at http://localhost:8443"
echo "  2. Login with admin/admin"
echo "  3. Create realm 'grid'"
echo "  4. Create client 'grid-api' (see docs/local-dev.md)"
echo "  5. Configure groups claim mapper"
echo ""
echo "Note: Initial setup takes ~30 seconds for Keycloak to initialize"
