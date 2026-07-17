#!/usr/bin/env sh
set -eu

data_root=${HOMEHUB_DATA_ROOT:-/srv/homehub}

install -d -m 0750 "$data_root"
install -d -o 70 -g 70 -m 0700 "$data_root/postgres"
install -d -m 0750 "$data_root/services"
install -d -o 65532 -g 65532 -m 0700 "$data_root/services/beszel"
install -d -o 65532 -g 65532 -m 0700 "$data_root/services/beszel/data"
install -d -o 65532 -g 65532 -m 0700 "$data_root/services/beszel/agent-data"
install -d -o 65532 -g 65532 -m 0700 "$data_root/services/beszel/socket"
install -d -m 0750 "$data_root/files"
install -d -m 0750 "$data_root/backups"
install -d -m 0750 "$data_root/runtime"
install -d -m 0700 "$data_root/runtime/secrets"
install -d -m 0755 "$data_root/runtime/acme-webroot/.well-known/acme-challenge"
install -d -m 0700 "$data_root/runtime/tls"

printf '%s\n' "Prepared persistent directories under $data_root."
