#!/usr/bin/env sh
set -eu

base_url=${HOMEHUB_DEV_URL:-http://127.0.0.1:18080}

curl --fail --silent --show-error --output /dev/null "$base_url/"
session=$(curl --fail --silent --show-error "$base_url/api/v1/auth/session")
protected_status=$(curl --silent --output /dev/null --write-out '%{http_code}' "$base_url/api/v1/system")
if [ "$protected_status" != "401" ]; then
  printf '%s\n' "Expected protected API to return 401, got $protected_status" >&2
  exit 1
fi

printf '%s\n' "HomeHub portal and authentication API are reachable; owner APIs deny anonymous access."
printf '%s\n' "$session"
