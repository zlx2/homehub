#!/usr/bin/env sh
set -eu

base_url=${HOMEHUB_PUBLIC_URL:-https://111.229.205.99}

curl --fail --silent --show-error --output /dev/null "$base_url/"
session=$(curl --fail --silent --show-error "$base_url/api/v1/auth/session")
protected_status=$(curl --silent --output /dev/null --write-out '%{http_code}' "$base_url/api/v1/system")
redirect=$(curl --silent --output /dev/null --write-out '%{redirect_url}' "http://111.229.205.99/")

if [ "$protected_status" != "401" ]; then
  printf '%s\n' "Expected anonymous API status 401, got $protected_status" >&2
  exit 1
fi
if [ "$redirect" != "$base_url/" ]; then
  printf '%s\n' "Unexpected HTTP redirect: $redirect" >&2
  exit 1
fi

printf '%s\n' "Public HTTPS, certificate verification, redirect, and anonymous denial passed."
printf '%s\n' "$session"
