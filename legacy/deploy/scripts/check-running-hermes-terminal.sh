#!/usr/bin/env bash
set -euo pipefail

systemctl --user is-active --quiet homehub-hermes-terminal.service
curl --fail --silent --show-error --output /dev/null http://10.0.0.15:7681/hermes/
test "$(docker inspect --format '{{.State.Health.Status}}' homehub-hermes-terminal-web-1)" = "healthy"

public_status="$(curl --insecure --silent --show-error --output /dev/null \
  --write-out '%{http_code}' https://111.229.205.99/hermes/)"
case "${public_status}" in
  401|403) ;;
  *)
    echo "Expected anonymous /hermes/ request to be denied, got HTTP ${public_status}." >&2
    exit 1
    ;;
esac

echo "Hermes terminal web client and host transport are healthy; the public route requires HomeHub authentication."
