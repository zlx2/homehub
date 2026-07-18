# Deployment

```sh
cd /home/ubuntu/homehub-v2

cp deploy/compose/.env.example deploy/compose/.env.v2   # first time only
# edit .env.v2 with real passwords

make config     # validate
make up         # build + start
make check      # health checks
make logs       # follow logs
make down       # stop
```

Persistent data: `/srv/homehub-v2` + PostgreSQL Docker volume `homehub-v2-postgres`.

Secrets are managed by Bitwarden Secrets Manager:

```sh
make install-bws
make secrets-sync
```

Never place secret values in `.env.v2`, Compose files, Git, shell history, or logs.
