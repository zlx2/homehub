# ADR 0009: Expose the native Hermes TUI through HomeHub

- Status: Accepted
- Date: 2026-07-17

## Context

The server already has a complete, customized Nous Research Hermes Agent
installation under `~/.hermes`. The owner primarily uses Hermes through its TUI
and wants the same terminal workflow from a browser. The Hermes Dashboard is a
larger management application than this use case requires.

Running Hermes inside a container would change the command execution
environment and require duplicating host tools, paths, credentials, and browser
integrations. Reimplementing PTY and terminal emulation in HomeHub would also
duplicate mature existing software.

## Decision

- Keep Hermes Agent installed and executed natively as the `ubuntu` user.
- Run ttyd 1.7.7 as a user-level systemd service on the host.
- Run `hermes --tui` inside a persistent tmux session named
  `homehub-hermes`.
- Publish ttyd only through the existing HomeHub `/hermes/` route.
- Reuse HomeHub's owner session through Traefik ForwardAuth. Hermes Terminal
  does not implement another login or consume HomeHub identity headers.
- Do not expose this module through share links.
- HomeHub does not read Hermes configuration, sessions, memories, secrets, or
  databases. The integration boundary is the interactive terminal process.
- Do not make Hermes Gateway a HomeHub runtime dependency. The existing
  messaging gateway continues to run independently.

## Consequences

The browser receives the same TUI, slash commands, session picker, tools, and
host environment as an SSH terminal. Closing the browser detaches the tmux
client without ending Hermes, and a later browser can reattach.

This module intentionally has the same effective host privileges as the
owner's native Hermes process. It is therefore owner-only and is a special host
integration rather than a normal containerized business microservice. A future
structured web client may use `hermes serve`, but it is not required for the
terminal-first interface.
