# Development infrastructure

The default Compose profile starts only Traefik and the restricted Docker socket
proxy. It binds the development HTTP entry point and dashboard to loopback:

- `127.0.0.1:18080`: service routes
- `127.0.0.1:18081`: Traefik ping, API, and dashboard

The existing port 443 listener is not changed.

PostgreSQL belongs to the `data` profile and will not start until its secret file
exists and the profile is explicitly enabled.

```sh
cp deploy/compose/.env.example deploy/compose/.env
make compose-config
make edge-up
make edge-check
```

Do not place secret values in `.env`. Runtime secret files are materialized under
`/srv/homehub/runtime/secrets` by the deployment process and are not
tracked by Git.

Before PostgreSQL is started for the first time, run the data directory helper
with root privileges:

```sh
sudo ./deploy/scripts/prepare-data-dirs.sh
sudo ./deploy/scripts/bootstrap-beszel.sh
```

This gives the PostgreSQL data directory to the UID used by the official image
without making the platform data tree world-writable.

## Bitwarden Secrets Manager

Production values are sourced from the `HomeHub Production` Secrets Manager
project. A project-scoped, read-only machine-account token is stored outside
the repository at `/etc/homehub/bws-access-token` with owner `root:root` and
mode `0400`.

Install the pinned CLI and materialize per-container files with:

```sh
make install-bws
make secrets-sync
```

The sync validates all required keys before atomically replacing any runtime
file. It creates separate copies where Control, PostgreSQL, and Drop require
different Unix ownership. Do not place the BWS access token in `.env`, shell
history, Compose configuration, or Git.

AI Gateway additionally requires `ai_deepseek_api_key` and
`ai_opencode_go_api_key` in the same Bitwarden project. These values are mounted
only into the internal AI Gateway container; business services receive signed,
short-lived model permissions instead.

The Beszel bootstrap creates its first local user and SSH identity without
persisting the generated bootstrap password. Normal access is then delegated to
HomeHub Control by the trusted `X-HomeHub-Email` header. The agent listens on a
shared Unix socket and Docker access is limited to read-only endpoints exposed at
`127.0.0.1:23750` for the host-networked agent.
