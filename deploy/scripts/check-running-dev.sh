#!/usr/bin/env sh
set -eu

base_url=${HOMEHUB_DEV_URL:-http://127.0.0.1:18080}

curl --fail --silent --show-error --output /dev/null "$base_url/"
system=$(curl --fail --silent --show-error "$base_url/api/v1/system")
services=$(curl --fail --silent --show-error "$base_url/api/v1/services")

printf '%s\n' "HomeHub portal and Control API are reachable."
printf '%s\n' "$system"
printf '%s\n' "$services"
