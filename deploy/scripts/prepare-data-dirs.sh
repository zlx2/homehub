#!/usr/bin/env sh
set -eu

data_root=${HOMEHUB_DATA_ROOT:-/srv/homehub/data}
runtime_root=${HOMEHUB_RUNTIME_ROOT:-/srv/homehub/runtime}

install -d -m 0750 "$data_root"
install -d -o 65532 -g 65532 -m 0700 "$data_root/drop"
install -d -m 0711 "$runtime_root"
install -d -m 0755 "$runtime_root/acme-webroot/.well-known/acme-challenge"

printf '%s\n' "Prepared HomeHub directories."
