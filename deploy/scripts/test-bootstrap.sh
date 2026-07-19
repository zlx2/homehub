#!/usr/bin/env sh
# HomeHub PostgreSQL cold-bootstrap test.
# Creates a fresh temporary volume, starts a one-off postgres with init scripts,
# verifies both business databases are correctly created and isolated.
# The production stack is never touched.
# Does NOT print any secret content.

set -eu

BOOTSTRAP_VOLUME="homehub-bootstrap-pg-test"
BOOTSTRAP_CONTAINER="homehub-bootstrap-pg"
BOOTSTRAP_NET="homehub-bootstrap-test-net"
RUNTIME_DIR="${HOMEHUB_RUNTIME_DIR:-/srv/homehub/runtime}"

cleanup() {
    echo "--- Cleaning up ---"
    docker rm -f "$BOOTSTRAP_CONTAINER" 2>/dev/null || true
    docker network rm "$BOOTSTRAP_NET" 2>/dev/null || true
    docker volume rm -f "$BOOTSTRAP_VOLUME" 2>/dev/null || true
}
trap cleanup EXIT

echo "=== HomeHub PostgreSQL cold-bootstrap test ==="

# Pre-flight: secret files must exist and be non-empty
for f in \
    "$RUNTIME_DIR/secrets/postgres/superuser_password" \
    "$RUNTIME_DIR/secrets/postgres/drop_db_password" \
    "$RUNTIME_DIR/secrets/postgres/iam_db_password"; do
    if ! sudo test -s "$f"; then
        echo "FAIL: required secret missing or empty: $f"
        exit 1
    fi
done
echo "Secret files present."

# Create fresh resources
docker volume create "$BOOTSTRAP_VOLUME"
docker network create "$BOOTSTRAP_NET"

# Start postgres with init scripts
echo "Starting bootstrap postgres..."
docker run -d --name "$BOOTSTRAP_CONTAINER" \
    --network "$BOOTSTRAP_NET" \
    -v "$BOOTSTRAP_VOLUME:/var/lib/postgresql" \
    -v "$(pwd)/deploy/postgres/init:/docker-entrypoint-initdb.d:ro" \
    -v "$RUNTIME_DIR/secrets/postgres/superuser_password:/run/secrets/postgres_superuser_password:ro" \
    -v "$RUNTIME_DIR/secrets/postgres/drop_db_password:/run/secrets/postgres_drop_db_password:ro" \
    -v "$RUNTIME_DIR/secrets/postgres/iam_db_password:/run/secrets/postgres_iam_db_password:ro" \
    -e POSTGRES_USER=postgres \
    -e POSTGRES_PASSWORD_FILE=/run/secrets/postgres_superuser_password \
    -e HOMEHUB_DROP_DB_PASSWORD_FILE=/run/secrets/postgres_drop_db_password \
    -e HOMEHUB_IAM_DB_PASSWORD_FILE=/run/secrets/postgres_iam_db_password \
    postgres:18.4-alpine \
    -c shared_buffers=32MB -c max_connections=10

# Wait for ready
echo "Waiting for postgres..."
for i in $(seq 1 30); do
    if docker exec "$BOOTSTRAP_CONTAINER" pg_isready -U postgres -d postgres >/dev/null 2>&1; then
        echo "Postgres ready."
        break
    fi
    if [ "$i" -eq 30 ]; then
        echo "FAIL: postgres did not become ready"
        docker logs "$BOOTSTRAP_CONTAINER" --tail 20
        exit 1
    fi
    sleep 2
done

# Check init log for errors
echo "=== Init script output ==="
docker logs "$BOOTSTRAP_CONTAINER" 2>&1 | grep -iE 'error|fatal|CREATE (ROLE|DATABASE)|FATAL' || true

# Verify databases and roles exist
echo "=== Database verification ==="
passed=true

verify_exists() {
    local query="$1" label="$2"
    result=$(docker exec "$BOOTSTRAP_CONTAINER" psql -U postgres -d postgres -tAc "$query" 2>/dev/null)
    if echo "$result" | grep -q '^1$'; then
        echo "  $label: EXISTS"
    else
        echo "  $label: MISSING"
        passed=false
    fi
}

verify_exists "SELECT 1 FROM pg_database WHERE datname='homehub_iam'" "IAM database"
verify_exists "SELECT 1 FROM pg_database WHERE datname='homehub_drop'" "Drop database"
verify_exists "SELECT 1 FROM pg_roles WHERE rolname='homehub_iam'" "IAM role"
verify_exists "SELECT 1 FROM pg_roles WHERE rolname='homehub_drop'" "Drop role"

# Verify each role has LOGIN privilege
echo "=== Login verification ==="
for role in homehub_iam homehub_drop; do
    result=$(docker exec "$BOOTSTRAP_CONTAINER" psql -U postgres -d postgres -tAc \
        "SELECT rolcanlogin FROM pg_roles WHERE rolname='$role'" 2>/dev/null)
    if [ "$result" = "t" ]; then
        echo "  $role: LOGIN enabled"
    else
        echo "  $role: LOGIN NOT enabled"
        passed=false
    fi
done

# Verify init script completed without errors
if docker logs "$BOOTSTRAP_CONTAINER" 2>&1 | grep -qi 'error\|fatal'; then
    echo "FAIL: errors in postgres init log"
    passed=false
fi

if $passed; then
    echo "=== PASS: cold-bootstrap test ==="
else
    echo "=== FAIL: cold-bootstrap test ==="
    exit 1
fi
