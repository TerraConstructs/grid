#!/bin/bash
set -e

DB_URL="${DB_URL:-postgres://grid:gridpass@localhost:5432/grid?sslmode=disable}"

echo "Cleaning test data from database..."

# Use psql to clean test data
docker compose exec -T postgres psql "$DB_URL" <<EOF
-- Delete test states (those starting with 'test-')
DELETE FROM states WHERE logic_id LIKE 'test-%';

-- Show remaining count
SELECT COUNT(*) as remaining_states FROM states;
EOF

echo "✓ Test data cleaned"
