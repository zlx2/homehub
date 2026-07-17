#!/usr/bin/env bash
set -euo pipefail

TTYD_VERSION="1.7.7"
RELEASE_URL="https://github.com/tsl0922/ttyd/releases/download/${TTYD_VERSION}"

if [[ ${EUID} -eq 0 ]]; then
  echo "Run this installer as the ubuntu user, not root." >&2
  exit 1
fi

case "$(uname -m)" in
  x86_64) asset="ttyd.x86_64" ;;
  aarch64|arm64) asset="ttyd.aarch64" ;;
  *) echo "Unsupported architecture: $(uname -m)" >&2; exit 1 ;;
esac

tmp_dir="$(mktemp -d)"
trap 'rm -rf "${tmp_dir}"' EXIT

download() {
  local url="$1"
  local output="$2"
  if ! curl --fail --silent --show-error --location --output "${output}" "${url}"; then
    curl --fail --silent --show-error --location \
      --proxy http://127.0.0.1:1081 --output "${output}" "${url}"
  fi
}

download "${RELEASE_URL}/${asset}" "${tmp_dir}/${asset}"
download "${RELEASE_URL}/SHA256SUMS" "${tmp_dir}/SHA256SUMS"

expected="$(awk -v name="${asset}" '$2 == name || $2 == "*" name { print $1 }' "${tmp_dir}/SHA256SUMS")"
if [[ -z "${expected}" ]]; then
  echo "No checksum found for ${asset}." >&2
  exit 1
fi
echo "${expected}  ${tmp_dir}/${asset}" | sha256sum --check --status

install -d -m 0755 "${HOME}/.local/bin" "${HOME}/.config/systemd/user"
install -m 0755 "${tmp_dir}/${asset}" "${HOME}/.local/bin/ttyd"
install -m 0644 \
  "$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)/systemd/homehub-hermes-terminal.service" \
  "${HOME}/.config/systemd/user/homehub-hermes-terminal.service"

systemctl --user daemon-reload
systemctl --user enable --now homehub-hermes-terminal.service

"${HOME}/.local/bin/ttyd" --version
systemctl --user --no-pager --full status homehub-hermes-terminal.service
