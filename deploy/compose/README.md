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
`/srv/hermes-platform/runtime/secrets` by the deployment process and are not
tracked by Git.

Before PostgreSQL is started for the first time, run the data directory helper
with root privileges:

```sh
sudo ./deploy/scripts/prepare-data-dirs.sh
```

This gives the PostgreSQL data directory to the UID used by the official image
without making the platform data tree world-writable.
