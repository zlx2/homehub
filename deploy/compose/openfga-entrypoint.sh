#!/bin/sh
# OpenFGA startup wrapper — reads database password from file and constructs DATASTORE_URI.
# Uses only POSIX shell builtins and I/O redirection — no external binaries.
# (The base OpenFGA image is distroless; only /bin/sh and /openfga are available.)

set -eu

readonly PASSWORD_FILE="${OPENFGA_DB_PASSWORD_FILE:-/run/secrets/openfga_db_password}"
readonly DB_HOST="${OPENFGA_DB_HOST:-postgres}"
readonly DB_PORT="${OPENFGA_DB_PORT:-5432}"
readonly DB_USER="${OPENFGA_DB_USER:-homehub_openfga}"
readonly DB_NAME="${OPENFGA_DB_NAME:-homehub_openfga}"
readonly DB_SSLMODE="${OPENFGA_DB_SSLMODE:-disable}"

# Read password using only shell builtins, strip trailing whitespace/newlines
password=""
while IFS= read -r line || [ -n "$line" ]; do
  password="$password$line"
done <"$PASSWORD_FILE"

export OPENFGA_DATASTORE_URI="postgres://${DB_USER}:${password}@${DB_HOST}:${DB_PORT}/${DB_NAME}?sslmode=${DB_SSLMODE}"

exec /openfga "$@"
