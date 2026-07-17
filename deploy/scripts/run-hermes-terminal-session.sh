#!/usr/bin/env bash
set -euo pipefail

session="homehub-hermes"
hermes="${HOME}/.local/bin/hermes"

if ! tmux has-session -t "${session}" 2>/dev/null; then
  tmux new-session -d -s "${session}" -c "${HOME}" "${hermes}" --tui
fi

tmux set-option -t "${session}" status off
tmux set-option -t "${session}" window-size latest
tmux set-window-option -t "${session}" aggressive-resize on

exec tmux attach-session -t "${session}"
