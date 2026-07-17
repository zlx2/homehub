#!/usr/bin/env sh
set -eu

admin_url=${HERMES_TRAEFIK_ADMIN_URL:-http://127.0.0.1:18081}

ping_response=$(curl --fail --silent --show-error "$admin_url/ping")
if [ "$ping_response" != "OK" ]; then
  printf '%s\n' "Unexpected Traefik ping response: $ping_response" >&2
  exit 1
fi

curl --fail --silent --show-error --output /dev/null "$admin_url/dashboard/"
overview=$(curl --fail --silent --show-error "$admin_url/api/overview")

printf '%s\n' "Traefik ping and dashboard are healthy."
printf '%s\n' "$overview"
