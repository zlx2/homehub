#!/usr/bin/env sh
set -eu

version=2.1.0
asset="bws-x86_64-unknown-linux-gnu-$version.zip"
expected_sha256=ba8233c3a4aee5d43e3c73bbd04d99e9bc5aba13bbbfd06d89b073abe732b860
base_url="https://github.com/bitwarden/sdk-sm/releases/download/bws-v$version"

temporary=$(mktemp -d /tmp/homehub-bws-install.XXXXXX)
case "$temporary" in
  /tmp/homehub-bws-install.*) ;;
  *) exit 1 ;;
esac
trap 'rm -rf -- "$temporary"' EXIT

curl --fail --silent --show-error --location "$base_url/$asset" --output "$temporary/$asset"
printf '%s  %s\n' "$expected_sha256" "$temporary/$asset" | sha256sum --check --status
unzip -q "$temporary/$asset" -d "$temporary/release"
binary=$(find "$temporary/release" -type f -name bws -print -quit)
test -n "$binary"
install -o root -g root -m 0755 "$binary" /usr/local/bin/bws

printf '%s\n' "Installed $(/usr/local/bin/bws --version) with pinned SHA-256 verification."
