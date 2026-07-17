# Hermes Terminal Web

A mobile-first xterm.js client for the host-native Hermes TUI. It speaks the
small ttyd WebSocket protocol directly and stores no Hermes data.

- `/hermes/` serves this static client.
- `/hermes/token` and `/hermes/ws` are routed by Traefik to host-native ttyd.
- ttyd attaches each browser to the persistent `homehub-hermes` tmux session.
- HomeHub ForwardAuth remains the only browser authentication boundary.

The UI deliberately stays terminal-first: it adds responsive sizing, a softer
ANSI palette, connection controls, font sizing, fullscreen support, and a
mobile special-key bar without parsing or duplicating Hermes sessions.
