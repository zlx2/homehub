# Deployment

```sh
cd /home/ubuntu/homehub-v2

cp deploy/compose/.env.example deploy/compose/.env   # first time only
# edit .env with local runtime values; never commit it

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

Never place secret values in Compose files, Git, shell history, or logs.
