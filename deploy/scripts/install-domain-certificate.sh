#!/usr/bin/env sh
set -eu

lineage=${RENEWED_LINEAGE:-/etc/letsencrypt/live/zlx2.com}
tls_dir=${HOMEHUB_DOMAIN_TLS_DIR:-/srv/homehub-v2/runtime/tls-zlx2}

case "$(basename "$lineage")" in
  zlx2.com) ;;
  *) printf '%s\n' "Refusing unexpected certificate lineage: $lineage" >&2; exit 1 ;;
esac

install -d -o root -g root -m 0700 "$tls_dir"
install -o root -g root -m 0444 "$lineage/fullchain.pem" "$tls_dir/fullchain.pem.new"
install -o root -g root -m 0400 "$lineage/privkey.pem" "$tls_dir/privkey.pem.new"
mv -f "$tls_dir/fullchain.pem.new" "$tls_dir/fullchain.pem"
mv -f "$tls_dir/privkey.pem.new" "$tls_dir/privkey.pem"

printf '%s\n' "Installed renewed zlx2.com certificate for HomeHub V2."
