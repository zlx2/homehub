# Hermes HomeHub integration

Hermes is HomeHub's root housekeeper. A long-lived, revocable `agent.root` token is
stored in Bitwarden Secrets Manager as `hermes_root_token` and materialized to:

```text
/srv/homehub/runtime/secrets/hermes_root_token
```

The token file is readable only by the Hermes host user. The containing secrets
directory grants that user's group traverse permission but not directory listing.
Other HomeHub secret files retain their individual ownership and modes.

Hermes calls HomeHub through `homehub-api`. The wrapper sends the credential to
Control, which authorizes the requested catalog service and issues a short-lived,
audience-bound internal identity. Business services never receive the long-lived
root token directly.

Install the repository-backed entry points on the server:

```bash
mkdir -p ~/.local/bin ~/.hermes/skills/homehub-housekeeper
ln -sfn /home/ubuntu/homehub/deploy/scripts/homehub-agent-api ~/.local/bin/homehub-api
ln -sfn /home/ubuntu/homehub/integrations/hermes/homehub/SKILL.md \
  ~/.hermes/skills/homehub-housekeeper/SKILL.md
```

Run the BWS materializer after creating or rotating the secret. A new Hermes
session discovers the skill; Hermes itself does not need to be restarted.
