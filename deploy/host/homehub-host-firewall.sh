#!/bin/sh
set -eu

iptables_bin=/usr/sbin/iptables

# Fail2Ban redirects banned SSH clients to 127.0.0.1:2222. Keep direct
# connections to the public address away from Endlessh while allowing that
# loopback-destination redirect to reach the tarpit.
if ! "$iptables_bin" -C INPUT ! -d 127.0.0.1/32 -p tcp --dport 2222 -j DROP 2>/dev/null; then
  "$iptables_bin" -I INPUT 1 ! -d 127.0.0.1/32 -p tcp --dport 2222 -j DROP
fi
