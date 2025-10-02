#!/bin/bash
set -e

SERVER_URL="${SERVER_URL:-http://localhost:8080}"
HEALTH_ENDPOINT="${SERVER_URL}/health"
MAX_RETRIES="${MAX_RETRIES:-60}"
RETRY_INTERVAL="${RETRY_INTERVAL:-1}"

echo "Waiting for server health check at ${HEALTH_ENDPOINT}..."

for i in $(seq 1 $MAX_RETRIES); do
    if curl -s -f "${HEALTH_ENDPOINT}" > /dev/null 2>&1; then
        echo "✓ Server is healthy"
        exit 0
    fi

    echo "Attempt $i/$MAX_RETRIES: Server not ready yet..."
    sleep $RETRY_INTERVAL
done

echo "✗ Server failed to become healthy after ${MAX_RETRIES} attempts"
exit 1
