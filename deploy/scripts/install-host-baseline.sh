#!/bin/sh
set -eu

if [ "$(id -u)" -ne 0 ]; then
  printf '%s\n' "This installer must run as root." >&2
  exit 1
fi

repo_root=$(CDPATH= cd -- "$(dirname "$0")/../.." && pwd)
host_dir="$repo_root/deploy/host"

dockerd --validate --config-file "$host_dir/docker-daemon.json"

install -d -o root -g root -m 0755 /etc/systemd/journald.conf.d
install -d -o root -g root -m 0755 /etc/docker
install -o root -g root -m 0755 \
  "$host_dir/homehub-host-firewall.sh" \
  /usr/local/sbin/homehub-host-firewall
install -o root -g root -m 0644 \
  "$host_dir/homehub-host-firewall.service" \
  /etc/systemd/system/homehub-host-firewall.service
install -o root -g root -m 0644 \
  "$host_dir/journald-homehub.conf" \
  /etc/systemd/journald.conf.d/90-homehub-limits.conf
install -o root -g root -m 0644 \
  "$host_dir/docker-daemon.json" \
  /etc/docker/daemon.json

systemctl daemon-reload
systemctl enable --now homehub-host-firewall.service
systemctl restart systemd-journald.service

printf '%s\n' "Host baseline installed. Docker logging defaults apply after the next planned Docker restart."
