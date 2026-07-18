#!/usr/bin/env sh
set -eu

data_root=${HOMEHUB_DATA_ROOT:-/srv/homehub}
repo_root=$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)
data_dir="$data_root/services/beszel/data"
socket_dir="$data_root/services/beszel/socket"
secrets_dir="$data_root/runtime/secrets"
agent_key_file="$secrets_dir/beszel_agent_key"
hub_image=${BESZEL_HUB_IMAGE:-henrygd/beszel:0.18.7}
bootstrap_name=homehub-beszel-bootstrap

if [ "$(id -u)" -ne 0 ]; then
  printf '%s\n' "Run this script as root so runtime files receive their container UID." >&2
  exit 1
fi

install -d -o 65532 -g 65532 -m 0700 "$data_dir" "$socket_dir"
install -d -m 0700 "$secrets_dir"

cleanup() {
  docker rm -f "$bootstrap_name" >/dev/null 2>&1 || true
}
trap cleanup EXIT INT TERM

if [ ! -f "$data_dir/data.db" ]; then
  bootstrap_password=$(od -An -N32 -tx1 /dev/urandom | tr -d ' \n')
  cleanup
  docker run --detach --name "$bootstrap_name" \
    --user 65532:65532 \
    --read-only \
    --tmpfs /tmp:uid=65532,gid=65532,mode=0700 \
    --volume "$data_dir:/beszel_data" \
    --volume "$socket_dir:/beszel_socket" \
    --volume "$repo_root/deploy/beszel/config.yml:/beszel_data/config.yml:ro" \
    --env USER_EMAIL=owner@homehub.local \
    --env USER_PASSWORD="$bootstrap_password" \
    --env APP_URL=https://111.229.205.99/server \
    --env CHECK_UPDATES=false \
    --env HTTP_PROXY= --env HTTPS_PROXY= --env http_proxy= --env https_proxy= \
    "$hub_image" >/dev/null

  ready=false
  for _ in $(seq 1 30); do
    if docker exec "$bootstrap_name" /beszel health --url http://127.0.0.1:8090 >/dev/null 2>&1; then
      ready=true
      break
    fi
    sleep 1
  done
  if [ "$ready" != true ]; then
    docker logs "$bootstrap_name" >&2
    printf '%s\n' "Beszel bootstrap did not become healthy." >&2
    exit 1
  fi
  cleanup
fi

if [ ! -f "$data_dir/id_ed25519" ]; then
  printf '%s\n' "Beszel hub private key is missing from $data_dir." >&2
  exit 1
fi

ssh-keygen -y -f "$data_dir/id_ed25519" > "$agent_key_file"
chown 65532:65532 "$agent_key_file"
chmod 0400 "$agent_key_file"

trap - EXIT INT TERM
printf '%s\n' "Beszel data and agent identity are ready. No bootstrap password was persisted."
