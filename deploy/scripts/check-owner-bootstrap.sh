#!/usr/bin/env sh
set -eu

base_url=${HOMEHUB_DEV_URL:-http://127.0.0.1:18080}
token_file=${HOMEHUB_BOOTSTRAP_TOKEN_FILE:-/srv/homehub/runtime/secrets/owner_setup_token}
response_file=$(mktemp)
trap 'rm -f "$response_file"' EXIT

token=$(cat "$token_file")
status=$(curl --silent --show-error --output "$response_file" --write-out '%{http_code}' \
  -H "Origin: $base_url" -H 'Content-Type: application/json' \
  --data-binary "{\"bootstrap_token\":\"$token\",\"username\":\"homehub-integration-check\",\"password\":\"integration-check-password\"}" \
  "$base_url/api/v1/setup/begin")
if [ "$status" != "201" ]; then
  printf '%s\n' "Owner bootstrap begin returned $status" >&2
  exit 1
fi

setup_id=$(sed -n 's/.*"setup_id":"\([^"]*\)".*/\1/p' "$response_file")
if [ -z "$setup_id" ]; then
  printf '%s\n' "Owner bootstrap response did not include a setup ID" >&2
  exit 1
fi

status=$(curl --silent --show-error --output /dev/null --write-out '%{http_code}' \
  -H "Origin: $base_url" -H 'Content-Type: application/json' \
  --data-binary "{\"setup_id\":\"$setup_id\",\"totp_code\":\"invalid\"}" \
  "$base_url/api/v1/setup/confirm")
if [ "$status" != "401" ]; then
  printf '%s\n' "Invalid TOTP confirmation returned $status" >&2
  exit 1
fi

docker exec homehub-postgres-1 psql -U postgres -d homehub_control -q \
  -c "DELETE FROM setup_attempts WHERE username='homehub-integration-check'" >/dev/null

printf '%s\n' "Owner bootstrap created encrypted pending credentials and rejected an invalid TOTP code."
