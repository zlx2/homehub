#!/usr/bin/env sh
set -eu

base_url=${HOMEHUB_PUBLIC_URL:-https://111.229.205.99}
compose_file=${HOMEHUB_COMPOSE_FILE:-deploy/compose/compose.yaml}
env_file=${HOMEHUB_ENV_FILE:-deploy/compose/.env.example}

anonymous_status=$(curl --silent --output /dev/null --write-out '%{http_code}' "$base_url/server/")
if [ "$anonymous_status" != "401" ]; then
  printf '%s\n' "Expected anonymous server panel status 401, got $anonymous_status" >&2
  exit 1
fi

docker compose --env-file "$env_file" -f "$compose_file" exec -T beszel /beszel health --url http://127.0.0.1:8090 >/dev/null
docker compose --env-file "$env_file" -f "$compose_file" exec -T beszel-agent /agent health >/dev/null

proxy_status=$(curl --silent --output /dev/null --write-out '%{http_code}' http://127.0.0.1:23750/version)
if [ "$proxy_status" != "200" ]; then
  printf '%s\n' "Expected loopback Docker proxy status 200, got $proxy_status" >&2
  exit 1
fi

mutation_status=$(curl --silent --request POST --output /dev/null --write-out '%{http_code}' http://127.0.0.1:23750/containers/create)
if [ "$mutation_status" != "403" ]; then
  printf '%s\n' "Expected Docker proxy mutation denial 403, got $mutation_status" >&2
  exit 1
fi

printf '%s\n' "Beszel hub, local agent, anonymous denial, and read-only Docker API checks passed."
