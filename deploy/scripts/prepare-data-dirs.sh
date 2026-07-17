#!/usr/bin/env sh
set -eu

data_root=${HERMES_DATA_ROOT:-/srv/hermes-platform}

install -d -m 0750 "$data_root"
install -d -o 999 -g 999 -m 0700 "$data_root/postgres"
install -d -m 0750 "$data_root/services"
install -d -m 0750 "$data_root/files"
install -d -m 0750 "$data_root/backups"
install -d -m 0750 "$data_root/runtime"
install -d -m 0700 "$data_root/runtime/secrets"

printf '%s\n' "Prepared persistent directories under $data_root."
