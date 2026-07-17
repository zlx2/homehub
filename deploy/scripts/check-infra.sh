#!/usr/bin/env sh
set -eu

repo_root=$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)
compose_file="$repo_root/deploy/compose/compose.yaml"

docker compose --env-file "$repo_root/deploy/compose/.env.example" \
  -f "$compose_file" config --quiet

printf '%s\n' "Compose configuration is valid."
