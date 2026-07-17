#!/usr/bin/env sh
set -eu

base_url=${HOMEHUB_PUBLIC_URL:-https://111.229.205.99}
compose_file=${HOMEHUB_COMPOSE_FILE:-deploy/compose/compose.yaml}
env_file=${HOMEHUB_ENV_FILE:-deploy/compose/.env.example}

for route in /demo/decider/ /demo/counter/; do
  status=$(curl --silent --output /dev/null --write-out '%{http_code}' "$base_url$route")
  if [ "$status" != "401" ]; then
    printf '%s\n' "Expected anonymous status 401 for $route, got $status" >&2
    exit 1
  fi
done

docker compose --env-file "$env_file" -f "$compose_file" exec -T demo-decider /demo-decider healthcheck
docker compose --env-file "$env_file" -f "$compose_file" exec -T demo-counter /demo-counter healthcheck

printf '%s\n' "Demo services are healthy and deny anonymous public access."
