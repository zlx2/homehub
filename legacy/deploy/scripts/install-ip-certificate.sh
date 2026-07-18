#!/usr/bin/env sh
set -eu

lineage=${RENEWED_LINEAGE:-/etc/letsencrypt/live/111.229.205.99}
tls_dir=${HOMEHUB_TLS_DIR:-/srv/homehub/runtime/tls}

case "$(basename "$lineage")" in
  111.229.205.99) ;;
  *) printf '%s\n' "Skipping non-IP certificate lineage: $lineage"; exit 0 ;;
esac

install -d -o root -g root -m 0700 "$tls_dir"
install -o root -g root -m 0444 "$lineage/fullchain.pem" "$tls_dir/fullchain.pem.new"
install -o root -g root -m 0400 "$lineage/privkey.pem" "$tls_dir/privkey.pem.new"
mv -f "$tls_dir/fullchain.pem.new" "$tls_dir/fullchain.pem"
mv -f "$tls_dir/privkey.pem.new" "$tls_dir/privkey.pem"

printf '%s\n' "Installed renewed HomeHub IP certificate for Traefik."
