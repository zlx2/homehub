#!/usr/bin/env sh
set -eu

data_root=${V2_DATA_ROOT:-/srv/homehub-v2/data}
runtime_root=${V2_RUNTIME_ROOT:-/srv/homehub-v2/runtime}

install -d -m 0750 "$data_root"
install -d -o 65532 -g 65532 -m 0700 "$data_root/drop"
install -d -m 0711 "$runtime_root"
install -d -m 0755 "$runtime_root/acme-webroot/.well-known/acme-challenge"

printf '%s\n' "Prepared HomeHub V2 directories."
